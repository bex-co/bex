# w4 · m164 — Count initial and newly scraped traffic before free web auto-sleep

**Worker:** worker4 **Goal:** a free web service's idle clock includes its first served request and activity scraped after the most recent subquery step. **Status:** blocked — t001/t002/t005/t006 done 2026-10-03; t003 live journeys, t004 live parity half and t007 closeout remain

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 — **DONE** | Audit the activity reader's callers, aliases and timing boundaries | 30m | — |
| t002 — **DONE** | Preserve initial positive samples and current-step activity in sleep decisions | 60m | t001 |
| t003 | Repeat sparse traffic, steady traffic, idle and wake journeys live | 45m | t002 |
| t004 | Render parity | 20m | t003 |
| t005 — **DONE** | Simplify | 15m | t004 |
| t006 — **DONE** | Test coverage with an actual PromQL evaluator | 45m | t004 |
| t007 | Closeout | 10m | t005, t006 |

Total: **225m / 7 tasks**. Ships the filing only; implementation remains scheduled.

## Definition of done

- Create a Free Go web service from `bex-co/bex`, root `examples/hello-go`, build `go build -o app .`, start `./app`, port 3000, auto-deploy Off. Set the dashboard idle timeout to **5 min**, reload to verify it, and issue its first GET and HEAD about 30 seconds after Running. Without further public traffic, its last-active stamp must advance and it must remain Running until at least 300 seconds after the last request; it must subsequently sleep after the quiet window with bounded scrape/reconcile slack. Sweep 71 instead slept exactly 300 seconds after initial Running, only 270 seconds after HEAD.
- Repeat on a fresh service using `setIdleTimeout(..., idleTTLSeconds:60)`, issue just one GET about 39 seconds after initial Running, and watch status without probing the public URL again. No hibernation before 60 seconds after that GET. Sweep 71's independent fixture slept only 21.2 seconds after it. REST, GraphQL, MCP and a freshly loaded dashboard must report the actual phase, with the timeout still 60 and `suspended:not_suspended`.
- At TTL 60, wake the service and send GETs every 20 seconds across multiple idle windows. Once it returns the application's `200 OK`, served traffic must keep it awake. Capture request logs and raw counter samples so a missed first sample or off-grid scrape cannot be mistaken for inactivity. Sweep 71 received a second activator 503 in this control despite two intervening 200s. After stopping requests, verify eventual sleep and then browser automatic wake back to `OK`; do not keep it awake forever merely because a counter remains positive.
- Delete the owned fixtures, verify API 404s and no owned cluster remnants, and revoke only this run's session.

## Source + Goal linkage

- **Source:** `$qa-find-bugs`, muse credential profile, dashboard production sweep 71 on 2026-10-03 local / 2026-10-04 UTC. User explicitly selected w4. [Finding and durable probes](finding.md).
- **Goal linkage:** ADR008 reliable Render-alternative hosting; `docs/ADR003-control-plane.md:80` free web sleep-when-idle decision and ADR018 suspend/resume parity. This is a residual regression against the complete guarantee of [w1/m151](../../w1/done/m151/README.md), with [w1/m161](../../w1/done/m161/README.md)'s two WebSocket directions preserved. Resolve those board paths from `.pm/`.
- **Expected outcome:** sparse inbound traffic extends the idle clock; a scrape visible at the decision timestamp is not discarded by subquery alignment. Quiet services still release compute.
- **Why now:** two clean fixtures reproduce premature sleep, and the steady control exposes an additional boundary in the same decision. Avoidable 503/cold starts affect the Free web journey already promised by the product.
- **Render parity:** included because user-visible availability, service phases and timeout semantics change, although no API shape change is intended. Render documents 15 minutes without inbound HTTP/WebSocket traffic; custom positive idle TTL is the existing Bex extension.
- **Unverified:** WebSocket-only live traffic, default-900s long-duration control, paid plans, Docker/CNB/image/Blueprint creation paths, scrape outages and concurrent wake/stamp conflicts were not exercised in this sweep. These are verification work, not claimed live failures.
