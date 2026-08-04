# w1 · m64 — Agent-sessions dashboard UI on the target API (ADR047 D9)

DO NOT WORK on this until .pm/w3/m43 is done

**Worker:** worker1 **Goal:** the Devin-shaped session surface — `/agents` list + composer, `/agents/$id` detail with a live conversation column — built **directly on the D9 target API**: control-plane metadata over GraphQL polling, the conversation over `useChat` + the same-origin stream endpoint (`api.bex.co/v1/agent-sessions/{id}/stream`, transcript replay + live tail). No interim transcript synthesis, no client-side prompt persistence — the stream's replay mode is the single history source. **Status:** in progress (t001 + t003 **DONE**). t001 — agent-sessions feature scaffold (GraphQL ops + per-feature `definitions.ts` splice, phase-aware polling hooks, typed `AGENT_SESSION_*` error mapping, `AgentSessionView` mapper). t003 — `/agents` route: sessions list (phase chips + PR badges + relative time) + sidebar nav (en/zh) + Devin-style new-session composer (task/repo/branch/agent + Advanced model/endpoint/egress) with inline typed-error mapping + 503 house callout; a minimal placeholder `/agents/$agentSessionId` route keeps links typed until t004 replaces it. `yarn typecheck && yarn lint && yarn test` green (1841 tests). **w3/m43 done ⇒ t004's live gate lifted.** t002 (AI-SDK-v6 conversation column + transport + AI Elements + mock harness), t004 (detail page: header/PR/evidence/conversation/steering), t006 (Simplify — reviewed, already DRY, no changes), t007 (Test coverage — 58 tests across mapper/errors/composer/list/evidence/steering; 1905 total) all **DONE**. **t005 Render parity:** agent sessions are a **bex extension** (Render has no equivalent); the dashboard consumes the same GraphQL surface (agentSessions/agentSession/create/steer/resume/cancel/attach) that mirrors REST/MCP (shipped m39/m41/m43), and the UI exposes it consistently (list + all `agentConfig`/egress create fields, phase/PR/evidence/turns/deliveryMode/failureReason, state-routed steer/cancel). No drift. **Route-nesting bug found + fixed via the live render walk:** the detail route was `agents.$agentSessionId.tsx` (dot ⇒ nested under the Outlet-less `/agents` list route ⇒ rendered the list at `/agents/{id}`, shipped to prod); renamed to the flat `agents_.$agentSessionId.tsx` (repo's opt-out convention, cf. `env-groups_`) — the route-inventory test now pins the filename as the regression tripwire. **Remaining for t008:** the live-substrate browser walk against real m43 needs a logged-in prod session (Playwright can't auth); the full UI incl. detail + graceful degradation is render-verified locally, and the live stream itself is proven by w3/m43's green E2E.

## Gating

Control-plane tasks (t001, t003) consume the shipped m39/m41 GraphQL surface and can start now. The conversation column (t002, t004) targets `w3/m43`'s endpoint: t002 may develop against a **mocked v1 UI-message stream** (the wire format is fixed by the shipped driver — `w5/done/035.md`'s head-start clause), but t004's DoD verification requires m43 live. Do not invent transport workarounds (no polling-synthesized transcript, no localStorage prompts — the deleted first-cut approach); if m43 slips, this milestone waits.

## Tasks (in order)

| id   | title                                                                                            | est | depends_on |
| ---- | ------------------------------------------------------------------------------------------------ | --- | ---------- |
| t001 | Feature scaffold: control-plane GraphQL ops + typed hooks (phase-aware polling, attach-ticket)    | 45m | — — **DONE** |
| t002 | Conversation column: vendor AI Elements + useChat over the same-origin stream (v6 pin, data-acp) | 60m | t001 — **DONE** |
| t003 | /agents route: sessions list, sidebar nav, new-session composer with typed error mapping         | 60m | t001 — **DONE** |
| t004 | /agents/$id detail: metadata header + PR/evidence cards + live conversation + steering            | 60m | t002, t003, w3/m43 — **DONE** (code; live-substrate browser walk pending prod auth) |
| t005 | Render parity                                                                                    | 30m | t004        — **DONE** |
| t006 | Simplify                                                                                         | 20m | t005        — **DONE** |
| t007 | Test coverage                                                                                    | 60m | t005        — **DONE** |
| t008 | Closeout                                                                                         | 10m | t007       |

## Definition of done

- `/agents` lists the workspace's sessions (chip keyed on `phase`, PR badge, relative time) and hosts the composer (`createAgentSession`: repo/branch with `bex-agent/*` guidance, task textarea, agent select, advanced model/endpoint/egress fields); every `AGENT_SESSION_*` code surfaces as an inline human-readable message.
- `/agents/$agentSessionId`: metadata (phase, duration, turns, cancel-with-confirm, draft-PR card, bounded evidence panels, `failureReason`) from GraphQL polling; the conversation column on `useChat` — **terminal session: full transcript via the stream's replay mode + `[DONE]`; running session: replay then live tail**; reconnect re-mints via `attach-ticket` (`prepareReconnectToStreamRequest`); the `data-acp` parts render as collapsible Reasoning/Task/Tool/Terminal groups (the Devin "Worked/Thought" shape) via `dataPartSchemas`.
- Steering: live session ⇒ the composer sends a chat `POST` (a `useChat` `sendMessage`); idle session ⇒ `resume`/steer redispatch path; disabled states carry reasons.
- Honest degraded states: m43 endpoint unavailable/unconfigured ⇒ the conversation column says so while metadata views keep working; stream drop mid-turn reconnects without duplicating parts.
- Same-origin only: the transport hits `api.bex.co` — no CORS configuration, no second origin in dashboard config; the ticket rides a header, never a URL.
- AI SDK pinned to the **v6** line matching the driver (`acp-ai-provider` constraint); no `ai@7` in the lockfile; upgrade only in lockstep with the driver.
- `yarn typecheck && yarn lint && yarn test` green; live walk against m43 on the real substrate recorded.

## Source + Goal linkage

- **Source:** re-materialized 2026-08-02 (user deleted the first cut, which pre-dated the target-API decision) from `w5/035` + ADR047 § D9 "Target API shape" ([docs/ADR047-cloud-coding-agent-sessions.md](../../../docs/ADR047-cloud-coding-agent-sessions.md)); placed in w1 by user direction.
- **Goal linkage:** ADR008 pillar 5 — the human surface of the agent-session product; consumes `w3/m43`'s conversation API exactly as any external AI SDK client would (dogfooding the public contract); feeds ADR048 mobile (`w11/m6`/`m7`).
- **Expected outcome:** assign → watch the live transcript → steer mid-run → open the draft PR, all in the dashboard, with the conversation column running on the same public API an external `useChat` app would use.
- **Why now:** the contract is frozen (§ D9) and m43 is materialized with a clear gate; building the UI against the real contract (mock first, live on m43) avoids ever shipping the discarded interim design. Render parity included: agent sessions are a bex extension exposed consistently across REST/GraphQL/MCP + UI (m39 precedent).
