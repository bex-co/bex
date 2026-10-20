# Render rate-limit response headers — capture (w7/m34/t001, superseded by w8/147)

**Captured:** 2026-07-15 **Finding:** parity-by-absence. **Superseded 2026-10-20 (w8/147):** see the correction below.

## Correction (w8/147, 2026-10-20)

The 2026-07-15 conclusion came from an older spec copy and a docs paraphrase. Two later sources contradict it. [api-docs.render.com/reference/rate-limiting](https://api-docs.render.com/reference/rate-limiting) (read 2026-10-20) says: "Three rate limit headers are returned with every request: `Ratelimit-Limit` … `Ratelimit-Remaining` … `Ratelimit-Reset`." The pinned spec `lego/backend/internal/api/openapi/render-public-api-1.json` (2026-09-11) defines `components.responses.429RateLimit` with `Retry-After`, `RateLimit-Limit`, `RateLimit-Remaining` ("Always `0` for this response") and `RateLimit-Reset`. It also states that rate-limited endpoints return the three headers on successful responses.

bex now sends all three on every response its limiters admit, and on 429s alongside `Retry-After` with Remaining `0`. That covers the shared API limiter for REST, GraphQL and MCP, and the deploy-hook limiter (`core.SetRateLimitHeaders`). On bex's token buckets, Limit is the burst, Remaining is the whole tokens left, and Reset is the seconds until the bucket is full. The tighter of the caller and workspace buckets is reported. Render's per-endpoint budgets are not adopted.

The original capture follows, kept as history.

## Evidence

### Pinned OpenAPI spec

`lego/backend/internal/api/testdata/render-openapi.json` (Render Public API — pinned response schemas):

- `components.headers`: empty object (no header components defined)
- No `429` response object on any endpoint (only `200` responses are declared)
- No `headers` field on any response object

### Render API docs reference

Source: `api-docs.render.com/reference/rate-limiting` (cited in ADR006 §Rate limits):

> A caller that exceeds the budget receives HTTP 429 Too Many Requests + `Retry-After: <seconds>` + `{"id":"rate_limited","message":"…"}`

No proactive headers (`RateLimit-Limit`, `RateLimit-Remaining`, `RateLimit-Reset`, `X-RateLimit-*`, or IETF draft combined `RateLimit`) are documented for ordinary (non-429) responses.

### ADR006 corroboration

`docs/ADR006-bex-api.md` §Rate limits (written from the live API source): only `Retry-After` on 429 is documented; no proactive pacing headers are mentioned.

## Conclusion

**Render ships no proactive rate-limit headers on ordinary responses.** The only rate-limit header Render sends is `Retry-After` on 429 responses. bex already matches this behavior exactly (`lego/backend/internal/api/ratelimit.go`).

This is the parity-by-absence finding that closes w7/m34 without code changes. Recorded in ADR018 §API contract rate-limit row.
