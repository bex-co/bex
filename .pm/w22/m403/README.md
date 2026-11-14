# w22 · m403 — A message sent to a running agent session is queued, and Stop ends only the current turn, as on Devin

**Worker:** worker22 **Goal:** a user can send a message to an agent session while its turn is running and it is queued, not refused. Queued messages can be edited, removed, reordered or sent now, and they run in order as the turn settles. A new "Stop turn" ends only the running turn and leaves the session alive and steerable with the reply streamed so far kept; Cancel is unchanged. Dashboard, REST, GraphQL, MCP and `bex agent chat` all use the one server-side queue. **Status:** todo (t001 done)

## Tasks (in order)

| id | title | est | depends_on |
| ---- | ----------------------------------------------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Design record in ADR047 and a driver spike: the queue's rules, the interrupt's semantics, whether the driver and each engine can end only the current turn — **DONE** | 45m | — |
| t002 | The queue: store, service and API: a steer during a turn answers "queued"; list, edit, delete, reorder; REST, GraphQL and MCP | 75m | t001 |
| t003 | The dispatcher: when a turn settles the next queued message runs through the existing steer path; safe across restarts and replicas | 60m | t002 |
| t004 | The interrupt verb: ends only the current turn, the session returns to idle with its reply and branch kept; REST, GraphQL, MCP and the gateway-to-driver path | 60m | t001 |
| t005 | Dashboard: the composer works during a turn, queued-message chips (edit, remove, reorder, send now) and "Stop turn" beside Cancel | 90m | t002, t004 |
| t006 | `bex agent chat`: the server queue replaces the client's one-slot queue | 30m | t002 |
| t007 | Render parity (Devin) | 20m | t003, t005, t006 |
| t008 | Simplify | 20m | t007 |
| t009 | Test coverage | 30m | t007 |
| t010 | Closeout | 10m | t008, t009 |

## Definition of done

- Sending a message to a session whose turn is creating, running, resuming or redispatching is accepted and queued on REST, GraphQL and MCP. `AGENT_SESSION_TURN_IN_FLIGHT` is no longer returned for it. A full queue answers a coded 409 `AGENT_SESSION_QUEUE_FULL`, and a canceled or archived session still refuses with its existing codes.
- Queued messages run one per turn in order after the current turn settles (completed, failed or interrupted), each through the existing steer path, so the spend cap, the live-sandbox limit, the branch rules and the prompt-size quota apply at dispatch. A message blocked at dispatch stays queued and shows why. The queue survives a bex-api restart and two replicas never dispatch one message twice (real-Postgres tests).
- A user who can operate the session can list, edit, delete, reorder a queued message and send one now (interrupt the running turn, then run it next). The queue is cleared when the session is canceled, archived or deleted.
- "Stop turn" (REST, GraphQL, MCP and the dashboard) ends only the current turn: the session returns to idle and stays steerable, what the assistant had already streamed stays in the transcript, commits already pushed stay on the branch, and queued messages are kept. Cancel keeps its current meaning (terminal).
- The composer accepts input while a turn runs; queued messages appear as chips; `bex agent chat` sends to the server queue instead of holding one message in the client.
- Every path has an automated test, and a live run on `dev-N` is recorded in this README.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w22` (2026-11-14), filed by `/pmx all to w22`.
  - **bex today:** `lego/backend/internal/agentsessions/service.go` (~lines 1818–1820) answers a steer on a session in `creating`/`running`/`resuming`/`redispatching` with 409 `AGENT_SESSION_TURN_IN_FLIGHT` ("a turn is already running; wait for it to finish before steering"), and the store-level race (`ErrAgentSessionTurnState`, ~line 2126) does the same. `dashboard/src/features/agent-sessions/components/steering-composer.tsx` (~lines 60–76) disables the composer while a turn is live. `bex agent chat` queues one message in the client (ADR106 D2). Cancel is terminal (`AGENT_SESSION_NOT_STEERABLE`).
  - **bex's own design record names the gap:** ADR047 § D9 lists a "mid-run-capable steering composer" among the table stakes held to, and records the public live-turn path as unavailable (`AGENT_SESSION_LIVE_TURN_UNAVAILABLE`).
  - **Reusable parts:** ADR047 § D11 (w9/m344) already queues PR feedback durably per session and delivers it as a steering turn once the session is idle (`lego/backend/internal/agentsessions/pr_feedback.go`: poll, lease, batch; table `agent_session_feedback`, migration 0237). The driver's turn runner takes an external abort signal (`lego/agent-image/driver/src/session.ts`, ~lines 211–226, "agent turn terminated"); whether that reaches a live turn and leaves the session usable is t001's spike.
  - **Devin** ([docs.devin.ai/release-notes/2026](https://docs.devin.ai/release-notes/2026), entries read 2026-11-14): 2026-06-05 "You can now send messages to a session that is waiting for capacity"; 2026-07-17 "You can now set a preference to automatically queue messages while Devin is working"; 2026-08-21 "Press Cmd/Ctrl+Shift+Backspace to stop Devin while it is working"; 2026-08-26 "pressing Cmd/Ctrl+Enter while editing a queued message sends it immediately"; 2026-09-09 "Messages queued in a session are no longer lost when the session goes idle and resumes"; 2026-10-14 interrupting Devin "the part of the reply you already saw stays in the chat instead of disappearing"; 2026-10-16 "Queued messages can be sent, edited, removed or reordered after Devin stops or the session goes to sleep"; 2026-11-06 (Side Chats) "send to interrupt it or queue to run after it finishes".
- **Goal linkage:** ADR008's AI-native thesis and pillar 5 (agent sessions); Devin is the reference product for cloud coding agents (AGENTS.md, ADR047).
- **Expected outcome:** a person can add to or correct a running agent without waiting for the turn to end or killing the session, which today they can only do by cancelling.
- **Why now:** `w9/m344` just shipped the mechanism that starts a new turn on an existing session from an outside event and a durable per-session queue to model on; a user message is a second source for the same loop. The mid-run steering composer is the one item on ADR047's own table-stakes list that is not delivered.
- **Anti-goal check:** `#APPROVAL-MODE` forbids approval gating, human-in-the-loop pauses and an agent-to-user question channel. This is the opposite direction (user to agent), gates nothing and pauses nothing, so it does not conflict.
- **Open recommendations, not user decisions** (t001 records them in ADR047 as recommendations): queue by default with interrupt as an explicit action, because bex has no live injection into a running turn; a per-session cap on pending messages; FIFO one message per turn. Where Devin's default differs (its queueing is a preference, 2026-07-17), t001 records bex's choice in ADR047 and ADR018 as a divergence awaiting the user's sign-off. That sign-off is open and does not block closing the milestone: the repository rule is that parity work is never parked on an optional decision.
- **Render parity included (reference product: Devin):** new fields and verbs on REST, GraphQL and MCP plus the dashboard and CLI, so t007 compares each with Devin.

## Progress 2026-11-14 (/loopx w22)

- **t001 done:** the design is [ADR047 D12](../../../docs/ADR047-cloud-coding-agent-sessions.md#d12--a-message-to-a-running-session-is-queued-stop-turn-ends-only-the-current-turn-w22m403-2026-11-14), with the Devin entries re-read on 2026-11-14 (two of the dates in the original proposal were paraphrased wrongly and are corrected here and in the ADR: queueing while Devin works is a preference, 2026-07-17; `Cmd/Ctrl+Enter` sends a queued message at once, 2026-08-26). Read-only code mapping found that every steer starts a fresh sandbox and driver, that the driver already has a per-turn `AbortController` wired only to SIGTERM, that there is no bex-api-to-driver push channel (the gateway exec path that reads the status file is the cheapest one), and that no phase constraint needs to change (an interrupted turn finalizes as `completed` with the turn flagged `interrupted`). The spike is by reading; the live check of the abort is t004's.
- **ADR018** records the divergence awaiting the user's sign-off (queue by default, where Devin's queueing is a preference). It does not block this milestone.
