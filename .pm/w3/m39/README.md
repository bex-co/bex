# w3 · m39 — Agent-sessions API: lifecycle, OpenFGA, tickets (ADR047 D3)

**Worker:** worker3 **Goal:** `POST /v1/agent-sessions` (+ list/get/cancel, GraphQL/MCP twins) creates an authorized agent session — OpenFGA-checked, recorded in the control plane, wired to sandbox create/resume, with an HMAC session ticket minted on the web-shell pattern for the coming attach path. **Status:** todo

## Tasks (in order)

| id   | title                                                                                    | est | depends_on |
| ---- | ----------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Control-plane `agent_sessions` table + store                                              | 45m | —          |
| t002 | OpenFGA first-class agent-session tuples                                                  | 30m | t001       |
| t003 | REST `POST/GET /v1/agent-sessions` + cancel, GraphQL/MCP twins, sandbox create/resume wiring | 90m | t002    |
| t004 | HMAC session ticket mint (web-shell pattern: DB nonce, subject+pod+workspace claims)      | 45m | t003       |
| t005 | Render parity: cross-surface consistency (REST/GraphQL/MCP fields, errors, `ea` naming)   | 30m | t004       |
| t006 | Simplify pass over the session feature                                                    | 20m | t005       |
| t007 | Test coverage: authz, lifecycle, ticket claims, cross-workspace isolation                 | 45m | t005       |
| t008 | Closeout                                                                                  | 10m | t007       |

## Definition of done

- `agent_sessions` control-plane table (id minted via `id.New` with a new registered kind, ADR020) records session lifecycle: workspace, repo/branch target, agent config, sandbox id, phase/status, timestamps.
- Sessions are first-class in the OpenFGA model (gap 6 — replacing the code-level-only sandbox authz pattern); create/get/cancel check `can_operate` (or the modeled equivalent) and cross-workspace access is denied by tuples, not code.
- `POST /v1/agent-sessions` creates (or resumes) the session's sandbox through the existing lifecycle client with reserved-metadata stamping; list/get/cancel work; GraphQL and MCP twins expose the same fields and error shapes.
- Session create mints an HMAC ticket on the `BEX_SHELL_TICKET_SECRET` design (DB-backed single-use nonce for long-lived streams, short expiry, claims binding subject + sandbox pod + workspace) and returns the gateway origin — consumed by the wave-2 attach path; bex-api never touches the sandbox network path.
- Backend suite + lint green.

## Source + Goal linkage

- **Source:** [docs/ADR047-cloud-coding-agent-sessions.md](../../../docs/ADR047-cloud-coding-agent-sessions.md) D3 (API paragraph) + gap 6; `/pm-brainstorm` decomposition 2026-08-01. Precedents: sandbox surface (w3/m32 `/v1/sandboxes*`), web-shell ticket (w2/m55, ADR035).
- **Goal linkage:** pillar 5 — the tenant-facing entry point for cloud coding-agent sessions across bex's API surfaces.
- **Expected outcome:** an authorized tenant (or agent over MCP) can create, inspect, and cancel an agent session; unauthorized and cross-workspace calls are denied at the FGA layer.
- **Why now:** ADR047 wave 1, parallel with m37/m38/m40 — integrates with the driver only at contract level (ticket + sandbox template selection). Render parity included (t005): Render has no coding-agent product, so parity here means bex's own cross-surface consistency (same fields/semantics/error dialect across REST/GraphQL/MCP) and `ea`-style early-access naming per the w3/m32 sandbox precedent — divergences from that pattern get flagged, not silently shipped.
