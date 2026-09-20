# Suppliers CRUD

## Overview

Add full CRUD for `suppliers` to `apps/api`, following the same layered shape
as the medicine endpoints.

| Endpoint                | Purpose                                             | Success |
|--------------------------|------------------------------------------------------|---------|
| `POST /suppliers`       | Register a new supplier                             | `201`   |
| `GET /suppliers`        | List suppliers, paginated, optionally filtered by name | `200`   |
| `GET /suppliers/{id}`   | Get a single supplier                                | `200`   |
| `PUT /suppliers/{id}`   | Update a supplier's details                          | `204`   |
| `DELETE /suppliers/{id}`| Soft delete a supplier, unless a purchase order uses it | `204`   |

All five require the auth middleware, like the medicine endpoints. RBAC is
still out of scope.

Unlike `medicines`, `suppliers` has **no uniqueness rule** — the table has no
`UNIQUE` constraint beyond the primary key, and `database-schema.md` doesn't
describe one. So create and update have no duplicate checks, and no new
migration is needed: `suppliers` (`00011`) is already soft-deletable and
already has its `updated_at` trigger (`00020`, unlike `medicines`, which
needed a dedicated migration for it in `07-medicine-RUD.spec.md`).

The one business rule this spec does add is the same shape as medicine's
delete: a supplier referenced by a non-deleted purchase order
(`purchase_orders.supplier_id`) cannot be deleted.

New packages `internal/{handler,service,repository}/supplier`. No new
migration.

---

## Requirements

Layered per @context/go-standards.md — Handler → Service → Repository, each
depending on the layer below via interfaces.

### 1. Domain (`internal/domain`)

New file `supplier.go`:

```go
type Supplier struct {
    ID          string
    Name        string
    ContactName *string
    Email       *string
    Phone       *string
    Address     *string
    CreatedAt   time.Time
    UpdatedAt   time.Time
}
```

### 2. Repository (`internal/repository/supplier`)

GORM, private `record` mapped to `suppliers` via `TableName()`, following the
soft-delete convention: `DeletedAt gorm.DeletedAt`, no `gorm.Model`.

```go
type Repository interface {
    Create(ctx context.Context, supplier *domain.Supplier) (*domain.Supplier, error)
    // List returns one page of active suppliers, newest first, and the total
    // number matching the filter.
    List(ctx context.Context, filter ListFilter) ([]*domain.Supplier, int64, error)
    // FindByID returns the active supplier with the ID, or apperror.ErrNotFound.
    FindByID(ctx context.Context, id string) (*domain.Supplier, error)
    // Update writes name, contact_name, email, phone and address. Returns
    // apperror.ErrNotFound when no active supplier has the ID.
    Update(ctx context.Context, supplier *domain.Supplier) error
    // LockByID is like medicine.Repository.LockByID: reports whether an
    // active supplier has the ID, and locks the row until the transaction ends.
    LockByID(ctx context.Context, id string) (bool, error)
    // ExistsInPurchaseOrder reports whether a purchase order that is not
    // soft-deleted references the supplier, whatever the order's status.
    ExistsInPurchaseOrder(ctx context.Context, id string) (bool, error)
    // Delete soft-deletes the supplier. Returns apperror.ErrNotFound when no
    // active supplier has the ID.
    Delete(ctx context.Context, id string) error
    // Transaction runs fn with a Repository bound to a single transaction.
    Transaction(ctx context.Context, fn func(tx Repository) error) error
}
```

- `ListFilter` has `Limit`, `Offset` and `Name`. `Name`, when not empty, is a
  **case-insensitive contains** match (`ILIKE '%term%'` with `%`, `_`, `\`
  escaped — reuse the escaping approach in `repository/medicine`)
- `ExistsInPurchaseOrder` is a direct `WHERE supplier_id = ? AND deleted_at IS
  NULL` on `purchase_orders` — no join needed, unlike medicine's version
  (which joins through `purchase_order_items`), because `purchase_orders`
  references `suppliers` directly
- `Create`/`Update` map `nil` pointer fields to `NULL`; a struct `Updates`
  would skip zero values, so `Update` needs `Select(...)` or a map, same as
  `medicine.Repository.Update`
- Maps to `domain.Supplier` at the package boundary — `record` is never
  exposed

### 3. Service (`internal/service/supplier`)

```go
type CreateInput struct {
    Name        string
    ContactName *string
    Email       *string
    Phone       *string
    Address     *string
}

type UpdateInput = CreateInput // same fields, aliased for clarity at call sites

type Service interface {
    Create(ctx context.Context, input CreateInput) (*domain.Supplier, error)
    List(ctx context.Context, filter ListFilter) (*Page, error)
    GetByID(ctx context.Context, id string) (*domain.Supplier, error)
    Update(ctx context.Context, id string, input UpdateInput) error
    Delete(ctx context.Context, id string) error
}
```

- `Create` calls `repo.Create` directly — no transaction needed, since there
  are no pre-checks
- `List` and `GetByID` call the repository directly
- `Update` calls `repo.Update` directly; its own `apperror.ErrNotFound` (from
  `RowsAffected == 0`) passes straight through — no separate existence check
  needed, unlike medicine's `Update`, which pre-checks so an unknown ID can
  never be masked by a duplicate error. There is no duplicate error here
- `Delete`, in one transaction:
  1. `LockByID` → not found → `apperror.ErrNotFound`
  2. `ExistsInPurchaseOrder` → true → `apperror.ErrSupplierInPurchaseOrder`
     (new, in `pkg/apperror`, wraps `apperror.ErrConflict` like
     `ErrMedicineInPurchaseOrder`); nothing is deleted
  3. `Delete`
- `apperror.ErrNotFound` and `apperror.ErrConflict` pass through unchanged;
  other repository errors are logged and wrapped with
  `fmt.Errorf("<operation> supplier: %w", err)`

### 4. Handler (`internal/handler/supplier`)

Read the caller from `middleware.AccountFromContext` (fail closed with `401
unauthorized` if missing — `suppliers` has no `created_by`, so the caller is
only used to enforce auth, not stored). Decode/validate, map errors, respond
with the `response` helpers only.

**`POST /suppliers` and `PUT /suppliers/{id}` — body and validation**

Both use the same request shape and validation (`PUT` is a full replacement).
Trim all string fields first; empty optional fields are stored as `NULL`.

| Field          | Rule                                                    | Message                                             |
|-----------------|-----------------------------------------------------------|-------------------------------------------------------|
| body            | Valid JSON, max 1 MiB                                     | `invalid request body`                                 |
| `name`          | Required, non-empty, max 255 chars                        | `name is required` / `name is too long`                |
| `contact_name`  | Optional, max 255 chars                                    | `contact_name is too long`                              |
| `email`         | Optional, max 255 chars, valid address if given (`net/mail.ParseAddress`) | `email is too long` / `email must be a valid email address` |
| `phone`         | Optional, max 32 chars                                     | `phone is too long`                                     |
| `address`       | Optional, max 500 chars                                    | `address is too long`                                   |

**`GET /suppliers` — query parameters**

Same mechanism as `GET /medicines`:

| Param    | Rule                    | Default | Message on failure                                  |
|----------|--------------------------|---------|-------------------------------------------------------|
| `limit`  | Whole number, `1`–`100`  | `20`    | `limit must be a whole number between 1 and 100`       |
| `offset` | Whole number, `>= 0`     | `0`     | `offset must be a whole number of zero or greater`     |
| `name`   | Optional, trimmed, max 255 chars. Empty is ignored | none | `name is too long` |

**Path values**

- `id` (`GET /suppliers/{id}`, `PUT`, `DELETE`) — read with `r.PathValue("id")`,
  must be a valid UUID; otherwise `400 invalid supplier id`

**Error mapping**

| Error                              | Status | Message                                                       | Used by            |
|--------------------------------------|--------|-------------------------------------------------------------------|---------------------|
| no caller in context                | `401`  | `unauthorized`                                                     | all                 |
| invalid query, path id, or body/field | `400`  | the validation message                                             | all                 |
| `apperror.ErrNotFound`              | `404`  | `supplier not found`                                                | get, update, delete |
| `ErrSupplierInPurchaseOrder`        | `409`  | `supplier is used in a purchase order and cannot be deleted`       | delete              |
| anything else                       | `500`  | `internal server error` (log the real error, never return it)     | all                 |

### 5. Wiring

In `apps/api/cmd/api/main.go`, register through the prefix wrapper, each
wrapped with the existing `auth` middleware:

```go
r.Handle("POST /suppliers", auth(supplierHandler.Create))
r.Handle("GET /suppliers", auth(supplierHandler.List))
r.Handle("GET /suppliers/{id}", auth(supplierHandler.GetByID))
r.Handle("PUT /suppliers/{id}", auth(supplierHandler.Update))
r.Handle("DELETE /suppliers/{id}", auth(supplierHandler.Delete))
```

### 6. Swagger

Update `apps/api/internal/handler/swagger/openapi.json` once the endpoints
work:

- Add a `suppliers` tag
- `POST /suppliers` (`operationId: createSupplier`): request body
  `CreateSupplierRequest`, responses `201`, `400`, `401`, `500`
- `GET /suppliers` (`operationId: listSuppliers`): `limit`, `offset`, `name`
  query params, responses `200`, `400`, `401`, `500`
- `GET /suppliers/{id}` (`operationId: getSupplier`): `id` path param
  (`format: uuid`), responses `200`, `400`, `401`, `404`, `500`
- `PUT /suppliers/{id}` (`operationId: updateSupplier`): request body
  `UpdateSupplierRequest`, responses `204`, `400`, `401`, `404`, `500`
- `DELETE /suppliers/{id}` (`operationId: deleteSupplier`): responses `204`,
  `400`, `401`, `404`, `409`, `500`
- Every operation uses `security: [{"bearerAuth": []}]`; `204` responses have
  no `content`
- Add schemas `CreateSupplierRequest`, `UpdateSupplierRequest` (same shape),
  `Supplier`, `SupplierResponse`, `SupplierListResponse`; reuse `ErrorResponse`
- Must remain valid JSON; `internal/handler/swagger/handler_test.go` must
  still pass

### 7. Tests

- **Service** (mocked repository): `Create` success and repository error;
  `List` passes the filter through; `GetByID` found/not-found/error;
  `Update` success, not-found passes through, repository error; `Delete`
  success, not found → no purchase-order check, in a purchase order →
  `ErrSupplierInPurchaseOrder` and `Delete` not called, each repository error
  path (lock, purchase-order check, delete)
- **Handler** (mocked service, through the real middleware): every `400` case
  in the validation table for both `Create` and `Update`, invalid path id for
  `get`/`update`/`delete`, `201`/`200`/`204` body shapes, `404`, `409`, `500`,
  no caller in context → `401` for all five, `List` defaults and custom
  `limit`/`offset`/`name`, each invalid `limit`/`offset`, empty result is
  `"data": []`
- Skip unit tests on thin GORM passthroughs (`Create`, `FindByID`, `List`,
  `Update`'s happy path) per @context/go-standards.md; a GORM DryRun SQL test
  for `Delete`'s lock (`FOR UPDATE`) and `ExistsInPurchaseOrder`'s query shape,
  mirroring `repository/medicine/repository_sql_test.go`
- Coverage ≥ 90% on service and handler

---

## Endpoints

All require `Authorization: Bearer <access_token>`, served under `API_PREFIX`
(default `/api`).

### `POST /suppliers`

Request:

```json
{
  "name": "Acme Pharma Distribution",
  "contact_name": "Jane Cruz",
  "email": "jane@acmepharma.example",
  "phone": "+63 917 123 4567",
  "address": "123 Industrial Ave, Quezon City"
}
```

`201 Created`:

```json
{
  "message": "supplier created",
  "data": {
    "id": "b1a2c3d4-6a1e-4c3e-9d0e-5f1d2a7c9e11",
    "name": "Acme Pharma Distribution",
    "contact_name": "Jane Cruz",
    "email": "jane@acmepharma.example",
    "phone": "+63 917 123 4567",
    "address": "123 Industrial Ave, Quezon City",
    "created_at": "2026-09-20T08:15:30Z",
    "updated_at": "2026-09-20T08:15:30Z"
  }
}
```

### `GET /suppliers?limit=20&offset=0&name=acme`

```json
{
  "data": [ { "...": "same shape as create's data" } ],
  "total": 12,
  "limit": 20,
  "offset": 0
}
```

Newest first. `name` matches anywhere, ignoring case. Soft-deleted suppliers
are never listed. An offset past the end returns `200` with `"data": []` and
the real `total`.

### `GET /suppliers/{id}`

```json
{ "data": { "...": "same shape as create's data" } }
```

### `PUT /suppliers/{id}`

Request: same shape as create. `204 No Content` — empty body. Full
replacement; an omitted optional field clears it.

### `DELETE /suppliers/{id}`

`204 No Content` — empty body. Refused with `409` while any non-deleted
purchase order references the supplier.

### Errors

| Status | When                                                        | Body                                                                    |
|--------|----------------------------------------------------------------|----------------------------------------------------------------------------|
| 400    | Bad query param, bad path id, invalid body or field             | `{"error": "<message>"}`                                                    |
| 401    | Missing, malformed, expired or non-access token                 | `{"error": "unauthorized"}`                                                 |
| 404    | No active supplier with that id                                 | `{"error": "supplier not found"}`                                           |
| 409    | Delete: a purchase order uses the supplier                      | `{"error": "supplier is used in a purchase order and cannot be deleted"}`   |
| 500    | Anything unexpected                                              | `{"error": "internal server error"}`                                        |

---

## Open Questions

- **`GET /suppliers` filters by `name` only** — mirrors the medicine list's
  `name` filter. If you'd rather search `contact_name`/`email`/`phone` too
  (e.g. a single `q` param matching any field), that's a small change to the
  same `ListFilter`/`matching` scope.
- **No uniqueness rule** — two suppliers can have the same name, email or
  phone; `database-schema.md` doesn't call for one and the migration has no
  `UNIQUE` constraint. If duplicates should be prevented (e.g. by email), that
  needs a new unique index migration plus a conflict check, the same pattern
  as `uq_medicines_name_batch`.
- **What blocks a delete** — same choice already made for medicines: *any*
  non-deleted purchase order referencing the supplier blocks the delete,
  whatever its status (`draft` through `cancelled`/`received`). If only open
  orders should block it, the check needs a status filter.
- **`UpdateInput = CreateInput`** — they happen to be identical today (no
  field like medicine's `quantity` is excluded from update). If update ever
  needs to diverge, split them into separate structs.

---

## References

- @context/go-standards.md
- @context/database-schema.md
- @context/features/06-medicine.spec.md
- @context/features/07-medicine-RUD.spec.md
- `apps/migration/transactions/00011_create_suppliers_table.sql`
- `apps/migration/transactions/00012_create_purchase_orders_table.sql`
- `apps/migration/transactions/00020_create_updated_at_triggers.sql`
- `apps/api/internal/repository/medicine/repository.go` — `record`,
  `escapeLike`, `ExistsInPurchaseOrder`, `LockByID`, `Transaction`,
  `Update`'s `Select`-based write conventions to mirror
- `apps/api/pkg/apperror/apperror.go`
- `apps/api/pkg/response/response.go`
- `apps/api/internal/handler/swagger/openapi.json`
