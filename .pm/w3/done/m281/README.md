# w3 · m281 — Protected environments restrict creating and moving resources to Admins, as on Render

**Worker:** worker3 **Goal:** in a `protectedStatus=protected` environment, only workspace Admins (and Owners) can create resources or move resources in or out, on every surface, as Render documents **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Inventory every resource placement path and its current gate — **DONE** | 30m | — |
| t002 | Creating a resource in a protected environment requires a fresh Admin check — **DONE** | 45m | t001 |
| t003 | Moving a resource into or out of a protected environment requires a fresh Admin check — **DONE** | 45m | t001 |
| t004 | Blueprint sync and PITR recovery honor the protected placement gate — **DONE** | 30m | t002 |
| t005 | Dashboard disables protected create and move controls for non-Admins — **DONE** | 40m | t002, t003 |
| t006 | Amend ADR032 and ADR018 row 225 — **DONE** | 20m | t005 |
| t007 | Render parity — **DONE** | 30m | t006 |
| t008 | Simplify — **DONE** | 20m | t007 |
| t009 | Test coverage — **DONE** | 40m | t007 |
| t010 | Closeout — **DONE** | 10m | t009 |

## Definition of done

- A workspace Member gets 403 on REST, GraphQL and MCP when they create a Service, Postgres, Key Value or environment group in a protected environment, move one into or out of a protected environment, or Blueprint-sync or recover into one. An Admin succeeds at each.
- Behavior in unprotected environments and with no environment is unchanged.
- The dashboard shows those actions disabled, with a reason, for non-Admins.
- ADR032 lists create and move as Admin-only. ADR018 row 225 no longer records the placement divergence.

## Source + Goal linkage

- **Source:** `/pm-brainstorm w3` 2026-11-07, item 1, code-verified at `16b4a0d20`.
  - Render's [projects docs](https://render.com/docs/projects) restrict create, move and delete in a protected environment to Admins.
  - `core.ProtectedVerbRestricted` (`lego/backend/internal/core/confirmation.go:92`) omits create and move.
  - `ResolveEnvironmentForCreate` callers have no protected check.
  - ADR018 row 225 records the divergence without user sign-off.
- **Goal linkage:** Render parity first ([ADR008](../../../docs/ADR008-vision.md#render-parity-first)). Protected environments ([ADR032](../../../docs/ADR032-environments.md)).
- **Expected outcome:** a Member can no longer place a resource into a protected environment, or take one out of it, which Render forbids.
- **Why now:** this is an authorization-boundary gap that is documented but has no owner. Every other protected verb was aligned in w8/187 and w8/193, so placement is the last one.
- **Render parity:** included. The change touches REST, GraphQL, MCP and dashboard authorization.

## Closed — 2026-11-08

In a protected environment only workspace Admins create resources or move them in or out, on REST, GraphQL, MCP, Blueprint sync and recovery, with a coded 403; the dashboard disables those targets for non-Admins. Follow-up `w3/059`.
