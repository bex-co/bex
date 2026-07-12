# w8 · m2 — Usage API: month-to-date usage over REST · GraphQL · MCP

**Worker:** worker8 **Goal:** m1's recorded usage becomes a workspace-scoped product surface: one core `Usage` verb (OpenFGA-authorized) with three thin adapters — a GraphQL dashboard query (companion style, like the existing `monthToDateBandwidth`), `GET /v1/usage` REST, and an MCP `get_usage` tool so agents can reason about their own consumption. One core, thin adapters — the `docs/ADR006-bex-api.md` pattern. **Status:** DONE 2026-07-09

## Tasks (in order)

| id   | title                                                                                                         | est | depends_on       |
| ---- | --------------------------------------------------------------------------------------------------------------- | --- | ---------------- |
| t001 | Core `Usage` verb: month-to-date per workspace with per-service/kind breakdown, workspace-scoped OpenFGA authz | 45m | —                | — **DONE** |
| t002 | GraphQL adapter: dashboard `usage` query alongside the `monthToDateBandwidth` companion                          | 30m | t001             | — **DONE** |
| t003 | REST adapter: `GET /v1/usage?ownerId=…&period=…` (documented bex extension — see parity note)                    | 30m | t001             | — **DONE** |
| t004 | MCP `get_usage` tool (per-feature MCP registration pattern)                                                      | 30m | t001             | — **DONE** |
| t005 | Docs: `docs/ADR023-usage-metering.md` + CLAUDE.md docs index + `docs/ADR018-render-parity.md` bex-extras row                   | 30m | t002, t003, t004 | — **DONE** |
| t006 | Render parity: same fields/semantics/errors across REST · GraphQL · MCP; declared drift only                     | 30m | t005             | — **DONE** |
| t007 | Simplify: `/simplify` over the code this milestone changed                                                       | 30m | t006             | — **DONE** |
| t008 | Test coverage: adapter-consistency + authz-denial tests                                                          | 45m | t006             | — **DONE** |
| t009 | Closeout: verify DoD, mark done, move milestone to `done/`                                                       | 15m | t008             | — **DONE** |

## Definition of done

`GET /v1/usage`, the GraphQL `usage` query, and MCP `get_usage` all return the same month-to-date quantities (instance-seconds by tier · egress bytes · build seconds) for the caller's workspace; a caller from another workspace is denied once OpenFGA is enforced; the surface is documented in `docs/ADR023-usage-metering.md` and listed in `docs/ADR018-render-parity.md`.

## Source + Goal linkage

- **Source:** `/pm-brainstorm w8` 2026-07-09 (same provenance as m1).
- **Goal linkage:** V0 roadmap item 5; pillar 1 (API-first — "no dashboard-only features, ever") and pillar 3 (agents as operators — an agent that deploys should be able to check what it consumes).
- **Expected outcome:** any client — curl, the dashboard, or an MCP agent — can read a workspace's month-to-date usage and get identical numbers.
- **Why now:** m1's rows are write-only until a surface exists; m3 (dashboard page) needs this API first.
- **Render parity: included** (t006) — with one **pre-declared drift**: Render's public REST API has no billing/usage endpoints (billing is dashboard-only; the w1/m13 parity audit against Render's OpenAPI found none, hence no ledger row). The REST + MCP surface is a deliberate bex extension of the same class as the ledger's "API-key management over the API" extra; the GraphQL shape stays dashboard-consistent where Render's shapes are capturable. t006 verifies cross-adapter consistency and that the extension is documented as such, not silent.
