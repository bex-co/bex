# w8 · m39 — Complete Blueprint execution fencing

**Worker:** worker8 **Goal:** Interrupted Blueprint execution stops managing resources, including after recovery or disconnect. **Status:** todo

**Estimate:** 3h20m implementation; 5h20m including standing closing tasks. **Priority:** 1 in the approved 2026-09-09 proposal.

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Define execution expiry and resource mutation boundaries | 40m | — |
| t002 | Enforce execution authority across apply mutations | 60m | w8/m39/t001 |
| t003 | Coordinate recovery and disconnect with expired execution | 60m | w8/m39/t002 |
| t004 | Preserve honest interrupted-run outcomes across surfaces | 40m | w8/m39/t003 |
| t005 | Render parity | 30m | w8/m39/t004 |
| t006 | Simplify | 30m | w8/m39/t005 |
| t007 | Test coverage | 45m | w8/m39/t005, w8/m39/t006 |
| t008 | Closeout | 15m | w8/m39/t007 |

## Definition of done

- A paused worker resumed after authority retirement cannot start further mutations or overwrite successor-owned state; cover in-flight requests as well as later steps.
- Real-Postgres and resource-client fault scenarios exercise recovery, successor admission, disconnect, deferred patches, and ownership stamping.
- History and resource state agree; partial application remains an explicit failure/retry outcome.
- Complete the outstanding m37 dev-8 two-client walkthrough: concurrent sync busy behavior, disconnect/absence, interrupted-run history, actual resource state, and fixture cleanup. Do not close on an environment blocker.

## Source + Goal linkage

- **Source:** User-approved /pm-brainstorm for w8 proposal 1, materialized 2026-09-09. m37 fences lifecycle records but deployParsedStack has no execution-authority checks between resource writes; the ownership stamp checks generation only once. This corrective milestone completes unmet m37 acceptance, including its outstanding dev-8 two-client walkthrough. Source review is not a live reproduction.
- **Goal linkage:** [ADR008](../../../docs/ADR008-vision.md) deterministic deployment and agent-readable state; [.pm/GOAL.md](../../GOAL.md) goal 3 (Git push to deploy), with tenant resource integrity under goal 5.
- **Expected outcome:** Interrupted Blueprint execution stops managing resources, including after recovery or disconnect.
- **Why now:** Complete this before m38 expands automatic intake. A terminal history row must mean the retired worker cannot continue managing resources.
- **Render parity:** included; tenant-facing Blueprint behavior changes across REST/GraphQL/MCP/UI. Update the relevant clauses of [ADR018’s Blueprint row](../../../docs/ADR018-render-parity.md#deployment-sources--iac), keeping its independent unsupported-subset/evidence gaps partial. Ownership enforcement remains a documented bex divergence; a new reviewed-source API precondition is a bex extension unless equivalent Render behavior is verified.
- **Anti-goals:** honors [DO_NOT_DO.md](../../DO_NOT_DO.md); no excluded capability is reopened.

## Dependencies and execution

t001 is immediately actionable; it builds on the existing m37 implementation, without treating its archived status as verification of the missing guarantees. Resolve archived prerequisites under done/ without recreating old paths. Use [dev-8](../README.md) for live checks.

m38 must wait for this closeout and m40 closeout before automatic intake expands. Preserve m37 historical evidence; this milestone explicitly owns its outstanding execution guarantee and walkthrough.

## Scope

No generic workflow engine, automatic partial-apply replay, new deployment products, or resource deletion. Preserve the operator → types ← backend boundary.
