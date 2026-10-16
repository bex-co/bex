# Edge compression attempt — 2026-10-16

Implemented, measured on the deployed proxy (Traefik v3.7.5, Yaegi v0.16.1) and **not shipped**. The full change is [edge-compression-with-guard.patch](edge-compression-with-guard.patch) (operator chain, `bexCompressGuard` local plugin, chart/values wiring, ADR018 note, projection + plugin + actual-proxy tests); it applies to `main` at fa91e4e97 with `git apply`.

## Traefik `compress` alone (allowlisted types, `br`,`gzip`)

[Probe](neighbor-probe.py.txt) against [origin](neighbor-origin.py.txt), every body decoded and compared byte for byte:

- Works: Brotli preferred, q-values honored, identity/`q=0`/empty offers stay identity, JSON/SVG/text compressed (324 300 → 41 B br, 1 893 B gzip), small bodies and images untouched, an app's own `Content-Encoding` passes through, 206 Range untouched, HEAD untouched, WebSocket upgrade works, HTTP/2 works, chunked text/NDJSON/SSE stream flush by flush (no buffering).
- Router byte metrics (the HTTP egress meter) count the compressed bytes actually sent (3 × 41 B for three 324 300 B responses).
- **Diverges:** a `Cache-Control: no-transform` response is compressed, and a strong `ETag` is kept unchanged on the encoded body (an `If-Range` resume against a compliant origin could splice identity bytes onto a compressed prefix). `text/event-stream` is compressed by default unless excluded (the allowlist excludes it).

## Guard plugin (inner/outer phases around `compress`)

Header behavior was exactly right (no-transform held, strong ETag weakened only when the edge encoded, forged markers ignored, nothing leaked). **But it breaks streaming on every guarded route:** Yaegi wraps an interpreted `http.ResponseWriter` passed to compiled code in a composed wrapper that only knows `Hijacker` (`traefik/yaegi@v0.16.1/stdlib/wrapper-composed.go`), so the plugin's `Flush` (and `Unwrap`/`FlushError`) never reach Traefik's proxy. [Timing rig](flush-timing-rig.go.txt) / [results](flush-timing-results.txt): an origin flushing three events 1 s apart reaches the client at 0 s with no middleware or `compress` alone, and at 3 s through either guard phase or the full chain, for both `text/event-stream` and `application/octet-stream`. No plugin-side workaround exists: any plugin that must see response headers has to wrap the writer. (The existing `websocketegress` plugin is unaffected because it passes the original writer for ordinary HTTP.)

## Cost notes (efficiency review, 2 CPUs)

Brotli ≈ 18 ms CPU per 1 MB JS response vs gzip ≈ 5.9 ms (output 13 % smaller), ≈ 105 req/s/2 cores at 1 MB; Traefik's `compress` exposes no level or max-size knob. Production runs one hostNetwork Traefik replica. Listing `gzip` first cuts the CPU ~3× at the cost of Render's Brotli-first preference. The two Yaegi guard wrappers added ≈ 15–25 µs per small response.

## Decision needed

1. **Ship Traefik `compress` alone** (the patch minus the guard: drop `compressguard`, keep the allowlist): streaming-safe, but `no-transform` responses get compressed and strong ETags stay strong — the "bare enable" the finding rejects; record both as ADR018 divergences. Also choose `br`-first (Render parity) or `gzip`-first (CPU).
2. **Fix upstream first**: add a composed `ResponseWriter`+`Flusher` (+`Hijacker`) wrapper to traefik/yaegi (or `no-transform`/ETag options to Traefik's `compress`), wait for a Traefik release, then ship the saved guard design. Needs authorization for an external PR.
3. **Keep identity** (status quo, protocol-valid) and close m83 as won't-fix.
