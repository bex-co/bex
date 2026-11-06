# w9 · m176 — Approval mode: the agent asks before risky actions (needs_input)

**Worker:** worker9 **Goal:** a session can pause in `needs_input` asking the user to approve or answer, and resumes when they reply — on API, MCP (state=needs_input in wait), dashboard and push. **Status:** in progress — t001–t003, t005 done; next t004 parity, then the live pass

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Backend needs_input lifecycle and contracts — **DONE** | 90m | — |
| t002 | Approval policy and bypass — **DONE** | 60m | t001 |
| t003 | Dashboard: answer a pending decision — **DONE** | 60m | t001 |
| t004 | Render parity (Devin reference for agent surfaces) | 25m | t002, t003 |
| t005 | Simplify — **DONE** | 15m | t004 |
| t006 | Test coverage | 45m | t004, t005 |
| t007 | Closeout | 10m | t006 |

## Definition of done

- A live session pauses for approval before a risky action, appears as needs_input on REST/GraphQL/MCP/wait, and continues after an approve/answer from the dashboard; bypassApproval skips it.

## Source + Goal linkage

- **Source:** user request 2026-11-05 ("把 w21 以及需要跟进这些问题移动到 w9 然后你统一跟进解决", `/pm them all`), from the ADR094/ADR095 recap next-steps list.
- **Goal linkage:** ADR008 pillar 5; ADR094 D7 needs_input; Devin parity (approval mode).
- **Expected outcome:** customers can let agents run autonomously but gate risky steps.
- **Why now:** `needs_input` is reserved in the shipped contract but never emitted; w11/m7 waits on it.
- **Parity:** included — Devin reference.

## Design decision (2026-11-06, loopx w9, Devin-reference rule)

- **Source:** existing rule (cloud-agent work matches Devin) plus verified Devin behavior. v3 `status_detail` has `waiting_for_user` and `waiting_for_approval` ("awaiting action approval in safe mode"); create takes `bypass_approval`; Devin's permission model asks before destructive, publishing, cloud-CLI and outside-workspace actions ([Get Session](https://docs.devin.ai/api-reference/v3/sessions/get-organizations-session), [Create Session](https://docs.devin.ai/api-reference/v3/sessions/post-organizations-sessions), [permissions](https://docs.devin.ai/cli/reference/permissions)).
- **Chosen:** [ADR100](../../../docs/ADR100-agent-approval-mode.md).
  - `needs_input` gets a `waitingFor` field (`approval` or `user`), backed by durable `agent_session_decisions` with a 24h expiry.
  - Workspace `approvalMode` is `autonomous` (default, unchanged behavior), `risky` or `all`, plus `bypassApproval` on create.
  - The driver gates ACP `request_permission` against one risky-action table, and an `ask_user` tool covers questions.
- **Effect on tasks:**
  - t001 implements D1, D2 and D5 (backend and driver channel).
  - t002 implements D3 and D4 (policy, bypass, `ask_user`).
  - t003 implements D6 (dashboard callout and Blocked group).
  - w11/m7 t003 can consume the D2 answer route.

## CLI follow-through (2026-11-06, from w9/m177 t004)

- When this milestone adds its create/steer field, add `--approval-mode` and `--bypass-approval` on `bex agent run` (ADR100 D3) in `lego/cli/internal/agent`, with a contract test, as part of the implementation task.
