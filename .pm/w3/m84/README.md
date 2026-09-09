# w3 · m84 — bex-api first-party request telemetry: per-route, per-operation, per-tool histograms and origin SLIs

**Worker:** worker3 **Goal:** bex-api's own latency and error picture exists — a bounded-cardinality request histogram per REST route pattern, GraphQL operation, and MCP tool on the existing `/metrics` registry, with origin-side SLI panels and alert rules — so an API incident can be attributed to the edge or the origin, and agent traffic is observable per tool. **Status:** todo

## Tasks (in order)

| id   | title                                                                                                                | est | depends_on |
| ---- | -------------------------------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | HTTP middleware: duration histogram, request counter, in-flight gauge by surface/route-pattern/method/status          | 45m | —          |
| t002 | GraphQL operation and MCP tool dimensions with bounded label sets                                                    | 45m | t001       |
| t003 | Scrape keep-list, api-gateways dashboard panels, two origin alert rules, coverage check green                        | 60m | t002       |
| t004 | Records: close the ADR088 §6 known gap, ADR010 note, origin-vs-edge correlation recipe on the FUTURE-MAYBE entry     | 20m | t003       |
| t005 | Simplify                                                                                                             | 20m | t004       |
| t006 | Test coverage                                                                                                        | 30m | t004       |
| t007 | Closeout                                                                                                             | 10m | t006       |

## Definition of done

A live bex-api exposes `bex_api_http_request_duration_seconds` (+ `_requests_total`, in-flight) labelled by `surface`, `route` (the registered mux **pattern**, never a raw path), `method`, `status`, plus `bex_api_graphql_operation_duration_seconds{operation,type,outcome}` and `bex_api_mcp_tool_duration_seconds{tool,outcome}`; a test asserts no label value carries an id-shaped segment. The api-gateways dashboard shows origin p50/p95/p99 per surface, error ratio by route, slowest routes, GraphQL mutation error ratio, and MCP tool latency, fed by those series; `BexApiOriginHighErrorRate` and `BexApiOriginLatencyHigh` exist with panels and `scripts/obs-coverage-check.sh` is green. ADR088 §6 no longer lists the histogram as a known gap.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w3` 2026-09-08 #3, verified the same day: `lego/backend` has no HTTP request histogram anywhere in bex-api (the ssh-gateway, agent sessions, and webhooks each have their own), and the ADR088 platform-availability dashboard reads only Traefik's `traefik_service_request_duration_seconds`. ADR088 §6 records it verbatim: "Known gap (recorded, not hidden): bex-api has no first-party per-route request histogram — its error/latency view is Traefik's edge perspective … adding a native histogram is the first candidate when edge-vs-origin attribution starts mattering." The open FUTURE-MAYBE entry on intermittent `ERR_CONNECTION_CLOSED` mutations says to capture and correlate a request before promoting — impossible without an origin-side per-route counter.
- **Goal linkage:** the AI-native thesis (ADR008) — agents' MCP and GraphQL calls become observable per tool and operation; the ADR088 SLI baseline.
- **Expected outcome:** edge-versus-origin attribution for any API incident, a per-tool latency view for agent traffic, and a concrete correlation path for the connection-closed mystery.
- **Why now:** the dashboards shipped this week cannot attribute origin faults, and the connection-closed trigger cannot be acted on without this.
- **Render parity task omitted:** internal telemetry only — no REST/GraphQL/MCP/UI contract change.

## Notes

- Cardinality is the risk: route labels must be mux patterns (Go ≥1.23 `r.Pattern`, set on the same `*Request` the middleware holds, so reading it after `next.ServeHTTP` returns yields the innermost matched pattern even through nested muxes); unknown GraphQL operations fold to `other`; MCP tool names are a closed set.
- Streaming handlers (SSE log tails, MCP SSE, WebSocket upgrades) must keep `http.Flusher` / `http.Hijacker` through the response-writer wrapper — the existing `internal/api/gzip.go` wrapper is the precedent.
