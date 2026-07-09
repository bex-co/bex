# w4 · m13 — API-key hygiene: token TTL + key metadata

**Worker:** worker4 **Goal:** Protect bex's ahead-of-Render API-key surface with basic hygiene: Hydra's `client_credentials` access-token TTL deliberately tuned (shorter is safer now that keys re-mint freely), and keys carrying `created-by` + `last-used` metadata — recorded off the hot path, surfaced on every list surface and the m8 dashboard page. The prerequisite for any future stale-key/rotation policy. **Status:** todo

## Tasks (in order)

| id   | title                                                                                          | est | depends_on |
| ---- | ------------------------------------------------------------------------------------------------ | --- | ---------- |
| t001 | Hydra `client_credentials` TTL review/tune in `hydra.values.yaml`; decision in docs/auth.md      | 25m | —          |
| t002 | Metadata writes: `created-by` (subject from `api.IdentityFrom`) at mint; throttled `last-used` on introspection (at-most-once-per-minute per key, async — never a store write per request) | 30m | —          |
| t003 | Surface metadata: REST/GraphQL/MCP `api-keys` list shapes + created/last-used columns on the m8 dashboard list | 25m | t002       |
| t004 | Render parity — cross-surface consistency; comparison target is bex's documented divergence (Render has no key API — matrix §bex-ahead) | 15m | t001, t003 |
| t005 | Simplify — `/simplify` over the code this milestone changed                                      | 20m | t004       |
| t006 | Test coverage — meaningful tests for the metadata + TTL behavior                                 | 30m | t004       |
| t007 | Closeout — DoD met → move milestone to `done/`                                                   | 10m | t006       |

## Definition of done

A freshly minted key shows who created it; using a key updates `last-used` within a minute without adding a synchronous store write to the request path; the TTL change is live (introspected token expiry reflects it) and its rationale recorded in docs/auth.md; REST, GraphQL, MCP, and the dashboard all show the same metadata.

## Source + Goal linkage

- **Source:** promotion of inbox `w4/004` (its original plan — ride along with m8 t001/t002 — didn't happen; m8 shipped without it); `/pm-brainstorm for w4` 2026-07-09.
- **Goal linkage:** roadmap #7 (security review) — credential hygiene evidence; protects the API-key surface docs/render-parity.md lists as ahead-of-Render.
- **Expected outcome:** stale keys become identifiable (`last-used`), key provenance auditable (`created-by`), token lifetimes deliberate instead of default.
- **Why now:** the note was orphaned by m8's shipping; `last-used` is the prerequisite for any rotation policy and cheapest while the apikeys feature is fresh.
- **Render parity closing task: included** (REST/GraphQL/MCP/UI list shapes change) — with the explicit caveat that Render mints keys dashboard-only, so the comparison verifies bex's own cross-surface consistency + documented divergence, not a Render shape.
