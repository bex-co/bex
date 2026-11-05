# w3 · m276 — Web services' extra private listeners are discovered, and openPorts lists only ports that listen, as on Render

**Worker:** worker3 **Goal:** every service's private-network listeners are reachable and reported the way Render's are: a web service's extra listeners too, and never a declared port nothing listens on **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Scan web services for listeners beyond the primary | 2h | — |
| t002 | openPorts lists only ports that listen | 1h | t001 |
| t003 | ADR089 D3 phase 2 and ADR018 row 107 | 30m | t002 |
| t004 | Render parity | 30m | t003 |
| t005 | Simplify | 20m | t004 |
| t006 | Test coverage | 40m | t004 |
| t007 | Closeout | 10m | t005, t006 |

## Definition of done

- A web service's extra private TCP listener is discovered, published and listed in openPorts
- After a scan, openPorts lists only listening ports; before one, the declared ports
- REST/GraphQL/MCP/dashboard agree (one App.OpenPorts)

## Source + Goal linkage

- **Source:** promotes the TCP half of `w3/052` (residual of w3/m273), 2026-11-05 loopx w3; UDP/any-protocol split to `w3/054`
- **Goal linkage:** Render parity first (ADR008); bring-your-container compatibility (ADR089)
- **Expected outcome:** Render-style multi-port apps reach every listener on bex's private network
- **Why now:** w3/m273 shipped discovery for private services only; these are the remaining TCP gaps it recorded
- **Render parity:** included — operator behavior surfaced through openPorts on every surface
