# w6 · m33 — Environments surface-parity chores: create ACL + list filters + field drift

**Worker:** worker6 **Goal:** The just-shipped environments surface stops being REST-first: creating a protected/isolated/ip-gated environment is one call on every surface, the documented list filters filter or 400 (never silently no-op), and the REST/GraphQL field sets match the pinned Render schema. **Status:** done

## Tasks (in order)

| id   | title                                                                                        | est | depends_on       |
| ---- | ---------------------------------------------------------------------------------------------- | --- | ---------------- |
| t001 | Pin Render's environment object schema + list-filter semantics from the fresh spec — **DONE**            | 20m | —                |
| t002 | ACL triple on GraphQL `createEnvironment` + MCP `create_environment` via the REST path — **DONE**        | 45m | t001             |
| t003 | Honor the documented list filters or 400 (inherit w1/m42-t001's absent-`limit` convention) — **DONE**    | 45m | t001             |
| t004 | Field-parity fix per t001's evidence (REST `ownerId`/`createdAt` or record as bex extensions) — **DONE** | 25m | t001             |
| t005 | Render parity — **DONE**                                                                                  | 25m | t002, t003, t004 |
| t006 | Simplify — **DONE**                                                                                       | 20m | t005             |
| t007 | Test coverage — **DONE**                                                                                  | 35m | t005             |
| t008 | Closeout — **DONE**                                                                                       | 15m | t007             |

## Definition of done

Creating a protected/isolated/ip-gated environment succeeds in one call on REST, GraphQL, and MCP with identical semantics (cross-surface test); each filter Render documents on `GET /environments` (`name`/`environmentId`/`createdBefore|After`/`updatedBefore|After`/`ownerId`) either filters correctly or returns an honest 400, never a silent no-op (table test); the REST/GraphQL environment field sets match the pinned schema or the divergence is recorded in ADR018/ADR032.

## Source + Goal linkage

- **Source:** `/pm-brainstorm` round 14, 2026-07-15 — consistency mining on the newly shipped environments surface, verified in code: Render's `POST /environments` accepts `protectedStatus`/`networkIsolationEnabled`/`ipAllowList` (fresh OpenAPI) and bex REST does too (`lego/backend/internal/environments/rest.go:157-193`, w4/017), but GraphQL `createEnvironment` (`graphql.go:77-80`) and MCP `create_environment` (`mcp.go:37-40`) take only `name`+`projectId`; the list handler reads none of Render's documented filters (`rest.go:125-148` — the w7/m38 silently-ignored-filters class on a sibling route); GraphQL resolves `ownerId`/`createdAt` (`graphql.go:36-37`) that REST's `renderEnvironment` omits (`rest.go:51-64`).
- **Goal linkage:** Render parity (docs/ADR032-environments.md) + machine surfaces (pillar 3, docs/ADR008-vision.md) — agents predominantly use GraphQL/MCP, currently the crippled surfaces here.
- **Expected outcome:** environments behave like the rest of the API on every surface; no silently-ignored documented params.
- **Why now:** the surface shipped this week (w4/017, w5/m31) — same-week polish while the code is warm; w6's m31/m32 are exactly this chore shape.
- **Render parity:** included — REST/GraphQL/MCP surface change.
- **Coordinate with — never duplicate:** `w2/m43` (owns GraphQL/MCP list *paging*), `w1/m42/t001` (owns the absent-`limit` convention this inherits), `w4/m24` (ipAllowList descriptions), `w4/m28` (enforcement semantics — this milestone is wire-surface only).

## Closeout evidence

- Render's public OpenAPI was captured on 2026-07-15. It confirms the ACL create triple, exact Environment field set, list envelope, filters, and default-20 paging convention; the evidence and bex feasibility decisions are pinned in `docs/render-artifacts/protected-environments.md`.
- `CreateWithACL` is the shared validation/create/apply path used by REST, GraphQL, and MCP. Cross-surface success and invalid-CIDR/no-orphan tests cover all three adapters.
- REST honors `name`, `projectId`, `createdBefore`/`createdAfter`, `ownerId`, and `environmentId`; `updatedBefore`/`updatedAfter` return named 400s because there is no `updatedAt`. Paging remains owned by `w2/m43`.
- Render REST intentionally omits `ownerId`/`createdAt`; ADR018 and ADR032 identify those GraphQL fields as bex extensions. The existing dashboard create query remains source-compatible and `yarn typecheck` passes.
- `cd lego/backend && go build ./... && go test ./...` passed; `make lint-backend` passed with 0 issues. The simplify pass consolidated creation and ACL application behind shared service-layer paths.
