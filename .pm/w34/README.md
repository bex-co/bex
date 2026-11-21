# w34 — project workstream (worker34)

**Worker:** worker34. This is a general-purpose bex workstream. It may take work anywhere in the project; the milestones below are scheduled work and historical records, not a permanent purpose, specialty, or ownership boundary.

## Milestones

- [ ] **m541** — **BLOCKED (clears by itself, nothing is needed from the user: pin must carry `a2c7040f4` (the w34/m541 commit), then switch `bex-canary` to Pro under the user's 2026-10-31 permission, walk t008 (DoD 1–5) on production with the runbook's `/api postgres-logical-replication` and an external PostgreSQL 18 subscriber, switch it back to Hobby, delete the fixtures and close with t012)** — [A Pro-workspace Postgres with at least 10 GB publishes to an external subscriber once logical replication is enabled, as Render enables it](blocked/m541/README.md) (12 tasks) ← from `/pm-brainstorm` for one aspect of render parity (Postgres logical replication), 2026-11-20, item 1. t001–t007 and t009–t011 done 2026-11-21 (`a2c7040f4`): the spec field, the operator's projection, the operators' verb, the runbook, the dashboard note and the agent skill, tested on real PostgreSQL 17 and 18 with a real subscription; no dev stack exists for w34, so the cluster walk is on production
- [ ] **m542** — [A bex Postgres subscribes to another bex database or one outside bex once it is enabled as a subscriber, as on Render](m542/README.md) (10 tasks) ← same brainstorm, item 2; builds on m541
- [ ] **m543** — [Logical replication survives failover, switchover, restarts, suspend and major-version upgrades](m543/README.md) (10 tasks) ← same brainstorm, item 3; builds on m541

## Inbox

- [ ] [001](001.md) — Capture Render's answers where its logical-replication docs are silent, on a Pro workspace (~1h with a Render Pro workspace) ← same brainstorm, item 4; needs a Render workspace, which the board does not have (the [`w10/blocked/002`](../w10/blocked/002.md) gate)
- [ ] [002](002.md) — Decide how a workspace asks for logical replication: a request channel bex does not have yet, or self-service as a signed-off divergence (~45m, a user decision) ← same brainstorm, item 5
