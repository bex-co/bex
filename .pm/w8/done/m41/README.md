# w8 · m41 — Bind manual Blueprint sync to its reviewed source

**Worker:** worker8 **Goal:** Manual sync cannot silently apply a Git source different from the one the user or agent reviewed. **Status:** done

**Estimate:** 2h50m implementation; 4h50m including standing closing tasks. **Priority:** 3 in the approved 2026-09-09 proposal.

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Define the reviewed-source precondition — **DONE**| 40m | w8/m39/t008 |
| t002 | Enforce reviewed source across API adapters — **DONE**| 55m | w8/m41/t001 |
| t003 | Carry review through dashboard confirmation retries — **DONE**| 45m | w8/m41/t002 |
| t004 | Offer actionable refresh and renewed review — **DONE**| 30m | w8/m41/t003 |
| t005 | Render parity — **DONE**| 30m | w8/m41/t004 |
| t006 | Simplify — **DONE**| 30m | w8/m41/t005 |
| t007 | Test coverage — **DONE**| 45m | w8/m41/t005, w8/m41/t006 |
| t008 | Closeout — **DONE**| 15m | w8/m41/t007 |

## Definition of done

- A branch move between preview and confirmation never applies unreviewed bytes; verify through REST, GraphQL, MCP, and the dashboard flow.
- Source-setting changes and protected-confirmation retries preserve the approval boundary.
- History identifies the actual consumed revision; existing callers without a precondition retain documented semantics.
- A dev-8 review/branch-movement walkthrough verifies refresh, renewed approval, resource state, and cleanup.

## Source + Goal linkage

- **Source:** User-approved /pm-brainstorm for w8 proposal 3, materialized 2026-09-09. m21 provides a pre-sync preview and m36 binds each Git read to its reported commit, but dashboard confirmation sends only Blueprint ID and confirmation phrase; runSync resolves the branch again. The gap is between review and apply, not within one Git read.
- **Goal linkage:** [ADR008](../../../docs/ADR008-vision.md) deterministic deployment and agent-readable state; [.pm/GOAL.md](../../GOAL.md) goal 3 (Git push to deploy), with tenant resource integrity under goal 5.
- **Expected outcome:** Manual sync cannot silently apply a Git source different from the one the user or agent reviewed.
- **Why now:** The review UI already implies approval of a specific source. Build on m39 execution authority; schedule after m40 for shared-file collision avoidance, without a hard dependency on m38.
- **Render parity:** included; tenant-facing Blueprint behavior changes across REST/GraphQL/MCP/UI. Update the relevant clauses of [ADR018’s Blueprint row](../../../docs/ADR018-render-parity.md#deployment-sources--iac), keeping its independent unsupported-subset/evidence gaps partial. Ownership enforcement remains a documented bex divergence; a new reviewed-source API precondition is a bex extension unless equivalent Render behavior is verified.
- **Anti-goals:** honors [DO_NOT_DO.md](../../DO_NOT_DO.md); no excluded capability is reopened.

## Dependencies and execution

t001 depends on w8/m39/t008. Resolve archived prerequisites under done/ without recreating old paths. Use [dev-8](../README.md) for live checks.

## Scope

No full live-state plan locking, PR preview environments, additional Git providers, bespoke CLI, or automatic-sync source policy change; m38 owns automatic intake.
