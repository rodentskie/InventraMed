# Current Feature: Suppliers CRUD

Spec: @context/features/09-suppliers.spec.md

## Status

In Progress

## Goals

- Add full CRUD for `suppliers` to `apps/api`, layered Handler → Service → Repository via interfaces, mirroring the medicine endpoints
- `POST /suppliers` → `201`, registers a supplier (no uniqueness rule, no duplicate checks)
- `GET /suppliers` → `200`, offset-paginated (`limit` 1–100 default 20, `offset` >= 0 default 0), newest first, optional case-insensitive contains `name` filter (`%`, `_`, `\` escaped); returns `data`, `total`, `limit`, `offset`; empty page is `"data": []`
- `GET /suppliers/{id}` → `200`, `404 supplier not found` for unknown/soft-deleted
- `PUT /suppliers/{id}` → `204`, full replacement (omitted optional field clears it to `NULL`); `404` for unknown
- `DELETE /suppliers/{id}` → `204`, soft delete; `409 supplier is used in a purchase order and cannot be deleted` while any non-deleted purchase order references the supplier (any status)
- All five endpoints wrapped with the existing `auth` middleware (`401 unauthorized` if no caller); RBAC out of scope
- Request validation (trimmed; empty optional → `NULL`): `name` required max 255; `contact_name` max 255; `email` max 255 and valid via `net/mail.ParseAddress`; `phone` max 32; `address` max 500; body valid JSON max 1 MiB; path `id` must be a UUID (`400 invalid supplier id`)
- New `domain.Supplier`, new `apperror.ErrSupplierInPurchaseOrder` (wraps `ErrConflict`), new packages `internal/{handler,service,repository}/supplier`, wired in `apps/api/cmd/api/main.go`
- Update `apps/api/internal/handler/swagger/openapi.json`: `suppliers` tag, five operations (`createSupplier`, `listSuppliers`, `getSupplier`, `updateSupplier`, `deleteSupplier`), schemas `CreateSupplierRequest`, `UpdateSupplierRequest`, `Supplier`, `SupplierResponse`, `SupplierListResponse`; stays valid JSON and swagger `handler_test.go` passes
- Tests: service (mocked repo) and handler (mocked service, real middleware) covering every case in spec §7, plus a GORM DryRun SQL test for `Delete`'s `FOR UPDATE` lock and `ExistsInPurchaseOrder`'s query; coverage ≥ 90% on service and handler
- `tidy`, `lint`, `build` and `test` pass via Nx for `api`

## Notes

- No new migration: `suppliers` (`00011`) is already soft-deletable and has its `updated_at` trigger (`00020`)
- Repository: private `record` mapped via `TableName()`, `DeletedAt gorm.DeletedAt`, no `gorm.Model`; `Update` must use `Select(...)` or a map so `nil` fields write `NULL`; map to `domain.Supplier` at the package boundary
- Reuse from `repository/medicine`: `escapeLike`, `LockByID`, `Transaction`, `Update`'s `Select`-based write; `ExistsInPurchaseOrder` is a direct `WHERE supplier_id = ? AND deleted_at IS NULL` on `purchase_orders` (no join)
- Service: `Create`/`List`/`GetByID`/`Update` call the repository directly (`Update`'s `ErrNotFound` passes through, no pre-check); `Delete` runs in one transaction: `LockByID` → `ExistsInPurchaseOrder` → `Delete`. Other repository errors are logged and wrapped `fmt.Errorf("<operation> supplier: %w", err)`
- `UpdateInput = CreateInput` (identical today; split if update diverges)
- Handler: caller from `middleware.AccountFromContext` only enforces auth (`suppliers` has no `created_by`); responses via `response` helpers only; `500` logs the real error and returns `internal server error`
- Skip unit tests on thin GORM passthroughs (`Create`, `FindByID`, `List`, `Update` happy path) per @context/go-standards.md
- Open questions in the spec (defaults chosen, change only if told): `name`-only list filter; no uniqueness rule; any non-deleted purchase order blocks delete; `UpdateInput = CreateInput`
- References: @context/go-standards.md, @context/database-schema.md, @context/features/06-medicine.spec.md, @context/features/07-medicine-RUD.spec.md, `apps/api/internal/repository/medicine/repository.go`, `apps/api/pkg/apperror/apperror.go`, `apps/api/pkg/response/response.go`
