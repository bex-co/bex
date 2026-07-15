# w1 — bex platform roadmap (worker1)

**Worker:** worker1 Converted from the `.tmp/` backlog (items 001–010) on 2026-07-02. Ordered roughly by priority/dependency: de-risk the live system, build the source-of-truth control plane, then the elastic/cost machinery, then pipeline, isolation, and hardening.

## Local dev environment

Develop against `.pm/w1/dev-1/`, this worker's own isolated stack on the shared local kind/CAPD cluster — never the shared cluster's default `auth`/`bex-system` namespaces or standard ports (5173/4433/4445/8090/8091/5432), which any other worker's session may also be using. `dev-1` gets its own Kratos + Hydra + Mailpit (namespace `dev-1-auth`) and app namespace (`dev-1`), reusing the shared cluster's CNPG operator and bex operator, plus a locally-built `bex-api` on dedicated ports derived from N=1 (`dev-1/ports.env`) so it never collides with any other workstream's `dev-N`.

- `bash .pm/w1/dev-1/up.sh` — bring it up (idempotent — safe to re-run)
- `bash .pm/w1/dev-1/status.sh` — health check (processes, pods, HTTP)
- `bash .pm/w1/dev-1/down.sh` — tear it down (leaves the shared cluster and every other workstream's `dev-N` untouched)

`up.sh` prints the dashboard command to point at it once bex-api is running.

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
- [x] **m21** — Static sites (Render `static_site` type) (8 tasks) ← promoted from `012` 2026-07-11 (unlocks the CDN edge rules DO_NOT_DO parked until 012 lands); done 2026-07-12 (moved to `done/m21`)
- [x] **m22** — Managed Postgres HA (Render `enableHighAvailability` + failover + read replicas) (9 tasks) ← promoted from `013` 2026-07-11 (unblocked by m17 + m19) — done 2026-07-12, moved to `done/m22/`
- [x] **m23** — Misc: small parity + hardening/dev-infra chores (7 tasks) ← groups `005`, `006`, `015`, `016` 2026-07-11 (each sub-hour) — done 2026-07-12, moved to `done/m23/` (healthCheckPath→ReadinessProbe wired + envtested; Dependabot triaged + safe batch, residuals in `018.md`; mock workers labeled `bex.co/pool=platform` via CAPD template; stale single-node/data-loss comments + dead `10.0.0.0/16` swept)
- [x] **m24** — Multi-service `bex.yml`: Blueprint-shaped stack deploys (9 tasks) ← from `/pm-brainstorm more` 2026-07-12 (revives the 2026-07-09 proposal; DO_NOT_DO routes `fromDatabase` spec work to w1; all ingredients — types m15, env groups m16, Postgres m17, KV m14 — now shipped) — done 2026-07-13, moved to `done/m24/`
- [x] **m25** — Managed Postgres observability: processes · top-queries · sizes · table-scans · parameter-overrides (10 tasks) ← from `/pm-brainstorm more` 2026-07-12 (extends w1/m17, last open Postgres parity row) — done 2026-07-12, moved to `w2/done/m25/`
- [x] **m26** — Harden the build-image pull path (Zot node access, retention, drift guards) (10 tasks) ← promoted from `017` 2026-07-12 (found live during a routine backfill: no git-built image was reliably pullable on prod; autoscaler-minted nodes fail today without this) — done 2026-07-13, moved to `done/m26/` (t007 live-verified PASS on prod: a fresh autoscaled tenant node pulled a git-built image first-try, zero manual config)
- [x] **m27** — ~~Close the control-plane disaster-recovery gaps~~ **stale entry, fixed 2026-07-13**: this scope shipped as `w2/m27` (done 2026-07-12, `docs/ADR031-platform-data-backup.md`) — a concurrent `/pm-brainstorm` session materialized the identical proposal under `w2` before this README line was ever promoted to a real `w1/m27/` directory. No `w1/m27/` directory ever existed; nothing to move. See `w2/README.md`'s `m27` entry for the actual work.
- [x] **m28** — Gate deploys on real CI test runs (10 tasks) ← from a CI-workflow audit during `/pm-brainstorm more milestones to work on` 2026-07-13 (no `.github/workflows/*.yml` anywhere runs `go test`/`make test`/`yarn test`; `deploy.yml` builds+pushes+deploys on every push to `main` with zero test gate); not Render parity — pursued on reliability merits per user decision 2026-07-13 — done 2026-07-13, moved to `done/m28/`
- [x] **m29** — Managed Postgres external connectivity: SNI proxy for preamble-mode TLS clients (8 tasks) ← from `/pm-brainstorm more milestones to work on` 2026-07-13 (`docs/ADR009-postgresql-management.md:53` names the fix, never built) — done 2026-07-13, moved to `done/m29/`
- [x] **m30** — SIGTERM shutdown fix + Dependabot residual watch (6 tasks) ← groups `018`, `019` 2026-07-13 (each sub-hour), same pattern as `m23`/`w6/m15`/`w7/m10` — done 2026-07-13, moved to `done/m30/` (SIGTERM graceful-shutdown helper for both servers + regression test; 5/6 Dependabot residuals closed, `@tanstack/start-server-core` deferred; incidentally fixed a pre-existing m31 test regression that was reddening the dashboard suite)
- [x] **m31** — Projects: group services within a workspace (6 tasks) ← from `/pm-brainstorm more milestones to work on` 2026-07-13 (verified live via search that Render's Projects feature is real — render.com/docs/projects; scoped to grouping only, Environments deliberately deferred as a much larger follow-on)
- [x] **m32** — Environments: named subsets of a Project's services (11 tasks) ← follow-on to m31, closing the Environments half it deferred; rebuilt after an independent same-feature implementation on `w6/m16` collided with this milestone on `/ship` and was discarded in favor of composing with what had already shipped here — done 2026-07-13 (`internal/environments` layered on `internal/projects`; REST/GraphQL/MCP full CRUD; assignment auto-joins the project; live-verified against the CAPD mock cluster, `scripts/environments-verify.sh`; also fixed a 500-vs-503 gap `ErrEnvironmentsUnavailable` would have inherited from `projects.ErrProjectsUnavailable`'s equivalent unfixed gap), moved to `done/m32/`
- [x] **m33** — Pre-Deploy Command: gate rollout on a setup/migration step (9 tasks) ← from `/pm-brainstorm more` 2026-07-13 (`docs/ADR006-bex-api.md:124` lists `preDeployCommand` as an "ignored" bex.yml field — Render's Deploy section confirmed live via `w5/done/m13`; no overlap with `w6/m21`'s `dockerfilePath`/`startCommand` scope)
- [x] **m34** — Build filters: `buildFilter.paths` + `ignoredPaths` (9 tasks) ← from `/pm-brainstorm more milestones for each worker` 2026-07-14 (`docs/ADR018-render-parity.md` "buildFilter is not editable — ◐" row understates it: zero hits in `lego/`, the field is entirely unbuilt); generalizes w1/m18's `rootDirMatches` webhook path filter — done 2026-07-14, moved to `done/m34/`
- [x] **m35** — Blueprint field completeness: envVarGroups · fromGroup · sync:false · fromService (10 tasks) ← from `/pm-brainstorm more milestones for each worker` 2026-07-14 (ADR018 Blueprint row's named-error rejections; every ingredient shipped — m16 env groups, m24 stacks, w2/m15 verbs); `generateValue` acceptance rides `w8/m10`'s core verb — done 2026-07-14, moved to `done/m35/` (also landed `w8/m10/t001`, the shared `generateValue` core verb)
- [x] **m36** — Node bring-up efficiency: baked snapshot image + trimmed provisioning (8 tasks) ← promotes `.pm/FUTURE-MAYBE.md`'s node-bring-up entry 2026-07-14 — its trigger ("app images move into a pullable registry — zot wired end-to-end") fired when `w1/m26` closed 2026-07-13; the roll-safety condition that deferred it is gone for the same reason. **Done 2026-07-15**: baked `bex-worker` snapshot + tenant pool rolled live (54 s vs ~101–218 s, github/pkgs-independent); platform-pool roll spun out to `w1/022` (blocked on CNPG HA)
- [x] **m37** — Maintenance mode: Render-compatible public traffic interstitial (10 tasks) ← from `/pm-brainstorm` round 6, 2026-07-14; corrected against Render's live docs/OpenAPI/dashboard/event contract 2026-07-15 (paid web services; default/custom page; REST + Blueprint + UI + audit/webhook parity; NOT the ADR018 `/maintenance` managed-infra non-goal) — **done 2026-07-15**, executed and closed under w9's directory for capacity (`.pm/w9/done/m37/`; see `w9/README.md`)
- [ ] **m38** — Platform-pool drainability: CNPG HA + staged baked-image roll (7 tasks) ← promotes `022` (renumbered `done/023.md` — number collision with the done error-body note) 2026-07-15; the m36 Push 2b blocker, also closes the m19.1 `bex-db` single-copy risk
- [ ] **m39** — Dependency security: clear the 18-alert spike (7 tasks) ← from `/pm-brainstorm` round 10, 2026-07-15 (Dependabot 4 → 18 alerts — 7 critical, 3 high — during the round-9 push wave, likely the SSH-gateway dep surface and/or the vendored CLI checkout; the m23/m30 triage pattern at milestone size)

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


_(`018`, `019` promoted to `m30` 2026-07-13)_

> `021.md` (`restart` CLI fix — bare-name fallback in `cli/pkg/resource/service.go`) fixed 2026-07-14, moved to `done/`.

> `022.md` (REST error-body shape swallowing every CLI-visible error message) fixed 2026-07-15 (`dfff3034`), re-verified live against the official Render CLI — note moved to `done/`.

> A **second** `022.md` (platform-pool CNPG HA blocker, filed by m36's pre-flight — a number reused in a concurrent-session collision) promoted to **m38** 2026-07-15 and renumbered to `done/023.md`.

> **Promoted 2026-07-13:** `018` (Dependabot residual triage) and `019` (bex-api SIGTERM shutdown bug) → **m30**; both notes moved to `done/`.

> **Promoted 2026-07-12:** `017` (Zot node-pull path: DNS/TLS, NetworkPolicy drift, retention, generation churn, migration-ownership drift) → **m26**; note moved to `done/`.

> **Promoted 2026-07-11 (`/pm group them into milestones`):** `008`→**m20** (per-service autoscaling), `012`→**m21** (static sites), `013`→**m22** (Postgres HA); the four sub-hour notes `005`+`006`+`015`+`016`→**m23** (misc chores milestone — each below milestone size individually, grouped per the sizing rule). All seven notes moved to `done/`.
> **Promoted 2026-07-08:** `007`→**m14**, `009`→**m15**, `010`→**m16**, `011`→**m17** (notes moved to `done/`); `008` kept as a note (gated on m3). See the m13 note above. **Promoted 2026-07-10:** `014` (prod KCP unmanageable, m7 aftermath) → **m19** via `docs/rearchitecture.md` (since absorbed into ADR002-architecture.md); note moved to `done/`.

> `003` (custom-domains API) promoted to **m11** and `004` (scale API) promoted to **m12** on 2026-07-08; notes moved to `done/`. m12 was subsequently relocated to **w2** (done: `w2/done/m12/`).

> **m6 (Multi-tenant isolation) removed 2026-07-07** — the plan leaned on vcluster-per-tenant, which is the wrong isolation model for bex (see [`.pm/DO_NOT_DO.md`](../DO_NOT_DO.md)). If tenant isolation is re-scoped later, it must be namespace-tier → microVM, not per-tenant virtual control planes.
