# w7 · m9 — Per-workspace abuse limits: creation caps + build concurrency

**Worker:** worker7 **Goal:** A workspace at its plan's resource-creation or build-concurrency limit gets a clean, Render-shaped rejection on every surface — never a 500, never a silent queue — and raising the plan raises the cap live. Today there is no ceiling at all on how many services/databases/Key Value instances a workspace can create, or how many builds it can run concurrently. **Status:** todo

## Tasks (in order)

| id   | title                                                                                                     | est | depends_on             |
| ---- | ------------------------------------------------------------------------------------------------------------ | --- | ----------------------- |
| t001 | Design: per-workspace resource creation caps (max services/DBs/KV instances) keyed to plan, mirroring `w1/m8`'s instance-tier catalog | 30m | —                        |
| t002 | Enforce caps at `Core.Create` + Postgres/KeyValue create paths — Render-shaped 4xx on cap exceeded            | 45m | t001                     |
| t003 | Build concurrency cap: bound concurrent build Jobs per workspace (queue or reject)                            | 40m | t001                     |
| t004 | REST/GraphQL/MCP: surface current usage vs. limit (e.g. "3/5 services")                                       | 30m | t002                     |
| t005 | Dashboard: cap-hit messaging + upgrade nudge                                                                  | 30m | t004                     |
| t006 | Acceptance: hit a cap → clean rejection on every surface; upgrade plan → cap raises live                      | 25m | t002, t003, t005         |
| t007 | Render parity — verify the cap-rejection shape/semantics across REST/GraphQL/MCP/UI vs render.com's plan-limit UX | 20m | t006                     |
| t008 | Simplify — `/simplify` over the code this milestone changed                                                   | 20m | t007                     |
| t009 | Test coverage — meaningful tests for cap enforcement, concurrency bounding, and plan-change cap changes       | 35m | t007                     |
| t010 | Closeout — verify DoD met, then move the milestone to `done/`                                                 | 10m | t009                     |

## Definition of done

A workspace at its plan's service/DB/KV/build-concurrency limit gets a clean, Render-shaped rejection (never a 500 or silent queue) on every surface — REST, GraphQL, MCP, and the dashboard show the same limit and the same rejection; raising the workspace's plan (`w6/m12`'s `changeWorkspacePlan`) raises the cap immediately, verified live, not just unit-tested.

## Source + Goal linkage

- **Source:** `/pm-brainstorm more milestones to work on` 2026-07-12, re-deriving the scope for a phantom board entry — `w7/README.md` had already listed this milestone (from `/pm-brainstorm more for w7` round 2, 2026-07-12) but `.pm/w7/m9/` was never materialized on disk; this fills that gap under the number the README already reserved.
- **Goal linkage:** `GOAL.md` #7 (security review) — w7's own abuse-hardening mandate (`w7/README.md`: "Ordered by hole size: network isolation first, then workload hardening, then API abuse limits").
- **Expected outcome:** a real, standing per-workspace creation/build-concurrency ceiling where none exists today; the cap and the current-usage count are visible on every surface, not just enforced silently.
- **Why now:** `w7/m3` capped per-*caller* request rate; this closes the sibling gap that round explicitly left open — per-*workspace* resource creation. Real tenants and multi-workspace users already exist (`w6`, `w1/m9`) with zero ceiling on how many services/databases/KV instances they can create or how many builds they can run at once — a live cost/abuse vector, not speculative hardening.
- **Render parity closing task: included** (t007) — new REST/GraphQL/MCP/UI surface (cap visibility + rejection shape).
