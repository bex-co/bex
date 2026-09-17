# w4 · m113 — Static sites are dark on request observability: no request counts, latency, or request logs

**Worker:** worker4 **Goal:** a static site that serves traffic shows it — Total Requests, Response Times, method/status discovery, and `type=request` log lines all work for `static_site` exactly as they do for compute services, on REST, GraphQL, MCP, and the dashboard. **Status:** todo

## Tasks (in order)

| id   | title                                                                                    | est  | depends_on |
| ---- | ---------------------------------------------------------------------------------------- | ---- | ---------- |
| t001 | Router-attributed `http_requests` for static sites (Prometheus path)                     | 1h   | —          |
| t002 | Router-attributed `http_latency` for static sites                                        | 45m  | t001       |
| t003 | `metricsFilters` status/instance discovery for static sites                              | 30m  | t001       |
| t004 | Static request lines into Loki: shipper attribution or static-server emission            | 1h30 | —          |
| t005 | Render parity + docs                                                                     | 20m  | t003, t004 |
| t006 | Test coverage                                                                            | 45m  | t005       |
| t007 | Closeout                                                                                 | 10m  | t006       |

## Definition of done

Each bullet is a click the next person can repeat on production and watch succeed.

- **Request counts show traffic.** On a throwaway static site, after ~10 served requests, the Metrics tab Total Requests chart is non-zero over the traffic window; `GET /v1/metrics/http-requests?resource=<id>` returns a non-empty series (today: `200 []`).
- **Latency shows traffic.** The same window renders Response Times p90 (today: "No data in range"); `http-latency` REST/GraphQL/MCP return series.
- **Discovery agrees.** `metricsFilters` (and REST `/filters/http`) return the observed status codes for the static site (today: `STATUS_CODE: []`), so the dashboard Status Code filter is usable.
- **Request logs exist.** `type=request` returns the served lines with method/status (today: `{"logs":[]}`); the host/path-filtered metrics read (served from the same store) works for static.
- **Bandwidth stays correct.** The already-working router-attributed bandwidth series is unchanged (the regression control).

## Source + Goal linkage

- **Source:** live `/qa-find-bugs` pass on `https://dashboard.bex.co`, 2026-09-17 (w4-targeted run, `muse.env` credentials). Fixture: static site `qa-20260917-p11-site` (`srv-dam398js0ils73bgp0lg`, deleted after the pass); ~25 requests driven against it (200s, 301s, 404s, SPA-fallback 200s) over ~10 minutes. Metrics tab: Total Requests + Response Times "No data in range", bandwidth 16 KiB month-to-date. GraphQL `metrics(name:HTTP_REQUESTS|HTTP_LATENCY)` → `[]`, `metricsFilters` INSTANCE/STATUS_CODE → `[]`; REST `/v1/metrics/http-requests` → `200 []` while `/v1/metrics/bandwidth` → 5 nonzero buckets; GraphQL `logs(type:request)` → `{"logs":[]}`.
- **Goal linkage:** product truthfulness on the static-site journey ([docs/ADR029-static-sites.md](../../../docs/ADR029-static-sites.md)) and the parity ledger ([docs/ADR018-render-parity.md](../../../docs/ADR018-render-parity.md) row 84: "requests stay — the static site's Ingress routers attribute Traefik series per App"). A static-site owner who just shipped a launch currently sees flat "No data" charts and an empty request log while bandwidth proves the traffic happened.
- **Expected outcome:** static request observability matches compute services on all four surfaces, attributed via the Ingress-router path the ledger already promises (Prometheus) plus request-line attribution (Loki).
- **Why now:** the fix composes from existing machinery — the router-aware `egressquery.App(AppID, Routers, Direct=false)` path already attributes bandwidth per static App; requests/latency/filters just never got the same treatment, and the shipper's App-attribution regex structurally cannot see static traffic.
- **Explicitly out:** the deliberate divergences this pass re-confirmed (default SPA fallback for extension-less misses, rooted-only route destinations, search-without-highlight in the log viewers) — not re-filed. The m108 rate-vs-count fix (web services) is untouched; this milestone extends request metrics to the service type m108 never covered.
