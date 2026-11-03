# w21 — project workstream (worker21)

**Worker:** worker21. This is a general-purpose bex workstream. It may take work anywhere in the project; the milestones below are scheduled work and historical records, not a permanent purpose, specialty, or ownership boundary.

## Milestones

- [x] **m257** — Cloud Agents MCP phase 1: contract hygiene — honest descriptions, `state`, `outcome` (8 tasks) → [`done/m257/`](done/m257/README.md) ← from [ADR094](../../docs/ADR094-coding-agent-mcp.md) D10 phase 1
- [ ] **m258** — Cloud Agents MCP phase 2: the `/mcp/cloud-agents` toolset, `wait_agent_sessions`, idempotent create, `bex.cloud_agents` scope (11 tasks) → [`m258/`](m258/README.md) ← from ADR094 D10 phase 2; after m257
- [ ] **m259** — Cloud Agents MCP phase 3: schema-validated structured output (`submit_result`) (9 tasks) → [`m259/`](m259/README.md) ← from ADR094 D10 phase 3; after m258
- [ ] **m260** — Cloud Agents MCP phase 4: completion webhooks, per-session spend ceiling, `outcome.cost` (7 tasks) → [`m260/`](m260/README.md) ← from ADR094 D10 phase 4; spend tasks gated on `w10/m14` t004–t005

## Inbox

- [ ] **001** — Dashboard `/agents` and mobile read `state`/`outcome` instead of decoding `phase` → [`001.md`](001.md)
