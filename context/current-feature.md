# Current Feature: Migration

## Status

In Progress

## Goals

- Build the `apps/migration` app to handle GORM schema migrations via Goose
- Put all `.sql` migration files in `transactions/`
- Create migrations for the full schema in @context/database-schema.md:
  - RBAC: `users`, `roles`, `policies`, `user_roles` (join), `role_policies` (join)
  - Medicine inventory: `medicines`, `settings`
- Seed data:
  - `settings` — single row with `warning_threshold_days` default `30`
  - RBAC roles: `admin`, `standard`
  - RBAC policies:
    - `admin` — allow all (`*`/`*`)
    - `standard` — cannot CRUD RBAC-related tables (`users`, `roles`, `policies`, `user_roles`, `role_policies`) or `settings`
- Add `deleted_at` (soft delete) to every table where applicable, per GORM conventions

## Notes

- Stack: GORM (models) + Goose (migrations), PostgreSQL, UUID PKs via `gen_random_uuid()`
- App already scaffolded at `apps/migration` (`cmd/`, `config/`, `transactions/`, `go.mod`, `project.json`) — not yet committed
- RBAC evaluation is IAM-style: default deny, explicit deny always wins, `action`/`resource` support trailing `*` wildcard
- Expiration status (green/yellow/red) is derived at read time, not stored on `medicines`
- Open questions from schema doc (not blocking migration work, flag if relevant):
  - Whether `policies.resource` needs per-record scoping beyond `*`
  - Additional medicine fields (manufacturer, unit of measure, storage location)

## References

- @context/database-schema.md
- @context/features/01-migrations.spec.md
