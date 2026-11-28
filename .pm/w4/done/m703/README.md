# w4 · m703 — Every answer the edge sends carries `cf-cache-status`, as Render's does: the HTTP-to-HTTPS redirect, the unknown host's 404, and a static site's answers

**Worker:** worker4 **Goal:** a site moved from Render reads the same `cf-cache-status` on bex as on Render for every answer its host sends, and a redirect or unknown-host answer carries no `rndr-id`, as Render's do not. Since w4/m687 only a web service's own answers read `cf-cache-status: DYNAMIC`; every other answer has none. **Status:** done (all 9 tasks; verified on production pin `7d1dce861173` on 2026-11-28)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | The HTTP-to-HTTPS redirect answers `cf-cache-status` and no `rndr-id` — **DONE** | 1h15m | — |
| [t002](done/t002.md) | The unknown host's `404` reads `cf-cache-status: DYNAMIC` and no `rndr-id` — **DONE** | 45m | t001 |
| [t003](done/t003.md) | Capture Render's static-site statuses for the answers not yet captured — **DONE** | 30m | — |
| [t004](done/t004.md) | The static server answers a `CF-Cache-Status` on every answer — **DONE** | 1h30m | t003 |
| [t005](done/t005.md) | Amend ADR018, ADR022 and ADR029, then verify on production — **DONE** | 45m | t001, t002, t004 |
| [t006](done/t006.md) | Render parity — **DONE** | 20m | t005 |
| [t007](done/t007.md) | Simplify — **DONE** | 15m | t006 |
| [t008](done/t008.md) | Test coverage — **DONE** | 30m | t006 |
| [t009](done/t009.md) | Closeout — **DONE** | 10m | t007, t008 |

Total: **9 tasks, about 6h**.

## Definition of done

1. `curl -I http://<web service host>/` answers `301` with `cf-cache-status: DYNAMIC`, `server: cloudflare`, a `cf-ray` and no `rndr-id`; a `POST` over `http://` answers `307` with the same. A static site's `http://` redirect answers the status t001 records for it, and its `POST` `307` reads `DYNAMIC`.
2. `http://` and `https://` to an unknown `*.onbex.co` host answer `cf-cache-status: DYNAMIC` and no `rndr-id`, as `no-such-service.onrender.com` does.
3. A static site's `GET /` reads a `cf-cache-status` on every answer, with the mapping t004 records: a status for an object read through the static server's cache and one for an object fetched from the origin, `HEAD` as `GET`, and `BYPASS` for a missing file's `404` and a route redirect, as on Render. A static site's header rule cannot change it.
4. Plugin tests (`deploy/gitops/charts/traefik-plugins/clientipheaders`), operator tests and static-server tests cover each answer, and the `BEX_TRAEFIK_INTEGRATION` gates for the redirect and the static site assert the headers on the deployed Traefik release.
5. ADR018's "Edge headers a web service receives" row records each choice and every difference that stays; ADR022's table and ADR029 agree with it.
6. On a named production pin, after the Traefik rollout and the operator's: a disposable static site and a disposable web service in `bex-canary` show items 1–3, the unknown host shows item 2, and `beancount.io`, `bex.co`, `dashboard.bex.co` and `api.bex.co` answer as before the change, checked right after each rollout.

## Source + Goal linkage

- **Source:** promoted from [w4/736](../736.md) on 2026-11-27 by `/loopx w4`. w4/m687 t007's parity check filed it: the note sized the redirect half at about an hour and said the static-site half might need a promotion. Triage found a third answer with the same gap (the unknown host) and that the static half needs the static server's own cache state, so the work is about 6h across several tasks.
- **Render (captured 2026-11-27 with `curl` from one US client; the first rows are in [edge-headers.md](../../../../docs/render-artifacts/edge-headers.md), the rest at 11:30–11:40 UTC for this triage):**
  - A web service: `http://httpbin.onrender.com/` answers `301` with `cf-cache-status: DYNAMIC`, `Server: cloudflare`, `CF-RAY` and no `rndr-id`; a `POST` answers `307` with `DYNAMIC`.
  - A static site's `http://` redirect (`vue.onrender.com`, `create-react-app.onrender.com`): `301` with `cf-cache-status: EXPIRED` and no `rndr-id` on the first request; the next three answered `HIT` with `Age: 7`, so Cloudflare caches the redirect. A `POST` answered `307` with `DYNAMIC`.
  - A static site's `GET /` over `https://` (`vue`, `sveltekit-static`): `200`, `cache-control: public, max-age=0, s-maxage=300`, `cf-cache-status: EXPIRED`; the next read `HIT` with `age: 0` and the same `rndr-id` as the first (the stored answer's). A `HEAD` read `HIT` with `age: 7`.
  - A missing file: `404` with `BYPASS` (`create-react-app`, `sveltekit-static`). A route redirect (`vue`'s `/* → /index.html`, answering `301` `location: /index.html`): `BYPASS`, each with a fresh `rndr-id`.
  - An unknown host (`no-such-service-w4-736.onrender.com`): `http://` answers `301` and `https://` `404`, both `cf-cache-status: DYNAMIC`, `server: cloudflare` and no `rndr-id`.
- **bex today (production, same hour):** `http://no-such-service-w4-736.onbex.co/` answers `301` with `Cf-Ray`, `Rndr-Id`, `Server: cloudflare` and no `cf-cache-status`; `https://` answers `404` the same way. In the code:
  - `deploy/gitops/charts/traefik-plugins/clientipheaders/clientipheaders.go`: the entrypoint's sanitizer sets `Cf-Ray` and `Rndr-Id` on every response before the route runs. Only the `originServer` instance (each web service's own route, last in its chain) adds `Cf-Cache-Status: DYNAMIC` (`moveServer`). The `httpsRedirect` instance leads every tenant route and the fallback, so a redirect never reaches `originServer`.
  - `lego/operator/internal/controller/app_controller.go`: `reconcileIngress` puts `httpsRedirectSpec` first in every App's chain, web service and static site alike; `reconcileStaticSiteMiddlewares` gives a static site the IP allow-list and the `staticSite` instance, which edits only the request.
  - `lego/operator/config/activator/platform-host-fallback-*.yaml`: the unknown host's route is `platform-host-fallback-https` (the same redirect instance) in front of the activator's `404`.
  - `lego/operator/internal/staticserver/`: `get` serves an object from the pod's in-memory cache or fetches it from the origin (`staticserver.go`), and caches a confirmed miss too. Objects are immutable per revision ([ADR029](../../../../docs/ADR029-static-sites.md)), so nothing in it ever expires.
- **Related:** [w4/742](../../blocked/742.md) asks for a global CDN in front of static sites. If that ships, the CDN reports the static statuses and t004's mapping moves to it; until then the static server is the only cache in the path. w4/m687 stays parked on its own t004 decision and does not gate this.
- **Goal linkage:** [ADR008 § Render parity first](../../../../docs/ADR008-vision.md#render-parity-first). Render's docs tell apps to use what Cloudflare adds ([uptime-best-practices](https://render.com/docs/uptime-best-practices)), and uptime checks and CDN debugging read `cf-cache-status`.
- **Expected outcome:** every answer a bex host sends reads a `cf-cache-status` in Render's vocabulary, and only answers that reached a service carry a `rndr-id`.
- **Why now:** w4/m687 made web services report the header, so the answers that still lack it now stand out against the ones that have it.
- **Rollout risk:** the plugin reaches production through Argo CD from `main` (a new Traefik pod template), the operator through `deploy.yml`'s pin, in either order. w4/m687's push took `beancount.io` and `bex.co` down for about 14 minutes. Each task that changes the plugin checks those first-party sites right after the Traefik rollout and rolls back at once if they fail; t001 also proves that either order of the two rollouts is safe.
- **Render parity included:** no REST, GraphQL, MCP or dashboard verb changes, but every request to a tenant host gets these answers, so t006 walks them against Render's.

## Record — 2026-11-27

- **Shipped:** `9e9cb29f8f` (t001–t004: the plugin's redirect statuses, the unknown host's `404`, the static server's statuses, Render's captures, ADR018/ADR022/ADR029, and [w4/755](../../755.md) for the suspended answers).
- **On production:** the Traefik pods run the new plugin since 12:26:07 UTC; the four first-party sites answered as before throughout; the unknown host shows item 2 (t005).
- **Gate:** the operator image, which writes a static site's redirect spec and runs the static server, deploys with the next `chore(deploy): pin platform images to …` that carries `9e9cb29f8f` (`deploy.yml` deploys at most every 6 hours). Then t005 steps 3–4, t006 steps 1 and 3, and t009.

## Unblock (2026-11-28, `/loopx w4` worker)

- **Affects:** t005 steps 3–4 (the production walk), t006 steps 1 and 3, then t009.
- **Source:** verified evidence. No user decision is involved.
- **Release gate: clear.** The pin the gate waited for, `d465649ee4` (2026-11-27 16:20 UTC), pins `546d61ac7343`, and `git merge-base --is-ancestor 9e9cb29f8f 546d61ac7343` exits 0. Production has since moved to the next pin, `207a050b22` (22:56 UTC), pinning `7d1dce861173`, which also carries `9e9cb29f8f`: `bex-controller-manager`, `bex-static-server` and `bex-activator` run its operator digest (`sha256:d2f83bec…`), the pods started 23:00:27–23:00:47 UTC with no restarts.
- **First-party sites after the operator's rollout (00:57 UTC):** `https://beancount.io/`, `https://bex.co/`, `https://dashboard.bex.co/` and `https://api.bex.co/` answer `200`, `200`, `307` and `404`, as before the change, each with `server: cloudflare`, a `cf-ray` and `cf-cache-status: DYNAMIC`.
- **Next:** t005 steps 3–4 walk a disposable static site and web service in `bex-canary` with `curl`, t006 compares the answers with Render's, and t009 closes the milestone if definition-of-done item 6 holds.

## Live check: DONE (2026-11-28, `/loopx w4` worker, pin `7d1dce861173`)

- **Walk (t005):** a disposable static site and image web service in `bex-canary` and an unknown host showed definition-of-done items 1–3 with `curl`: the web service's `http://` `301` and every `307` read `DYNAMIC` with no `rndr-id`; the static site's `http://` `301` read `MISS`, then `HIT` with a counting `age`, and `BYPASS` with a query; its files read `MISS` then `HIT` (`HEAD`, `304` and a query too), its missing file and route redirect `BYPASS`, `POST` `DYNAMIC`, and its `CF-Cache-Status` header rule changed nothing; the unknown host read `DYNAMIC` with no `rndr-id` over both schemes. Both services are deleted.
- **First-party sites:** `beancount.io`, `bex.co`, `dashboard.bex.co` and `api.bex.co` answered `200`, `200`, `307` and `404` at 00:57 and 01:03 UTC, as before the change, and Traefik counted no `5xx` on the API's or the dashboard's routers through either operator rollout.
- **Parity (t006):** every answer matches Render's capture or is in ADR018's row; Render's `HIT` repeating the stored answer's `rndr-id` joined difference (5), and a `POST` under a local redirect rule (`301` on bex, a bare `200` on Render) is [w4/758](../../758.md).
