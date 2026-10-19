# Finding — a free-tier wake is recorded as an outage recovery, and its wake event lands after it

Live functional `/qa-find-bugs w4`, pass 7d3e19, 2026-10-18/19 UTC, `muse.env` credentials, workspace `bex-canary` (`tea-daif693dqjvc73e7as3g`). Production was pinned to `d5c3373ed`; research is on main `5e75de920`. All requests, responses and event ids are in [evidence.json](evidence.json).

Fixture: `qa-20261018-7d3e19-egress` (`srv-dbalnfu804hs73aaebm0`), a Free web service created from the dashboard with the Existing Image `docker.io/library/python:3.13-alpine`. It runs `python3 -m http.server $PORT` and logs its egress IP once a minute. Settings → Idle timeout → **5 min**. It was deleted at the end of the pass.

## 1. A cold wake records `server_available` ("Service recovered") with no `server_failed`

- **Severity:** major. The product records a recovery from an outage that never happened. It shows the recovery in Activity, and the same fact feeds the opt-in Important push "Service recovered" and the `server_available` webhook. Intermittent: 1 of the 4 cold starts this pass, plus an independent sighting on 2026-10-02.
- **Repro:**
  1. Create a Free web service from an image that takes ~20 s to become ready, and set its Idle timeout to 5 min.
  2. Wait for `service_hibernated`.
  3. `curl -sS -i https://<slug>.onbex.co/` and poll it until it returns 200.
  4. Read `GET /v1/services/<id>/events` and the service's Activity page.
- **Expected:** the wake records exactly `service_hibernated` followed by `service_woken`. Render documents `server_available` as "A previously unavailable service became available", following `server_failed` ("A service became unavailable, usually due to a runtime error"). It publishes no spin-up or spin-down events (render.com/docs/webhooks, fetched 2026-10-19).
- **Actual (wake 1):**
  - `service_hibernated` at 23:59:25. A curl at 00:01:04 got `503 {"error":"service not ready","retryAfter":5}`, and the first 200 came at 00:01:25.
  - REST and GraphQL both list `server_available` `evt-5vdpjtesam68fbismh35` at **00:01:25**, then `service_woken` `evt-6o5oco1ubp651o2d26r6` at **00:01:40**.
  - The service's whole history contains **no** `server_failed`.
  - Activity on a fresh load reads, newest first: "Service woke up 15m · **Service recovered** 15m · Service went to sleep 17m".
- **Controls in the same pass:**
  - Wake 2 (browser interstitial, 00:08:13; `service_woken` 00:08:36) recorded no `server_available`.
  - Wake 3 (1 s polling: 503 at 00:15:01, 200 at 00:15:22; `service_woken` 00:15:25) recorded none either.
  - A user Resume (POST at 00:17:57, 200 at 00:18:09; `service_resumed` 00:18:10) recorded none.
  - `w4/done/m147/README.md:52` independently recorded "`service_woken` (07:03:27, plus `server_available`)" for a 35 s wake on 2026-10-02.
- **Root cause:**
  - `lego/operator/internal/controller/app_controller.go:3256-3257` (`reportRolloutProgress`). While the woken pod starts, the App reads `Phase=Deploying`, `Ready=False`, reason `RolloutProgressing`.
  - `lego/backend/internal/store/reconciler.go:1862-1866` (`observedServiceStateFor`) exempts ordinary rollout progress from availability only when `hasOpenDeploy` is true. A wake opens no deploy row, so the pass falls through to `:1873-1878`: `Availability = "unhealthy"`, `ReasonCode = readiness_failed`.
  - The debounce (`reconciler.go:293`, `unhealthyConfirmAfter = 10s`, `w4/207`) confirms that sighting whenever a second pass also lands before Ready.
  - The checkpoint left Hibernated with availability `""` (`reconciler.go:1830-1832`). So the `""` → `unhealthy` step emits nothing, and the later `unhealthy` → `healthy` step emits `server_available` (`store/event_facts.go:547-549`).
  - Whether a given wake trips this depends on where the reconciler's passes fall inside the startup window. That explains 1 of 3 wakes here.
- **Consumers checked:**
  - `notifications/push_worker.go:647-651` projects the fact as push "Service recovered", urgency Important. The comment eleven lines below (`:662-668`, w6/m47) excludes `service_hibernated`/`service_woken` precisely so that idle cycles do not page owners.
  - `webhooks/service.go:200` maps it to the subscribable `server_available` type (present in the live `webhookEventTypes`).
  - The events API (REST/GraphQL/MCP share `events.Service`).
  - The dashboard and mobile catalogs label it "Service recovered" (`dashboard/src/features/services/locales/en.ts:3928-3931`).
  - ADR052:113 states the intent: `server_available` closes the Critical `server_failed` page.
- **Fix (target behavior):** a startup from the Hibernated checkpoint (an idle wake or a user Resume) whose only Ready=False reason is ordinary scale-up progress is not an availability conclusion. The checkpoint keeps the hibernated baseline. The first healthy pass sets the baseline to healthy and emits nothing. The cycle records exactly `service_woken` / `service_resumed`.
  - The rule must key on the Hibernated baseline. It must not key on the `RolloutProgressing` reason alone, or a running service whose only instance disappears without a deploy would stop paging (see Adjacent classes).
  - Two seams can express this. In `RecordObservedServiceState`, the checkpoint already knows the held Hibernated phase (`checkpointServicePhase`, `event_facts.go:467-472`). Alternatively, the operator could write a distinct waking reason. The first keeps the change in one module; t001 decides.
- **Blast radius:** `observedServiceStateFor` has exactly 1 caller (`store/reconciler.go:476`). `observedStateFacts` emits all four phase facts and both availability edges for every service type. The fix must leave deploy-row rollouts (already exempt), real crash edges (`TestPGObservedCrashEdgeEmitsExactlyOnePair`) and Resume unchanged.
- **Adjacent classes:**
  - **Running service, no open deploy, unready (node loss or eviction):** stays `server_failed` then `server_available`. This is the control.
  - **Concrete container failure during a wake (CrashLoopBackOff/ImagePullBackOff, `concreteContainerFailure`):** today, from the `""` baseline, it would never record `server_failed`. That is reasoned from `event_facts.go:543-549` and was not probed. A service that cannot wake is down to every caller. Recommendation: record `server_failed` at the failure's transition time, so a later recovery pairs with it. t003 decides.
  - **User Resume:** same path. One 11 s probe passed.
  - **Datastores:** a separate projector (`datastore_event_facts.go`). Not examined.
- **Unverified:** push and webhook delivery were not exercised (no device or endpoint was created); they share the fact through the code paths above. MCP `list_events` was not called (same core). The failed-wake behavior and the datastore analogs were not probed.
- **Render:** see Expected.

## 2. `service_woken` is stamped when the reconciler notices, after the recovery edge from the same pass

- **Severity:** minor. With finding 1 the feed reads "recovered" before "woke up". Without it, the wake time is still 3–15 s late.
- **Evidence:**
  - Wake 1: first 200 and the Ready transition at 00:01:25, `service_woken` at 00:01:40 (+15 s).
  - Wake 3: first 200 at 00:15:22, `service_woken` at 00:15:25 (+3 s).
  - `w4/m147`: 200 at 07:03:15, `service_woken` at 07:03:27 (+12 s).
- **Root cause:** `store/event_facts.go:505-512`. `makeFact` stamps every phase fact with `obs.At`, the observing pass. Because `checkpointServicePhase` holds Hibernated through Deploying, the woken fact and the `server_available` edge come from the **same** pass. Only the edge is back-dated to the Ready transition (`availabilityEdgeAt`, `:496-501`, introduced by `w4/196`, which scoped itself to `server_failed`/`server_available`). Before `w4/196` both carried the same time.
- **Fix (target behavior):** `service_woken` carries the moment the woken instance became ready: the Ready=True `LastTransitionTime`, bounded by the same rule `availabilityEdgeAt` applies (after the checkpoint's last change, not after the observation).
- **Blast radius:** `makeFact` serves `service_suspended`, `service_hibernated`, `service_resumed` and `service_woken` (`event_facts.go:522-534`). This pass observed only the woken lag; `service_resumed` at 00:18:10 sat 1 s after its first 200. Each sibling needs an explicit disposition.
- **Unverified:** the hibernation lag (true scale-to-zero time not measured).
- **Render:** n/a (bex extension events).

## Dedupe

- Searched `.pm` open and done for `server_available`, "Service recovered", `service_woken` and `woken` with wake or hibernate.
  - `w4/196` and `w4/207` (done) fixed edge timing and debounce for real outages; neither covers a wake.
  - `w4/068` and `w4/m147` (done) recorded wake behavior and accepted the 503 + `Retry-After` wake response, which this finding does not dispute.
  - `w3/m78` and `w6/m47` (done) define the push semantics this finding relies on.
- `DO_NOT_DO.md:34` forbids filing self-waking without traffic. Every wake here was triggered by this pass's own request, so it does not apply.
- `git log` on `store/reconciler.go` and `store/event_facts.go` up to `origin/main` `5e75de920`: no fix landed.
