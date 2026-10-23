# w8 · m135 — `PATCH /services/{id}` saves settings without deploying, as on Render

**Worker:** worker8 **Goal:** A Render-shaped service update (`bex services update`, REST `PATCH /v1/services/{id}`) saves the change and deploys nothing. The service reports `undeployedChanges: true` until the caller deploys, as Render's API documents and as bex already does for env vars (w8/186) and image/source changes. **Status:** blocked — needs your decision before work: the recorded runtime config holds only start command, health path, replicas, env and sources, so staging pre-deploy, shutdown delay, build commands or plan means extending the held-release record, diff and selection (w8/m119–m121) in lego/types and the operator — see the milestone's Triage section; approve that scoped design as a multi-task milestone, or sign off the w6/m51 tracked-rollout behavior as an ADR018 divergence

## Tasks (in order)

| id   | title                                                                          | est | depends_on                 |
| ---- | ------------------------------------------------------------------------------ | --- | -------------------------- |
| t001 | Stage release-relevant PATCH fields as pending on the REST/CLI path             | 60m | —                          |
| t002 | The next deploy (manual, hook, autodeploy commit) applies every staged field    | 30m | w8/m135/t001               |
| t003 | Deploy trigger detail: `envUpdated` true only when env actually changed         | 20m | w8/m135/t001               |
| t004 | Render parity: GraphQL/MCP/dashboard save modes, ADR004/ADR018 (w6/m51 note)    | 30m | w8/m135/t002, w8/m135/t003 |
| t005 | Simplify the changed code                                                      | 15m | w8/m135/t004               |
| t006 | Test coverage: each field stages, deploy applies, cancel/rollback neighbors     | 40m | w8/m135/t004               |
| t007 | Close out the milestone                                                        | 30m | w8/m135/t005, w8/m135/t006 |

## Definition of done

Live on production after the release pipeline deploys bex-api (and the dashboard if t004 changes it), on an owned Free `docker.io/ealen/echo-server:0.9.2` web service, deleted by ID with a 404 proof:

- Each of `bex services update <id> --health-check-path /hz2`, `--max-shutdown-delay 40`, `--pre-deploy-command "echo x"`, and a REST `PATCH` of `serviceDetails.envSpecificDetails.dockerCommand`: exit 0 / 200, **no new deploy** (`GET …/deploys` count unchanged after 20 s), `undeployedChanges: true`, and the running instance unchanged (same instance id).
- `bex deploys create <id> --wait` then applies all four. The new deploy's events show the pre-deploy marker, the new health path in the probe, and so on. `undeployedChanges` clears.
- The deploy's `trigger` is `api` with `envUpdated: false`.
- Controls: `--name`, `autoDeploy` and plan PATCHes behave as today. `PUT /env-vars` still saves only (w8/186).

## Source + Goal linkage

- **Source:** `/qa-find-bugs-cli` loop sweep 6e1b, 2026-10-23 UTC: bex v0.3.3, pin Render CLI v2.27.0 `a764810a7682`, `https://api.bex.co/v1/`, platform pin `fa8ac5bba24a`, `bex-canary` (`tea-daif693dqjvc73e7as3g`), human device login. Evidence below.
- **Goal linkage:** Render parity first ([ADR008](../../../docs/ADR008-vision.md#render-parity-first)). [Render API, Update service](https://api-docs.render.com/reference/update-service): "Update service allows you to update the configuration details of service, such as start commands and branches. Changes will **not** be deployed automatically. Instead you must call the deploy API to have changes pushed out to your service, irrespective of the `autoDeploy` option set on the service." w8/186 applied the identical sentence to env vars; this is the settings sibling.
- **Expected outcome:** scripts and CI that change several settings in a row (common with the CLI and Terraform-style tooling) get one deploy when they ask for it, not one unrequested production rollout per field with intermediate configurations.
- **Why now:** major. Unrequested production rollouts, each against a half-applied configuration. A pre-deploy (migration) command change runs the migration immediately. Render parity task included: REST/CLI behavior changes; GraphQL/MCP/dashboard save semantics must be decided consistently.

## Evidence (sweep 6e1b)

Owned fixture `srv-dbdh5dnsf6rc738q76r0` (`qa-20261023-6e1b-drain`, free web, echo-server, `PORT=80`). Deploy count read with `GET /v1/services/{id}/deploys?limit=50` before each REST `PATCH /v1/services/{id}` and 6 s after it:

```text
{"serviceDetails":{"healthCheckPath":"/hz2"}}                              → 200, deploys 4→5
{"serviceDetails":{"maxShutdownDelaySeconds":40}}                          → 200, deploys 5→6
{"serviceDetails":{"envSpecificDetails":{"dockerCommand":"node index.js"}}} → 200, deploys 6→7
{"serviceDetails":{"preDeployCommand":"echo pd2-6e1b"}}                    → 200, deploys 7→8
{"name":"qa-20261023-6e1b-drain2"}                                         → 200, deploys 8→8
{"serviceDetails":{"plan":"free"}}                                         → 200, deploys 8→8
{"autoDeploy":"no"}                                                        → 200, deploys 8→8
```

The pinned CLI shows the same: `bex services update srv-dbdh5dnsf6rc738q76r0 --pre-deploy-command "sh -c 'echo pd-6e1b-a \$((6*7))'" --confirm -o json` → exit 0, and within a second deploy `dep-dbdh6eg8dkas73a0v56g` (`trigger: service_updated`, `bexTrigger: config_change`) started, ran the pre-deploy (`pd-6e1b-a 42` in the logs) and was then superseded by the manual deploy issued a second later (`Superseded by dep-dbdh6evsf6rc738q76tg`). Its `deploy_started` event carried `"envUpdated": true` although no env var changed.

Control (sweep 3b7d, 2026-10-23): `bex services update <id> --image docker.io/ealen/echo-server:0.9.1` → no deploy, `undeployedChanges: true`, the next `deploys create` shipped it. That is Render's behavior, implemented by `SetSourceAndRegistryCredential` staging.

## Root cause

- `lego/backend/internal/apps/service.go:5082-5099` (`patchFetched`, the w6/m51 deploy-history gate): every release-relevant mutation (start/build command, root dir, Dockerfile path, pre-deploy command, health check path, plan, disks, shutdown delay) goes through `patchTracked(…, store.TriggerConfigChange, …)`, which rolls a release immediately. The comment's premise, "bex cannot apply a build- or release-relevant spec change without rolling a new release", is contradicted by the existing staging paths: Update Source (`SetSourceAndRegistryCredential`) and w8/186's `secrets.WithStagedWrite`.
- `lego/backend/internal/events/service.go:939`: `EnvUpdated: r.Trigger == store.TriggerConfigChange` marks every config roll as an env update.

## Fix (target)

- t001: on the Render-shaped REST PATCH (and so the pinned CLI), store release-relevant fields as pending intent, as `SetSourceAndRegistryCredential` and w8/186 do. `undeployedChanges` turns true, no `restartedAt` bump, no deploy row. Keep the audit/config-changed event (`RecordAppConfigChanged`).
- t002: every deploy path (manual, deploy hook, autodeploy commit, Blueprint sync) adopts all pending fields. Cancel/rollback keep w8/m119–m121 held-release semantics.
- t003: `envUpdated` true only for env/secret/env-group rollouts.
- t004: GraphQL/MCP/dashboard. The dashboard Settings page can keep an explicit "Save and deploy" choice, like Render's "Save, rebuild, and deploy" / "Save and deploy" / "Save only" (Render changelog 2024-09-11), but REST and MCP `update_service` must save only. Amend ADR004 and ADR018 (the w6/m51 note at ADR018:143) and record the plan field's behavior (see Unverified).

## Blast radius

`patchFetched`/`patchTracked` serve ~16 setters (`recordedPatch` callers in `internal/apps`). Shared with GraphQL, MCP and the dashboard, so t001 must gate by surface or save mode, not change every caller blindly. Every workspace, no allowlist. Suspended-service staging (ADR004:432) and protected environments (`requireUnprotected`) must keep working.

## Unverified

- Whether Render deploys on a plan/instance-type change through PATCH (Render docs describe a restart to resize; this sweep's plan PATCH was a no-op `free → free`).
- Render's exact field list behind "configuration details" (the docs give start commands and branches as examples).

## Dedupe

Searched `.pm` open and done for PATCH + deploy/rollout/automatic: w6/m51 (done; introduced the deploy rows deliberately, before the 2026-10-17 parity rule), w8/186 (done; env vars only), w4/m184 (applying notice copy), w4/369 (PATCH shape). No item covers settings PATCH auto-deploys.

## Triage 2026-10-23 (loopx w8): why this is parked, not worked

Reading the code before starting showed the milestone is several times its 225m estimate, and a partial version would be worse than none:

- **The staging machinery covers only some fields.** "Undeployed changes" is the saved `spec` compared with the serving release's recorded runtime config (`lego/operator/internal/controller/saved_configuration.go`, `release_config_selection.go` `runtimeConfigRecord.projectRuntimeApp`). That record captures only `StartCommand`, `Command`, `HealthCheckPath`, `Replicas`, `Env` and the sources. It does **not** capture `PreDeployCommand`, `MaxShutdownDelaySeconds`, build commands, Dockerfile path/context, root dir, plan or disks, so a PATCH of those cannot be left "saved but unapplied": the operator projects the Deployment straight from `spec`, so the pods would still roll (without the deploy row).
- **So t001 is not a backend-only switch.** It needs the release config record (`lego/types` + operator snapshot writer + selection + the diff + cancel/rollback/restart/held-release rules from w8/m119–m121) extended to every release-relevant field, then backend staging per setter. A half measure (stage only the two record-backed fields) would leave PATCH inconsistent: some fields save-only, the rest still rolling, which keeps the harm this milestone names (half-applied intermediate configurations).
- **What is unblocked once decided:** t003 (`envUpdated` only for env rollouts) is independent and small; t004's surface decisions follow t001.

**Needs your call:** approve the scoped design (extend the recorded runtime config to all release-relevant fields, then stage them on REST/MCP) as its own multi-task milestone, or keep the w6/m51 "every settings change is a tracked rollout" behavior and record it in ADR018 as a divergence with your sign-off (the only way it may stand under the parity rule).
