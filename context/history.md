# History

## API app scaffold with root endpoint

- Built `apps/api` entrypoint at `cmd/api/main.go` (`net/http`, no third-party router), wired to `config.LoadConfig()` and the shared `logger`/`env` libs
- Added `internal/handler/root` and `internal/service/root` following Handler → Service (no repository — no data access needed) for `GET /`, returning `{"message":"inventramed REST API"}`
- Added `pkg/response` — a minimal shared JSON response helper (go-standards.md requires handlers to use one; none existed yet in the repo)
- 100% test coverage on `handler/root`, `service/root`, and `pkg/response`
- Swagger documentation and further endpoints deferred to a later phase per @context/features/03-api.spec.md

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

## API login endpoint, hash library and route prefix

- Added `libs/go/hash` (generated via `nx-go:lib-generate`) wrapping `golang.org/x/crypto/bcrypt`: `Hash` at `bcrypt.DefaultCost` and `Compare`, which wraps `ErrMismatch` so callers can use `errors.Is`. 100% coverage
- Added `router.Router` in `apps/api`, a `ServeMux` wrapper that applies a configurable `API_PREFIX` (default `/api`, normalized) to routes registered with `Handle`; `HandleExempt` serves a route as written. `GET /` is exempt, and `/ping` later takes one line
- Added `POST /login` (Handler → Service → Repository): validates the email and password, verifies with `hash.Compare`, and returns `access_token` and `refresh_token` from the `jwt` lib v0.1.1 `BuildTokenPair`. Token payload carries user id, email, and role names
- Unknown email and wrong password both return the same 401; the unknown-email path compares against a precomputed bcrypt hash so response time doesn't reveal which emails exist
- Repository is read-only GORM and filters `deleted_at IS NULL` explicitly on `users` and `roles`, since models don't use `gorm.DeletedAt`
- `LoadConfig` now returns an error: `JWT_SECRET` is required and `JWT_ACCESS_EXPIRY` (`15m`) and `JWT_REFRESH_EXPIRY` (`168h`) are parsed. `api` now connects to PostgreSQL via GORM on startup
- Added `pkg/apperror` and `response.Error` (`{"error": "message"}`)
- Coverage is 91.3% on the login service and 100% on the login handler, router, config and `response`. Not exercised against a live database yet, and the repository has no unit tests
- `libs/go/hash` resolves through `go.work` only, so building with `GOWORK=off` fails. No refresh endpoint, rate limiting or lockout yet, and the auth middleware is still to do (jwt v0.1.1 can't tell expired from invalid tokens)

## API Swagger documentation

- Added `GET /swagger` (exempt from the `API_PREFIX`) in `apps/api`, redirecting to the Swagger UI at `/swagger/index.html`; the UI and its assets are served under `/swagger/` and the spec at `/swagger/openapi.json`
- Documentation is a hand-written OpenAPI 3.0.3 file, `internal/handler/swagger/openapi.json`, embedded into the binary — no annotations in handler comments and no code generation. It covers `GET /` and `POST /login` with their 200/400/401/500 responses
- Added `internal/handler/swagger` (spec served through `response.JSON`) and the `swaggo/http-swagger/v2` dependency, used only for the bundled Swagger UI assets
- The spec is static: it assumes the default `API_PREFIX=/api` (`servers[0].url`), and `/` has its own server override because it is served without the prefix. It is not generated from the code, so it must be updated by hand when routes change
- 100% test coverage on `handler/swagger`

## Create medicine endpoint with auth middleware

- Added `POST /medicines` (served at `/api/medicines`) in `apps/api` (Handler → Service → Repository): validates the input, checks name + batch number (case-insensitive, ignoring soft-deleted rows) and then barcode (exact, including soft-deleted rows), and returns `201` with `{"message": "medicine created", "data": {...}}`. Duplicates return `409` with a message per case
- Added the auth middleware (`internal/middleware/auth.go`): validates `Authorization: Bearer <token>` with `jwt.ParseAccessToken`, so refresh tokens are rejected. Every failure returns the same `401` with `WWW-Authenticate: Bearer`. It stores the `AccountPayload` in the request context, and `created_by` is taken from it, never from the body. It is applied per route by wrapping the handler; `/login`, `/` and swagger stay public. Validation is stateless, so a deleted user's token works until it expires
- Added migration `00022`: a unique index on `lower(name)` and `coalesce(lower(batch_number), '')` where `deleted_at IS NULL`, so concurrent requests can't both insert the same name + batch. Insert-time unique violations are mapped by constraint name (`medicines_barcode_key`, `uq_medicines_name_batch`) to `apperror.ErrBarcodeExists` and `apperror.ErrNameBatchExists`, both wrapping `ErrConflict`
- The repository has a `Transaction(ctx, fn)` method that hands the callback a transaction-bound repository, so the service can run the checks and the insert in one transaction while still being unit-tested with a mock. `created_at` and `updated_at` come from the database defaults
- Updated `context/go-standards.md`: `api` now reads and writes, and soft-deleted tables declare `DeletedAt gorm.DeletedAt` on their `record` (`gorm.Model` is not used because its `ID` is a `uint` and this project uses UUIDs), with `Unscoped()` for queries that must see deleted rows
- Swagger updated: `medicines` tag, `bearerAuth` scheme, and `POST /medicines` with `201`/`400`/`401`/`409`/`500`
- 100% test coverage on the middleware, medicine handler and medicine service; the repository is at 17.6% (only the constraint-name mapping is unit-tested). The migration and the repository queries, including the `now()` defaults, have not been run against a live database yet

## Read, update and delete medicine endpoints

- Added `GET /medicines`: a page of medicines, newest first, with `limit` (default `20`, `1`–`100`), `offset` and optional `name` and `barcode` filters. The filters are case-insensitive "contains" matches with LIKE wildcards escaped, and both together must match. It returns `{data, total, limit, offset}`, where `total` counts the matches and an empty page is `[]`. Invalid values return `400` and are not clamped
- Added `GET /medicines/barcode/{barcode}`: an exact barcode match on active medicines, `200` with `{"data": {...}}` or `404`
- Added `PUT /medicines/{id}` (returns `204`): a full replacement of `name`, `barcode`, `batch_number` and `expiration_date`, with the same validation and duplicate checks as create. The checks skip the medicine itself, and an unknown id returns `404` before any duplicate check. `quantity` cannot be changed here and is ignored if sent, so a body copied from a `GET` response works. A cleared `batch_number` is written as `NULL`
- Added `DELETE /medicines/{id}` (returns `204`): a soft delete that locks the row (`SELECT … FOR UPDATE`) and refuses with `409` when any purchase order that has not been deleted has an item for the medicine, whatever the order's status. Its barcode stays locked after a delete
- A non-UUID id returns `400`, all four routes are behind the auth middleware, and there are still no role checks, so any signed-in user can update or delete
- Added migration `00023`, which attaches `set_updated_at()` to `medicines`. Without it `updated_at` never changes after insert, so apply it before deploying. Added `response.NoContent` and `apperror.ErrMedicineInPurchaseOrder`. The repository's `Transaction` helper is reused, and `ListFilter` lives in the repository and is aliased in the service
- Updated `context/go-standards.md`: pagination is offset-based (`limit` and `offset`), no longer cursor-based
- Swagger updated for all four operations, with `UpdateMedicineRequest` (no `quantity`), `MedicineResponse` and `MedicineListResponse`, and shared `401`/`404`/`500` responses
- 100% test coverage on the medicine handler, service, middleware and `response`; the repository is at 82.4%. Its SQL is checked with GORM in DryRun mode, which pins the SQL text (soft-delete scoping, `FOR UPDATE`, LIKE escaping, the columns an update writes) but does not run against a database. The migration and the queries have not been run against a live PostgreSQL yet
- A delete that commits first lets a waiting purchase order insert succeed against a soft-deleted medicine, so the future create-purchase-order feature must reject deleted medicines

## Inventory entry create, list and get endpoints

- Added `POST /inventory-entries`, `GET /inventory-entries` and `GET /inventory-entries/{id}` in `apps/api`, new `internal/{handler,service,repository}/inventory` packages layered the same way as `medicine`. No update or delete: per `context/database-schema.md`, `inventory_entries` is append-only, so correcting a mistake is a new offsetting entry, not an edit
- Create validates `medicine_id` (UUID, must resolve to an active medicine → `404`), `direction` (`addition`/`subtraction`), `quantity` (`> 0`), `reason` (required, free text, ≤100 chars — no closed enum, matching the DB's lack of a `CHECK`) and optional `notes` (≤500 chars). `counted_by` is taken from the auth token, never the body
- Added `medicine.Repository.LockQuantityByID` (a locked read of a medicine's quantity, additive alongside the existing `LockByID`) and `apperror.ErrInsufficientQuantity`. Inside one transaction, `Create` locks the medicine row, and a `subtraction` that would take `quantity` below zero is rejected with `409` before any insert — a new rule this feature introduces, since neither `medicines.quantity` nor `inventory_entries` has a DB constraint against going negative
- The insert relies on the existing `trg_inventory_entries_sync_quantity` trigger (`00018`) to update `medicines.quantity`; the service never writes it directly. No new migration was needed
- `GET /inventory-entries` only supports `limit`/`offset` for now (same rules as `GET /medicines`), with no `medicine_id` filter yet
- 100% test coverage on the inventory handler and service; the repository is at 76.3% (thin GORM passthroughs, checked with GORM DryRun SQL tests only, consistent with the medicine repository's 82.2%)
- Full spec: `context/features/08-inventory.spec.md`

## Supplier create, list, get, update and delete endpoints

- Added `POST /suppliers`, `GET /suppliers`, `GET /suppliers/{id}`, `PUT /suppliers/{id}` and `DELETE /suppliers/{id}` in `apps/api`, new `internal/{handler,service,repository}/supplier` packages layered like `medicine`, all behind the auth middleware (no role checks yet)
- Suppliers have no uniqueness rule (the table has no `UNIQUE` beyond the primary key), so create and update do no duplicate checks and two suppliers can share a name, email or phone. No new migration: `suppliers` is already soft-deletable and has its `updated_at` trigger (`00020`)
- Create and update share one body and validation: `name` required (≤255), `contact_name` (≤255), `email` (≤255, valid address), `phone` (≤32), `address` (≤500), all trimmed, with empty optional fields stored as `NULL`. `PUT` is a full replacement, so an omitted optional field is cleared. The email check also requires the parsed address to equal the input, so a display-name form like `Jane <jane@x.com>` is rejected instead of being stored whole
- `GET /suppliers` uses the same `limit`/`offset` rules as `GET /medicines` and an optional `name` filter, a case-insensitive "contains" match with LIKE wildcards escaped. There is no filter on `contact_name`, `email` or `phone` yet
- `DELETE /suppliers/{id}` is a soft delete that locks the row (`SELECT … FOR UPDATE`) and returns `409` (`apperror.ErrSupplierInPurchaseOrder`, wrapping `ErrConflict`) while any purchase order that has not been deleted references the supplier, whatever its status. The check is a direct `supplier_id` lookup on `purchase_orders`, with no join. As with medicines, a delete that commits first lets a waiting purchase order insert succeed against a soft-deleted supplier, so the future create-purchase-order feature must reject deleted suppliers
- `Update` has no existence pre-check: the repository's `apperror.ErrNotFound` (zero rows affected) passes straight through, since there is no duplicate error to mask
- Swagger updated: `suppliers` tag, five operations, `CreateSupplierRequest`, `UpdateSupplierRequest`, `Supplier`, `CreateSupplierResponse`, `SupplierResponse`, `SupplierListResponse`, and a shared `SupplierNotFound` response
- 100% test coverage on the supplier handler and service; the repository is at 58.7% (thin GORM passthroughs, with GORM DryRun SQL tests for the delete lock, the purchase-order lookup, LIKE escaping, `NULL` writes and the soft delete). The queries have not been run against a live PostgreSQL yet
- Full spec: `context/features/09-suppliers.spec.md`

## Purchase order create, list, get and receive endpoints

- Added `POST /purchase-orders`, `GET /purchase-orders`, `GET /purchase-orders/{id}` and `POST /purchase-orders/{id}/receive` in `apps/api`, new `internal/{handler,service,repository}/purchaseorder` packages layered like `medicine`, all behind the auth middleware (no role checks yet). No update, cancel or delete of an order and no editing of its items
- Create writes the order (status `draft`) and its items in one transaction and returns the order with `items` and an empty `receipts`. It requires `supplier_id`, `order_date` (past dates allowed) and 1–100 `items`, with optional `expected_date` (not before `order_date`) and `notes`. A medicine may appear once per order and each `quantity_ordered` is `1`–`2147483647`, so a bad item is reported by position (`items[1].quantity_ordered …`). `created_by` comes from the token, never the body
- Create locks the supplier and then each distinct medicine (in ascending ID order) before inserting, instead of only checking they exist. That closes the race the medicine and supplier delete features left open: a supplier or medicine deleted first is `404 supplier not found` / `404 medicine not found`, and a delete that races after a create is refused with `409`. The ascending order keeps concurrent creates that share medicines from deadlocking
- `GET /purchase-orders` uses the same `limit`/`offset` rules as `GET /medicines` and returns each order's header only. `GET /purchase-orders/{id}` returns the header, its `items` and its `receipts` (each with its items) in a constant number of queries. There are no list filters yet
- `POST /purchase-orders/{id}/receive` receives the whole order (returns `201` with the receipt; the body is optional and only takes `notes`): one receipt with one receipt item per order item, received equal to ordered and damaged and returned `0`, and the status set to `received`. Only a `draft` or `ordered` order can be received, otherwise `409`. It locks the order row, so two simultaneous receives can't both succeed, and inserts the receipt items in `medicine_id` order to keep concurrent receives of orders that share medicines from deadlocking. The service never writes `medicines.quantity`: the `00018` trigger adds each received quantity. `received_by` comes from the token
- This is the first feature to write to audited tables. The repository's `Transaction(ctx, actorID, fn)` runs `set_config('app.actor_id', …, true)` as its first statement and hands `fn` a purchase order, a medicine and a supplier repository bound to the same transaction, so every audit row the `00019` triggers write carries the caller as `actor_id`. Any future write to an audited table must go through a transaction that does the same
- Added `domain/purchase_order.go` and `apperror.ErrSupplierNotFound`, `ErrMedicineNotFound` (both wrap `ErrNotFound`, so the handler matches them before the bare `ErrNotFound`) and `ErrPurchaseOrderNotReceivable` (wraps `ErrConflict`). No new migration
- Swagger updated: `purchase-orders` tag, four operations, the request, item, receipt and response schemas, and a shared `PurchaseOrderNotFound` response
- 100% test coverage on the purchase order handler and service; the repository is at 47.6%, with GORM DryRun SQL tests for the actor `set_config`, the `FOR UPDATE` lock, active-only scoping, ordering and the status-only update (DryRun can't open a transaction, so `Transaction` is covered through its `setActor` statement)
- Also run against a real PostgreSQL 16 in a throwaway database at the service and repository layers (a temporary integration test, not kept): a full create, get, receive, get flow with stock updated; every audit row carrying its actor; six simultaneous receives giving one success; no deadlocks across 20 creates with medicines in opposite orders plus 20 receives; 15 supplier-delete-versus-create races leaving no live order on a deleted supplier. The HTTP layer has not been exercised end to end against a database
- Known limits: `GET /purchase-orders/{id}` reads in separate queries without a transaction, so a receive committing in between could return status `draft` with a receipt; receiving a line near `2147483647` into a medicine that already has stock overflows the column and returns a `500` (the receive rolls back cleanly); a purchase order can't be deleted, so a medicine or supplier that has ever been on one can no longer be deleted; receiving adds stock to the referenced `medicines` row, so a restock with a different batch or expiry inherits the old row's date
- Full spec: `context/features/10-purchase-orders.spec.md`

## App login page

- Made the index page (`/`) of `apps/app` the login page, matching the prototype: dotted background, "IM" logo and wordmark, a card with email and password fields, "Forgot password?", a full-width Sign in button, the staff-only note and the footer. Built with Chakra UI and the shared snippets (`PasswordInput`, `ColorModeButton`, `toaster`); no separate `/login` route
- The form calls a Server Action (`src/actions/auth.ts`) that posts to `${API_URL}${API_PREFIX}/login`. `API_URL` (`http://localhost:8080`) and `API_PREFIX` (`/api`) are read from the app's env, with a committed `apps/app/.env.example`; neither is `NEXT_PUBLIC_`, so the browser never sees the API address and there is no CORS. If `API_PREFIX` is unset, no prefix is used
- A `200` stores `access_token` and `refresh_token` in httpOnly, `sameSite=lax` cookies (`secure` in production, no expiry, so they last for the browser session) and the client redirects to `/home`. `400` and `401` show the API's `error` message and anything else a generic message, all as toasts. The action returns `{ success, data, error }` and never sends the tokens to the client
- Added `/home` as a placeholder page with a heading. The root layout now mounts `<Toaster />` (it had none) and defaults to dark mode; the `ColorModeButton` sits in the login card so light mode is reachable
- Added `BrandLogo` (`src/components/brand`) and the `ActionResult`, `LoginInput`, `TokenResponse` and `ErrorResponse` types
- Known limits: `/home` is not guarded, so it can be opened without logging in, and nothing reads or refreshes the tokens yet; "Forgot password?" points to `#` because no route exists; no unit tests for the action, and the pages were built and linted but not exercised in a browser
- Full spec: `context/features/11-app-login.spec.md`

## App home layout

- Guarded `/home` with `src/proxy.ts` (Next 16's replacement for `middleware.ts`): without both the `access_token` and `refresh_token` cookies the request is redirected to `/`. It only checks that the cookies exist, so an expired or forged token still gets through the guard and is rejected by the API on the next call. The cookie names moved into `src/lib/auth.ts` so the Server Action and the proxy share them
- Removed the "Forgot password?" link from the login page
- Built the `/home` shell in `src/app/home/layout.tsx`: a sticky top nav over a side nav and the page content, with no dashboard content (the page is a "Dashboard" heading). Responsive: from the `md` breakpoint the side nav is always visible, below it the nav sits behind a hamburger button in a drawer that closes when a link is tapped
- Side nav follows the prototype: a "Workspace" section with Dashboard, Medicines, Inventory Entries, Suppliers and Purchase Orders, the current page highlighted. Purchase Orders is collapsible with Create (`/purchase-orders/new`) and Receive (`/purchase-orders/receive`) below it; it starts open when the current path is under `/purchase-orders`. Items are defined in `src/lib/nav.ts`, where an item can have `children`
- Top nav has the app icon on the left (`public/favicon.ico`, falling back to `BrandLogo` if the image fails to load) and the avatar plus the color-mode button on the right. The API token carries no user name and there is no `/me` endpoint, so the avatar shows the first letter of the email, decoded from the `access_token` payload without verifying it (display only), or `?` if it can't be read
- Known limits: only `/home` exists, so every other nav link, including Create and Receive, is a 404 until those features are built; the Create and Receive routes were my choice, not in the spec; the avatar letter should become the real name once the API exposes it; no unit tests for `getEmailFromToken` and `getInitial`, and the layout was built and linted but not exercised in a browser
- Full spec: `context/features/12-home.spec.md`

## Medicines page

- `home/` moved into a route group, `src/app/(app)/`, so its layout (guard, side nav, top nav) is shared without changing any URLs: `(app)/layout.tsx` (renamed `HomeLayout` → `AppLayout`), `(app)/home/page.tsx`, and the new `(app)/medicines/page.tsx`. `proxy.ts`'s matcher now also covers `/medicines` and `/medicines/:path*`
- The page (`MedicinesPageClient`, a client component; `page.tsx` itself renders nothing else, per the "client-side data fetching only" rule in `ts-standards.md`) lists medicines in a table (Name, Barcode, Batch Number, Expiration Date, Quantity, Status, Actions) with an `EmptyState` when there are none, and paginates with Chakra's `Pagination` (Previous/Next only) driven by `MedicineListResponse.total` against `offset`/`limit` (fixed at 20), so the buttons disable correctly at the boundaries regardless of `data.length`
- `Status` (green/yellow/red) is computed client-side in `src/lib/medicine-status.ts` from `expiration_date`, since the API doesn't return one. The yellow threshold (30 days) mirrors the DB's `settings.warning_threshold_days` default — there's no `/settings` endpoint yet to read it from, so it's a hardcoded constant with a comment explaining why
- Create and update share one right-side drawer (`MedicineFormDrawer`): Name, Barcode, Batch Number, Expiration Date, plus Quantity on create only, since `PUT /medicines/{id}` ignores `quantity` in the body. Both post through `src/actions/medicines.ts` (`ActionResult<T>` Server Actions, bearer token forwarded from the `access_token` cookie) and surface `400`/`409` errors as a form-level alert rather than per-field messages, since the 409 messages are prose ("medicine with this barcode already exists"), not tied to a single field
- Delete uses a confirmation dialog (`role="alertdialog"`) that requires typing "delete" before the button enables. Every delete attempt (success, `404`, or `409`) closes the dialog, toasts the outcome, and refreshes the table from the server, rather than threading the HTTP status back through `ActionResult` to special-case each one
- Added `src/types/medicine.ts` mirroring the swagger schemas
- Known limits: no test runner is configured for `apps/app` yet (`nx run app:test` has no target), so `getMedicineStatus` has no unit test despite being the one function here with real branching logic; `name`/`barcode` list filters from the API are not exposed in the UI; not exercised in a browser
- Full spec: `context/features/13-medicines-page.spec.md`

## Inventory entries page

- List + create page for inventory entries (manual stock adjustments) at `/inventory-entries` in `apps/app` (`app/(app)/inventory-entries/page.tsx`), sharing the `(app)` route group's layout. `proxy.ts`'s matcher now also covers `/inventory-entries`, `/inventory-entries/:path*`
- `InventoryEntriesTable` lists Date, Medicine, Direction, Quantity, Reason, Notes, Counted By, with an `EmptyState` for no data and `Tag`-based direction badges (green for `addition`, red for `subtraction`). Pagination is the same `PaginationRoot`/`total`-vs-`offset`/`limit` pattern as the medicines page. There's no Actions column and no edit/delete: entries are append-only per the API, so a mistake is corrected by recording a new offsetting entry
- The list response only carries `medicine_id`, not a name, and there's no `GET /medicines/{id}`, so the table resolves names via a one-time bulk lookup (`listMedicines({ limit: 100, offset: 0 })`) mapped by id. Known limit: an entry referencing a medicine outside that first page (more than 100 active medicines, or one deleted since) falls back to showing the raw `medicine_id`
- The create drawer (`InventoryEntryFormDrawer`, "Record Stock Adjustment") posts to `POST /inventory-entries` via `src/actions/inventory-entries.ts`; `400`/`404`/`409` (e.g. insufficient quantity for a subtraction) all show as a form-level alert and keep the drawer open rather than closing, since these are create-time errors to correct and resubmit, not races to recover from
- The Medicine field is a searchable `Combobox` (`MedicineCombobox`, new), not a plain select populated from the bulk lookup: each keystroke (debounced ~300ms) searches `GET /medicines?name=` through `listMedicines`, which now takes an optional `name` param. This means the create form can find any medicine regardless of how many exist, even though the table's lookup is still capped at 100. Built with Chakra v3's `Combobox` + `useListCollection` (`set()` to swap in server results each search) — checked against Context7 and the installed `@ark-ui/react`/`@zag-js/combobox` types, since Context7's indexed docs describe the async-loading pattern but don't surface its full code sample
- API change: `apps/api`'s inventory entry endpoints (`GET /inventory-entries`, `GET /inventory-entries/{id}`, `POST /inventory-entries`) now return `counted_by` as `{ id, name }` instead of a bare UUID. `domain.InventoryEntry.CountedBy` became `InventoryEntryUser{ID, Name}`; the repository's reads `LEFT JOIN` `inventory_entries.counted_by` to `users.id` (not `Unscoped()` — a plain LEFT JOIN, so a soft-deleted user's name still resolves for historical entries), and `Create` reloads the row through the same joined query right after inserting so its response also carries the name. Swagger's `InventoryEntry.counted_by` now `$ref`s a new `InventoryEntryCountedBy` schema
- Repository test coverage for `inventory` is 76.2% (DryRun SQL tests updated for the join and the reload-after-create); handler and service stay at 100%
- Known limits: no test runner is configured for `apps/app` yet, so `MedicineCombobox` has no unit test; not exercised in a browser
- Full spec: `context/features/14-inventory-page.spec.md`

## Suppliers page

- CRUD page for suppliers at `/suppliers` in `apps/app` (`app/(app)/suppliers/page.tsx`), sharing the `(app)` route group's layout. `proxy.ts`'s matcher now also covers `/suppliers`, `/suppliers/:path*`
- `SuppliersTable` lists Name, Contact Name, Email, Phone, Address, Actions, with an `EmptyState` for no data and the same `PaginationRoot`/`total`-vs-`offset`/`limit` pattern as the medicines page. Null optional fields show an em dash
- Create and update share one right-side drawer (`SupplierFormDrawer`): Name, Contact Name, Email, Phone, Address — all five fields on both forms, unlike the medicines drawer's create-only Quantity, since `PUT /suppliers/{id}` is a full replace and an omitted field would be cleared rather than left alone
- Delete uses the same type-"delete"-to-confirm dialog pattern as medicines (`DeleteSupplierDialog`); every outcome (success, `404`, `409` for a supplier used in a purchase order) closes the dialog, toasts, and refreshes the table
- Added `src/actions/suppliers.ts` (`ActionResult<T>` Server Actions: list/create/update/delete) and `src/types/supplier.ts` mirroring the swagger schemas
- Known limits: no test runner is configured for `apps/app` yet; the `name` list filter from the API is not exposed in the UI; not exercised in a browser
- Full spec: `context/features/15-suppliers.spec.md`

## Purchase orders page

- List, create, view-detail and receive pages for purchase orders in `apps/app`, wired to the existing `apps/api` endpoints (`GET/POST /purchase-orders`, `GET /purchase-orders/{id}`, `POST /purchase-orders/{id}/receive`). There is no update or delete: the API doesn't support them yet
- Unlike the drawer-based medicines/suppliers pages, Create (`/purchase-orders/new`) and Receive (`/purchase-orders/receive`) are full pages, since `NAV_ITEMS` in `src/lib/nav.ts` already routes their sublinks there directly. Get one is also a full page (`/purchase-orders/[id]`) — a PO's items and receipts don't fit a drawer well
- `PurchaseOrder`/`PurchaseOrderItem` only carry `supplier_id`/`medicine_id`, not joined names, so two bulk lookups (`useSupplierLookup`, `useMedicineLookup`) resolve them client-side, same pattern and 100-row limit as the existing medicine lookup on the inventory entries page. The create page's item picker instead reuses the existing type-to-search `MedicineCombobox`; a new `SupplierCombobox` mirrors it for the supplier field
- The create page's Items section is a dynamic list (1–100 rows, each a medicine combobox + quantity), blocking a medicine from being selected twice across rows client-side, with the API's `items[i].medicine_id is repeated` as the backstop. On success it redirects to the new order's detail page
- The receive page lists purchase orders with a per-row Receive action (disabled unless `status` is `draft`/`ordered`) and also accepts a `?id=` query param — used by the detail page's "Receive" link — to jump straight to the confirmation dialog, which fetches that order's detail for the item count/supplier before confirming
- Fixed a pre-existing gap in `NavGroup.tsx` (shared by the desktop side nav and mobile drawer nav): a nav item with `children` rendered its whole label as a `Collapsible.Trigger`, so clicking it only toggled the submenu and never navigated — invisible until "Purchase Orders" became the first `NAV_ITEMS` entry with children. Split it into a real link (to `item.href`) plus a separate chevron button that toggles the submenu
- Added `src/actions/purchase-orders.ts` and `src/types/purchase-order.ts` mirroring the swagger schemas; widened `proxy.ts`'s matcher to cover `/purchase-orders`, `/purchase-orders/:path*`
- Known limits: no test runner is configured for `apps/app` yet; the bulk lookups' 100-row cap means a PO referencing a supplier/medicine outside that set falls back to showing the raw id; only full-order receiving exists (matches the API), so there's no partial/damaged/returned UI
- Full spec: `context/features/16-po.spec.md`

## Medicine barcode scanner page

- Added a public `/scanner` page in `apps/app` that reads a medicine's barcode via the webcam and looks it up through `GET /medicines/barcode/{barcode}`, showing its details (Name, Barcode, Batch Number, Expiration Date, Quantity, Status) plus a redrawn barcode as visual confirmation. Camera-only input, no manual/typed entry
- Camera decoding uses `react-barcode-scanner` (native Barcode Detection API + WASM polyfill fallback), restricted to the 1D formats medicine barcodes use (`code_128`, `ean_13`, `ean_8`, `upc_a`, `upc_e`). Rendering the matched barcode back uses the unrelated `react-barcode` (wraps JsBarcode), format `CODE128` since barcode values are arbitrary text, not a checksummed numeric format
- No "Scan Again" button: after a lookup settles, scanning automatically re-arms itself once `NEXT_PUBLIC_SCANNER_COOLDOWN_SECONDS` (new env var, default 5s) elapses. Must be `NEXT_PUBLIC_`-prefixed since the timer runs client-side in `ScannerPageClient`. Camera view and result card render side by side
- Both the page and the API endpoint are intentionally public (no login required), a mid-implementation decision, not the original plan: `GET /medicines/barcode/{barcode}` is registered in `apps/api`'s `main.go` without the `auth(...)` wrapper, and `/scanner` lives outside the `(app)` guarded route group in `apps/app` (its own top-level route with a minimal header instead of `SideNav`/`TopNav`), removed from `proxy.ts`'s matcher
- Removing the `auth(...)` wrapper alone did not make the endpoint public: `GetByBarcode` also had its own `h.caller` fail-closed guard (a pattern shared by every `medicine` handler) that 401s independently whenever there's no authenticated account in request context, caught only by testing with `curl` directly against the running server. Removed that guard from `GetByBarcode` specifically (it never used the caller's identity), dropped it from the `TestNoCallerInContext` table, and added `TestGetByBarcode_NoCallerInContext` asserting it succeeds with no auth context
- Added `getMedicineByBarcode` to `src/actions/medicines.ts`; no new type, reuses `Medicine`
- Known limits: a camera error (permission denied, unsupported browser, no camera) leaves the page with no way to look anything up, since there's no manual entry fallback; not exercised in a real browser with a live camera
- Full spec: `context/features/17-scanner.spec.md`
