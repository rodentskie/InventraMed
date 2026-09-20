# Inventory Entries (Create + Read)

## Overview

This will be done in `apps/api`. Add endpoints to record and browse manual stock
adjustments (`inventory_entries`), the same layered shape as the medicine
endpoints.

| Endpoint                       | Purpose                                   | Success |
|---------------------------------|--------------------------------------------|---------|
| `POST /inventory-entries`      | Record a stock adjustment                  | `201`   |
| `GET /inventory-entries`       | List entries, paginated                    | `200`   |
| `GET /inventory-entries/{id}`  | Get a single entry by ID                   | `200`   |

No update, no delete. Per @context/database-schema.md, `inventory_entries` is
**not soft-deletable** — an entry is a fact that happened; correcting a mistake
is a new offsetting entry, not an edit. This mirrors `purchase_order_items`
and `purchase_order_receipts`, which also have no update/delete.

All three require the auth middleware, like the medicine endpoints. RBAC is
still out of scope.

New packages `internal/handler/inventory`, `internal/service/inventory`,
`internal/repository/inventory`, plus one addition to
`internal/repository/medicine` (a locked quantity read, needed for the
insufficient-stock guard below). No new migration — the table
(`00016_create_inventory_entries_table.sql`) and its quantity-sync trigger
(`00018_create_quantity_sync_triggers.sql`) already exist.

---

## Requirements

Layered per @context/go-standards.md — Handler → Service → Repository, each
depending on the layer below via interfaces.

### 1. Domain (`internal/domain`)

New file `inventory_entry.go`:

```go
type InventoryEntry struct {
    ID         string
    MedicineID string
    Direction  string // "addition" | "subtraction"
    Quantity   int
    Reason     string
    CountedBy  string
    Notes      *string
    CreatedAt  time.Time
}
```

No `UpdatedAt` — the table has none.

### 2. Medicine repository addition

Add one method to the existing `medicine.Repository` interface (additive —
`Update`/`Delete`/etc. are unchanged):

- `LockQuantityByID(ctx, id) (quantity int, found bool, err error)` — like
  `LockByID`, but `SELECT quantity ... FOR UPDATE` on an active (not
  soft-deleted) medicine, and returns the quantity. Locks the row until the
  caller's transaction ends, so a concurrent subtraction or receipt on the
  same medicine has to wait, keeping the guard below race-free.

### 3. Repository (`internal/repository/inventory`)

- GORM, private `record` mapped to `inventory_entries` via `TableName()`. No
  `DeletedAt`, no `autoUpdateTime` field — the table has neither
- `Create(ctx, *domain.InventoryEntry) (*domain.InventoryEntry, error)` — `id`
  and `created_at` come from the database defaults
- `FindByID(ctx, id) (*domain.InventoryEntry, error)` — returns
  `apperror.ErrNotFound` when there is none
- `List(ctx, filter ListFilter) ([]*domain.InventoryEntry, int64, error)` —
  one page plus the total. `ListFilter` has `Limit` and `Offset` only (see
  Open Questions). Order by `created_at DESC, id DESC` for stable pages
- `Transaction(ctx, fn func(tx Repository, medicines medicine.Repository) error) error`
  — opens one GORM transaction and hands `fn` both an `inventory.Repository`
  and a `medicine.Repository` bound to it (`medicine.NewRepository(tx)`), so
  the quantity guard (medicines table) and the insert (inventory_entries
  table) are atomic
- Maps to `domain.InventoryEntry` at the package boundary — `record` is never
  exposed

### 4. Service (`internal/service/inventory`)

```go
type Service interface {
    Create(ctx context.Context, input CreateInput) (*domain.InventoryEntry, error)
    List(ctx context.Context, filter ListFilter) (*Page, error)
    GetByID(ctx context.Context, id string) (*domain.InventoryEntry, error)
}
```

`CreateInput` carries the validated fields plus `CountedBy` (the caller's user
ID). `Create`, in one transaction:

1. `medicines.LockQuantityByID(ctx, input.MedicineID)` → not found →
   `apperror.ErrNotFound`
2. If `Direction == "subtraction"` and the locked quantity is less than
   `input.Quantity` → `apperror.ErrInsufficientQuantity` (new, wraps
   `apperror.ErrConflict`, added to `pkg/apperror` next to the medicine ones);
   nothing is inserted
3. Insert the entry. The existing `trg_inventory_entries_sync_quantity`
   trigger (`00018`) updates `medicines.quantity` automatically — the service
   does not write it directly

`List` and `GetByID` call the repository directly. `apperror.ErrNotFound`
passes through unchanged; other repository errors are logged and wrapped with
`fmt.Errorf("<operation> inventory entry: %w", err)`.

### 5. Handler (`internal/handler/inventory`)

Read the caller from `middleware.AccountFromContext` (fail closed with `401
unauthorized` if missing), decode/validate, map errors, respond with the
`response` helpers only. `counted_by` is never read from the body.

**`POST /inventory-entries` — body and validation**

Trim all string fields first. First failing rule wins, `400`:

| Field         | Rule                                                        | Message                                                    |
|---------------|---------------------------------------------------------------|-------------------------------------------------------------|
| body          | Valid JSON, max 1 MiB                                        | `invalid request body`                                      |
| `medicine_id` | Required, valid UUID                                          | `medicine_id is required` / `invalid medicine_id`            |
| `direction`   | Required, exactly `addition` or `subtraction`                 | `direction is required` / `direction must be addition or subtraction` |
| `quantity`    | Required, integer, `> 0`. Use `*int` so a missing value isn't mistaken for `0` | `quantity is required` / `quantity must be greater than zero` |
| `reason`      | Required, non-empty, max 100 chars. Free text — see Open Questions | `reason is required` / `reason is too long`                 |
| `notes`       | Optional, max 500 chars. Empty or missing stored as `NULL`    | `notes is too long`                                          |

**`GET /inventory-entries` — query parameters**

Same mechanism as `GET /medicines` (@context/features/07-medicine-RUD.spec.md):

| Param    | Rule                   | Default | Message on failure                                 |
|----------|-------------------------|---------|-----------------------------------------------------|
| `limit`  | Whole number, `1`–`100` | `20`    | `limit must be a whole number between 1 and 100`     |
| `offset` | Whole number, `>= 0`    | `0`     | `offset must be a whole number of zero or greater`   |

No other filters for this pass (see Open Questions).

**`GET /inventory-entries/{id}`**

`id` read with `r.PathValue("id")`, must be a valid UUID → `400 invalid
inventory entry id` otherwise.

**Error mapping**

| Error                          | Status | Message                                    | Used by        |
|---------------------------------|--------|---------------------------------------------|----------------|
| no caller in context            | `401`  | `unauthorized`                              | all            |
| invalid body / query / path id  | `400`  | the validation message                      | create, list, get |
| medicine not found              | `404`  | `medicine not found`                        | create         |
| entry not found                 | `404`  | `inventory entry not found`                 | get            |
| `ErrInsufficientQuantity`       | `409`  | `insufficient quantity for subtraction`     | create         |
| anything else                   | `500`  | `internal server error` (log the real error, never return it) | all |

### 6. Wiring

In `apps/api/cmd/api/main.go`, register through the prefix wrapper, wrapped
with the existing `auth` middleware:

```go
r.Handle("POST /inventory-entries", auth(inventoryHandler.Create))
r.Handle("GET /inventory-entries", auth(inventoryHandler.List))
r.Handle("GET /inventory-entries/{id}", auth(inventoryHandler.GetByID))
```

→ served at `/api/inventory-entries` and `/api/inventory-entries/{id}` with
the default prefix.

### 7. Swagger

Update `apps/api/internal/handler/swagger/openapi.json` once the endpoints
work:

- Add an `inventory-entries` tag
- `POST /inventory-entries` (`operationId: createInventoryEntry`): request
  body `CreateInventoryEntryRequest`, responses `201`, `400`, `401`, `404`,
  `409`, `500`
- `GET /inventory-entries` (`operationId: listInventoryEntries`): `limit`,
  `offset` query params, responses `200`, `400`, `401`, `500`
- `GET /inventory-entries/{id}` (`operationId: getInventoryEntry`): `id` path
  param (`format: uuid`), responses `200`, `400`, `401`, `404`, `500`
- Every operation uses `security: [{"bearerAuth": []}]`
- Add schemas `CreateInventoryEntryRequest`, `InventoryEntry`,
  `InventoryEntryResponse`, `InventoryEntryListResponse`; reuse
  `ErrorResponse`
- Must remain valid JSON; `internal/handler/swagger/handler_test.go` must
  still pass

### 8. Tests

- **Medicine repository**: `LockQuantityByID` — found (returns quantity),
  not found, soft-deleted counts as not found
- **Inventory service** (mocked repositories): create success (addition and
  subtraction), medicine not found short-circuits before any insert,
  subtraction exactly equal to stock succeeds, subtraction greater than stock
  → `ErrInsufficientQuantity` and no insert, `CountedBy` reaches the
  repository, `List` passes the filter through and returns the page/total,
  `GetByID` found/not-found/error, each repository error path
- **Inventory handler** (mocked service, through the real middleware):
  `201` body shape, every `400` case in the validation table, `404` (medicine
  not found), `409`, `500`, `counted_by` in the body is ignored; `List`
  defaults, each invalid `limit`/`offset`, `200` body shape, empty result is
  `"data": []`; `GetByID` `200`, invalid id, `404`, `500`; all three → no
  caller in context is `401`
- Skip unit tests on thin GORM passthroughs (`Create`, `FindByID`, `List`)
  per @context/go-standards.md
- Coverage ≥ 90%

---

## Endpoints

All require `Authorization: Bearer <access_token>`, served under `API_PREFIX`
(default `/api`).

### `POST /inventory-entries`

Request:

```json
{
  "medicine_id": "0b8f3c62-6a1e-4c3e-9d0e-5f1d2a7c9e11",
  "direction": "subtraction",
  "quantity": 5,
  "reason": "damaged",
  "notes": "water damage during storage"
}
```

`201 Created`:

```json
{
  "message": "inventory entry recorded",
  "data": {
    "id": "3a1e2c4f-...",
    "medicine_id": "0b8f3c62-6a1e-4c3e-9d0e-5f1d2a7c9e11",
    "direction": "subtraction",
    "quantity": 5,
    "reason": "damaged",
    "counted_by": "7d2f4a10-3b5c-4e8a-a1f6-9c0b2d4e6f88",
    "notes": "water damage during storage",
    "created_at": "2026-09-20T08:15:30Z"
  }
}
```

The insert triggers `medicines.quantity` to update automatically
(`trg_inventory_entries_sync_quantity`).

### `GET /inventory-entries?limit=20&offset=0`

```json
{
  "data": [ { "...": "same shape as create's data" } ],
  "total": 42,
  "limit": 20,
  "offset": 0
}
```

Newest first. An offset past the end returns `200` with `"data": []` and the
real `total`.

### `GET /inventory-entries/{id}`

```json
{ "data": { "...": "same shape as create's data" } }
```

### Errors

| Status | When                                                        | Body                                                   |
|--------|--------------------------------------------------------------|---------------------------------------------------------|
| 400    | Bad query param, bad path id, invalid body or field           | `{"error": "<message>"}`                                |
| 401    | Missing, malformed, expired or non-access token               | `{"error": "unauthorized"}`                             |
| 404    | Create: `medicine_id` doesn't reference an active medicine    | `{"error": "medicine not found"}`                       |
| 404    | Get: no entry with that ID                                    | `{"error": "inventory entry not found"}`                |
| 409    | Create: subtraction would take the medicine below zero        | `{"error": "insufficient quantity for subtraction"}`    |
| 500    | Anything unexpected                                            | `{"error": "internal server error"}`                    |

---

## Open Questions

- **No `medicine_id` filter on list** — you asked for "same setup, limit and
  offset, as get all medicines," so this pass only paginates. Filtering
  entries down to one medicine's history is a likely near-term ask (mirrors
  `name`/`barcode` on `GET /medicines`) — easy to add as an optional query
  param later without touching this shape.
- **`reason` is free text, not a closed enum** — `database-schema.md` lists
  `count_adjustment`, `returned`, `damaged`, `lost`, `expired_removal` as
  *examples* (`e.g.`), and the DB column has no `CHECK`. This spec validates
  only presence and length, matching the DB. If you want it locked to a fixed
  set, that's a `CHECK` constraint plus a matching handler-side allow-list.
- **Insufficient-quantity guard (`409`) is new — not enforced by the schema.**
  Neither `medicines.quantity` nor `inventory_entries` has a DB constraint
  stopping stock from going negative; the sync trigger just applies the delta.
  I added the guard because an unguarded subtraction producing negative stock
  seemed like a real bug, not a hypothetical, but it's an API-level decision.
  If you'd rather allow it (e.g. to record a known discrepancy before a
  recount corrects it), drop step 2 of `Create` and this becomes non-blocking.

---

## References

- @context/go-standards.md
- @context/database-schema.md
- @context/features/06-medicine.spec.md
- @context/features/07-medicine-RUD.spec.md
- `apps/migration/transactions/00016_create_inventory_entries_table.sql`
- `apps/migration/transactions/00018_create_quantity_sync_triggers.sql`
- `apps/api/internal/repository/medicine/repository.go` — `LockByID`,
  `Transaction`, `record`/`toDomain` conventions to mirror
- `apps/api/pkg/apperror/apperror.go`
- `apps/api/pkg/response/response.go`
- `apps/api/internal/handler/swagger/openapi.json`
