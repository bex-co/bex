# w5 · m107 — Preserve saved settings during rollback

**Worker:** worker5 **Goal:** Run the selected historical configuration during rollback while retaining the saved configuration for the next standard deploy. **Status:** done (2026-10-01)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Research Render rollback configuration and reconcile m152 — **DONE** | 30m | — |
| t002 | Apply historical configuration only to the rollback release — **DONE** | 45m | t001 |
| t003 | Keep saved settings authoritative for the next standard deploy — **DONE** | 45m | t002 |
| t004 | Verify rollback A and next-deploy B on isolated dev-5 — **DONE** | 45m | t003 |
| t005 | Render parity and surface disposition — **DONE** | 25m | t004 |
| t006 | Simplify — **DONE** | 20m | t005 |
| t007 | Test coverage — **DONE** | 30m | t005, t006 |
| t008 | Closeout — **DONE** | 10m | t007 |

## Definition of done

- A disposable dev-5 service demonstrates deploy A → save/deploy B → rollback A: the ready runtime uses A while saved Settings, environment and secret-file reads retain B. The next standard deploy runs B.
- Release-specific configuration remains correct through reconciliation, pod replacement, explicit service Restart and operator restart, with no transient mutation of shared environment groups or saved configuration.
- The researched target/current configuration matrix is applied to supported fields, with unsupported or unverified behavior explicitly dispositioned. Existing release snapshots and cancel-deploy protections remain covered; current plan/disks/domains retain the researched current-setting semantics.
- REST, GraphQL, MCP and dashboard agree on saved values and rollback outcomes; preserve the documented dashboard-versus-API auto-deploy distinction. Target records missing or evicted have an explicit compatible disposition. Same-image/config-only targets, already-running-target guards and saved-versus-running indicators remain truthful.
- Relevant backend/operator/type tests and lint pass, dashboard checks pass if affected, and local runtime evidence plus fixture cleanup are recorded before closeout.

## Source + Goal linkage

- **Source:** User-approved follow-up to the w5 brainstorm and Render parity review, 2026-10-01; reproduced saved-state drift in [w5/m105](../../blocked/m105/README.md). Corrects the saved-state interpretation in [w1/m152](../../../w1/done/m152/README.md) while retaining its release snapshots and cancellation protections.
- **Goal linkage:** ADR008 dependable hosting and ADR004/ADR018 Render-compatible deploy lifecycle.
- **Expected outcome:** After deploying A, saving/deploying B and rolling back to A, the service runs A while Settings still contains B; the next standard deploy runs B.
- **Why now:** The m105 local replay observed both the start command and API-owned environment reverting in saved state, and the next normal deploy kept the old command. Research and correct this narrow semantic gap before extending rollback behavior.
- **Render parity included:** This changes user-visible deploy or cron behavior. Check REST, GraphQL, MCP and dashboard semantics against the t001 research, distinguishing direct Render requirements from bex correctness and undocumented details.

## Execution notes

Start with t001 before implementation. Work only in isolated dev-5 for live checks. Queue order is m107 → m108 → m109; m108 and m109 share cron code, so land m108 before changing it for m109. Research and all implementation tasks remain todo at filing.

## Research — 2026-10-01

[Render rollback documentation](https://render.com/docs/rollbacks) preserves saved settings and applies them on the next standard deploy. Its selection matrix is:

| Configuration | Runtime selection |
| --- | --- |
| Start/Docker command, health path, instance count, environment, built artifact | Target deploy |
| Compute plan, disks, domains, static routes/headers, platform settings | Current |
| Registry image | Target tag/digest, fetched again; mutable tags may resolve differently |
| Environment groups | Target associations; shared values unchanged; deleted groups omitted |

Dashboard rollback disables auto-deploy; API rollback does not. Artifact availability bounds target eligibility. The docs do not specify internal configuration-record retention, missing-record fallback, secret-file handling, autoscaling interaction or pre-deploy commands. Secret-file saved-state preservation here is a bex correctness requirement. No authenticated Render experiment was performed.

[Render restart documentation](https://render.com/docs/deploys#restarting-a-service) retains the running commit and configuration, including environment values. Therefore rollback A → Restart must keep A, while saved B remains available for a standard deploy.

### Code disposition before implementation

- `restoreTargetConfig` writes saved environment and `Rollback` overwrites saved start/image settings. This supersedes the saved-state interpretation in w1/m152 t009; its immutable snapshots and cancel restoration remain useful.
- `ReleaseRecordSpec` currently records only start command; the pod template also retains command/probes/env/source references. Instance count is absent, so legacy records cannot supply it honestly. Extend future records and state legacy limitations.
- A whole-template rollback would restore old plan/disk/platform fields incorrectly. Select only historical runtime fields and retain current operational settings.
- Bind the historical image/configuration selection to the newly opened release, without overwriting saved settings. Copy selected snapshot inputs into that release so retention cannot delete live dependencies. Normal deployment expires the selection; Restart explicitly inherits the running release.
- `Restart` currently follows normal triggering and can rebuild saved B; `RollbackActionable` compares image/static commit only; `UndeployedChanges` only describes cancellation. All three require a deliberate update and regression coverage.
- Existing missing-record image-only fallback is bex drift, not verified Render parity; preserve saved state and document its limit. Never invent historical values from current settings or silently ignore a missing selected snapshot after dispatch.

## Implementation and disposition

Rollback now opens a generation-bound release selection without mutating saved image, command, environment or files. The operator copies selected snapshot inputs into the new release, projects historical runtime fields, and retains current plan/disks/platform settings. New release records retain the supported historical fields; legacy pod templates provide command/health/env but cannot invent a historical replica count. Missing records retain the existing image-only Bex fallback, explicitly documented as drift; missing selected snapshots fail safely.

Restart selects the live release configuration (including its group snapshots); a normal deployment clears selection and uses saved settings. Rollback reuses target group associations with current surviving group values. Same-image recovery and already-live refusal account for the selected generation. Explicit scaling still works even if the requested count already equals the saved count. Static republish retains its current-build-settings limitation and honors the dashboard/API auto-deploy distinction.

REST, GraphQL and MCP share dispatch semantics. The dashboard explains saved-versus-live divergence without describing all divergence as cancellation. CLI behavior follows the existing REST path; no CLI implementation changed. See [local evidence](evidence/README.md) and [sanitized observations](evidence/runtime.json). This does not close the independent m105 production incident. Broader restart exactness after canceled plan/disk changes remains unverified; the supported selection fields and group snapshots are covered here.

## Validation and closeout

- Final operator `make test` (codegen + envtest), types `go test ./...`, backend `go test ./...` and four-module `make lint` (including deadcode) passed.
- Dashboard `yarn test`: 456 files, 3785 tests passed. Dashboard `yarn lint` (typecheck/ESLint/knip) passed.
- The new saved-state regression fails against the pre-change backend: rollback changes saved image to `web:v1`; it passes with this change. Coverage includes same-image rollback, Restart then normal deploy, missing/corrupt records, deleted groups, selected-snapshot loss, scale, cancellation, and per-surface auto-deploy policy.
- Three simplify reviews found and removed duplicate same-pass snapshot reads and a redundant record read; regression verifies later reconciles still detect a deleted snapshot. No unnecessary helper merger was made where missing-source semantics differ.
- Local live checks and cleanup passed as recorded in `evidence/`. Tests using optional local Postgres/OpenFGA integrations may skip those integrations; the real API/store/Kubernetes fixture supplies the recorded lifecycle evidence.
- Known limits: no authenticated Render experiment, production reproduction or shared-manager restart; legacy records lack replica history; evicted records use explicit image-only fallback; static republish uses current build settings. No broader historical autoscaling/pre-deploy or canceled-plan/disk Restart guarantee is claimed.
