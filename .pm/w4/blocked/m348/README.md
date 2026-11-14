# w4 · m348 — A dashboard deploy can cache the HTML shell under a JavaScript chunk's URL for a year, in Cloudflare and in the browser, so service pages fail with "Something went wrong" and neither Try again nor a reload recovers them

**Worker:** worker4 **Goal:** a dashboard rollout never leaves a browser or the edge holding HTML for a script URL. A request for an asset that doesn't exist answers 404 and is never cached, a real asset stays immutable, and a page that hit a version mismatch recovers on Try again. **Status:** blocked — release gate (t001–t005 done, code shipped 2026-11-14 by `/loopx w4`; t006 replays the live DoD after the next production deploy)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | A missing `/assets/*` file answers 404 with `Cache-Control: no-store`, never the HTML shell or the immutable header — **DONE** (shipped by 9484c8339 with [w21/m394 t004](../../w21/m394/done/t004.md), 2026-11-14; its built-bundle check covers the missing-file, real-chunk, page and association-document criteria. This milestone extended it to `/_assets/`) | 45m | — |
| [t002](done/t002.md) | A rollout's overlap serves both builds' assets, or fails only once and recovers — **DONE** | 45m | t001 |
| [t003](done/t003.md) | Move every asset URL once, so browsers that already cached HTML under a chunk URL stop using it — **DONE** | 20m | t001 |
| [t004](done/t004.md) | Simplify — **DONE** | 15m | t002, t003 |
| [t005](done/t005.md) | Test coverage — **DONE** | 30m | t002, t003 |
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

## Implementation (2026-11-14, t001–t005)

- **Asset directory moved once:** build files are served from `/_assets/` (`ASSETS_DIR`, `dashboard/server/assets-paths.ts`). The DoD's `/assets/...` probes still apply: every path there now answers `404 no-store`. Probe `/_assets/qa-missing-<ts>.js` as well. The live `index-*` and its chunks are under `/_assets/`.
- **Miss handling (t001):** shipped by 9484c8339 (w21/m394 t004). Its global middleware `dashboard/server/middleware/assets-miss.ts` answers every miss with `404`, `no-store`, `nosniff` and a `text/plain` body, before the SSR renderer. This milestone extended `isAssetsPath` to cover `/_assets/` as well as `/assets/`. A real file keeps `public, max-age=31536000, immutable`.
- **No Cloudflare purge:** the deploy token is DNS-only (`.github/workflows/deploy.yml:702-706`). HTML copies stored under `/assets/` URLs, at the edge or in a browser, are never requested again, because no page names those URLs any more.
- **Tests:** `yarn build && yarn test:built-server` (`dashboard/scripts/check-built-server.mjs`, run in `dashboard-test.yml`) plus two unit suites. `server/__tests__/security-headers.test.ts` checks that every immutable route rule covers a path the miss middleware answers with 404. `server/__tests__/assets-cache-guard.test.ts` covers both directories.

### Rollout overlap decision (t002): (c)

- **Chosen:** (c). t001 keeps every miss out of every cache, so a document that meets a version mismatch recovers: TanStack's `lazyRouteComponent` reloads the document once by itself on a missing module, and otherwise w4/269's Try again does it.
- **Why not (a):** shipping the previous release's `assets/` in the new image only covers an old document reaching the new pod. A new document whose chunk request reaches the old pod still misses, because the old pod can never hold the new build's chunks. So (a) can't deliver "no error at all". It would also make every image build depend on the last-pushed `:latest`, which is not always the live build, and need generation pruning.
- **Why not (b):** an object-store asset origin would solve both directions, but it is a new serving path (bucket, CDN rule, upload step in `deploy.yml`) for an overlap that lasts a few seconds per deploy.
- **What's left:** while both pods serve, each request is load-balanced separately. A full load in that window can end on "Something went wrong", and so can a Try again inside the window. Once the old pod is gone, Try again recovers. The local two-version replay is in [t002](done/t002.md).

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

## Reproduced again (round 235, 2026-11-12 21:53–21:55 UTC)

Live `/qa-find-bugs w4` loop, round 235, during the rollout of pin `764072bb8e8a`. The pin commit `f12c28585` landed at 21:49:46, and the `deploy.yml` run for `764072bb8e8a` was still `in_progress` at 21:53:04. Same mechanism and same DoD as round 193; nothing in this milestone has shipped yet.

- **21:53:5x.** `page.goto('/services/srv-dbr3aifst1cc739eveqg/settings')` (a fresh Free fixture, deleted afterwards) showed "Something went wrong — Failed to fetch dynamically imported module: https://dashboard.bex.co/assets/ory-toaster-BLHMTEtc.js". The console logged `Expected a JavaScript-or-Wasm module script but the server responded with a MIME type of "text/html"` for about 15 chunk URLs.
- **21:54:24.** `curl` from outside already got JavaScript for the same URLs: `ory-toaster-BLHMTEtc.js` → `200 text/javascript`, `public, max-age=31536000, immutable`, `cf-cache-status: MISS`, `last-modified: Thu, 12 Nov 2026 21:42:48 GMT`.
- **21:54:37.** A full reload still failed. The browser served its own cached HTML for:
  - `services._serviceId-B-0w4F0-.js`
  - `service-detail-layout-CoxPgBCU.js`
  - `use-cached-service-type-Ncc-kd95.js`
  - `requires-ssh-key-D5N1Z8dc.js`
  - `chevron-left-CPCkh6pj.js`
  - `ory-toaster-BLHMTEtc.js`
  - `octagon-x-BbWxamwr.js`

  Each came back `200 text/html; charset=utf-8`, `public, max-age=31536000, immutable`, `cf=MISS`.
- In-page `fetch('/assets/<chunk>')`:
  - with the default cache: `text/html`;
  - with `cache: 'no-store'`: `text/javascript`.
- **Try again** (w4/269) failed the same way.
- **The serving policy is unchanged on the new pin.** `curl -sSI https://dashboard.bex.co/assets/qa-r235-missing-<ts>.js` twice: `200 text/html; charset=utf-8`, `public, max-age=31536000, immutable`, `cf-cache-status: MISS`, then `HIT`, and the body was `<!DOCTYPE html>…`. That is DoD 1, still failing.
- **How far it reached.** By 21:55:51 the edge served JavaScript for these URLs, with and without `Accept-Encoding: gzip, deflate, br, zstd` (`cf-cache-status: HIT`, colo SJC). So this round's poisoning stayed in the browser.
  - The browser still loaded the HTML after DevTools `Network.clearBrowserCache` and a fresh `page.goto` (21:55:42). Its responses read `cf=HIT age=134`, which places the store at about 21:53:28, during the rollout.
  - Only an in-page `fetch('/assets/<chunk>', {cache: 'reload'})` of each of the seven URLs recovered the page. A user has no equivalent short of clearing site data.
