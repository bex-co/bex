# m50 — `databaseLogs` GraphQL type exposes `instance`/`type` fields

**Status:** DONE 2026-07-16

## Problem

`databaseLogGQLType` (the GraphQL object returned by the `databaseLogs` query
in `postgres/graphql.go`) only exposed `timestamp` and `message`. The comment
above the type said it exposed `(service/instance/type)`, but the `Fields` block
only had two resolvers.

REST (`GET /v1/postgres/{id}/logs`) and MCP (`get_postgres_logs`) already
returned `instance` (the CNPG pod name) and `type` (`"postgres"`) via
`DatabaseLogEntry.Labels` — the data was always there, just not wired into the
GraphQL type.

This meant GraphQL callers who needed to know which CNPG pod (primary vs standby)
produced a log line had no way to see it, despite the `instance` filter argument
being present.

## What shipped

- **`databaseLogGQLType`** (`postgres/graphql.go`): two resolvers added —
  `"instance"` reading `e.Labels["instance"]` and `"type"` reading
  `e.Labels["type"]`, matching the pattern in `logs/graphql.go:35-36`.
- **`TestGQLDatabaseLogEntryExposesInstanceAndType`** (`postgres/logs_test.go`):
  drives the real GraphQL schema end-to-end (seeds a pod, reads its log via
  `graphql.Do`), confirms `instance` and `type` both resolve to expected values.
  Added `"github.com/graphql-go/graphql"` import to the test file.
- **ADR018** "Postgres log history" row updated with `w2/m50` note; gap-backlog
  row added.

## Commit

`fix(postgres): expose instance and type fields on databaseLogs GraphQL type (w2/m50)`
