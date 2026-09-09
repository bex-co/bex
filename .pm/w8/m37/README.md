# w8 · m37 — Blueprint lifecycle consistency

**Worker:** worker8 **Goal:** Disconnected Blueprints stay disconnected, concurrent syncs apply in a coordinated order, and interrupted runs settle to an honest retryable outcome. **Status:** todo

**Estimate:** 4h 30m implementation; 6h 30m including standing closing tasks. **Priority:** 2 in the approved 2026-09-08 queue.

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Treat disconnected Blueprint IDs as absent on ordinary operations | 45m | w8/m36/t008 |
| t002 | Coordinate sync admission and completion across replicas | 60m | w8/m37/t001 |
| t003 | Coordinate disconnect with active execution and ownership cleanup | 60m | w8/m37/t002 |
| t004 | Settle abandoned sync runs after process loss | 60m | w8/m37/t002 |
| t005 | Preserve current settings and surface lifecycle outcomes | 45m | w8/m37/t003, w8/m37/t004 |
| t006 | Render parity | 30m | w8/m37/t005 |
| t007 | Simplify | 30m | w8/m37/t006 |
| t008 | Test coverage | 45m | w8/m37/t006, w8/m37/t007 |
| t009 | Closeout | 15m | w8/m37/t008 |

## Definition of done

- Disconnect is durable: lists and ordinary by-ID operations agree on absence, and old sync/update calls cannot reactivate the record. Explicit creation is the only permitted re-establishment path and fences old execution authority.
- One Blueprint has at most one active apply across API replicas. A stale worker cannot start another apply step or overwrite a newer run, current settings, or disconnected status.
- Disconnect during an active apply returns the documented busy conflict; after successful disconnect, old workers cannot resume management. Ownership/grouping cleanup preserves all deployed resources and retains observable failures.
- A process dying after acceptance or during application leads to an interrupted/error result within the documented recovery bound. Healthy runs are retained, recovery work is bounded, and partial application is retried only through an explicit new attempt.
- Real-Postgres concurrency, paused-worker, disconnect, completion-failure, restart, and settings-change cases pass. Existing REST/GraphQL/MCP/dashboard views agree, including a dev-8 two-client walkthrough.

## Source + Goal linkage

- **Source:** User-approved proposal 2 from /pm-brainstorm for w8 on 2026-09-08; source-reviewed disconnected reads, unguarded upsert/completion, and missing interrupted-run recovery.
- **Goal linkage:** ADR008 deterministic intent and agent-readable state; ADR049's safe Blueprint apply contract. See [ADR008](../../../docs/ADR008-vision.md), [ADR049](../../../docs/ADR049-render-yaml-parity.md), and [the project goals](../../GOAL.md).
- **Expected outcome:** Disconnect remains effective, syncs do not fight over one Blueprint, and history reaches an honest terminal state after failures or restarts.
- **Why now:** The existing verbs already allow sequential resurrection and concurrent lifecycle overwrites. m36 fixes source integrity first; m38 must consume this coordination before admitting more automatic work.
- **Gap analysis:** w2/m62 added lifecycle and history. w8/m20 added grouping cleanup; m23 added resource ownership. Neither coordinates active execution with disconnect. w5/m85 provides a local pattern for persisted bounded recovery but is a separate capability.
- **Render parity:** included. This changes tenant-facing Blueprint behavior across existing REST/GraphQL/MCP/UI surfaces. Update the relevant source/lifecycle/auto-sync clauses of [ADR018's Blueprint row](../../../docs/ADR018-render-parity.md#deployment-sources--iac); the row's independent unsupported-field and evidence gaps stay partial.
- **Anti-goals:** this scope stays within [DO_NOT_DO.md](../../DO_NOT_DO.md) and does not reopen any deliberate non-goal.

## Evidence and implementation anchors

The paths/functions below were reviewed on 2026-09-08; recheck current code before implementation. This is source-review evidence, not a live production reproduction.

- `lego/backend/internal/store/blueprints.go`: GetBlueprint vs ListBlueprints; UpsertBlueprint, UpdateBlueprint, DisconnectBlueprint, and sync state writes.
- `lego/backend/internal/apps/blueprint.go`: prepareSyncManifest, runSync, UpdateBlueprint, DisconnectBlueprint.
- `lego/backend/internal/apps/blueprint_ownership.go`: post-apply ownership stamping and disconnect clearing.
- [Grouping cleanup](../done/m20/README.md), [ownership](../done/m23/README.md), and [agent-dispatch recovery precedent](../../w5/done/m85/README.md).

## Dependencies and execution

t001 depends on w8/m36/t008 (verified closeout). Resolve the task under done/ after archival; never recreate its old path.

Use the workstream's isolated dev-8 environment for any live development checks, following [.pm/w8/README.md](../README.md). Preserve namespace, authorization, billing, ownership, and protected-environment boundaries throughout. IDs in depends_on remain canonical when their files move under done/.

## Scope

Use backend/store coordination and existing resource intent boundaries. Recommend a busy conflict for disconnect while apply still owns authority; interrupted partial work becomes an explicit retryable error. No resource deletion, generic workflow engine, or automatic partial-apply replay.
