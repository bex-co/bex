# Browsers get zstd, not gzip, from the gzip-only web-service edge compression

Why: w8/m83 configures Traefik `compress` with `encodings: [gzip]`, and ADR018 says web responses are gzip. But any client that offers both gzip and zstd gets `Content-Encoding: zstd`. That includes every current Chrome, Edge and Firefox (`gzip, deflate, br, zstd`). Render's edge serves Brotli/gzip, and zstd is not one of the three ADR018 differences the user signed off on (w8/227). The production encoding therefore differs from both Render and our own record, and nobody has measured it: streaming flush, `ETag`/`If-Range` and the non-Range path were all measured with gzip.

Severity: **minor** (bodies decode byte-exact; browsers that send zstd can decode it). Attribution: operator edge config (`lego/operator/internal/controller/edge_compression.go:49`) on Traefik v3.7.5. Estimate: ~45m. Source: `/qa-find-bugs-cli` loop sweep 3093, 2026-11-01 07:25 UTC, pin `a9c66371978a` (contains `a15472735`), workspace `bex-canary`.

## Evidence

Fresh Free web service `qa-20261101-3093-gz` (`srv-dbjegt9mkpvc73fi38t0`, `render-examples/express-hello-world`, 1678 B `text/html`, the app itself does not compress):

```text
Accept-Encoding                 → status  bytes  Content-Encoding
identity                        → 200     1678   —
gzip                            → 200      710   gzip   (vary: Accept-Encoding)
br                              → 200     1678   —
zstd                            → 200     1678   —
deflate                         → 200     1678   —
gzip, zstd                      → 200      761   zstd
gzip, deflate, br, zstd         → 200      761   zstd   (Chrome/Firefox default offer)
br, gzip                        → 200      710   gzip
gzip;q=1, zstd;q=0.1            → 200      710   gzip
```

`curl --compressed` (zstd) and identity bodies hash the same (`35c5531384ba…`). zstd alone is refused, but zstd is chosen whenever gzip is also acceptable. The m83 attempt notes already saw this ("an offer naming `zstd` still gets zstd even with `encodings: [br, gzip]` (the gzip library prefers it)", `blocked/m83/evidence/edge-compression-attempt/README.md:11`). The shipped comment and ADR018 entry dropped it.

## Fix

- Make the edge emit only gzip. Two ways: strip `zstd` from `Accept-Encoding` before `compress` (a `headers`/plugin step scoped to the compress chain), or pin a Traefik release or option whose encoding negotiation honours `encodings`. Then add a `edge_compression_traefik_test.go` case: `gzip, deflate, br, zstd` → gzip, decoded byte-exact, streaming flushed.
- Or, if zstd is kept, record it in ADR018 as a fourth signed-off difference (user decision), and extend the measured matrix (streaming flush, `If-Range`, `206`) to zstd.

## Acceptance

On a fresh Free web service, `Accept-Encoding: gzip, deflate, br, zstd` returns `Content-Encoding: gzip` (or the recorded decision), with the body decoded byte-exact. The `zstd`-only, `br`-only and `identity` offers stay identity.

Dedupe: searched `.pm` and `docs` for `zstd`. m132 (static sites, done) refuses zstd correctly. The open w8/227 covers only no-transform, strong ETag and Brotli. m83 (blocked on t006 live acceptance) does not list zstd.
