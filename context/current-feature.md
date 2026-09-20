# Current Feature: Inventory Entries (Create + Read)

## Status

In Progress

## Goals

- `POST /inventory-entries` — record a stock adjustment (`addition`/`subtraction`), validated, `counted_by` from the auth token, returns `201`
- `GET /inventory-entries` — paginated list (`limit`/`offset`, same rules as `GET /medicines`), returns `200`
- `GET /inventory-entries/{id}` — get a single entry, `200` or `404`
- No update, no delete — `inventory_entries` is append-only; corrections are new offsetting entries
- New `internal/{handler,service,repository}/inventory` packages, layered Handler → Service → Repository per @context/go-standards.md
- One additive method on `medicine.Repository`: `LockQuantityByID` (locked read of a medicine's quantity, used by the insufficient-stock guard)
- No new migration — `inventory_entries` table (`00016`) and its quantity-sync trigger (`00018`) already exist
- Update `internal/handler/swagger/openapi.json` for the three new endpoints
- Unit tests ≥ 90% coverage per the spec's test list

## Notes

- Full spec: @context/features/08-inventory.spec.md
- Create validation: `medicine_id` (required UUID, must resolve to an active medicine → `404 medicine not found`), `direction` (exactly `addition`/`subtraction`), `quantity` (`*int`, `> 0`), `reason` (required free text, ≤100 chars — not a closed enum), `notes` (optional, ≤500 chars)
- New rule introduced by this spec, not the DB schema: a `subtraction` that would take `medicines.quantity` below zero is rejected with `409 insufficient quantity for subtraction`, guarded by locking the medicine row (`LockQuantityByID`) inside the same transaction as the insert
- List has only `limit`/`offset` for this pass — no `medicine_id` filter yet (flagged as a likely near-term addition in the spec's Open Questions)
- `reason` is intentionally free text, matching the DB (no `CHECK` constraint) and the schema doc's "e.g." examples (`count_adjustment`, `returned`, `damaged`, `lost`, `expired_removal`)
- The insert relies on the existing `trg_inventory_entries_sync_quantity` trigger to update `medicines.quantity` — the service never writes it directly
