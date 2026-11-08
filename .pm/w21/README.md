# w21 — project workstream (worker21)

**Worker:** worker21. This is a general-purpose bex workstream. It may take work anywhere in the project; the milestones below are scheduled work and historical records, not a permanent purpose, specialty, or ownership boundary.

## Milestones

- [x] **m257** — Cloud Agents MCP phase 1: contract hygiene — honest descriptions, `state`, `outcome` (8 tasks) → [`done/m257/`](done/m257/README.md) ← from [ADR094](../../docs/ADR094-coding-agent-mcp.md) D10 phase 1
- [x] **m258** — Cloud Agents MCP phase 2: the `/mcp/cloud-agents` toolset, `wait_agent_sessions`, idempotent create, `bex.cloud_agents` scope (11 tasks) → [`done/m258/`](done/m258/README.md) ← from ADR094 D10 phase 2; after m257
- [x] **m259** — Cloud Agents MCP phase 3: schema-validated structured output (`submit_result`) (9 tasks) → [`done/m259/`](done/m259/README.md) ← from ADR094 D10 phase 3; after m258 — DONE 2026-11-05
- [ ] **m260** — moved to **w9/m172** (2026-11-05, user decision: w9 follows up all w21 work) → [`../w9/m172/`](../w9/m172/README.md)
- [ ] **m278** — [Agent Automations: scheduled cloud-agent sessions (Devin parity)](m278/README.md) (12 tasks) ← from [ADR101](../../docs/ADR101-agent-automations.md)
- [ ] **m279** — [Agent child sessions: an agent can spawn and track child sessions (Devin parity)](m279/README.md) (11 tasks) ← from [ADR094](../../docs/ADR094-coding-agent-mcp.md) deferral; t002–t003 after w9/m179
- [ ] **m280** — [Skill version history: every save keeps a version you can read, diff and restore (Devin parity)](m280/README.md) (8 tasks) ← from [ADR099](../../docs/ADR099-agent-skills.md) follow-up

## Inbox

- [x] **001** — Dashboard `/agents` and mobile read `state`/`outcome` instead of decoding `phase` → [`001.md`](done/001.md)
- [x] **002** — Dashboard `/agents` create sends an `idempotencyKey` and shows `title`/`tags` → [`002.md`](done/002.md)
- [x] **003** — Dashboard `/agents` renders `outcome.result` and the in-flight partial result → [`003.md`](done/003.md)
- [ ] **004** — moved to **w9/076** (2026-11-05) → [`../w9/076.md`](../w9/076.md)
- [x] **005** — Dashboard `/agents` creates with a `maxSpendUSD` ceiling and shows `outcome.cost` → [`005.md`](done/005.md)
- [ ] **006** — moved to **w9/078** (2026-11-05, user decision pending) → [`../w9/blocked/078.md`](../w9/blocked/078.md)
- [ ] **007** — moved to **w9/077** (2026-11-05) → [`../w9/077.md`](../w9/077.md)
- [ ] **008** — Document the "Deploy to bex" button in the public docs → [`008.md`](008.md)
- [ ] **009** — Check the Deploy-to-bex link survives sign-up and the payment gate for a new account → [`009.md`](009.md)
- [ ] **010** — Automations: count a workspace's automations in the database, and read the previous run's phase with one query → [`010.md`](010.md)
