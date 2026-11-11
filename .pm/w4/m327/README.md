# w4 · m327 — A live service whose instance fails its health check reads "Deploying" on every surface though no deploy runs, and its saved settings are announced as going live "when the current deploy finishes"

**Worker:** worker4 **Goal:** a health-check outage of the release that is already live reads as that release's outage, not as a deploy. The service status agrees with the Deploys list, the saved-changes notice tells the user to deploy, and the Events reason names a health check. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | Capture Render's service status during an instance failure and record the target in ADR004 § Health checks | 30m | — |
| [t002](t002.md) | Operator: a served release whose pods go unready keeps phase Running when no newer release is requested or rolling | 60m | t001 |
| [t003](t003.md) | Operator: `undeployedChangesApplying` is true only while a newer release is requested or rolling | 30m | t002 |
| [t004](t004.md) | Blast radius: every reader of phase `Deploying` in the operator, backend and dashboard | 45m | t002, t003 |
| [t005](t005.md) | Dashboard: the `readiness_failed` Events reason names a health check, en and zh | 10m | — |
| [t006](t006.md) | Live verification on an owned Free image web service | 30m | t004, t005 |
| [t007](t007.md) | Render parity | 15m | t006 |
| [t008](t008.md) | Simplify | 10m | t007 |
| [t009](t009.md) | Test coverage | 45m | t007 |
| [t010](t010.md) | Closeout | 10m | t008, t009 |

Ten tasks, 285m (4h45m).

**Major (the product misleads).** During any health-check outage of a live service, the dashboard says a deploy is running and that the saved settings will go live when it finishes. Neither is true. A user waits for a deploy that doesn't exist, and the saved change stays unapplied after the instance recovers.

## Finding

Live `/qa-find-bugs w4` loop, round 100, 2026-11-11 11:38–12:01 UTC (`muse.env`, workspace `bex-canary`; production runs the code of `7ca0227aa477`).

Fixture: Free image web service `qa-20261111-hc100` (`srv-dbq57dq34ugc73cu229g`, `docker.io/traefik/whoami:v1.10.1`, `serviceDetails.healthCheckPath: "/health"`). Its one deploy `dep-dbq57dq34ugc73cu22a0` went `live` at 11:39:42. whoami's `POST /health` with body `500` makes its own `/health` answer 500 until the process restarts, which is a steady-state health failure with no deploy. The service was deleted afterwards (DELETE 204, then GET 404, and the name lists `[]`).

### Probes

- **The outage itself behaves as Render's does.** After `curl -X POST -d 500 https://qa-20261111-hc100.onbex.co/health` at 11:41:42:
  - `/` answers 200 at 11:41:50, then `503` from 11:41:59 (`x-render-routing: no-server`, `{"error":"service not ready","retryAfter":5}`);
  - `200` again from 11:43:59, after the liveness restart;
  - `GET /v1/services/srv-dbq57dq34ugc73cu229g/instances` keeps the single instance `…-i6slqdlq0chc9g1ma1hd` created 11:38:36, so the container restarted in place;
  - Events: `server_failed {"reason":{"evicted":false,"unhealthy":"The instance failed its health check"}}` at 11:42:08, then `server_available` at 11:43:55.
- **A saved Health Check Path waits for a deploy, as documented (ADR004:247).** `PATCH /v1/services/srv-dbq57dq34ugc73cu229g {"serviceDetails":{"healthCheckPath":"/api"}}` → 200 and the path reads `/api`. For the next 85 s no deploy was opened and the instance list didn't change. With the instance healthy, Settings shows `status: Saved changes aren't live yet. Use a standard deploy to apply them.` That is correct.
- **During the next outage, every surface claims a deploy.** `POST /health 500` again at 11:58:44. At 11:59:14–11:59:34:
  - GraphQL `service(id:"srv-dbq57dq34ugc73cu229g") { phase undeployedChanges undeployedChangesApplying undeployedChangesReason healthCheckPath }` → `{"phase":"Deploying","undeployedChanges":true,"undeployedChangesApplying":true,"undeployedChangesReason":"saved","healthCheckPath":"/api"}`;
  - `GET /v1/services/srv-dbq57dq34ugc73cu229g/deploys?limit=3` → only `dep-dbq57dq34ugc73cu22a0: live`;
  - dashboard Settings header: `text: Service Deploying`, `link "Latest deploy: Live"`, and `status: Saved changes go live when the current deploy finishes.`;
  - dashboard Overview row: `row "qa-20261111-hc100 Web Service Deploying 20m …"`.
- **After recovery the claim turns out false.** At 12:01:19 (after `server_available` at 12:01:00), GraphQL reads `{"phase":"Running","undeployedChanges":true,"undeployedChangesApplying":false}`. No deploy ran, the instance was never replaced, and `/api` was never applied.
- **The Events copy names the wrong check.** The `server_failed` entry renders "Instance failed — A running instance stopped passing readiness checks." REST says "The instance failed its health check". Settings names the setting "Health Check Path". zh renders "运行中的实例未能通过就绪检查。"

### Root cause

- **The steady-state loss of readiness is reported as a rollout.**
  - `reportKubernetesRunning` (`lego/operator/internal/controller/app_controller.go:3315`) computes `rolloutDone` from current pod readiness (`:3336`). A live pod that fails its readiness probe makes `deploymentPodsReady` false, though the Deployment's template is the already-served revision.
  - `reportRolloutProgress` (`:3699-3709`) then stamps `Phase = PhaseDeploying` and `Ready=False/RolloutProgressing`, overlaid with the `HealthCheckFailing` stall diagnosis. It never asks whether a newer release exists.
  - The same file already states the rule for the deadline case: "A served release is excluded, because its pods going unready is not a deploy" (`:3798`, `scaledUpPastDeadline`).
- **The saved-changes notice trusts that phase.**
  - `releaseRequestedOrRolling` (`lego/operator/internal/controller/saved_configuration.go:170-176`) returns true for any `PhaseDeploying`.
  - `inFlightCarriesSavedConfiguration` (`:143-163`) takes `pending = requestedReleaseGeneration(app)`. A Save only stamps no release, so this falls back to `app.Generation` (`release_identity.go:744-749`), which the PATCH raised above the active revision. Finding no release record for that generation, it answers true (`:152-154`, "a normal release that has not dispatched yet … carries them").
  - So `Status.UndeployedChangesApplying` is set, and `undeployedChangesHintKey` (`dashboard/src/features/services/lib/status.ts:370-380`) picks `services.undeployedChangesApplyingHint` (`locales/en.ts:307-311`) in `service-detail-header.tsx:311`.
- **The header and list read the same phase.** `PHASE_STATUS.deploying` (`status.ts:256`) gives "Deploying".
- **The Events copy (separate cause).** `services.eventsReason.readiness_failed` (`dashboard/src/features/services/locales/en.ts:4592-4595`, `zh.ts:4485-4487`) says "readiness checks", the Kubernetes term. The reason code comes from `store/event_facts.go:112`, and REST maps it to `unhealthyMessage` (`lego/backend/internal/events/rest.go:174`).

### Fix (target behavior)

- **A served release's outage is not a deploy.** When the Deployment's template revision is the active revision and no newer release is stamped, requested or rolling, pods that lose readiness leave the phase `Running`. Ready stays `False` with `HealthCheckFailing` (or the existing stall, quota or node-loss diagnosis), as today.
  - The outage keeps being reported by `server_failed` / `server_available` and their notifications and webhooks, as Render reports an instance failure.
  - t001 confirms Render's service status during an instance failure. If Render shows a distinct unhealthy status, that becomes a follow-up, not a reason to keep "Deploying".
- **Unchanged:** a real rollout (a newer template), a pre-deploy step, and the wake or resume start of the served release (ADR004:322, "with the phase Deploying") keep `PhaseDeploying`.
- **The notice follows the release, not the phase.** `undeployedChangesApplying` is true only while a release newer than the serving one is stamped (`stampedReleaseGeneration`) or its rollout is in progress. During a served release's outage the notice stays "Saved changes aren't live yet. Use a standard deploy to apply them."
- **Pre-settle state:** the operator writes phase and saved-configuration status in separate passes (`app-config-status` queue). The notice must never read "applying" for a phase change alone, so a status read between the two passes shows the "Use a standard deploy" notice, or none.
- **Events copy:** "A running instance failed its health check." / "运行中的实例未通过健康检查。"
- **Surfaces:** REST `phase` and `undeployedChangesApplying` (`apps/render.go:173-180`), GraphQL `phase` and `undeployedChangesApplying` (`apps/graphql.go:360`), MCP's service read and the dashboard all read the same status fields, so the operator change covers all four.

### Blast radius

- **Phase `Deploying` readers:** 15 non-test references in `lego/operator`, 7 in `lego/backend`, and 4 in `dashboard/src`. Run `grep -rn "PhaseDeploying\|\"Deploying\"\|\"deploying\"" <dir> --include='*.go' --include='*.ts' --include='*.tsx' | grep -v "_test.go\|__tests__\|locales"` on each. The load-bearing ones:
  - `store/reconciler.go:2196-2205` treats `PhaseDeploying` with `Serving=True` as healthy (a new revision's failure is a deploy diagnosis). With the phase now `Running` during a served outage, `server_failed` must still fire, as it does today through the `readiness_failed` path.
  - `store/reconciler.go:1581-1593` maps an open deploy row under `PhaseDeploying` to `update_in_progress`. A served outage has no open row, but the tests must confirm a just-closed row isn't reopened.
  - `apps/crash_loop.go:75` (w2/m253 `stuck_crashlooping`) accepts Running, Deploying and Failed, so it is unaffected, but it needs a regression test.
  - `apps/container_policy_migration.go:143` and `store/product_inventory.go:220` count Deploying as converging.
  - Free-tier idle sleep defers its park mid-rollout (w6/m147). Decide and test whether an unhealthy served release may park.
  - Dashboard: `features/projects/lib/resource-health.ts:20` and `services/lib/status.ts:321` (`CONVERGING_PHASES`).
- **`releaseRequestedOrRolling` callers:** `inFlightCarriesSavedConfiguration` only, but w4/529 (a requested deploy counts as applying before it rolls) and w4/m184 must keep passing.

### Unverified

- Render's dashboard header and services-list status while an instance fails its health check (t001).
- A multi-instance service where only one instance is unready. `deploymentRolloutReady` requires every replica, so it presumably reads Deploying too. It wasn't probed here (the Free plan has one instance).
- A Starter or paid service, a private service, and a background worker (worker readiness has no probe; `app_controller.go:3661-3665` takes the same `reportRolloutProgress` path for any unready pod).

### Render

- [render.com/docs/health-checks](https://render.com/docs/health-checks): an instance that fails its health check is removed from rotation and then restarted. That is an instance event, not a deploy. Render's deploy list and service status keep describing the live deploy (t001 confirms the badge).

### Dedupe

- **w4/211 (done)** recorded `phase: "Deploying"` during this exact outage (its probe JSON) but fixed only the activator's public response. **w4/m184** and **w4/529 (done)** built `undeployedChangesApplying` for real deploys. **w5/m113 (done)** fixed event timing. **w2/m253 (done)** built crash-loop suspension.
- `grep -rln "reportRolloutProgress" .pm` finds no item about a served release's phase. `grep -rli "go live when the current deploy finishes" .pm` finds only the m184 / 529 lineage.
- Nothing open covers it: the open milestones were scanned with `find .pm -path '*/done' -prune -o -name README.md -print`.
- **Not deploy lag:** `git log 7ca0227aa477..origin/main` on `app_controller.go`, `saved_configuration.go` and `release_plan.go` lists only `7dec58bb5` (w4/m320), which doesn't change how a served release's unready pods are phased.

## Definition of done

Signed in to `bex-canary` after the release pins this build, on an owned Free web service `qa-<date>-hc` from `docker.io/traefik/whoami:v1.10.1` with `healthCheckPath: "/health"` (gate: t006):

1. After the first deploy is live, save a different Health Check Path (`/api`) through `PATCH /v1/services/{id}`. Settings shows "Saved changes aren't live yet. Use a standard deploy to apply them."
2. `curl -X POST -d 500 https://<slug>.onbex.co/health`, then wait 20 s. While `/` answers 503:
   - GraphQL `service { phase undeployedChangesApplying }` reads `Running` and `false`;
   - the dashboard header and the Overview row read "Running", never "Deploying";
   - the notice still reads "Saved changes aren't live yet. Use a standard deploy to apply them." (en), with its zh equivalent;
   - Events shows "Instance failed — A running instance failed its health check." (zh "运行中的实例未通过健康检查。").
3. The outage still produces `server_failed` and then `server_available` (REST `/events`), and the instance restarts in place.
4. **Controls:** a Manual Deploy reads "Deploying" and "Saved changes go live when the current deploy finishes" until it is live, and then the path reads applied. A Free service waking from sleep still reads Deploying while its pod starts.

## Source + Goal linkage

- **Source:** the finding above (round 100 of the `/qa-find-bugs w4` loop, 2026-11-11). The durable evidence is the probe record.
- **Goal linkage:**
  - [ADR004](../../../docs/ADR004-app-deployment.md) § Health checks and § save-only (`:247`).
  - [ADR018](../../../docs/ADR018-render-parity.md) Health checks row.
  - [ADR008 § Render parity first](../../../docs/ADR008-vision.md#render-parity-first).
- **Expected outcome:** during a health-check outage, the service's status and the saved-changes notice agree with the Deploys list, and a user knows to deploy to apply their saved change.
- **Why now:** every health-check outage of every live service shows this, and it is exactly when a user is reading the status to decide what to do.
- **Render parity included (t007):** REST, GraphQL, MCP and the dashboard read the changed status.
