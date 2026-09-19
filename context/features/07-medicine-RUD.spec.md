# Read, Update and Delete Medicine

## Overview

Add the remaining medicine endpoints to `apps/api`, on top of the create endpoint from @context/features/06-medicine.spec.md:

| Endpoint                             | Purpose                                                        | Success |
|--------------------------------------|----------------------------------------------------------------|---------|
| `GET /medicines`                     | List medicines, paginated, optionally filtered by name/barcode | `200`   |
| `GET /medicines/barcode/{barcode}`   | Get a single medicine by its exact barcode                     | `200`   |
| `PUT /medicines/{id}`                | Update a medicine's details (not its quantity)                 | `204`   |
| `DELETE /medicines/{id}`             | Soft delete a medicine, unless a purchase order uses it        | `204`   |

All four require the auth middleware, like create. Role/permission (RBAC) checks are still out of scope.

Supporting work, in order:

1. A migration that keeps `medicines.updated_at` current on update
2. A `response.NoContent` helper and a new `apperror` for the delete rule
3. The four endpoints

This depends on the create-medicine work (PR #6) being merged: it reuses the medicine `record`, the `apperror` conflict errors, the middleware and the constraint-name mapping.

This will be done in `apps/api`, plus one new file under `apps/migration/transactions/`.

---

## Requirements

Layered per @context/go-standards.md — Handler → Service → Repository, each depending on the layer below via interfaces. The existing `medicine` handler, service and repository packages are extended; no new packages.

### 1. Migration

New file `apps/migration/transactions/00023_create_medicines_updated_at_trigger.sql`, following the conventions in @context/go-standards.md.

```sql
-- +goose Up
CREATE TRIGGER trg_medicines_set_updated_at
    BEFORE UPDATE ON medicines
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- +goose Down
DROP TRIGGER IF EXISTS trg_medicines_set_updated_at ON medicines;
```

- `set_updated_at()` already exists (`00020`) and is attached to `suppliers` and `purchase_orders` only. `medicines` was left out, and the `record` sets `autoUpdateTime:false`, so without this trigger `updated_at` would never change after insert
- The trigger fires on every update, including the quantity syncs from receipts and inventory entries and the soft delete. That is intended: the row did change
- Run it through the `migration` app CLI, up and down, to confirm both directions work. Never auto-run on startup

### 2. Helpers and errors

- `response.NoContent(w http.ResponseWriter)` in `pkg/response` writes status `204` with no body and no `Content-Type`. Handlers never write raw responses (@context/go-standards.md), and the existing helpers only write JSON
- `apperror.ErrMedicineInPurchaseOrder` in `pkg/apperror`, wrapping `ErrConflict` like the other medicine conflicts, so `errors.Is(err, apperror.ErrConflict)` still works

### 3. Repository (`internal/repository/medicine`)

Changes to existing methods, so update can exclude the medicine being edited:

- `ExistsByNameAndBatch(ctx, name, batchNumber *string, excludeID string)` and `ExistsByBarcode(ctx, barcode, excludeID string)` — when `excludeID` is not empty, add `AND id <> ?`. Create passes `""`. Matching rules are unchanged: name + batch is case-insensitive and ignores soft-deleted rows; barcode is exact and uses `Unscoped()`

New methods:

- `ExistsByID(ctx, id) (bool, error)` — active (not soft-deleted) medicines only
- `LockByID(ctx, id) (bool, error)` — same check, but as `SELECT … FOR UPDATE` so the row stays locked until the transaction ends. Used by delete: inserting a purchase order item takes a shared lock on the medicine row through its foreign key, so the lock makes a concurrent "add to a purchase order" wait for the delete, or the delete wait for it, and the PO check below can't go stale
- `ExistsInPurchaseOrder(ctx, id) (bool, error)` — true when any `purchase_order_items` row references the medicine and its `purchase_orders` row is not soft-deleted (`deleted_at IS NULL`). Every status counts, including `cancelled` and `received`. Join the two tables the same way the user repository joins `roles`; do not add a `record` for tables this package doesn't own
- `List(ctx, filter ListFilter) ([]*domain.Medicine, int64, error)` — one page plus the total number of medicines that match the filter. `ListFilter` has `Limit`, `Offset`, `Name` and `Barcode`
  - Order by `created_at DESC, id DESC` so pages are stable when rows share a timestamp
  - `Name` and `Barcode`, when not empty, are **case-insensitive contains** matches (`ILIKE '%term%'`). Escape `%`, `_` and `\` in the term so they match literally. Both given means both must match
  - Soft-deleted rows are excluded automatically by `gorm.DeletedAt`. Returns an empty non-nil slice when nothing matches
- `FindByBarcode(ctx, barcode) (*domain.Medicine, error)` — exact match, active medicines only. Returns `apperror.ErrNotFound` when there is none
- `Update(ctx, *domain.Medicine) error` — writes `name`, `barcode`, `batch_number` and `expiration_date` for the given ID. **It never writes `quantity`**, `created_by` or `created_at`. Use explicit columns (`Select`) or a map, **not** a struct `Updates`, because a struct skips zero values and a cleared `batch_number` (`nil`) must be written as `NULL`. Returns `apperror.ErrNotFound` when no active row was updated (`RowsAffected == 0`). Unique violations go through the existing constraint-name mapping (`medicines_barcode_key` → `apperror.ErrBarcodeExists`, `uq_medicines_name_batch` → `apperror.ErrNameBatchExists`); generalize its wrapping text, which currently says "create medicine"
- `Delete(ctx, id) error` — soft delete through GORM (`gorm.DeletedAt` turns `Delete` into an `UPDATE deleted_at`). Returns `apperror.ErrNotFound` when no active row was deleted, so deleting twice returns not found the second time

### 4. Service (`internal/service/medicine`)

Add to the `Service` interface:

- `List(ctx, filter ListFilter) (*Page, error)` — `Page` carries the medicines and the total. The service does not re-validate the filter; the handler owns that
- `GetByBarcode(ctx, barcode string) (*domain.Medicine, error)`
- `Update(ctx, id string, input UpdateInput) error` — `UpdateInput` is `Name`, `Barcode`, `BatchNumber` and `ExpirationDate`. It has no `Quantity`
- `Delete(ctx, id string) error`

`Update`, in one transaction:

1. `ExistsByID` → if not, `apperror.ErrNotFound`. This runs first so an unknown ID is a `404`, never a `409` caused by some other medicine
2. Check name + batch number, excluding this ID → `apperror.ErrNameBatchExists`
3. Check barcode, excluding this ID → `apperror.ErrBarcodeExists`
4. `Update`

Same rules as create: name + batch is checked before barcode, and conflict errors pass through unchanged. Because the checks exclude the medicine itself, saving a medicine with its own unchanged name, batch and barcode succeeds.

`Delete`, in one transaction:

1. `LockByID` → if not found, `apperror.ErrNotFound`
2. `ExistsInPurchaseOrder` → if true, `apperror.ErrMedicineInPurchaseOrder`; nothing is deleted
3. `Delete`

`List` and `GetByBarcode` call the repository directly. `apperror.ErrNotFound` and every conflict error pass through unchanged; other repository errors are logged and wrapped with `fmt.Errorf("<operation> medicine: %w", err)`.

### 5. Handler (`internal/handler/medicine`)

Add `List`, `GetByBarcode`, `Update` and `Delete`. Errors are mapped here only, and responses use the `response` helpers only.

**Path values**

- `id` (`Update`, `Delete`) — read with `r.PathValue("id")`. It must be a valid UUID; otherwise `400` with `invalid medicine id`. This also stops a bad value reaching Postgres, which would fail with a `500`
- `barcode` (`GetByBarcode`) — read with `r.PathValue("barcode")`, trimmed, with the same rules as create: required and at most 128 characters (`barcode is required` / `barcode is too long`). Clients must URL-encode barcodes that contain reserved characters

**`List` — query parameters**

| Param     | Rule                                                        | Default | Message on failure                                  |
|-----------|-------------------------------------------------------------|---------|-----------------------------------------------------|
| `limit`   | Whole number, `1`–`100`                                     | `20`    | `limit must be a whole number between 1 and 100`    |
| `offset`  | Whole number, `>= 0`                                        | `0`     | `offset must be a whole number of zero or greater`  |
| `name`    | Optional, trimmed, max 255 chars. Empty is ignored          | none    | `name is too long`                                  |
| `barcode` | Optional, trimmed, max 128 chars. Empty is ignored          | none    | `barcode is too long`                               |

- Missing or empty `limit` / `offset` use the default. Out-of-range or non-numeric values return `400`; they are not clamped
- Define the default and max as constants in the handler

**`Update` — body and validation**

The body has `name`, `barcode`, `batch_number` and `expiration_date`, and the validation rules are exactly the create rules for those four fields (see @context/features/06-medicine.spec.md), trimmed. It is a full replacement of those fields — an omitted `batch_number` clears it. Share the validation code with `Create`; `quantity` validation belongs to create only.

`quantity` cannot be updated: if the body contains it, it is ignored, like `created_by`, and is never validated or written. That means a body copied from a `GET` response (which includes `quantity`, `id` and the timestamps) can be sent back as is. Stock changes go through inventory entries and receipts.

**Error mapping**

| Error                              | Status | Message                                                            | Used by                     |
|------------------------------------|--------|--------------------------------------------------------------------|-----------------------------|
| no caller in context               | `401`  | `unauthorized`                                                     | all                         |
| invalid path ID / barcode          | `400`  | `invalid medicine id`, or the barcode validation message           | update, delete, by barcode  |
| invalid query or body              | `400`  | the validation message                                             | list, update                |
| `apperror.ErrNotFound`             | `404`  | `medicine not found`                                               | by barcode, update, delete  |
| name + batch conflict              | `409`  | `medicine with this name and batch number already exists`          | update                      |
| barcode conflict                   | `409`  | `medicine with this barcode already exists`                        | update                      |
| `ErrMedicineInPurchaseOrder`       | `409`  | `medicine is used in a purchase order and cannot be deleted`       | delete                      |
| anything else                      | `500`  | `internal server error` (log the real error, never return it)      | all                         |

### 6. Wiring

In `apps/api/cmd/api/main.go`, register through the prefix wrapper, each wrapped with the existing `auth` middleware:

```go
r.Handle("GET /medicines", auth(medicineHandler.List))
r.Handle("GET /medicines/barcode/{barcode}", auth(medicineHandler.GetByBarcode))
r.Handle("PUT /medicines/{id}", auth(medicineHandler.Update))
r.Handle("DELETE /medicines/{id}", auth(medicineHandler.Delete))
```

→ served at `/api/medicines`, `/api/medicines/barcode/{barcode}` and `/api/medicines/{id}` with the default prefix. The router wrapper only prefixes the path part, so `{...}` patterns need no router change, and the barcode route does not clash with `/{id}` (different method and segment count).

### 7. Swagger

Update `apps/api/internal/handler/swagger/openapi.json` once the endpoints work:

- Add `get` to `/medicines` (`operationId: listMedicines`) with `limit`, `offset`, `name` and `barcode` query parameters (with `minimum`, `maximum` and `default` where they apply), and `200`, `400`, `401` and `500` responses
- Add `/medicines/barcode/{barcode}` with a `get` (`operationId: getMedicineByBarcode`) and a `barcode` path parameter; responses `200`, `400`, `401`, `404` and `500`
- Add `/medicines/{id}` with a `put` (`operationId: updateMedicine`) and a `delete` (`operationId: deleteMedicine`). Both take an `id` path parameter (`format: uuid`)
  - `put`: request body `UpdateMedicineRequest`; responses `204`, `400`, `401`, `404`, `409` and `500`
  - `delete`: responses `204`, `400`, `401`, `404`, `409` (a purchase order uses it) and `500`
- Every new operation uses `security: [{"bearerAuth": []}]`, and the `204` responses have no `content`
- Add schemas `UpdateMedicineRequest` (`name`, `barcode`, `batch_number`, `expiration_date` — no `quantity`), `MedicineResponse` (`data`) and `MedicineListResponse` (`data`, `total`, `limit`, `offset`), reusing `Medicine` and `ErrorResponse`
- The file must remain valid JSON and `apps/api/internal/handler/swagger/handler_test.go` must still pass

### 8. Tests

- **`response.NoContent`**: status `204`, empty body, no `Content-Type`
- **Service** (mocked repository):
  - `List`: passes the filter through, returns the page and total, repository error
  - `GetByBarcode`: found, not found passes through, repository error
  - `Update`: success; ID not found → not found and no checks run; name + batch conflict; barcode conflict; both conflict (name + batch wins); the checks receive the medicine's own ID as `excludeID`; conflict from the update itself passes through unchanged; not found from the update itself (deleted in between) passes through; each repository error path (exists, both checks, update)
  - `Delete`: success; ID not found → not found and no PO check; in a purchase order → `ErrMedicineInPurchaseOrder` and `Delete` is not called; each repository error path (lock, PO check, delete)
- **Repository error translation**: the existing unit test also covers `Update` through the shared function; update its expected wrap text. Also unit test the LIKE-escaping helper (`%`, `_`, `\` and a plain term)
- **Handler** (mocked service, through the real middleware):
  - `List`: defaults applied, custom values and filters passed through (trimmed), each invalid `limit` / `offset` case (non-numeric, `0`, `101`, negative offset, fractional), too-long `name` / `barcode`, `200` body shape, an empty result is `"data": []` and not `null`, `500`
  - `GetByBarcode`: `200` body shape, trimmed barcode passed through, too-long barcode, `404`, `500`
  - `Update`: `204` with an empty body, the trimmed input and path ID reach the service, every `400` case from the create validation table except the quantity rows, invalid path ID, `404`, both `409` cases, `500`, `created_by` and `quantity` in the body are ignored (even an invalid `quantity` does not cause a `400`)
  - `Delete`: `204` with an empty body, invalid path ID, `404`, purchase order `409`, `500`
  - All four: no caller in context → `401`
- The rest of the repository is thin GORM calls — skip unit tests per the standards
- Coverage ≥ 90%. `cover: true` is already set on the `api` test target — verify it stays

---

## Endpoints

All require `Authorization: Bearer <access_token>`, and are served under the `API_PREFIX` (default `/api`).

### `GET /medicines?limit=20&offset=0&name=para&barcode=890`

`limit`, `offset`, `name` and `barcode` are all optional. `200 OK`:

```json
{
  "data": [
    {
      "id": "0b8f3c62-6a1e-4c3e-9d0e-5f1d2a7c9e11",
      "name": "Paracetamol 500mg",
      "barcode": "8901234567890",
      "batch_number": "B2026-001",
      "expiration_date": "2027-03-31",
      "quantity": 120,
      "created_by": "7d2f4a10-3b5c-4e8a-a1f6-9c0b2d4e6f88",
      "created_at": "2026-09-20T08:15:30Z",
      "updated_at": "2026-09-20T08:15:30Z"
    }
  ],
  "total": 57,
  "limit": 20,
  "offset": 0
}
```

- Newest first. Soft-deleted medicines are never listed
- `name` and `barcode` match anywhere in the value, ignoring case; give both to narrow the result. `total` counts the matches, not the whole table
- An offset past the end returns `200` with `"data": []` and the real `total`

### `GET /medicines/barcode/{barcode}`

`200 OK`:

```json
{
  "data": {
    "id": "0b8f3c62-6a1e-4c3e-9d0e-5f1d2a7c9e11",
    "name": "Paracetamol 500mg",
    "barcode": "8901234567890",
    "batch_number": "B2026-001",
    "expiration_date": "2027-03-31",
    "quantity": 120,
    "created_by": "7d2f4a10-3b5c-4e8a-a1f6-9c0b2d4e6f88",
    "created_at": "2026-09-20T08:15:30Z",
    "updated_at": "2026-09-20T08:15:30Z"
  }
}
```

The barcode must match exactly. A soft-deleted medicine is not found.

### `PUT /medicines/{id}`

Request:

```json
{
  "name": "Paracetamol 500mg",
  "barcode": "8901234567890",
  "batch_number": "B2026-002",
  "expiration_date": "2027-06-30"
}
```

`204 No Content` — empty body. `quantity`, if sent, is ignored.

### `DELETE /medicines/{id}`

`204 No Content` — empty body. The medicine is soft-deleted, so its barcode stays locked (see the create spec). Refused with `409` while any non-deleted purchase order has an item for it.

### Errors

| Status | When                                                           | Body                                                                   |
|--------|----------------------------------------------------------------|------------------------------------------------------------------------|
| 400    | Bad query param, bad path ID or barcode, invalid body or field | `{"error": "<message>"}`                                               |
| 401    | Missing, malformed, expired or non-access token                | `{"error": "unauthorized"}`                                            |
| 404    | No active medicine with that ID or barcode                     | `{"error": "medicine not found"}`                                      |
| 409    | Update: another medicine has the same `name` + `batch_number`  | `{"error": "medicine with this name and batch number already exists"}` |
| 409    | Update: another medicine has the same `barcode`                | `{"error": "medicine with this barcode already exists"}`               |
| 409    | Delete: a purchase order uses the medicine                     | `{"error": "medicine is used in a purchase order and cannot be deleted"}` |
| 500    | Anything unexpected                                            | `{"error": "internal server error"}`                                   |

---

## Open Questions

- **Which purchase orders block a delete** — I took "in a PO" literally: any purchase order item for the medicine blocks the delete, whatever the order's status, including `cancelled` and `received`, so a medicine that was ever ordered can never be deleted. Only soft-deleted orders are ignored. If only open orders (`draft`, `ordered`, `partially_received`) should block it, the check needs a status filter.
- **Scanning a deleted medicine's barcode** — the barcode lookup returns `404` for a soft-deleted medicine, but creating a medicine with that barcode returns `409`, because the barcode stays locked. In the scanner flow that means "not found", then "already exists". It is consistent with the rules so far; a restore feature or a clearer `409` message would fix the experience later.
- **Name and barcode filters** — they are two separate parameters, both partial and case-insensitive, and combining them narrows the result (AND). If you would rather have one search box that matches either field, it becomes a single `q` parameter matching name OR barcode. There is no index for `ILIKE '%…%'`, so it scans the table — fine for a modest inventory.
- **What is not included** — no get-by-ID endpoint, no sorting options, and no restore for a deleted medicine.

---

## References

- @context/go-standards.md
- @context/features/06-medicine.spec.md
- @context/features/05-swagger.spec.md
- `apps/migration/transactions/00007_create_medicines_table.sql`
- `apps/migration/transactions/00013_create_purchase_order_items_table.sql`
- `apps/migration/transactions/00020_create_updated_at_triggers.sql`
- `apps/api/pkg/apperror/apperror.go`
- `apps/api/pkg/response/response.go`
- `apps/api/internal/repository/user/repository.go` — example of a cross-table join
- `apps/api/internal/handler/swagger/openapi.json`
