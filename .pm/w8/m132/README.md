# w8 · m132 — Static sites answer gzip-only clients with gzip, as on Render

**Worker:** worker8 **Goal:** A static-site asset requested with `Accept-Encoding: gzip` (no `br`) is served gzip-compressed, as Render serves it. Brotli stays preferred when offered. **Status:** todo

## Tasks (in order)

| id   | title                                                                 | est | depends_on                 |
| ---- | --------------------------------------------------------------------- | --- | -------------------------- |
| t001 | Negotiate br then gzip in the static server, one variant cache/budget  | 50m | —                          |
| t002 | Neighbors: q-values, ranges, HEAD, ETag, authored encodings, no-transform | 30m | w8/m132/t001               |
| t003 | Render parity: ADR029/ADR018 and every static serving caller          | 15m | w8/m132/t002               |
| t004 | Simplify the changed code                                             | 15m | w8/m132/t003               |
| t005 | Test coverage: negotiation table, decode-exact, budget fallback       | 30m | w8/m132/t003               |
| t006 | Close out the milestone                                               | 30m | w8/m132/t004, w8/m132/t005 |

## Definition of done

Live on production after the release pipeline deploys the operator (static server), on a fresh owned Free static site whose build writes a ≥ 100 kB `big.js`, deleted by ID with 404 proofs:

- `curl -H 'Accept-Encoding: gzip' …/big.js` → `content-encoding: gzip`, `vary: Accept-Encoding`, body decodes to the exact original bytes and is smaller than identity. Same for `gzip, deflate`.
- `br` and `gzip, br` → `content-encoding: br` (unchanged). `gzip;q=0` / `identity` / `deflate` / `zstd` → identity.
- `If-None-Match` with the gzip response's ETag → 304; a `Range` request on a gzip response is satisfiable against the encoded bytes, as it is for br today.
- Bodies under the minimum size and non-compressible types stay identity.

## Source + Goal linkage

- **Source:** `/qa-find-bugs-cli` loop sweep 64af, 2026-10-23 UTC: bex v0.3.3, pin Render CLI v2.27.0 `a764810a7682`, `https://api.bex.co/v1/`, `bex-canary` (`tea-daif693dqjvc73e7as3g`), human device login. Evidence below.
- **Goal linkage:** Render parity first ([ADR008](../../../docs/ADR008-vision.md#render-parity-first)); static sites ([ADR029](../../../docs/ADR029-static-sites.md)). w8/m79 shipped Render's _documented_ Brotli only. Render's live static sites also gzip (captured below), and many real clients advertise gzip without br: Go `net/http`'s transparent decompression, Python `requests`/`urllib3`, Java `HttpClient`, older proxies and CDNs.
- **Expected outcome:** those clients download ~3× fewer bytes for JS/CSS/HTML, matching Render.
- **Why now:** minor (performance/parity, protocol-valid identity). The serving path, cache and budget seams from m79 make this a contained change; the web-service equivalent is blocked separately (w8/m83). Render parity task included: tenant-facing static responses.

## Evidence (sweep 64af)

Fixture `srv-dbdbssfvlr1c73fj1av0` (`qa-20261022-64af-site`, free static site, `--repo https://github.com/bex-co/bex --branch main --publish-directory out`, build command writes `out/big.js` of 300 000 bytes and a few HTML files), created with `bex services create --type static_site … --confirm -o json` → exit 0, deploy `live`.

bex:

```text
Accept-Encoding    status  content-encoding  bytes
br                 200     br                13
gzip               200     (none)            300000
gzip, deflate      200     (none)            300000
deflate / zstd     200     (none)            300000
```

Every response carried `vary: Accept-Encoding`; `If-None-Match` → 304 (control).

Render (public example static site `https://create-react-app.onrender.com/static/js/1.13eeb203.chunk.js`, `server: cloudflare`, `rndr-id` present), 2026-10-23:

```text
Accept-Encoding    content-encoding  bytes
identity           (none)            119346
gzip               gzip              37744
gzip, deflate      gzip              37744
br                 br                38440
deflate / zstd     (none)            119346
```

`hugo.onrender.com`, `jekyll.onrender.com`, `gatsby.onrender.com`, `vue.onrender.com` also answer `/` with `content-encoding: gzip` for `Accept-Encoding: gzip`.

## Root cause

`lego/operator/internal/staticserver/staticserver.go:494-520` (`serveObject`) offers only a Brotli variant: `acceptsBrotli` (`encoding.go:116`) is the sole negotiation, `brotliVariant` (`encoding.go:178`) the sole encoder and cache (`brotliVariantKey`, `encoding.go:72`). A gzip-only offer falls to identity.

## Fix (target)

- Replace `acceptsBrotli` with `negotiateCoding(header) string` returning `br`, `gzip` or `""` (br preferred at equal q; explicit q-values and `*` honored; malformed q = 0, as today).
- Add a gzip variant beside br (stdlib `compress/gzip`, level ~6), cached under its own variant key, leased from the same `encodeLease`/`liveBodies` budgets, served through the same `brotliResponseWriter`-style length wrapper (rename it coding-neutral), with `weakenETag`.
- Keep m79's rules: `Cache-Control: no-transform`, authored `Content-Encoding`, size bounds and type eligibility apply to both codings.

## Blast radius

`serveObject` is the one encoding point; its callers (direct file, directory index, rewrite targets and the new 404 page) inherit it. No REST/GraphQL/MCP/dashboard field. Every workspace.

## Unverified

- Render's exact size thresholds and type list for gzip (Cloudflare-side). The target reuses m79's.

## Dedupe

Searched `.pm` open and done for gzip/compression/Accept-Encoding: w8/m79 (done, Brotli only by scope), w8/m83 (blocked, web services), w9/m61 and w9/044 (bex-api gzip). No item covers static gzip.
