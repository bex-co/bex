# w4 · m144 — Recover service metrics from Prometheus's OOM during WAL replay

**Worker:** worker4 **Goal:** production service CPU, memory and instance charts read real samples again, with a Prometheus startup budget that can replay its retained data. **Status:** done — 2026-10-02; all 6 tasks done. Operational acceptance passed read-only ([live acceptance](live-acceptance.md)).

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Size and recover the production Prometheus replay budget — **DONE** | 60m | — |
| t002 | Verify shared metrics consumers and local overlay boundaries — **DONE** | 40m | t001 |
| t003 | Render parity — **DONE** | 20m | t002 |
| t004 | Simplify — **DONE** | 10m | t003 |
| t005 | Test coverage — **DONE** | 25m | t003, t004 |
| t006 | Closeout — **DONE** | 10m | t005 |

## Definition of done

- Load an authorized existing service's Scaling page (the hunt used `srv-d9bj8s3eg85c7390eb9g`), wait for the 48h queries, then hard-reload. CPU, memory and instance charts contain collected data where samples exist; an unavailable backend is still an error, never fake zero usage.
- Replay the exact GraphQL, REST CPU and MCP `get_metrics` probes in [finding.md](finding.md), updating the window to include fresh samples. They return successful, scoped series; the CPU_LIMIT control still returns the pod's configured limit. A historical gap caused by the outage remains a gap.
- Repeat the recorded `kubectl get pod`, container-status, previous-log and EndpointSlice reads: Prometheus completes WAL replay, becomes Ready with a ready service endpoint, and its restart count stays stable across a 15-minute observation. Preserve the existing volume/history; do not clear WAL to make this pass.

The live probes above were exercised in failing form. Other consumers and restart regression validation are work for t002/t005; they were not all exercised in this hunt.

## Source + Goal linkage

- **Source:** continuous `$qa-find-bugs`, 2026-10-02, `muse.env` account, r1 Scaling failure followed by read-only r2 investigation. [Complete request/response and runtime evidence](finding.md); verified local screenshot `.playwright-mcp/qa-metrics-r2-1.png` and JSON `.playwright-mcp/qa-metrics-r2-1.json`.
- **Goal linkage:** ADR008 dependable hosting and [ADR010 observability](../../../../docs/ADR010-observability.md). Configuration-only limits remain visible while the measurements needed to operate a service fail.
- **Expected outcome:** real ranged metrics become available again and Prometheus can restart with its existing retained data and required scrape/rule set.
- **Why now:** production had 572 restarts and its most recent termination was OOMKilled during WAL replay. The deployed memory limit matches the committed 512 MiB budget; this is an active platform dependency failure, not a chart formatting bug.
- **Render parity:** included as a consumer verification task because the outage affects REST/GraphQL/MCP and the dashboard. No API shape or authorization change is proposed.

## Outcome — 2026-10-02

[Verification](verification.md) records the original OOM, node capacity, bounded production memory change, corrected local Helm precedence, shared consumer matrix and automated render evidence. [Sweep 7 recheck](qa-r7-recheck.md) then found the production 1Gi/2Gi budget applied, a Ready endpoint with zero restarts, successful own-web CPU/memory/instance GraphQL samples and working sleep/wake. This is partial recovery evidence; no historical/billing repair or full consumer recovery is claimed. t004 is done; implementation/render work in t001/t002/t005 is complete, but their operational criteria remain open. **BLOCKED:** QA/platform must measure successful replay/runtime/query peaks and cardinality, verify a retained-data restart with 15 minutes of stability, and complete the original API/UI probes plus shared-consumer matrix. t003 live parity and t006 closeout remain open.

## Operational acceptance — 2026-10-02

All checks were read-only. Under the 1Gi/2Gi budget, the 08:35:57Z start replayed 580 retained WAL segments in 54 s from the existing PVC and became Ready. It has stayed up about 23 h with zero restarts, including a 15-minute observation window with ready EndpointSlice. Measured runtime peak is 760 MiB working set. The replay peak is bounded below 2 GiB (no OOM) but was not directly measurable without a restart. Head series 114,635; node at 65% memory. GraphQL/REST/MCP return fresh 13-point CPU/memory/instance series; CPU_LIMIT returns 1; the outage window stays an honest gap. The Scaling page loads charts with zero errors after a hard reload. 114 rules are healthy. **Separate follow-up (not this OOM):** the `valkey-instances` exporter for production `red-d9p49kdrtmes73c34ovg` times out because the 10m-CPU sidecar is 98% throttled (`keyvalue_controller.go:171`). Details: [live-acceptance.md](live-acceptance.md).
