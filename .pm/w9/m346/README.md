# w9 · m346 — Coordinator tools: a parent session waits for, messages and cancels its children from inside its sandbox

**Worker:** worker9 **Goal:** a running parent session coordinates the children it spawned without polling, as Devin's coordinator does: it waits until they finish, sends one a follow-up, and cancels one, and it can touch nothing outside its own tree. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | Gateway: a parent's Pod can wait on, message and cancel its own descendants | 90m | — |
| [t002](t002.md) | Driver: `wait_child_sessions`, `message_child_session` and `cancel_child_session` tools, and the overlay line that says when to use them | 45m | t001 |
| [t003](t003.md) | ADR102: record the coordinator tools and the wait bound | 20m | t002 |
| [t004](t004.md) | Live check on production: a parent spawns two children, waits for both, messages one and cancels the other | 45m | t002 |
| [t005](t005.md) | Render parity (Devin): coordinator tools match Devin's coordinator behavior and the session views agree on every surface | 20m | t003, t004 |
| [t006](t006.md) | Simplify | 15m | t005 |
| [t007](t007.md) | Test coverage | 45m | t005, t006 |
| [t008](t008.md) | Closeout | 10m | t007 |

## Definition of done

On production:

- A parent session calls `wait_child_sessions` on two children and the call returns when both have ended, with their phases, PRs and costs, without the agent polling `get_child_session`.
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
