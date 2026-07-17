# m46 — Postgres + Key Value `updatedAt`/`region`/`dashboardUrl` on GraphQL and MCP

**Status:** DONE 2026-07-16

## Problem

ADR018's w9/m41 audit found that `updatedAt`, `region`, and `dashboardUrl` were
present on the REST wire shape for Postgres and Key Value (via the `renderPostgres`
/ `renderKeyValue` wrapper structs) but absent from GraphQL and MCP, which read
directly off `PostgresView` / `KeyValueView`. The `apps` package had already solved
this with the `Service.view()` pattern, but the two datastore packages hadn't.

## What shipped

- **`PostgresView`** gains `Region string` + `DashboardURL string` fields.
- **`KeyValueView`** gains `Region string` + `DashboardURL string` fields.
- **`postgres.Service.view(d)`** wrapper populates them from
  `s.Metadata.PlatformRegion()` / `s.Metadata.DashboardURL(PostgresDashboardRoute, id)`.
- **`keyvalue.Service.view(kv)`** wrapper populates them the same way.
- All 7 `pgView(...)` call sites in `postgres/service.go` replaced with `s.view(...)`.
- All 8 `kvView(...)` call sites in `keyvalue/service.go` replaced with `s.view(...)`.
- **GraphQL** (`postgres/graphql.go`, `keyvalue/graphql.go`): `updatedAt`, `region`,
  `dashboardUrl` fields added to both type definitions.
- **REST** (`postgres/rest.go`, `keyvalue/rest.go`): simplified — `Region`/`DashboardURL`
  now come from the View, so the REST wrapper no longer needs to re-populate them.
- **ADR018** gap-backlog row added documenting this closure.

## Commit

`5d81a284 feat(postgres,keyvalue): updatedAt/region/dashboardUrl on GraphQL and MCP (w2/m46)`
