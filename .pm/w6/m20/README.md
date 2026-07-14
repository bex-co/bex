# w6 · m20 — Environments coverage for Databases & Key Value

**Worker:** worker6 **Goal:** close the Environments services-only asymmetry so Database/KeyValue can join an Environment, matching what Projects already do **Status:** todo

## Tasks (in order)

| id   | title                                                                                                     | est  | depends_on |
| ---- | ------------------------------------------------------------------------------------------------------- | ---- | ---------- |
| t001 | `SetEnvironmentID` verb on `postgres.Service`/`keyvalue.Service`, writing `core.LabelEnvironment` on the CR | 1h   | w6/m19/t003 |
| t002 | Environments service: `SetEnvironmentDatabases`/`SetEnvironmentKeyValues` verbs (full-replace-by-name)   | 1h   | t001       |
| t003 | REST/GraphQL/MCP adapters exposing the new verbs + Environment responses including member databases/keyvalues | 1.5h | t002       |
| t004 | Read-path: label-selector list of member Databases/KeyValues per environment                             | 1h   | t001       |
| t005 | Render parity: verify field shape/semantics consistent across REST/GraphQL/MCP + dashboard UI             | 45m  | t003, t004 |
| t006 | Simplify                                                                                                   | 30m  | t005       |
| t007 | Test coverage                                                                                              | 1h   | t005       |
| t008 | Closeout                                                                                                   | 15m  | t007       |

## Definition of done

An Environment's REST/GraphQL/MCP response lists member Databases and KeyValues, not just Apps. Assigning a Database/KeyValue to an Environment via any surface persists as a CR label and round-trips through get/list, with a regression test proving it.

## Source + Goal linkage

- **Source:** `/pm-brainstorm more milestones` 2026-07-13 — `docs/ADR018-render-parity.md` Environments row; code survey confirmed Projects cover all 3 resource types (`apps.project_id` column for Apps, `core.LabelProject` CR label for Database/KeyValue) but Environments only cover Apps (`apps.environment_id`, no CR-label analog for Database/KeyValue). Materialized under `w6` **per user direction**, same as `w6/m19`.
- **Goal linkage:** Render parity — Environments is meant to be a full resource-grouping primitive like Projects, not services-only.
- **Expected outcome:** a "production" environment can mean an App + its database + its cache, matching how teams actually group resources.
- **Why now:** the asymmetry is a structural gap surfaced immediately after `w1/m32` shipped; closing it before `w6/m19`'s isolation/protection semantics compound the inconsistency (a "protected" environment should protect its database too). Render parity included — REST/GraphQL/MCP + dashboard UI all need the same member-resource visibility.
