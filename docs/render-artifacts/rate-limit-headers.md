# Render rate-limit response headers — capture (w7/m34/t001)

**Captured:** 2026-07-15 **Finding:** parity-by-absence

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
