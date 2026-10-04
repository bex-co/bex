# w1 · m172 — A free service that sleeps through a failed rollout wakes onto the broken release: `503 service hibernated` while the API reads Running

**Worker:** worker1 **Goal:** a free service whose newest rollout failed over a prior release wakes on, and keeps serving, the release that last succeeded; its phase never reads Running while its URL can only answer the activator's 503. **Status:** done (2026-10-04, verified live on pin `e13afdd59e73`)

## Tasks (in order)

| id   | title                                                                                                                          | est | depends_on |
| ---- | ------------------------------------------------------------------------------------------------------------------------------ | --- | ---------- |
| t001 | Failed rollout over a prior release: restore the served release's pod template so wake, resume and scale run the prior release — **DONE** | 1h  | —          |
| t002 | Phase truth: never report Running from desired scale while no replica of the serving template can become ready — **DONE** | 30m | t001       |
| t003 | Live: reproduce on production, then verify wake after a failed rollout (both orderings) — **DONE** | 40m | t002       |
| t004 | Render parity — **DONE** | 20m | t003       |
| t005 | Simplify — **DONE** | 15m | t004       |
| t006 | Test coverage — **DONE** | 40m | t004       |
| t007 | Closeout — **DONE** | 10m | t006       |

## Definition of done

Run on a disposable free image web service (`docker.io/mendhak/http-https-echo:35`, `HTTP_PORT=3000`) that is live and returns `200`:

- **It wakes after a failed rollout.** `bex deploys create <srv> --image docker.io/library/memcached:1.6-alpine` closes `update_failed` with the TCP-probe line. Leave the service idle until `service_hibernated`, then request its URL every second: within 60 s the requests get the prior release's own `200`. At filing time every request for 6+ minutes after `service_woken` (20+ samples, 07:08:31–07:15Z on 2026-10-03) got `503 {"error":"service hibernated","retryAfter":5}` while REST read `phase: Running`, `replicas: 1`.
- **Same when the failed rollout starts while the service is hibernated** (the observed ordering): after the row closes, a request wakes the service and it serves the prior release within 60 s.
- **Phase truth.** `GET /v1/services/<srv>` reads Running only while the URL serves the prior release. The failed deploy stays `update_failed` and no new rollout of the failed image starts on wake.
- **Control (no regression).** A hibernated free service whose latest deploy is live still wakes on request (w6/m94 shape); w1/m156's failed-pre-deploy wake and w1/m157's failed-build wake still hold.

## Evidence

`.pm/w8/done/m44/README.md` § "Live closeout 2026-10-02/03", adjacent observation 1. Production pin `1263d12ae3fa`, `bex v0.2.1`, workspace `bex-canary`, fixture `srv-db09v90ehcmc739j12gg` (deleted). The service auto-hibernated at 06:45:36Z during an earlier rollout; the failing deploy `dep-db0acnqtm2ss7389qoc0` started while it was hibernated and closed at 07:08:31Z (`deploy_ended` + `service_woken`). Unverified at filing: the Deployment template and ReplicaSet state (no cluster access that run).

## Mechanism (current `main`)

- **Template stays on the failed release.** `settleFailedRolloutMessage` (`lego/operator/internal/controller/app_controller.go:3026-3036`) over a prior release only stamps `ConditionRollout` and calls `settleFailureOverPriorRelease`; nothing restores the Deployment's pod template. `reconcileKubernetes` keeps applying the failed `image` on every pass (`:2229` `applyServingDeployment`). `holdUnpassedRelease` (`:2207`, `:5404`, w1/m156) holds only a pre-deploy step that has not passed — nothing holds a failed rollout.
- **Waking scales the broken ReplicaSet (inferred).** Parked, both ReplicaSets are at 0; scaling the Deployment 0 → 1 gives the replica to the newest ReplicaSet (the failed template), so `ReadyReplicas` stays 0.
- **Route held on the activator.** `ingressBackend` (`:2629`) keeps the public route on the activator while nothing is ready, so the activator keeps answering `503 service hibernated` (`lego/operator/cmd/activator/main.go:218`).
- **Phase from desired scale.** `settleFailureOverPriorRelease` / `settlePriorRelease` (`:5108-5150`) derive Running from `dep.Spec.Replicas == 1`, so the API says Running while the URL is down — on every sleep/wake cycle until a new successful release.
- w1/m156 § Unverified ("The failed rollout deadline … probably not affected because `settleFailedRollout` runs after the Deployment write") guessed this path safe; this run shows it is not.

## Fix direction

- **t001.** When a rollout settles failed over a served release, re-apply that release's recorded pod template: reuse w1/m152's `servedPodTemplateForCancel` (`lego/operator/internal/controller/release_config_snapshot.go:463-484`, the `ReleaseRecordName` Secret), generalised from cancel to failed rollout, so later passes scale and route the prior release (m156's `convergeServingRuntime` invariant). If the record is missing (GC'd or pre-m152), fall back to scaling the prior ReplicaSet or report Failed honestly — never Running.
- **t002.** `settlePriorRelease` must not report Running when the template it scales is not the served one.

## Implementation (2026-10-03)

- **t001.** `settleFailedRolloutMessage` restores the served release's pod template at the verdict (`restoreServedTemplate`, `lego/operator/internal/controller/release_config_snapshot.go`): the w1/m152 release record first, else the template of the ReplicaSet the Deployment still retains for `status.activeRevision`. `holdFailedRollout` (`app_controller.go`, next to `holdUnpassedRelease`) then keeps the failed release off the template on every later pass and hands replicas and routing to m156's `settleHeldRuntime` / `convergeServingRuntime`. The hold is keyed on `ConditionRollout` false at the release generation, so a newer release ends it and rolls normally. Background workers are held the same way.
- **t002.** With nothing to restore from (no record, no retained ReplicaSet) the settle is `Failed` with the rollout's own reason, never Running. `ConditionRollout` is kept either way, so bex-api still closes the row `update_failed` with the diagnosis.
- **Known limit, kept from m156.** On the wake pass the phase reads Running as soon as the served template is scaled to 1, a few seconds before its pod is ready and the route leaves the activator (`settleHeldRuntime` settles from the scale it wrote; `TestWakeOverFailedPreDeployRestoresPriorRelease` pins that). The broken state in this milestone, Running with a template that can never become ready, is gone.
- **t006.** `wake_over_failed_rollout_test.go`: both orderings, the next deploy rolling normally, the ReplicaSet fallback, nothing-to-restore settling Failed, and a worker's suspend/resume. All six fail with the hold and the settle-time restore disabled. They run the full `Reconcile` on the fake client like the m156/m157 siblings (no Deployment controller in envtest either), with Deployment status written by hand. `failed_rollout_phase_test.go` fixtures gained the served ReplicaSet they implied.
- **t005.** `/simplify` (three reviewers): the held pass now skips the uncached record read once the template is back on the served revision, a failed record read in the hold returns the error instead of stamping Failed over a serving release, and the settle uses the Deployment it already holds. One finding is filed separately as `w1/123` (snapshot GC can reclaim the served release's record).
- **Verification.** `make test` and `make lint` from `lego/operator/` pass. `docs/ADR004-app-deployment.md` documents the rule.

## Live round 1 (2026-10-04, pin `54221109227c`)

Driven through the dashboard's GraphQL API with the QA browser session (`scripts/qa-login.sh`): the CLI device login asks for a password re-authentication an agent cannot perform. Workspace `bex-canary` (`tea-daif693dqjvc73e7as3g`), two free image web services on `docker.io/mendhak/http-https-echo:35`, `HTTP_PORT=3000`; the failing release is `setImage` to `docker.io/library/memcached:1.6-alpine` plus `triggerDeploy`.

- **Control.** `srv-db0vhumkrnec73b0q0t0`, hibernated on a healthy release: first request 09:01:50Z `503`, `200` at 09:02:11Z (21 s).
- **Fail, then sleep: holds.** `dep-db11c6clp43c73cufiog` started 09:02:17Z with a request every 20 s; closed `update_failed` at 09:17:23Z with the TCP-probe line; 57 of 57 requests got the prior release's `200`. Idle window set to 60 s, `service_hibernated` 09:25:10Z. Requests every second from 09:25:26Z: 13 x `503`, then `200` at 09:25:46Z (20 s) with the echo image's body, `service_woken` 09:25:40Z. Phase Running, the deploy still `update_failed`, no new deploy row.
- **Deploy while asleep, no traffic: found a second path.** `srv-db0vi0em0lvc73av9r9g`, `dep-db11c6klp43c73cufipg` started 09:02:18Z while hibernated. Nothing woke it, the rollout never ran, and bex-api closed the row at its 18-minute gate (09:20:24Z, generic health-gate line; that diagnosis is w6/m147's). The first request at 09:22:48Z woke it onto the failing template: `503` on every sample through 09:45:13Z (135 samples, 22 minutes), phase Deploying throughout. The parked pass had written the failing template at 0 replicas, so the wake was a scale-up, not a rollout: no `ProgressDeadlineExceeded`, no verdict, nothing for t001 to restore from.

## Implementation, part 2 (2026-10-04)

- `holdUnservedRelease` (`app_controller.go`): while parked, a release that has not served stays off the pod template (a mid-rollout park puts the served template back); a wake starts the served release's pod first, behind the activator, phase Deploying; once it is ready the normal path rolls the newer release over it as a real rolling update, which can fail into t001's restore. After `servedWakeBudget` (5 minutes of pod age) the newer release rolls anyway, so a served release that can no longer start does not block its fix. `holdNewerRelease` runs the three holds in order.
- Tests: the parked ordering rewritten to this flow, plus a mid-rollout park and the budget escape. All three fail with the hold disabled.
- **Overlap with w6/m147 t002** (settle or defer the park of an unsettled rollout): this takes the unsettled release off the parked template and re-rolls it on wake. It does not settle a verdict at park time or change the deploy row's reason; those stay m147's.

## Live round 2 (2026-10-04, pin `e13afdd59e73`) — DONE

Same fixtures, same GraphQL route. Both deleted afterwards (`service(id)` answers not found); the QA Kratos session was revoked with `scripts/qa-login.sh --logout`.

- **Deploy while asleep, then a request (the observed ordering): holds.** `srv-db0vhumkrnec73b0q0t0` hibernated; `dep-db13llnhb1uc73ebif7g` (memcached) created 11:39:02Z, phase still Hibernated 20 s later. Requests every second from 11:39:31Z: 7 x `503`, `200` at 11:39:42Z (11 s) with the echo image's body. The release then rolled over the live pod and closed `update_failed` at 11:54:50Z with the operator's TCP-probe line (not the backend gate). Phase Running; dashboard header "Service Running · Latest deploy Failed · Revision rev-1".
- **After that verdict, sleep and wake: holds.** `service_hibernated` 12:02:50Z; requests every second from 12:04:23Z: 7 x `503`, `200` at 12:04:34Z (11 s). No new deploy row.
- **The service round 1 left stuck recovers.** `srv-db0vi0em0lvc73av9r9g` had answered `503` from 09:22:48Z to 11:38:51Z on the pre-fix template (it never auto-hibernated in that time, cause not established: see `w1/124`). Suspend 11:39:50Z, resume 11:40:16Z: `200` at 11:40:37Z (21 s). Its release then rolled over the live pod and settled at 11:55:50Z, phase Running. Sleep and wake again: `503` from 12:04:35Z, `200` at 12:04:56Z (21 s).
- **Traffic during the two rollouts.** 51 of 57 samples `200` on the first fixture and 50 of 54 on the second. Both timed out together (curl `000`) from 11:53:03Z to 11:55:04Z, four to five consecutive samples each, and the first then answered one `503` at 11:55:34Z. The window starts before either settle and hits both hosts on the same sample, which points at something shared (ingress, node, or the probing machine's network), not at the hold; without cluster access the cause is unverified. Round 1's rollout had 57 of 57.
- **t004, parity.** GraphQL and the dashboard agree (above). REST and MCP were not exercised live (no CLI or API-key auth this run); all three project the CR phase verbatim (`lego/backend/internal/apps/service.go`) and bex-api's deploy close reads the unchanged `ConditionRollout`. Render keeps the last successful deploy serving after a failed one and wakes a spun-down free service on request; both now hold here. No drift filed.
- **Not done here.** `bex deploys create --image` (the DoD's exact verb) was replaced by `setImage` + `triggerDeploy`, the same operator path with a saved rather than per-deploy image. w1/m156 and w1/m157's wakes were not re-run live; their fake-client suites pass unchanged.

## Related

- [w6/m147](../../w6/m147/README.md) — the same failed-rollout-meets-auto-hibernate family: a park that lands **mid-rollout** erases the stall diagnosis (generic health-gate failure reason) and the dashboard header disagrees with the phase. Its t002 (settle or defer the park of an unsettled rollout) must leave a verdict that this milestone's t001 can restore from; coordinate, do not duplicate.

## Source + Goal linkage

- **Source:** w8/m44 live closeout (2026-10-02/03, `/qa-find-bugs-cli`), adjacent observation 1, filed 2026-10-03; sibling of w1/m156 (failed pre-deploy variant, fixed in `1cb1f2d27`) and w1/m157 (failed build variant).
- **Goal linkage:** free-tier sleep/wake must be invisible to users (ADR008 dense free tier; w6/m116 made auto-hibernate the default); Render keeps running the most recent successful deploy after a failed deploy (ADR018, ADR004 § "A held release does not freeze the serving one").
- **Expected outcome:** one bad deploy no longer takes a free service offline at its next sleep, and the API/dashboard stop saying Running for a URL that only returns 503.
- **Why now:** every free web service with a failed rollout over a live release is down from its next wake until a new successful deploy; w8/042 per-deploy `imageUrl` makes bad-image rollouts a routine CI outcome. m156 and m157 closed the two sibling paths, leaving this one.
- **Render parity included:** it changes the service phase on REST/GraphQL/MCP and the dashboard header.
