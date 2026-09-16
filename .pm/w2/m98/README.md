# w2 · m98 — A suspended web service answers with a bex response

**Worker:** worker2 **Goal:** a suspended web service's public host answers with a bex "this service is suspended" response, content-negotiated for browsers and API clients, instead of Traefik's raw `503 no available server`. The URL and certificate stay the same, the App stays at zero replicas, and nothing about the response wakes it. Resume restores the App's own backend. **Status:** todo

## Tasks (in order)

| id   | title                                                                                                                                                   | est | depends_on |
| ---- | ------------------------------------------------------------------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | `ingressBackend` routes a suspended web service to the activator alias, ordered maintenance → suspended → sleep → own Service                             | 30m | —          |
| t002 | The activator serves a content-negotiated 503 "This service is suspended" with `Retry-After` and never scales or touches `last-active` for a suspended App | 30m | —          |
| t003 | Capture Render's actual suspended response and record the decision in the ADR018 Suspend/Resume row                                                     | 15m | t001, t002 |
| t004 | Render parity                                                                                                                                           | 10m | t003       |
| t005 | Simplify                                                                                                                                                | 15m | t004       |
| t006 | Test coverage                                                                                                                                           | 40m | t004       |
| t007 | Closeout                                                                                                                                                | 10m | t006       |

## Definition of done

Run on production, workspace `bex`, with a throwaway free web service (`examples/hello-go`) created and deleted inside the run:

1. `POST /v1/services/<srv>/suspend` → `202`; `GET /v1/services/<srv>` reads `phase=Hibernated suspended=suspended`.
2. `curl -i -H 'Accept: text/html,application/xhtml+xml' https://<slug>.onbex.co/` → `HTTP/2 503`, `content-type: text/html`, a bex page saying the service is suspended, a `Retry-After` header.
3. `curl -i -H 'Accept: application/json' …` → `503`, `content-type: application/json`, body `{"error":"service suspended"}`.
4. `curl -i …` with no `Accept` → `503` with a bex plain-text body, not `no available server`.
5. The TLS certificate is unchanged (issuer Let's Encrypt, subject CN = the slug host).
6. After the three requests the App's Deployment still has zero replicas, `app.bex.co/last-active` has not advanced, and the Events feed shows no `service_woken` after `service_suspended`.
7. `POST …/resume` → the service returns to `Running` and `curl https://<slug>.onbex.co/` → `200` from the App's own backend.
8. A suspended static site (control) still serves the static-server response from `w3/m46`; a sleeping free service (control) still serves the activator wake interstitial from `w6/m94`.
9. `DELETE /v1/services/<srv>` → `204`, then `GET` → `404`.

## Source + Goal linkage

- **Source:** `w1/094` (live `/qa-find-bugs` 2026-09-14 pass 18; absorbed into this milestone, note retired to `.pm/w1/done/094.md`), proposed by `/pm-brainstorm for w1` 2026-09-15 as candidate #5.
- **Goal linkage:** pillar 2 (truthful, machine-readable state): a deliberate suspend must not look like a platform outage to a browser, an uptime monitor, or a webhook sender. ADR018 row 73 (Suspend / Resume) claims parity on every surface without deciding what the public host serves while suspended.
- **Expected outcome:** every visitor to a suspended web service gets a bex 503 in the format they asked for; `w1/094` pass 13 saw a webhook delivery to a suspended receiver record `responseBody "no available server"`, which becomes a recognisable `{"error":"service suspended"}`.
- **Why now:** suspended static sites (`w3/m46`) and sleeping free services (`w6/m94`) already got real responses; compute suspend is the last raw Traefik text on a public host. The activator already owns content-negotiated pages and a no-wake maintenance branch, so this is routing plus one responder, not a new component.
- **Render parity included** because the public host's response is tenant-facing and ADR018's Suspend/Resume row records no suspended-response decision; t003 captures Render's actual behavior rather than assuming it.
