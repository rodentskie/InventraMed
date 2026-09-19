# Create Medicine

## Overview

Add an endpoint to `apps/api` that creates a new medicine. This spec covers **create only** — list, get, update and delete are separate features.

Supporting work, in order:

1. A migration adding a unique index on name + batch number
2. An auth middleware that validates the JWT from the `Authorization` header
3. The create-medicine endpoint, protected by that middleware

The `medicines` table already exists (`apps/migration/transactions/00007_create_medicines_table.sql`) and is empty, so the index can be added without cleaning up data.

This will be done in `apps/api`, plus one new file under `apps/migration/transactions/`.

---

## Requirements

Layered per @context/go-standards.md — Handler → Service → Repository, each depending on the layer below via interfaces.

### 1. Migration

New file `apps/migration/transactions/00022_create_medicines_name_batch_unique_index.sql`, following the conventions in @context/go-standards.md (SQL format, Up and Down). Do not edit `00007`.

```sql
-- +goose Up
CREATE UNIQUE INDEX uq_medicines_name_batch
    ON medicines (lower(name), coalesce(lower(batch_number), ''))
    WHERE deleted_at IS NULL;

-- +goose Down
DROP INDEX uq_medicines_name_batch;
```

- Makes the name + batch rule a database guarantee, so two concurrent requests can't both insert. It is the same rule as the service check: **case-insensitive**, ignores soft-deleted rows, and a missing batch counts as the empty string, so two rows with no batch and the same name conflict
- `batch_number` is stored as `NULL` when empty (the handler trims and normalizes it), so the `coalesce` only matters for `NULL`
- The `barcode` `UNIQUE` from `00007` is untouched; Postgres names it `medicines_barcode_key`
- Run it through the `migration` app CLI, up and down, to confirm both directions work. Never auto-run on startup

### 2. Auth middleware (`internal/middleware/auth.go`)

Validates the access token issued by `POST /login` and puts the caller's identity in the request context.

**Header format**

```
Authorization: Bearer <token>
```

- The scheme is matched case-insensitively (`Bearer`, `bearer`); the token is everything after the first space, trimmed, and must be non-empty

**Behavior**

- `Auth(secret []byte, log *zap.Logger) func(http.HandlerFunc) http.HandlerFunc` — takes the same secret `login` signs with (`JWT_SECRET`, already in config; no new config)
- Apply it per route by wrapping the handler: `r.Handle("POST /medicines", auth(medicineHandler.Create))`. The router does not change
- Validate with `jwt.ParseAccessToken[domain.AccountPayload]` (`github.com/rodentskiedev/go-libraries/lib/jwt` v0.1.1). It checks the signature, the HMAC signing method, expiry, and that `token_type` is `access`, so a **refresh token is rejected**
- Also reject a token whose payload `user_id` is empty
- On success, store the `domain.AccountPayload` in the request context and call the next handler
- Expose `AccountFromContext(ctx context.Context) (domain.AccountPayload, bool)` to read it back. The context key is an unexported type
- Validation is stateless — the database is not consulted. A token stays valid until it expires even if its user has since been deleted; this is accepted

**Failure** — every failure returns the same response, so the caller can't tell a missing token from a bad one:

- Status `401`, body `{"error": "unauthorized"}`, header `WWW-Authenticate: Bearer`
- Applies to: missing header, wrong scheme, empty token, malformed token, bad signature, expired token, refresh token, empty `user_id`
- Write it with the `response` helpers. Never log the token; only log a failure to write the response

**Out of scope:** role/permission checks (RBAC) — a later `permission.go` middleware. This one only proves who the caller is.

### 3. Create medicine

#### Domain (`internal/domain`)

- `Medicine` type: `ID`, `Name`, `Barcode`, `BatchNumber` (optional), `ExpirationDate`, `Quantity`, `CreatedBy` (optional), `CreatedAt`, `UpdatedAt`
- `ExpirationDate` is a date, not a timestamp

#### Repository (`internal/repository/medicine`)

- GORM, private `record` struct mapped to `medicines` via `TableName()`, following the soft-delete convention in @context/go-standards.md: `DeletedAt gorm.DeletedAt`, no `gorm.Model`
- `ExistsByNameAndBatch(ctx, name, batchNumber *string) (bool, error)` — **case-insensitive**: compare `lower(name) = lower(?)` and `lower(batch_number) = lower(?)`, so `Paracetamol` / `paracetamol` and `b-001` / `B-001` are duplicates. Soft-deleted rows are excluded automatically by `gorm.DeletedAt`. When `batchNumber` is nil, match rows whose `batch_number IS NULL` (a plain `=` never matches NULL). The medicine is stored with the casing the caller sent
- `ExistsByBarcode(ctx, barcode) (bool, error)` — exact match (barcodes are not case-folded). Must use `Unscoped()` to include soft-deleted rows: a soft-deleted medicine keeps its barcode locked, which matches the table-wide `UNIQUE` on `barcode`
- `Create(ctx, *domain.Medicine) (*domain.Medicine, error)` — `id`, `created_at` and `updated_at` come from the database defaults; return the persisted row. The pre-checks give a clean error in the normal case; the unique indexes catch concurrent requests. Translate a unique violation (SQLSTATE `23505`) by its constraint name: `medicines_barcode_key` → `apperror.ErrBarcodeExists`, `uq_medicines_name_batch` → `apperror.ErrNameBatchExists`. Any other error is wrapped and returned as is
- Keep that translation in a small pure function so it can be unit tested without a database
- Maps to `domain` types at the package boundary — `record` is never exposed

#### Service (`internal/service/medicine`)

`Create(ctx, CreateInput) (*domain.Medicine, error)`, where `CreateInput` carries the validated fields plus `CreatedBy` (the caller's user ID):

1. Check name + batch number → if it exists, return a conflict error
2. Check barcode → if it exists, return a conflict error
3. Insert and return the new medicine

- Run the checks and the insert in one GORM transaction
- The two conflict errors, `apperror.ErrNameBatchExists` and `apperror.ErrBarcodeExists`, are added to `apps/api/pkg/apperror`. Each wraps `apperror.ErrConflict` (so `errors.Is(err, apperror.ErrConflict)` still works), and they live there because the repository, service and handler all need them and a handler may not import the repository. If both checks would fail, name + batch is reported (it runs first)
- Errors from the repository's insert (the concurrent-request case) pass through unchanged so they reach the handler as the same two errors
- Wrap repository errors with `fmt.Errorf("create medicine: %w", err)`

#### Handler (`internal/handler/medicine`)

Read the caller from `middleware.AccountFromContext` (fail closed with `401 unauthorized` if it is missing, i.e. the route was registered without the middleware), decode JSON, validate, call the service with `CreatedBy` set to the caller's `UserID`, map errors, respond with the `response` helpers only. The client cannot set `created_by`; a `created_by` field in the body is ignored.

**Validation** — trim all string fields first. Any failure returns `400` with the first message that applies:

| Field             | Rule                                                              | Message                                   |
|-------------------|-------------------------------------------------------------------|-------------------------------------------|
| body              | Must be valid JSON, max 1 MiB                                     | `invalid request body`                    |
| `name`            | Required, non-empty, max 255 chars                                | `name is required` / `name is too long`   |
| `barcode`         | Required, non-empty, max 128 chars. Supplied by the client — the scanner reads it, so trimming also strips stray whitespace or a trailing newline | `barcode is required` / `barcode is too long` |
| `batch_number`    | Optional, max 64 chars. Empty or missing is stored as `NULL`      | `batch_number is too long`                |
| `expiration_date` | Required, format `YYYY-MM-DD`. Past dates are allowed — expired stock must still be registrable (it maps to the red LED status) | `expiration_date is required` / `expiration_date must be a valid date in YYYY-MM-DD format` |
| `quantity`        | Required, integer, `>= 0`. The starting stock count, stored as given (no `inventory_entries` row is written for it). Use `*int` so a missing value is not mistaken for `0` | `quantity is required` / `quantity must be zero or greater` |

**Error mapping** (handler only):

| Error                          | Status | Message                                               |
|--------------------------------|--------|-------------------------------------------------------|
| no caller in context           | `401`  | `unauthorized`                                        |
| name + batch conflict          | `409`  | `medicine with this name and batch number already exists` |
| barcode conflict               | `409`  | `medicine with this barcode already exists`           |
| anything else                  | `500`  | `internal server error` (log the real error, never return it) |

### 4. Wiring

In `apps/api/cmd/api/main.go`, build the middleware once from `cfg.JWTSecret` and register `POST /medicines` through the prefix wrapper with the handler wrapped → served at `POST /api/medicines` with the default prefix. `POST /login`, `GET /` and the swagger routes stay public.

### 5. Swagger

Update `apps/api/internal/handler/swagger/openapi.json` once the endpoint works:

- Add a `medicines` tag
- Add a `bearerAuth` security scheme under `components.securitySchemes` (`type: http`, `scheme: bearer`, `bearerFormat: JWT`) and reference it on `/medicines` only, via `security: [{"bearerAuth": []}]`. `/login`, `/` and the swagger routes remain unauthenticated
- Add `/medicines` with a `post` operation (`operationId: createMedicine`), the request body, and the `201`, `400`, `401`, `409` and `500` responses, each with an `example`
- Add schemas `CreateMedicineRequest`, `Medicine` and `CreateMedicineResponse`; reuse the existing `ErrorResponse`
- The file must remain valid JSON and `apps/api/internal/handler/swagger/handler_test.go` must still pass

### 6. Tests

- **Middleware** (build tokens with the `jwt` lib): valid access token calls next with the payload in context; missing header; wrong scheme (`Basic`); `Bearer` with empty token; malformed token; wrong secret; expired token; refresh token; empty `user_id`; lowercase `bearer` accepted; every failure returns the `401` body plus `WWW-Authenticate` and never calls next; `AccountFromContext` on an empty context returns false
- **Service** (mocked repository): success, name + batch exists, barcode exists, both exist (name + batch wins), null batch number is passed through, `CreatedBy` reaches the repository, each repository error path (both checks and the insert), each conflict error from the insert (concurrent request) passes through unchanged
- **Repository error translation** (the pure function only): a `23505` on `medicines_barcode_key` → `ErrBarcodeExists`, on `uq_medicines_name_batch` → `ErrNameBatchExists`, a `23505` on any other constraint and a non-Postgres error → returned wrapped, not as a conflict
- **Handler** (mocked service): `201`, every `400` case in the validation table, no caller in context → `401`, both `409` cases, `500`, `created_by` in the body is ignored
- The rest of the repository is thin GORM calls — skip unit tests per the standards
- Coverage ≥ 90%. `cover: true` is already set on the `api` test target — verify it stays

---

## Endpoint

`POST /medicines` → served at `POST /api/medicines` with the default prefix. Requires `Authorization: Bearer <access_token>`.

Request:

```json
{
  "name": "Paracetamol 500mg",
  "barcode": "8901234567890",
  "batch_number": "B2026-001",
  "expiration_date": "2027-03-31",
  "quantity": 120
}
```

`201 Created`:

```json
{
  "message": "medicine created",
  "data": {
    "id": "0b8f3c62-6a1e-4c3e-9d0e-5f1d2a7c9e11",
    "name": "Paracetamol 500mg",
    "barcode": "8901234567890",
    "batch_number": "B2026-001",
    "expiration_date": "2027-03-31",
    "quantity": 120,
    "created_by": "7d2f4a10-3b5c-4e8a-a1f6-9c0b2d4e6f88",
    "created_at": "2026-09-20T08:15:30Z",
    "updated_at": "2026-09-20T08:15:30Z"
  }
}
```

Errors:

| Status | When                                            | Body                                                                   |
|--------|-------------------------------------------------|------------------------------------------------------------------------|
| 400    | Malformed JSON, missing/invalid fields          | `{"error": "<validation message>"}`                                    |
| 401    | Missing, malformed, expired or non-access token | `{"error": "unauthorized"}`                                            |
| 409    | Same `name` + `batch_number` already exists     | `{"error": "medicine with this name and batch number already exists"}` |
| 409    | `barcode` already exists                        | `{"error": "medicine with this barcode already exists"}`               |
| 500    | Anything unexpected                             | `{"error": "internal server error"}`                                   |

---

## Open Questions

None outstanding.

---

## References

- @context/go-standards.md
- @context/features/03-api.spec.md
- @context/features/04-api-login.spec.md
- @context/features/05-swagger.spec.md
- `apps/migration/transactions/00007_create_medicines_table.sql`
- `apps/api/pkg/apperror/apperror.go`
- `apps/api/internal/handler/swagger/openapi.json`
- `github.com/rodentskiedev/go-libraries/lib/jwt` v0.1.1 — `ParseAccessToken`
