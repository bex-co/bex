# w7 · m152 — Validate grouping mutations before changing members

**Worker:** worker7 **Goal:** ADR008 predictable agent operations; ADR018 Projects & environments (grouping). **Status:** todo

**Size:** 150m implementation; 265m including closing tasks (8 tasks).

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Capture invalid membership and later-environment refusal cases | 40m | — |
| t002 | Validate requested datastore IDs before membership writes | 40m | w7/m152/t001 |
| t003 | Preflight project-cascade permissions before member clearing | 40m | w7/m152/t001 |
| t004 | Align refusal behavior across adapters | 30m | w7/m152/t002, w7/m152/t003 |
| t005 | Render parity | 30m | w7/m152/t004 |
| t006 | Simplify | 25m | w7/m152/t005 |
| t007 | Test coverage | 45m | w7/m152/t005 |
| t008 | Closeout | 15m | w7/m152/t006, w7/m152/t007 |

## Definition of done

- [ ] Capture both datastore kinds and mixed valid/invalid replacement sets.
- [ ] A multi-environment fixture exposes any mutation preceding a later authorization refusal.
- [ ] Invalid replacement sets leave every existing membership unchanged.
- [ ] Unknown and foreign ids have equivalent non-disclosing refusal behavior.
- [ ] An authorized empty list still clears membership.
- [ ] A project delete refused on any child leaves every child and member unchanged.
- [ ] Authorized cascade still succeeds and protected-environment checks remain enforced.
- [ ] Adapters agree on rejection and unchanged state.
- [ ] Exact error semantics and compatibility implications are documented without invented Render claims.

## Source + Goal linkage

- **Source:** Approved by `$pm all for w7 and $ship` after the additional w7 brainstorm (2026-09-17); source review at d7ea2ac52. These are source-backed findings, not newly reproduced production failures. environments/service.go SetDatabases/SetKeyValues, setResourceMembers, clearMembersForProject and clearEnvironmentMembers; projects/service.go setResourceMembers and Delete.
- **Goal linkage:** ADR008 predictable agent operations; ADR018 Projects & environments (grouping).
- **Expected outcome:** Validate grouping mutations before changing members. The acceptance matrix above defines the observable result.
- **Why now / deduplication:** Environment datastore membership uses ignoreUnknownIDs while env groups reject unknown IDs before writing. Project deletion authorizes and clears one environment at a time, so later refusal can follow earlier changes. Current grouping milestones implement assignment and ACL checks, not whole-request admission.
- **Render parity:** Included, correcting lifecycle/admission behavior under the existing ADR018 grouping row (currently ✅), not adding a new capability.

## Dependencies and scope

Implementation may begin independently. Live dev-7 acceptance depends on `w7/m148/t007` (currently `.pm/w7/blocked/m148/t007.md`); do not recreate its former path. Coordinate shared grouping files between m151 and m152; serialize conflicting edits while preserving both sets of invariants. No new cross-milestone completion dependency is required.

No distributed transaction guarantee across Kubernetes and Postgres, new roles, weakened authorization, or lifecycle-cleanup duplication with m151.
