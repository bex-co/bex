# w34 · m587 — Real path-filter suggestions and wildcard path matching for HTTP metrics

**Worker:** worker34 **Goal:** replace the hardcoded-empty `/v1/metrics/filters/path` and GraphQL `metricsPathFilterSuggestions` stubs with Loki-backed path suggestions (with proper authorization) and support `*` wildcard path matching on HTTP request/latency metrics, matching Render's pinned metrics API contract. **Status:** todo

## Tasks (in order)

| id   | title                                                                                                                               | est | depends_on |
| ---- | ----------------------------------------------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Add Loki-backed `PathFilterSuggestions` source and service verb (last 5,000 matching access-log lines, authz, plan-retention clamp) | 1h  | —          |
| t002 | Wire `GET /v1/metrics/filters/path` and GraphQL `metricsPathFilterSuggestions` to `PathFilterSuggestions`                           | 45m | t001       |
| t003 | Support `*` wildcard path matching in `lokiRequestPipeline` across `http-requests` and `http-latency` (REST, GraphQL, MCP)          | 45m | —          |
| t004 | Update `docs/ADR018-render-parity.md`, `docs/render-artifacts/metrics-page.md`, and `docs/ADR010-observability.md`                  | 30m | t002, t003 |
| t005 | Render parity check across REST, GraphQL, MCP, and dashboard                                                                        | 30m | t004       |
| t006 | Simplify changed code                                                                                                               | 20m | t005       |
| t007 | Test coverage for path suggestions, authz, wildcards, and failure modes                                                             | 30m | t005, t006 |
| t008 | Closeout                                                                                                                            | 15m | t007       |

## Definition of done

- `GET /v1/metrics/filters/path` with one or more `resource` IDs (and optional `startTime`/`endTime`, `host`, `statusCode`, `path`) queries up to the 5,000 most recent matching Traefik access-log lines in Loki within the workspace's plan retention floor and returns deduplicated, bounded path suggestions (`[]` when no lines match, `503` when Loki is unwired, `403`/`404` on unauthorized or unknown services).
- GraphQL `metricsPathFilterSuggestions(query: {serviceIDs, paths})` authorizes every requested service ID and returns the same suggestions as REST.
- Passing wildcard path patterns such as `path=/api/*` on REST `GET /v1/metrics/http-requests` and `/v1/metrics/http-latency`, GraphQL `metrics(filters:[{field:"PATH", values:["/api/*"]}])`, and MCP `get_metrics(httpPath:"/api/*")` matches paths under `/api/` via safely escaped RE2 regex matching, while literal paths without `*` keep exact equality.
- `go test ./internal/metrics ./internal/api` passes with all new behavioral and cross-surface parity tests.

## Source + Goal linkage

- **Source:** `/pm-brainstorm` (hosting metrics backend, 2026-11-23), verified against `lego/backend/internal/metrics/rest.go:263-272`, `graphql.go:476-484`, `lokisource.go:173-195`, and `lego/backend/internal/api/openapi/render-public-api-1.json` (`list-path-filter-values`).
- **Goal linkage:** Render parity first (`docs/ADR008-vision.md#render-parity-first`) — closes the `Metrics → Core metrics` path-suggestion and wildcard-filter gaps in `docs/ADR018-render-parity.md`.
- **Expected outcome:** API, CLI/SDK, and dashboard callers receive real observed path suggestions and working `*` wildcard path filters instead of empty stubs and literal-only path matches, and GraphQL `metricsPathFilterSuggestions` enforces authorization consistently.
- **Why now:** Traefik already ships `RequestPath`, `RequestHost`, and `status` to Loki (`deploy/gitops/base/values/traefik.values.yaml:180-185`) and `metrics` already queries Loki for HTTP metrics (`lokisource.go`); only the stubbed filter endpoints and exact-only path matcher remain. Render parity closing task (`t005`) is included because this changes REST, GraphQL, and MCP user-facing surfaces.
