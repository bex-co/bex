# w1 · m64 — Agent-sessions dashboard UI on the target API (ADR047 D9)

DO NOT WORK on this until .pm/w3/m43 is done

**Worker:** worker1 **Goal:** the Devin-shaped session surface — `/agents` list + composer, `/agents/$id` detail with a live conversation column — built **directly on the D9 target API**: control-plane metadata over GraphQL polling, the conversation over `useChat` + the same-origin stream endpoint (`api.bex.co/v1/agent-sessions/{id}/stream`, transcript replay + live tail). No interim transcript synthesis, no client-side prompt persistence — the stream's replay mode is the single history source. **Status:** todo (t001/t003 startable now; t002 mockable; t004 verification hard-gated on `w3/m43`)

## Gating

Control-plane tasks (t001, t003) consume the shipped m39/m41 GraphQL surface and can start now. The conversation column (t002, t004) targets `w3/m43`'s endpoint: t002 may develop against a **mocked v1 UI-message stream** (the wire format is fixed by the shipped driver — `w5/done/035.md`'s head-start clause), but t004's DoD verification requires m43 live. Do not invent transport workarounds (no polling-synthesized transcript, no localStorage prompts — the deleted first-cut approach); if m43 slips, this milestone waits.

## Tasks (in order)

| id   | title                                                                                            | est | depends_on |
| ---- | ------------------------------------------------------------------------------------------------ | --- | ---------- |
| t001 | Feature scaffold: control-plane GraphQL ops + typed hooks (phase-aware polling, attach-ticket)    | 45m | —          |
| t002 | Conversation column: vendor AI Elements + useChat over the same-origin stream (v6 pin, data-acp) | 60m | t001       |
| t003 | /agents route: sessions list, sidebar nav, new-session composer with typed error mapping         | 60m | t001       |
| t004 | /agents/$id detail: metadata header + PR/evidence cards + live conversation + steering            | 60m | t002, t003, w3/m43 |
| t005 | Render parity                                                                                    | 30m | t004       |
| t006 | Simplify                                                                                         | 20m | t005       |
| t007 | Test coverage                                                                                    | 60m | t005       |
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
