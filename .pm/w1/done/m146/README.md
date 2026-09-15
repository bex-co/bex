# w1 · m146 — The live log tail stays connected when a service has no running instance, instead of reconnecting every 3 seconds

**Worker:** worker1 **Goal:** a live log subscription to a service whose instances have all exited, such as a cron job between runs, stays open. It idles, heartbeats, and picks up the next instance's output when that instance starts. Today the stream ends after about 300 ms and the page shows "Live tail disconnected — reconnecting…" while re-subscribing forever. **Status:** done (2026-09-15). Every task is complete, and the definition of done passed live on production (images pinned to `c4212ec71`). The dashboard Logs-page check on zero-pod services could not run once the browser disconnected; the stream-level checks passed, see § Live verification.

## Tasks (in order)

| id   | title                                                                                                                               | est | depends_on       |
| ---- | ----------------------------------------------------------------------------------------------------------------------------------- | --- | ---------------- |
| t001 | `FollowLogs` keeps the stream open: heartbeat, and attach to each new app pod as it appears (the `followBuildLogs` watch shape) — **DONE** | 75m | —                |
| t002 | Adjacent classes: subscription slot caps under long-idle streams, the codex #3 no-producer rule, watchdog/deletion, edge idle limits — **DONE** | 30m | t001             |
| t003 | Aliases and sibling states: the WebSocket transport, NDJSON/Render CLI tail, and zero-pod states (suspended, hibernated, pre-first-pod) — **DONE** | 45m | t001             |
| t004 | Render parity — **DONE**                                                                                                            | 30m | t001, t002, t003 |
| t005 | Simplify — **DONE**                                                                                                                 | 20m | t004             |
| t006 | Test coverage — **DONE**                                                                                                            | 45m | t004             |
| t007 | Closeout — **DONE**                                                                                                                            | 10m | t006             |

## Definition of done

Each bullet can be repeated from a signed-in page on production (or `dev-1`). The first four were run at filing time and failed; the control was run and passed:

- **No reconnect loop.** On a cron job that has completed at least one run, open `/services/<id>/logs` with Live on and count `/v1/logs/subscribe` responses for 60 s: **exactly 1**, and the `Live tail disconnected — reconnecting…` banner **never** appears. At filing time there were **19** subscribes, at 1.9, 5.1, 8.4 … 60.8 s (a steady ~3.3 s cadence), and the banner was present in 10 of 12 five-second samples.
- **SSE stays open.** `fetch('https://api.bex.co/v1/logs/subscribe?resource=<cron srv-id>', {credentials:'include', headers:{accept:'text/event-stream'}})` is **still open after 8 s**. At filing time it closed after **276 ms**, having sent one replayed frame and no `event: error`.
- **NDJSON stays open.** The same request with `accept: application/x-ndjson` is still open after 8 s. At filing time it closed after **298 ms**.
- **The next run arrives live.** With that Logs tab open and no reload, Events → Trigger Run → the run's output line (`qa-cron-ran` for an `echo qa-cron-ran` job) appears in the open tail, and no additional subscribe request is made.
- **Control (must not regress):** a Running `web_service`'s Logs page makes 1 subscribe, shows no banner over 20 s, and its SSE fetch is still open at 6 s. This passed at filing time on `eden-dash-v3` (`srv-d9ndt8hmcglc739fkp50`, read-only).
- **Terminal cases still end:** deleting the service ends the tail (at filing time the resubscribe after deletion got a 404), and revoking `can_view_logs` still ends it within one watchdog interval.

## Evidence (probes run 2026-09-14, production, workspace `bex` / `tea-d98210cbbpdc73dcrkvg`)

Fixture: cron job `qa-20260914-cron` (`srv-dajrb6i6m8ac739r5j10`, free, `echo qa-cron-ran`, schedule `0 0 * * 0`), after one manual run `crr-egur44u0t9uf505t6p3j` (`successful`, 08:56:49Z → 08:56:58Z). Created and deleted within the run.

```text
GET /v1/logs/subscribe?resource=srv-dajrb6i6m8ac739r5j10   Accept: text/event-stream
→ 200 text/event-stream, closed after 276 ms, full body:
id: 2026-09-14T08:56:55.056548271Z
data: {"id":"srv-dajrb6i6m8ac739r5j10-6hngegougdjv7qdiu9o0-2026-09-14T08:56:55.056548271Z-0eef5cb6","message":"qa-cron-ran","timestamp":"2026-09-14T08:56:55.056548271Z","labels":[{"name":"type","value":"app"},{"name":"resource","value":"srv-dajrb6i6m8ac739r5j10"},{"name":"instance","value":"srv-dajrb6i6m8ac739r5j10-6hngegougdjv7qdiu9o0"},{"name":"container","value":"app"}]}

GET … Accept: application/x-ndjson → 200 application/x-ndjson, 1 line, closed after 298 ms

Logs page, fresh load, 60 s: subscribe responses at [1.9, 5.1, 8.4, 11.7, 15, 18.2, 21.5, 24.8, 28, 31.3, 34.6, 37.8, 41.1, 44.4, 47.7, 50.9, 54.2, 57.5, 60.8] s, all 200
banner samples every 5 s: B B B B B – B B B B B –

Control, eden-dash-v3 (web_service, phase Running): 1 subscribe in 20 s, no banner; SSE fetch still open when aborted at 6 s
```

Reproduced on three separate fresh loads (30 s, 30 s and 60 s samples). No duplicate lines appeared, because `Last-Event-ID` resume (w6/m93) holds. No screenshots were taken; the transcripts above are the evidence.

## Root cause

`lego/backend/internal/logs/service.go:846-884` (`FollowLogs`) lists the App's pods **once**, at subscribe time (`AppPodsIn`, `lego/backend/internal/core/base.go:1644-1652`, with no phase filter, so the Succeeded cron run pod is included). It starts one `streamPodLogs` per pod, and closes the channel when every follow has returned (`:866`). It then returns `nil` (`:873-874`). The follow of an already-terminated container ends as soon as its log is replayed, so the whole subscription ends. With zero pods, `wg.Wait()` returns immediately and the stream ends the same way (reasoned; see Unverified). `subscribeStream` (`lego/backend/internal/logs/rest.go:397-416`) writes a frame only on a non-nil error, so the response simply ends.

On the client, EventSource treats any server close as an error. `dashboard/src/features/logs/hooks/use-live-logs.ts:250-254` sets `status: "error"` for an error event with no `data` (read as a transport drop), and the browser reconnects after its default retry. `dashboard/src/features/logs/components/log-viewer.tsx:194` shows `logs.disconnected` while `status === "error"`. Neither side is wrong about its own contract. The server is wrong to end a subscription that has nothing wrong with it.

## Fix — the target behavior, named

A subscription on an App **stays open for as long as it is authorized and the App exists**:

- It writes an SSE comment heartbeat (`: keepalive`, and an equivalent for NDJSON/WebSocket) at an interval below every hop's idle timeout.
- It re-lists pods on an interval and starts a follow for each pod it is not yet following, whether it is the next cron run, a wake from hibernation, a new deploy's pod, or a scale-up. This is the shape `followBuildLogs` already uses for build phases (`service.go:960-`, w6/m123).

A pod already followed to EOF is not re-followed. Resume via `Last-Event-ID` and the millisecond dedupe (w9/053) still covers genuine reconnects. The dashboard then stays on `status: "open"` (the `onopen` at `use-live-logs.ts:237`), and the banner predicate at `log-viewer.tsx:194` is false. **Only the backend has to move for the loop to stop.** The consumer predicate needs no change, because heartbeat comments fire neither `onmessage` nor `onerror`.

## Blast radius

- `FollowLogs` has **2** production callers (grep 2026-09-14): `rest.go:338` (`subscribeWebSocket`, selected at `rest.go:195`) and `rest.go:397` (`subscribeStream`, SSE and NDJSON, `rest.go:198`). There is no GraphQL or MCP live-tail caller. The fix is global, so both transports change together.
- Consumers: the dashboard Logs page (`log-viewer.tsx` → `useLiveLogs`) and the official Render CLI's `render logs --tail` (over the NDJSON/SSE path).
- **Not** affected: `type=build` subscriptions (`followBuildLogs`, a separate producer with its own `retryDelayMs: BUILD_RETRY_MS`, `dashboard/src/features/deploys/hooks/use-deploy-logs.ts:145-153`), and Postgres/Key Value (refused up front, `service.go:785-796`).
- Correct today and must stay correct: a Running service's tail (the control above).

## Adjacent classes

- **Capacity:** an idle-but-open tail now holds a subscription slot for the whole page view, instead of ~300 ms out of every ~3.3 s. The caps are `MaxSSEConns` / `MaxSSEConnsPerSubject` / `MaxSSEConnsPerWorkspace` (`service.go:174-181`, enforced at `rest.go:218-256`; `BEX_MAX_SSE_CONNS` defaults per `lego/backend/CLAUDE.md`). t002 must confirm a user with several Logs tabs across idle services is not starved into `log subscription capacity reached`, or it must change the cap policy.
- **No producer at all** (codex #3, `service.go:836-844`): a query whose _type_ has no live producer (e.g. `type=predeploy`) must still be refused immediately and never park. "This App has zero pods right now" is a different case from "this query type can never have a producer".
- **Authorization lifetime:** the watchdog (`service.go:829-832`) must still cancel an idle stream on revocation or App deletion.
- **Edge idle limits:** `api.bex.co` sits behind Cloudflare. The heartbeat interval must stay under its idle-stream cutoff, which is Unverified (t002 measures it).

## Unverified (reasoned, not probed this run)

- Zero-pod states: a suspended service, a hibernated free `web_service`, a service before its first pod. They are expected to loop the same way, via `wg.Wait()` with no goroutines, but none was opened live.
- The WebSocket transport (`rest.go:323-338`), and the Render CLI's `logs --tail` itself. Only the NDJSON wire form was probed.
- That the Kubernetes follow of a terminated container returns at EOF. Inferred from the observed 276 ms close; `PodLogsFollow`'s client-go path was not read.
- Whether the next scheduled run's output reaches an already-open tab today on its next reconnect. It probably does, a few seconds late, but no second run was observed with the tab open.
- The Cloudflare idle-stream timeout in front of `api.bex.co`.

## Live verification (2026-09-15, production pinned to `c4212ec71`)

- **Fixtures.**
  - `qa-20260915-m146` (`srv-dakecao1e15c73bfgprg`): a free cron job from `examples/hello-go` running `echo qa-cron-ran`, with a Sunday schedule, run by hand.
  - `qa-20260915-m146pf` (`srv-dakfshjidljc7398tnng`): a free web service tailed before its first pod.
  - `qa-20260915-m147` (suspended) and `qa-20260915-m151ws` (hibernated), borrowed from the same run.
- **Before (production still on `137a5186e`, 06:49:08Z).** On the cron job after one successful run, the SSE tail closed after 336 ms (413 bytes, the replayed `qa-cron-ran`), and NDJSON after 231 ms.

| Check | Result |
| --- | --- |
| SSE stays open (DoD) | 07:32:56Z: `200`, still open after 8 s |
| NDJSON stays open (DoD) | 07:33:04Z: `200`, still open after 8 s |
| No reconnect loop (DoD) | 07:33:10Z, cron Logs page for 60 s: exactly 1 `/v1/logs/subscribe` (at 1.7 s); the "Live tail disconnected" banner never appeared (sampled every 5 s) |
| The next run arrives live (DoD) | A run triggered at +63.7 s: the second `qa-cron-ran` line appeared in the open page at +69.7 s, with 0 extra subscribes and no banner |
| Web control (DoD) | A running web service's Logs page for 21 s: 1 subscribe, no banner |
| Deleting the service ends the tail (DoD) | `qa-20260915-m146pf`: tail open at 09:17:26.850Z; `DELETE` → `204` at 09:17:27.239Z; keepalives at 09:17:47Z and 09:18:12Z; the response ended at 09:18:22.115Z, 54.9 s after the delete (within the 1-minute revalidation watchdog). Then `GET` → `404` and a new subscribe → `404`. The cron job's own deletion (09:14:12Z) also ended its tail; that run did not capture the time. |
| WebSocket transport (t003) | 08:12:44Z: `101 Switching Protocols` without an Origin header; the last run replayed; pings at 08:13:09, 08:13:34, 08:13:59 and 08:14:24 (25 s); a run triggered at 08:13:19Z delivered its `qa-cron-ran` frame at 08:13:24Z; held for 100 s. The same upgrade with `Origin: https://dashboard.bex.co` gets `403`: the default gorilla upgrader's same-origin check. The Render CLI sends no Origin. |
| Render CLI tail (t003) | A temporary API key (created, then revoked `204`) exchanged for a 15-minute token; `BEX_ACCESS_TOKEN=… bex logs --resources srv-dakecao1e15c73bfgprg --tail -o text` printed the two earlier runs, kept running through 35 s idle, printed `2026-09-15 08:19:27  qa-cron-ran` for a run triggered at 08:19:25Z, and was still running at 95 s |
| Suspended service (t003) | 08:13:49Z: the suspended `qa-20260915-m147` (phase Hibernated) tail → `200`, open after 8 s |
| Before the first pod (t003) | 08:13:59Z: `qa-20260915-m146pf` tailed right after create → `200`, open after 8 s |
| Hibernated service, and a wake's line arrives (t003) | `qa-20260915-m151ws` hibernated at 09:11:26Z; its tail → `200`, open after 10 s; wake requests from 09:11:50Z (`503`), `200` at 09:12:02Z; the new pod's `ws server on 3000` arrived in the open tail at 09:11:53Z, and the tail stayed open |
| Subscription cap (t002) | 08:11:58Z, twelve concurrent tails as one user: tails 1–10 accepted and held open, tails 11 and 12 got `429 too many active log subscriptions`. The per-subject share (5) is counted per bex-api process, and production runs 2 replicas; `docs/ADR006-bex-api.md` now says so. |

- **Not probed live.**
  - The dashboard Logs page (1 subscribe, no banner over 60 s) on the suspended, hibernated and pre-first-pod services: the Playwright browser disconnected before those checks. Their streams passed at the SSE level above.
  - Revoking `can_view_logs` on an open tail. That stays pinned by `TestIdleTailStillEndsOnRevocationAndDeletion`.
- **Found along the way.** The CI recipe in `docs/bex-cli.md` exchanged the API key with HTTP Basic, which Hydra refuses (`401 invalid_client`), because API-key clients authenticate with `client_secret_post`. The recipe now sends the credentials in the form body and names `https://oauth.bex.co/oauth2/token`. The form-body exchange drove the CLI tail above.

## Dedupe

- `w9/done/053.md` recorded this banner as "transient" on 2026-08-20 and fixed only the duplicated-lines half (millisecond dedupe). The banner and loop were not investigated. This is the unowned half, with a measured cause.
- `w6/m93` added `Last-Event-ID` resume, which is why the loop no longer duplicates lines, and which is also why it has been easy to miss.
- `w4/done/m96`'s DoD verified cron logs with Live **disabled**.
- No open milestone covers it; `w11/m7`'s EventSource work is the mobile agent attach, not service logs.
- `.pm/DO_NOT_DO.md`: no conflict. External log drains are a non-goal; the live tail is core.
- Not fixed on `main`: `service.go:873-874` is unchanged at `34d0fa153`.

## Source + Goal linkage

- **Source:** live `/qa-find-bugs` hunt of `https://dashboard.bex.co`, 2026-09-14 (run from a `/loop`, filed to w1 by user direction), journeys 7 (logs) and 10 (cron job). Governing docs: `docs/ADR038-cron-jobs.md`, `docs/ADR018-render-parity.md` (logs rows), `docs/ADR006-bex-api.md`.
- **Goal linkage:** pillar 1, Render-compatible observability. `/v1/logs/subscribe` is Render's contract, and a tail that hangs up on idle services breaks both the dashboard and the official CLI.
- **Expected outcome:** the Logs page of any service that is not currently running reads as live and quiet, not broken. The next run's output appears as it happens, and bex-api stops serving a steady ~0.3 subscribe/s per open tab of an idle service.
- **Why now:** cron jobs (w1/m15, w5/m18) and free-tier sleep (w1/m4) make "a service with no running instance" the normal state for a large share of services, not an edge case. The banner has already been seen and misfiled as transient once (w9/053). Resume (w6/m93) now hides the duplicate-line symptom, so the loop no longer produces a visible defect that would force someone to look.
- **Render parity:** included. The fix changes the wire behavior of Render's `GET /logs/subscribe` (idle streams stay open, with heartbeats), which the official CLI consumes.
