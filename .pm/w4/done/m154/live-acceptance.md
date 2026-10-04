# m154 live acceptance — 2026-10-02 (UTC 2026-10-03T07:16–07:21Z)

**Deployed:** `bex-static-server` / `bex-api` `ghcr.io/bex-co/bex-operator@sha256:e396cf3585ac…` and the dashboard, from GitOps pin `20fb64867` → `1263d12ae`. That commit contains `703068251`, `966328891` and the requested `18958459c`. **Fixture:** Free static site `qa-20261003-static-m154` / `srv-db0anjg9ohsc73e5r17g` (`bex-co/bex` `main`, `examples/static-site`, publish `.`, no build, auto-deploy off). Live on the first deploy; `/render.yaml` serves 795 bytes, sha256 `8c76de576c5a…`.

Saves used the dashboard's `SetStaticRoutes` mutation via curl with the signed-in session. HTTP probes used curl GET/HEAD without following redirects. The browser checks, fresh UI read and typed-confirmation delete ran in an independent headless Chromium, because the shared MCP browser was busy.

## Nine finding rows (saved 07:17:35Z; live by 07:17:37Z)

GET and HEAD gave identical status/Location:

| Request | Result |
| --- | --- |
| `/qa-jump/a%3Fb.yaml` | 301 `location: /docs/a%3Fb.yaml` |
| `/qa-jump/a%23b.yaml` | 301 `location: /docs/a%23b.yaml` |
| `/qa-jump/a%25b.yaml` | 301 `location: /docs/a%25b.yaml` |
| `/qa-jump/a%20b.yaml` | 301 `location: /docs/a%20b.yaml` |
| `/qa-jump/plain.yaml` | 301 `/docs/plain.yaml` |
| `/qa-rewrite.yaml` (→ `/render.yaml?from=rule`) | **200, 795 B, sha `8c76de576c5a`**, no Location |
| `/qa-encoded.yaml` (→ `/%72ender.yaml`) | **200, 795 B, sha `8c76de576c5a`**, no Location |
| direct `/render.yaml?from=rule`, `/%72ender.yaml`, `/qa-base.yaml` | 200, same bytes/hash |
| `/qa-copy/render.yaml?incoming=one` (`/:splat`) | 200, same bytes/hash |
| `/qa-order` | 301 `/render.yaml` (redirect before rewrite) |
| `/render.yaml` (has a redirect to `/index.html`) | 200 file (file wins) |
| `/qa-redirect` | 301 `/render.yaml?from=rule#frag` (configured components kept) |

Browser navigation: `/qa-jump/a%23b.yaml` ends at pathname `/docs/a%23b.yaml` with an empty hash, and `/qa-jump/a%3Fb.yaml` ends at `/docs/a%3Fb.yaml`. These are 404 because no such object exists, which is expected. Before the fix, the hash case navigated to `/docs/a#b.yaml`.

## Alias and plain-destination change

Saved `/qa-jump/* → /docs/*` and `/qa-rewrite.yaml → /render.yaml` at 07:18:12Z. To prove the snapshot had refreshed, a tenth marker row `/qa-refresh-v2` was added at 07:18:52Z; it was live at 07:19:00Z. After that: question and hash cases are again 301 to `/docs/a%3Fb.yaml` and `/docs/a%23b.yaml` (GET and HEAD); the browser keeps pathname `/docs/a%23b.yaml`; `plain.yaml` is 301 to `/docs/plain.yaml`; `/qa-rewrite.yaml` is 200 with the same 795-byte hash, and HEAD is 200.

## Read surfaces

REST `GET /v1/services/{id}/routes`, GraphQL `service.routes`, MCP `list_static_routes` and a freshly loaded `/static/{id}/redirects` page all returned the identical ordered rows, including the exact `/render.yaml?from=rule#frag`, `/:splat`, `/docs/*` and `/%72ender.yaml` strings. 0 console errors. One earlier page load hit a transient `net::ERR_TIMED_OUT`; the retry loaded normally.

## Delete through typed confirmation

Settings → Delete Service opened the dialog "Type `sudo delete static site qa-20261003-static-m154` below to confirm". The confirm button was disabled until the phrase was typed. `DeleteService` returned `{"deleteService":true}` at 07:20:19Z and the page landed on Overview. At 07:21:11Z: REST service and `/routes` returned 404; the name-filtered list returned `[]`; GraphQL returned null + `not found`; public `/render.yaml` returned 404. The pre-delete App (UID `4676c96e-…`), `bex-static-…` Service, Ingress, Certificate `…-tls`, pull Secret and clone Secret were all absent, with no cluster-wide pod/job/secret/svc/ingress match. The session was revoked at the end of the run.
