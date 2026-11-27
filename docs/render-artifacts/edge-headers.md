# Render's edge headers: what a web service receives and what its edge answers (w4/m687)

**Captured:** 2026-11-27, 01:58 and 02:40–02:45 UTC, with `curl` 8.7.1 from one client in the US, against public apps on Render: `httpbin.onrender.com` (a web service; the same echo [ADR022](../ADR022-tenant-isolation.md) cites) and the static sites `create-react-app.onrender.com`, `sveltekit-static.onrender.com` and `vue.onrender.com`. Client addresses are left out; `<client>` stands for the address that sent the request.

**Why it matters.** Every Render host sits behind Cloudflare, and Render's docs tell apps to use what Cloudflare adds. [uptime-best-practices](https://render.com/docs/uptime-best-practices) says of `CF-Ray`: "Render includes this header in the request it sends along to your service", and asks apps to log every request with it. The [PocketBase article](https://render.com/articles/host-pocketbase-on-render) sets `settings.trustedProxy.headers = ["CF-Connecting-IP"]` and notes that private-network requests bypass Cloudflare.

## What the web service receives

`GET https://httpbin.onrender.com/headers` with no extra headers. The app received:

| Header | Value seen | Shape |
| --- | --- | --- |
| `cf-connecting-ip` | `<client>` | the client address Cloudflare saw |
| `cf-ipcountry` | `US` | ISO 3166-1 alpha-2 of the client address; Cloudflare's docs use `XX` for unknown and `T1` for Tor |
| `cf-ray` | `a60dc4825e08e9e7-CMH` | 16 lowercase hex characters, `-`, a three-letter data-center code. The app's ray ends in the data center nearest Render's origin (`CMH`); the answer's ray (below) has the same hex and the client's data center (`SJC`) |
| `cf-visitor` | `{"scheme":"https"}` | JSON, the scheme the client used |
| `cdn-loop` | `cloudflare; loops=1` | RFC 8586; see "A caller's values" |
| `true-client-ip` | `<client>` | as `cf-connecting-ip` |
| `x-forwarded-for` | `<client>, <cloudflare edge>, <internal hop>` | a caller's list is kept in front ([ADR022](../ADR022-tenant-isolation.md), w8/402) |
| `x-forwarded-proto` | `https` |  |
| `rndr-id` | `61017a39-c443-476f` | 8-4-4 lowercase hex, also on the answer |
| `x-request-start` | `1795747220911324` | microseconds since the epoch |
| `render-proxy-ttl` | `4` | Render-internal; not documented |
| `accept-encoding` | `gzip, br` | for a `curl` that sent none: the edge normalizes it |

## What Render's edge answers

The same request's answer carried `server: cloudflare`, `cf-ray: a60dc4825e08e9e7-SJC`, `cf-cache-status: DYNAMIC` (the service has no edge caching), `rndr-id`, `x-render-origin-server: uvicorn` and `alt-svc: h3=":443"; ma=86400`.

- A static site answers `server: cloudflare`, `cf-ray` and its CDN's own status: `cf-cache-status: EXPIRED` on a first `GET /`, `HIT` on the next, `BYPASS` on a missing file's 404.
- `http://httpbin.onrender.com/headers` answers `301` with `cf-cache-status: DYNAMIC`, `Server: cloudflare` and `CF-RAY`, and no `rndr-id`. A `POST` over `http://` answers `307` with the same three. A static site's `http://` redirect carries `cf-cache-status: EXPIRED`.

## A caller's values

| Sent by the caller | What happened |
| --- | --- |
| `CF-Connecting-IP: 203.0.113.9` | refused before the app: `403`, `content-type: text/plain; charset=UTF-8`, `content-length: 17`, body `error code: 1000` and a newline (17 bytes), `cache-control: private, max-age=0, no-store, no-cache, must-revalidate, post-check=0, pre-check=0`, `expires: Thu, 01 Jan 1970 00:00:01 GMT`, `referrer-policy: same-origin`, `x-frame-options: SAMEORIGIN`, `server: cloudflare`, `cf-ray`, `alt-svc`. No `rndr-id`: Render's proxy never saw it |
| `CF-Connecting-IP: hello` | the same `403` |
| `CF-Connecting-IP` with an empty value | `200`, reaches the app with Cloudflare's `cf-connecting-ip` |
| `CF-Connecting-IP: 203.0.113.9` over `http://` | the same `403`, before the `301` |
| `CF_Connecting_IP: 203.0.113.9` (underscore) | `200`; the header does not reach the app |
| `CF-Connecting-IPv6: 2001:db8::9` | `200`; the header does not reach the app |
| `CF-IPCountry: KP`, `CF-Ray: 0000000000000000-AAA`, `CF-Visitor: {"scheme":"http"}`, `True-Client-IP: 203.0.113.10` | each overwritten: `US`, a fresh ray, `{"scheme":"https"}`, `<client>` |
| `CDN-Loop: forged` | `forged, cloudflare; loops=1`: Cloudflare keeps the list and adds itself |
| `CDN-Loop: cloudflare; loops=1` | `cloudflare; loops=2` |
| `CDN-Loop: cloudflare` | `cloudflare; loops=1` |
| `CDN-Loop: foo, cloudflare; loops=3` | `foo, cloudflare; loops=4` |
| `CDN-Loop: Cloudflare; loops=2` | `Cloudflare; loops=3` (the caller's case is kept) |
| `CDN-Loop: cloudflare;loops=2;x=y` | `cloudflare; loops=3; x=y` (parameters re-joined with `; `) |
| `CDN-Loop: cloudflare; x=y` | `cloudflare; x=y; loops=1` |
| `CDN-Loop: cloudflare; loops=abc` | `cloudflare; loops=1` |
| `CDN-Loop: cloudflare; loops=2, cloudflare; loops=5` | `cloudflare; loops=3, cloudflare; loops=6` |
| `CDN-Loop: foo;loops=2` | `foo; loops=2, cloudflare; loops=1` |
| `CDN-Loop: cloudflare; loops=15` | `cloudflare; loops=16` |
| `CDN-Loop: cloudflare; loops=16` (or 17–20) | the `403` `error code: 1000` |
| two `CDN-Loop` lines, `a` and `b` | the app read `a, cloudflare; loops=1`; whether Cloudflare or the echo dropped `b` is not known |

## Left for later decisions

- `server: cloudflare` names a vendor bex's edge is not. w4/m687 sends it, as the parity rule asks (ADR018, Services & lifecycle, "Edge headers a web service receives").
- `accept-encoding` normalization: Render's app saw `gzip, br` for a client that sent none. On bex, Traefik's forwarder asks a web service for `gzip` itself when the client sent none, and the static-site route marks such a request for the static server (w8/363).
- `alt-svc: h3` is a signed-off difference ([ADR018 deploys](../ADR018/deploys.md), w8/211).
- `render-proxy-ttl` is undocumented and not reproduced.
- The answer's `cf-ray` ends in the client's data center and the app's in the origin's. bex has one data center, so both end in the same code.
