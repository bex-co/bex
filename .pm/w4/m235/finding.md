# Finding — a new service's URL answers Traefik's raw `404 page not found` until its first release is routed

Live functional `/qa-find-bugs w4`, pass f26a22, 2026-10-19 UTC, `muse.env`, workspace `bex-canary` (Hobby). Production was pinned to `db7e50ced338`. All fixtures are deleted (`DELETE` 204, then `GET` 404), and only this run's session is revoked.

- **Severity:** minor. The dashboard shows the new service's URL as a link from the moment it is created. For the first one to two minutes that link answers a bare `404 page not found`, which reads as the owner's own app returning 404 or the URL being wrong. bex already replaced raw infrastructure answers for the suspended (w2/m98), waking (w6/m94) and failed-first-release (w1/m179) states. This is the remaining public not-yet-serving state.

## Repro (as performed)

Each host was polled every 3 s from the moment `POST /v1/services` returned 201, logging every distinct status and body (`curl -s -D - --max-time 8`):

**Docker web service** `qa-20261019-f26a22-first` (`srv-dbat39t212nc73a1i7c0`, Free, repo `https://github.com/bex-co/bex`, root `examples/docker-build-env`), created 08:15:35Z:

| From | Response |
| --- | --- |
| 08:15:38 | `HTTP/2 404`, `content-type: text/plain; charset=utf-8`, body `404 page not found` (Traefik's default: no route exists), with no `Retry-After` or `Cache-Control` |
| 08:17:07 | `HTTP/2 503`, `text/plain`, body `service unavailable`. This is the activator's unrouted-host branch: the Ingress now points at the activator, whose host cache has not seen the App yet |
| 08:17:14 | `HTTP/2 200` `first` |

The same shape showed on the first fixture of the pass (`qa-20261019-f26a22-dbe`, polled every 10 s): `404 page not found` 08:08:12–08:09:05, `503 service unavailable` 08:09:16, `200` 08:09:26.

**Static site** `qa-20261019-f26a22-st` (`srv-dbat4kt212nc73a1i7e0`, root `examples/static-site`, `publishPath: .`), created 08:18:27Z:

| From | Response |
| --- | --- |
| 08:18:29 | `HTTP/2 404` `text/plain` `404 page not found` (Traefik) |
| 08:18:48 | `HTTP/2 404` `text/plain` `no static site for host` (static-server, before its resolver lists the new site) |
| 08:18:52 | `HTTP/2 200` `text/html` |

TLS was valid throughout (curl negotiated without `-k`).

## Root cause

- **Web:** while a first build runs there is no prior release, and `buildFromSource` halts the reconcile at the build observation (`lego/operator/internal/controller/app_controller.go:1171-1173`). That is before `reconcileKubernetes` applies the ClusterIP Service and the Ingress (`:2553-2580`), so the public host has no route and Traefik answers its default 404.
- **Web handover:** once the Deployment exists but has no ready pod, `ingressBackend` routes a sleep-eligible App to the activator (`:2893-2939`). The activator's host cache misses it and answers the bare `http.Error(w, "service unavailable", 503)` (`lego/operator/cmd/activator/main.go:142-146`).
- **Static:** `reconcileStaticSite` halts while the first publish Job runs (`app_controller.go:3866-3870`, via `publishStaticRevision` at `:3947`). That is before `reconcileStaticIngress` (`:4011`), and then the static-server's resolver answers `no static site for host` (`lego/operator/internal/staticserver/staticserver.go:229`) until it refreshes.
- **Precedents in the same activator:** a suspended App gets `writeSuspendedResponse` (`cmd/activator/main.go:247`: content-negotiated 503 with `Retry-After`). A failed or canceled first release answers 503 deliberately without `Retry-After` (`:170-178`, w1/m179).

## Fix (target behavior)

From the first reconcile of a publicly routable web service or static site until its first release serves, its host answers bex's own content-negotiated response over its managed certificate. That response is a 503 with `Retry-After` saying the first deploy is in progress, in the same shape as the suspended response. It is never Traefik's `404 page not found`, never the static-server's `no static site for host`, and never the bare unrouted `service unavailable`.

Unchanged:

- w1/m179's failed or canceled first-release 503 (no `Retry-After`).
- The suspended, maintenance and sleep branches.
- Private services (not public).
- Time to the first 200.

## Unverified

- Paid web services. They are not sleep-eligible, so `ingressBackend` would route them to their own endpoint-less Service, and Traefik would presumably answer `503 no available server` after the 404 phase (code-read).
- Image-backed services: only a short pre-deploy window, not probed.
- Custom domains attached before the first deploy.
- Render's own pre-first-deploy response (t001 captures it).

## Dedupe

- No board item covers the pre-first-release window. The `404 page not found` mentions in `w4/done/*` are post-delete probes.
- w2/m98 (suspended), w6/m94 (wake race), w1/m179 and w1/132 (failed/canceled first release) and w3/m46 (suspended static TLS) each closed a neighbouring state.
- Nothing in `.pm/DO_NOT_DO.md` matches.
