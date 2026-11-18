# w3 · m488 — An edge-cached response answers conditional requests with 304 and carries its Age

**Worker:** worker3 **Goal:** turning on edge caching no longer turns an app's 304s into full 200s: a conditional request whose validator matches the cached response gets `304 Not Modified`, on a hit and on the fill alike, and every response answered from the cache carries `Age`. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | A cache hit answers a matching If-None-Match or If-Modified-Since with 304 | 45m | — |
| [t002](t002.md) | A miss or expired fill answers the client's matching conditional with 304 and still stores the full body | 45m | t001 |
| [t003](t003.md) | Every hit and 304 carries Age | 20m | t001 |
| [t004](t004.md) | Record it in ADR004, ship and verify on production | 30m | t002, t003 |
| [t005](t005.md) | Simplify | 15m | t004 |
| [t006](t006.md) | Test coverage | 30m | t004 |
| [t007](t007.md) | Closeout | 10m | t005, t006 |

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
