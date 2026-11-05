# w3 · m276 — Web services' extra private listeners are discovered, and openPorts lists only ports that listen, as on Render

**Worker:** worker3 **Goal:** every service's private-network listeners are reachable and reported the way Render's are: a web service's extra listeners too, and never a declared port nothing listens on **Status:** blocked — t001–t006 done; t007 Closeout waits on the production deploy and a live check

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Scan web services for listeners beyond the primary — **DONE** | 2h | — |
| t002 | openPorts lists only ports that listen — **DONE** | 1h | t001 |
| t003 | ADR089 D3 phase 2 and ADR018 row 107 — **DONE** | 30m | t002 |
| t004 | Render parity — **DONE** | 30m | t003 |
| t005 | Simplify — **DONE** | 20m | t004 |
| t006 | Test coverage — **DONE** | 40m | t004 |
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

## Blocked (2026-11-05, loopx w3)

- **Gate:** the definition of done is live behavior of the operator, which dev-N cannot run from a branch (one shared operator). Once this commit is in production, run the check with the QA account (`scripts/qa-login.sh`). A web service listening on a second port is reachable on it from a sibling and lists it in `openPorts`. An image-declared port with no listener drops from `openPorts` after the first scan. Do it together with `w3/m273` t005, then close both.
- No user decision needed.

- **2026-11-05 14:20Z attempt (loopx w3):** ran the live check in `bex-canary` with the QA account (`m273-priv` with python's http.server on an undeclared 9001, an nginx `m273-web`, and a curl `m273-probe` worker). `priv:9001` and `web:10000` did not answer, and `openPorts` showed only 10000. **Inconclusive:** production is still pinned to `c8bbc07cbeeb`, from before this code. Every deploy run since then was marked superseded (`check-supersession`, newer production manifests on main), so build and deploy were skipped. This is w6/m156's deploy-latency area. The three services were deleted (204) and the QA session logged out. Rerun once a deploy pins a commit at or after this one.
