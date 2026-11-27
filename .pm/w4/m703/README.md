# w4 · m703 — Every answer the edge sends carries `cf-cache-status`, as Render's does: the HTTP-to-HTTPS redirect, the unknown host's 404, and a static site's answers

**Worker:** worker4 **Goal:** a site moved from Render reads the same `cf-cache-status` on bex as on Render for every answer its host sends, and a redirect or unknown-host answer carries no `rndr-id`, as Render's do not. Since w4/m687 only a web service's own answers read `cf-cache-status: DYNAMIC`; every other answer has none. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | The HTTP-to-HTTPS redirect answers `cf-cache-status` and no `rndr-id` | 1h15m | — |
| [t002](t002.md) | The unknown host's `404` reads `cf-cache-status: DYNAMIC` and no `rndr-id` | 45m | t001 |
| [t003](t003.md) | Capture Render's static-site statuses for the answers not yet captured | 30m | — |
| [t004](t004.md) | The static server answers a `CF-Cache-Status` on every answer | 1h30m | t003 |
| [t005](t005.md) | Amend ADR018, ADR022 and ADR029, then verify on production | 45m | t001, t002, t004 |
| [t006](t006.md) | Render parity | 20m | t005 |
| [t007](t007.md) | Simplify | 15m | t006 |
| [t008](t008.md) | Test coverage | 30m | t006 |
| [t009](t009.md) | Closeout | 10m | t007, t008 |

Total: **9 tasks, about 6h**.

## Definition of done

1. `curl -I http://<web service host>/` answers `301` with `cf-cache-status: DYNAMIC`, `server: cloudflare`, a `cf-ray` and no `rndr-id`; a `POST` over `http://` answers `307` with the same. A static site's `http://` redirect answers the status t001 records for it, and its `POST` `307` reads `DYNAMIC`.
2. `http://` and `https://` to an unknown `*.onbex.co` host answer `cf-cache-status: DYNAMIC` and no `rndr-id`, as `no-such-service.onrender.com` does.
3. A static site's `GET /` reads a `cf-cache-status` on every answer, with the mapping t004 records: a status for an object read through the static server's cache and one for an object fetched from the origin, `HEAD` as `GET`, and `BYPASS` for a missing file's `404` and a route redirect, as on Render. A static site's header rule cannot change it.
4. Plugin tests (`deploy/gitops/charts/traefik-plugins/clientipheaders`), operator tests and static-server tests cover each answer, and the `BEX_TRAEFIK_INTEGRATION` gates for the redirect and the static site assert the headers on the deployed Traefik release.
5. ADR018's "Edge headers a web service receives" row records each choice and every difference that stays; ADR022's table and ADR029 agree with it.
6. On a named production pin, after the Traefik rollout and the operator's: a disposable static site and a disposable web service in `bex-canary` show items 1–3, the unknown host shows item 2, and `beancount.io`, `bex.co`, `dashboard.bex.co` and `api.bex.co` answer as before the change, checked right after each rollout.

## Source + Goal linkage

- **Source:** promoted from [w4/736](../done/736.md) on 2026-11-27 by `/loopx w4`. w4/m687 t007's parity check filed it: the note sized the redirect half at about an hour and said the static-site half might need a promotion. Triage found a third answer with the same gap (the unknown host) and that the static half needs the static server's own cache state, so the work is about 6h across several tasks.
- **Render (captured 2026-11-27 with `curl` from one US client; the first rows are in [edge-headers.md](../../../docs/render-artifacts/edge-headers.md), the rest at 11:30–11:40 UTC for this triage):**
  - A web service: `http://httpbin.onrender.com/` answers `301` with `cf-cache-status: DYNAMIC`, `Server: cloudflare`, `CF-RAY` and no `rndr-id`; a `POST` answers `307` with `DYNAMIC`.
  - A static site's `http://` redirect (`vue.onrender.com`, `create-react-app.onrender.com`): `301` with `cf-cache-status: EXPIRED` and no `rndr-id` on the first request; the next three answered `HIT` with `Age: 7`, so Cloudflare caches the redirect. A `POST` answered `307` with `DYNAMIC`.
  - A static site's `GET /` over `https://` (`vue`, `sveltekit-static`): `200`, `cache-control: public, max-age=0, s-maxage=300`, `cf-cache-status: EXPIRED`; the next read `HIT` with `age: 0` and the same `rndr-id` as the first (the stored answer's). A `HEAD` read `HIT` with `age: 7`.
  - A missing file: `404` with `BYPASS` (`create-react-app`, `sveltekit-static`). A route redirect (`vue`'s `/* → /index.html`, answering `301` `location: /index.html`): `BYPASS`, each with a fresh `rndr-id`.
  - An unknown host (`no-such-service-w4-736.onrender.com`): `http://` answers `301` and `https://` `404`, both `cf-cache-status: DYNAMIC`, `server: cloudflare` and no `rndr-id`.
- **bex today (production, same hour):** `http://no-such-service-w4-736.onbex.co/` answers `301` with `Cf-Ray`, `Rndr-Id`, `Server: cloudflare` and no `cf-cache-status`; `https://` answers `404` the same way. In the code:
  - `deploy/gitops/charts/traefik-plugins/clientipheaders/clientipheaders.go`: the entrypoint's sanitizer sets `Cf-Ray` and `Rndr-Id` on every response before the route runs. Only the `originServer` instance (each web service's own route, last in its chain) adds `Cf-Cache-Status: DYNAMIC` (`moveServer`). The `httpsRedirect` instance leads every tenant route and the fallback, so a redirect never reaches `originServer`.
  - `lego/operator/internal/controller/app_controller.go`: `reconcileIngress` puts `httpsRedirectSpec` first in every App's chain, web service and static site alike; `reconcileStaticSiteMiddlewares` gives a static site the IP allow-list and the `staticSite` instance, which edits only the request.
  - `lego/operator/config/activator/platform-host-fallback-*.yaml`: the unknown host's route is `platform-host-fallback-https` (the same redirect instance) in front of the activator's `404`.
  - `lego/operator/internal/staticserver/`: `get` serves an object from the pod's in-memory cache or fetches it from the origin (`staticserver.go`), and caches a confirmed miss too. Objects are immutable per revision ([ADR029](../../../docs/ADR029-static-sites.md)), so nothing in it ever expires.
- **Related:** [w4/742](../742.md) asks for a global CDN in front of static sites. If that ships, the CDN reports the static statuses and t004's mapping moves to it; until then the static server is the only cache in the path. w4/m687 stays parked on its own t004 decision and does not gate this.
- **Goal linkage:** [ADR008 § Render parity first](../../../docs/ADR008-vision.md#render-parity-first). Render's docs tell apps to use what Cloudflare adds ([uptime-best-practices](https://render.com/docs/uptime-best-practices)), and uptime checks and CDN debugging read `cf-cache-status`.
- **Expected outcome:** every answer a bex host sends reads a `cf-cache-status` in Render's vocabulary, and only answers that reached a service carry a `rndr-id`.
- **Why now:** w4/m687 made web services report the header, so the answers that still lack it now stand out against the ones that have it.
- **Rollout risk:** the plugin reaches production through Argo CD from `main` (a new Traefik pod template), the operator through `deploy.yml`'s pin, in either order. w4/m687's push took `beancount.io` and `bex.co` down for about 14 minutes. Each task that changes the plugin checks those first-party sites right after the Traefik rollout and rolls back at once if they fail; t001 also proves that either order of the two rollouts is safe.
- **Render parity included:** no REST, GraphQL, MCP or dashboard verb changes, but every request to a tenant host gets these answers, so t006 walks them against Render's.
