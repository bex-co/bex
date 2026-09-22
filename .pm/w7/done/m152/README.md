# w7 · m152 — Validate grouping mutations before changing members

**Worker:** worker7 **Goal:** ADR008 predictable agent operations; ADR018 Projects & environments (grouping). **Status:** done

**Size:** 150m implementation; 265m including closing tasks (8 tasks).

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Capture invalid membership and later-environment refusal cases — **DONE** | 40m | — |
| t002 | Validate requested datastore IDs before membership writes — **DONE** | 40m | w7/m152/t001 |
| t003 | Preflight project-cascade permissions before member clearing — **DONE** | 40m | w7/m152/t001 |
| t004 | Align refusal behavior across adapters — **DONE** | 30m | w7/m152/t002, w7/m152/t003 |
| t005 | Render parity — **DONE** | 30m | w7/m152/t004 |
| t006 | Simplify — **DONE** | 25m | w7/m152/t005 |
| t007 | Test coverage — **DONE** | 45m | w7/m152/t005 |
| t008 | Closeout — **DONE** | 15m | w7/m152/t006, w7/m152/t007 |

## Definition of done

- [x] Capture both datastore kinds and mixed valid/invalid replacement sets.
- [x] A multi-environment fixture exposes any mutation preceding a later authorization refusal.
- [x] Invalid replacement sets leave every existing membership unchanged.
- [x] Unknown and foreign ids have equivalent non-disclosing refusal behavior.
- [x] An authorized empty list still clears membership.
- [x] A project delete refused on any child leaves every child and member unchanged.
- [x] Authorized cascade still succeeds and protected-environment checks remain enforced.
- [x] Adapters agree on rejection and unchanged state.
- [x] Exact error semantics and compatibility implications are documented without invented Render claims.

## Source + Goal linkage

- **Source:** Approved by `$pm all for w7 and $ship` after the additional w7 brainstorm (2026-09-17); source review at d7ea2ac52. These are source-backed findings, not newly reproduced production failures. environments/service.go SetDatabases/SetKeyValues, setResourceMembers, clearMembersForProject and clearEnvironmentMembers; projects/service.go setResourceMembers and Delete.
- **Goal linkage:** ADR008 predictable agent operations; ADR018 Projects & environments (grouping).
- **Expected outcome:** Validate grouping mutations before changing members. The acceptance matrix above defines the observable result.
- **Why now / deduplication:** Environment datastore membership uses ignoreUnknownIDs while env groups reject unknown IDs before writing. Project deletion authorizes and clears one environment at a time, so later refusal can follow earlier changes. Current grouping milestones implement assignment and ACL checks, not whole-request admission.
- **Render parity:** Included, correcting lifecycle/admission behavior under the existing ADR018 grouping row (currently ✅), not adding a new capability.

## Dependencies and scope

Implementation may begin independently. Live dev-7 acceptance depends on `w7/m148/t007` (now `.pm/w7/done/m148/done/t007.md`). Coordinate shared grouping files between m151 and m152; serialize conflicting edits while preserving both sets of invariants. No new cross-milestone completion dependency is required.

No distributed transaction guarantee across Kubernetes and Postgres, new roles, weakened authorization, or lifecycle-cleanup duplication with m151.

## Completion evidence — 2026-09-21

Validated locally: full backend suite, all-module lint/deadcode, focused race tests, 75 composed API cases, 89 dashboard grouping tests, and dashboard typecheck pass. Red overlays proved both original defects before their fixes. See done/t001.md through done/t007.md for evidence and exact boundaries.

No live dev-7 acceptance was performed: the shared OrbStack VM memory gate recorded in blocked/m151 still prevents a stable local stack. This milestone's filed acceptance matrix requires behavioral regressions and adapter agreement, not a live walkthrough; all those criteria are met. Opt-in real Postgres/OpenFGA tests were not configured. Admission validates each membership replacement and preflights child permissions; it does not promise rollback after concurrent revocation or across multi-field MCP updates. ADR018 grouping parity remains partial.
