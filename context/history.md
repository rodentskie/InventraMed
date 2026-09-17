# History

## Audit trail and purchase order schema

- Added migrations for `suppliers`, `purchase_orders`, `purchase_order_items`, `purchase_order_receipts`, `purchase_order_receipt_items`, `inventory_entries`, and `audit_logs`
- `purchase_order_items`/`purchase_order_receipts`/`purchase_order_receipt_items` are immutable (no `updated_at`/`deleted_at`); `suppliers`/`purchase_orders` are soft-deletable
- Added DB triggers: sync `medicines.quantity` on `purchase_order_receipt_items`/`inventory_entries` insert, auto-populate `audit_logs` on write to `purchase_orders` and its child tables (not `medicines`, to avoid double-logging quantity syncs), and auto-maintain `updated_at` on `suppliers`/`purchase_orders`
- Audit trigger reads the acting user from an `app.actor_id` session setting — callers must `SET LOCAL` it per-transaction; not yet wired into any Go code
- Schema-only pass — the "Create PO" / "Receive full PO" API endpoints (handler/service/repository in `apps/api`) from @context/features/02-audit-purchase-orders.spec.md are not yet implemented

## Migration app with RBAC and medicine inventory schema

- Built `apps/migration` (GORM + Goose) with schema migrations for RBAC (`users`, `roles`, `policies`, `user_roles`, `role_policies`) and medicine inventory (`medicines`, `settings`)
- Added `deleted_at` soft-delete columns where applicable
- Seeded default `settings` row and `admin`/`standard` roles with corresponding IAM-style policies
