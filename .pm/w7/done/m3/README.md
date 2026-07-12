# w7 · m3 — bex-api abuse hardening: Render-shaped rate limits + request caps

**Worker:** worker7 **Goal:** bex-api enforces per-caller rate limits and request caps with Render-shaped 429 semantics, so a single tenant, key, or anonymous client can't starve the public API once signup opens. **Status:** done

## Tasks (in order)

| id   | title                                                                                             | est | depends_on |
| ---- | --------------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Capture Render's documented rate-limit contract into docs/ADR006-bex-api.md                                | 30m | —          | — **DONE** |
| t002 | Per-caller token-bucket middleware at the shared mux (REST + GraphQL + MCP)                          | 60m | t001       | — **DONE** |
| t003 | Request caps: body-size limit, log/metrics query-range bounds, SSE connection cap                    | 45m | t002       | — **DONE** |
| t004 | Env knobs for limits, documented + mirrored in `.env.example` / `.env.template`                      | 30m | t002       | — **DONE** |
| t005 | Render parity — 429 shape/semantics consistent across REST · GraphQL · MCP, vs Render's docs; dashboard handles 429 | 30m | t003, t004 | — **DONE** |
| t006 | Simplify — `/simplify` over the code this milestone changed                                          | 20m | t005       | — **DONE** |
| t007 | Test coverage — meaningful tests for limiting, reset, per-caller isolation, caps                     | 30m | t005       | — **DONE** |
| t008 | Closeout — DoD verified, milestone moved to `done/`                                                  | 15m | t007       | — **DONE** |

## Definition of done

A caller exceeding the configured rate gets a Render-shaped **429 + `Retry-After`** on REST, GraphQL, and MCP alike while an under-limit caller is unaffected; two distinct API keys don't share a bucket; oversized bodies and unbounded log/metrics ranges are refused with clear errors; the limits are env-tunable and documented in docs/ADR006-bex-api.md + the CLAUDE.md env table, with `.env.example`/`.env.template` in sync.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w7` (2026-07-09, take 2). Verified 2026-07-09: no 429 handling or rate limiting anywhere in `lego/backend/` — the public API is unmetered. Re-verified 2026-07-11: still true, and api.bex.co is now live behind the rebuilt cluster's LB.
- **Goal linkage:** GOAL.md V0 #7 (security review); pillar 1 (Render-compatible API — Render documents API rate limits and 429 responses, so matching them *is* parity).
- **Expected outcome:** the API surface w1/m9 exposes to open signup can't be trivially DoS'd or monopolized by one caller; agents get a machine-readable back-off signal (`Retry-After`) instead of degraded latency.
- **Why now:** w1/m9 shipped 2026-07-09 — the API is publicly signup-reachable **today**, served at api.bex.co through the LB (`lego/operator/config/api/ingress.yaml`) with **zero rate limiting**. The precondition already fired: this is a live exposure, not a pre-signup prerequisite, and the last open w7 milestone — schedule it next for worker7. _(Updated 2026-07-11 board review; originally written when m9 was still upcoming.)_
- **Render parity: included** — this changes REST/GraphQL/MCP response semantics (429 envelope + headers); t005 checks the three adapters against each other and against Render's documented contract, and that the dashboard degrades gracefully.
