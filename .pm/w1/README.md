# w1 — bex platform roadmap (worker1)

**Worker:** worker1 Converted from the `.tmp/` backlog (items 001–010) on 2026-07-02. Ordered roughly by priority/dependency: de-risk the live system, build the source-of-truth control plane, then the elastic/cost machinery, then pipeline, isolation, and hardening.

## Milestones

- [x] **m1** — Reliability: fix config drift + back up etcd (4 tasks) ← from `009`, `007` — done 2026-07-05, moved to `done/m1/`
- [ ] **m2** — Control plane: Postgres source of truth in `lego/backend` (7 tasks) ← from `005` (t001–t006 done, committed `aebbd43`; open: t007 live acceptance — prod `BEX_CP_DB_URI` still off)
- [x] **m2.5** — Refactor bex-api into feature packages (one package per feature) (9 tasks) ← from `/pm` architecture review 2026-07-06 — done (shipped `06f247e` 2026-07-06, verified + board synced 2026-07-08), in `done/m2.5/`
- [ ] **m3** — Elastic substrate: bin-pack + autoscale (7 tasks) ← from `002`, `004` (t001 done; retrofitted to current `/pm` canon 2026-07-08)
- [x] **m4** — Free tier = sleep: scale-to-zero + wake activator (5 tasks) ← from `003` — done 2026-07-08, moved to `done/m4/`
- [ ] **m4.5** — Sleep in the dashboard: hibernated state, idle-timeout setting, wake UX (6 tasks) ← user request 2026-07-08, UI half of m4 (unblocked — m4 shipped 2026-07-08)
- [ ] **m5** — Build & deploy from git, in-cluster (5 tasks) ← from `008` (retrofitted 2026-07-08; t003 webhook shipped by w2 deploy-from-chat — reduced to verify-and-close)
- [x] **m7** — Prod hardening: network · secrets · images (6 tasks) ← from `010` (+ OpenBao backup, added 2026-07-08) — done 2026-07-08, moved to `done/m7/`
- [x] **m8** — Instance tiers: one catalog, Render-shaped plan API, limits everywhere (6 tasks) ← from architecture discussion 2026-07-08 (metrics page's "No limit configured" gap; Render compute-plans ladder) — done 2026-07-08, moved to `done/m8/`
- [ ] **m9** — Tenant onboarding: real workspaces + OpenFGA enforced in prod (6 tasks) ← from `/pm-brainstorm for w1` 2026-07-08 (m2 deferrals; prod authz is allow-all)
- [x] **m10** — OpenBao prod wiring: env-vars live in prod (6 tasks) ← from `/pm-brainstorm for w1` 2026-07-08 (docs/secrets.md "Prod deploy path") — done 2026-07-08 (implementation shipped + locally validated), moved to `done/m10/`; prod activation (first init + live PUT) is the operator's runbook there
- [ ] **m11** — Render custom-domains API over `App.spec.hosts[]` (5 tasks) ← promoted from `003` 2026-07-08
- [ ] **m13** — Render parity audit: REST · GraphQL · MCP · UI matrix (6 tasks) ← user request 2026-07-08

## Suggested execution order (2026-07-08 refinement)

1. **m9** — tenant onboarding + enforced OpenFGA (m2/t007 marked done; prod authz is still allow-all — the standing security gap).
2. **m13** — parity audit (independent; orders the remaining parity queue with evidence).
3. **m11** — custom-domains API; **m5** — build-from-git (t001/t002 are the last missing piece of the flow; the webhook half already shipped via w2).
4. **m3** — elastic substrate (replica-semantics contract settled by w2/m12; scale-down pays off most now that m4's sleep empties nodes).
5. **m4.5** — dashboard sleep UX (pairs with shipped m4; UI-half work).

> **Board refinement (2026-07-08, from `/pm` + `/pm-brainstorm` conventions):** m3 and m5 retrofitted to the current canon — `## Source + Goal linkage` sections, the two standing closing tasks (Simplify, Test coverage), and pre-`lego/` paths fixed; m3/t001 (done) moved to `m3/done/`. m5/t003 reduced to verify-and-close (its webhook shipped in bex-api via w2's deploy-from-chat — where this refinement had independently concluded it belongs; operator stays mechanism-only). Planned m4/m7 retrofits and an m2/t007 rewrite were dropped — those milestones completed upstream first. No milestone was removed — all open ones map to a V0 roadmap item or Render parity and pass the DO_NOT_DO screen.

## Inbox

- `005.md` — Wire `spec.healthCheckPath` into a ReadinessProbe (or drop the field) — from the 2026-07-08 docs-vs-code audit
- `006.md` — Triage 36 Dependabot findings (2 critical, 15 high) reported 2026-07-08
- `007.md` — Key Value (Valkey/Redis) managed store: `KeyValue` CR + reconciler (mechanism half; surfaces follow in w2/w5) — from `/pm-brainstorm for w2` 2026-07-08

> `003` (custom-domains API) promoted to **m11** and `004` (scale API) promoted to **m12** on 2026-07-08; notes moved to `done/`. m12 was subsequently relocated to **w2** (done: `w2/done/m12/`).

> **m6 (Multi-tenant isolation) removed 2026-07-07** — the plan leaned on vcluster-per-tenant, which is the wrong isolation model for bex (see [`.pm/DO_NOT_DO.md`](../DO_NOT_DO.md)). If tenant isolation is re-scoped later, it must be namespace-tier → microVM, not per-tenant virtual control planes.
