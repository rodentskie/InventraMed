# Purchase Orders (Create, Read, Receive)

## Overview

This will be done in `apps/api`. Add endpoints to create a purchase order (PO)
with its line items, browse POs, and receive a PO in full, the same layered
shape as the medicine, inventory and supplier endpoints.

| Endpoint                              | Purpose                                              | Success |
|----------------------------------------|-------------------------------------------------------|---------|
| `POST /purchase-orders`               | Create a PO for a supplier with line items            | `201`   |
| `GET /purchase-orders`                | List POs, paginated                                   | `200`   |
| `GET /purchase-orders/{id}`           | Get one PO with its items and receipts                | `200`   |
| `POST /purchase-orders/{id}/receive`  | Receive the whole PO into stock                       | `201`   |

This is the scope @context/features/02-audit-purchase-orders.spec.md set for
the first pass: "Create PO" and "Receive full PO". Partial, damaged and
returned receiving is additive later — the schema already holds it, and this
spec always writes `quantity_damaged = 0` and `quantity_returned = 0`. There is
no update, cancel or delete of a PO and no editing of its items (see Open
Questions — this has a knock-on effect on medicine and supplier deletes).

All four require the auth middleware, like the other endpoints. RBAC is still
out of scope.

New packages `internal/handler/purchaseorder`, `internal/service/purchaseorder`
and `internal/repository/purchaseorder`. No new migration — the four tables
(`00012`–`00015`), the quantity-sync trigger (`00018`), the audit triggers
(`00019`) and the `updated_at` trigger (`00020`) already exist.

Two things this feature does that the earlier ones did not need to:

- **It is the first feature to write to audited tables.** The audit trigger
  (`00019`) reads the acting user from the `app.actor_id` session setting, and
  @context/database-schema.md flags that every write path must set it, or
  `audit_logs.actor_id` silently ends up `NULL`. `suppliers`, `medicines` and
  `inventory_entries` are not audited, so nothing has set it yet. This spec
  wires it into the repository's transaction helper (section 3).
- **It closes the deleted-row race** that the medicine and supplier delete
  features called out: a delete that commits first lets a waiting PO insert
  succeed against a soft-deleted supplier or medicine, because the foreign key
  only checks that the row exists. Create therefore locks and re-checks both
  (section 4).

---

## Requirements

Layered per @context/go-standards.md — Handler → Service → Repository, each
depending on the layer below via interfaces.

### 1. Domain (`internal/domain`)

New file `purchase_order.go`:

```go
const (
    PurchaseOrderStatusDraft             = "draft"
    PurchaseOrderStatusOrdered           = "ordered"
    PurchaseOrderStatusPartiallyReceived = "partially_received"
    PurchaseOrderStatusReceived          = "received"
    PurchaseOrderStatusCancelled         = "cancelled"
)

type PurchaseOrder struct {
    ID           string
    SupplierID   string
    Status       string
    // OrderDate and ExpectedDate are calendar dates; only year, month and day
    // are meaningful.
    OrderDate    time.Time
    ExpectedDate *time.Time
    CreatedBy    string
    Notes        *string
    CreatedAt    time.Time
    UpdatedAt    time.Time
    // Items and Receipts are only filled by FindByID and Create.
    Items        []*PurchaseOrderItem
    Receipts     []*PurchaseOrderReceipt
}

type PurchaseOrderItem struct {
    ID              string
    PurchaseOrderID string
    MedicineID      string
    QuantityOrdered int
    CreatedAt       time.Time
}

type PurchaseOrderReceipt struct {
    ID              string
    PurchaseOrderID string
    ReceivedBy      string
    ReceivedAt      time.Time
    Notes           *string
    CreatedAt       time.Time
    Items           []*PurchaseOrderReceiptItem
}

type PurchaseOrderReceiptItem struct {
    ID                     string
    PurchaseOrderReceiptID string
    PurchaseOrderItemID    string
    QuantityReceived       int
    QuantityDamaged        int
    QuantityReturned       int
    Notes                  *string
    CreatedAt              time.Time
}
```

Items, receipts and receipt items have no `UpdatedAt` or `DeletedAt` — the
tables have neither (they are immutable once created).

### 2. Errors (`pkg/apperror`)

Add next to the medicine ones:

```go
var (
    // ErrSupplierNotFound and ErrMedicineNotFound are returned by create when
    // the body references a supplier or medicine that is not active. They wrap
    // ErrNotFound, so callers that don't care which one can still match it.
    ErrSupplierNotFound = fmt.Errorf("supplier not found: %w", ErrNotFound)
    ErrMedicineNotFound = fmt.Errorf("medicine not found: %w", ErrNotFound)

    // ErrPurchaseOrderNotReceivable is returned when receiving a PO whose
    // status is not draft or ordered. It wraps ErrConflict.
    ErrPurchaseOrderNotReceivable = fmt.Errorf("purchase order cannot be received: %w", ErrConflict)
)
```

Create can miss two different things, so a bare `ErrNotFound` is no longer
enough to pick the message.

### 3. Repository (`internal/repository/purchaseorder`)

GORM. The package owns four tables, so it declares four private `record`
types (`purchase_orders`, `purchase_order_items`, `purchase_order_receipts`,
`purchase_order_receipt_items`), each mapped via `TableName()`.
`purchase_orders` follows the soft-delete convention (`DeletedAt
gorm.DeletedAt`, no `gorm.Model`); the other three declare no `DeletedAt` and
no `autoUpdateTime` field. `id`, `created_at`, `updated_at` and `received_at`
come from the database defaults.

```go
type ListFilter struct {
    Limit  int
    Offset int
}

type Repository interface {
    // Create inserts the PO and its items. The PO's Status is written as given.
    // The returned PO has its Items, with IDs.
    Create(ctx context.Context, po *domain.PurchaseOrder) (*domain.PurchaseOrder, error)
    // List returns one page of active POs without their items or receipts,
    // newest first, and the total number of active POs.
    List(ctx context.Context, filter ListFilter) ([]*domain.PurchaseOrder, int64, error)
    // FindByID returns the active PO with its items and receipts (each
    // receipt with its items), or apperror.ErrNotFound. Uses a constant
    // number of queries, never one per row.
    FindByID(ctx context.Context, id string) (*domain.PurchaseOrder, error)
    // LockStatusByID returns the status of the active PO with the ID and locks
    // the row until the transaction ends. found is false when there is none.
    LockStatusByID(ctx context.Context, id string) (status string, found bool, err error)
    // ListItems returns the PO's items ordered by medicine_id, id.
    ListItems(ctx context.Context, purchaseOrderID string) ([]*domain.PurchaseOrderItem, error)
    // CreateReceipt inserts the receipt and its items, in the order given.
    CreateReceipt(ctx context.Context, receipt *domain.PurchaseOrderReceipt) (*domain.PurchaseOrderReceipt, error)
    // UpdateStatus writes only the status. Returns apperror.ErrNotFound when no
    // active PO has the ID.
    UpdateStatus(ctx context.Context, id, status string) error
    // Transaction runs fn in a single transaction, first setting the
    // transaction-local app.actor_id to actorID so the audit triggers record
    // who acted. fn gets a Repository plus a medicine and a supplier
    // Repository bound to the same transaction.
    Transaction(
        ctx context.Context,
        actorID string,
        fn func(tx Repository, medicines medicinerepo.Repository, suppliers supplierrepo.Repository) error,
    ) error
}
```

- **`Transaction` sets the actor itself**, as its first statement:
  `SELECT set_config('app.actor_id', ?, true)` (`true` = local to the
  transaction; `SET LOCAL` cannot take a bind parameter). Making `actorID` a
  parameter means a write path cannot forget it. This closes the
  `app.actor_id` open question in @context/database-schema.md for this
  feature; every future write to an audited table must go through a
  transaction that does the same
- `Create` inserts the header, then the items. `Update`s use `Select(...)`
  like the other repositories, so `UpdateStatus` writes only `status`
  (`updated_at` is set by the `00020` trigger)
- `FindByID` orders items by `created_at, id` and receipts by `received_at,
  id` — deterministic, though not necessarily the order the client sent items
  in (an item has no position column)
- `List` orders by `created_at DESC, id DESC` for stable pages
- Maps to `domain` types at the package boundary — `record` is never exposed

### 4. Service (`internal/service/purchaseorder`)

```go
type ItemInput struct {
    MedicineID      string
    QuantityOrdered int
}

type CreateInput struct {
    SupplierID   string
    OrderDate    time.Time
    ExpectedDate *time.Time
    Notes        *string
    Items        []ItemInput
    // CreatedBy is the ID of the user creating the PO.
    CreatedBy string
}

type ReceiveInput struct {
    PurchaseOrderID string
    Notes           *string
    // ReceivedBy is the ID of the user receiving the PO.
    ReceivedBy string
}

type Service interface {
    Create(ctx context.Context, input CreateInput) (*domain.PurchaseOrder, error)
    List(ctx context.Context, filter ListFilter) (*Page, error)
    GetByID(ctx context.Context, id string) (*domain.PurchaseOrder, error)
    Receive(ctx context.Context, input ReceiveInput) (*domain.PurchaseOrderReceipt, error)
}
```

`ListFilter = purchaseorder.ListFilter`; `Page` is `{PurchaseOrders
[]*domain.PurchaseOrder, Total int64}` like the other services.

**`Create`**, in one `Transaction(ctx, input.CreatedBy, ...)`:

1. `suppliers.LockByID(ctx, input.SupplierID)` → not found →
   `apperror.ErrSupplierNotFound`
2. For each distinct medicine ID **in ascending order**,
   `medicines.LockByID(ctx, id)` → not found → `apperror.ErrMedicineNotFound`
3. `tx.Create` with `Status = draft` and `CreatedBy = input.CreatedBy`; nothing
   is inserted if step 1 or 2 failed

Why lock rather than just check: `LockByID` takes `FOR UPDATE` on an *active*
row, so it serializes against a concurrent supplier or medicine delete (which
locks the same row and then looks for POs). Whichever transaction gets there
first wins cleanly: if the delete commits first, create's lock finds no active
row and returns not found; if create commits first, the delete finds the PO
and returns `409`. Locking medicines in ascending ID order keeps two
concurrent creates that share medicines from deadlocking.

**`Receive`** ("receive full PO"), in one `Transaction(ctx, input.ReceivedBy,
...)`:

1. `LockStatusByID` → not found → `apperror.ErrNotFound`. The row lock also
   stops two people receiving the same PO at once — the second waits, then
   sees `received` and gets step 2's error
2. Status other than `draft` or `ordered` →
   `apperror.ErrPurchaseOrderNotReceivable`; nothing is written
3. `ListItems` (ordered by `medicine_id, id`)
4. `CreateReceipt` with `ReceivedBy` and `Notes`, and one receipt item per PO
   item: `QuantityReceived = QuantityOrdered`, `QuantityDamaged = 0`,
   `QuantityReturned = 0`, in that same item order (a consistent order keeps
   concurrent receives of POs that share medicines from deadlocking on the
   quantity-sync trigger's row updates)
5. `UpdateStatus(ctx, id, received)`

The service never writes `medicines.quantity` — `trg_purchase_order_receipt_items_sync_quantity`
(`00018`) increments it when each receipt item is inserted.

`List` and `GetByID` call the repository directly. `apperror.ErrNotFound`,
`ErrSupplierNotFound`, `ErrMedicineNotFound` and `ErrConflict` errors pass
through unchanged; other repository errors are logged and wrapped with
`fmt.Errorf("<operation> purchase order: %w", err)`.

### 5. Handler (`internal/handler/purchaseorder`)

Read the caller from `middleware.AccountFromContext` (fail closed with `401
unauthorized` if missing), decode/validate, map errors, respond with the
`response` helpers only. `created_by` and `received_by` are taken from the
token, never the body.

**`POST /purchase-orders` — body and validation**

Trim all string fields first. Lowercase every UUID before checking it, so the
same ID in different cases counts as the same. Top-level fields are checked in
the order below, then `items` in order. First failing rule wins, `400`. In the
item messages, `i` is the zero-based position, e.g. `items[0].medicine_id is
required`.

| Field                        | Rule                                                              | Message                                                                                   |
|-------------------------------|---------------------------------------------------------------------|---------------------------------------------------------------------------------------------|
| body                          | Valid JSON, max 1 MiB                                                | `invalid request body`                                                                       |
| `supplier_id`                 | Required, valid UUID                                                 | `supplier_id is required` / `invalid supplier_id`                                            |
| `order_date`                  | Required, `YYYY-MM-DD`. Past dates are allowed                       | `order_date is required` / `order_date must be a valid date in YYYY-MM-DD format`            |
| `expected_date`               | Optional, `YYYY-MM-DD`, not before `order_date`. Empty is stored as `NULL` | `expected_date must be a valid date in YYYY-MM-DD format` / `expected_date must not be before order_date` |
| `notes`                       | Optional, max 500 chars. Empty is stored as `NULL`                   | `notes is too long`                                                                          |
| `items`                       | Required, 1–100 entries                                              | `items is required` / `items must have at most 100 entries`                                  |
| `items[i].medicine_id`        | Required, valid UUID, not repeated within `items`                    | `items[i].medicine_id is required` / `items[i].medicine_id is invalid` / `items[i].medicine_id is repeated` |
| `items[i].quantity_ordered`   | Required integer, `1`–`2147483647`. Use `*int` so a missing value isn't mistaken for `0` | `items[i].quantity_ordered is required` / `items[i].quantity_ordered must be greater than zero` / `items[i].quantity_ordered is too large` |

`2147483647` is the largest value the `int` column holds; without the cap a
larger number would pass validation and fail in the database as a `500`.

**`GET /purchase-orders` — query parameters**

Same mechanism as `GET /medicines` (@context/features/07-medicine-RUD.spec.md):

| Param    | Rule                   | Default | Message on failure                                 |
|----------|-------------------------|---------|-----------------------------------------------------|
| `limit`  | Whole number, `1`–`100` | `20`    | `limit must be a whole number between 1 and 100`     |
| `offset` | Whole number, `>= 0`    | `0`     | `offset must be a whole number of zero or greater`   |

No other filters for this pass (see Open Questions).

**`POST /purchase-orders/{id}/receive` — body**

The body is optional: an empty body is the same as `{}`. When present it is
valid JSON, max 1 MiB, and `notes` is optional, trimmed, max 500 chars, empty
stored as `NULL` (`invalid request body` / `notes is too long`). Any other
field in it is ignored — `received_by` in particular is never read.

**Path values**

- `id` (`GET`, `receive`) — read with `r.PathValue("id")`, must be a valid
  UUID; otherwise `400 invalid purchase order id`

**Error mapping**

Check `ErrSupplierNotFound` and `ErrMedicineNotFound` before the bare
`ErrNotFound`, since they wrap it.

| Error                              | Status | Message                                                  | Used by      |
|--------------------------------------|--------|-------------------------------------------------------------|---------------|
| no caller in context                | `401`  | `unauthorized`                                              | all           |
| invalid body / query / path id      | `400`  | the validation message                                      | all           |
| `ErrSupplierNotFound`               | `404`  | `supplier not found`                                        | create        |
| `ErrMedicineNotFound`               | `404`  | `medicine not found`                                        | create        |
| `apperror.ErrNotFound`              | `404`  | `purchase order not found`                                  | get, receive  |
| `ErrPurchaseOrderNotReceivable`     | `409`  | `purchase order cannot be received in its current status`   | receive       |
| anything else                       | `500`  | `internal server error` (log the real error, never return it) | all         |

### 6. Wiring

In `apps/api/cmd/api/main.go`, register through the prefix wrapper, each
wrapped with the existing `auth` middleware:

```go
r.Handle("POST /purchase-orders", auth(purchaseOrderHandler.Create))
r.Handle("GET /purchase-orders", auth(purchaseOrderHandler.List))
r.Handle("GET /purchase-orders/{id}", auth(purchaseOrderHandler.GetByID))
r.Handle("POST /purchase-orders/{id}/receive", auth(purchaseOrderHandler.Receive))
```

→ served at `/api/purchase-orders...` with the default prefix.

### 7. Swagger

Update `apps/api/internal/handler/swagger/openapi.json` once the endpoints
work:

- Add a `purchase-orders` tag
- `POST /purchase-orders` (`operationId: createPurchaseOrder`): request body
  `CreatePurchaseOrderRequest`, responses `201`, `400`, `401`, `404`, `500`
- `GET /purchase-orders` (`operationId: listPurchaseOrders`): `limit`,
  `offset` query params, responses `200`, `400`, `401`, `500`
- `GET /purchase-orders/{id}` (`operationId: getPurchaseOrder`): `id` path
  param (`format: uuid`), responses `200`, `400`, `401`, `404`, `500`
- `POST /purchase-orders/{id}/receive` (`operationId: receivePurchaseOrder`):
  optional request body `ReceivePurchaseOrderRequest`, responses `201`,
  `400`, `401`, `404`, `409`, `500`
- Every operation uses `security: [{"bearerAuth": []}]`
- Add schemas `CreatePurchaseOrderRequest` (with an inline or named item
  schema), `ReceivePurchaseOrderRequest`, `PurchaseOrder` (header only, used
  by the list), `PurchaseOrderItem`, `PurchaseOrderDetail` (header plus
  `items` and `receipts`), `PurchaseOrderReceipt`, `PurchaseOrderReceiptItem`,
  `CreatePurchaseOrderResponse`, `PurchaseOrderResponse`,
  `PurchaseOrderListResponse`, `ReceivePurchaseOrderResponse`; reuse
  `ErrorResponse`
- The `404` on create lists both examples (`supplier not found`, `medicine
  not found`)
- Must remain valid JSON; `internal/handler/swagger/handler_test.go` must
  still pass

### 8. Tests

- **Repository** — a GORM DryRun SQL test, mirroring
  `repository/medicine/repository_sql_test.go`, for: `Transaction` issuing
  `set_config('app.actor_id', …, true)` before anything else;
  `LockStatusByID` (`FOR UPDATE`, `deleted_at IS NULL`); `List` and
  `FindByID` scoping to active POs and ordering; `UpdateStatus` writing only
  `status`. Skip the thin passthroughs (`Create`, `CreateReceipt`,
  `ListItems`) per @context/go-standards.md
- **Service** (mocked repositories) —
  - `Create`: success (call order is actor-bound transaction, supplier lock,
    medicine locks in ascending ID order, create; status is `draft`;
    `CreatedBy` reaches the repository), supplier not found short-circuits
    before any medicine lock or insert, a missing medicine short-circuits
    before the insert, each repository error path
  - `Receive`: success for `draft` and for `ordered` (one receipt item per PO
    item, `QuantityReceived == QuantityOrdered`, damaged and returned `0`,
    items in `ListItems` order, `ReceivedBy` and `Notes` reach the
    repository, status set to `received` last), not found, each of `received`,
    `cancelled` and `partially_received` → `ErrPurchaseOrderNotReceivable`
    with no receipt written and no status update, each repository error path
  - `List` passes the filter through and returns the page/total; `GetByID`
    found/not-found/error
  - The actor ID reaches `Transaction` for both `Create` and `Receive`
- **Handler** (mocked service, through the real middleware) — `201` body
  shape for create and receive, every `400` case in the validation table
  (including each item rule, a repeated medicine in different letter case,
  `expected_date` before `order_date`, 0 and 101 items, `2147483648`), an
  empty receive body accepted, `received_by` / `created_by` in a body ignored,
  invalid path id for `get`/`receive`, `404` (all three messages), `409`,
  `500`, no caller in context → `401` for all four, `List` defaults and custom
  `limit`/`offset`, each invalid `limit`/`offset`, empty result is `"data": []`
- Coverage ≥ 90% on service and handler

---

## Endpoints

All require `Authorization: Bearer <access_token>`, served under `API_PREFIX`
(default `/api`).

### `POST /purchase-orders`

Request:

```json
{
  "supplier_id": "b1a2c3d4-6a1e-4c3e-9d0e-5f1d2a7c9e11",
  "order_date": "2026-09-20",
  "expected_date": "2026-09-30",
  "notes": "Quarterly restock",
  "items": [
    { "medicine_id": "0b8f3c62-6a1e-4c3e-9d0e-5f1d2a7c9e11", "quantity_ordered": 200 },
    { "medicine_id": "5c7d9e21-3b5c-4e8a-a1f6-9c0b2d4e6f88", "quantity_ordered": 50 }
  ]
}
```

`201 Created`:

```json
{
  "message": "purchase order created",
  "data": {
    "id": "e4f1a7b2-6a1e-4c3e-9d0e-5f1d2a7c9e11",
    "supplier_id": "b1a2c3d4-6a1e-4c3e-9d0e-5f1d2a7c9e11",
    "status": "draft",
    "order_date": "2026-09-20",
    "expected_date": "2026-09-30",
    "created_by": "7d2f4a10-3b5c-4e8a-a1f6-9c0b2d4e6f88",
    "notes": "Quarterly restock",
    "created_at": "2026-09-20T08:15:30Z",
    "updated_at": "2026-09-20T08:15:30Z",
    "items": [
      {
        "id": "9a3c5e70-6a1e-4c3e-9d0e-5f1d2a7c9e11",
        "medicine_id": "0b8f3c62-6a1e-4c3e-9d0e-5f1d2a7c9e11",
        "quantity_ordered": 200,
        "created_at": "2026-09-20T08:15:30Z"
      },
      { "...": "one entry per item" }
    ],
    "receipts": []
  }
}
```

`expected_date` and `notes` are `null` when not set.

### `GET /purchase-orders?limit=20&offset=0`

```json
{
  "data": [ { "...": "the PO header only — no items or receipts keys" } ],
  "total": 12,
  "limit": 20,
  "offset": 0
}
```

Newest first. Deleted POs are never listed. An offset past the end returns
`200` with `"data": []` and the real `total`.

### `GET /purchase-orders/{id}`

```json
{
  "data": {
    "...": "the header fields, as in create",
    "status": "received",
    "items": [ { "...": "as in create" } ],
    "receipts": [
      {
        "id": "c2d8f4a6-6a1e-4c3e-9d0e-5f1d2a7c9e11",
        "purchase_order_id": "e4f1a7b2-6a1e-4c3e-9d0e-5f1d2a7c9e11",
        "received_by": "7d2f4a10-3b5c-4e8a-a1f6-9c0b2d4e6f88",
        "received_at": "2026-09-28T14:02:11Z",
        "notes": "Delivered by courier",
        "created_at": "2026-09-28T14:02:11Z",
        "items": [
          {
            "id": "f6a0b3d1-6a1e-4c3e-9d0e-5f1d2a7c9e11",
            "purchase_order_item_id": "9a3c5e70-6a1e-4c3e-9d0e-5f1d2a7c9e11",
            "quantity_received": 200,
            "quantity_damaged": 0,
            "quantity_returned": 0,
            "notes": null,
            "created_at": "2026-09-28T14:02:11Z"
          }
        ]
      }
    ]
  }
}
```

### `POST /purchase-orders/{id}/receive`

Request (the whole body is optional):

```json
{ "notes": "Delivered by courier" }
```

`201 Created` — the new receipt:

```json
{
  "message": "purchase order received",
  "data": {
    "id": "c2d8f4a6-6a1e-4c3e-9d0e-5f1d2a7c9e11",
    "purchase_order_id": "e4f1a7b2-6a1e-4c3e-9d0e-5f1d2a7c9e11",
    "received_by": "7d2f4a10-3b5c-4e8a-a1f6-9c0b2d4e6f88",
    "received_at": "2026-09-28T14:02:11Z",
    "notes": "Delivered by courier",
    "created_at": "2026-09-28T14:02:11Z",
    "items": [ { "...": "one per PO item, quantity_received = quantity_ordered" } ]
  }
}
```

Every medicine on the PO has its `quantity` increased by the ordered amount
(`trg_purchase_order_receipt_items_sync_quantity`), and the PO's status
becomes `received`. Both happen in one transaction with the receipt.

### Audit trail

Both writes are recorded in `audit_logs` by the `00019` triggers, with
`actor_id` set to the caller: create writes a `created` row for the PO and one
per item; receive writes `created` rows for the receipt and each receipt item
and an `updated` row (before/after) for the PO's status change. An `updated`
row for a PO also appears on any later change to it.

### Errors

| Status | When                                                              | Body                                                                     |
|--------|---------------------------------------------------------------------|-----------------------------------------------------------------------------|
| 400    | Bad query param, bad path id, invalid body or field                  | `{"error": "<message>"}`                                                     |
| 401    | Missing, malformed, expired or non-access token                      | `{"error": "unauthorized"}`                                                  |
| 404    | Create: `supplier_id` isn't an active supplier                       | `{"error": "supplier not found"}`                                            |
| 404    | Create: an item's `medicine_id` isn't an active medicine             | `{"error": "medicine not found"}`                                            |
| 404    | Get / receive: no active PO with that ID                             | `{"error": "purchase order not found"}`                                      |
| 409    | Receive: the status is not `draft` or `ordered`                      | `{"error": "purchase order cannot be received in its current status"}`       |
| 500    | Anything unexpected                                                   | `{"error": "internal server error"}`                                         |

---

## Open Questions

- **The status lifecycle is mostly unimplemented.** Create always writes
  `draft` (the schema default), and receive accepts `draft` or `ordered`, so
  `ordered` is unreachable today and nothing writes `cancelled` or
  `partially_received`. I let receive accept `draft` so the one flow this pass
  needs works without a "place the order" step. If you want `draft → ordered`
  first, that is a small extra endpoint (or an optional `status` on create)
  and receive would then require `ordered`.
- **A PO can't be edited, cancelled or deleted, and that has a side effect.**
  Medicine and supplier deletes are refused while *any* non-deleted PO
  references them, whatever its status
  (@context/features/07-medicine-RUD.spec.md,
  @context/features/09-suppliers.spec.md). With no way to delete a PO, a
  medicine or supplier that has ever been on one can no longer be deleted.
  Options: a PO soft delete (`DELETE /purchase-orders/{id}`, which the
  soft-delete column already supports), or narrowing those two checks to
  open statuses (`draft`, `ordered`, `partially_received`). Worth deciding
  before this ships to real users.
- **Receiving adds stock to the *referenced* `medicines` row.** That is what
  the `00018` trigger does, but @context/database-schema.md already notes that
  a `medicines` row is one batch with one `expiration_date`. A restock that
  arrives with a different batch or expiry would be added into the old row's
  count and inherit its expiration date, so the LED status for that stock
  could be wrong. Not solved here; it needs a product/batch concept or
  receiving that creates a new `medicines` row.
- **No filters on the list** — only `limit`/`offset`, like `GET
  /inventory-entries`. Filtering by `supplier_id` and `status` is a likely
  near-term ask and can be added as optional query params without changing
  this shape.
- **Repeated medicines and the 100-item cap are my choices, not the
  schema's.** `purchase_order_items` has no unique `(purchase_order_id,
  medicine_id)`, so the database would allow the same medicine twice. I
  reject it because receive-full would otherwise add two lines for one
  medicine and the response is harder to read; the cap keeps one request (and
  its audit rows — one per item, plus one per receipt item on receive)
  bounded. Both are a one-line change if you disagree.
- **`404` for a missing `supplier_id` / `medicine_id` in the body.** A `400`
  or `422` is arguably more precise, but this matches
  `POST /inventory-entries`, which returns `404 medicine not found`.
- **Receive returns `201` with the receipt**, since it creates one. If you'd
  rather have `200` with the updated PO (as `GET /purchase-orders/{id}`
  returns it), that is a handler-only change.
- **Header edits aren't included** (`notes`, `expected_date`, `supplier_id`
  while `draft`). The `updated_at` trigger and audit `updated` rows are
  already in place for when they are.

---

## References

- @context/go-standards.md
- @context/database-schema.md
- @context/features/02-audit-purchase-orders.spec.md
- @context/features/07-medicine-RUD.spec.md
- @context/features/08-inventory.spec.md
- @context/features/09-suppliers.spec.md
- `apps/migration/transactions/00012_create_purchase_orders_table.sql`
- `apps/migration/transactions/00013_create_purchase_order_items_table.sql`
- `apps/migration/transactions/00014_create_purchase_order_receipts_table.sql`
- `apps/migration/transactions/00015_create_purchase_order_receipt_items_table.sql`
- `apps/migration/transactions/00018_create_quantity_sync_triggers.sql`
- `apps/migration/transactions/00019_create_audit_log_triggers.sql`
- `apps/migration/transactions/00020_create_updated_at_triggers.sql`
- `apps/api/internal/repository/inventory/repository.go` — the `Transaction`
  that hands `fn` a second repository bound to the same transaction
- `apps/api/internal/repository/medicine/repository.go` and
  `apps/api/internal/repository/supplier/repository.go` — `LockByID`
- `apps/api/pkg/apperror/apperror.go`
- `apps/api/pkg/response/response.go`
- `apps/api/internal/handler/swagger/openapi.json`
