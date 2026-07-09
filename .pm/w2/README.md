# w2 — AI-native surface (worker2)

**Worker:** worker2 The vision's differentiator (`docs/vision.md` pillars 3–5), owned by no workstream today. w1 builds the platform/control-plane; w2 makes it _native_ for agents. Ordered by dependency: MCP first (thin adapter, no new backend), then deploy-from-chat (needs w1's control plane + in-cluster builds), then hosted sandboxes.

## Milestones

- [x] **m1** — MCP server over bex-api verbs (4 tasks) ← pillar 3
- [ ] **m2** — Deploy-from-chat + HMAC git webhook (4 tasks; t001–t003 DONE 2026-07-08, t004 live acceptance unblocked — w1/m5 landed) ← pillar 4, needs w2/m4 + w1/m2 + w1/m5 (t001 amended 2026-07-08 to ride m4's `Core.Create`)
- [ ] **m4** — Render-shaped service create & delete: verify shipped create + build delete (8 tasks) ← from `/pm-brainstorm for w2` 2026-07-08, re-scoped 2026-07-08 (create half shipped via m2/t001)
- [ ] **m5** — Deploy history + trigger (`list_deploys` · `get_deploy` · `POST /deploys`) (7 tasks) ← from `/pm-brainstorm for w2` 2026-07-08, needs w1/m2
- [x] **m6** — MCP `query_render_postgres` — read-only SQL for agents ← reassigned to **w5/m9** (label collision with w5's done m6) and shipped there 2026-07-08 (`w5/done/m9/`)
- [ ] **m7** — Key Value API surface (REST/GraphQL/MCP, Render-shaped) (10 tasks) ← promoted from `003` 2026-07-09, unblocked by w1/m14 (mechanism live in prod); dashboard half is `w5/m12`
- [x] **m12** — Render scale API (`POST /v1/services/{id}/scale`) (5 tasks) ← relocated from w1 2026-07-08

## Inbox

- (moved) agent OAuth 2.1 provider → promoted as `w4/m9` (auth workstream owns it; w2's MCP milestones consume it)
- `002.md` — Workspace MCP tools (`list_workspaces`/`select_workspace`/`get_selected_workspace`) — superseded by `w6/m2` (t005) 2026-07-08; retire to `done/` when w6/m2 closes
- `004.md` — Agent-connect recipe: document Claude Code/Cursor → `/mcp` with OAuth 2.1 (w4/m9 flow)

> `003.md` promoted to `m7` (Key Value API surface) 2026-07-09 — moved to `done/`.

> **m3 (E2B-compatible sandboxes) removed 2026-07-08** — hosted agent sandboxes (pillar 5) are off the roadmap by user decision; see [`.pm/DO_NOT_DO.md`](../DO_NOT_DO.md). The architecture record stays in docs/sandboxes.md (ADR, status: proposed) for a future explicit re-open.
