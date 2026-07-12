# w1 · m13 — Render parity audit: REST · GraphQL · MCP · UI matrix

**Worker:** worker1 **Goal:** A systematic, evidence-based check of how far bex actually matches render.com — one pass per surface (public REST vs Render's OpenAPI, GraphQL vs Render's dashboard operations, MCP vs Render's official server, dashboard UI vs Render's dashboard IA) — synthesized into a living parity matrix (`docs/ADR018-render-parity.md`) with every gap filed as a sized inbox note. **Status:** done (2026-07-08) — **DONE**

## Tasks (in order)

| id   | title                                                                            | est | depends_on         | status        |
| ---- | --------------------------------------------------------------------------------- | --- | ------------------ | ------------- |
| t001 | REST parity: bex `/v1/*` vs Render's OpenAPI spec, endpoint-by-endpoint            | 30m | —                  | — **DONE**    |
| t002 | GraphQL + MCP parity: dashboard operations + `render-oss/render-mcp-server` tools  | 30m | —                  | — **DONE**    |
| t003 | UI parity: `dashboard/` vs Render's dashboard IA, page-by-page                     | 30m | —                  | — **DONE**    |
| t004 | Synthesize `docs/ADR018-render-parity.md` matrix; file each gap as a sized inbox note     | 30m | t001, t002, t003   | — **DONE**    |
| t005 | Simplify — `/simplify` over anything this milestone changed                        | 20m | t004               | — **DONE**    |
| t006 | Test coverage — tests for any code shipped by the audit (else close n/a)           | 20m | t004               | — **DONE** (n/a) |

## Definition of done

`docs/ADR018-render-parity.md` exists: one row per Render capability, columns REST / GraphQL / MCP / UI, each cell ✅ (parity, with evidence pointer) / ◐ (partial, divergence documented) / ✖ (missing) / — (deliberate non-goal, with rationale). Every ✖/◐ worth doing has a filed inbox note (or a pointer to the existing milestone that covers it); the matrix links its evidence (OpenAPI spec, captures in `docs/render-artifacts/`, the official MCP repo).

## Source + Goal linkage

- **Source:** user request during `/pm` materialization 2026-07-08 ("a task about checking render.com feature parity including MCP/REST/GraphQL APIs parity and UI parity"); builds on the 2026-07-08 docs-vs-code audit.
- **Goal linkage:** pillar 1 (Render-compatible surfaces) is bex's core compatibility promise — this makes "compatible" measurable instead of asserted.
- **Expected outcome:** a single truthful map of parity that ranks the remaining Render-parity backlog (feeds w1/w5 planning); undocumented divergences become documented decisions.
- **Why now:** parity claims are spread across docs and were audit-corrected today; several parity milestones (m11 custom domains, m12 scale) are queued — the matrix orders that queue by real gaps instead of guesses.
