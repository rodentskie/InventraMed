# History

## Migration app with RBAC and medicine inventory schema

- Built `apps/migration` (GORM + Goose) with schema migrations for RBAC (`users`, `roles`, `policies`, `user_roles`, `role_policies`) and medicine inventory (`medicines`, `settings`)
- Added `deleted_at` soft-delete columns where applicable
- Seeded default `settings` row and `admin`/`standard` roles with corresponding IAM-style policies
