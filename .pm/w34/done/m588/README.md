# w34 · m588 — Time-windowed, cross-filtered, multi-resource HTTP and Application filter discovery

**Worker:** worker34 **Goal:** bring `GET /v1/metrics/filters/http`, `GET /v1/metrics/filters/application`, and GraphQL `metricsFilters` to Render parity with real observed `host` discovery, `host`/`statusCode` cross-filtering, `[startTime, endTime]` historical instance discovery, and multi-resource queries. **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Extend `MetricsFiltersQuery` and `MetricsFilters` with `[Start, End]` (retention-clamped), multi-resource authz + fan-out, and datastores — **DONE** | 1h | — |
| [t002](done/t002.md) | Discover historical instances over `[Start, End]` from Prometheus cAdvisor series (merged with live pods) — **DONE** | 1h | t001 |
| [t003](done/t003.md) | Discover observed `host` values and support `host`/`statusCode` cross-filtering in `MetricsFilters` (Loki + bounded Prometheus fallback) — **DONE** | 1h | t001 |
| [t004](done/t004.md) | Wire multi-resource, time-windowed, and cross-filtered inputs through REST `/v1/metrics/filters/{application,http}` and GraphQL + docs — **DONE** | 45m | t002, t003 |
| [t005](done/t005.md) | Render parity — **DONE** | 20m | t004 |
| [t006](done/t006.md) | Simplify — **DONE** | 20m | t005 |
| [t007](done/t007.md) | Test coverage — **DONE** | 30m | t005, t006 |
| [t008](done/t008.md) | Closeout — **DONE** | 15m | t007 |

## Outcome (2026-11-23)

- Extended `MetricsFiltersQuery` and `metrics.Service.MetricsFilters` with repeatable `Resources` (keeping backward compatibility with scalar `App`), `[Start, End]` (default `now-1h..now`, validated against `MaxQueryHours`/`ValidateQueryRange`, clamped per resource to `AppNotBefore` and plan retention via `withinRetention`, empty `[]` for windows wholly before retention), `checkFanOut`, per-resource authorization (`AuthorizeApp` for services; `AuthorizeDatabase`/`AuthorizeKeyValue` + `NotFoundIfDeleting` for `dpg-`/`red-` on `INSTANCE`/`RESOURCE`, refusing HTTP filter fields on datastores with 400 after authz), and up-front `Hosts`/`StatusCodes` validation.
- Added `InstanceFilterValuesSource` and `NewPrometheusInstanceFilterValuesSource` (`/api/v1/label/pod/values` with `start`/`end` over cAdvisor `container_cpu_usage_seconds_total` matching `serviceContainers` + `container!="POD"` for services and `instancePods` + `postgres`/`valkey` container for datastores), wired in `cmd/api/main.go` when `BEX_PROM_URL` is set, unioned with live pods (`OwnAppPods`, `DatabasePods`/`KeyValuePods` filtered to instance pods; static sites remain `[]`), projected via `ids.ServiceInstanceID`, deduplicated, and sorted.
- Added `End` to `MetricsFilterValuesRequest` / `NewPrometheusFilterValuesSource` (sharing `promLabelValues`) and added `HTTPFilterValuesSource` / `NewLokiHTTPFilterValuesSource` (sharing `lokiStreamQuery`, scanning up to 5,000 matching Traefik access-log lines `direction=backward` with `{{with .request_host}}{{.}}{{end}}`), wired in `cmd/api/main.go` when `BEX_LOKI_URL` is set. Cross-filtered HTTP queries require Loki (503 `LOG_STORE_UNAVAILABLE` when unwired); unfiltered `STATUS_CODE` preserves Prometheus-only support with honest `[]` for `HOST` and avoids Loki load on dashboard `INSTANCE + STATUS_CODE` polls.
- Wired REST `GET /v1/metrics/filters/{application,http}` (removing the single-resource check and 400 refusal on `host`/`statusCode`) and GraphQL `metricsFilters` (`windowFromInput`, multi-`RESOURCE`, `HOST`/`STATUS_CODE`), updated `render_metrics_contract_test.go`, `conformance_platform_test.go`, `docs/ADR018-render-parity.md`, `docs/ADR010-observability.md`, and `docs/render-artifacts/metrics-page.md`, and verified with `go test -race ./internal/metrics`, `go test ./internal/api` (+ `-race` on contract/authz/conformance tests), `make lint` (0 issues across all four modules + deadcode), real-Loki integration coverage in `lokisource_loki_test.go`, `pm-board-validate.py`, and `docs-links-validate.sh`.

## Definition of done

- `GET /v1/metrics/filters/http` accepts repeatable `resource`/`service`, `startTime`/`endTime` (default `now-1h..now`, clamped to plan retention), repeatable `host`, and repeatable `statusCode`, returning observed `statusCode` and `host` values (never hardcoded `host: []`, no 400 refusal on `?host=` or `?statusCode=`).
- `GET /v1/metrics/filters/application` accepts repeatable `resource`/`service` (including datastore IDs `dpg-`/`red-`, noted as an additive consistency extension where Render's spec text says `Service ID`) and `startTime`/`endTime`, returning the deduplicated union of canonical instance IDs observed in Prometheus during that window plus currently running pods.
- GraphQL `metricsFilters(query: {filters, start, end, outputFilters})` forwards `start`, `end`, multiple `RESOURCE` entries, and `HOST`/`STATUS_CODE` narrowing through the same shared `MetricsFilters` core.
- `go test -race ./internal/metrics ./internal/api` and `make lint` pass.

## Source + Goal linkage

- **Source:** `/pm-brainstorm` (hosting metrics backend, 2026-11-23), verified against `lego/backend/internal/metrics/rest.go:209-261`, `service.go:1360-1455`, `graphql.go:679-689`, and `lego/backend/internal/api/openapi/render-public-api-1.json` (`list-application-filter-values`, `list-http-filter-values`).
- **Goal linkage:** Render parity first (`docs/ADR008-vision.md#render-parity-first`) — closes `Metrics → Core metrics` filter-discovery gaps in `docs/ADR018-render-parity.md`.
- **Expected outcome:** API/SDK and dashboard callers can discover historical pods replaced during a window, discover real observed HTTP hosts, cross-filter status codes by host (and hosts by status code), and query filters across multiple resources in one request.
- **Why now:** Completes the filter-discovery triad alongside `w34/m587` and removes explicit `400` refusals and hardcoded empty `host` arrays on pinned Render OpenAPI routes. Datastore IDs on `/filters/application` are an additive extension consistent with `/metrics/cpu` and `/metrics/memory` accepting datastore `instance` IDs (recommendation under ADR008; open sign-off if a stricter service-only behavior is ever preferred). Render parity closing task (`t005`) is included because this changes REST and GraphQL surfaces.
