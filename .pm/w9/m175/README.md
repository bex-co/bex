# w9 · m175 — Knowledge and Playbooks for cloud agents (Devin parity)

**Worker:** worker9 **Goal:** a workspace can save reusable instructions (knowledge notes and playbooks) that agent sessions apply automatically or on request. **Status:** todo (t001 done)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Design: knowledge notes + playbooks model — **DONE** | 45m | — |
| t002 | CRUD API for notes and playbooks | 90m | t001 |
| t003 | Apply in the prompt overlay | 60m | t002 |
| t004 | Dashboard: manage notes/playbooks, pick a playbook | 60m | t002 |
| t005 | Render parity (Devin reference for agent surfaces) | 25m | t003, t004 |
| t006 | Simplify | 15m | t005 |
| t007 | Test coverage | 45m | t005, t006 |
| t008 | Closeout | 10m | t007 |

## Definition of done

- Workspace skills ([ADR099](../../../docs/ADR099-agent-skills.md)) are manageable on REST/GraphQL/MCP/dashboard, delivered at dispatch as ADR099 D5 documents, and a live session demonstrably follows an explicitly invoked skill (the playbook case) and an automatically applied one (the knowledge case).

## Source + Goal linkage

- **Source:** user request 2026-11-05 ("把 w21 以及需要跟进这些问题移动到 w9 然后你统一跟进解决", `/pm them all`), from the ADR094/ADR095 recap next-steps list.
- **Goal linkage:** ADR008 pillar 5; Devin parity (Knowledge/Playbooks, its core differentiator).
- **Expected outcome:** teams stop re-typing the same instructions every session.
- **Why now:** second-highest Devin gap after secrets.
- **Parity:** included — Devin reference.

## Design decision (2026-11-06, loopx w9, Devin-reference rule)

- **Source:** existing rule (cloud-agent work matches Devin's observable behavior, user preference 2026-10-30) plus verified evidence. Devin deprecated Knowledge and is migrating Playbooks to **Skills** (`SKILL.md`, the Agent Skills format); accounts created from 2026-10-24 have Skills only ([Knowledge](https://docs.devin.ai/product-guides/knowledge), [Playbooks](https://docs.devin.ai/product-guides/creating-playbooks), [Skills](https://docs.devin.ai/product-guides/skills)).
- **Chosen:** one entity, the workspace skill ([ADR099](../../../docs/ADR099-agent-skills.md)), in place of separate notes and playbooks. Knowledge maps to an enabled skill with model triggers; a playbook maps to a skill invoked explicitly through `skills` on create/steer (the `playbook_id` equivalent).
- **Effect on tasks:** t002 builds skill CRUD and `skills` on create/steer (in place of `playbookId`); t003 delivers per ADR099 D5 (files in `/bex/skills`, engine-folder links, an overlay index, and invocation lines); t004 adds a Skills settings page and a composer picker.
