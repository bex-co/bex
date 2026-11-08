# w21 · m280 — Skill version history: every save keeps a version you can read, diff and restore (Devin parity)

**Worker:** worker21 **Goal:** every save of a workspace skill is kept as a version with message, author, time and short id, viewable and diffable on every surface, and restorable. **Status:** todo (t001, t002, t003, t004, t005, t006, t007 done)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Amend ADR099: per-save version history — **DONE** | 30m | — |
| t002 | Version store: write on every save — **DONE** | 75m | t001 |
| t003 | REST, GraphQL and MCP: list and read versions — **DONE** | 60m | t002 |
| t004 | Dashboard: Version history tab — **DONE** | 75m | t003 |
| t005 | Render parity (Devin reference for agent surfaces) — **DONE** | 25m | t004 |
| t006 | Simplify — **DONE** | 15m | t005 |
| t007 | Test coverage — **DONE** | 45m | t005, t006 |
| t008 | Closeout | 10m | t007 |

## Definition of done

- After three saves of a skill the dashboard's Version history tab lists three versions with message, author, time, short id and a Current badge; Changes shows the line diff and Content the full text.
- Load into editor then Save adds a new version; history is never rewritten.
- REST, GraphQL and MCP return the same list and content; unknown skill or version return named errors.
- ADR099 describes the version record and the restore flow.

## Source + Goal linkage

- **Source:** User request 2026-11-07 (`/pm-brainstorm w21`, then `/pm them all to w21`; this explicitly places the items in w21 despite the 2026-11-05 default of following w21 work in w9). Closes the "per-save version history" follow-up in [ADR099](../../docs/ADR099-agent-skills.md). Reference: Devin, verified live in the Devin dashboard on 2026-11-06; docs.devin.ai Creating Playbooks (each save creates a version that can be viewed and restored).
- **Goal linkage:** ADR008 pillar 5 (cloud coding agents); Devin parity. Customers can safely edit instructions that agents load automatically, and recover from a bad edit.
- **Expected outcome:** A broken skill edit can be found and undone in the dashboard without anyone keeping their own copy.
- **Why now:** Skills load into agent sessions automatically, so a bad edit changes what every later session does; history is the only way back. The design is small and settled by Devin's behavior, and w9/m175 has landed the skill store, API and editor it extends.
- **Render parity:** included. This milestone changes REST, GraphQL, MCP and dashboard surfaces for cloud coding agents; the reference product is Devin.
- **Sequence:** none. w9/m175 (skills) is done, so the skill store, API and editor this builds on exist.
