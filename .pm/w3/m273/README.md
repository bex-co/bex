# w3 · m273 — Private services discover runtime ports and web services answer on private port 10000, as on Render

**Worker:** worker3 **Goal:** an undeclared private-service listener is reachable on the private network, and every web service answers on private port 10000 **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | ADR089 D3 phase 2 design: private-service port discovery and the 10000 alias | 1h | — |
| t002 | Extend the port scanner to private services (image and native) and publish openPorts | 3h | t001 |
| t003 | Port-10000 alias on web-service ClusterIP Services | 1.5h | t001 |
| t004 | Republish and pin revision when the discovered set changes | 1.5h | t002 |
| t005 | Hosted acceptance on dev-3 or the canary | 2h | t003, t004 |
| t006 | Render parity | 30m | t005 |
| t007 | Simplify | 20m | t006 |
| t008 | Test coverage | 40m | t006 |
| t009 | Closeout | 10m | t007, t008 |

## Definition of done

- A private service (image or native) listening on a port its image does not declare is reachable from the private network and listed in `openPorts`
- Every web service answers on private port 10000 with its primary HTTP server
- `openPorts` agrees on REST, GraphQL, MCP and the dashboard Service Address; ADR018 row 107 reads ✅

## Source + Goal linkage

- **Source:** `/pm-brainstorm w3` 2026-11-05 (Render-parity-filtered round, item 4); code-verified at `158cb45d0`; ADR018 row 107 (◐)
- **Goal linkage:** Render parity first (ADR008); "bring your container" compatibility (ADR089)
- **Expected outcome:** multi-service apps written for Render's private network connect on bex without declaring ports
- **Why now:** this is the remaining ADR089 gap; Render-style apps silently fail to connect privately today
- **Render parity:** included — a feature touching `openPorts` on REST/GraphQL/MCP and the dashboard
