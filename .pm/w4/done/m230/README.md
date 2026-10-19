# w4 · m230 — Record a free-tier wake as a wake, not a recovery

**Worker:** worker4 **Goal:** an idle wake or a user Resume records only its own lifecycle event, at the time it happened. `server_available` ("Service recovered") stays the partner of a recorded `server_failed`, as Render and ADR052 define it. **Status:** done (2026-10-19, loopx w4)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Keep the hibernated baseline through a startup that is only scale-up progress — **DONE** | 60m | — |
| [t002](done/t002.md) | Stamp `service_woken` at the woken instance's Ready transition — **DONE** | 30m | — |
| [t003](done/t003.md) | Audit the observed-lifecycle family and place the adjacent classes — **DONE** | 30m | w4/m230/t001, w4/m230/t002 |
| [t004](done/t004.md) | Render parity — **DONE** | 15m | w4/m230/t003 |
| [t005](done/t005.md) | Simplify — **DONE** | 10m | w4/m230/t004 |
| [t006](done/t006.md) | Test coverage — **DONE** | 45m | w4/m230/t004 |
| [t007](done/t007.md) | Closeout — **DONE** | 60m | w4/m230/t005, w4/m230/t006 |

Seven tasks, 250m total (4h10m). One **major** finding (an unpaired recovery event on a cold wake) and one **minor** finding (the wake event is stamped late). Both come from the same observed-state projector. This milestone schedules the work only.

## Definition of done

Replay the workflow in [finding.md](finding.md) with the [recorded probes](evidence.json) on a fresh owned `qa-`-prefixed Free web service: image `python:3.13-alpine` running `python3 -m http.server $PORT`, Idle timeout 5 min. Never adopt a foreign resource.

- **Six cold wakes.** For each: wait for `service_hibernated`, then poll `curl -sS -o /dev/null -w '%{http_code}' https://<slug>.onbex.co/` every 1 s until it returns 200. Afterwards `GET /v1/services/<id>/events?limit=100`, GraphQL `serviceEvents(serviceId:, limit:100)` and MCP `list_events` each list exactly one `service_hibernated` and one `service_woken` per cycle, and **zero** `server_available` and zero `server_failed`. The Activity page on a fresh load shows no "Service recovered" row. At least three of the six wakes must take ≥15 s from first 503 to first 200, because that is the window pass 7d3e19's wake 1 tripped in.
- **Woken timestamps.** Each `service_woken` timestamp lies within 2 s of that cycle's first 200 (bounded below by the cycle's `service_hibernated`). Activity lists each "Service woke up" above that cycle's "Service went to sleep" and below the next one.
- **Resume control.** Suspend, wait for Hibernated, then Resume. This records `service_suspended`, `service_resumed` and no `server_available`, as the 00:17:57 control did.
- **Outage control, kept.** On an awake owned service with no open deploy, make the only instance unready, using the existing crash-edge method (an image that crash-loops, or the `TestPGObservedCrashEdgeEmitsExactlyOnePair` scenario run against the real store). This still records exactly one `server_failed` followed by one `server_available`. Deploy-row rollouts remain exempt as before.
- **Adjacent decisions recorded.** t003's decision on a wake whose instance fails concretely is recorded in ADR052 and covered by a store test. Each `makeFact` sibling (`service_suspended`, `service_hibernated`, `service_resumed`) has a written disposition.
- **Checks pass.** Backend `go test ./...` (real Postgres) and `make lint` pass. The failing-direction tests fail on the pre-fix code.
- **Cleanup.** Delete the fixture, verify REST 404, and revoke only this run's QA session.

Push and webhook delivery are not live-asserted here: no device or endpoint is required. t006 covers them through the shared fact projection in tests.

## Source + Goal linkage

- **Source:** live functional `/qa-find-bugs w4`, pass 7d3e19, 2026-10-18/19 UTC, `muse.env`, workspace `bex-canary`. See [finding.md](finding.md) and [evidence.json](evidence.json); the screenshot `.playwright-mcp/qa-wake-activity-7d3e19.png` is local and gitignored. Production was pinned to `d5c3373ed`; research is on main `5e75de920`. Independent prior sighting: `w4/done/m147/README.md:52` (2026-10-02).
- **Goal linkage:** [ADR008](../../../docs/ADR008-vision.md) dependable hosting with truthful status. [ADR052](../../../docs/ADR052-notifications.md):113 says `server_available` closes the Critical `server_failed` page. w6/m47 (`notifications/push_worker.go:662-668`) keeps idle wake cycles out of push, which this leak undoes.
- **Expected outcome:** owners who subscribe to "Service recovered" and webhook consumers of `server_available` hear about recoveries only after a recorded failure. The Activity feed tells a sleep/wake cycle in causal order.
- **Why now:** the class widened when `w4/207` (`cc281d02e`, 2026-10-08) cut the debounce confirm to 10 s: any wake longer than about two passes can now trip it. `w4/196` back-dated only the availability edges, which inverted the feed order relative to the woken fact emitted in the same pass. The fix is one module (`internal/store`) with a single caller of the observation builder.
- **Render parity included:** the change alters what REST/GraphQL/MCP events, webhooks, push and the dashboard Activity feed report. Render's documented `server_failed`/`server_available` pair (render.com/docs/webhooks, fetched 2026-10-19) is the target. `service_hibernated`/`service_woken` are bex extensions.
- **Anti-goals:** `DO_NOT_DO.md:34` (do not file self-waking without traffic) does not apply. Every wake here followed this pass's own request, and the defect is the extra fact, not the wake.
- **Unverified:** push and webhook delivery, MCP, failed-wake behavior, datastore analogs, and the true hibernation time. See finding.md.
