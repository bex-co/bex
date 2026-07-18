# w1 · m50 — Outbound Bandwidth: un-blank the panel (observability vs billing health gates)

**Worker:** worker1 **Goal:** make the service metrics page's Outbound Bandwidth panel actually show data on prod — split ADR023's billing-grade egress health gate (correctly strict for usage metering) from the observability read path (which must be best-effort), surface per-source health as data instead of an error, and stop the dashboard from masking gate errors as "No data in range". **Status:** in progress — t001–t005 + t007–t009 done 2026-07-18 (implementation complete: best-effort interactive reads with degraded-as-data, dashboard three-state panel, ADR023/ADR010/ADR018 updated, roll-acceptance decision recorded; the ungated query verified against prod Prometheus returns 145 points / 80 non-zero for the reference service); t006 prod verification awaits the next deploy, t010 closes after it

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Record the investigation: ADR023 § observability-vs-billing split + prod evidence — **DONE** | 30m | — |
| t002 | Backend: best-effort `Metrics(BANDWIDTH)` — no whole-window all-or-nothing error — **DONE** | 60m | t001 |
| t003 | Backend: per-source health as data on the bandwidth payload — **DONE** | 30m | t002 |
| t004 | Dashboard: three panel states — data / empty / source-degraded — **DONE** | 45m | t003 |
| t005 | Platform mitigation: shrink the meter's up-gap surface (DS rolls, scheduling) — **DONE** | 45m | t001 |
| t006 | Prod verification: bandwidth renders through deploys and meter rolls | 30m | t002, t004, t005 |
| t007 | Render parity — REST/GraphQL/MCP consistency for the changed bandwidth surface — **DONE** | 30m | t006 |
| t008 | Simplify — `/simplify` over the changed code — **DONE** | 30m | t007 |
| t009 | Test coverage — bucket/degraded gating against synthetic reset+gap fixtures — **DONE** | 45m | t007 |
| t010 | Closeout — verify DoD, sync status, move to done | 15m | t008, t009 |

## Definition of done

On production (dashboard.bex.co, api.bex.co):

- `https://dashboard.bex.co/services/<srv>/metrics` shows an Outbound Bandwidth series for a service with traffic over the 12h **and** 7d windows, including windows that contain an app deploy (Traefik router-counter reset) and an egress-meter roll (`up` gap) — the two events that blank the panel today.
- `MonthToDateBandwidth` returns non-zero `egressBandwidthMB` for a service with traffic; degraded sources are reported as data, not an error.
- When a source is genuinely unhealthy in-window, the panel says so (distinct degraded state naming the source) — never the empty-series "No data in range", and never a silently absent chart.
- The usage-metering (billing) pipeline's strict gate is untouched: hourly rollup numbers for a fixture window are byte-identical before/after the change.
- REST/GraphQL/MCP bandwidth responses stay shape-consistent; backend + dashboard suites green.

## Source + Goal linkage

- **Source:** user report 2026-07-18 ("research why Outbound Bandwidth No data in range on /services/srv-d9bj8s3eg85c7390eb9g/metrics") + same-day live prod investigation: captured GraphQL errors (`egress source direct unhealthy`, `egress source http unhealthy`), term-by-term prod Prometheus evaluation of `egressquery.Health()` (direct: `up` min 0 with 26/≥2304 samples in 12h — single meter instance absent through tenant-node churn, `bex-tenant-0-…-2dw27` cordoned; http: 32 router-counter resets in 12h from deploy-driven Ingress reloads; websocket: healthy but never evaluated), plus the DS's template generation 67 in 45h (rolls with every image pin).
- **Goal linkage:** Render parity on the metrics surface (ADR018 — Render's bandwidth chart is best-effort observability); protects ADR023's usage-metering integrity by _separating_ the two consumers instead of weakening the billing gate.
- **Expected outcome:** the Outbound Bandwidth panel goes from structurally-blank-on-prod (any meter restart or app deploy poisons every window containing it — with ~daily deploys, that is nearly always) to always-rendering, with honest degradation states; operators can finally distinguish "no traffic" from "meter unhealthy".
- **Why now:** user-reported, live on prod today; the panel has likely never shown data in production (deploy cadence guarantees an in-window reset); every future deploy keeps re-blanking it. The health-gate design flaw also silently zeroes `MonthToDateBandwidth`, which understates usage the moment anyone reads it for capacity/billing sanity.
- **Render parity task included** because the fix changes a tenant-facing bandwidth payload across REST/GraphQL/MCP and the dashboard panel.

## Non-goals

- Changing the usage-metering (billing) gate semantics — the hourly rollup keeps ADR023's strict never-invent-bytes contract.
- Multi-replica / HA egress meter.
- Render's private-link/NAT bandwidth split beyond the already-stubbed fields.
