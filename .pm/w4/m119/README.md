# w4 · m119 — `http_requests` lies about bursty traffic: 25 requests read back 27.8 or 6.67 by query alignment

**Worker:** worker4 **Goal:** The m108 "counts are counts" guarantee holds for bursty traffic: N driven requests sum to N ±1 on every metrics surface for any query alignment, reconciling with the access log. **Status:** todo

## Tasks (in order)

| id   | title                                                                                        | est   | depends_on       |
| ---- | -------------------------------------------------------------------------------------------- | ----- | ---------------- |
| t001 | `http_requests` returns exact per-bucket counts for bursty traffic — todo                    | 1h30m | —                |
| t002 | Bandwidth sibling: live-verify `SumIncreases` extrapolation, apply the same treatment — todo | 1h    | w4/m119/t001     |
| t003 | Render parity: cross-surface check for the metrics changes — todo                            | 30m   | w4/m119/t001, w4/m119/t002 |
| t004 | Simplify the code this milestone touched — todo                                              | 20m   | w4/m119/t003     |
| t005 | Test coverage for the shipped behavior — todo                                                | 40m   | w4/m119/t003     |
| t006 | Closeout — todo                                                                              | 15m   | w4/m119/t004, w4/m119/t005 |

## Definition of done

Each bullet is a command or a click the next person can repeat on production and watch succeed.

- **Bursty counts are exact.** Create a free web service, send exactly 25 `curl` GETs to its `.onbex.co` URL, wait 2 min, then `GET /v1/metrics/http-requests?resource=<id>&resolutionSeconds=60` over three differently-aligned windows containing them: every window's value-sum is 25 ±1 — not 27.8 in one alignment and 6.67 in another (the 2026-09-20 capture below).
- **Counts reconcile with the access log.** The same 25 requests read `nlogs: 25, hasMore: False` from `GET /v1/logs?resource=<id>&type=request&limit=100`; the metrics sum for any containing window equals that line count ±1.
- **Cross-surface agreement.** The same window through GraphQL `metrics(name: HTTP_REQUESTS)` and MCP `http_request_count` returns the same numbers as REST.
- **Low-traffic non-zero still holds (m108 bullet 3).** A service that served requests shows a non-zero Total Requests summary and non-zero bars.
- **Bandwidth sibling settled.** Driven traffic's Outbound Bandwidth buckets reconcile with the month-to-date figure and usage metering within a documented tolerance, or a written, evidenced decision in `docs/render-artifacts/metrics-page.md` records the deliberate divergence with the axis/unit labelled so a reader cannot mistake a rate for bytes-per-bucket.

## Source + Goal linkage

- **Source:** live `/qa-find-bugs` pass 71 on `https://dashboard.bex.co`, 2026-09-20 (w4-targeted run, `muse.env` credentials). Fixture `qa-20260920-p71-met` (`srv-dansb83s0ils73bgpaj0`, go runtime, deleted at the end of the run). 25 `curl -L` GETs to `/room/hn` at ~11:41–11:42Z; browser MCP dead so the pass was API-only — the durable evidence is the probes quoted in t001.
- **Goal linkage:** product truthfulness and Render parity ([docs/ADR018-render-parity.md](../../../docs/ADR018-render-parity.md), [docs/ADR006-bex-api.md](../../../docs/ADR006-bex-api.md), [docs/ADR010-observability.md](../../../docs/ADR010-observability.md) for metrics semantics). Follow-up to `w4/m108` (done 2026-09-16): its t001 replaced `sum(rate(…))` with `sum(increase(…))`, which is exact for steady traffic but Prometheus `increase()` extrapolates partial-window counters, so bursty traffic — the free-tier norm — misreports by alignment. m108 DoD walk: bullet 1 (20 curls sum to 20 ±1) broken live; bullet 3 (low-traffic non-zero) still holds; bullet 2 (busy-service 12h reconciliation) not re-probed this pass; deploy-provenance bullets untouched by this finding.
- **Expected outcome:** the Metrics page's Total Requests can be reconciled against the access log for any traffic shape; the same one-line primitive choice is verified (not assumed) on the bandwidth sibling that shares it.
- **Why now:** the defect is invisible precisely because it degrades gracefully — a plausible small number instead of an error — on the chart users read to make scaling and debugging decisions. The exact-count primitive already exists in the file (`lokisource.go:116` `count_over_time`, same `RequestMetricsSource` signature), so the fix is a change of call, not of architecture.
- **Render parity task included** because the fix changes a user-facing surface with REST + GraphQL + MCP + UI representations.
