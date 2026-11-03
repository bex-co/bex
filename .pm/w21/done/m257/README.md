# w21 · m257 — Cloud Agents MCP phase 1: contract hygiene — honest descriptions, `state`, `outcome`

**Worker:** worker21 **Goal:** The agent MCP tools stop lying, an LLM caller can read 'is it done / what happened' from one compact field, and the contract is pinned against drift. **Status:** done (2026-11-03)

## Tasks (in order)

| id | title | est | depends_on |
| -- | ----- | --- | ---------- |
| | t001 | Fix spawn_/resume_ descriptions and pin every agent tool with a golden test — **DONE** | 45m | — |
| | t002 | Derived `state` and `steerable` on every session view (REST, GraphQL, MCP) — **DONE** | 1.5h | — |
| | t003 | `outcome` block: summary, pullRequest, deliveryNote, failureReasonCode — **DONE** | 1.5h | t002 |
| | t004 | Canonical `create_agent_session` / `send_agent_session_message` names with aliases — **DONE** | 45m | t001 |
| | t005 | Render parity (vs Devin): REST, GraphQL, MCP and dashboard consistency — **DONE** | 30m | t001, t002, t003, t004 |
| | t006 | Simplify — **DONE** | 20m | t005 |
| | t007 | Test coverage — **DONE** | 45m | t005 |
| | t008 | Closeout — **DONE** | 10m | t006, t007 |

## Definition of done

On REST, GraphQL and MCP every agent-session view carries `state`, `steerable` and `outcome` (summary, pullRequest, deliveryNote, failureReasonCode); `create_agent_session`/`send_agent_session_message` exist with `spawn_`/`steer_` aliases; no description claims a ticket it does not return; a golden test pins all agent tool metadata. Verified by `cd lego/backend && go test ./internal/agentsessions/... ./internal/api/...`.

## Source + Goal linkage

- **Source:** [ADR094](../../../../docs/ADR094-coding-agent-mcp.md) D2, D7, D10 phase 1 (user-requested design 2026-10-30; naming decision 2026-11-02).
- **Goal linkage:** Pillar: AI-native coding agents (ADR008) and Devin-reference parity for cloud agent surfaces (ADR047).
- **Expected outcome:** MCP callers and the dashboard/mobile app read one server-computed answer instead of decoding `phase` client-side.
- **Why now:** Phases 2–4 build on `state`/`outcome`; fixing the description drift first is cheap and stops misleading clients immediately. Phase 1 needs no engine changes.
- **Render parity:** included as a closing task, checked against Devin (the reference product for cloud coding-agent surfaces, ADR047), because this milestone changes REST/GraphQL/MCP/dashboard-facing behavior.

## Completion record

- Shipped 2026-11-03: honest `spawn_`/`resume_` descriptions + golden test (`agentsessions/mcp_golden_test.go`), `state`/`steerable`/`outcome` on REST/GraphQL/MCP (`models.go`, `outcome.go`, `graphql.go`), `create_agent_session`/`send_agent_session_message` with aliases, `AgentSessionTurnTail` store read, `agenttranscript.ClipHeadTail`.
- Parity (Devin): REST, GraphQL, MCP identical (`TestEverySurfaceServesStateAndOutcome`); `outcome.pullRequest` omits `draft` (no stored source; ADR094 sketch only). Dashboard still decodes `phase` — filed as `w21/001`.
