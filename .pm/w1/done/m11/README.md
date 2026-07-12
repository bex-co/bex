# w1 · m11 — Render custom-domains API over `App.spec.hosts[]`

**Worker:** worker1 **Goal:** Expose Render's custom-domains surface as a thin bex-api layer over the existing `App.spec.host`/`spec.hosts[]` (the operator already reconciles Traefik + cert-manager per docs/ADR005-custom-domain.md), with three-adapter parity. **Status:** done

## Tasks (in order)

| id   | title                                                                              | est | depends_on   |
| ---- | ----------------------------------------------------------------------------------- | --- | ------------ |
| t001 | REST custom-domains endpoints, shapes verified against Render's OpenAPI             | 30m | —            | — **DONE** |
| t002 | GraphQL + MCP parity fragments                                                       | 30m | t001         | — **DONE** |
| t003 | Domain verification status from App status / cert Secret state                       | 25m | t001         | — **DONE** |
| t004 | Simplify — `/simplify` over the code this milestone changed                           | 20m | t002, t003   | — **DONE** |
| t005 | Test coverage — meaningful tests for the behavior this milestone shipped              | 30m | t002, t003   | — **DONE** |

## Definition of done

Add a domain via REST → it appears in `spec.hosts[]` → the operator converges Traefik + TLS → list shows it with a truthful verification status → delete removes the host (and its per-host TLS secret reference). GraphQL and MCP return the same data through the same service methods.

## Source + Goal linkage

- **Source:** promoted from inbox `w1/003` (2026-07-08, originally from `/pm-brainstorm more milestones for w5`); mechanism in docs/ADR005-custom-domain.md.
- **Goal linkage:** Render parity, pillar-1 API-first (API must exist before any dashboard surface).
- **Expected outcome:** unblocks the paired dashboard note `w5/004` (custom-domains section in service Settings).
- **Why now:** the mechanism is fully shipped (operator + Traefik + cert-manager); only the API veneer is missing, and the w5 UI half is queued behind it.
