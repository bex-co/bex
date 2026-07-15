# w2 · m43 — Machine-surface list consistency: GraphQL/MCP pagination + service identity drift (verify-first)

**Worker:** worker2 **Goal:** The cross-surface drift w8/m13 closed for datastores is closed for services and environments too: GraphQL list queries page like their REST siblings, the GraphQL `Service.id`-returns-the-App-name drift is resolved (aligned to `srv-…` or recorded as an ADR020 known deviation), GraphQL `Service` gains `updatedAt`, and MCP arg descriptions stop telling agents to pass names as ids. **Status:** todo

## Tasks (in order)

| id   | title                                                                                                       | est | depends_on |
| ---- | ------------------------------------------------------------------------------------------------------------ | --- | ---------- |
| t001 | Add `cursor`/`limit` to GraphQL `services` + `environments` (and MCP `list_environments`); record whether MCP `list_services` arg-free is by-design | 60m | —          |
| t002 | Audit dashboard usage of GraphQL `Service.id`; align to `srv-…` or record ADR020 deviation; add `updatedAt`  | 60m | —          |
| t003 | Fix the stale "bex App name" MCP arg descriptions                                                            | 20m | —          |
| t004 | Render parity                                                                                                 | 25m | t001, t002, t003 |
| t005 | Simplify                                                                                                      | 20m | t004       |
| t006 | Test coverage                                                                                                 | 30m | t004       |
| t007 | Closeout                                                                                                      | 15m | t006       |

## Definition of done

GraphQL `services` and `environments` accept `cursor`/`limit` with the same semantics as their REST routes (walk test); the MCP `list_services` decision (paginate vs Render-MCP-mirror) is recorded with evidence; the `Service.id` decision is made and shipped — either GraphQL returns the minted `srv-…` id (dashboard audited, nothing breaks) or docs/ADR020-identifiers.md lists it under Known deviations; GraphQL `Service` exposes `updatedAt`; no MCP tool description says "bex App name" for a `serviceId` arg.

## Source + Goal linkage

- **Source:** `/pm-brainstorm` round 13, 2026-07-15 — mechanical-consistency mining, all verified in code: GraphQL `services` takes only `ownerId` (`lego/backend/internal/apps/graphql.go:688-695`) and MCP `list_services` only `OwnerID` (`apps/mcp.go:47-51`) while REST paginates (`apps/rest.go:549-551`); environments GraphQL/MCP likewise unpaged (`environments/graphql.go:51-59`, `environments/mcp.go:29-30,81-83` — bex-extension surfaces, so no Render-MCP constraint); GraphQL `Service.id` returns the App name (`apps/graphql.go:254`) vs REST/MCP's minted `srv-…` (`apps/render.go:165-168`) — unlisted in ADR020's Known deviations; GraphQL `Service` lacks `updatedAt` (REST has it, `render.go:46`); ~10 MCP arg structs still describe `serviceId` as "the service id (bex App name)" (`apps/mcp.go:39` et al.).
- **Goal linkage:** pillar 3 (AI-native machine surfaces, docs/ADR008-vision.md) — agents consume GraphQL/MCP; identity drift breaks cross-surface joins and stale descriptions actively mislead them.
- **Expected outcome:** an agent can join `list_services`/GraphQL/REST results by id and page any list the same way on every surface.
- **Why now:** commit 89c42936 established the datastore pattern this week; services/environments are the leftover half of the same sweep.
- **Why verify-first:** MCP `list_services` arg-free may be deliberate Render-MCP mirroring (the ledger records exactly that rationale for `list_postgres_instances`), and flipping GraphQL `id` could break dashboard routing — "document as known deviation" is an acceptable t002 outcome.
- **Render parity:** included — GraphQL/MCP surface changes.
