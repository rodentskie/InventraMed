### Inventory Entries Page

This will be on `apps/app`.

List + create page for inventory entries (manual stock adjustments). Unlike medicines, entries are **append-only** per @apps/api/internal/handler/swagger/openapi.json (`POST /inventory-entries` description: "correcting a mistake means recording a new offsetting entry, not editing or deleting this one") — there is no update or delete here, and no `PUT`/`DELETE` exists on the API for entries.

## Requirements

### Routing & layout

- Page route: `/inventory-entries` (matches the existing `NAV_ITEMS` entry in `src/lib/nav.ts`, no change needed there), at `app/(app)/inventory-entries/page.tsx` — same route group as `medicines`, so it gets the shared guard + side nav + top nav for free
- Widen the `proxy.ts` matcher from `["/home", "/home/:path*", "/medicines", "/medicines/:path*"]` to also cover `/inventory-entries`, `/inventory-entries/:path*`

### Page layout

- Heading ("Inventory Entries") with a "Record Adjustment" button top-right that opens the create drawer
- Responsive: table scrolls horizontally on narrow viewports instead of breaking layout; drawer is full-width on mobile, fixed-width panel on desktop (same `DrawerRoot` `size` pattern as the medicines drawer)

### Medicine lookup (table)

- The entry list/response has no embedded medicine details (only `medicine_id`), and there's no `GET /medicines/{id}`. Fetch a lookup list once per page load via the existing `listMedicines` action (`src/actions/medicines.ts`) with `limit=100, offset=0` (the API's max page size) and build a `Map<string, Medicine>` keyed by `id`, used to render the table's Medicine column (name + barcode)
- Known limit, don't try to work around it: this lookup only covers the first 100 active medicines and excludes deleted ones, so an entry referencing a medicine outside that set (more medicines than the page size, or a medicine deleted after the entry was recorded) shows a fallback (the raw `medicine_id`) in the table instead of a name
- This limit does **not** apply to the create drawer's medicine picker — see Create (drawer) below, which searches the API directly instead of relying on this bulk lookup

### List (table)

- Table listing inventory entries: Date, Medicine, Direction, Quantity, Reason, Notes, Counted By
- Date is `created_at`, formatted like the medicines table's date columns
- Medicine resolved via the lookup map (name + barcode), falling back to the raw `medicine_id` when not found
- Direction (`addition`/`subtraction`) rendered with the `Tag` snippet — `colorPalette="green"` for addition, `colorPalette="red"` for subtraction, capitalized label
- Notes shows an em dash or similar when `null`
- Counted By shows `counted_by.name` — the API joins `inventory_entries.counted_by` to `users` server-side and returns `counted_by` as `{ id, name }`, so no client-side resolution is needed. Falls back to the em dash on the rare case `name` comes back empty (e.g. a hard-deleted or missing user row)
- No Actions column: entries can't be updated or deleted
- Empty state (no entries yet) uses the `EmptyState` snippet
- Data source: `GET /inventory-entries` (query params: `limit`, `offset` only — the API has no filters for this endpoint)

### Pagination

- Same pattern as the medicines page: `PaginationRoot` + `PaginationPrevTrigger` / `PaginationNextTrigger`, no page-number list
- Fixed `limit` (e.g. 20); `offset` tracked in component state, incremented/decremented by `limit` per click
- `InventoryEntryListResponse` returns `{ data, total, limit, offset }` — Next disabled when `offset + limit >= total`; Previous disabled when `offset <= 0`, same as medicines (don't rely on `data.length < limit`)

### Create (drawer)

- "Record Adjustment" button opens a right-side drawer (`DrawerRoot`, `placement="end"`) titled "Record Stock Adjustment"
- Form fields:
  - Medicine (`Combobox`, required) — a type-to-search field (`MedicineCombobox`, `libs/ts/snippets/src/lib/combobox.tsx` + Chakra's `useListCollection`), not the bulk lookup: each keystroke (debounced ~300ms) calls `listMedicines({ name, limit: 20, offset: 0 })` against the API's `name` filter (`GET /medicines?name=`), so it finds a match regardless of how many medicines exist. Shows an initial page of medicines before the user types anything, and a loading/empty state while a search is in flight or comes back empty
  - Direction (`NativeSelect`, required) — "Addition" / "Subtraction", backed by `addition`/`subtraction`
  - Quantity (number, required, min 1 per the API's `minimum: 1`)
  - Reason (text, required, maxLength 100) — free text per the API; placeholder suggests common values from the swagger example (`count_adjustment`, `returned`, `damaged`, `lost`, `expired_removal`) without constraining input to them
  - Notes (textarea, optional, maxLength 500)
- Submits `POST /inventory-entries` with `{ medicine_id, direction, quantity, reason, notes }`; on success (`201`) close the drawer, toast success, and refresh the table (and prepend/refetch so the new entry and any stock change are visible)
- `400` (validation), `404` (medicine not found — e.g. deleted concurrently) and `409` (subtraction would take quantity below zero) all show the server's `error` message as a form-level alert (mirrors the medicines drawer's `Alert` usage) and keep the drawer open so the user can correct and resubmit, rather than closing on error
- Use the `Field` snippet for labels/validation messages and `Toaster` for the success toast

### API: `counted_by` as a joined user object

- `apps/api`'s `GET /inventory-entries`, `GET /inventory-entries/{id}` and `POST /inventory-entries` now return `counted_by` as `{ id, name }` instead of a bare UUID, joining `inventory_entries.counted_by` to `users.id` (LEFT JOIN, so a soft-deleted user's name still resolves for historical entries) in `apps/api/internal/repository/inventory`. `POST`'s response reloads the just-created row through the same joined query so it also carries the name. Swagger's `InventoryEntry.counted_by` now `$ref`s a new `InventoryEntryCountedBy` schema

### No update or delete

- Entries are immutable: no edit drawer, no delete dialog, no Actions column. A miscount or mistake is corrected by recording a new offsetting entry through the same create drawer, per the API's own description

### Data fetching

- Follow existing patterns: Server Actions in `src/actions/inventory-entries.ts` returning `ActionResult<T>` (see `src/actions/medicines.ts`), using `process.env.API_URL` / `API_PREFIX` and forwarding the `access_token` cookie as `Authorization: Bearer <token>`
  - `listInventoryEntries({ limit, offset })` → `GET /inventory-entries`
  - `createInventoryEntry(input)` → `POST /inventory-entries`
- Types in `src/types/inventory-entry.ts` mirroring `InventoryEntry`, `InventoryEntryListResponse`, `CreateInventoryEntryRequest` from the swagger schemas
- Reuse `listMedicines` from `src/actions/medicines.ts` for the medicine lookup — no new medicines action needed
- Table refresh after create: re-run the list action and update local state (client component); no full page reload

## References

- @context/features/13-medicines-page.spec.md
- @apps/api/internal/handler/swagger/openapi.json
- @libs/ts/snippets/src/lib/*.tsx
- @apps/app/src/app/(app)/medicines/page.tsx
- @apps/app/src/components/medicines/*.tsx
- @apps/app/src/actions/medicines.ts
- @apps/app/src/types/medicine.ts
- @apps/app/src/types/action.ts
- @apps/app/src/lib/nav.ts
- @apps/app/src/proxy.ts
