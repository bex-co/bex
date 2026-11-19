# w3 · m488 — An edge-cached response answers conditional requests with 304 and carries its Age

**Worker:** worker3 **Goal:** turning on edge caching no longer turns an app's 304s into full 200s: a conditional request whose validator matches the cached response gets `304 Not Modified`, on a hit and on the fill alike, and every response answered from the cache carries `Age`. **Status:** done (2026-11-19; shipped in `f718f80d1`, verified on production pin `7af50a28168d`, § Production record)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | A cache hit answers a matching If-None-Match or If-Modified-Since with 304 — **DONE** | 45m | — |
| [t002](done/t002.md) | A miss or expired fill answers the client's matching conditional with 304 and still stores the full body — **DONE** | 45m | t001 |
| [t003](done/t003.md) | Every hit and 304 carries Age — **DONE** | 20m | t001 |
| [t004](done/t004.md) | Record it in ADR004, ship and verify on production — **DONE** | 30m | t002, t003 |
| [t005](done/t005.md) | Simplify — **DONE** | 15m | t004 |
| [t006](done/t006.md) | Test coverage — **DONE** | 30m | t004 |
| [t007](done/t007.md) | Closeout — **DONE** | 10m | t005, t006 |

## Definition of done

- Unit tests: a fresh stored response answers a matching `If-None-Match` (weak comparison, `*`, lists) or `If-Modified-Since` (only without `If-None-Match`) with 304, no body, the RFC 9110 §15.4.5 headers and `CF-Cache-Status: HIT`; a non-matching validator gets the full 200.
- The request that fills the cache (MISS or EXPIRED) gets 304 when its validator matches, and the cache still stores the full body.
- Every HIT and 304 carries `Age` (upstream Age at store time plus seconds since).
- On production (t004), a second conditional request for a cached path answers `304` with `CF-Cache-Status: HIT` and `Age`; recorded in this README.

## Source + Goal linkage

- **Source:** `/pm-brainstorm` for w3 2026-11-18 (topic: one aspect of Render parity, web-service edge caching), item 3; filed by `/pmx them to w3` 2026-11-18. Evidence on `origin/main` `4e3c87031`: `lego/operator/internal/edgecache/edgecache.go:321-322` strips `If-None-Match`/`If-Modified-Since` before the fill, and `writeEntry` (`:489-499`) always writes the stored status and full body with no `Age`. So on every cacheable path an app's own 304s become full 200s, and those bytes leave through the pod's eth0, which the bandwidth meter counts against the plan's included bandwidth (w6/m319, `docs/ADR030-pricing.md` §14).
- **Goal linkage:** ADR008 § Render parity first: caching on Render is a CDN feature that should not make an app's responses heavier. RFC 9111 §4: a cache using a stored response "MUST generate an Age header field"; §4.3.2: a cache handles conditional requests against stored responses.
- **Evidence boundary:** Render's own 304 and `Age` behavior on a cached web service is unverified (a Render workspace capture needs `w10/blocked/002`). This milestone rests on the RFCs and on not regressing the app's own 304s, not on an observed Render response.
- **Expected outcome:** revalidating clients get empty 304s again, saving the customer's metered bandwidth; downstream caches see the response's real age.
- **Why now:** bandwidth above the plan's included amount is billed since w6/m319, and every caching service pays for the lost 304s today.
- **Render parity omitted:** no REST, GraphQL, MCP or dashboard field, value or message changes; only the tenant HTTP responses the edge cache serves. t004 records the behavior against Render's documented cache.
- **Not filed (considered in the brainstorm):** serving `Range` requests from a stored object, and `stale-while-revalidate`/`stale-if-error`. Neither is documented for Render's edge cache, and Range has little effect under today's 6 MiB object cap; revisit with `w3/m489`.

## Unblocked (2026-11-19, `/loopx w3` routine `pin-gated-recheck`)

- **Affected:** t004, t007. **Source:** verified evidence, no user decision.
- **Finding:** the newest pin on origin/main, `826d32e42` (`chore(deploy): pin platform images to 7af50a28168d`, which moves the `bex-operator` image that carries the edge cache), contains `f718f80d1`: `git merge-base --is-ancestor f718f80d1 7af50a28168d` holds. The release condition in t004's progress note is met.
- **Before measuring:** The pin was pushed by `deploy.yml` run 44451653443, whose `deploy` job was still running at 2026-11-19 03:50Z: confirm that run finished green and that `bex-system`'s images match the pin before measuring. The pin does not carry `a6b1658fb` (`w3/m489`'s per-service tier), so production still runs the per-pod edge-cache sidecar this milestone changed; the deploy that carries `a6b1658fb` moves caching services to the tier.
- **Status:** pending verification. A deployed fix is not proof that the definition of done holds on production.
- **Next:** t004 steps 2–3 as its progress note says (an edge-cached fixture on a paid web service in `bex-canary`, or `w3/m486`'s if it is up): fetch a cacheable path twice, then once with `If-None-Match: <its ETag>`; record the statuses, `CF-Cache-Status`, `Age` and bytes here, then t007.

## Production record (t004, 2026-11-19)

**Release.** `deploy.yml` run 44451653443 finished green. Every `bex-system` Deployment ran `bex-operator@sha256:7ce6ae1e…`, the digest pin `826d32e42` (`7af50a28168d`) writes, and so did the fixture's `edge-cache` sidecar. The pin contains `f718f80d1`. It does not carry `a6b1658fb`, so this is the per-pod sidecar, not `w3/m489`'s tier.

**Fixture.** `m488-t004-cache` (`srv-dbv7kcna346c73cf1pvg`) was a starter web service in `bex-canary` running `docker.io/library/nginx:1.27-alpine`. Its `dockerCommand` wrote a 17-byte `/m488.css` before starting nginx, so the path is a static extension under `origin-controlled` and carries nginx's `ETag` and `Last-Modified`. The QA user (`scripts/qa-login.sh`) created it through REST at 04:25:22Z. Before caching, nginx answered `If-None-Match` itself with a 0-byte 304 and no `CF-Cache-Status`. REST `PATCH` `serviceDetails.cache.profile=origin-controlled` at 04:26:17Z added the sidecar, which rolled the service once (the residual `w3/m489` owns). Every request below went to `https://m488-t004-cache.onbex.co` and reached the pod `…-54d4bf554-zgf52`. Its `ETag` was `"6afe7a72-11"`.

| Request (UTC) | Status | `CF-Cache-Status` | `Age` | Body / header bytes |
| --- | --- | --- | --- | --- |
| 04:27:48 `GET /m488.css` | 200 | `MISS` | — | 17 / — |
| 04:27:49 `GET` | 200 | `HIT` | 0 | 17 / — |
| 04:28:11 `GET` | 200 | `HIT` | 22 | 17 / 283 |
| 04:28:11 `GET`, `If-None-Match: "6afe7a72-11"` | **304** | **`HIT`** | **23** | 0 / 193 |
| 04:28:12 `GET`, `If-Modified-Since: <its Last-Modified>` | 304 | `HIT` | 24 | 0 / 193 |
| 04:28:13 `GET`, `If-None-Match: "other"` | 200 | `HIT` | 24 | 17 / 283 |
| 04:28:14 `GET /m488.css?fill=1` (a new key), `If-None-Match: "6afe7a72-11"` | 304 | `MISS` | — | 0 / 185 |
| 04:28:14 `GET /m488.css?fill=1` | 200 | `HIT` | 0 | 17 / 282 |
| 04:28:15 `GET /m488.css?fill=1`, `If-None-Match: "6afe7a72-11"` | 304 | `HIT` | 1 | 0 / 192 |

- **Validators.** The first pass, 04:27:48–04:27:59, printed headers and body bytes but no status lines; its two 17-byte answers are the table's first two rows. In it, `If-None-Match` with the plain tag, with `W/"6afe7a72-11"`, with a list `"other", "6afe7a72-11"` and with `*` each read `HIT` with 0 body bytes and the 304 header set. `If-None-Match: "other"` together with a matching `If-Modified-Since` read the full 17 bytes, because `If-None-Match` decides when it is present.
- **The 304's headers.** It carried `accept-ranges`, `age`, `cf-cache-status: HIT`, the stored `date`, `etag`, `rndr-id` and `x-render-origin-server`. It carried no `content-type`, `content-length` or `last-modified`, as RFC 9110 §15.4.5 allows when an `ETag` is present.
- **The fill.** The new key's first request carried a matching validator. It got a bodiless 304 `MISS`, and the next plain `GET` read the full 17 bytes from the store (`HIT`, `Age: 0`).
- **Age.** Three conditional requests 5.6 s apart read `Age` 147, 153 and 158, the seconds since the 04:28:14 store. nginx sends no `Age` of its own.
- **Deviation, HEAD.** t004 step 2's literal `curl -sI` twice, then with `If-None-Match`, read `200 MISS`, `200 MISS` and `304 MISS`. That 304 was nginx's own, with `Last-Modified` and no `Age`. The cause is Traefik's transport, which adds `Accept-Encoding: gzip` to a GET that carries none but not to a HEAD. So a bare client's GETs fill the gzip key and its HEADs look up the identity key. On port 19099 directly, a bare GET and then a bare HEAD read `MISS`, then `HIT` with `Age: 0`. Through the edge, `curl -sI -H 'Accept-Encoding: gzip'` read `200 HIT` (`Age` 80) and then `304 HIT` (`Age` 81). The client still got a 304, so no app 304 became a 200. This milestone's definition of done is about GETs, and the HEAD gap predates it. It is filed as [w3/075](../075.md).
- **Cleanup.** The fixture was deleted (`204`, then `GET` `404`), and its pod and App are gone. This run's QA session was logged out (`ok logged-out`, then `401`).

**Definition of done.** Each item holds:

- The unit tests are `edgecache_conditional_test.go` (10 tests), and `go test ./internal/edgecache/` passed on `origin/main` `54db798f3`.
- On production, the fill answered a matching validator with 304 and stored the full body, as recorded above.
- Every HIT and HIT 304 above carried `Age`, and it advanced with the clock.
- On production, a second conditional request for a cached path answered `304` with `CF-Cache-Status: HIT` and `Age`, as recorded above.
