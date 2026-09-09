# w8 · m38 — Resource-independent Blueprint auto-sync

**Worker:** worker8 **Goal:** A signed push reliably schedules the affected connected Blueprint even when no repository-backed App exists, and acknowledged automatic work survives an API restart. **Status:** todo

**Estimate:** 4h implementation; 6h including standing closing tasks. **Priority:** 3 in the approved 2026-09-08 queue.

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Match connected Blueprints independently of App candidates | 45m | w8/m37/t009 |
| t002 | Detect changes to the configured Blueprint manifest | 45m | w8/m38/t001 |
| t003 | Persist deduplicated Blueprint intent before webhook acknowledgment | 60m | w8/m38/t002 |
| t004 | Process automatic syncs through bounded durable execution | 60m | w8/m38/t003 |
| t005 | Wire automatic run lifecycle into existing startup and history surfaces | 30m | w8/m38/t004 |
| t006 | Render parity | 30m | w8/m38/t005 |
| t007 | Simplify | 30m | w8/m38/t006 |
| t008 | Test coverage | 45m | w8/m38/t006, w8/m38/t007 |
| t009 | Closeout | 15m | w8/m38/t008 |

## Definition of done

- A signed push to the linked branch and changed manifest updates database-only, environment-group-only, image-backed, and cross-repository service Blueprints without requiring a matching repository-backed App.
- Wrong branches, inactive/disconnected Blueprints, Auto Sync disabled, unrelated files with complete evidence, invalid signatures, and foreign/unbound installations do not apply or allocate unnecessary work.
- Incomplete changed-path evidence does not lose a real change; removal/fetch failure is diagnosable and accepted source provenance is immutable.
- Successful acknowledgment follows durable acceptance. Duplicate delivery produces one logical intent, and restarting after acknowledgment does not lose pending work or repeat App redeploys.
- Workers are bounded, coordinated across replicas, and preserve later eligible changes without allowing stale deliveries to roll configuration backward. Policy is rechecked at execution; ambiguous partial apply follows m37's explicit failure/retry contract.
- Real-Postgres replay/restart tests and a dev-8 signed-push walkthrough pass, including no-App fixtures, disconnect/Auto Sync changes, history agreement, and fixture cleanup.

## Source + Goal linkage

- **Source:** User-approved proposal 3 from /pm-brainstorm for w8 on 2026-09-08; source review found the no-App early return, App-derived tenant discovery, and unpersisted goroutine dispatch.
- **Goal linkage:** ADR008 deterministic deployment, .pm/GOAL.md goal 3 (Git push to deploy), and the supported Blueprint compositions in ADR049. See [ADR008](../../../docs/ADR008-vision.md), [ADR049](../../../docs/ADR049-render-yaml-parity.md), and [the project goals](../../GOAL.md).
- **Expected outcome:** All supported connected Blueprint compositions receive reliable automatic updates, with accepted work and failures visible in existing history.
- **Why now:** Database-only, image-backed, and separate-source compositions already ship but bypass automatic delivery. m36 and m37 must finish first so expanded intake uses correct sources and coordinated execution.
- **Gap analysis:** w2/m62 established auto-sync. w1/m69 fixed acting-tenant attribution but retained App-derived discovery. w8/m19 supports the compositions/custom paths; none of those milestones persisted automatic acceptance.
- **Render parity:** included. This changes tenant-facing Blueprint behavior across existing REST/GraphQL/MCP/UI surfaces. Update the relevant source/lifecycle/auto-sync clauses of [ADR018's Blueprint row](../../../docs/ADR018-render-parity.md#deployment-sources--iac); the row's independent unsupported-field and evidence gaps stay partial.
- **Anti-goals:** this scope stays within [DO_NOT_DO.md](../../DO_NOT_DO.md) and does not reopen any deliberate non-goal.

## Evidence and implementation anchors

The paths/functions below were reviewed on 2026-09-08; recheck current code before implementation. This is source-review evidence, not a live production reproduction.

- `lego/backend/internal/apps/webhook.go`: no-candidate early return (~329), goroutine dispatch (~357), repoCandidates and App-derived tenant collection.
- `lego/backend/internal/apps/blueprint.go`: triggerBlueprintSync and runSync.
- `lego/backend/internal/store/gitwebhook.go`: existing replay/claim boundary to preserve.
- [Auto-sync tenant-binding milestone](../../w1/done/m69/README.md) and [Git-connected Blueprint milestone](../../w2/done/m62/README.md).
- [Render Blueprints documentation](https://render.com/docs/infrastructure-as-code): pushes changing the configured Blueprint file apply added/modified resources.

## Dependencies and execution

t001 depends on w8/m37/t009 (verified closeout). Resolve the task under done/ after archival; never recreate its old path.

Use the workstream's isolated dev-8 environment for any live development checks, following [.pm/w8/README.md](../README.md). Preserve namespace, authorization, billing, ownership, and protected-environment boundaries throughout. IDs in depends_on remain canonical when their files move under done/.

## Scope

GitHub-only, current Blueprint resources, and the existing backend store/worker architecture. No new deployment products, external queue, resource sync-delete, takeover automation, or change to App build-filter semantics.
