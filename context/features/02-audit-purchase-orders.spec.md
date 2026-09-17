# Audit Trail & Purchase Orders

## Overview

Three additions on top of the base schema from @context/features/01-migrations.spec.md:

1. An **audit trail** for key entities, so changes (and later, medicine disposal/removal) can be traced over time.
2. A **basic Purchase Order (PO) module** for restocking: create a PO for a supplier with line items (medicine + quantity), then receive it. Only "create PO" and "receive full PO" need to work for now, but the schema must already be able to represent partial receiving, damaged goods, and returns so a later feature doesn't require another migration.
3. **Inventory entries** — this app is inventory-only (no POS), so there's no sales flow to consume stock. Instead, staff periodically (e.g. end of day) reconcile physical stock by hand: subtracting quantity (consumed, lost, damaged, expired removal) or adding it back (e.g. a returned item). The schema needs a table to record these manual adjustments per medicine.

## Requirements

### Audit Trail

- `audit_logs` — generic, append-only log (no update/delete):
  - `entity_type` (e.g. `medicine`, `purchase_order`)
  - `entity_id`
  - `action` (e.g. `created`, `updated`, `deleted`, `received`)
  - `changes` — before/after payload (jsonb)
  - `actor_id` — fk `users.id`, nullable (system-initiated changes)
  - `created_at`
- Applies to `medicines` and `purchase_orders` (and their child tables) to start

### Suppliers

- `suppliers`: `name`, `contact_name`, `email`, `phone`, `address`, soft-deletable

### Purchase Orders

- `purchase_orders`: `supplier_id`, `status` (`draft`, `ordered`, `partially_received`, `received`, `cancelled`), `order_date`, `expected_date`, `created_by` (fk `users.id`), `notes`, soft-deletable
- `purchase_order_items`: `purchase_order_id`, `medicine_id`, `quantity_ordered`
- `purchase_order_receipts`: one row per receiving event on a PO (supports multiple partial shipments) — `purchase_order_id`, `received_by` (fk `users.id`), `received_at`, `notes`
- `purchase_order_receipt_items`: per receiving event, per PO item — `quantity_received`, `quantity_damaged`, `quantity_returned`, `notes`

"Receive full PO" (the only receiving flow implemented now) creates a single `purchase_order_receipts` row where every `purchase_order_receipt_items.quantity_received` equals the item's `quantity_ordered` and damaged/returned are `0`. Partial/damaged/returned receiving later is additive — no schema change needed.

### Behavior for this pass

- Create PO: supplier + line items (medicine, quantity)
- Receive PO: mark full quantities received, set `purchase_orders.status = received`
- Reconciling received quantity into `medicines.quantity`, and turning damaged/returned quantities into actual inventory adjustments, is business logic for a later feature — not part of this migration

### Inventory Entries

- `inventory_entries`: manual stock adjustment, one row per medicine per adjustment
  - `medicine_id` — fk `medicines.id`
  - `direction` — `addition` | `subtraction`
  - `quantity` — positive int, the amount being added or subtracted
  - `reason` — e.g. `count_adjustment`, `returned`, `damaged`, `lost`, `expired_removal`
  - `counted_by` — fk `users.id`
  - `notes`, `created_at`
- Not soft-deletable — an adjustment is a fact that happened; correcting a mistake is a new offsetting entry, not an edit/delete

### Triggers

- Add DB triggers where they keep derived/denormalized data correct without relying on every caller remembering to do it, for example:
  - Keep `medicines.quantity` in sync when a `purchase_order_receipt_items` row is inserted (increment) or an `inventory_entries` row is inserted (increment/decrement by `direction`)
  - Auto-populate `audit_logs` on insert/update/delete of tracked tables, instead of each service call doing it manually
  - Auto-maintain `updated_at` on row update
- Only add a trigger where it removes a real risk of drift/inconsistency — don't add one just because a table changes

## Open Questions

- `purchase_order_items` reference `medicines.id` directly, per instruction — but `medicines` currently models a specific stock batch (`batch_number` + `expiration_date`), not a reorderable "product." A restock may arrive with a different batch/expiration than the referenced row. May need a separate product/catalog concept later, or receiving may always insert a new `medicines` row.
- Should `purchase_order_items` / `purchase_order_receipts` / `purchase_order_receipt_items` be soft-deletable, or treated as immutable once created?
- Do damaged/returned quantities need a `reason`/`disposition` field (e.g. "expired on arrival" vs "wrong item shipped")?
- `audit_logs` records write events — it does not by itself produce "history of expired medicines," since expiration status is derived at read time and nothing writes when a medicine's status silently flips (e.g. green → yellow overnight). Closing that loop (e.g. logging a disposal/removal event) is a separate decision.
- If a trigger updates `medicines.quantity` on `inventory_entries`/receipt inserts, does that same trigger also write an `audit_logs` row, or would that double-log against the `inventory_entries`/`purchase_order_receipt_items` row that already records the change?
- Should `direction` + `quantity` be a single signed integer (`quantity_delta`) instead? Simpler for a trigger to apply (`quantity + delta`), at the cost of losing the readable `addition`/`subtraction` label.

## References

- @context/database-schema.md
- @context/features/01-migrations.spec.md
