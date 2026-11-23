# w34 · m589 — Multi-resource datastore metrics and datastore `instance`/`percentage` parity

**Worker:** worker34 **Goal:** accept repeatable `resource` IDs on `GET /v1/metrics/{disk-usage,disk-capacity,active-connections,replication-lag}` and support `instance` filtering (and `percentage` normalization consistent with services) on datastore `cpu`/`memory`/`cpu-limit`/`memory-limit` across REST, GraphQL, and MCP. **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Support `instance` filtering and `percentage` normalization on datastore `cpu`/`memory` (and `instance` on `cpu-limit`/`memory-limit`) — **DONE** | 1h15m | — |
| [t002](done/t002.md) | Support repeatable `resource` arrays (with per-resource authz, `checkFanOut`, mixed kinds on disk routes) on datastore REST metrics endpoints — **DONE** | 1h | — |
| [t003](done/t003.md) | Align MCP `get_metrics` and GraphQL `metrics` with the updated datastore core and update `ADR018` / `ADR010` — **DONE** | 45m | t001, t002 |
| [t004](done/t004.md) | Render parity — **DONE** | 20m | t003 |
| [t005](done/t005.md) | Simplify — **DONE** | 20m | t004 |
| [t006](done/t006.md) | Test coverage — **DONE** | 30m | t004, t005 |
| [t007](done/t007.md) | Closeout — **DONE** | 15m | t006 |

## Outcome (2026-11-23)

- Added `DatastoreMetricQuery.Percentage` and shared `steppedLimitLookup` (`service.go`) so `DatastoreMetrics` normalizes datastore `cpu`, `memory`, and Key Value `kv_memory` against per-pod `DatastoreResource` limit history via `normalizeUsageToPercentage` before public instance-ID projection (returning `ErrMetricsUnavailable` when `DatastoreResource` is unwired and refusing `percentage` on limits/other datastore metrics with `400`), while stamping every returned series with its canonical `resource` label (`LabelResource`).
- Removed the `400` refusal on `q.Percentage || len(q.Instances) > 0` in `datastoreAppMetric` (`rest.go`), honoring `instance` filtering (canonical public instance IDs or raw pod names, empty `[]` on unresolved IDs, never broadening) and `ReplicaAggregate` (`aggregateAllMethod`) on datastore `cpu`, `memory`, `cpu-limit`, and `memory-limit` while still rejecting unsupported metrics/filters with `400`.
- Updated `parseDatastoreMetricParams` and unified `datastoreMetricQuery` / `activeConnectionsQuery` through `serveDatastoreResources` (`rest.go`) to accept repeatable `resource` (and `service` where declared) query params, enforce `checkFanOut(len(resources), 1, 1)`, infer `kind` per resource prefix (`dpg-`/`red-`/`srv-`) when omitted (enabling mixed kinds on `disk-usage`, `disk-capacity`, and `active-connections`), authorize each resource fail-closed, and return merged resource-labeled series (`[]` when empty).
- Dispatched `core.IsDatastoreResource` IDs through `datastoreAppMetric` in GraphQL `metrics` (`graphql.go`) alongside REST and MCP `get_metrics`, aligned `getMetricsArgs.Instance` schema copy (`mcp.go`), updated `docs/ADR018-render-parity.md` and `docs/ADR010-observability.md`, and verified with `go test -race ./internal/metrics`, `go test ./internal/api` + `-race` on `TestRenderMetrics|TestMCPParity|TestRenderConformance` (zero new `rendertest.Allowlist` entries), `make lint` (0 issues across all four modules + deadcode), `pm-board-validate.py`, and `docs-links-validate.sh`.

## Definition of done

- `GET /v1/metrics/cpu?resource=dpg-…&instance=…`, `GET /v1/metrics/memory?resource=dpg-…&percentage=true`, and `cpu-limit`/`memory-limit` (for Postgres `dpg-` and Key Value `red-`) return filtered/normalized series on REST and MCP `get_metrics` instead of `400`, joining usage against `DatastoreResource` limit history via the shared `normalizeUsageToPercentage` helper.
- `GET /v1/metrics/disk-usage`, `/v1/metrics/disk-capacity`, `/v1/metrics/active-connections`, and `/v1/metrics/replication-lag` accept repeatable `resource` (and deprecated `service` where declared) params, returning merged resource-labeled series while enforcing per-resource `canView` authorization and `checkFanOut` (including mixed `srv-`/`dpg-`/`red-` on `disk-usage` and `disk-capacity`).
- `go test -race ./internal/metrics ./internal/api` and `make lint` pass.

## Source + Goal linkage

- **Source:** `/pm-brainstorm` (hosting metrics backend, 2026-11-23), verified against `lego/backend/internal/metrics/rest.go:167-186,374-395`, `datastore.go:250-519`, and `lego/backend/internal/api/openapi/render-public-api-1.json`.
- **Goal linkage:** Render parity first (`docs/ADR008-vision.md#render-parity-first`) — closes `Metrics → Core metrics` and `Extended metrics` multi-resource and datastore `instance` parity gaps in `docs/ADR018-render-parity.md`.
- **Expected outcome:** Monitoring scripts, agents, and dashboards querying multiple datastores in one call or inspecting an HA standby / read replica's CPU and memory by `instance` (or `percentage`) work without hitting artificial `400` refusals.
- **Why now:** Multi-pod Postgres (HA + read replicas, `w6/m467`) already emits per-pod series in Prometheus, and Render's pinned OpenAPI already declares repeatable `resource` on all four datastore endpoints and `instance` on datastore `cpu`/`memory`/`*-limit`. Note: `percentage=true` on REST is bex's own cross-surface extension (Render's REST has no `percentage` param, `render_openapi.go:292`); enabling it on datastores keeps REST/MCP/GraphQL consistent with services. Render parity closing task (`t004`) is included because this changes REST, GraphQL, and MCP surfaces.
