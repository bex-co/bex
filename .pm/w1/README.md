# w1 — bex platform roadmap (worker1)

**Worker:** worker1 Converted from the `.tmp/` backlog (items 001–010) on 2026-07-02. Ordered roughly by priority/dependency: de-risk the live system, build the source-of-truth control plane, then the elastic/cost machinery, then pipeline, isolation, and hardening.

## Milestones

- [x] **m1** — Reliability: fix config drift + back up etcd (4 tasks) ← from `009`, `007` — done 2026-07-05, moved to `done/m1/`
- [x] **m2** — Control plane: Postgres source of truth in `lego/backend` (7 tasks; DONE 2026-07-09 — committed `aebbd43`, prod `BEX_CP_DB_URI` on since m9, live acceptance via `scripts/auth-tenant-e2e.sh` in m9/t004) ← from `005`, moved to `done/m2/`
- [x] **m2.5** — Refactor bex-api into feature packages (one package per feature) (9 tasks) ← from `/pm` architecture review 2026-07-06 — done (shipped `06f247e` 2026-07-06, verified + board synced 2026-07-08), in `done/m2.5/`
- [x] **m3** — Elastic substrate: bin-pack + autoscale (8 tasks) ← from `002`, `004` (retrofitted to current `/pm` canon 2026-07-08) — done 2026-07-11, moved to `done/m3/` (t008 unblocked by m19: 014 repaired, scheduler --config live, CI green; elastic behaviors proven on prod)
- [x] **m4** — Free tier = sleep: scale-to-zero + wake activator (5 tasks) ← from `003` — done 2026-07-08, moved to `done/m4/`
- [x] **m4.5** — Sleep in the dashboard: hibernated state, idle-timeout setting, wake UX (6 tasks; DONE 2026-07-09 — Sleeping badge + idle-timeout control, verified live) ← user request 2026-07-08, UI half of m4, moved to `done/m4.5/`
- [x] **m5** — Build & deploy from git, in-cluster (5 tasks; DONE 2026-07-09 — in-cluster BuildKit builds, verified live, simplified + tested; unblocks w2/m2 t004) ← from `008`, moved to `done/m5/`
- [x] **m7** — Prod hardening: network · secrets · images (6 tasks) ← from `010` (+ OpenBao backup, added 2026-07-08) — done 2026-07-08, moved to `done/m7/`
- [x] **m8** — Instance tiers: one catalog, Render-shaped plan API, limits everywhere (6 tasks) ← from architecture discussion 2026-07-08 (metrics page's "No limit configured" gap; Render compute-plans ladder) — done 2026-07-08, moved to `done/m8/`
- [x] **m9** — Tenant onboarding: real workspaces + OpenFGA enforced in prod (6 tasks; DONE 2026-07-09 — mint-on-first-login, key→tenant binding, workspace-scoped authz, prod OpenFGA flip, simplified + tested) ← from `/pm-brainstorm for w1` 2026-07-08 (m2 deferrals; prod authz is allow-all), moved to `done/m9/`
- [x] **m10** — OpenBao prod wiring: env-vars live in prod (6 tasks) ← from `/pm-brainstorm for w1` 2026-07-08 (docs/ADR013-secrets.md "Prod deploy path") — done 2026-07-08 (implementation shipped + locally validated), moved to `done/m10/`; prod activation (first init + live PUT) is the operator's runbook there
- [x] **m11** — Render custom-domains API over `App.spec.hosts[]` (5 tasks) ← promoted from `003` 2026-07-08
- [x] **m11.5** — Custom-domains dashboard section, UI half of m11 (8 tasks) ← promoted from `w5/003` 2026-07-09 — done 2026-07-09, moved to `done/m11.5/`
- [x] **m13** — Render parity audit: REST · GraphQL · MCP · UI matrix (6 tasks) ← user request 2026-07-08 — done 2026-07-08, moved to `done/m13/` (shipped `docs/ADR018-render-parity.md`; 5 gap notes filed: `008`–`011`, `w3/002`)
- [x] **m14** — Key Value (Valkey/Redis) managed store: `KeyValue` CRD + reconciler (6 tasks) ← promoted from `007` 2026-07-08 (mechanism; REST/GraphQL/MCP surfaces done in w2/m7, UI in w5/m12) — done 2026-07-09, moved to `done/m14/`
- [x] **m15** — Additional service types: background worker + cron job (8 tasks) ← promoted from `009` 2026-07-08 (static site split to `012`) — done 2026-07-09, moved to `done/m15/` (had been staged under `w7/m15/`)
- [x] **m16** — Config surfaces beyond env vars: environment groups + secret files (7 tasks) ← promoted from `010` 2026-07-08
- [x] **m17** — Managed Postgres advanced: data-protection + lifecycle + access (8 tasks) ← promoted from `011` 2026-07-08 (HA/replicas split to `013`) — done 2026-07-09, moved to `done/m17/` (HA/replicas remain in note `013`)
- [x] **m18** — Root Directory: CRD + build engine + webhook path-filter + API surface (10 tasks) ← from `/pm-brainstorm for w1` 2026-07-09 (Root Directory topic, monorepo support); UI half is `w5/m13` — done 2026-07-10, moved to `done/m18/`
- [ ] **m19** — Rearchitecture: rebuild the Hetzner substrate right (12 tasks) ← from docs/rearchitecture.md 2026-07-10 (absorbed into [docs/ADR002-architecture.md](../../docs/ADR002-architecture.md) §The production substrate, 2026-07-11; original in git history), promoted from `014` (CAPH-owned network · 3×CP tainted · platform/tenant pools · self-managed pivot · port firewall + WireGuard); closes `w1/014`, unblocks m3/t008, `008`, `013`
- [x] **m19.1** — 5-server interim: rescue prod + early pivot (9 tasks) ← from m19 t006's Hetzner-quota blocker + user decisions 2026-07-10/11 (pivot pulled forward, `bex-infra` destroyed to free the slot for the first tenant node; tenant pool floor becomes min 1; m19 resumes with a two-number revert after the quota raise) — done 2026-07-11, moved to `done/m19.1/`; prod serving, self-managed, verify-substrate green
- [x] **m20** — Per-service autoscaling (Render `PUT …/autoscaling`) (8 tasks) ← promoted from `008` 2026-07-11 (unblocked by m3 + m19) — done 2026-07-11, moved to `done/m20/`
- [ ] **m21** — Static sites (Render `static_site` type) (8 tasks) ← promoted from `012` 2026-07-11 (unlocks the CDN edge rules DO_NOT_DO parked until 012 lands)
- [ ] **m22** — Managed Postgres HA (Render `enableHighAvailability` + failover + read replicas) (9 tasks) ← promoted from `013` 2026-07-11 (unblocked by m17 + m19)
- [ ] **m23** — Misc: small parity + hardening/dev-infra chores (7 tasks) ← groups `005`, `006`, `015`, `016` 2026-07-11 (each sub-hour)
- [ ] **m24** — Multi-service `bex.yml`: Blueprint-shaped stack deploys (9 tasks) ← from `/pm-brainstorm more` 2026-07-12 (revives the 2026-07-09 proposal; DO_NOT_DO routes `fromDatabase` spec work to w1; all ingredients — types m15, env groups m16, Postgres m17, KV m14 — now shipped)

## Suggested execution order (2026-07-08 refinement, superseded 2026-07-09)

> All four items below are now done (m9, m5, m4.5 moved to `done/`; m3 remains open — see the Milestones list above for current status). Kept for provenance.

1. **m9** — tenant onboarding + enforced OpenFGA (m2/t007 marked done; prod authz is still allow-all — the standing security gap).
2. **m5** — build-from-git (t001/t002 are the last missing piece of the flow; the webhook half already shipped via w2).
3. **m3** — elastic substrate (replica-semantics contract settled by w2/m12; scale-down pays off most now that m4's sleep empties nodes).
4. **m4.5** — dashboard sleep UX (pairs with shipped m4; UI-half work).

> **Parity-backlog tier (promoted from the m13 audit, 2026-07-08):** **m14** Key Value · **m15** worker + cron service types · **m16** env-groups + secret-files · **m17** advanced Postgres. Sequence these after the V0 items above, ordered by the gap ranking in `docs/ADR018-render-parity.md`. **008** (per-service autoscaling) stays an inbox note until **m3** lands the node elasticity its replica scale-ups need (the metric→replica reconciler itself is new work in 008; m3 moves nodes, never `spec.replicas`).

> **m13 (parity audit) done 2026-07-08** — its output, `docs/ADR018-render-parity.md`, orders the remaining parity queue with evidence. Its gap notes were arranged into milestones on 2026-07-08: `007`→**m14**, `009`→**m15**, `010`→**m16**, `011`→**m17** (with `012` static-site + `013` Postgres-HA split off as notes); `008` (autoscaling) + `w3/002` (request logs) stay notes; the rest cross-reference existing owners (w2/m4 delete, w2/m5 deploys, w4/m8 keys-UI, w4/m12 members, w5/004 scaling-UI, w5/006 DNS-UI).

> **Board refinement (2026-07-08, from `/pm` + `/pm-brainstorm` conventions):** m3 and m5 retrofitted to the current canon — `## Source + Goal linkage` sections, the two standing closing tasks (Simplify, Test coverage), and pre-`lego/` paths fixed; m3/t001 (done) moved to `m3/done/`. m5/t003 reduced to verify-and-close (its webhook shipped in bex-api via w2's deploy-from-chat — where this refinement had independently concluded it belongs; operator stays mechanism-only). Planned m4/m7 retrofits and an m2/t007 rewrite were dropped — those milestones completed upstream first. No milestone was removed — all open ones map to a V0 roadmap item or Render parity and pass the DO_NOT_DO screen.

## Inbox

_(empty — all open notes promoted into milestones 2026-07-11; see below)_

> **Promoted 2026-07-11 (`/pm group them into milestones`):** `008`→**m20** (per-service autoscaling), `012`→**m21** (static sites), `013`→**m22** (Postgres HA); the four sub-hour notes `005`+`006`+`015`+`016`→**m23** (misc chores milestone — each below milestone size individually, grouped per the sizing rule). All seven notes moved to `done/`.
> **Promoted 2026-07-08:** `007`→**m14**, `009`→**m15**, `010`→**m16**, `011`→**m17** (notes moved to `done/`); `008` kept as a note (gated on m3). See the m13 note above. **Promoted 2026-07-10:** `014` (prod KCP unmanageable, m7 aftermath) → **m19** via `docs/rearchitecture.md` (since absorbed into ADR002-architecture.md); note moved to `done/`.

> `003` (custom-domains API) promoted to **m11** and `004` (scale API) promoted to **m12** on 2026-07-08; notes moved to `done/`. m12 was subsequently relocated to **w2** (done: `w2/done/m12/`).

> **m6 (Multi-tenant isolation) removed 2026-07-07** — the plan leaned on vcluster-per-tenant, which is the wrong isolation model for bex (see [`.pm/DO_NOT_DO.md`](../DO_NOT_DO.md)). If tenant isolation is re-scoped later, it must be namespace-tier → microVM, not per-tenant virtual control planes.
