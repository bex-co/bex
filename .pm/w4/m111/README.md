# w4 · m111 — Environment-group creation offers scope-incompatible links and cannot create in-env groups

**Worker:** worker4 **Goal:** creating an environment group with service links stops failing on scope: the dialog only offers link candidates compatible with the group's scope, and both create paths can mint an in-environment group in one step. **Status:** todo

## Tasks (in order)

| id   | title                                                                                                  | est   | depends_on |
| ---- | ------------------------------------------------------------------------------------------------------ | ----- | ---------- |
| t001 | Scope picker + scope-filtered link candidates in the create dialog                                     | 1h30m | —          |
| t002 | Service-page create path threads the service's environment through creation                            | 45m   | t001       |
| t003 | Render parity                                                                                          | 30m   | t002       |
| t004 | Simplify                                                                                               | 15m   | t003       |
| t005 | Test coverage                                                                                          | 45m   | t003       |
| t006 | Closeout                                                                                               | 10m   | t005       |

## Definition of done

Each bullet is a click the next person can repeat on production and watch succeed.

- **Workspace create filters candidates.** On `/env-groups`, **New Environment Group** with scope left at Workspace lists only workspace-scoped (environment-less) services as linkable; services living in an environment are absent or disabled with a reason naming their environment. Checking every offered candidate never produces `ENV_GROUP_SERVICE_ENVIRONMENT_MISMATCH`.
- **In-env create works in one step.** Picking an environment in the dialog lists only that environment's services, and creating with links checked mints the group already in that environment — no create-unlinked-then-Move dance.
- **Service-page create succeeds.** From an in-environment service's Environment tab, **Create group** keeps the service pre-checked and the create succeeds, landing the group in the service's environment. From a workspace-scoped service, the same flow keeps today's working behavior.
- **Backend enforcement unchanged.** `ENV_GROUP_SERVICE_ENVIRONMENT_MISMATCH` still rejects genuinely mismatched writes on REST/GraphQL/MCP; this milestone only stops the dashboard from constructing them.

## Source + Goal linkage

- **Source:** live `/qa-find-bugs` pass on `https://dashboard.bex.co`, 2026-09-17 (w4-targeted run, `muse.env` credentials). Repro: service `qa-20260917-p3-web` (`srv-dalpb5l2r12c73cngju0`, in `qa-env`) + group `qa-20260917-p3-evg` — both create paths failed with "linked services must have the same Environment scope as the environment group" (evidence screenshot `.playwright-mcp/qa-p3-evg-create-scope-fail.png`, gitignored, local to that session). Same run verified clean and NOT filed: notification per-service override add/save/exact-empty-list/remove round-trips, webhook `deploy_ended` delivery end to end (payload + 404 detail + scheduled retry), deploy-hook trigger/label/regenerate rotation, cron create/schedule-preview/run-detail/trigger/cancel-race, suspend/resume.
- **Goal linkage:** Render parity and product truthfulness ([docs/ADR018-render-parity.md](../../../docs/ADR018-render-parity.md), [docs/ADR006-bex-api.md](../../../docs/ADR006-bex-api.md)). Every service created inside a project environment — the default journey — hits a dead-end dialog the moment it tries the obvious next step (share config via a group), and the service-page path pre-checks the failing option so the user cannot succeed by any click sequence.
- **Expected outcome:** in-environment services can adopt shared config in one dialog instead of a three-step workaround (create unlinked → Move → link); the create dialog never presents a link choice the backend will reject.
- **Why now:** the backend already supports everything the fix needs (`createEnvGroup(environmentId, serviceIds)` on GraphQL/REST/MCP, validated by `prepareCreateServices`), and the detail page already implements the correct filter (`linked-services-card.tsx:45-54`) — the fix is dashboard wiring against settled contracts, not new architecture. Render parity is included because the fix touches the shared create surface (dashboard + GraphQL variables).
