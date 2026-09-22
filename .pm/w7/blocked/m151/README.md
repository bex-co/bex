# w7 · m151 — Coherent datastore placement after moves and deletion

**Worker:** worker7 **Goal:** ADR008 deterministic, machine-readable resource state; ADR018 Projects & environments (grouping). **Status:** blocked (t001–t003, t005–t007 done; t004 live walkthrough gated; t008 pending)

**Size:** 180m implementation; 295m including closing tasks (8 tasks).

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Pin datastore move and delete behavior — **DONE** | 40m | — |
| t002 | Clear incompatible environment state on project departure — **DONE** | 50m | w7/m151/t001 |
| t003 | Clear deleted grouping references with retry-safe handling — **DONE** | 50m | w7/m151/t001 |
| t004 | Reconcile existing drift and verify read surfaces | 40m | w7/m151/t002, w7/m151/t003 |
| t005 | Render parity — **DONE** | 30m | w7/m151/t004 |
| t006 | Simplify — **DONE** | 25m | w7/m151/t005 |
| t007 | Test coverage — **DONE** | 45m | w7/m151/t005 |
| t008 | Closeout | 15m | w7/m151/t006, w7/m151/t007 |

## Definition of done

- [x] Reproductions distinguish deleted references from incompatible live-environment membership.
- [x] The acceptance matrix covers both datastore kinds and preserves resource-owned allowlists.
- [x] A datastore moved to another project has no membership or inherited rules from the former project’s environment.
- [x] Authorized no-op reassignment does not churn state; refusal does not weaken protection.
- [x] Environment deletion preserves valid parent-project membership.
- [x] Project deletion leaves surviving datastores unassigned.
- [x] Interrupted cleanup can be retried without clearing a concurrent valid reassignment.
- [x] Existing provably stale references are repaired idempotently.
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

## Drain verdict — 2026-09-21: blocked after implementation

The source findings were reproduced and fixed for Postgres and Key Value. Project departures atomically clear incompatible environment membership and inherited policy; environment deletion preserves the project; project deletion leaves surviving datastores unassigned. Conditional cleanup and optimistic locking preserve concurrent reassignment; expected-environment checks also prevent stale ACL fan-out from reapplying old rules after departure. Resource-owned allowlists remain unchanged, and authorized no-ops avoid writes. The existing reconciler repairs only provably stale placement in control-plane-owned workspace namespaces and retains/reports ambiguous ownership.

**Evidence:** before-fix failures in `/tmp/bex-w7-m151-repair-red.log` and `/tmp/bex7-m151-deletion-baseline.log`; datastore placement regressions; 27 composed REST/GraphQL/MCP mutation/read cases; 85 dashboard grouping tests; focused race tests; full backend suite; and workspace lint/dead-code analysis. The repair cancellation test proves fairness without sleep-based timing. Postgres/OpenFGA opt-in integration endpoints were unset. No live UI or cluster walkthrough is claimed.

**Parity:** ADR018/ADR032 now explicitly distinguish bex's preserve-and-detach deletion from Render's empty-only REST deletion and cascading dashboard deletion. This milestone changes no datastore deletion, ownership-transfer or public grouping API contract.

**Remaining gate:** t004 requires a dated live dev-7 move/delete/recovery walkthrough and matching dashboard/API reads; t008 must wait for that acceptance. The cluster was restored, but the shared OrbStack VM's 16 GiB memory ceiling causes kernel global OOM and unstable dev-7 database/auth/API dependencies. The user/operator must restore enough shared memory capacity, scheduling a shared OrbStack restart if increasing its limit or freeing sufficient memory without disrupting unrelated stacks. Then run `bash scripts/dev-env.sh 7 up`, exercise isolated datastore fixtures through moves, deletion and recovery, remove the fixtures and record the live result. The detailed recovery evidence is in [m149](../m149/README.md). No implementable m151 task is left open.
