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
