# w2 — AI-native surface (worker2)

**Worker:** worker2 The vision's differentiator (`docs/ADR008-vision.md` pillars 3–5), owned by no workstream today. w1 builds the platform/control-plane; w2 makes it _native_ for agents. Ordered by dependency: MCP first (thin adapter, no new backend), then deploy-from-chat (needs w1's control plane + in-cluster builds), then hosted sandboxes.

## Milestones

- [x] **m1** — MCP server over bex-api verbs (4 tasks) ← pillar 3
- [x] **m2** — Deploy-from-chat + HMAC git webhook (4 tasks; DONE — t001–t003 2026-07-08, t004 live acceptance PASSED 2026-07-09) ← pillar 4, needs w2/m4 + w1/m2 + w1/m5 (t001 amended 2026-07-08 to ride m4's `Core.Create`)
- [x] **m4** — Render-shaped service create & delete: verify shipped create + build delete (8 tasks; DONE 2026-07-09 — delete on all 3 surfaces, create verified vs Render OpenAPI, live delete-cascade acceptance PASSED) ← from `/pm-brainstorm for w2` 2026-07-08, re-scoped 2026-07-08 (create half shipped via m2/t001)
- [x] **m5** — Deploy history + trigger (`list_deploys` · `get_deploy` · `POST /deploys`) (7 tasks; DONE — t005 live acceptance PASSED 2026-07-09) ← from `/pm-brainstorm for w2` 2026-07-08, needs w1/m2
- [x] **m6** — MCP `query_render_postgres` — read-only SQL for agents ← reassigned to **w5/m9** (label collision with w5's done m6) and shipped there 2026-07-08 (`w5/done/m9/`)
- [x] **m7** — Key Value API surface (REST/GraphQL/MCP, Render-shaped) (10 tasks) ← promoted from `003` 2026-07-09, unblocked by w1/m14 (mechanism live in prod); dashboard half is `w5/m12` — done (in `done/m7/`; row synced 2026-07-12, was stale)
- [x] **m12** — Render scale API (`POST /v1/services/{id}/scale`) (5 tasks) ← relocated from w1 2026-07-08
- [x] **m8** — Connect GitHub: GitHub App connection + repo listing (REST/GraphQL/MCP/UI) (10 tasks; DONE 2026-07-12 — live DoD PASSED against the real `bex-co` GitHub App on the mock cluster; dashboard card re-landed after the codegen fix) ← from `/pm-brainstorm for w2` 2026-07-11 (parity row "Git connections" ◐)
- [x] **m9** — Private-repo deploys + zero-config GitHub push-to-deploy (9 tasks; DONE 2026-07-12 — t005 live acceptance PASSED: private repo → live URL, hands-free push → rev-2, `autoDeploy:false` suppresses, tampered signature 401; follow-up `005.md` filed on store-path composition) ← from `/pm-brainstorm for w2` 2026-07-11, needs w2/m8; unblocks `w5/m15`
- [ ] **m10** — Deploy cancel + rollback (9 tasks) ← from `/pm-brainstorm more` 2026-07-12 (the two ✖ Deploys rows m5 left; rollback target recorded per deploy); UI buttons ride `w5/007`
- [ ] **m11** — Unify service creation through the control-plane store (9 tasks) ← promoted from `005` 2026-07-12 (found during the m8/m9 live acceptance: private-repo store-managed builds fail with no clone secret; public creates have no deploy history)

## Inbox

- `006.md` — GraphQL `triggerDeploy` mutation (deploy-trigger is REST-only since w2/m5; unblocks the Manual Deploy button riding `w5/007`) ← from `/pm-brainstorm more for w5` 2026-07-12
- (moved) agent OAuth 2.1 provider → promoted as `w4/m9` (auth workstream owns it; w2's MCP milestones consume it)

> `002.md` retired to `done/` after w6/m2 closed (line repaired 2026-07-12 — a stale "none open" bullet sat here while `005.md` was open).
> `003.md` promoted to `m7` (Key Value API surface) 2026-07-09 — moved to `done/`.
> `004.md` — Agent-connect recipe: done 2026-07-11 (`docs/ADR025-connect-an-agent.md`) — moved to `done/`.
> `005.md` promoted to **m11** (unify service creation) 2026-07-12 — moved to `done/`.

> **m3 (E2B-compatible sandboxes) removed 2026-07-08** — hosted agent sandboxes (pillar 5) are off the roadmap by user decision; see [`.pm/DO_NOT_DO.md`](../DO_NOT_DO.md). The architecture record stays in docs/ADR014-sandboxes.md (ADR, status: proposed) for a future explicit re-open.
