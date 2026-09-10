# w8 · m40 — Make Blueprint ownership reliable during apply

**Worker:** worker8 **Goal:** Concurrent and partially failed Blueprint syncs preserve one authorized resource owner. **Status:** todo (t001–t006 done; t007 live walkthrough outstanding)

**Estimate:** 3h20m implementation; 5h20m including standing closing tasks. **Priority:** 2 in the approved 2026-09-09 proposal.

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Establish conditional resource claims — **DONE** | 50m | w8/m39/t008 |
| t002 | Couple resource writes to the authorized claim — **DONE** | 60m | w8/m40/t001 |
| t003 | Preserve ownership through failure and disconnect — **DONE** | 50m | w8/m40/t002 |
| t004 | Propagate claim failures and managed-resource state — **DONE** | 40m | w8/m40/t003 |
| t005 | Render parity — **DONE** | 30m | w8/m40/t004 |
| t006 | Simplify — **DONE** | 30m | w8/m40/t005 |
| t007 | Test coverage | 45m | w8/m40/t005, w8/m40/t006 |
| t008 | Closeout | 15m | w8/m40/t007 |

## Definition of done

- Two Blueprints racing for one App, Postgres, or Key Value yield one authorized owner; the loser makes no configuration mutation.
- Partial apply, failed ownership persistence, explicit takeover, retry, and disconnect preserve truthful ownership and terminal results.
- Real-Postgres multi-connection races and resource-write fault injection cover creation, adoption, and takeover. A dev-8 two-Blueprint walkthrough confirms state and cleanup.
- Tenant, protected-environment, and billing boundaries remain enforced. Disconnect preserves deployed resources.

## Source + Goal linkage

- **Source:** User-approved /pm-brainstorm for w8 proposal 2, materialized 2026-09-09. m23 checks ownership before apply and stamps labels best-effort only after the entire stack succeeds. Partial apply can leave changed resources unclaimed; two different Blueprints can pass the check concurrently. m37 coordinates one Blueprint, not competing resource owners. This is corrective coverage for the shipped ownership guarantee.
- **Goal linkage:** [ADR008](../../../docs/ADR008-vision.md) deterministic deployment and agent-readable state; [.pm/GOAL.md](../../GOAL.md) goal 3 (Git push to deploy), with tenant resource integrity under goal 5.
- **Expected outcome:** Concurrent and partially failed Blueprint syncs preserve one authorized resource owner.
- **Why now:** Complete after m39 and before m38 increases automatic intake; stronger same-Blueprint execution authority is the prerequisite for resource claims.
- **Render parity:** included; tenant-facing Blueprint behavior changes across REST/GraphQL/MCP/UI. Update the relevant clauses of [ADR018’s Blueprint row](../../../docs/ADR018-render-parity.md#deployment-sources--iac), keeping its independent unsupported-subset/evidence gaps partial. Ownership enforcement remains a documented bex divergence; a new reviewed-source API precondition is a bex extension unless equivalent Render behavior is verified.
- **Anti-goals:** honors [DO_NOT_DO.md](../../DO_NOT_DO.md); no excluded capability is reopened.

## Dependencies and execution

t001 depends on w8/m39/t008. Resolve archived prerequisites under done/ without recreating old paths. Use [dev-8](../README.md) for live checks.

## Scope

No automatic takeover, sync-delete, new resource families, or ownership redesign for ordinary non-Blueprint operations beyond the enforcement needed for this guarantee.


## Closeout evidence

See [walkthrough-evidence.txt](walkthrough-evidence.txt).
