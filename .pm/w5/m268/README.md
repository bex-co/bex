# w5 · m268 — Key Value "Enable Internal Authentication", as on Render

**Worker:** worker5 **Goal:** a customer can require authentication on a Key Value store's internal (private-network) connections, as Render's Info → Connections button does. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Verify Render's internal-auth behavior | 30m | — |
| t002 | CRD field and operator enforcement on the internal front | 1h30m | t001 |
| t003 | Core verb on REST, GraphQL and MCP; audit event; connection info | 1h30m | t002 |
| t004 | Dashboard Connections: Enable Internal Authentication | 45m | t003 |
| t005 | Live walk: before and after enabling | 45m | t004 |
| t006 | Render parity | 30m | t005 |
| t007 | Simplify | 20m | t006 |
| t008 | Test coverage | 45m | t006, t007 |
| t009 | Closeout | 10m | t008 |

Total: **9 tasks, about 6h45m**.

## Definition of done

- With internal auth enabled (on any surface), an unauthenticated client on the store's internal address is refused and a client using the password works; external connections are unchanged.
- The audit log records `EnableRedisInternalAuthEvent`; connection info reflects the mode; the dashboard Connections section has the button.
- Reversibility matches Render's verified behavior.

## Source + Goal linkage

- **Source:** `/pm-brainstorm w5` 2026-11-05 item 4; `.pm/w8/done/204.md:51` (toggle deferred as a follow-up); render.com/docs/key-value.
- **Goal linkage:** ADR021 managed Key Value; ADR043 tenant isolation; ADR008 parity first.
- **Expected outcome:** customers who need authenticated private-network Key Value traffic can have it.
- **Why now:** w8/204 changed every store in every workspace to auth-free internal access; customers with compliance needs currently have no way back.
- **Render parity:** included.
