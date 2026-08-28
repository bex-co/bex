# w4 · m89 — Honest agent-session states and a real error taxonomy

**Worker:** worker4 **Goal:** a tenant can tell "this platform was never set up" from "something is temporarily down" from "your workspace is missing a model key" — and a session that is merely starting no longer reads as broken **Status:** todo

## Tasks (in order)

| id   | title                                                                            | est | depends_on       |
| ---- | -------------------------------------------------------------------------------- | --- | ---------------- |
| t001 | Split `ErrAgentSessionsUnavailable` into distinct coded reasons                   | 1h  | —                |
| t002 | Map each code to specific dashboard copy                                         | 45m | t001             |
| t003 | Model conversation states as not-started / connecting / live / broken            | 45m | —                |
| t004 | Surface capability gaps as pre-flight, not a post-hoc 503                        | 30m | t002             |
| t005 | Render parity — coded errors identical across REST/GraphQL/MCP/UI                | 30m | t002, t003, t004 |
| t006 | Simplify the code this milestone changed                                         | 30m | t005             |
| t007 | Test coverage for the error taxonomy and the state machine                       | 40m | t005             |
| t008 | Closeout                                                                         | 15m | t007             |

## Definition of done

- A **transient dependency failure** and a **genuinely unconfigured platform** produce visibly different messages in the composer. Today both render "Agent sessions aren't configured — ask your operator to configure the agent-session gateway."
- No failure-shaped copy appears while a session is in its normal `Creating` phase. Today the composer says "The conversation stream is unavailable, so live steering is paused" while the banner says "Starting the sandbox…".
- The steering control never says "Sending…" for a turn the user did not send.
- A workspace missing a model key or a GitHub connection is told **before** it starts a session, using the existing `capabilities` projection, not by a 503 afterwards.
- Coded errors stay identical across REST, GraphQL and MCP, asserted by the existing cross-surface tests.

## Source + Goal linkage

- **Source:** `/pm-brainstorm` 2026-08-28, findings 3 and 4 — hit live in `dev-1`, where a transient etcd timeout produced a 503 and the UI reported the platform had never been configured. `dashboard/src/features/agent-sessions/components/new-session-composer.tsx:171` maps **any** `AgentSessionsUnavailableError` to that one alert, and `lego/backend/internal/agentsessions/service.go` has **17** `ErrAgentSessionsUnavailable` return sites covering causes as different as "gateway not configured", "OpenFGA unavailable", "snapshot store unavailable" and "read model key failed".
- **Goal linkage:** pillar 5 (ADR008) and the ADR006 Render-compatible coded-error dialect, which agent sessions already follow for `AGENT_SESSION_INPUT_INVALID` / `_NOT_FOUND` / `_NOT_RESUMABLE`.
- **Expected outcome:** support burden drops and users stop being sent to an operator for faults that clear by themselves. An operator reading a report can tell which of the 17 causes fired.
- **Why now:** the collapse is actively misleading rather than merely terse, and it gets worse as pillar 5 opens up — every new dependency adds another cause to the same undifferentiated 503. Cheaper to split now, while the call sites are all in one service.
- **Render parity:** **included.** Render has no agent-session product (`docs/ADR018-render-parity.md`), so the parity task's job is bex's own cross-surface discipline: the new codes must appear identically on REST, GraphQL and MCP, in Render's error dialect.
