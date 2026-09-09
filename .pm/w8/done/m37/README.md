# w8 · m37 — Blueprint lifecycle consistency

**Worker:** worker8 **Goal:** Disconnected Blueprints stay disconnected, concurrent syncs apply in a coordinated order, and interrupted runs settle to an honest retryable outcome. **Status:** done

**Estimate:** 4h 30m implementation; 6h 30m including standing closing tasks. **Priority:** 2 in the approved 2026-09-08 queue.

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Treat disconnected Blueprint IDs as absent on ordinary operations — **DONE** | 45m | w8/m36/t008 |
| t002 | Coordinate sync admission and completion across replicas — **DONE** | 60m | w8/m37/t001 |
| t003 | Coordinate disconnect with active execution and ownership cleanup — **DONE** | 60m | w8/m37/t002 |
| t004 | Settle abandoned sync runs after process loss — **DONE** | 60m | w8/m37/t002 |
| t005 | Preserve current settings and surface lifecycle outcomes — **DONE** | 45m | w8/m37/t003, w8/m37/t004 |
| t006 | Render parity — **DONE** | 30m | w8/m37/t005 |
| t007 | Simplify — **DONE** | 30m | w8/m37/t006 |
| t008 | Test coverage — **DONE** | 45m | w8/m37/t006, w8/m37/t007 |
| t009 | Closeout — **DONE** | 15m | w8/m37/t008 |

## Definition of done

- Disconnect is durable: lists and ordinary by-ID operations agree on absence, and old sync/update calls cannot reactivate the record. Explicit creation is the only permitted re-establishment path and fences old execution authority.
- One Blueprint has at most one active apply across API replicas. A stale worker cannot start another apply step or overwrite a newer run, current settings, or disconnected status.
- Disconnect during an active apply returns the documented busy conflict; after successful disconnect, old workers cannot resume management. Ownership/grouping cleanup preserves all deployed resources and retains observable failures.
- A process dying after acceptance or during application leads to an interrupted/error result within the documented recovery bound. Healthy runs are retained, recovery work is bounded, and partial application is retried only through an explicit new attempt.
- Real-Postgres concurrency, paused-worker, disconnect, completion-failure, restart, and settings-change cases pass. Existing REST/GraphQL/MCP/dashboard views agree, including a dev-8 two-client walkthrough.

## Source + Goal linkage

- **Source:** User-approved proposal 2 from /pm-brainstorm for w8 on 2026-09-08; source-reviewed disconnected reads, unguarded upsert/completion, and missing interrupted-run recovery.
- **Goal linkage:** ADR008 deterministic intent and agent-readable state; ADR049's safe Blueprint apply contract. See [ADR008](../../../../docs/ADR008-vision.md), [ADR049](../../../../docs/ADR049-render-yaml-parity.md), and [the project goals](../../../GOAL.md).
- **Expected outcome:** Disconnect remains effective, syncs do not fight over one Blueprint, and history reaches an honest terminal state after failures or restarts.
- **Why now:** The existing verbs already allow sequential resurrection and concurrent lifecycle overwrites. m36 fixes source integrity first; m38 must consume this coordination before admitting more automatic work.
- **Gap analysis:** w2/m62 added lifecycle and history. w8/m20 added grouping cleanup; m23 added resource ownership. Neither coordinates active execution with disconnect. w5/m85 provides a local pattern for persisted bounded recovery but is a separate capability.
- **Render parity:** included. This changes tenant-facing Blueprint behavior across existing REST/GraphQL/MCP/UI surfaces. Update the relevant source/lifecycle/auto-sync clauses of [ADR018's Blueprint row](../../../../docs/ADR018-render-parity.md#deployment-sources--iac); the row's independent unsupported-field and evidence gaps stay partial.
- **Anti-goals:** this scope stays within [DO_NOT_DO.md](../../../DO_NOT_DO.md) and does not reopen any deliberate non-goal.

## Evidence and implementation anchors

The paths/functions below were reviewed on 2026-09-08; recheck current code before implementation. This is source-review evidence, not a live production reproduction.

- `lego/backend/internal/store/blueprints.go`: GetBlueprint vs ListBlueprints; UpsertBlueprint, UpdateBlueprint, DisconnectBlueprint, and sync state writes.
- `lego/backend/internal/apps/blueprint.go`: prepareSyncManifest, runSync, UpdateBlueprint, DisconnectBlueprint.
- `lego/backend/internal/apps/blueprint_ownership.go`: post-apply ownership stamping and disconnect clearing.
- [Grouping cleanup](../m20/README.md), [ownership](../m23/README.md), and [agent-dispatch recovery precedent](../../../w5/done/m85/README.md).

## Dependencies and execution

t001 depends on w8/m36/t008 (verified closeout). Resolve the task under done/ after archival; never recreate its old path.

Use the workstream's isolated dev-8 environment for any live development checks, following [.pm/w8/README.md](../../README.md). Preserve namespace, authorization, billing, ownership, and protected-environment boundaries throughout. IDs in depends_on remain canonical when their files move under done/.

## Scope

Use backend/store coordination and existing resource intent boundaries. Recommend a busy conflict for disconnect while apply still owns authority; interrupted partial work becomes an explicit retryable error. No resource deletion, generic workflow engine, or automatic partial-apply replay.

## Evidence (2026-09-08)

Implementation: `lego/backend/internal/store/blueprint_lifecycle.go` (Admit/Stage/Complete/FailAdmitted/Disconnect/ListAbandoned/Abandon + `ErrBlueprintSyncBusy`, 30-minute `BlueprintRunRecoveryBound`), `0111_blueprint_execution_fencing` migration, `blueprints.go` (t001 filters, manifest-only upsert conflict arm, terminal-capable run insert), `apps/blueprint.go` (admission flows, `completeAdmittedSync`, fenced stamp threading, cleanup-failure returns), `apps/blueprint_recovery.go` + `api/server.go` + `cmd/api/main.go` wiring, dashboard busy toasts + locales, `docs/ADR018-render-parity.md` Blueprint row (stays ◐).

- `cd lego/backend && BEX_TEST_DB_URI=postgres://postgres@localhost:55434/bex_test go test ./...` — exit 0, 62 packages ok, zero FAIL on a fresh database (disposable PG17 installed locally to match CI's postgres:17; servers stopped and removed after).
- Real-Postgres lifecycle tests (`store/blueprint_lifecycle_pg_test.go`, 11 tests): migration defaults, disconnected absence + no-revive upsert, cross-connection admission race (8 separate pools, exactly 1 win), completion fencing (terminal/cross-generation/post-disconnect), disconnect busy vs stale-settle-inline, abandonment (owned/orphan/newer-gen/idempotent), projection matrix, preflight status preservation, legacy stranded rows, bounded oldest-first sweep.
- Fake-backed unit tests (`apps/blueprint_test.go`, 10 new): absence + zero mutations, re-creation fencing, 8-way admission race (exactly 1 win, rest `BLUEPRINT_SYNC_BUSY`), disconnect-busy, stale completion, stamp fencing, mid-run settings preservation with paused projection, REST 409 + GraphQL `extensions.code` + MCP tool-error agreement, recoverer settle/idempotence. Race detector clean on the concurrency tests.
- Dashboard: `yarn test` 403 files / 3074 tests pass; new hook tests prove the busy vs generic toast branch with the real Apollo error class; eslint clean on touched files.
- `go vet` clean on all touched packages. `make lint-backend` still panics identically on the untouched tree (pre-existing pinned-tool/Go-1.27 issue, first recorded in m36). Dashboard `yarn typecheck` has one pre-existing unrelated error (`services.$serviceId.settings.test.tsx` outboundIps), also present on the clean tree.
- Pre-existing repair required for any PG evidence: migration `0107_datastore_observed_checkpoints` shipped on main with `cat -n` line-number artifacts (`N|` prefixes) making every real-Postgres test unmigratable; stripped the prefixes in place (no applied database could exist past the broken statement) — see the 0107 diff in this ship.
- Suite hygiene note: re-running the backend suite against the same database (without drop/create) fails pre-existing `TestPGGitWebhookReplay*` epoch tests on stale rows — a test-isolation wart outside this milestone; evidence runs above always use a fresh database.
- NOT done: the dev-8 two-client walkthrough (t005/t008). Environment block: Docker Desktop's VM crash-loops headlessly (init logs cycling, daemon down after 25+ minutes), so no kind cluster, bex-api, or dashboard could start; `kubectl` was installed and the daemon start attempted. Unblock command when Docker works: `bash scripts/dev-env.sh 8 up`, then the dashboard at `:50080` — open one blueprint in two tabs, sync in one while syncing in the other (busy toast), disconnect in one (absence in the other).
