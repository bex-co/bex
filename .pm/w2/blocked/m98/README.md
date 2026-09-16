# w2 · m98 — A suspended web service answers with a bex response

**Worker:** worker2 **Goal:** a suspended web service's public host answers with a bex "this service is suspended" response, content-negotiated for browsers and API clients, instead of Traefik's raw `503 no available server`. The URL and certificate stay the same, the App stays at zero replicas, and nothing about the response wakes it. Resume restores the App's own backend. **Status:** t001–t006 done; t007 blocked on live production verification.

## Tasks (in order)

| id | title | est | depends_on | status |
| --- | --- | --- | --- | --- |
| t001 | `ingressBackend` routes a suspended web service to the activator alias, ordered maintenance → suspended → sleep → own Service | 30m | — | — **DONE** |
| t002 | The activator serves a content-negotiated 503 "This service is suspended" with `Retry-After` and never scales or touches `last-active` for a suspended App | 30m | — | — **DONE** |
| t003 | Capture Render's actual suspended response and record the decision in the ADR018 Suspend/Resume row | 15m | t001, t002 | — **DONE** |
| t004 | Render parity | 10m | t003 | — **DONE** |
| t005 | Simplify | 15m | t004 | — **DONE** |
| t006 | Test coverage | 40m | t004 | — **DONE** |
| t007 | Closeout | 10m | t006 | blocked on live production verification |

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

## Implementation record (t001–t006, 2026-09-15)

**What shipped.** `ingressBackend` (`lego/operator/internal/controller/app_controller.go`) now resolves the public backend in a fixed precedence — **maintenance → suspended → sleep → own Service** — via a new `suspendedRoutable` predicate (suspended **and** a web service **and** an activator is configured). The activator (`lego/operator/cmd/activator/main.go`) gained a suspended branch that returns immediately after the maintenance branch and **before** `wakeApp`, answering `503` + `Retry-After: 3600` with an HTML page / `{"error":"service suspended"}` / `This service is suspended.` by `Accept`. The suspended branch holds no Kubernetes client call at all, so it cannot patch replicas or write `app.bex.co/last-active` — the separation `.pm/DO_NOT_DO.md` requires between the suspend path and free-tier sleep/wake.

**t003 — where the Render capture came from.** Render's public community record, checked **2026-09-15**: the thread "Getting 503 'This service has been suspended by its owner.'…" (`community.render.com/t/…/450`, now redirecting to `render.discourse.group`) with a Render staff reply confirming the 503 is emitted by Render's **proxy layer**, never reaching the tenant container, plus Render's API reference `api-docs.render.com/reference/suspend-service-1`. `render.com/docs/service-actions` and `render.com/docs/suspending-a-service` both still return **404**, exactly as `w1/094` found.

**Outstanding — not verified (run-level gate, not a defect in this work):**

- [ ] **Render's suspended response was not captured live.** This run had **no working Render credential** and no working production credential (both stale). Render's exact `content-type`, full header set, `Retry-After` presence, and whether the body varies by `Accept` are therefore **unknown**; only the status code (`503`) and the message text (`This service has been suspended by its owner.`) are established, and those from Render's own public community record rather than a `curl`. The ADR018 row states this limitation explicitly and marks each divergence as deliberate. Re-probe when a Render account is available.
- [ ] **The Definition-of-done steps 1–9 above have not been re-probed live.** They require a deployed operator + activator on production; that is t007's job and it stays open.

**t004 — Render parity check (static, from the diff).**

- **Wire shapes: unchanged.** The m98 diff is **operator-only plus `docs/ADR018-render-parity.md`** — `lego/operator/internal/controller/app_controller.go`, `lego/operator/cmd/activator/main.go`, and their tests. Nothing under `lego/backend/` (REST/GraphQL/MCP), `dashboard/`, or `lego/types/` is touched, so `POST …/suspend` / `…/resume`, GraphQL `suspendService`/`resumeService`, and MCP `suspend_service`/`resume_service` return byte-identically to before.
- **Surfaces agree, by construction.** All four surfaces read the same `App.spec.suspended`: REST/GraphQL/MCP project it as `suspended: "suspended"`, the dashboard header renders from that same field, and the public host's response is now derived from it too (operator routing + activator branch). There is no second source of truth to drift, and no new field was added. The *live* four-surface agreement after a suspend/resume round trip is part of t007's Definition-of-done re-probe.
- **Residual divergence:** the three deliberate divergences from Render (three-way negotiation, `Retry-After: 3600`, wording) are recorded in the ADR018 row rather than filed as notes — they are the intended behavior, not drift. Nothing else was found to file.

**t005 — simplify.** Applied, behavior-preserving: `acceptsHTML` is now a one-line wrapper over a shared `acceptsMedia(values, want)` (the wake path's exact semantics kept, including the explicit-`q=0` refusal and the wildcard fall-through); a new `writeBody` is `writeHTMLPage`'s non-document sibling, and `writeWakeResponse`'s JSON arm was folded onto it so all four responders (maintenance, suspended HTML/JSON/text, wake) share one no-store + HEAD-aware write shape; the `type == "" || type == web_service` equality that `autoSleepEligible` open-coded became `isWebService`, so the sleep and suspended branches cannot disagree about an untyped App.

**t006 — tests added.** Operator (`lego/operator/internal/controller/suspended_route_test.go`): `TestSuspendedWebServiceRoutesToActivator`, `TestResumeReturnsSuspendedRouteToOwnService`, `TestMaintenanceWinsOverSuspendedRouting`, `TestSuspendedPrivateServiceHasNoIngress`, `TestSuspendedRoutingWithoutActivatorKeepsOwnService`, `TestSleepingFreeServiceStillRoutesToActivator`. Activator (`lego/operator/cmd/activator/main_test.go`): `TestSuspendedHandlerNegotiatesContentAndNeverWakes` (4 cases, asserting **zero** patches through a counting fake client), `TestSuspendedHTMLHeadHasNoBody`, `TestMaintenanceWinsOverSuspended`. The suspended-path tests were confirmed **failing on the pre-fix code** by disabling both new branches. The suspended **static site** control is already covered by the existing `w3/m46` envtest in `app_controller_staticsite_test.go` (suspended site keeps the static-server alias + TLS), so it was not duplicated.
