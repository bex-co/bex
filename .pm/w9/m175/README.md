# w9 · m175 — Knowledge and Playbooks for cloud agents (Devin parity)

**Worker:** worker9 **Goal:** a workspace can save reusable instructions (knowledge notes and playbooks) that agent sessions apply automatically or on request. **Status:** in progress — t001–t006 done; next t007 (CLI `--skill`, tests) and the live checks after deploy

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Design: knowledge notes + playbooks model — **DONE** | 45m | — |
| t002 | CRUD API for notes and playbooks — **DONE** | 90m | t001 |
| t003 | Apply in the prompt overlay — **DONE** | 60m | t002 |
| t004 | Dashboard: manage notes/playbooks, pick a playbook — **DONE** | 60m | t002 |
| t005 | Render parity (Devin reference for agent surfaces) — **DONE** | 25m | t003, t004 |
| t006 | Simplify — **DONE** | 15m | t005 |
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

## CLI follow-through (2026-11-06, from w9/m177 t004)

- When this milestone adds its create/steer field, add `--skill name[:args]` on `bex agent run`/`send` (ADR099 D4) in `lego/cli/internal/agent`, with a contract test, as part of the implementation task.

## CLI `--skill` (2026-11-06)

- `bex agent run|send --skill NAME[:ARGUMENTS]` (repeatable) sends `skills` on create and steer, tested by `TestSkillsReachCreateAndSteer` and documented in `docs/bex-cli.md`. It ships with w9/m176.

## Live check (2026-11-06)

- **Live check attempt 2026-11-06 ~13:10 UTC: NOT RUN.** Production bex-api is still on the 92ff00293 pin (gitops `bab5d417e`). `GET /v1/agent-skills` and `GET /v1/agent-secrets` answer 404, and capabilities has no per-profile `modelKeyReady`. Every deploy run since then (379af6c08, 3310f6484, 8c498d33c) ended `success` with `build`/`deploy` skipped by supersession, so none of this work is live yet. Rerun after a real pin.
