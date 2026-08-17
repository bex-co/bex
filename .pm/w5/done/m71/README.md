# w5 · m71 — Agent-session conversation persistence closure

**Worker:** worker5 **Goal:** every accepted agent-session prompt and every recoverable assistant part survives refresh, reconnect, redispatch, hibernation, partial live tee, and sandbox teardown with explicit completeness semantics **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Persistence audit + contract amendments — **DONE** | 45m | — |
| t002 | Durable turn intents and legacy initial-prompt backfill — **DONE** | 60m | t001 |
| t003 | Per-turn transcript identity and partial-harvest merge — **DONE** | 60m | t002 |
| t004 | Lossless bounded harvest + explicit completeness state — **DONE** | 45m | t003 |
| t005 | Role-correct replay and dashboard durable user messages — **DONE** | 60m | t002, t003 |
| t006 | Rehydrate/turn-accounting closure — **DONE** | 45m | t002 |
| t007 | Render parity — **DONE** | 30m | t005, t006 |
| t008 | Simplify — **DONE** | 30m | t003–t007 |
| t009 | Test coverage — **DONE** | 60m | t007, t008 |
| t010 | Closeout — **DONE** | 20m | t007, t009 |

## Definition of done

1. An initial task or accepted follow-up prompt is a durable turn record before asynchronous sandbox work begins; reloading a terminal session shows every recoverable user message in chronological order without relying on React optimistic state.
2. Transcript parts are idempotent by `(session, turn, part index)`. A new sandbox may restart its local part ordinal at zero without colliding with prior turns, and completion merges a missing suffix after a partial live tee instead of skipping the turn.
3. The headless recorder can harvest the driver's full bounded 16 MiB log. Any driver-log, part-count, byte-quota, read, or parse truncation is persisted and surfaced instead of replaying a silently complete-looking `[DONE]`.
4. Resume-without-prompt restores a hibernated workspace with an empty `BEX_AGENT_PROMPT` and does not increment `turns`; hibernated Steer persists and runs exactly one new prompt. The unsupported/racy live-POST turn path cannot accept non-durable work.
5. REST, GraphQL, MCP, and the gateway replay expose the same durable turn/completeness facts. Dashboard refresh/reconnect renders durable user prompts and assistant output in turn order; rejected prompts do not persist.
6. Legacy sessions recover turn 1 from `agent_config.task` when available and explicitly leave unrecoverable historical follow-up prompts absent rather than inventing content.
7. Focused migration/store/service/completer/gateway/dashboard regressions and the backend, agent-image, and dashboard gates are green; touched Markdown is prettier-clean.

## Source + Goal linkage

- **Source:** user report on 2026-08-17 that `ags-da1m1ah0htvc73dtrkug` did not show the submitted chat message, followed by an explicit request to deep-research every agent-session persistence loophole, hand the fix to `$pm` in w5, and implement it.
- **Observed production failure:** the accepted turn reached the agent and produced a second assistant transcript/PR update, but the prompt existed only in the browser's optimistic `useChat` state and `BEX_AGENT_PROMPT`; the terminal-state reconciliation hid that bubble because it incorrectly assumed the durable transcript contained the user role.
- **Goal linkage:** ADR008's AI-native control plane requires an agent conversation to be durable product data, not a best-effort view over a disposable sandbox. ADR047/ADR051/ADR059/ADR065 already promise replay after teardown and hibernation; this milestone closes the implementation gaps behind those promises.
- **Expected outcome:** refresh, reconnect, redispatch, hibernate/rehydrate, partial tee, and quota truncation all have deterministic, testable conversation outcomes.
- **Why now:** this is active production data loss. It also invalidates the existing w3/m43 acceptance text (“per-turn prompt recoverable from transcript alone”) and w2/m64's optimistic-echo reconciliation assumption.
- **Render parity task included:** the conversation/history behavior is user-facing. Compare Render's durable operation/event-history expectations and document the deliberate Bex extension (role-correct AI conversation plus explicit incomplete-history state).
- **Out of scope:** reconstructing already-lost historical follow-up prompt text, changing agent providers, retaining deleted sessions, or making the transcript quota unbounded.
