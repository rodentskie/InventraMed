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
