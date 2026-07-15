# w7 · m34 — Rate-limit response headers (verify-first)

**Worker:** worker7 **Goal:** Clients can pace themselves without hitting 429s: bex emits whatever rate-limit response headers Render actually sends on ordinary responses (`w7/m3` shipped the limits themselves; today bex only sends `Retry-After` on the 429). Verify-first — the exact header set must be captured from Render's live API/pinned spec, never assumed. **Status:** done

## Tasks (in order)

| id   | title                                                                                                    | est | depends_on |
| ---- | ------------------------------------------------------------------------------------------------------------ | --- | ---------- |
| t001 | Capture Render's actual rate-limit headers (live API + pinned OpenAPI): names, which responses, reset semantics; if none exist, record parity-by-absence and skip to closeout | 30m | —          | — **DONE** |
| t002 | Emit the captured header set from the token-bucket middleware on every authenticated response                  | 40m | t001       | — **SKIPPED** |
| t003 | Middleware tests: presence, monotonic remaining, reset semantics; 429 body re-checked against the `{id,message}` envelope | 20m | t002       | — **SKIPPED** |
| t004 | Render parity — header names/semantics vs the capture; conformance-suite header assertions where the spec declares them | 20m | t003       | — **DONE** |
| t005 | Simplify — `/simplify` over the code this milestone changed                                                   | 15m | t004       | — **SKIPPED** |
| t006 | Test coverage — burst-boundary behavior (headers at 0 remaining vs the 429 itself)                            | 20m | t004       | — **SKIPPED** |
| t007 | Closeout — DoD met → move milestone to `done/`                                                                | 10m | t006       | — **DONE** |

## Definition of done

Either: bex's authenticated responses carry exactly the header set t001 captured, with semantics matching (remaining decrements, reset advances, the 429 keeps `Retry-After` + the `{id,message}` body), held by tests — or: t001 finds Render ships no such headers, and that parity-by-absence finding is recorded in the ledger with evidence, closing the milestone without code.

**DoD met via parity-by-absence path (2026-07-15):** t001 found Render ships no proactive rate-limit headers (`RateLimit-Limit`, `RateLimit-Remaining`, `RateLimit-Reset`, `X-RateLimit-*`) on ordinary responses. Evidence: (1) pinned OpenAPI spec (`lego/backend/internal/api/testdata/render-openapi.json`) has no `headers` component and no `429` response objects in any endpoint; (2) api-docs.render.com/reference/rate-limiting documents only `Retry-After` on 429; (3) ADR006 §Rate limits already captured the same finding. Parity-by-absence recorded in ADR018 §API contract rate-limit row. t002–t003/t005–t006 skipped (no headers to emit or test). t004 satisfied by the ADR018 ledger update.

## Source + Goal linkage

- **Source:** `/pm-brainstorm` round 10 (2026-07-15). Verified in `lego/backend/internal/api/ratelimit.go`: only `Retry-After` on the 429 exists. Render's docs describe client-side pacing against its published budget — the exact response-header contract is what t001 pins.
- **Goal linkage:** Render parity (pillar 1) + agent ergonomics (pillar 3 — MCP/agent clients are exactly the callers that need machine-readable pacing).
- **Expected outcome:** well-behaved clients (including the official CLI and MCP agents) throttle before the 429 instead of after it.
- **Why now:** `w7/m3` is the shipped feature being polished; the verify-first shape caps the cost if Render turns out to ship nothing.
- **Render parity closing task: included** (t004) — recorded as parity-by-absence in ADR018.
