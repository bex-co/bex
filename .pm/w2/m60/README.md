# w2 · m60 — P1 operator correctness: lifecycle, storage, type, and health

**Worker:** worker2 **Goal:** Make deletion, grow-only storage, workload type, and child-resource health converge correctly under real Kubernetes event semantics. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Admit Database deletion and finalizer events | 30m | w2/m59/t009 |
| t002 | Preserve grow-only Postgres storage across plan changes | 45m | w2/m59/t009 |
| t003 | Implement grow-only Valkey PVC expansion | 60m | w2/m59/t009 |
| t004 | Make App workload type immutable and reject transitions | 45m | w2/m59/t009 |
| t005 | Reconcile status-only child health and rollout revisions | 60m | w2/m59/t009 |
| t006 | Verify Render parity across affected lifecycle surfaces | 30m | t001, t002, t003, t004, t005 |
| t007 | Simplify the correctness fixes | 30m | t006 |
| t008 | Complete lifecycle and failure-mode coverage | 60m | t007 |
| t009 | Close out m60 | 15m | t008 |

## Definition of done

A Database deletion always reaches and completes finalization; Postgres plan downgrades never request PVC shrink; Valkey storage increases patch and observe the real PVC; App type changes are rejected consistently before they can orphan another workload kind; and App/KeyValue status follows child crashes and failed rollouts without requiring a spec edit. Tests prove each failure mode and the Render-facing surfaces return consistent results.

## Source + Goal linkage

- **Source:** `docs/ADR039-operator-audit-and-platform-reuse.md` O-03 through O-06. This is a corrective follow-up to completed datastore/service milestones (`w1/m14`, `w1/m15`, `w8/m14`, `w7/m12`) based on newly demonstrated event/storage/transition gaps.
- **Goal linkage:** ADR008 pillar 1's reliable self-service deployment and `GOAL.md` #1/#4/#7 (service lifecycle, PostgreSQL, security/correctness review).
- **Expected outcome:** Kubernetes actual state, bex CR status, and Render-facing lifecycle responses converge after delete, resize, plan change, type update, crash, and rollout failure.
- **Why now:** O-03–O-06 are High correctness blockers and form the lifecycle acceptance baseline any future, explicitly approved OpenChoreo/Korifi adapter must also pass. Render parity closing is included because plan/type/lifecycle errors and observed health are tenant-visible across REST, GraphQL, MCP, CLI, and dashboard paths.
