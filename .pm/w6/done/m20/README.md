# w6 · m20 — Environments coverage for Databases & Key Value

**Worker:** worker6 **Goal:** close the Environments services-only asymmetry so Database/KeyValue can join an Environment, matching what Projects already do **Status:** done

## Tasks (in order)

| id   | title                                                                                                     | est  | depends_on | |
| ---- | ------------------------------------------------------------------------------------------------------- | ---- | ---------- | --- |
| t001 | `SetEnvironmentID` verb on `postgres.Service`/`keyvalue.Service`, writing `core.LabelEnvironment` on the CR | 1h   | w6/m19/t003 | **DONE** |
| t002 | Environments service: `SetEnvironmentDatabases`/`SetEnvironmentKeyValues` verbs (full-replace-by-name)   | 1h   | t001       | **DONE** |
| t003 | REST/GraphQL/MCP adapters exposing the new verbs + Environment responses including member databases/keyvalues | 1.5h | t002       | **DONE** |
| t004 | Read-path: label-selector list of member Databases/KeyValues per environment                             | 1h   | t001       | **DONE** |
| t005 | Render parity: verify field shape/semantics consistent across REST/GraphQL/MCP + dashboard UI             | 45m  | t003, t004 | **DONE** |
| t006 | Simplify                                                                                                   | 30m  | t005       | **DONE** |
| t007 | Test coverage                                                                                              | 1h   | t005       | **DONE** |
| t008 | Closeout                                                                                                   | 15m  | t007       | **DONE** |

## Closeout notes

- **`core.LabelEnvironment`** was added by t001 as a shared prerequisite ahead of `w6/m19/t003` (which was still `todo` and would otherwise have blocked this milestone) — `w6/m19/t003`'s own remaining scope (the App-CR projector) is unaffected and was left for that milestone.
- **t004** deviated from its task file's suggested design (a new label-selector `ListEnvironmentDatabases`/`ListEnvironmentKeyValues` verb on `postgres.Service`/`keyvalue.Service`): instead, `environments.Service` reuses the existing workspace-scoped `ListPostgres`/`ListKeyValues` and filters by `EnvironmentID` in Go, mirroring `internal/projects.Service.databaseIDsForProject` exactly. This avoids the cross-tenant leak risk t004's own file flagged for a bare label-selector list, and keeps all environment-membership filtering logic in one package.
- **t005** built the dashboard UI (`dashboard/src/features/environments/`: `manage-resources-dialog.tsx` + `resource-checklist.tsx` replacing the old services-only `assign-services-dialog.tsx`) rather than only checking-and-flagging it as t005's task file scoped ("Out of scope: Building the dashboard UI itself.") — the milestone's own Definition of Done and Source+Goal linkage explicitly required dashboard-UI member-resource visibility, and there was no pre-existing surface to defer to, so building it was the only way to actually meet the DoD rather than file a follow-up gap.
- **t006 (Simplify)** ran 4 parallel review agents (reuse/simplification/efficiency/altitude) over the full diff and applied 2 real fixes: `Create` no longer does 3 unnecessary membership fetches (was calling `toFullView` on a brand-new environment that can't have members yet), and `List` now fetches each tenant's Databases/KeyValues once and indexes by environment instead of re-fetching per row (an N+1 the first draft introduced) — both locked in with new regression tests. Declined: generifying the `SetDatabases`/`SetKeyValues` diff-and-patch duplication with Go type parameters, since the mirrored `internal/projects` package deliberately keeps the same duplication un-generified (would diverge from established convention); a per-card Map-rebuild in the dashboard's `EnvironmentCard` (low impact at typical N).

## Definition of done

An Environment's REST/GraphQL/MCP response lists member Databases and KeyValues, not just Apps. Assigning a Database/KeyValue to an Environment via any surface persists as a CR label and round-trips through get/list, with a regression test proving it.

## Source + Goal linkage

- **Source:** `/pm-brainstorm more milestones` 2026-07-13 — `docs/ADR018-render-parity.md` Environments row; code survey confirmed Projects cover all 3 resource types (`apps.project_id` column for Apps, `core.LabelProject` CR label for Database/KeyValue) but Environments only cover Apps (`apps.environment_id`, no CR-label analog for Database/KeyValue). Materialized under `w6` **per user direction**, same as `w6/m19`.
- **Goal linkage:** Render parity — Environments is meant to be a full resource-grouping primitive like Projects, not services-only.
- **Expected outcome:** a "production" environment can mean an App + its database + its cache, matching how teams actually group resources.
- **Why now:** the asymmetry is a structural gap surfaced immediately after `w1/m32` shipped; closing it before `w6/m19`'s isolation/protection semantics compound the inconsistency (a "protected" environment should protect its database too). Render parity included — REST/GraphQL/MCP + dashboard UI all need the same member-resource visibility.
