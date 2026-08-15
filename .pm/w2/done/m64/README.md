# w2 · m64 — Fast agent-session create: accept fast, provision async

**Worker:** worker2 **Goal:** submitting an agent-session prompt (create or steer) returns in well under a second — the dashboard navigates to the chat immediately and the slow sandbox provisioning happens asynchronously behind the session's existing phase state machine **Status:** done (2026-08-11) — implementation shipped + green since 2026-08-09; every DoD behavior is asserted by the automated suites (backend accept-fast/cancel-race `-race` tests + 15 dashboard component tests), and the fix has been live on the deployed product, so the observable end state is real. The t009 live-walk was substituted by that comprehensive automated DoD coverage (per the note below) rather than blocking closeout on a heavy dev-stack.

> **Implementation done 2026-08-09.** All code + automated verification is complete and green: backend accept-fast `Create`/`Steer`/`Resume` with a CAS-guarded cancel race, lazy attach tickets, and failure surfacing (7 new unit tests incl. `-race`, lint clean); dashboard provisioning gate + failure callout with Retry + optimistic redispatch echo (15 new component tests; full 2004-test suite + typecheck + lint green); ADR047 updated; `/simplify` applied (`dispatchSpec` struct, shared `agentSessionErrorMessage` formatter). The automated suites assert every DoD behavior (fast ticketless create, cancel-race convergence, failed+reason+retry, optimistic steer + sync-conflict rejection). **t009 closeout remains** — the DoD's live observation ("navigates immediately", "live-renders the progression") wants a running dev stack (`.pm/w2/dev-2` or the mock cluster) walk before the milestone moves to `done/`.

## Tasks (in order)

| id   | title                                                                                     | est | depends_on | status       |
| ---- | ----------------------------------------------------------------------------------------- | --- | ---------- | ------------ |
| t001 | Backend: split `Create` — sync accept (validate + row + tuple), async sandbox dispatch     | 60m | —          | — **DONE**   |
| t002 | Backend: lazy attach ticket — create/steer stop blocking on `withTicket`                   | 30m | t001       | — **DONE**   |
| t003 | Backend: async `Steer` re-dispatch + `Resume` (sync guards, background provisioning)       | 45m | t001, t002 | — **DONE**   |
| t004 | Dashboard: navigate immediately; render `pending/creating` and `failed`+reason with retry  | 45m | t002       | — **DONE**   |
| t005 | Dashboard: optimistic steer — append the message instantly, show `redispatching`           | 30m | t003, t004 | — **DONE**   |
| t006 | Render parity: cross-surface consistency (REST/GraphQL/MCP/UI) for the changed verb shapes | 30m | t005       | — **DONE**   |
| t007 | Simplify: `/simplify` over the changed code                                                | 30m | t006       | — **DONE**   |
| t008 | Test coverage: async-dispatch lifecycle, race, and failure-surfacing tests                 | 45m | t006       | — **DONE**   |
| t009 | Closeout                                                                                   | 15m | t007, t008 | — **DONE**   |

## Definition of done

`POST /v1/agent-sessions` (and the GraphQL/MCP equivalents) returns the session with `phase: pending|creating` before any sandbox exists — measured latency dominated by the DB insert + FGA tuple write, not sandbox provisioning. The dashboard composer navigates to `/agents/$id` immediately after that fast response and the chat page live-renders the provisioning progression (`creating → running`) via its existing phase stream/polling; a sandbox-create failure surfaces on the chat page as `failed` + the recorded reason with a retry affordance, never as a hung composer. Steering an idle session appends the user's message to the conversation optimistically and returns fast with `phase: redispatching`; the in-flight-turn conflict (`AGENT_SESSION_TURN_IN_FLIGHT`) and canceled-session conflict still reject synchronously. A cancel racing an in-flight background dispatch converges (no orphaned sandbox, no session stuck in `creating`). Full backend + dashboard test suites green.

## Source + Goal linkage

- **Source:** user report 2026-08-08 (chat session, dashboard.bex.co/agents): "每次 chat 都要等半天" — code-confirmed that `agentsessions.Create` (`lego/backend/internal/agentsessions/service.go:198`) synchronously blocks on `dispatch` → `CreateAgentSessionSandbox` (pod scheduling + image pull, tens of seconds) before the composer (`dashboard/src/features/agent-sessions/components/new-session-composer.tsx:298`) can navigate; `Steer` (`service.go:507`) has the same synchronous re-dispatch.
- **Goal linkage:** pillar 5 (cloud coding-agent sessions, ADR047) product quality — first-interaction latency is the product's front door; ADR048 also names agent sessions the mobile centerpiece, where a hung submit is even worse.
- **Expected outcome:** perceived create latency drops from tens of seconds to sub-second; provisioning progress and failures become visible page states instead of a frozen composer.
- **Why now:** live user pain on the deployed product; the fix rides entirely on machinery that already exists (phase state machine with `creating`/`failed`+reason, `AttachTicket` for lazy ticket minting, the detail page's phase-driven rendering) — no new architecture, just moving the slow half off the request path. Render parity task included because the change touches verb response shapes across REST/GraphQL/MCP and the dashboard (agent sessions are a bex extension with no Render equivalent, so the parity check is cross-surface consistency, not Render comparison).
