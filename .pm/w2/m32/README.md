# w2 · m32 — DeployStatus/DeployTrigger enum completeness + updatedAt

**Worker:** worker2 **Goal:** bex's `store.Deploy` status vocabulary (`update_in_progress`/`live`/`update_failed`/`canceled`) is 4 of Render's real 11-value `DeployStatus` enum; the reconciler cannot today distinguish "the build Job is still building" from "the rollout is still in progress" — both collapse to `update_in_progress`. `w1/m5` (in-cluster BuildKit builds) has since shipped, so `build_in_progress`/`build_failed` are now genuinely trackable — retrofit them. Also add the `updated_at` field Render's `Deploy` object has and bex's doesn't, and fix `TriggerCreate="create"`, the one of bex's three trigger values with no Render equivalent. **Status:** todo

## Tasks (in order)

| id   | title                                                                                                          | est | depends_on |
| ---- | ------------------------------------------------------------------------------------------------------------------ | --- | ---------- |
| t001 | Add `build_in_progress`/`build_failed` to `store`'s status vocabulary; wire the reconciler to distinguish build-phase from rollout-phase | 50m | —          |
| t002 | Add `updated_at` column + `Deploy.UpdatedAt`; write it on every status transition                                   | 30m | t001       |
| t003 | Rename/relabel `TriggerCreate="create"` — pick a Render-shaped value or explicitly document it as a bex-only extra  | 15m | —          |
| t004 | REST/GraphQL/MCP: surface the two new statuses + `updatedAt`                                                        | 20m | t001, t002 |
| t005 | Render parity — cross-surface consistency; refresh `docs/ADR018-render-parity.md`'s Deploys rows                    | 20m | t004       |
| t006 | Simplify — `/simplify` over the code this milestone changed                                                         | 15m | t005       |
| t007 | Test coverage — build-vs-rollout status transitions, updatedAt writes, trigger-value correctness                    | 30m | t005       |
| t008 | Closeout — DoD met → move milestone to `done/`                                                                      | 10m | t007       |

## Definition of done

A build failure (BuildKit Job fails) is distinguishable from a rollout failure (the built image fails health checks) in deploy history — today both read `update_failed` with no way to tell which stage broke. `Deploy.UpdatedAt` is present and accurate on all three surfaces. `docs/ADR018-render-parity.md`'s Deploys rows state the remaining deliberate gap (`created`/`deactivated`/`queued`/`pre_deploy_*` — no backing bex feature: bex has no pre-deploy-command feature, no deactivation concept, and `created`/`queued` don't correspond to any distinct bex-observable state) instead of implying full status-enum parity.

## Source + Goal linkage

- **Source:** `/pm-brainstorm` 2026-07-14, same audit as `w2/m30`/`w2/m31` — ground truth `render-oss/render-mcp-server`'s `types_gen.go` `DeployStatus`/`DeployTrigger` enums (11 and 10 values respectively). bex's own existing code comment (`lego/backend/internal/store/store.go:78-96`) already anticipated this: "build_in_progress/build_failed are reserved for w1/m5 (build-from-git)" — `w1/m5` shipped 2026-07-09; this milestone is the retrofit that comment called for.
- **Goal linkage:** pillar 1 (Render parity — a build failure and a rollout failure are different operator actions to take, and Render's dashboard/API already distinguishes them; bex currently can't) + pillar 3 (an agent polling `get_deploy` needs to know whether to look at build logs or runtime logs).
- **Expected outcome:** deploy history distinguishes build failures from rollout failures; the parity ledger's Deploys section states its real, narrower-than-Render status coverage explicitly, with each remaining gap tied to a real missing bex feature (not an oversight).
- **Why now:** the reconciler is the riskiest part of this whole audit's proposed work (touches the status-writing path directly, shared with every other deploy) — sequenced last, after `m30`/`m31` prove out the lower-risk parts of the same package.
- **Render parity closing task: included** — REST, GraphQL, and MCP all change. Dashboard UI (Events tab status badges) already renders whatever status string it's given via `statusVariant`/`statusKey` in `services.$serviceId.events.tsx`; extending those switch statements for the two new values is small enough to fold into `t004`, not a separate dashboard milestone.
