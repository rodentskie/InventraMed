### Suppliers Page

This will be on `apps/app`.
CRUD page for suppliers: list, create, update and delete. Same shape as @context/features/13-medicines-page.spec.md, adapted to the supplier fields and endpoints.

## Requirements

### Routing & layout

- Page route: `/suppliers` (matches the existing `NAV_ITEMS` entry in `src/lib/nav.ts`, no change needed there), at `app/(app)/suppliers/page.tsx` — same route group as `medicines`/`inventory-entries`, so it gets the shared guard + side nav + top nav for free
- Widen the `proxy.ts` matcher from `["/home", "/home/:path*", "/medicines", "/medicines/:path*", "/inventory-entries", "/inventory-entries/:path*"]` to also cover `/suppliers`, `/suppliers/:path*`

### Page layout

- Heading ("Suppliers") with a "New Supplier" button top-right that opens the create drawer
- Responsive: table scrolls horizontally on narrow viewports instead of breaking layout; drawer is full-width on mobile, fixed-width panel on desktop (same `DrawerRoot` `size` pattern as the medicines drawer)

### List (table)

- Table listing suppliers: Name, Contact Name, Email, Phone, Address, Actions
- Contact Name, Email, Phone and Address each show an em dash or similar when `null` (all four are optional per the API)
- Actions column has two icon buttons: Update (opens the edit drawer prefilled with that row) and Delete (opens the delete confirmation dialog)
- Empty state (no suppliers match) uses the `EmptyState` snippet
- Data source: `GET /suppliers` (query params: `limit`, `offset`, optional `name` filter — filter UI is out of scope for this pass, page uses `limit`/`offset` only)

### Pagination

- Same pattern as the medicines page: `PaginationRoot` + `PaginationPrevTrigger` / `PaginationNextTrigger`, no page-number list
- Fixed `limit` (e.g. 20); `offset` tracked in component state, incremented/decremented by `limit` per click
- `SupplierListResponse` returns `{ data, total, limit, offset }` — Next disabled when `offset + limit >= total`; Previous disabled when `offset <= 0`, same as medicines (don't rely on `data.length < limit`)

### Create (drawer)

- "New Supplier" button opens a right-side drawer (`DrawerRoot` with `placement="end"`) titled "New Supplier"
- Form fields: Name (text, required, maxLength 255), Contact Name (text, optional, maxLength 255), Email (text, optional, maxLength 255, validated as an email format client-side), Phone (text, optional, maxLength 32), Address (textarea, optional, maxLength 500)
- Submits `POST /suppliers` with `{ name, contact_name, email, phone, address }` — omit or send empty string for unset optional fields, per the API storing an empty/omitted optional as no value
- On success (`201`) close the drawer, toast success, and refresh the table
- On `409` (duplicate name, case-insensitive) show the server's `error` message inline on the Name field (or as a form-level alert); on `400` show the field-level validation message from `error`
- Use the `Field` snippet for labels/validation messages and `Toaster` for success/error toasts

### Update (drawer)

- Update action opens the same style of drawer, titled "Edit Supplier", prefilled with the selected row's current values
- Fields: same as create — Name, Contact Name, Email, Phone, Address. Per the API, an update replaces every field, so an omitted/cleared optional field is stored as no value (the drawer must submit the field's current value, not skip it, to avoid accidentally clearing it)
- Submits `PUT /suppliers/{id}` with `{ name, contact_name, email, phone, address }`; on success (`204`) close the drawer, toast success, and refresh the table
- Same `400`/`409` error handling as create; `404` (supplier deleted by someone else in the meantime) closes the drawer, toasts an error, and refreshes the table

### Delete (dialog)

- Delete action opens a confirmation dialog (`DialogRoot`) — not the drawer
- Dialog explains the action is permanent-ish (soft delete) and requires typing `delete` (case-insensitive) into a text input before the confirm button is enabled
- Confirms via `DELETE /suppliers/{id}`; on success (`204`) close the dialog, toast success, and refresh the table
- On `409` (supplier is used in a purchase order) close the dialog and toast the server's `error` message instead of retrying
- On `404` close the dialog, toast an error, and refresh the table

### Data fetching

- Follow existing patterns: Server Actions in `src/actions/suppliers.ts` returning `ActionResult<T>` (see `src/actions/medicines.ts`), using `process.env.API_URL` / `API_PREFIX` and forwarding the `access_token` cookie as `Authorization: Bearer <token>`
  - `listSuppliers({ limit, offset })` → `GET /suppliers`
  - `createSupplier(input)` → `POST /suppliers`
  - `updateSupplier(id, input)` → `PUT /suppliers/{id}`
  - `deleteSupplier(id)` → `DELETE /suppliers/{id}`
- Types in `src/types/supplier.ts` mirroring `Supplier`, `SupplierListResponse`, `CreateSupplierRequest`, `UpdateSupplierRequest` from the swagger schemas
- Table refresh after create/update/delete: re-run the list action and update local state (client component); no need for full page reload

### Components

Mirror the medicines component split under `src/components/suppliers/`:

- `SuppliersPageClient.tsx` — owns list state, pagination offset, and which drawer/dialog is open
- `SuppliersTable.tsx` — renders the table + empty state
- `SupplierFormDrawer.tsx` — shared create/edit drawer (mode prop, like `MedicineFormDrawer`)
- `DeleteSupplierDialog.tsx` — delete confirmation dialog

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
