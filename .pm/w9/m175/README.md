# w9 · m175 — Knowledge and Playbooks for cloud agents (Devin parity)

**Worker:** worker9 **Goal:** a workspace can save reusable instructions (knowledge notes and playbooks) that agent sessions apply automatically or on request. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Design: knowledge notes + playbooks model | 45m | — |
| t002 | CRUD API for notes and playbooks | 90m | t001 |
| t003 | Apply in the prompt overlay | 60m | t002 |
| t004 | Dashboard: manage notes/playbooks, pick a playbook | 60m | t002 |
| t005 | Render parity (Devin reference for agent surfaces) | 25m | t003, t004 |
| t006 | Simplify | 15m | t005 |
| t007 | Test coverage | 45m | t005, t006 |
| t008 | Closeout | 10m | t007 |

## Definition of done

- Notes and playbooks are manageable on REST/GraphQL/MCP/dashboard, applied at dispatch in the documented order, and a live session demonstrably follows a playbook.

## Source + Goal linkage

- **Source:** user request 2026-11-05 ("把 w21 以及需要跟进这些问题移动到 w9 然后你统一跟进解决", `/pm them all`), from the ADR094/ADR095 recap next-steps list.
- **Goal linkage:** ADR008 pillar 5; Devin parity (Knowledge/Playbooks, its core differentiator).
- **Expected outcome:** teams stop re-typing the same instructions every session.
- **Why now:** second-highest Devin gap after secrets.
- **Parity:** included — Devin reference.
