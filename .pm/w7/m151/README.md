# w7 · m151 — Coherent datastore placement after moves and deletion

**Worker:** worker7 **Goal:** ADR008 deterministic, machine-readable resource state; ADR018 Projects & environments (grouping). **Status:** todo

**Size:** 180m implementation; 295m including closing tasks (8 tasks).

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Pin datastore move and delete behavior | 40m | — |
| t002 | Clear incompatible environment state on project departure | 50m | w7/m151/t001 |
| t003 | Clear deleted grouping references with retry-safe handling | 50m | w7/m151/t001 |
| t004 | Reconcile existing drift and verify read surfaces | 40m | w7/m151/t002, w7/m151/t003 |
| t005 | Render parity | 30m | w7/m151/t004 |
| t006 | Simplify | 25m | w7/m151/t005 |
| t007 | Test coverage | 45m | w7/m151/t005 |
| t008 | Closeout | 15m | w7/m151/t006, w7/m151/t007 |

## Definition of done

- [ ] Reproductions distinguish deleted references from incompatible live-environment membership.
- [ ] The acceptance matrix covers both datastore kinds and preserves resource-owned allowlists.
- [ ] A datastore moved to another project has no membership or inherited rules from the former project’s environment.
- [ ] Authorized no-op reassignment does not churn state; refusal does not weaken protection.
- [ ] Environment deletion preserves valid parent-project membership.
- [ ] Project deletion leaves surviving datastores unassigned.
- [ ] Interrupted cleanup can be retried without clearing a concurrent valid reassignment.
- [ ] Existing provably stale references are repaired idempotently.
- [ ] All read surfaces agree on final membership and inherited rules.
- [ ] A dated live walkthrough records successful moves, deletion and recovery.

## Source + Goal linkage

- **Source:** Approved by `$pm all for w7 and $ship` after the additional w7 brainstorm (2026-09-17); source review at d7ea2ac52. These are source-backed findings, not newly reproduced production failures. Environment deletion in environments/service.go explicitly retains datastore labels; keyvalue.Service.SetProjectID and its Postgres counterpart change the project without clearing an incompatible environment. Views expose these labels directly.
- **Goal linkage:** ADR008 deterministic, machine-readable resource state; ADR018 Projects & environments (grouping).
- **Expected outcome:** Coherent datastore placement after moves and deletion. The acceptance matrix above defines the observable result.
- **Why now / deduplication:** Ordinary grouping operations expose raw datastore labels and can leave contradictory placement or obsolete inherited policy. w4/m32 repaired App departures and deletion-time IP clearing; w6/m20 added datastore membership but neither establishes this complete datastore lifecycle invariant.
- **Render parity:** Included, correcting lifecycle/admission behavior under the existing ADR018 grouping row (currently ✅), not adding a new capability.

## Dependencies and scope

Implementation may begin independently. Live dev-7 acceptance depends on `w7/m148/t007` (currently `.pm/w7/blocked/m148/t007.md`); do not recreate its former path. Coordinate shared grouping files between m151 and m152; serialize conflicting edits while preserving both sets of invariants. No new cross-milestone completion dependency is required.

No datastore deletion, ownership transfer, new grouping API, automatic foreign-resource adoption, or broad architecture refactor.
