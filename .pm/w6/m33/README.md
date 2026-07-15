# w6 · m33 — Environments surface-parity chores: create ACL + list filters + field drift

**Worker:** worker6 **Goal:** The just-shipped environments surface stops being REST-first: creating a protected/isolated/ip-gated environment is one call on every surface, the documented list filters filter or 400 (never silently no-op), and the REST/GraphQL field sets match the pinned Render schema. **Status:** todo

## Tasks (in order)

| id   | title                                                                                        | est | depends_on       |
| ---- | ---------------------------------------------------------------------------------------------- | --- | ---------------- |
| t001 | Pin Render's environment object schema + list-filter semantics from the fresh spec            | 20m | —                |
| t002 | ACL triple on GraphQL `createEnvironment` + MCP `create_environment` via the REST path        | 45m | t001             |
| t003 | Honor the documented list filters or 400 (inherit w1/m42-t001's absent-`limit` convention)    | 45m | t001             |
| t004 | Field-parity fix per t001's evidence (REST `ownerId`/`createdAt` or record as bex extensions) | 25m | t001             |
| t005 | Render parity                                                                                  | 25m | t002, t003, t004 |
| t006 | Simplify                                                                                       | 20m | t005             |
| t007 | Test coverage                                                                                  | 35m | t005             |
| t008 | Closeout                                                                                       | 15m | t007             |

## Definition of done

Creating a protected/isolated/ip-gated environment succeeds in one call on REST, GraphQL, and MCP with identical semantics (cross-surface test); each filter Render documents on `GET /environments` (`name`/`environmentId`/`createdBefore|After`/`updatedBefore|After`/`ownerId`) either filters correctly or returns an honest 400, never a silent no-op (table test); the REST/GraphQL environment field sets match the pinned schema or the divergence is recorded in ADR018/ADR032.

## Source + Goal linkage

- **Source:** `/pm-brainstorm` round 14, 2026-07-15 — consistency mining on the newly shipped environments surface, verified in code: Render's `POST /environments` accepts `protectedStatus`/`networkIsolationEnabled`/`ipAllowList` (fresh OpenAPI) and bex REST does too (`lego/backend/internal/environments/rest.go:157-193`, w4/017), but GraphQL `createEnvironment` (`graphql.go:77-80`) and MCP `create_environment` (`mcp.go:37-40`) take only `name`+`projectId`; the list handler reads none of Render's documented filters (`rest.go:125-148` — the w7/m38 silently-ignored-filters class on a sibling route); GraphQL resolves `ownerId`/`createdAt` (`graphql.go:36-37`) that REST's `renderEnvironment` omits (`rest.go:51-64`).
- **Goal linkage:** Render parity (docs/ADR032-environments.md) + machine surfaces (pillar 3, docs/ADR008-vision.md) — agents predominantly use GraphQL/MCP, currently the crippled surfaces here.
- **Expected outcome:** environments behave like the rest of the API on every surface; no silently-ignored documented params.
- **Why now:** the surface shipped this week (w4/017, w5/m31) — same-week polish while the code is warm; w6's m31/m32 are exactly this chore shape.
- **Render parity:** included — REST/GraphQL/MCP surface change.
- **Coordinate with — never duplicate:** `w2/m43` (owns GraphQL/MCP list *paging*), `w1/m42/t001` (owns the absent-`limit` convention this inherits), `w4/m24` (ipAllowList descriptions), `w4/m28` (enforcement semantics — this milestone is wire-surface only).
