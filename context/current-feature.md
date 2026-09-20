# Current Feature: Purchase Orders (Create, Read, Receive)

Spec: @context/features/10-purchase-orders.spec.md

## Status

In Progress

## Goals

- Add purchase order endpoints to `apps/api`, layered Handler → Service → Repository via interfaces, in new `internal/{handler,service,repository}/purchaseorder` packages; all four behind the existing `auth` middleware (`401 unauthorized` if no caller); RBAC out of scope
- `POST /purchase-orders` → `201`, creates a PO (status `draft`) with its items in one transaction; returns the PO with `items` and an empty `receipts`
- `GET /purchase-orders` → `200`, offset-paginated (`limit` 1–100 default 20, `offset` >= 0 default 0), newest first, header only (no `items`/`receipts`); returns `data`, `total`, `limit`, `offset`; empty page is `"data": []`
- `GET /purchase-orders/{id}` → `200` with the header, `items` and `receipts` (each receipt with its items), loaded in a constant number of queries; `404 purchase order not found` for unknown/soft-deleted
- `POST /purchase-orders/{id}/receive` → `201`, receives the whole PO: one receipt plus one receipt item per PO item (`quantity_received` = `quantity_ordered`, damaged and returned `0`), status set to `received`, all in one transaction; only `draft` or `ordered` can be received, otherwise `409 purchase order cannot be received in its current status`; an empty body is accepted
- Create validation (trimmed, UUIDs lowercased): `supplier_id` required UUID; `order_date` required `YYYY-MM-DD` (past allowed); `expected_date` optional, not before `order_date`; `notes` optional max 500; `items` required 1–100; `items[i].medicine_id` required UUID, not repeated; `items[i].quantity_ordered` required, `1`–`2147483647`; body valid JSON max 1 MiB; path `id` must be a UUID (`400 invalid purchase order id`); messages exactly as in the spec table
- Create locks the supplier (`LockByID`) then each distinct medicine in ascending ID order (`LockByID`) before inserting, so a supplier or medicine deleted first is `404 supplier not found` / `404 medicine not found`, and a delete racing after a create is refused with `409`
- Receive locks the PO row (`LockStatusByID`, `FOR UPDATE`) so two receives can't both succeed, and inserts receipt items in `medicine_id, id` order; the service never writes `medicines.quantity` (the `00018` trigger does)
- The repository's `Transaction(ctx, actorID, fn)` sets `set_config('app.actor_id', actorID, true)` as its first statement and hands `fn` a PO, a medicine and a supplier repository bound to the same transaction, so audit rows get the caller as `actor_id`
- `created_by` and `received_by` come from the token, never the body
- New `domain/purchase_order.go` (PO, item, receipt, receipt item, status constants) and `pkg/apperror` additions: `ErrSupplierNotFound`, `ErrMedicineNotFound` (both wrap `ErrNotFound`), `ErrPurchaseOrderNotReceivable` (wraps `ErrConflict`)
- Wire the four routes in `apps/api/cmd/api/main.go`
- Update `apps/api/internal/handler/swagger/openapi.json`: `purchase-orders` tag, four operations (`createPurchaseOrder`, `listPurchaseOrders`, `getPurchaseOrder`, `receivePurchaseOrder`) and the schemas listed in the spec; stays valid JSON and swagger `handler_test.go` passes
- Tests per spec §8: service (mocked repos), handler (mocked service, real middleware), and GORM DryRun SQL tests for the actor `set_config`, the `FOR UPDATE` lock, active-only scoping and ordering, and the status-only update; ≥ 90% coverage on service and handler
- `tidy`, `lint`, `build` and `test` pass via Nx for `api`

## Notes

- No new migration: tables `00012`–`00015`, the quantity-sync trigger `00018`, the audit triggers `00019` and the `updated_at` trigger `00020` already exist
- This is the first feature to write to audited tables, so it is the first to need `app.actor_id`; suppliers, medicines and inventory entries are not audited and are unchanged
- Not in scope: updating, cancelling or deleting a PO, editing items, partial/damaged/returned receiving, and list filters
- Handler error mapping: check `ErrSupplierNotFound` and `ErrMedicineNotFound` before the bare `ErrNotFound`, since they wrap it; bare `ErrNotFound` is `purchase order not found` (get, receive)
- Open questions in the spec (defaults chosen, change only if told):
  - Create writes `draft` and receive accepts `draft` or `ordered`, so `ordered`, `cancelled` and `partially_received` are unreachable for now
  - No way to delete a PO, and medicine/supplier deletes are refused by any non-deleted PO, so anything ever put on a PO can no longer be deleted; likely needs a PO soft delete or a narrower check
  - Receiving adds stock to the referenced `medicines` row, so a restock with a different batch or expiry inherits the old row's date
  - No list filters (`supplier_id`, `status`); repeated medicines rejected and 100-item cap are spec choices; `404` for a missing body reference, matching inventory; receive returns `201` with the receipt
- Reuse: `LockByID` from `repository/medicine` and `repository/supplier`; the two-repository `Transaction` shape from `repository/inventory`
- References: @context/go-standards.md, @context/database-schema.md, @context/features/02-audit-purchase-orders.spec.md, @context/features/07-medicine-RUD.spec.md, @context/features/08-inventory.spec.md, @context/features/09-suppliers.spec.md, `apps/api/pkg/apperror/apperror.go`, `apps/api/pkg/response/response.go`
