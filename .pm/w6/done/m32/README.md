# w6 · m32 — Env groups, first-class: Render list envelope + pagination, purge on workspace delete, migration audit trail

**Worker:** worker6 **Goal:** Env groups stop being the last workspace resource with a non-Render list shape and a delete leak: `GET /v1/env-groups` returns Render's `{envGroup, cursor}` envelope and honors `cursor`/`limit`; deleting a workspace purges its env groups; the lazy on-read ownership migration leaves an audit trail. **Status:** done

## Tasks (in order)

| id   | title                                                                             | est | depends_on |
| ---- | ---------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Verify Render's `GET /env-groups` contract in the pinned OpenAPI — **DONE**                  | 15m | —                |
| t002 | Implement the `{envGroup, cursor}` envelope + `cursor`/`limit`; check GraphQL/MCP — **DONE** | 45m | t001             |
| t003 | Extend the workspace-delete purger to env groups — **DONE**                                  | 60m | —                |
| t004 | Emit an audit event when the lazy on-read ownership migration fires — **DONE**               | 25m | —                |
| t005 | Render parity — **DONE**                                                                      | 25m | t002, t003, t004 |
| t006 | Simplify — **DONE**                                                                           | 20m | t005             |
| t007 | Test coverage — **DONE**                                                                      | 45m | t005             |
| t008 | Closeout — **DONE**                                                                           | 15m | t007             |

## Definition of done

`GET /v1/env-groups` responses are Render-shaped (`[{envGroup: {...}, cursor: "..."}]`) and pageable (walk test terminates with full coverage); deleting a workspace leaves zero orphaned env groups (regression test); a lazy ownership migration writes an `audit_events` row; ADR018's env-groups row divergence list is updated.

## Source + Goal linkage

- **Source:** `/pm-brainstorm` round 13, 2026-07-15 — three converging finds: (a) mechanical-consistency mining, verified: `lego/backend/internal/envgroups/rest.go:33-40` returns a bare `[]EnvGroupView`, no envelope, no paging (the same envelope-class bug already fixed for environments in w4/m22 and datastores in w8/m13); also the unwritten "note in t005" residual from `w9/done/m10/done/t003.md:29`. (b) Closeout residual `.pm/w7/done/m12/t001.md:46`: the workspace purger's env-group exclusion rationale ("no tenant attribution") went stale the day w6/m24 shipped attribution. (c) Closeout residual `.pm/w6/done/m24/README.md:43`: the lazy on-read migration "would benefit from an explicit audit trail" — owned nowhere.
- **Goal linkage:** Render parity (env-groups rows, docs/ADR018-render-parity.md) + workspace-lifecycle completeness (w6's charter, docs/ADR024-members.md / RESEARCH-workspaces.md).
- **Expected outcome:** Render SDKs can parse and page the env-groups list; workspace delete really deletes; ownership migrations are auditable.
- **Why now:** the purger-exclusion rationale is documented as stale on the board itself; the envelope bug breaks any Render client that parses the list; t002 inherits w1/m42 t001's pagination convention while the `StablePage` pattern is warm.
- **Render parity:** included — REST/GraphQL/MCP list surface change.

## Closeout evidence

- Render's endpoint OpenAPI parameter/schema extraction and its official pagination contract were checked on 2026-07-15. The endpoint schema's bare-array declaration is stale relative to the live `{envGroup, cursor}` response; t001 records the discrepancy rather than hiding it.
- `TestREST_EnvGroupsPaginationWalkUsesRenderEnvelope`, `TestWorkspacePurgerDeletesOnlyTheGivenWorkspacesGroups`, and `TestEnvGroup_MigratesLegacyOwnerlessGroupOnceStoreIsLive` lock the three named failure modes. GraphQL and MCP paging have adapter-level regressions too.
- `cd lego/backend && go build ./... && go test ./...` passed; `make lint-backend` passed with 0 issues.
- `/simplify` consolidated pagination behind `pageEnvGroups` and all audit writes behind the bounded `recordAudit` sink path.
