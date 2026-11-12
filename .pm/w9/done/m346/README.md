# w9 · m346 — Coordinator tools: a parent session waits for, messages and cancels its children from inside its sandbox

**Worker:** worker9 **Goal:** a running parent session coordinates the children it spawned without polling, as Devin's coordinator does: it waits until they finish, sends one a follow-up, and cancels one, and it can touch nothing outside its own tree. **Status:** done (2026-11-12)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Gateway: a parent's Pod can wait on, message and cancel its own descendants — **DONE** | 90m | — |
| [t002](done/t002.md) | Driver: `wait_child_sessions`, `message_child_session` and `cancel_child_session` tools, and the overlay line that says when to use them — **DONE** | 45m | t001 |
| [t003](done/t003.md) | ADR102: record the coordinator tools and the wait bound — **DONE** | 20m | t002 |
| [t004](done/t004.md) | Live check on production: a parent spawns two children, waits for both, messages one and cancels the other — **DONE** | 45m | t002 |
| [t005](done/t005.md) | Render parity (Devin): coordinator tools match Devin's coordinator behavior and the session views agree on every surface — **DONE** | 20m | t003, t004 |
| [t006](done/t006.md) | Simplify — **DONE** | 15m | t005 |
| [t007](done/t007.md) | Test coverage — **DONE** | 45m | t005, t006 |
| [t008](done/t008.md) | Closeout — **DONE** | 10m | t007 |

## Definition of done

On production:

- A parent session calls `wait_child_sessions` on two children and the call returns when both have ended, with each one's state, without the agent polling `get_child_session`; one `get_child_session` per settled child then reads its outcome. (Amended 2026-11-12 by t001: the wait stays under bex-api's 30 s write timeout and answers state only.)
- `message_child_session` starts a new turn on a running child; `cancel_child_session` cancels a child.
- Every call naming a session outside the caller's tree is refused, and a message is refused when the session creator has lost `can_create`.
- ADR102 records the tools in place of its 'Not built' row.

## Source + Goal linkage

- **Source:** `/pm-brainstorm more for w9` (2026-11-11), item 5, filed by `/pmx` 2026-11-12.
  - `docs/ADR102-agent-child-sessions.md:156`: the coordinator wake on child finish is '**Not built.** … Follow-up'; the in-sandbox `bex` server offers only `spawn_session` and `get_child_session` (`lego/agent-image/driver/src/mcp-bex.ts:29-31`, checked at `3dd698375`).
  - Devin's coordinator messages running children, terminates them and is woken when one finishes (observed live 2026-11-06, ADR102:155-156).
- **Goal linkage:** pillar 5 (cloud coding agents), Devin parity for child sessions (ADR102).
- **Expected outcome:** fan-out tasks stop spending turns and tokens polling children, and a parent can correct or stop a child that goes wrong.
- **Why now:** child sessions are live; this is the last unbuilt row of ADR102's Devin comparison, and the gateway and `Wait` pieces it needs already exist.
- **Out of scope:** waking a parent because a child 'needs input' — forbidden by [DO_NOT_DO #APPROVAL-MODE](../../DO_NOT_DO.md).
- **Parity:** included (t005), against Devin, because the change adds agent-facing tools and must keep the session tree consistent across REST, GraphQL, MCP and the dashboard.
