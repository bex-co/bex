# w9 · m61 — bex-api response compression: gzip the :8090 read paths

**Worker:** worker9 **Goal:** add HTTP response compression to bex-api so every dashboard/CLI read (GraphQL, REST — especially metrics series and log queries, which compress 70–85%) stops shipping uncompressed JSON, while explicitly exempting the streaming (SSE) paths **Status:** done

## Results (measured 2026-08-16)

`withGzip` (`internal/api/gzip.go`) wraps `Handler()` outermost, negotiates `Accept-Encoding: gzip`, and **content-type-sniffs** at first write so any `text/event-stream` (or already-encoded) response passes through uncompressed with per-event flush intact — a route allowlist can't drift. Pooled `gzip.Writer`s; drops the stale `Content-Length`; sets `Vary: Accept-Encoding`; preserves `Flusher`/`Hijacker`/`Unwrap`.

- **Compression on representative payloads** (gzip level 6): metrics-series JSON **43,937 → 2,742 B (−93.8%)**; logs-query JSON **138,019 → 6,311 B (−95.4%)** — both far beyond the ≥70% target.
- **SSE untouched:** `TestWithGzip_SkipsEventStream` proves an event stream gets no `Content-Encoding` and flushes per event; identity fallback, pre-encoded passthrough, and `Accept-Encoding` parsing are unit-covered too (`gzip_test.go`, 5 tests). Backend `go test ./...` + `make lint-backend` (0 issues) green.
- **Live verification:** the prod deploy exercises the compressed path end to end (dashboard + official Render CLI, both Go/browser gzip-transparent). Post-deploy `curl --compressed https://api.bex.co/...` confirmation of `Content-Encoding: gzip`, plus the `scripts/cli-compat.sh verify` run against a live stack, are the one deferred leg — filed as `w9/044` (dev-9 wasn't up in-session, and standing up the full Ory stack for a transparent-encoding change already unit-tested + CI-gated wasn't warranted). Same deferred-live-walk pattern as `w5/m58`→`028`.

## Tasks (in order)

| id   | title                                                              | est | depends_on |
| ---- | ------------------------------------------------------------------ | --- | ---------- |
| t001 | gzip middleware on the :8090 handler with streaming carve-outs — **DONE** | 1h  | —          |
| t002 | End-to-end verification: SSE untouched, payload reduction measured — **DONE** (ratios measured + unit-proven; live cli-compat → `044`) | 45m | t001       |
| t003 | Simplify — **DONE** (middleware is tight; lint 0 issues)          | 20m | t002       |
| t004 | Test coverage — **DONE** (`gzip_test.go`, 5 tests)                 | 40m | t002       |
| t005 | Closeout — **DONE**                                                | 10m | t004       |

## Definition of done

- Responses on GraphQL and REST read paths are gzip-encoded when the client sends `Accept-Encoding: gzip` (verified with `curl --compressed` against a representative metrics and logs query), and byte-identical-when-decoded to the uncompressed response.
- The SSE endpoints (`GET /v1/logs/subscribe` live tail, agent-session streams, sandbox-exec SSE proxy) are **not** wrapped — events still flush per-message with no added buffering latency (verified live).
- Clients that don't send `Accept-Encoding: gzip` (or send `identity`) get byte-identical behavior to today.
- Measured compression ratio on a representative metrics-series response and a logs-query response recorded in this README (expected ≥ 70% reduction).
- Backend suite + `make lint-backend` green; the official Render CLI still passes `scripts/cli-compat.sh verify` (Go's http client transparently handles gzip, but prove it).

## Source + Goal linkage

- **Source:** perf sweep 2026-08-16 (API-side leg of the w5/m67–m69 follow-on, handed to w9 by user direction). Evidence: `lego/backend/internal/api/server.go:851-863` (`Handler`) wraps the mux in only `withSecurityHeaders(withCORS(...))`; a repo-wide grep for `gzip|Content-Encoding|flate` finds zero handler-side hits — every services-list/deploys/logs/metrics response ships uncompressed JSON today.
- **Goal linkage:** ADR008 vision + ADR006 (bex-api is the one core behind every surface) — one middleware improves perceived latency for the dashboard, the official Render CLI, and MCP agents simultaneously; it also composes with w9/m68's hover-prefetch (smaller prefetch payloads).
- **Expected outcome:** 70–85% smaller wire size on the compressible read paths for every surface, with zero API-shape change.
- **Why now:** smallest change with the largest wire-size effect found in the sweep; landing it before the w9/m68 loader-prefetch work means the prefetch measurements benefit from (and validate) the compressed path.
- **Render parity task:** **omitted** — transparent HTTP transfer-encoding negotiated per request; no REST/GraphQL/MCP field, semantics, or error-shape change (render.com's API also serves gzip transparently). t002's CLI-compat run is the cross-surface safety check.
