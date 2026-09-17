# Migration

## Overview

The migration application which will handle `GORM` schema migrations.
This will be done in `apps/migrations`.

## Requirements

- Use `GORM` and `Goose`
- Put all `.sql` files in `transactions/`.
- Look into the schema and create migrations.
    - seed data for settings
    - seed data for `RBAC`
        - roles: admin, standard
        - policy:
            - admin can do all
            - standard, cannot view do CRUD to rbac related tables and settings
- Look into context/database-schema.md, need to add deleted_at field to table which it is applicable, GORM will handle the `soft deletion`.

## References

- @context/database-schema.md