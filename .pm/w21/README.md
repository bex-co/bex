# w21 — project workstream (worker21)

**Worker:** worker21. This is a general-purpose bex workstream. It may take work anywhere in the project; the milestones below are scheduled work and historical records, not a permanent purpose, specialty, or ownership boundary.

## Milestones

- [x] **m257** — Cloud Agents MCP phase 1: contract hygiene — honest descriptions, `state`, `outcome` (8 tasks) → [`done/m257/`](done/m257/README.md) ← from [ADR094](../../docs/ADR094-coding-agent-mcp.md) D10 phase 1
- [x] **m258** — Cloud Agents MCP phase 2: the `/mcp/cloud-agents` toolset, `wait_agent_sessions`, idempotent create, `bex.cloud_agents` scope (11 tasks) → [`done/m258/`](done/m258/README.md) ← from ADR094 D10 phase 2; after m257
- [x] **m259** — Cloud Agents MCP phase 3: schema-validated structured output (`submit_result`) (9 tasks) → [`done/m259/`](done/m259/README.md) ← from ADR094 D10 phase 3; after m258 — DONE 2026-11-05
- [ ] **m260** — Cloud Agents MCP phase 4: completion webhooks, per-session spend ceiling, `outcome.cost` → [`blocked/m260/`](blocked/m260/README.md) — **BLOCKED (t001–t006 done and shipped; t007 closeout needs a live completion-webhook and spend-ceiling run on a dev stack with the agent leg up, tracked as inbox 007 — `dev-env.sh` has no dev-21, so someone must run it on dev-1…10; `outcome.cost.sandboxSeconds` is not delivered, see inbox 006)** ← from ADR094 D10 phase 4

## Inbox

- [x] **001** — Dashboard `/agents` and mobile read `state`/`outcome` instead of decoding `phase` → [`001.md`](done/001.md)
- [x] **002** — Dashboard `/agents` create sends an `idempotencyKey` and shows `title`/`tags` → [`002.md`](done/002.md)
- [x] **003** — Dashboard `/agents` renders `outcome.result` and the in-flight partial result → [`003.md`](done/003.md)
- [ ] **004** — Live structured-output session on a dev stack (m259 definition of done) → [`004.md`](blocked/004.md) — **BLOCKED (needs someone to run a live structured-output session on a dev stack (`dev-env.sh <1-10> agent-up` plus a model route); dev-21 does not exist)**
- [x] **005** — Dashboard `/agents` creates with a `maxSpendUSD` ceiling and shows `outcome.cost` → [`005.md`](done/005.md)
- [ ] **006** — `outcome.cost.sandboxSeconds`: attribute sandbox compute to a session → [`006.md`](006.md)
- [ ] **007** — Live completion webhook and spend ceiling on a dev stack (m260 definition of done) → [`007.md`](blocked/007.md) — **BLOCKED (needs someone to run a live completion-webhook and spend-ceiling session on a dev stack (`dev-env.sh <1-10> agent-up`, a webhook receiver, a model route); dev-21 does not exist)**
