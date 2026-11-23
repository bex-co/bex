# w34 · m588 — Time-windowed, cross-filtered, multi-resource HTTP and Application filter discovery

**Worker:** worker34 **Goal:** bring `GET /v1/metrics/filters/http`, `GET /v1/metrics/filters/application`, and GraphQL `metricsFilters` to Render parity with real observed `host` discovery, `host`/`statusCode` cross-filtering, `[startTime, endTime]` historical instance discovery, and multi-resource queries. **Status:** todo

## Tasks (in order)

| id   | title                                                                                                                                      | est | depends_on |
| ---- | ------------------------------------------------------------------------------------------------------------------------------------------ | --- | ---------- |
| t001 | Extend `MetricsFiltersQuery` and `MetricsFilters` with `[Start, End]` (retention-clamped), multi-resource authz + fan-out, and datastores | 1h  | —          |
| t002 | Discover historical instances over `[Start, End]` from Prometheus cAdvisor series (merged with live pods)                                  | 1h  | t001       |
| t003 | Discover observed `host` values and support `host`/`statusCode` cross-filtering in `MetricsFilters` (Loki + bounded Prometheus fallback)   | 1h  | t001       |
| t004 | Wire multi-resource, time-windowed, and cross-filtered inputs through REST `/v1/metrics/filters/{application,http}` and GraphQL + docs     | 45m | t002, t003 |
| t005 | Render parity                                                                                                                              | 20m | t004       |
| t006 | Simplify                                                                                                                                   | 20m | t005       |
| t007 | Test coverage                                                                                                                              | 30m | t005, t006 |
| t008 | Closeout                                                                                                                                   | 15m | t007       |

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
