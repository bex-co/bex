# w8 · m36 — Blueprint Git source integrity

**Worker:** worker8 **Goal:** A successful Git-backed Blueprint sync applies complete manifest bytes from the exact commit it reports, and source failures leave an actionable result without applying stale configuration. **Status:** todo

**Estimate:** 3h 15m implementation; 5h including standing closing tasks. **Priority:** 1 in the approved 2026-09-08 queue.

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Reject incomplete and oversized Git file responses | 45m | — |
| t002 | Fetch Blueprint content at its resolved immutable commit | 45m | w8/m36/t001 |
| t003 | Fail Git-backed sync when its source is unavailable | 45m | w8/m36/t002 |
| t004 | Record source failures and require a persisted sync run before apply | 60m | w8/m36/t003 |
| t005 | Render parity | 30m | w8/m36/t004 |
| t006 | Simplify | 20m | w8/m36/t005 |
| t007 | Test coverage | 45m | w8/m36/t005, w8/m36/t006 |
| t008 | Closeout | 10m | w8/m36/t007 |

## Definition of done

- Git-backed sync applies only complete content from the exact reported commit. Branch movement between resolution and fetch cannot mix revisions.
- Missing files, denied Git access, read interruption, over-limit responses, and missing fetch capability cause zero workload mutations; the last valid manifest is preserved.
- For an existing Blueprint with working persistence, source/validation failures appear in sync history with actionable sanitized reasons. Pre-identity create failures and persistence outages return errors without applying or inventing resources/history.
- Explicit supplied-manifest sync and approved legacy filename discovery preserve their documented behavior; billing, tenant, protected-environment, and ownership guards remain effective.
- Fault-injected Git responses and recorded side-effect assertions reproduce the original failures and pass after the fix. Applicable adapter and dashboard checks agree; evidence records what was executed.

## Source + Goal linkage

- **Source:** User-approved proposal 1 from /pm-brainstorm for w8 on 2026-09-08. Source review identified swallowed fetch/read failures and branch/content provenance drift; this materialization schedules the implementation, it does not claim a production reproduction.
- **Goal linkage:** ADR008 deterministic deployment and agent-readable state, plus .pm/GOAL.md goal 3 (Git push to deploy). See [ADR008](../../../docs/ADR008-vision.md), [ADR049](../../../docs/ADR049-render-yaml-parity.md), and [the project goals](../../GOAL.md).
- **Expected outcome:** A successful sync means the requested Git source was consumed intact; a source failure cannot silently reapply old configuration.
- **Why now:** Ordinary upstream failures currently produce misleading success, and every fetched manifest trusts a potentially incomplete file. Fix the source contract before m37 coordinates execution and m38 expands automatic delivery.
- **Gap analysis:** w2/m62 established Git-connected Blueprints; its t004 required error runs for fetch failures. w6/m50 added errorMessage after application failures. w8/m19/m21/m23 covered schema translation, dashboard review, and ownership, not this reader/source gap.
- **Render parity:** included. This changes tenant-facing Blueprint behavior across existing REST/GraphQL/MCP/UI surfaces. Update the relevant source/lifecycle/auto-sync clauses of [ADR018's Blueprint row](../../../docs/ADR018-render-parity.md#deployment-sources--iac); the row's independent unsupported-field and evidence gaps stay partial.
- **Anti-goals:** this scope stays within [DO_NOT_DO.md](../../DO_NOT_DO.md) and does not reopen any deliberate non-goal.

## Evidence and implementation anchors

The paths/functions below were reviewed on 2026-09-08; recheck current code before implementation. This is source-review evidence, not a live production reproduction.

- `lego/backend/internal/github/client.go` GetFileContents / GetRepoCommitSHA (review snapshot: ~813 onward).
- `lego/backend/internal/github/service.go` fetchBlueprintFile (~1003 onward).
- `lego/backend/internal/apps/blueprint.go` CreateBlueprint / runSync (~384 / ~592 onward).
- [Original sync requirement](../../w2/done/m62/done/t004.md) and [error-history milestone](../../w6/done/m50/README.md).
- [GitHub repository content API](https://docs.github.com/en/rest/repos/contents#get-repository-content): ref may identify a commit; raw content can exceed bex's local 1 MiB limit.

## Dependencies and execution

No open milestone prerequisite. Start with t001.

Use the workstream's isolated dev-8 environment for any live development checks, following [.pm/w8/README.md](../README.md). Preserve namespace, authorization, billing, ownership, and protected-environment boundaries throughout. IDs in depends_on remain canonical when their files move under done/.

## Scope

GitHub-only and the existing Blueprint surface. No preview environments, sync-delete, new CLI, or broader workflow engine. Execution recovery and serialization belong to m37; durable automatic acceptance belongs to m38.
