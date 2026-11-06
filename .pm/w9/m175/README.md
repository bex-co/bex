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

## Live check (2026-11-06, production 24a9203ee)

- Production pin 24a9203ee (gitops `d65dc2f2b`). An authenticated `GET /v1/agent-skills` and `GET /v1/agent-secrets` answered 200. QA user via CLI device login, workspace `bex` (QA user is an admin there). Every fixture was deleted and verified 404; the CLI grant was revoked (204, then 401) and the Kratos session logged out.
- **PASS — CRUD on every surface.**
  - REST create: `qa-api-echo` (`skl-dbmu1427qbhc7397040g`, triggers `[user]`) and `qa-api-codeword` (`skl-dbmu14e6kvbs738fl5qg`, defaults `[user, model]`), both 201.
  - The REST list carries metadata only (no body), and the REST get by name carries body and content.
  - GraphQL `agentSkills { skills { name triggers enabled } }` and MCP `list_agent_skills`/`get_agent_skill` (with `workspaceId`) return both.
- **PASS — explicit invocation (the playbook case).** Session `ags-dbmu1c27qbhc7397046g`, created with `skills: [{name: qa-api-echo, arguments: "42"}]`, replied exactly `SKILL-OK-42`. Its transcript turn records `skills: [{name: qa-api-echo, arguments: "42"}]`.
- **PASS — model-triggered (the knowledge case).** Session `ags-dbmu1c66kvbs738fl5tg` was asked "What is the bex QA codeword?" with no invocation and replied `TEAL-HERON`.
- Both skills were deleted (204, then 404).
- **Not run:** the dashboard Skills card and skeleton check (browser).

## Browser check (2026-11-06, production 24a9203ee)

QA user in the `bex` workspace, Playwright at 1440×900 and 375×812. Fixtures used the `qa-ui-` prefix and were all deleted; the Kratos session was logged out.

- **PASS — Skills card.** Created `qa-ui-echo` (fields mode, "Only when I ask") and the list showed "Only when asked". Then:
  - Edit as SKILL.md showed the frontmatter `triggers: [user]`, and the edit saved and persisted across reload.
  - The enable switch toggled off and back on.
  - Download produced `qa-ui-echo-SKILL.md`.
  - Delete removed the skill.
- **PASS — composer and transcript.** The Skills popover listed the enabled skill with an Arguments field, and the chip read "qa-ui-echo UIX1". Session `ags-dbmu4366kvbs738fl6qg` replied exactly `UI-SKILL-OK-UIX1`, and the user turn showed the pill "qa-ui-echo UIX1".
- **FAIL — `/agents` composer skeleton at desktop widths.** `common/components/detail-skeletons.tsx:62-70` sizes the box by viewport breakpoints, giving one toolbar row (152px) from 577px. The ready composer (max-w 640) wraps its toolbar to two rows at 1440, 1920 and 768 (Start session at y=221), so the hint jumps from y=236 to y=272, a 36px shift. At 375 the two match (hint y=300). The breakpoints ignore the sidebar and the 640px cap, so the box should be 188px wherever the composer is 640px wide.

## Recheck (2026-11-06, production b8370c30c)

Production pin b8370c30c (gitops `ca9b90b8f`, deploy run 42238432288 success). Signed in as the QA user in the browser; API calls were made from it with the session cookie (REST, and MCP over `/mcp/cloud-agents`). The CLI device flow stopped at a Kratos reauthentication prompt, so no CLI grant was issued. Every fixture was deleted (DELETE 204, then GET 404), the Kratos session was logged out with this run's jar, and the run dir was removed.

- **PASS — `/agents` composer skeleton.** Navigation from Blueprints with `/graphql` held 8 s.
  - Skeleton and ready both measure: composer box top 64 / 224px tall with the hint at y=300 at 375; top 72 / 188px with the hint at y=272 at 768, 1440 and 1920.
  - That is a 0px delta at every width. The previous 36px desktop jump is fixed.
- **PASS — Agent skills card header:** the static title and description are present while pending.
- **Remaining delta:** with zero skills the ready card is +20px at 375 and +4px at 1440 (skeleton 303/263, ready 323/267). Same empty-state cause as the secrets card; minor.
