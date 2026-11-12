# w4 · m348 — A dashboard deploy can cache the HTML shell under a JavaScript chunk's URL for a year, in Cloudflare and in the browser, so service pages fail with "Something went wrong" and neither Try again nor a reload recovers them

**Worker:** worker4 **Goal:** a dashboard rollout never leaves a browser or the edge holding HTML for a script URL. A request for an asset that doesn't exist answers 404 and is never cached, a real asset stays immutable, and a page that hit a version mismatch recovers on Try again. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | A missing `/assets/*` file answers 404 with `Cache-Control: no-store`, never the HTML shell or the immutable header | 45m | — |
| [t002](t002.md) | A rollout's overlap serves both builds' assets, or fails only once and recovers | 45m | t001 |
| [t003](t003.md) | Move every asset URL once, so browsers that already cached HTML under a chunk URL stop using it | 20m | t001 |
| [t004](t004.md) | Simplify | 15m | t002, t003 |
| [t005](t005.md) | Test coverage | 30m | t002, t003 |
| [t006](t006.md) | Closeout (live replay on production) | 30m | t004, t005 |

## Definition of done

All on production:

1. **A missing asset.** `curl -sI https://dashboard.bex.co/assets/qa-missing-<ts>.js`, twice:
   - each answers `404`, with `cache-control: no-store` and a body that is not the HTML shell;
   - the second answer is not a Cloudflare `HIT`.

   The same holds for a missing `.css` and for a nested path (`/assets/nope/qa.js`).
2. **A real asset.** A chunk named by the live `index-*.js` answers `200`, `text/javascript`, `public, max-age=31536000, immutable`.
3. **Pages unchanged.** A page keeps `private, no-cache` (w4/526), and `/.well-known/apple-app-site-association` and `/.well-known/assetlinks.json` keep `public, max-age=300`.
4. **Through a rollout.** During the next dashboard rollout, a browser that keeps its cache loads `/services/<id>/settings` every 5 s from before the new pod is Ready until the old pod is gone. Either no load ends on "Something went wrong", or every one that does recovers on Try again.
   - Afterwards, for every script the page loads, `fetch(url)` with the default cache answers `text/javascript`.
5. **Fixtures.** Any fixture service is deleted.

## Source + Goal linkage

- **Source:** live `/qa-find-bugs w4` loop, round 193, 2026-11-12 12:16–12:30 UTC (`muse.env`, workspace `bex-canary`), across the production rollout of pin `9a5b95e96210`. The pin commit `8d40888e1` landed at 12:12:31, and deploy run `43213664753`'s deploy job completed at 12:18:38. The finding is below.
- **Governing doc:** the dashboard's serving policy (`dashboard/server/security-headers.ts`, w4/526) and w4/269 (done), which made Try again reload the document after a module-load error. 269 recorded "the observed immutable cache header on the missing-chunk response" as trigger diagnostics and did not fix the serving policy.
- **Goal linkage:** dashboard reliability across platform deploys, which happen up to four times a day (`deploy.yml`, 6-hour pin age). Every dashboard surface is a Render-parity surface the user can't reach while this happens.
- **Expected outcome:** a deploy never strands a user on an error page, and a transient miss during a rollout is retried, not cached for a year.
- **Why now:** major. Once a browser has cached the HTML, every service page that imports the chunk fails, and w4/269's recovery does not help. Chunks whose content does not change keep their URL across builds (`vendor-react-v-sG7it2.js` answered with `age: 2388854`, 27 days), so the poisoning can outlive many deploys.
- **Render parity task omitted:** no REST, GraphQL, MCP or dashboard behavior changes. This is the dashboard server's response headers and status for asset paths.

## Finding (round 193)

### Probe

- **The failure.** At 12:25:3x, navigating to `/services/srv-dbqqp5i8vjhs73do9flg/settings` showed "Something went wrong — Failed to fetch dynamically imported module: https://dashboard.bex.co/assets/service-sidebar-CFrUhXZI.js".
  - The console logged `Expected a JavaScript-or-Wasm module script but the server responded with a MIME type of "text/html"` for `services._serviceId-BdP81r-h.js`, `service-sidebar-CFrUhXZI.js` and `braces-8cRA86PF.js`.
  - Three full `page.goto` loads at 12:26:20, 12:26:26 and 12:26:31 failed the same way. Each served the current entry `index-IKLlIlK-.js`.
  - The error page's **Try again** (w4/269's document reload) failed the same way.
- **What the browser held.** In-page `fetch('/assets/<chunk>')` with the default cache, for `service-sidebar-CFrUhXZI.js` and `braces-8cRA86PF.js`:
  - `200 content-type: text/html`, `cache-control: public, max-age=31536000, immutable`, `cf-cache-status: HIT`, `age: 509`;
  - `services._serviceId-BdP81r-h.js` likewise, with `cf: MISS`.
  - The same URLs with `cache: 'no-store'` answered `text/javascript`. So the browser was serving its own cached copy of HTML. Its `HIT, age: 509` says Cloudflare had served that HTML from its edge, stored about 12:17:3x, inside the rollout.
- **It reproduces without a rollout.** Any missing asset answers the HTML shell with the immutable header, and the edge caches it:

  ```text
  /assets/qa-r193-missing-chunk-Xy12Ab.js → 200 text/html; charset=utf-8, public, max-age=31536000, immutable, cf=MISS, body "<!DOCTYPE html>…"
  /assets/qa-r193-missing.css             → the same
  /assets/nope/qa.js                       → the same
  /assets/qa-r193-edgeprobe-<ts>.js        → 200 text/html cf=MISS, then 200 text/html cf=HIT age=1, then cf=HIT age=3
  ```

- **Recovery.** Only clearing the browser's cache (CDP `Network.clearBrowserCache`) let the page load: 16 assets, all `text/javascript`. By 12:29 the edge answered the three chunk URLs with JavaScript (`HIT`, `age: 226`–`292`). Whether the edge's HTML copy was replaced or another edge node answered was not traced.

### Root cause

- **The header is unconditional.** `buildRouteRules` (`dashboard/server/security-headers.ts:72-97`) gives every response under `/assets/**` `Cache-Control: public, max-age=31536000, immutable`. Nitro route-rule headers apply to whatever answers the path, including the renderer.
- **A missing asset falls through to the SSR shell.** The Nitro Vite plugin registers the client build as one public-asset directory at `baseURL: "/"` with `fallthrough: true` (`dashboard/node_modules/nitro/dist/vite.mjs:711-716`, nitro `3.0.1-20260310-233855-5ea3e153`). So a path under `/assets/` with no file reaches the renderer, which answers `200` with the HTML document.
  - Nitro's own 404-for-missing behavior applies only to a non-root, non-fallthrough asset base (`dist/_chunks/nitro.mjs:118-124`, `dist/_build/common.mjs:22176`).
- **A rollout creates misses for real chunk names.** The dashboard runs 1 replica with `maxSurge: 1`, `maxUnavailable: 0`, `minReadySeconds: 5` (`dashboard/deploy/deployment.yaml:9-18`). Old and new pods serve together for a window: a new document's chunk request can reach the old pod, and an old document's can reach the new one. Either way it gets the immutable HTML.
- **Both caches keep it.**
  - Cloudflare caches the `.js` path (MISS, then HIT).
  - The browser keeps it for a year without revalidating (`immutable`), so a reload and w4/269's Try again reuse it.
- **No purge.** The deploy workflow has no Cloudflare cache purge; its token is DNS-only (`.github/workflows/deploy.yml:702-706`, `:899-905`).

### Not this milestone

- **The first cancel's health-check event.** In the same round, a cancel of a building deploy recorded `server_failed {"unhealthy":"The instance failed its health check"}` at 12:16:28 and `server_available` at 12:16:32, on an instance that was not replaced. It was during the platform rollout. A second cancel at 12:19:50, on the settled platform, recorded none, and 1 s polls of the URL answered 200 throughout (66/66). It is recorded on w8/m160, not filed.
- **Unknown pages answer 200.** `/does-not-exist-page` answers `200` (`private, no-cache`) with the not-found page. That is a separate status question.

### Dedupe

- w4/269 (done) fixed the Try again action, and named the immutable header on the missing-chunk response without fixing it.
- w4/526 (done) set the page and asset cache policy (`buildRouteRules`).
- w3/m244 (QUIC idle) and w9/m60 (bundle size) are other dashboard-delivery items.
- `grep -rlnE "dynamically imported module|preloadError|stale chunk|max-age=31536000, immutable" .pm` finds only those and static-site items.
- Not deploy lag: `git log a5be11e4e517..origin/main -- dashboard/server dashboard/vite.config.ts dashboard/deploy` is empty.

## Evidence — QA loop round 196 (2026-11-12): the edge kept the HTML for an hour, and a second rollout reproduced the miss from the other side

- **A page every Rollback lands on.** At 13:16:34, the dashboard's Rollback (on `qa-20261112-r196-rb`) redirected to `/services/<id>/deploys/dep-dbqrobu54r0c73bpufv0`, which showed "Something went wrong". The console logged `Expected a JavaScript-or-Wasm module script but the server responded with a MIME type of "text/html"` for `services._serviceId.deploys._deployId-DPqgC_8P.js` and `deploy-detail-page-DuTmeOhw.js`. The browser cache had been cleared at about 12:30 (round 193).
- **The edge held the HTML for an hour.** In-page `fetch('/assets/deploy-detail-page-DuTmeOhw.js')` with the default cache answered:
  - `200 content-type: text/html`, `cf-cache-status: HIT`, `age: 3584`, `date: Thu, 12 Nov 2026 13:16:33 GMT`, body `<!DOCTYPE html>`.
  - So Cloudflare had stored the HTML under that chunk URL at about 12:16:49, inside the 12:16–12:18 rollout of `9a5b95e96210`. It served it for the whole hour that build was live, to every browser at that edge that did not already hold the JavaScript.
- **The other direction.** A platform deploy (`workflow_dispatch` run `43222770042`, `f018ce945`, completed about 13:20) was rolling the dashboard to a new build (HTML entry `index-bE9o0aTT.js`, replacing `index-IKLlIlK-.js`).
  - The same request with `cache: 'reload'` reached the origin (`cf: MISS`) and still answered `text/html`, because the new build has no `deploy-detail-page-DuTmeOhw.js`.
  - An open tab on the old build therefore fails on every lazy route it hasn't loaded yet, until it reloads the document. Each such miss is again stored with `public, max-age=31536000, immutable`.
- **Control.** After `Network.clearBrowserCache`, the same kind of page on the new build (`/services/srv-dbkq3hdhhjfs73eiahe0/deploys/<id>`) loaded 17 assets, all `text/javascript`, `cf: MISS`.
- **What this adds to the DoD:** the edge, not only the browser, must never keep a miss (DoD 1's second request). t002's overlap fix must also cover a tab left open on the previous build, which is the common case after a Rollback, a form submit or any lazy route.
