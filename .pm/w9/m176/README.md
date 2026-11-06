# w9 · m176 — Approval mode: the agent asks before risky actions (needs_input)

**Worker:** worker9 **Goal:** a session can pause in `needs_input` asking the user to approve or answer, and resumes when they reply — on API, MCP (state=needs_input in wait), dashboard and push. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Backend needs_input lifecycle and contracts | 90m | — |
| t002 | Approval policy and bypass | 60m | t001 |
| t003 | Dashboard: answer a pending decision | 60m | t001 |
| t004 | Render parity (Devin reference for agent surfaces) | 25m | t002, t003 |
| t005 | Simplify | 15m | t004 |
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
