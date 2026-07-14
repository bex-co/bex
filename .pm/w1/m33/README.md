# w1 · m33 — Pre-Deploy Command: gate rollout on a setup/migration step

**Worker:** worker1 **Goal:** let a deploy run a command (e.g. a DB migration) to completion before the new revision serves traffic, failing the deploy on a non-zero exit **Status:** todo

## Tasks (in order)

| id   | title                                                                                              | est  | depends_on |
| ---- | ---------------------------------------------------------------------------------------------------- | ---- | ---------- |
| t001 | `App.Spec.PreDeployCommand` field (types + deepcopy + CRD yaml regen)                                | 45m  | —          |
| t002 | Operator: run `PreDeployCommand` as a Job/init step against the new revision's image before rolling the Deployment; block rollout on non-zero exit | 2h   | t001       |
| t003 | Surface pre-deploy step status/logs on the deploy record (distinct from a failed health check)       | 1.5h | t002       |
| t004 | bex.yml/REST/GraphQL/MCP field wiring on App create/update                                            | 1h   | t001       |
| t005 | Dashboard: Build & Deploy settings field + deploy-detail view showing the pre-deploy step             | 1h   | t003, t004 |
| t006 | Render parity: verify field shape/semantics + deploy-record status consistent across REST/GraphQL/MCP/UI | 45m  | t005       |
| t007 | Simplify                                                                                              | 30m  | t006       |
| t008 | Test coverage                                                                                         | 1.5h | t006       |
| t009 | Closeout                                                                                              | 15m  | t008       |

## Definition of done

An App with `preDeployCommand` set runs that command to completion against the new revision's image before it serves traffic; a non-zero exit fails the deploy and leaves the previous revision live and serving; the pre-deploy step's outcome (running/succeeded/failed + logs) is visible on the deploy record across REST/GraphQL/MCP/dashboard.

## Source + Goal linkage

- **Source:** `/pm-brainstorm more` 2026-07-13 — `docs/ADR006-bex-api.md:124` lists `preDeployCommand` as a bex.yml field currently "ignored — bex has no equivalent" (a blueprint-parsing non-goal note, not a product decision to skip the underlying feature). Render's Deploy section (Pre-Deploy Command) confirmed live via `.pm/w5/done/m13/README.md:7,48`. Checked against `w6/m21` (build/start command override, `DockerfilePath`/`StartCommand` only) — no overlap; that milestone's own DoD doesn't touch pre-deploy gating.
- **Goal linkage:** Render parity on a safety-critical deploy primitive — migration-before-rollout is the standard safe-deploy pattern, and bex currently has no way to gate a rollout on a setup step.
- **Expected outcome:** users can run schema migrations safely as part of a deploy, without a manual out-of-band step or risking traffic hitting containers before migrations complete.
- **Why now:** this is core deploy-flow mechanism work (CRD + operator reconciliation), consistent with where `w1` already owns build/deploy (`w1/m4` deployment flow, `w1/m5` build system). Render parity included — the new field and deploy-record status must be consistent across REST/GraphQL/MCP/UI.
