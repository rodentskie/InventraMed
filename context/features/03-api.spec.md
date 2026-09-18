# API App

## Overview

The `api` application is the entry point. It exposes a REST API and swagger documentation.

This will be done in `apps/api`.

---

## Requirements for phase 1

- Use Go 1.27+ standard library `net/http` — no third-party router
- Follow the layered architecture: Handler → Service → Repository
- Each layer must depend on the layer below via interfaces, never concrete types
- Use `github.com/rodentskiedev/go-libraries/lib/env` for all ENV loading
- Use `github.com/rodentskiedev/go-libraries/lib/logger` for all logging — never `log` or `fmt` for logs
- Use GORM for all database access — never raw SQL
- Never use `gorm.Model` — define models manually without `deleted_at`

---

## Endpoint

Initial `/` endpoint, return just greeting message of `inventramed` REST API.

## References

- @context/go-standards.md