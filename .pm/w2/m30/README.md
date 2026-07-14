# w2 · m30 — Manual Deploy body: commitId, clearCache, deployMode, restart consolidation

**Worker:** worker2 **Goal:** `POST /v1/services/{id}/deploys` currently reads no request body at all — it ignores everything a caller sends. Bring it in line with Render's real `CreateDeploy` shape (`commitId`, `clearCache`, `deployMode`) where bex can honor it honestly, and stop `apps.Service.Restart` from being a second, untracked rollout mechanism that silently duplicates what `deploys.Trigger` already does. **Status:** todo

## Tasks (in order)

| id   | title                                                                                                                     | est | depends_on |
| ---- | -------------------------------------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Confirm whether BuildKit's rootless build Job persists a cache across builds at all, or every build is already cache-free | 20m | —          |
| t002 | `deploys.Service.Trigger` accepts `commitId` — checkout that ref instead of branch HEAD; reject for cron jobs             | 45m | —          |
| t003 | Design + implement `deployMode` — honest `deploy_only` semantics (see Definition of done)                                 | 45m | t002       |
| t004 | Consolidate `apps.Service.Restart`/GraphQL `restartServer` into `deploys.Trigger(deployMode=deploy_only)`                 | 45m | t003       |
| t005 | REST decodes the body (`commitId`/`clearCache`/`deployMode`); GraphQL `triggerDeploy` gains the matching optional args    | 40m | t002, t003 |
| t006 | Dashboard: `manual-deploy-button.tsx`/`use-trigger-deploy.ts` wire the real params; Restart calls the same mutation       | 40m | t004, t005 |
| t007 | Render parity — cross-surface consistency; refresh `docs/ADR018-render-parity.md`'s Deploys rows                          | 20m | t006       |
| t008 | Simplify — `/simplify` over the code this milestone changed                                                              | 20m | t007       |
| t009 | Test coverage — `commitId` checkout override, `deployMode` reject/accept paths, restart-opens-deploy-row                  | 35m | t007       |
| t010 | Closeout — DoD met → move milestone to `done/`                                                                            | 10m | t009       |

## Definition of done

A repo-backed service's Manual Deploy can target a specific commit (`commitId`) instead of always deploying branch HEAD. `clearCache` is honest — either it actually busts a real BuildKit cache (if t001 finds one exists) or the REST/GraphQL docs say plainly that bex has no build cache to clear, never a silently-ignored field. `deployMode: deploy_only` does NOT fake a no-rebuild redeploy for a repo-backed service (bex has no cached build artifact distinct from "rebuild from source" — confirmed: any `spec` generation bump unconditionally rebuilds, `lego/operator/internal/controller/app_controller.go`) — it returns a clear, typed error for a repo-backed service, and behaves identically to `build_and_deploy` for an image-backed one (nothing to build either way). Clicking "Restart service" anywhere in the dashboard opens a deploy-history row, same as every other rollout — `apps.Restart`'s direct `RestartedAt` bump is no longer a parallel, untracked path a user can reach from the UI.

## Source + Goal linkage

- **Source:** `/pm-brainstorm` 2026-07-14, prompted by a live audit of bex's deploy REST/GraphQL/MCP against Render's real API (ground truth: `render-oss/render-mcp-server`'s generated Go client, `pkg/client/types_gen.go`/`client_gen.go` — `CreateDeployJSONBody`'s `commitId`/`clearCache`/`deployMode` fields, not docs prose). The audit also surfaced a bex-native correctness gap independent of Render parity: `apps.Restart` and `deploys.Trigger` both just bump `Spec.RestartedAt` under the hood, but only `Trigger` opens a deploy-history row — today, clicking Restart is invisible in the Events tab.
- **Goal linkage:** pillar 1 (Render deploy-surface parity — `docs/ADR018-render-parity.md`'s Deploys rows currently claim ✅ REST/GraphQL/MCP, which this audit found overstated) + pillar 3 (agent-triggered deploys — `commitId` and honest `deployMode` are the exact primitives a deploy-from-chat agent needs).
- **Expected outcome:** `docs/ADR018-render-parity.md`'s "Trigger a deploy" row reflects the real, narrower field set bex supports (and says so explicitly, not silently); the Manual Deploy dropdown shipped this session (`dashboard/src/features/services/components/manual-deploy-button.tsx`) calls the correct verb for "Restart service" instead of the separate, untracked lifecycle mutation.
- **Why now:** directly fixes a button shipped this session with the wrong semantics (Restart wired to `apps.Restart` instead of `deploys.Trigger`), plus a real observability gap (untracked restarts) that predates this audit and only gets more expensive to unwind the longer both paths coexist.
- **Render parity closing task: included** — REST, GraphQL, and dashboard UI surfaces all change. MCP is unaffected (Render's own official MCP server ships no deploy-trigger tool at all — `list_deploys`/`get_deploy` only, confirmed against `render-oss/render-mcp-server`'s `pkg/deploy/tools.go` — so bex's MCP surface for this verb stays as-is, a documented non-gap).
