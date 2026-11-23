# w34 · m587 — Real path-filter suggestions and wildcard path matching for HTTP metrics

**Worker:** worker34 **Goal:** replace the hardcoded-empty `/v1/metrics/filters/path` and GraphQL `metricsPathFilterSuggestions` stubs with Loki-backed path suggestions (with proper authorization) and support `*` wildcard path matching on HTTP request/latency metrics, matching Render's pinned metrics API contract. **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Add Loki-backed `PathFilterSuggestions` source and service verb (last 5,000 matching access-log lines, authz, plan-retention clamp) — **DONE** | 1h | — |
| [t002](done/t002.md) | Wire `GET /v1/metrics/filters/path` and GraphQL `metricsPathFilterSuggestions` to `PathFilterSuggestions` — **DONE** | 45m | t001 |
| [t003](done/t003.md) | Support `*` wildcard path matching in `lokiRequestPipeline` across `http-requests` and `http-latency` (REST, GraphQL, MCP) — **DONE** | 45m | — |
| [t004](done/t004.md) | Update `docs/ADR018-render-parity.md`, `docs/render-artifacts/metrics-page.md`, and `docs/ADR010-observability.md` — **DONE** | 30m | t002, t003 |
| [t005](done/t005.md) | Render parity check across REST, GraphQL, MCP, and dashboard — **DONE** | 30m | t004 |
| [t006](done/t006.md) | Simplify changed code — **DONE** | 20m | t005 |
| [t007](done/t007.md) | Test coverage for path suggestions, authz, wildcards, and failure modes — **DONE** | 30m | t005, t006 |
| [t008](done/t008.md) | Closeout — **DONE** | 15m | t007 |

## Outcome (2026-11-23)

- Added `PathFilterSuggestionsSource`, `NewLokiPathFilterSuggestionsSource` (Loki `/loki/api/v1/query_range`, `direction=backward`, `limit=5000`, `| line_format "{{.request_path}}"`, capped at 100 deduplicated paths ordered by frequency descending then lexicographically), and shared `metrics.Service.PathFilterSuggestions` enforcing `MaxQueryHours`, `checkFanOut`, per-resource `AuthorizeApp(ctx, core.RelCanView, ...)`, App creation floor, workspace plan retention clamping, and `core.ErrLogStoreUnavailable` (503) when `BEX_LOKI_URL` is unset.
- Wired `GET /v1/metrics/filters/path` (multi-resource `resource`/`service`, `startTime`/`endTime`, `host`, `statusCode`, `path`) and GraphQL `metricsPathFilterSuggestions` (authorizing every `serviceIDs` entry; removed stub exemption from `crossworkspace_gqlmcp_test.go`), plus `NewLokiPathFilterSuggestionsSource` in `cmd/api/main.go`.
- Added `lokiPathFilter` / `wildcardPathRegex` supporting `*` wildcard patterns via `regexp.QuoteMeta`-escaped anchored RE2 regexes (`request_path=~"^...$"`) while keeping literal paths as exact `request_path="..."` equality across REST, GraphQL, and MCP (`http_requests`, `http_latency`, `filters/path`), bounded at 64 path filters / 512 bytes / 8 wildcards (and 64 hosts / 512 bytes).
- Verified with `go test -race ./internal/metrics`, `go test ./internal/api` (including `TestRenderConformance*`, `TestAuthzGuardsEveryVerb`, `TestCrossWorkspaceGraphQLMatrix`, `TestEmptyListRoutesSerializeAsJSONArray`), `make lint` (all four Go modules + deadcode), real-Loki integration coverage in `lokisource_loki_test.go`, `pm-board-validate.py`, and `docs-links-validate.sh`.

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
