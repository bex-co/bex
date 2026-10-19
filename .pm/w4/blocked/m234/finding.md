# Finding — "Save and deploy" on a suspended service says a deployment started; the deploy is canceled within seconds and never runs

Live functional `/qa-find-bugs w4`, pass 9d0542, 2026-10-19 UTC, `muse.env`, workspace `bex-canary` (Hobby). Production was pinned to `69e925197b39`, and research is on main at the same backend/dashboard code (no `secrets`, `rollout` or environment-editor commit since the pin). The probes are in [evidence.json](evidence.json). The owned fixture (`qa-20261019-9d0542-susp`, `srv-dbas8cf51eac73b158eg`, Free `docker.io/traefik/whoami:v1.10`) is deleted (`DELETE` 204, then `GET` 404).

- **Severity:** major. The dashboard and the API report a deployment that can never run. After Resume, the service runs the old values. Only a later standard deploy applies what the user "deployed".

## Repro (as performed)

1. Deploy a Free image web service to Live. Settings → **Suspend** → confirm. The URL answers `503 This service is suspended.` and Manual Deploy's items are disabled with "Unavailable while the service is suspended. Resume it first."
2. Environment → **Edit** → **Add variable** `QA_9D0542_FLAG=on`. The primary button still reads **Save and deploy**, and the options menu offers all three modes enabled: Save, rebuild, and deploy / Save and deploy / Save only.
3. Choose **Save and deploy**.
   - GraphQL `PatchServiceEnvironment` (`saveMode:"deploy"`) → `200 {"rolledOut":true,"rolloutError":""}`.
   - Toast: **"Environment saved and deployment started"**.
   - A config-change deploy (`dep-dbasbrf51eac73b158hg`) opens at 07:25:33 and closes `canceled` at 07:25:40 with "Canceled: the service was suspended". Events: `deploy_started` (`envUpdated:true`) → `deploy_ended canceled`.
   - The header now reads **Latest deploy: Canceled**, and the page says "Saved changes aren't live yet. Use a standard deploy to apply them." The standard deploy it points at is the disabled Manual Deploy.
4. **Resume.** The service serves again (`200`) on the prior release: header **Revision rev-2**, GraphQL `undeployedChanges: true`. No deploy row is opened by the resume. The saved variable is not live.
5. **Recovery (works):** Manual Deploy → Deploy latest image → `dep-dbasftn51eac73b158og` Live, `undeployedChanges: false`.

## Control and siblings (re-suspended, all over the API from the signed-in page)

| Write on the suspended service | Response | Deploy row |
| --- | --- | --- |
| GraphQL `patchServiceEnvironment` `saveMode:"rebuild"` (control) | `rolledOut:false`, `rolloutError:"conflict: service \"srv-dbas8cf51eac73b158eg\" is suspended"` | none |
| GraphQL `patchServiceEnvironment` `saveMode:"deploy"` | `rolledOut:true`, `rolloutError:""` | `dep-dbaseg751eac73b158kg` opened, then canceled (superseded by the next row) |
| REST `PUT /v1/services/{id}/env-vars/QA_9D0542_REST` (legacy immediate-roll write) | `200 {"key":…,"value":"1"}` | `dep-dbasein51eac73b158lg` opened, canceled "Canceled: the service was suspended" |
| REST `PATCH /v1/services/{id}` `{"serviceDetails":{"healthCheckPath":"/health-qa"}}` | `200` | `dep-dbasel74b8ns73ae2e80` opened, canceled "Canceled: the service was suspended" |

So the rebuild mode already has the right shape: the save stands, nothing rolls, and the response says why. Four ordinary saves on one suspended service left four canceled deploys and four `deploy_started`/`deploy_ended` pairs in its history.

## Root cause

- **Deploy mode has no suspended guard.**
  - `lego/backend/internal/secrets/batch.go:87-147`: `PatchEnvironment` checks only `servedArtifact` for deploy mode (`:118`).
  - `finalizeEnvironmentPatch` (`:349`) activates the pending references and bumps `restartedAt` for deploy mode (`:371-372`).
  - `:404` then answers `result.RolledOut = saveMode == SaveModeDeploy` unconditionally.
  - Rebuild mode instead goes through `deploys.Rebuild` → `validateTrigger`, which refuses a suspended App (`lego/backend/internal/deploys/service.go:630`, 409). `PatchEnvironment` turns that into `RolloutError` (`batch.go:139`).
  - Trigger and Rollback refuse a suspended service, and `deployActions` reports precondition `suspended` (`lego/backend/internal/deploys/actioncaps.go:161`). So the environment save is the one deploying verb that bypasses the gate.
- **The row opens, then the reconciler cancels it.**
  - `rollout.Snapshot.Stamp` (`lego/backend/internal/rollout/rollout.go:133`) decides "this write rolls a release" from the spec diff alone and never looks at `spec.suspended`.
  - `Tracker.Open` (`:234`) then files the config-change row.
  - The store reconciler sees an open row on a suspended App and closes it `canceled` with `suspendCancelReason` (`lego/backend/internal/store/reconciler.go:917`, `:1454`, `suspendEndsRollout` `:1464`). That close was built for w4/m171 (a suspend cutting off a running deploy), not for a deploy opened after the suspend.
- **The dashboard trusts the envelope, and gates nothing on suspension.**
  - `dashboard/src/features/services/components/service-environment-editor.tsx:170-181` reports the service as deployed when `rolledOut` is true, and `:644-651` toasts `services.environmentSaveDeploySuccess`.
  - The Save and deploy button and the three menu items (`:965-1009`) are gated only on `saveDisabled` (permission/busy).
  - Manual Deploy, by contrast, reads the `deploy` decision through `gateAction` (`dashboard/src/features/services/components/manual-deploy-button.tsx:98-110`, `dashboard/src/features/capabilities/lib/resource-actions.ts:180`) and shows `capabilities.blockedSuspended`.
  - The pending notice key (`dashboard/src/features/services/lib/status.ts:329-335`) has no suspended variant, so it tells a suspended service to "Use a standard deploy".
  - The rebuild toast shows `rolloutError` verbatim (`service-environment-editor.tsx:131`), so the control case would read `conflict: service "srv-…" is suspended`. That was code-read only; the UI rebuild path was not clicked this pass.

## Fix (target behavior)

A save on a suspended service is a save, never a deploy. Every surface reports it the way the rebuild control already does:

- The values persist, staged exactly as `save_only`: pending projection references, no `restartedAt` bump, no release stamp, no deploy row, no `deploy_started` event.
- `rolledOut: false`, and `rolloutError` says in product English that the service is suspended and must be resumed and then deployed. The same sentence covers deploy and rebuild modes; no `conflict:` prefix and no raw `srv-` id.
- REST `PATCH /v1/services/{id}/environment`, GraphQL `patchServiceEnvironment` and MCP `patch_service_environment` all share the one `PatchEnvironment` core path, so the fix lands there.
- Environment groups (`deploy` mode): a suspended linked service is reported in `pendingServiceIds` (as an Auto-Deploy-off service is), not rolled and not failed.
- The dashboard pre-empts it. While the service is suspended, Save and deploy and Save, rebuild, and deploy are disabled with the Manual Deploy reason ("Unavailable while the service is suspended. Resume it first."), the primary action is Save only, and the pending notice says to resume and then deploy.
- Resume keeps today's behavior: the prior live release serves with `undeployedChanges: true`, and the first standard deploy after resume applies the saved values (step 5).

## Blast radius

`rollout.Stamp`/`Open` sit under every release-rolling configuration writer, and a suspended App is never excluded:

- 19 `apps` call sites through `patch`/`patchFetched` → `patchTracked` (`lego/backend/internal/apps/service.go:4975`, `:4998`, `:5019`): start/build command, root dir, Dockerfile path, pre-deploy command, health check path, plan, disks. Operational verbs (scale, autoDeploy, domains, notifications) mint no row.
- 5 `secrets.rollApp` call sites (`lego/backend/internal/secrets/service.go:109`), including the legacy immediate-roll env-var and secret-file routes.
- 2 `envgroups.rollLinkedService` call sites (`lego/backend/internal/envgroups/service.go:1101`; `rollOne` `:1263` gates only Auto-Deploy).

Probed live: the GraphQL deploy-mode save, the REST env-var `PUT`, and the REST settings `PATCH`. Not probed: the env-group and secret-file writers, and the other settings fields.

## Adjacent classes

- **Forbidden / unauthenticated:** unchanged. The new suspended check must run after `scopeForWrite` (`RelCanCreate`, `batch.go:91`), so a caller without access never learns whether the service is suspended.
- **Billing-blocked:** keep `deployPreconditions`' order (suspended, then billing; `deploys/actioncaps.go:160-165`), so a service that is both reports the suspension.
- **Deleting service:** keeps `NotFoundIfDeleting`.
- **Suspend during an in-flight deploy (w4/m171, w5/m114):** unchanged. That close keeps "Canceled: the service was suspended".

## Unverified

- MCP `patch_service_environment` (same core path, not called).
- The env-group deploy mode with a suspended linked service (code-read: `rollGroupPatch` `envgroups/patch.go:217-252` reports it rolled).
- Whether the canceled phantom rows fire `deploy_started`/`deploy_ended` webhooks (w4/m171/t002 silenced failure notifications only).
- The dashboard rebuild-mode toast text.
- Render's behavior for a dashboard Save and deploy on a suspended service. Its docs only say API env changes never deploy on their own (`api-docs.render.com/reference/update-env-vars-for-service`), and that "Save and deploy" redeploys the existing build (`render.com/docs/configure-environment-variables`).

## Dedupe

- **w4/m171** (blocked) and **w5/m114** (blocked) cover a suspend that interrupts a deploy already running. This is the reverse order: a write made after the suspend.
- **w4/m145** (done) owns the Save only notices.
- **w4/m184** (blocked) owns the in-flight "applying" notice.
- **w4/216** (done) owns "a save that changed nothing rolls nothing".
- None covers a deploying save on a suspended service.
- Nothing in `.pm/DO_NOT_DO.md` matches.
- Port 3000 vs Render's 10000 (my first deploy's health-check stall) is the documented `PORT` divergence in ADR018 (w2/m95), not filed.
