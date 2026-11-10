# w3 · m273 — Private services discover runtime ports and web services answer on private port 10000, as on Render

**Worker:** worker3 **Goal:** an undeclared private-service listener is reachable on the private network, and every web service answers on private port 10000 **Status:** todo — t001–t004, t006–t008, t010 done; t005 live check pending (gate cleared 2026-11-10)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | ADR089 D3 phase 2 design: private-service port discovery and the 10000 alias — **DONE** | 1h | — |
| t002 | Extend the port scanner to private services (image and native) and publish openPorts — **DONE** | 3h | t001 |
| t003 | Port-10000 alias on web-service ClusterIP Services — **DONE** | 1.5h | t001 |
| t004 | Republish and pin revision when the discovered set changes — **DONE** | 1.5h | t002 |
| t010 | A private image service bound only to an undeclared port never becomes Ready, so discovery never runs — **DONE** | 50m | t004 |
| t005 | Hosted acceptance on dev-3 or the canary | 2h | t003, t004, t010 |
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

## Unblocked (2026-11-05 17:35Z, live `/qa-find-bugs w4` round 87)

- **Gate cleared:** production pin `cc3c51e12952` (`e3211740a`) contains `c6bcb8741` (this milestone) and `dd31fbdef` (m276). Checked with `git merge-base --is-ancestor`. Moved `w3/blocked/m273/` → `w3/m273/`.
- **Live result:** the DoD's first bullet **fails**.
  - A private `docker.io/traefik/whoami:v1.10` service run with `/whoami --port 8080` and `PORT=8080` (`srv-dbmblckrv9ic73d23e70`) logged `Starting up on port 8080`.
  - It stayed `update_in_progress` for more than 12 minutes on a TCP probe of the image-declared port 80, with `openPorts: [80]`.
  - The same spec as a web service went Live on 8080 in ~30s and returned 200.
  - Cause and fix: **t010** (readiness gates discovery; `PORT` discarded for image-port private services).
- **Observation for m276 (not concluded):** that web service's `openPorts` still read `[80, 8080]` at 17:34:09Z, about 4 minutes after Live and past the 2-minute scan interval; nothing binds 80. Whether a scan ran was not observable from outside. Recheck in m276's live check.
- **Next:** t010, then t005 with the failing case above plus the original undeclared-port and `:10000` checks.

## Blocked (2026-11-08, loopx w3)

- t010 shipped (PORT selects a private image service's primary; an undeclared listener on an unready first rollout becomes the primary). Its live rerun is part of t005.
- **Gate:** a `chore(deploy): pin platform images to <sha>` commit on main at or after t010's commit. Then a worker runs t005 with the QA account (`scripts/qa-login.sh`): the t010 whoami case with and without `PORT`, an undeclared-port private service reached from a peer, and a web service at `:10000`; then t009 Closeout. Run it together with `blocked/m276`'s live check. No user decision needed.

## Unblocked (2026-11-10, /pmx w3)

- **Source:** verified evidence (the release condition recorded on 2026-11-08), not a user decision.
- **Gate cleared:** main's latest `chore(deploy): pin platform images to 4630a48a765e` (`043519226`, deploy.yml run 42777932436, success) contains `c6bcb8741` (this milestone), `1aad33901` (t010) and `dd31fbdef` (m276), each checked with `git merge-base --is-ancestor`. Moved `w3/blocked/m273/` → `w3/m273/`.
- **Status:** a deployed version is not proof the DoD passed; t005 is pending live verification.
- **Next:** t005 with the QA account (`scripts/qa-login.sh`), together with m276's live check: the t010 whoami case with and without `PORT`, an undeclared-port private service reached from a peer, and a web service at `:10000`. Then t009 Closeout. Delete fixtures by id with 404 proofs.
