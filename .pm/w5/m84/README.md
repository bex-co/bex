# w5 · m84 — Agent-context continuity across sandbox generations (resume / steer / redispatch)

**Worker:** worker5 **Goal:** a resumed or steered agent session remembers its conversation — the fresh agent process is primed per the ADR047 D3 continuity ladder instead of cold-starting while the dashboard replays history it doesn't have. **Status:** todo (t001–t005 done 2026-08-30; t006 live E2E awaits ship + rollout of the platform AND agent-sandbox images)

## Tasks (in order)

| id   | title                                                                                  | est | depends_on |
| ---- | -------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Driver: transcript re-priming + task re-delivery (ladder rungs 2–3) — **DONE**                     | 60m | —          |
| t002 | Rung 1: snapshot carries agent session-state dirs + ACP `session/load` when advertised — **DONE**  | 60m | t001       |
| t003 | Surface the applied rung: turn annotation + dashboard restored/fresh-context hint — **DONE**       | 45m | t001       |
| t004 | Simplify — `/simplify` over the changed code — **DONE**                                            | 30m | t002, t003 |
| t005 | Test coverage — ladder selection, preamble bounds, no double-render, empty-snapshot resume — **DONE** | 45m | t004       |
| t006 | Live E2E: production resume + steer-redispatch answer a context-dependent prompt        | 45m | t005       |
| t007 | Closeout                                                                                | 15m | t006       |

## Definition of done

On production: (a) a hibernated session with real prior turns is resumed and a context-dependent steer ("continue where you left off") is answered **in context**; (b) a steer that redispatches onto a fresh sandbox retains context the same way; (c) resuming a session whose prior turns all failed during setup (the `ags-da9mh5vj596c73en5eq0` shape) re-delivers `agent_config.task` — a bare "try again" retries the task instead of asking what to retry; (d) the dashboard indicates restored-vs-fresh agent context. Unit tests pin ladder selection, the preamble byte budget, and that re-primed context is never double-rendered in replay.

## Source + Goal linkage

- **Source:** user report 2026-08-30 — post-fix resume of `ags-da9mh5vj596c73en5eq0` (dashboard.bex.co/agents/…): agent answered "I need more context. What should I try again?" beneath a fully replayed history. Root: ADR047 D3's continuity promise (`session/load` + "transcript replay is the universal fallback") was never implemented agent-side — replay reaches only the client; ADR059 D3's snapshot scope omitted agent session-state dirs; the setup-failed-resume case was unspecified. ADR047/D3, ADR059/D3+D4, ADR051 amendments (2026-08-30) now specify the contract this milestone implements.
- **Goal linkage:** pillar 5 agent-session product quality; direct follow-on to the m82/m83 reliability arc (those made sessions *run*; this makes multi-turn sessions *coherent*).
- **Expected outcome:** resume/steer behaves like the continuous conversation the UI presents; the "amnesia beneath replayed history" class is gone.
- **Why now:** every hibernate→resume and every redispatch hits this today; it defeats the purpose of hibernation (durable resume) and of the durable transcript.
- **Render parity:** omitted — bex-native agent-session surface (ADR047); Render has no comparable product surface. The t003 hint is a stream/data-part annotation, not an API schema change.
