# w2 — AI-native surface (worker2)

**Worker:** worker2 The vision's differentiator (`docs/vision.md` pillars 3–5), owned by no workstream today. w1 builds the platform/control-plane; w2 makes it _native_ for agents. Ordered by dependency: MCP first (thin adapter, no new backend), then deploy-from-chat (needs w1's control plane + in-cluster builds), then hosted sandboxes.

## Milestones

- [x] **m1** — MCP server over bex-api verbs (4 tasks) ← pillar 3
- [ ] **m2** — Deploy-from-chat + HMAC git webhook (4 tasks; t001–t003 DONE 2026-07-08, t004 live acceptance blocked on w1/m5 builds) ← pillar 4, needs w2/m4 + w1/m2 + w1/m5 (t001 amended 2026-07-08 to ride m4's `Core.Create`)
- [ ] **m3** — E2B-compatible sandboxes, idle-hibernated (4 tasks) ← pillar 5
- [ ] **m4** — Render-shaped service create & delete (`POST /v1/services` · `create_web_service`) (6 tasks) ← from `/pm-brainstorm for w2` 2026-07-08
- [ ] **m5** — Deploy history + trigger (`list_deploys` · `get_deploy` · `POST /deploys`) (7 tasks) ← from `/pm-brainstorm for w2` 2026-07-08, needs w1/m2
- [ ] **m6** — MCP `query_render_postgres` — read-only SQL for agents (5 tasks) ← from `/pm-brainstorm for w2` 2026-07-08
- [x] **m12** — Render scale API (`POST /v1/services/{id}/scale`) (5 tasks) ← relocated from w1 2026-07-08

## Inbox

- (moved) agent OAuth 2.1 provider → promoted as `w4/m9` (auth workstream owns it; w2's MCP milestones consume it)
- `002.md` — Workspace MCP tools (`list_workspaces`/`select_workspace`/`get_selected_workspace`) — blocked on w1/m9 (real workspaces)
