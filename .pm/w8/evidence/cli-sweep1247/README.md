# CLI sweep 1247 — restart routing and free-Postgres lifecycle fields (read-only, 2026-10-20)

Read-only: metadata reads of existing free databases, plus source reads. Nothing was created or mutated. Filed as [w8/145](../../145.md).

- The pinned `bex restart <id>` routes service, Postgres and Key Value ids through one resource service (`pkg/tui/views/restartview.go:27–60`). The Postgres false-success path is already tracked as w8/084.
- Free Postgres objects carry no `expiresAt` (Render's “applies to free tier databases only” field), and no TTL sweeper exists in the backend or operator, despite ADR009:148's 30-day TTL.
