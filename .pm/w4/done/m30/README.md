# w4 · m30 — Environments partial-update core verb + AuthorizeApp loop hoist

**Worker:** worker4 **Goal:** the environments PATCH merge lives exactly once in the service layer (all three surfaces can partial-update), and `AuthorizeApp`'s name-collision fallback resolves the acting workspace once, not once per candidate. **Status:** DONE 2026-07-16

## Tasks (in order)

| id   | title                                                                                                                | est | depends_on | status |
| ---- | ---------------------------------------------------------------------------------------------------------------------- | --- | ---------- | --- |
| t001 | Core `EnvironmentPatch` + `Update(ctx, id, patch)` verb owning the merge + pre-migration default; REST PATCH thins to decode+call | 45m | —          | DONE |
| t002 | GraphQL `updateEnvironment` + MCP `update_environment` riding the core verb                                             | 30m | t001       | DONE |
| t003 | Hoist the acting-workspace resolution above `AuthorizeApp`'s candidate loops; assess OpenFGA BatchCheck (verify-first)   | 40m | —          | DONE |
| t004 | Render parity — PATCH semantics identical across REST/GraphQL/MCP; compare against Render's `PATCH /environments/{id}`  | 30m | t002       | DONE |
| t005 | Simplify — `/simplify` over the changed code                                                                             | 20m | t003, t004 | DONE |
| t006 | Test coverage — merge semantics in the service layer across surfaces; store-call-count assertion on the collision path   | 40m | t003, t004 | DONE |
| t007 | Closeout — verify DoD, sync status, move to done                                                                         | 15m | t006       | DONE |

## Definition of done

The read-modify-write merge (pointer overrides + the `""→unprotected` pre-migration default) exists exactly once, in `environments/service.go`; REST PATCH, GraphQL `updateEnvironment`, and MCP `update_environment` all ride it and produce identical results for identical patches. In `AuthorizeApp`'s fallback loops, N colliding candidates trigger exactly one acting-workspace resolution (asserted by a store-call-count test).

## Source + Goal linkage

- **Source:** promotes inbox note `w4/022` (filed by w4/m23's simplify pass), re-scoped by `/pm-brainstorm` round 18, 2026-07-15: the note's create-with-ACL half shipped same-day in **w6/m33** (`CreateWithACL` is a core verb on all three surfaces, `436fd9c2^..` range) — this milestone covers only the remainder, both re-verified live: the PATCH merge is still adapter-owned (`environments/rest.go:229-296`, calling full-replace `SetACL`), GraphQL/MCP have no partial-update verb (only `Rename` + `setEnvironmentACL`), and `resourceWorkspace` (`core/base.go:625`) re-runs the loop-invariant `resolveWorkspace(ctx)` per candidate inside `AuthorizeApp`'s three fallback loops even though the acting workspace is already resolved at `base.go:452`.
- **Goal linkage:** ADR006's one-core/thin-adapters rule + cross-surface Render parity; the authz hoist trims the platform's worst-case-latency authorization path (w4 owns multi-tenant security).
- **Expected outcome:** any future surface (or a Render-client PATCH) gets the same merge semantics from one implementation; the pre-migration default cannot drift between surfaces; collision-heavy `AuthorizeApp` calls stop issuing N identical workspace-resolution queries.
- **Why now:** the create half landing today makes the remaining adapter-owned merge the odd one out — every day it stays there is a day a second surface can fork it. Render parity task included: the change adds GraphQL/MCP verbs and touches REST semantics.
