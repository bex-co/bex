# w34 · m589 — Multi-resource datastore metrics and datastore `instance`/`percentage` parity

**Worker:** worker34 **Goal:** accept repeatable `resource` IDs on `GET /v1/metrics/{disk-usage,disk-capacity,active-connections,replication-lag}` and support `instance` filtering (and `percentage` normalization consistent with services) on datastore `cpu`/`memory`/`cpu-limit`/`memory-limit` across REST, GraphQL, and MCP. **Status:** todo

## Tasks (in order)

| id   | title                                                                                                                                             | est   | depends_on |
| ---- | ------------------------------------------------------------------------------------------------------------------------------------------------- | ----- | ---------- |
| t001 | Support `instance` filtering and `percentage` normalization on datastore `cpu`/`memory` (and `instance` on `cpu-limit`/`memory-limit`)            | 1h15m | —          |
| t002 | Support repeatable `resource` arrays (with per-resource authz, `checkFanOut`, mixed kinds on disk routes) on datastore REST metrics endpoints     | 1h    | —          |
| t003 | Align MCP `get_metrics` and GraphQL `metrics` with the updated datastore core and update `ADR018` / `ADR010`                                      | 45m   | t001, t002 |
| t004 | Render parity                                                                                                                                     | 20m   | t003       |
| t005 | Simplify                                                                                                                                          | 20m   | t004       |
| t006 | Test coverage                                                                                                                                     | 30m   | t004, t005 |
| t007 | Closeout                                                                                                                                          | 15m   | t006       |

## Definition of done

- `GET /v1/metrics/cpu?resource=dpg-…&instance=…`, `GET /v1/metrics/memory?resource=dpg-…&percentage=true`, and `cpu-limit`/`memory-limit` (for Postgres `dpg-` and Key Value `red-`) return filtered/normalized series on REST and MCP `get_metrics` instead of `400`, joining usage against `DatastoreResource` limit history via the shared `normalizeUsageToPercentage` helper.
- `GET /v1/metrics/disk-usage`, `/v1/metrics/disk-capacity`, `/v1/metrics/active-connections`, and `/v1/metrics/replication-lag` accept repeatable `resource` (and deprecated `service` where declared) params, returning merged resource-labeled series while enforcing per-resource `canView` authorization and `checkFanOut` (including mixed `srv-`/`dpg-`/`red-` on `disk-usage` and `disk-capacity`).
- `go test -race ./internal/metrics ./internal/api` and `make lint` pass.

## Source + Goal linkage

- **Source:** `/pm-brainstorm` (hosting metrics backend, 2026-11-23), verified against `lego/backend/internal/metrics/rest.go:167-186,374-395`, `datastore.go:250-519`, and `lego/backend/internal/api/openapi/render-public-api-1.json`.
- **Goal linkage:** Render parity first (`docs/ADR008-vision.md#render-parity-first`) — closes `Metrics → Core metrics` and `Extended metrics` multi-resource and datastore `instance` parity gaps in `docs/ADR018-render-parity.md`.
- **Expected outcome:** Monitoring scripts, agents, and dashboards querying multiple datastores in one call or inspecting an HA standby / read replica's CPU and memory by `instance` (or `percentage`) work without hitting artificial `400` refusals.
- **Why now:** Multi-pod Postgres (HA + read replicas, `w6/m467`) already emits per-pod series in Prometheus, and Render's pinned OpenAPI already declares repeatable `resource` on all four datastore endpoints and `instance` on datastore `cpu`/`memory`/`*-limit`. Note: `percentage=true` on REST is bex's own cross-surface extension (Render's REST has no `percentage` param, `render_openapi.go:292`); enabling it on datastores keeps REST/MCP/GraphQL consistent with services. Render parity closing task (`t004`) is included because this changes REST, GraphQL, and MCP surfaces.
