### Medicines Page

This will be on `apps/app`.
CRUD page for medicines: list, create, update and delete.

## Requirements

### Routing & layout

- Move `home/` into a route group so its layout (guard + side nav + top nav) is shared without changing existing URLs: `app/(app)/home/` and `app/(app)/medicines/`, e.g. `app/(app)/layout.tsx` (moved from `home/layout.tsx`), `app/(app)/home/page.tsx`, `app/(app)/medicines/page.tsx`
- Page route: `/medicines` (matches the existing `NAV_ITEMS` entry in `src/lib/nav.ts`, no change needed there)
- Widen the `proxy.ts` matcher from `["/home", "/home/:path*"]` to also cover `/medicines`, `/medicines/:path*` (and, going forward, the other workspace routes as their pages are built)

### Page layout

- Heading ("Medicines") with a "New Medicine" button top-right that opens the create drawer
- Responsive: table scrolls horizontally on narrow viewports instead of breaking layout; drawer is full-width on mobile, fixed-width panel on desktop (chakra `DrawerRoot` `size` prop, e.g. `sm`/`md` on desktop via responsive value)

### List (table)

- Table listing medicines: Name, Barcode, Batch Number, Expiration Date, Quantity, Status, Actions
- `Status` (green/yellow/red) is derived client-side from `expiration_date` per @context/project-overview.md — the API does not return a status field. Render with the `Status` snippet (`libs/ts/snippets/src/lib/status.tsx`)
- Batch Number shows an em dash or similar when `null`
- Actions column has two icon buttons: Update (opens the edit drawer prefilled with that row) and Delete (opens the delete confirmation dialog)
- Empty state (no medicines match) uses the `EmptyState` snippet
- Data source: `GET /medicines` (query params: `limit`, `offset`, optional `name`/`barcode` filters — filters are out of scope for this pass, page uses `limit`/`offset` only)

### Pagination

- Basic offset pagination: Previous / Next buttons only (no page-number list), built from the `Pagination` snippet's `PaginationRoot` + `PaginationPrevTrigger` / `PaginationNextTrigger`
- Fixed `limit` (e.g. 20); `offset` tracked in component state (or URL search param) and incremented/decremented by `limit` per click
- `MedicineListResponse` returns `{ data, total, limit, offset }` — Next is disabled when `offset + limit >= total`; Previous is disabled when `offset <= 0`. Don't rely on `data.length < limit` alone, since `total` is the authoritative count

### Create (drawer)

- "New Medicine" button opens a right-side drawer (`DrawerRoot` with `placement="end"`) titled "New Medicine"
- Form fields: Name (text, required), Barcode (text, required), Batch Number (text, optional), Expiration Date (date, required), Quantity (number, required, min 0)
- Submits `POST /medicines` with `{ name, barcode, batch_number, expiration_date, quantity }`; on success (`201`) close the drawer, toast success, and refresh the table
- On `409` (duplicate name+batch, or duplicate barcode) show the server's `error` message inline on the relevant field (or as a form-level alert); on `400` show the field-level validation message from `error`
- Use the `Field` snippet for labels/validation messages and `Toaster` for success/error toasts

### Update (drawer)

- Update action opens the same style of drawer, titled "Edit Medicine", prefilled with the selected row's current values
- Fields: Name, Barcode, Batch Number, Expiration Date (no Quantity field — `PUT /medicines/{id}` ignores `quantity` in the body and cannot change it here, per the swagger description)
- Submits `PUT /medicines/{id}` with `{ name, barcode, batch_number, expiration_date }`; on success (`204`) close the drawer, toast success, and refresh the table
- Same `400`/`409` error handling as create; `404` (medicine deleted by someone else in the meantime) closes the drawer, toasts an error, and refreshes the table

### Delete (dialog)

- Delete action opens a confirmation dialog (`DialogRoot`) — not the drawer
- Dialog explains the action is permanent-ish (soft delete) and requires typing `delete` (case-insensitive) into a text input before the confirm button is enabled
- Confirms via `DELETE /medicines/{id}`; on success (`204`) close the dialog, toast success, and refresh the table
- On `409` (medicine is used in a purchase order) close the dialog and toast the server's `error` message instead of retrying
- On `404` close the dialog, toast an error, and refresh the table

### Data fetching

- Follow existing patterns: Server Actions in `src/actions/medicines.ts` returning `ActionResult<T>` (see `src/actions/auth.ts`, `src/types/action.ts`), using `process.env.API_URL` / `API_PREFIX` and forwarding the `access_token` cookie as `Authorization: Bearer <token>`
- Types in `src/types/medicine.ts` mirroring `Medicine`, `MedicineListResponse`, `CreateMedicineRequest`, `UpdateMedicineRequest` from the swagger schemas
- Table refresh after create/update/delete: re-run the list action and update local state (client component); no need for full page reload

## References

- @context/screenshots/InventraMed Prototype.pdf
- @apps/api/internal/handler/swagger/openapi.json
- @libs/ts/snippets/src/lib/*.tsx
- @apps/app/src/app/home/layout.tsx
- @apps/app/src/app/home/page.tsx
- @apps/app/src/actions/auth.ts
- @apps/app/src/types/action.ts
- @apps/app/src/lib/nav.ts
- @apps/app/src/proxy.ts
