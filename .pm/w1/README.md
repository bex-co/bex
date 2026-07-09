# w1 — bex platform roadmap (worker1)

**Worker:** worker1 Converted from the `.tmp/` backlog (items 001–010) on 2026-07-02. Ordered roughly by priority/dependency: de-risk the live system, build the source-of-truth control plane, then the elastic/cost machinery, then pipeline, isolation, and hardening.

## Milestones

- [x] **m1** — Reliability: fix config drift + back up etcd (4 tasks) ← from `009`, `007` — done 2026-07-05, moved to `done/m1/`
- [ ] **m2** — Control plane: Postgres source of truth in `lego/backend` (7 tasks) ← from `005` (t001–t006 done, committed `aebbd43`; open: t007 live acceptance — prod `BEX_CP_DB_URI` still off)
- [x] **m2.5** — Refactor bex-api into feature packages (one package per feature) (9 tasks) ← from `/pm` architecture review 2026-07-06 — done (shipped `06f247e` 2026-07-06, verified + board synced 2026-07-08), in `done/m2.5/`
- [ ] **m3** — Elastic substrate: bin-pack + autoscale (7 tasks) ← from `002`, `004` (t001 done; retrofitted to current `/pm` canon 2026-07-08)
- [x] **m4** — Free tier = sleep: scale-to-zero + wake activator (5 tasks) ← from `003` — done 2026-07-08, moved to `done/m4/`
- [x] **m4.5** — Sleep in the dashboard: hibernated state, idle-timeout setting, wake UX (6 tasks; DONE 2026-07-09 — Sleeping badge + idle-timeout control, verified live) ← user request 2026-07-08, UI half of m4, moved to `done/m4.5/`
- [x] **m5** — Build & deploy from git, in-cluster (5 tasks; DONE 2026-07-09 — in-cluster BuildKit builds, verified live, simplified + tested; unblocks w2/m2 t004) ← from `008`, moved to `done/m5/`
- [x] **m7** — Prod hardening: network · secrets · images (6 tasks) ← from `010` (+ OpenBao backup, added 2026-07-08) — done 2026-07-08, moved to `done/m7/`
- [x] **m8** — Instance tiers: one catalog, Render-shaped plan API, limits everywhere (6 tasks) ← from architecture discussion 2026-07-08 (metrics page's "No limit configured" gap; Render compute-plans ladder) — done 2026-07-08, moved to `done/m8/`
- [x] **m9** — Tenant onboarding: real workspaces + OpenFGA enforced in prod (6 tasks; DONE 2026-07-09 — mint-on-first-login, key→tenant binding, workspace-scoped authz, prod OpenFGA flip, simplified + tested) ← from `/pm-brainstorm for w1` 2026-07-08 (m2 deferrals; prod authz is allow-all), moved to `done/m9/`
- [x] **m10** — OpenBao prod wiring: env-vars live in prod (6 tasks) ← from `/pm-brainstorm for w1` 2026-07-08 (docs/secrets.md "Prod deploy path") — done 2026-07-08 (implementation shipped + locally validated), moved to `done/m10/`; prod activation (first init + live PUT) is the operator's runbook there
- [x] **m11** — Render custom-domains API over `App.spec.hosts[]` (5 tasks) ← promoted from `003` 2026-07-08
- [x] **m11.5** — Custom-domains dashboard section, UI half of m11 (8 tasks) ← promoted from `w5/003` 2026-07-09 — done 2026-07-09, moved to `done/m11.5/`
- [x] **m13** — Render parity audit: REST · GraphQL · MCP · UI matrix (6 tasks) ← user request 2026-07-08 — done 2026-07-08, moved to `done/m13/` (shipped `docs/render-parity.md`; 5 gap notes filed: `008`–`011`, `w3/002`)
- [ ] **m14** — Key Value (Valkey/Redis) managed store: `KeyValue` CRD + reconciler (6 tasks) ← promoted from `007` 2026-07-08 (mechanism; REST/GraphQL/MCP/UI surfaces are w2/w5 follow-ons)
- [x] **m15** — Additional service types: background worker + cron job (8 tasks) ← promoted from `009` 2026-07-08 (static site split to `012`) — done 2026-07-09, moved to `done/m15/` (had been staged under `w7/m15/`)
- [ ] **m16** — Config surfaces beyond env vars: environment groups + secret files (7 tasks) ← promoted from `010` 2026-07-08
- [ ] **m17** — Managed Postgres advanced: data-protection + lifecycle + access (8 tasks) ← promoted from `011` 2026-07-08 (HA/replicas split to `013`)

## Suggested execution order (2026-07-08 refinement, superseded 2026-07-09)

> All four items below are now done (m9, m5, m4.5 moved to `done/`; m3 remains open — see the Milestones list above for current status). Kept for provenance.

1. **m9** — tenant onboarding + enforced OpenFGA (m2/t007 marked done; prod authz is still allow-all — the standing security gap).
2. **m5** — build-from-git (t001/t002 are the last missing piece of the flow; the webhook half already shipped via w2).
3. **m3** — elastic substrate (replica-semantics contract settled by w2/m12; scale-down pays off most now that m4's sleep empties nodes).
4. **m4.5** — dashboard sleep UX (pairs with shipped m4; UI-half work).

> **Parity-backlog tier (promoted from the m13 audit, 2026-07-08):** **m14** Key Value · **m15** worker + cron service types · **m16** env-groups + secret-files · **m17** advanced Postgres. Sequence these after the V0 items above, ordered by the gap ranking in `docs/render-parity.md`. **008** (per-service autoscaling) stays an inbox note until **m3** settles the metric→replica loop it reuses.

> **m13 (parity audit) done 2026-07-08** — its output, `docs/render-parity.md`, orders the remaining parity queue with evidence. Its gap notes were arranged into milestones on 2026-07-08: `007`→**m14**, `009`→**m15**, `010`→**m16**, `011`→**m17** (with `012` static-site + `013` Postgres-HA split off as notes); `008` (autoscaling) + `w3/002` (request logs) stay notes; the rest cross-reference existing owners (w2/m4 delete, w2/m5 deploys, w4/m8 keys-UI, w4/m12 members, w5/004 scaling-UI, w5/006 DNS-UI).

> **Board refinement (2026-07-08, from `/pm` + `/pm-brainstorm` conventions):** m3 and m5 retrofitted to the current canon — `## Source + Goal linkage` sections, the two standing closing tasks (Simplify, Test coverage), and pre-`lego/` paths fixed; m3/t001 (done) moved to `m3/done/`. m5/t003 reduced to verify-and-close (its webhook shipped in bex-api via w2's deploy-from-chat — where this refinement had independently concluded it belongs; operator stays mechanism-only). Planned m4/m7 retrofits and an m2/t007 rewrite were dropped — those milestones completed upstream first. No milestone was removed — all open ones map to a V0 roadmap item or Render parity and pass the DO_NOT_DO screen.

## Inbox

- `005.md` — Wire `spec.healthCheckPath` into a ReadinessProbe (or drop the field) — from the 2026-07-08 docs-vs-code audit
- `006.md` — Triage 36 Dependabot findings (2 critical, 15 high) reported 2026-07-08
- `008.md` — Per-service autoscaling config (Render `PUT …/autoscaling`) — from the m13 audit; gated on **m3** settling the metric→replica loop it reuses
- `012.md` — Static sites (Render `static_site` type) — split from **m15** (build→CDN; a larger effort than the compute service types)
- `013.md` — Managed Postgres HA: high availability + failover + read replicas — split from **m17** (needs a replicated CNPG cluster)

> **Promoted 2026-07-08:** `007`→**m14**, `009`→**m15**, `010`→**m16**, `011`→**m17** (notes moved to `done/`); `008` kept as a note (gated on m3). See the m13 note above.

> `003` (custom-domains API) promoted to **m11** and `004` (scale API) promoted to **m12** on 2026-07-08; notes moved to `done/`. m12 was subsequently relocated to **w2** (done: `w2/done/m12/`).

> **m6 (Multi-tenant isolation) removed 2026-07-07** — the plan leaned on vcluster-per-tenant, which is the wrong isolation model for bex (see [`.pm/DO_NOT_DO.md`](../DO_NOT_DO.md)). If tenant isolation is re-scoped later, it must be namespace-tier → microVM, not per-tenant virtual control planes.
