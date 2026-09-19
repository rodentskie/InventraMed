# API Login

## Overview

Add a login endpoint to `apps/api` that authenticates a user by email + password and returns a signed JWT.

Supporting work, in order:

1. A new shared Go library, `hash`, wrapping bcrypt
2. A configurable route prefix for all API endpoints, with an exemption mechanism
3. The login endpoint itself

This will be done in `apps/api`, plus a new library under `libs/go/hash`.

---

## Requirements

### 1. `hash` library

- Generate with the existing script: `APP=hash yarn run nx-go:lib-generate` (→ `libs/go/hash`)
- Use `golang.org/x/crypto/bcrypt` — no other hashing dependency
- Public API (package `hash`):
  - `Hash(password string) (string, error)` — returns the bcrypt hash of `password` at `bcrypt.DefaultCost` (10, same cost as the seeded admin hash in `00021_seed_admin_user.sql`)
  - `Compare(hashed, plain string) error` — returns `nil` on match, an error on mismatch. Wrap `bcrypt.ErrMismatchedHashAndPassword` so callers can tell "wrong password" from other failures with `errors.Is`
- Add `"cover": true` to the `test` target in the lib's `project.json`
- Unit tests: hash → compare success, wrong password, malformed hash, empty password, and that two hashes of the same password differ (salted)

### 2. Route prefix

- All API endpoints are served under a configurable prefix, default `/api`
- Loaded in `apps/api/config/config.go` via `env.GetEnv("API_PREFIX", "/api")`
- Normalize the value: ensure a leading `/`, strip any trailing `/`. An empty value means "no prefix"
- Routes are registered through a small wrapper around `http.ServeMux` (not by hand-building patterns in `main.go`) so the prefix is applied in one place
  - Normal registration: `POST /login` → served at `POST {prefix}/login`
  - Exempt registration: served exactly as written, no prefix
  - The wrapper must handle patterns that start with a method (`"GET /"`) — prefix goes on the path part only
- Exempt routes for now:
  - `GET /` — the existing index/greeting endpoint stays at `/`
  - `/ping` is **not** implemented now, but adding it later must only take one exempt-registration line
- Update `apps/api/.env.example` with `API_PREFIX=/api`
- Unit tests for the wrapper: default prefix, custom prefix, empty prefix, normalization, exempt route unaffected, method-prefixed patterns

### 3. Login endpoint

Layered per @context/go-standards.md — Handler → Service → Repository, each depending on the layer below via interfaces.

**Repository** (`internal/repository/user`)
- GORM, **read only**, private `record` structs scoped to the tables it reads (`users`, plus `user_roles`/`roles` for role names). No `gorm.Model`
- `FindByEmail(ctx, email)` returns the user (id, email, name, password hash) and their role names, or `apperror.ErrNotFound`
- Maps to `domain` types at the package boundary — `record` is never exposed

**Service** (`internal/service/login`)
- Look up the user by email, verify with `hash.Compare`, then issue tokens with the `jwt` library
- Uses `github.com/rodentskiedev/go-libraries/lib/jwt` **v0.1.1** — `BuildTokenPair`
- Token payload type `T` (defined in the service/domain, not in the jwt lib):
  ```go
  type AccountPayload struct {
      UserID string   `json:"user_id"`
      Email  string   `json:"email"`
      Roles  []string `json:"roles"`
  }
  ```
- `subject` = the user's ID
- Unknown email and wrong password must return the **same** error (`apperror.ErrUnauthorized`) so the response doesn't reveal which emails exist. When the email isn't found, still run a `hash.Compare` against a dummy bcrypt hash so response time doesn't leak it either
- Never log the password or the tokens

**Handler** (`internal/handler/login`)
- Decode JSON body, validate: `email` required and a valid address, `password` required (non-empty). Trim/lowercase the email before lookup. Invalid → `400`
- Map errors to status codes here only: `ErrUnauthorized` → `401`, anything else → `500` with a generic message
- Write responses with the `response` helpers, never raw

**Config** (`apps/api/config/config.go`, all through `env.GetEnv`)

| Env var              | Default | Purpose                         |
|----------------------|---------|----------------------------------|
| `JWT_SECRET`         | —       | HMAC secret. Required — API fails to start if empty |
| `JWT_ACCESS_EXPIRY`  | `15m`   | Access token lifetime            |
| `JWT_REFRESH_EXPIRY` | `168h`  | Refresh token lifetime (7 days)  |

Add all three to `apps/api/.env.example`.

**Wiring**: register `POST /login` through the prefix wrapper (→ `POST /api/login` by default). Also connect to PostgreSQL via GORM on startup using `DATABASE_URL` (the config field exists but nothing uses it yet).

**Tests**: service (success, unknown email, wrong password, repository error, token contents/subject/roles) and handler (200, 400 invalid body/email/missing password, 401, 500) with mocked interfaces. Repository is a thin GORM read — skip unit tests per the standards. Coverage ≥ 90%.

---

## Endpoint

`POST /login` → served at `POST /api/login` with the default prefix

Request:

```json
{
  "email": "admin@local.com",
  "password": "secret"
}
```

`200 OK`:

```json
{
  "access_token": "<jwt>",
  "refresh_token": "<jwt>"
}
```

Errors:

| Status | When                                    | Body                                   |
|--------|-----------------------------------------|-----------------------------------------|
| 400    | Malformed JSON, missing/invalid fields  | `{"error": "<validation message>"}`     |
| 401    | Unknown email or wrong password         | `{"error": "invalid credentials"}`      |
| 500    | Anything unexpected                     | `{"error": "internal server error"}`    |

---

## Open Questions

- **Prefix vs. `/api/v1`** — @context/go-standards.md says to always version routes under `/api/v1/`, but this spec asks for `/api`. Since the prefix is an env var, `API_PREFIX=/api/v1` satisfies both. Default stays `/api` as requested unless told otherwise; the standard may need a matching update.
- **Refresh token** — `BuildTokenPair` always yields both tokens, so both are returned. There is no refresh endpoint in this spec; it would be a separate feature. If only one token is wanted now, use `SignToken` with an access `Claims` instead.
- **Error shape** — the existing `response` package only has `JSON(...)`. This spec uses `{"error": "..."}` for errors and adds a small error helper next to it; confirm that's the shape you want.
- **`hash` module path** — `apps/api` is module `apps/api`, so the generated lib will likely be `libs/go/hash`, which `go mod tidy` can't resolve from a proxy. Confirm during implementation how the nx-go generator + `go.work` wire this up, without hand-editing `go.mod` (per the Go standards).
- **jwt lib error granularity** — v0.1.1 returns plain-string errors, so expired vs. invalid can't be distinguished by callers. Not needed for login (signing only), but it will matter for the future auth middleware.
- **No rate limiting / lockout** — out of scope here, but login is a brute-force target; worth its own feature.

---

## References

- @context/go-standards.md
- @context/database-schema.md
- @context/features/03-api.spec.md
- `github.com/rodentskiedev/go-libraries/lib/jwt` v0.1.1 — https://github.com/rodentskiedev/go-libraries/releases/tag/lib%2Fjwt%2Fv0.1.1
