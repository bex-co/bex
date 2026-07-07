# w2 — AI-native surface (worker2)

**Worker:** worker2 The vision's differentiator (`docs/vision.md` pillars 3–5), owned by no workstream today. w1 builds the platform/control-plane; w2 makes it _native_ for agents. Ordered by dependency: MCP first (thin adapter, no new backend), then deploy-from-chat (needs w1's control plane + in-cluster builds), then hosted sandboxes.

## Milestones

- [x] **m1** — MCP server over bex-api verbs (4 tasks) ← pillar 3
- [ ] **m2** — Deploy-from-chat + HMAC git webhook (4 tasks) ← pillar 4, needs w1/m2 + w1/m5
- [ ] **m3** — E2B-compatible sandboxes, idle-hibernated (4 tasks) ← pillar 5

## Inbox

- `001.md` — bex as an OAuth 2.1 provider for agents (Kratos native `oauth2_provider` + headless consent acceptor + DCR) — deferred from the reverted w4/m9; candidate milestone alongside the MCP auth spec
