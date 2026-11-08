# w21 · m279 — Agent child sessions: an agent can spawn and track child sessions (Devin parity)

**Worker:** worker21 **Goal:** a running agent can start child sessions through a controlled path, their lineage, spend and cancellation are tracked, and the dashboard shows the tree. **Status:** todo (t001, t002, t003, t004, t005, t006, t008, t009, t010 done)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | ADR102: parent and child sessions — **DONE** | 60m | — |
| t002 | Lineage schema and store — **DONE** | 60m | t001 |
| t003 | Create with a parent: limits, spend roll-up, cancel cascade — **DONE** | 90m | t002 |
| t004 | Spawn from inside the sandbox (through ssh-gateway) — **DONE** | 90m | t003 |
| t005 | REST, GraphQL and MCP: lineage and children — **DONE** | 60m | t003 |
| t006 | Dashboard: child cards, Agents panel, sidebar tree — **DONE** | 90m | t005 |
| t007 | Live end-to-end on production as the QA user | 45m | t004, t006 |
| t008 | Render parity (Devin reference for agent surfaces) — **DONE** | 25m | t007 |
| t009 | Simplify — **DONE** | 15m | t008 |
| t010 | Test coverage — **DONE** | 45m | t008, t009 |
| t011 | Closeout | 10m | t010 |

## Definition of done

- On production, a parent session spawns two children; each child records `parentSessionId`; the dashboard shows child cards, a status-grouped Agents panel and nested sidebar rows; the children's spend rolls up into the parent's total; cancelling the parent cancels both.
- A child cannot exceed the parent's remaining spend ceiling, the depth or fan-out limit (also under concurrent inserts), or take secrets the parent does not have.
- A sandbox cannot spawn using another session's token and holds no bex API credential.
- ADR102 is accepted and ADR094's deferral points at it.

## Source + Goal linkage

- **Source:** User request 2026-11-07 (`/pm-brainstorm w21`, then `/pm them all to w21`; this explicitly places the items in w21 despite the 2026-11-05 default of following w21 work in w9). Closes the deferral in [ADR094](../../docs/ADR094-coding-agent-mcp.md) ("Parent and child sessions (Devin `parent_session_id`)"). Reference: Devin, verified live in the Devin dashboard on 2026-11-06; docs.devin.ai advanced capabilities.
- **Goal linkage:** ADR008 pillar 5 (cloud coding agents); Devin parity. Customers can let an agent split a large task across child sessions and still see and bound the whole tree.
- **Expected outcome:** Larger tasks can be decomposed by the agent itself, with one place to see progress and one ceiling on cost.
- **Why now:** ADR094 deferred it as the one structural gap left after the MCP phases. Secrets (w9/m174) and structured results (ADR094 phase 3) have shipped, so the pieces a child needs exist; the Automations milestone (m278) later benefits from the same lineage for runs that spawn children.
- **Render parity:** included. This milestone changes REST, GraphQL, MCP and dashboard surfaces for cloud coding agents; the reference product is Devin.
- **Sequence:** t001 (the ADR) can start at once. Start t002 and t003 after w9/m179 (removing approval mode) lands, because it edits the same session state machine and store code.
- **Approval:** none. Cloud sessions run unattended (`.pm/DO_NOT_DO.md` #APPROVAL-MODE), so a child starts without asking anyone; Devin's "Allow auto-start of child sessions" switch (default on) has no bex counterpart, and that is not a divergence to sign off, it is the existing anti-goal.
