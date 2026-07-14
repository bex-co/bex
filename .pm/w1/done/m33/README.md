# w1 · m33 — Pre-Deploy Command: gate rollout on a setup/migration step

**Worker:** worker1 **Goal:** let a deploy run a command (e.g. a DB migration) to completion before the new revision serves traffic, failing the deploy on a non-zero exit **Status:** done (2026-07-14)

## Tasks (in order)

| id   | title                                                                                              | est  | depends_on |
| ---- | ---------------------------------------------------------------------------------------------------- | ---- | ---------- |
| t001 | `App.Spec.PreDeployCommand` field (types + deepcopy + CRD yaml regen) — **DONE**                     | 45m  | —          |
| t002 | Operator: run `PreDeployCommand` as a Job/init step against the new revision's image before rolling the Deployment; block rollout on non-zero exit — **DONE** | 2h   | t001       |
| t003 | Surface pre-deploy step status/logs on the deploy record (distinct from a failed health check) — **DONE** | 1.5h | t002       |
| t004 | bex.yml/REST/GraphQL/MCP field wiring on App create/update — **DONE**                                 | 1h   | t001       |
| t005 | Dashboard: Build & Deploy settings field + deploy-detail view showing the pre-deploy step — **DONE**  | 1h   | t003, t004 |
| t006 | Render parity: verify field shape/semantics + deploy-record status consistent across REST/GraphQL/MCP/UI — **DONE** | 45m  | t005       |
| t007 | Simplify — **DONE**                                                                                  | 30m  | t006       |
| t008 | Test coverage — **DONE**                                                                             | 1.5h | t006       |
| t009 | Closeout — **DONE**                                                                                  | 15m  | t008       |

## Definition of done

An App with `preDeployCommand` set runs that command to completion against the new revision's image before it serves traffic; a non-zero exit fails the deploy and leaves the previous revision live and serving; the pre-deploy step's outcome (running/succeeded/failed + logs) is visible on the deploy record across REST/GraphQL/MCP/dashboard.

## Source + Goal linkage

- **Source:** `/pm-brainstorm more` 2026-07-13 — `docs/ADR006-bex-api.md:124` lists `preDeployCommand` as a bex.yml field currently "ignored — bex has no equivalent" (a blueprint-parsing non-goal note, not a product decision to skip the underlying feature). Render's Deploy section (Pre-Deploy Command) confirmed live via `.pm/w5/done/m13/README.md:7,48`. Checked against `w6/m21` (build/start command override, `DockerfilePath`/`StartCommand` only) — no overlap; that milestone's own DoD doesn't touch pre-deploy gating.
- **Goal linkage:** Render parity on a safety-critical deploy primitive — migration-before-rollout is the standard safe-deploy pattern, and bex currently has no way to gate a rollout on a setup step.
- **Expected outcome:** users can run schema migrations safely as part of a deploy, without a manual out-of-band step or risking traffic hitting containers before migrations complete.
- **Why now:** this is core deploy-flow mechanism work (CRD + operator reconciliation), consistent with where `w1` already owns build/deploy (`w1/m4` deployment flow, `w1/m5` build system). Render parity included — the new field and deploy-record status must be consistent across REST/GraphQL/MCP/UI.

## Completion notes (2026-07-14)

- **CRD (t001):** `App.spec.preDeployCommand` (string, `MaxLength=4096`) + a `status.preDeploy` sub-object (`PreDeployStatus{Job, Generation, StartedAt, FinishedAt, Status, Message}`, modeled on the `CronRun` precedent) with shared `PreDeploy{Running,Succeeded,Failed}` constants; `make manifests generate` regenerated the CRD YAML + deepcopy.
- **Operator (t002):** new `internal/predeploy` package (Job `predeploy-<name>-gen-<generation>` = the new image, `sh -c <command>`, app-pod env/secrets/pull-secrets, `BackoffLimit 0`, bounded deadline, TTL). `reconcileKubernetes` gains `reconcilePreDeploy`, an **async requeue gate** (consistent with the readiness gate) run before the Deployment `CreateOrUpdate`: it withholds the rollout while the step runs and `r.fail`s on a non-zero exit, leaving the previous Deployment untouched. Gated per generation (terminal-per-gen bookkeeping guards against re-running a passed/failed migration or re-creating a TTL-reaped Job) and skipped for suspended/hibernating/cron/static. envtest (`predeploy_test.go`) proves success gates the rollout and a failure keeps the previous revision live; an unset command is byte-identical.
- **Deploy record (t003):** `deploys.pre_deploy_status` column (migration `0022`) projected by the reconciler from `status.preDeploy` (generation-gated) — distinguishes a migration failure (`update_failed` + `preDeployStatus: failed`) from a health-check failure (`update_failed` + empty). Surfaced on `DeployView`/REST/GraphQL/MCP and (via a second column on the composed events feed) the dashboard Events tab. Step logs are a new **`predeploy` log type** — a live read of the Job pod (`core.PreDeployPods`, exclusive at the adapter, never the durable store).
- **API wiring (t004):** `preDeployCommand` threads through bex.yml, REST (top-level + `serviceDetails`, `PATCH` pointer), GraphQL (`createService`/`setPreDeployCommand`, `Service.preDeployCommand`), MCP (`create_web_service`/`set_pre_deploy_command`); `SetPreDeployCommand` verb mirrors `SetRootDir` and rejects cron/static. A new `pre_deploy_command_changed` event type covers the settings write.
- **Dashboard (t005):** Settings → Build & Deploy gains an inline-edit Pre-Deploy Command field (web/private/worker only); the Events tab shows a per-deploy pre-deploy line, red on failure. Hand-mirrored `definitions.ts` (no live bex-api for codegen), per w5/m13's technique.
- **Parity (t006):** `preDeployCommand` moved from ADR006's "ignored" list to honored; ADR018 gains a Pre-Deploy Command row (✅×4); ADR004 documents the gate mechanism; ADR010 documents the `predeploy` log type. Field name (`preDeployCommand`) and deploy-record status (`preDeployStatus`) verified byte-consistent across all four surfaces.
- **Simplify (t007):** 4-agent review → applied 3 fixes: gated the newest-wins `CancelSuperseded` List off the running-happy-path requeue; consolidated the three pre-deploy-failure paths into one `failPreDeploy` helper; extracted a shared `LogQuery.filterAndCap`. Reuse/altitude confirmed clean (reconcilePreDeploy reuses all six app-container helpers; the log-type early branch and the dual projection paths are appropriately deep).
- **Tests (t008):** operator `predeploy` unit test + envtest; backend reconciler-projection, apps-wiring, logs-type, deploys-surface, and events-view tests; dashboard hook + Build & Deploy field tests. All three suites green; backend `make lint-backend` (the CI-gated module) clean; codegen stable.
