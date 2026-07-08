# w1 — bex platform roadmap (worker1)

**Worker:** worker1 Converted from the `.tmp/` backlog (items 001–010) on 2026-07-02. Ordered roughly by priority/dependency: de-risk the live system, build the source-of-truth control plane, then the elastic/cost machinery, then pipeline, isolation, and hardening.

## Milestones

- [x] **m1** — Reliability: fix config drift + back up etcd (4 tasks) ← from `009`, `007` — done 2026-07-05, moved to `done/m1/`
- [ ] **m2** — Control plane: Postgres source of truth in `lego/backend` (7 tasks) ← from `005` (t001–t006 done, committed `aebbd43`; open: t007 live acceptance — prod `BEX_CP_DB_URI` still off)
- [x] **m2.5** — Refactor bex-api into feature packages (one package per feature) (9 tasks) ← from `/pm` architecture review 2026-07-06 — done (shipped `06f247e` 2026-07-06, verified + board synced 2026-07-08), in `done/m2.5/`
- [ ] **m3** — Elastic substrate: bin-pack + autoscale (5 tasks) ← from `002`, `004` (001 done)
- [x] **m4** — Free tier = sleep: scale-to-zero + wake activator (5 tasks) ← from `003` — done 2026-07-08, moved to `done/m4/`
- [ ] **m4.5** — Sleep in the dashboard: hibernated state, idle-timeout setting, wake UX (6 tasks) ← user request 2026-07-08, UI half of m4 (unblocked — m4 shipped 2026-07-08)
- [ ] **m5** — Build & deploy from git, in-cluster (3 tasks) ← from `008`
- [ ] **m7** — Prod hardening: network · secrets · images (6 tasks) ← from `010` (+ OpenBao backup, added 2026-07-08)
- [x] **m8** — Instance tiers: one catalog, Render-shaped plan API, limits everywhere (6 tasks) ← from architecture discussion 2026-07-08 (metrics page's "No limit configured" gap; Render compute-plans ladder) — done 2026-07-08, moved to `done/m8/`
- [ ] **m9** — Tenant onboarding: real workspaces + OpenFGA enforced in prod (6 tasks) ← from `/pm-brainstorm for w1` 2026-07-08 (m2 deferrals; prod authz is allow-all)
- [ ] **m10** — OpenBao prod wiring: env-vars live in prod (6 tasks) ← from `/pm-brainstorm for w1` 2026-07-08 (docs/secrets.md "Prod deploy path")
- [ ] **m11** — Render custom-domains API over `App.spec.hosts[]` (5 tasks) ← promoted from `003` 2026-07-08
- [ ] **m12** — Render scale API (`POST /v1/services/{id}/scale`) (5 tasks) ← promoted from `004` 2026-07-08
- [ ] **m13** — Render parity audit: REST · GraphQL · MCP · UI matrix (6 tasks) ← user request 2026-07-08

## Inbox

- `005.md` — Wire `spec.healthCheckPath` into a ReadinessProbe (or drop the field) — from the 2026-07-08 docs-vs-code audit
- `006.md` — Triage 36 Dependabot findings (2 critical, 15 high) reported 2026-07-08
- `007.md` — Key Value (Valkey/Redis) managed store: `KeyValue` CR + reconciler (mechanism half; surfaces follow in w2/w5) — from `/pm-brainstorm for w2` 2026-07-08

> `003` (custom-domains API) promoted to **m11** and `004` (scale API) promoted to **m12** on 2026-07-08; notes moved to `done/`.

> **m6 (Multi-tenant isolation) removed 2026-07-07** — the plan leaned on vcluster-per-tenant, which is the wrong isolation model for bex (see [`.pm/DO_NOT_DO.md`](../DO_NOT_DO.md)). If tenant isolation is re-scoped later, it must be namespace-tier → microVM, not per-tenant virtual control planes.
