# w3 · m273 — Private services discover runtime ports and web services answer on private port 10000, as on Render

**Worker:** worker3 **Goal:** an undeclared private-service listener is reachable on the private network, and every web service answers on private port 10000 **Status:** blocked — t001–t004, t006–t008 done; t005 hosted acceptance waits on this commit's production deploy

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | ADR089 D3 phase 2 design: private-service port discovery and the 10000 alias — **DONE** | 1h | — |
| t002 | Extend the port scanner to private services (image and native) and publish openPorts — **DONE** | 3h | t001 |
| t003 | Port-10000 alias on web-service ClusterIP Services — **DONE** | 1.5h | t001 |
| t004 | Republish and pin revision when the discovered set changes — **DONE** | 1.5h | t002 |
| t005 | Hosted acceptance on dev-3 or the canary | 2h | t003, t004 |
| t006 | Render parity — **DONE** | 30m | t005 |
| t007 | Simplify — **DONE** | 20m | t006 |
| t008 | Test coverage — **DONE** | 40m | t006 |
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

## Blocked (2026-11-05, loopx w3)

- **Gate:** t005 needs an operator built from this milestone's commit. dev-N shares the cluster's one bex operator, so the check cannot run before the commit deploys; replacing the shared operator is off-limits.
- **Release condition:** the commit is live in production (deploy.yml + Argo). Then, with the QA account (`scripts/qa-login.sh`), deploy a private image service listening on a port its image does not declare and a web service, and from a third service reach the private one on that port and the web one at `:10000`; record the result in t005, flip ADR018 row 107 to ✅, and close.
- No user decision needed.

- **2026-11-05 14:20Z attempt (loopx w3):** ran the live check in `bex-canary` with the QA account (`m273-priv` with python's http.server on an undeclared 9001, an nginx `m273-web`, and a curl `m273-probe` worker). `priv:9001` and `web:10000` did not answer, and `openPorts` showed only 10000. **Inconclusive:** production is still pinned to `c8bbc07cbeeb`, from before this code. Every deploy run since then was marked superseded (`check-supersession`, newer production manifests on main), so build and deploy were skipped. This is w6/m156's deploy-latency area. The three services were deleted (204) and the QA session logged out. Rerun once a deploy pins a commit at or after this one.
