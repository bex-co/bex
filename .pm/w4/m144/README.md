# w4 · m144 — Recover service metrics from Prometheus's OOM during WAL replay

**Worker:** worker4 **Goal:** production service CPU, memory and instance charts read real samples again, with a Prometheus startup budget that can replay its retained data. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Size and recover the production Prometheus replay budget | 60m | — |
| t002 | Verify shared metrics consumers and local overlay boundaries | 40m | t001 |
| t003 | Render parity | 20m | t002 |
| t004 | Simplify | 10m | t003 |
| t005 | Test coverage | 25m | t003, t004 |
| t006 | Closeout | 10m | t005 |

## Definition of done

- Load an authorized existing service's Scaling page (the hunt used `srv-d9bj8s3eg85c7390eb9g`), wait for the 48h queries, then hard-reload. CPU, memory and instance charts contain collected data where samples exist; an unavailable backend is still an error, never fake zero usage.
- Replay the exact GraphQL, REST CPU and MCP `get_metrics` probes in [finding.md](finding.md), updating the window to include fresh samples. They return successful, scoped series; the CPU_LIMIT control still returns the pod's configured limit. A historical gap caused by the outage remains a gap.
- Repeat the recorded `kubectl get pod`, container-status, previous-log and EndpointSlice reads: Prometheus completes WAL replay, becomes Ready with a ready service endpoint, and its restart count stays stable across a 15-minute observation. Preserve the existing volume/history; do not clear WAL to make this pass.

The live probes above were exercised in failing form. Other consumers and restart regression validation are work for t002/t005; they were not all exercised in this hunt.

## Source + Goal linkage

- **Source:** continuous `$qa-find-bugs`, 2026-10-02, `muse.env` account, r1 Scaling failure followed by read-only r2 investigation. [Complete request/response and runtime evidence](finding.md); verified local screenshot `.playwright-mcp/qa-metrics-r2-1.png` and JSON `.playwright-mcp/qa-metrics-r2-1.json`.
- **Goal linkage:** ADR008 dependable hosting and [ADR010 observability](../../../docs/ADR010-observability.md). Configuration-only limits remain visible while the measurements needed to operate a service fail.
- **Expected outcome:** real ranged metrics become available again and Prometheus can restart with its existing retained data and required scrape/rule set.
- **Why now:** production had 572 restarts and its most recent termination was OOMKilled during WAL replay. The deployed memory limit matches the committed 512 MiB budget; this is an active platform dependency failure, not a chart formatting bug.
- **Render parity:** included as a consumer verification task because the outage affects REST/GraphQL/MCP and the dashboard. No API shape or authorization change is proposed.
