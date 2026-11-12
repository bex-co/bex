# w8 · m158 — A crash or an out-of-memory kill of a running service is recorded with its cause

**Worker:** worker8 **Goal:** when the process of a running service exits or is killed for memory, the Events timeline records it once, with the exit status or the memory limit and the instance, even when the restart is quick, on REST, GraphQL, MCP and the dashboard. **Status:** in progress (t001–t007 done and shipped; the round 176 live replay on pin `3d66946c9e16` passed DoD 2, 4, 5, 6 and 7 and failed DoD 1 and 3, so t009 fixes those before the t008 closeout)

## Tasks (in order)

| id   | title                                                                                                    | est | depends_on |
| ---- | -------------------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | The operator records how a serving instance's app container exited — **DONE**                            | 60m | —          |
| t002 | Each exit becomes one `server_failed` with its cause and instance — **DONE**                             | 60m | t001       |
| t003 | REST, GraphQL and MCP carry the exit status, the memory limit, an early exit and the instance — **DONE** | 30m | t002       |
| t004 | Dashboard words the three causes for a service — **DONE**                                                | 30m | t003       |
| t005 | Render parity — **DONE**                                                                                 | 20m | t003, t004 |
| t006 | Simplify — **DONE**                                                                                      | 15m | t005       |
| t007 | Test coverage — **DONE**                                                                                 | 20m | t005       |
| [t009](done/t009.md) | One `server_failed` per outage, whatever arrives first: a 10 s crash records two, and a health-check outage within 3 minutes of a recovered crash records none and leaves an unpaired recovery — **DONE** | 60m | t007 |
| t008 | Closeout (live replay on production)                                                                     | 30m | t009       |

## Definition of done

1. A free web service that calls `process.exit(3)` once: within a minute `GET …/events?type=server_failed` holds one event with `reason.nonZeroExit: 3` and an `instanceID`, even though the service answered 200 again within 15 seconds.
2. The same service killed for memory: `reason.oomKilled.memoryLimit` names the plan's limit. `process.exit(0)`: `reason.earlyExit: true`.
3. A service whose health check path starts answering 500 keeps `reason.unhealthy`.
4. The dashboard Events tab shows the three causes in words, in English and Chinese.
5. A rollout failure still reports through `deploy_ended` and adds no `server_failed` for a service that never went down.
6. A crash that also lasts long enough for the sampled outage adds no second `server_failed`; its `server_available` still follows.
7. Fixtures deleted afterwards.

## Design (decided 2026-11-11, w8 drain)

- **The operator observes, the backend records.** The operator reads the app container's last termination from the pods of the release that is serving and keeps the newest few in `App.status.instanceExits` (instance, restart count, exit code, out-of-memory, finish time). It is the read `recordRolloutExit` already makes for a failing rollout. bex-api turns each into one fact keyed by instance and restart count, so a resync replays it without duplicates.
- **One event per crash.** The backend's sampled outage (a service not Ready on two passes) keeps recording `server_available`, and keeps recording `server_failed` for a process that is running and not Ready. When an exit was recorded for the same service inside the outage, the outage adds no `server_failed` of its own.
- **Paging.** The event is recorded either way. Alert emails are digested per recipient, so a crash loop does not send one mail per restart. Whether a single quick crash should page as Critical is ADR052's question and is left as it is.

## Implementation notes (2026-11-11)

- **What counts as the application exiting.** The app container's latest termination on a pod of the serving release, after that release's rollout settled: read while the container waits to restart, under the restart count its next run will carry, and again once it runs (t009; a first crash waits a 10 s back-off with a count of 0, and read only after the restart its exit reached bex-api after the outage it caused). Not counted: a container stopped from outside with SIGKILL or SIGTERM (exit 137 or 143 without the out-of-memory reason); a pod of another release; a sidecar; a crash while the release was still rolling out (it came up after, and the previous release served until then); an exit first seen more than ten minutes after it happened, so the operator's first pass does not report days-old restarts as new.
- **An exit that followed the outage.** When the instance had already stopped being ready more than three seconds before its process exited, the exit is recorded with `afterUnready` and bex-api writes no event for it. That is the platform restarting an instance that kept failing its health check, whose outage reads `unhealthy` (definition of done 3), and every exit of a crash loop after its first (one event per outage, not one per restart). The mark is decided the first time the exit is seen and kept.
- **Where it is read.** On the pass that finds the release Ready again, and on the pass that finds the served release's pods unready, so a process that never recovers is recorded too.
- **Status.** `App.status.instanceExits`, newest first in a total order, ten at most; unchanged passes rewrite nothing.
- **Facts.** One `server_failed` per unmarked exit, keyed `exit:<service>:<instance>:<restart count>`, dated at the exit, with `reason_code` `non_zero_exit` (and the exit code), `oom_killed` or the new `early_exit` (migration 0235 admits it).
- **One event per crash.** An exit and a sampled outage are one outage when the exit comes at most a minute before the outage's start or 5 s after it, with no `server_available` between them (t009). Whichever is recorded second adds no `server_failed`: normally the exit is first and the sampled outage adds none; an exit read after its outage was recorded adds none either. The outage's `server_available` is still written.
- **Memory limit.** The fact does not store it. The read fills `reason.oomKilled.memoryLimit` from the service's current plan ("512Mi" on Free), so an event from before a plan change shows the new limit.
- **Found by review and fixed before commit.** A new release that crashed once while starting and then came up was reported as a failed instance (now only exits after the rollout settled count). A health-check restart of a process that exits cleanly on the signal was reported as the application exiting, and a crash loop wrote one event per restart (now marked `afterUnready`). The kept set could flip at the bound when exits tied on their second (now a total order).
- **Not done.** The mobile app shows the raw reason code for the new causes (another workstream's screen). Whether Render groups a crash loop is unverified.

## Post-release probe (QA loop round 176, 2026-11-12): DoD 1 and 3 fail

- **Pin:** `3d66946c9e16`, deployed 07:27 UTC; it carries t001–t007.
- **Fixture:** Free web service `qa-20261112-r176-crash` (`srv-dbqmmsiibtfs738vknt0`, `node:22-alpine`, Health Check Path `/health`). Its `node -e` server exits on `/exit3` and `/exit0`, allocates memory on `/oom`, and answers 500 on `/health` after `/sick`. It was deleted afterwards (DELETE 204, then GET 404), which is DoD 7.
- **DoD 1 — fails.** `/exit3` at 07:33:48; the service answered 200 again from 07:33:59.
  - `GET …/events?type=server_failed` holds **two** events at 07:33:49: `{"instanceID":"…-h7vz4","reason":{"nonZeroExit":3}}` and `{"reason":{"unhealthy":"The instance failed its health check"}}`.
  - The sampled one was recorded on a pass at 07:34:01, before the exit fact existed. See t009, defect 1.
- **DoD 2 — passes.**
  - `/oom` at 07:35:11 → one `server_failed` at 07:35:20, `{"oomKilled":{"memoryLimit":"512Mi"}}` with `instanceID`; `server_available` at 07:36:10.
  - `/exit0` at 07:36:54 → one `server_failed` at 07:36:55, `{"earlyExit":true}`; `server_available` at 07:37:41.
- **DoD 3 — fails after a recent crash, passes without one.**
  - `/sick` at 07:38:07 (503 from 07:38:20) recorded **no** `server_failed`, then an unpaired `server_available` at 07:41:12. The 3-minute exit window covered the already-recovered 07:35:20 and 07:36:55 exits (t009, defect 2).
  - The control `/sick` at 07:42:47 recorded `server_failed` `unhealthy` at 07:43:07 and `server_available` at 07:45:48.
- **DoD 4 — passes.**
  - en reads "The instance's process exited with status 3 and was restarted.", "The instance ran out of memory (512 MB limit) and was restarted.", "The instance's process exited and was restarted. A service is expected to keep running." and "A running instance failed its health check.", each with "· Instance <pod>".
  - zh reads "实例进程以状态码 3 退出，已重新启动。", "实例内存不足（上限 512 MB），已重新启动。", "实例进程已退出并重新启动。服务应当持续运行。" and "运行中的实例未通过健康检查。".
- **DoD 5 — passes.**
  - A PATCH of the command to `node -e '…process.exit(1)'` recorded `commands_changed`. `POST …/deploys` closed `update_failed` at 07:47:10: "container exited shortly after start and is restarting repeatedly (last exit code 1) — check the service logs for the crash output…".
  - No `server_failed` appeared through 07:48:35, and the previous release kept answering 200.
- **DoD 6 — passes.** The OOM outage (about 40 s) and the `exit(0)` outage (about 45 s) each added no second `server_failed`, and their `server_available` followed.

## Source + Goal linkage

- **Source:** inbox note `w8/263` (promoted 2026-11-11), `/qa-find-bugs-cli` w8 loop sweep `9dd8`. Its finding follows.
- **Goal linkage:** Render parity ("An `Instance Failed` event occurs when your service's application exits unexpectedly… the event might include an exit code").
- **Expected outcome:** a user whose instance is killed for memory is told so, not sent to look at a health endpoint.
- **Why now:** major. The wrong cause is reported for the most common runtime failures.

## Finding (2026-11-11 UTC, `/qa-find-bugs-cli` w8 loop sweep `9dd8`)

Why: on Render the Events timeline is where a user learns that their app exited with status 1 or ran out of memory. On bex a crash that restarts within about half a minute leaves no event, and a longer one is recorded as a failed health check on a service that has no health check path. The exit status and the out-of-memory kill appear nowhere: not in the event, not in the dashboard, not in the logs. A user whose 512 MB instance is being killed for memory is sent to look at their health endpoint.

**Severity:** major (wrong cause on the most common runtime failures; the restart itself works). **Attribution:** server (operator status and backend event facts). **Estimate:** milestone-sized, ~4h. **Source:** `/qa-find-bugs-cli` w8 loop sweep `9dd8`, 2026-11-11 UTC, released `bex v0.5.0` (Render CLI v2.35.0), `https://api.bex.co/v1/`, production pin `7ca0227aa477`, workspace `bex-canary`, QA human device login.

### Evidence

One free native Node web service, no health check path, whose server exits or allocates memory on request:

```text
$ bex services create --name qa-20261111-9dd8-crash --type web_service --repo https://github.com/bex-co/bex --branch main \
    --runtime node --root-directory examples/static-site --plan free --build-command true \
    --start-command "<server with /crash?code=N, /exit0 and /oom>" --auto-deploy=false -o json --confirm
```

Two runs, the second on a fresh instance after `bex deploys create <srv> --wait`. Times are UTC; the outage is what `curl` saw every 2 s.

| Run | What the process did | Outage seen | Event recorded |
| --- | --- | --- | --- |
| 1 | `process.exit(3)` at 13:05:14 | 502 then 503 for ~14 s | none |
| 1 | killed at 531 MB RSS at 13:06:40 | 503 for 27–41 s | none |
| 1 | `process.exit(7)` at 13:08:02 | 502 then 503 for ~46 s | `server_failed` 13:08:16Z, `server_available` 13:08:48Z |
| 2 | `process.exit(3)` at 13:14:33 | 502 then 503 for ~13 s | none |
| 2 | killed at 531 MB RSS at 13:15:50 | 503 for ~34 s | `server_failed` 13:16:07Z, `server_available` 13:16:30Z |
| 2 | `process.exit(0)` at 13:17:29 | 502 then 503 for ~27 s | `server_failed` 13:17:35Z, `server_available` 13:17:55Z |

Every recorded `server_failed`, whatever the cause, reads the same on `GET /v1/services/<srv>/events` and `GET /v1/events/<id>`:

```json
{ "type": "server_failed", "details": { "reason": { "evicted": false, "unhealthy": "The instance failed its health check" } } }
```

- GraphQL `serviceEvents` gives `reasonCode: "readiness_failed"`, `instanceId: ""`, `reason: null`. The dashboard string for that code is "A running instance stopped passing readiness checks." (`dashboard/src/features/services/locales/en.ts:4592-4594`).
- `bex logs -r <srv> --type app` shows the app's last line (`RSS 531MB`, `CRASH exit 3`, `EXIT0`) and then the next start's `LISTEN 10000`. No line says the process exited or was killed.
- `bex services instances <srv>` shows the same instance id before and after: the container restarts in place.

### Expected

Render, [Troubleshooting deploys](https://render.com/docs/troubleshooting-deploys):

- "An `Instance Failed` event occurs when your service's application exits unexpectedly, or when Render restarts it after repeated health check failures."
- "If your application exits unexpectedly, the event might include an exit code."
- A service "can emit an event including an `Out of memory` (OOM) error."
- "Web services, private services, and background workers all run continuously and should never exit unless they receive a `SIGTERM` signal."

Render's API (the pinned OpenAPI, `lego/backend/internal/api/openapi/render-public-api-1.json`, "Server Failed"): `details.instanceID` and a `reason` with `nonZeroExit` ("the application exited with the specified non-zero status"), `earlyExit` ("the application exited early"), `oomKilled.memoryLimit` and `unhealthy`.

So on bex:

- exit 3 → `server_failed` with `reason.nonZeroExit: 3`
- memory kill → `reason.oomKilled.memoryLimit` naming the plan's limit (512 MB on Free)
- exit 0 → `reason.earlyExit: true`
- `unhealthy` only when a running process stopped passing its health check
- each of them recorded even when the restart is quick, with the `instanceID` of the instance that failed

### Root cause

1. **The cause is never observed.** `observedServiceStateFor` sets `obs.ReasonCode = EventReasonReadinessFailed` for every down service that is not a node loss (`lego/backend/internal/store/reconciler.go:2222`, `:2246`). `toFailureReason` turns that code into the fixed `unhealthyMessage` (`lego/backend/internal/events/rest.go:174-197`). The `non_zero_exit` and `oom_killed` codes exist, but only a cron run and a failed rollout fill them: the operator reads `LastTerminationState` in `recordRolloutExit` (`lego/operator/internal/controller/app_controller.go:3948-3978`, w8/m146) and stores it in `status.rolloutFailure`, which is written only while a rollout is failing. A running release has no equivalent.
2. **`instanceID` is never set.** `ObservedServiceState.InstanceID` is copied into the fact (`lego/backend/internal/store/event_facts.go:602`) and nothing assigns it.
3. **The event is a sampled outage, not a crash.** The backend looks at the App's Ready condition on a 30 s resync (`reconciler.go:83`) and records an outage only on the second unhealthy pass, 10 s after the first (`debounceUnhealthy`, `:376`; `unhealthyConfirmAfter`, `:328`). A crash whose restart is Ready again before that second pass is never seen. That is why the two exit-3 crashes and one of the two memory kills left nothing.

### Fix

1. Operator: when the app container of a serving pod restarts, record the termination in the App status: instance id, exit code, whether it was `OOMKilled`, the finish time and the pod's restart count. Use the read `recordRolloutExit` already does, for the active revision instead of a failing one.
2. Backend: emit one `server_failed` per recorded termination, keyed by instance and restart count so a resync replays it idempotently, with `ReasonCode` `non_zero_exit` (and the exit code), `oom_killed`, or a new early-exit code for status 0. Keep `readiness_failed` for a process that is running and not Ready. The sampled outage path stays for that case and for `server_available`.
3. REST and MCP: `reason.nonZeroExit`, `reason.oomKilled.memoryLimit` from the instance's plan, `reason.earlyExit`, and `instanceID`. GraphQL already has the fields.
4. Dashboard: word the three causes for a service ("exited with status 3", "ran out of memory (512 MB limit)", "exited"). The existing `non_zero_exit` and `oom_killed` strings say "The run …" and are for cron.
5. Decide with ADR052 whether a single quick crash should page as Critical; the event must be recorded either way.

### Acceptance

- A free web service that calls `process.exit(3)` once: within a minute `GET …/events?type=server_failed` holds one event with `reason.nonZeroExit: 3` and an `instanceID`, even though the service answered 200 again within 15 s.
- The same service killed for memory: `reason.oomKilled.memoryLimit` names the plan's limit. `process.exit(0)`: `reason.earlyExit: true`.
- A service whose health check path starts answering 500 keeps `reason.unhealthy`.
- The dashboard Events tab shows the three causes in words, in en and zh.
- A rollout failure still reports through `deploy_ended` (w8/m146) and does not add a `server_failed` for a service that never went down (w4/161, w4/533).

### Unverified

- What Render writes to the log stream when a running process exits. bex writes nothing; the docs only describe the event.
- Whether Render records one event per crash while a process is crash-looping, or groups them.
- Why the first run's memory kill was missed while the second's was caught. The sampling in root cause 3 explains it, but the reconciler's passes are not visible from the tenant side.

### Dedupe

w3/m302 (blocked on its live check) renamed the keys and maps a failed health check to `unhealthy`; it records that fields bex does not store stay omitted. w8/m146 (done) added the exit code and memory kill to `deploy_ended`. w4/196, w4/200 and w4/207 (done) fixed when the sampled outage is stamped. w4/m114 (done) is the cron run's reason. w4/m327 (open) is how a live service failing its health check reads in the dashboard. None records why a running web service's process ended.

### Cleanup

The service was deleted after the reads; it answers 404.
