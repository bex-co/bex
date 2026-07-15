# w9 — Deploy experience (worker9)

**Worker:** worker9 Created 2026-07-14 from a user request (`/pm for w9`): close the gap between bex's deploy UX and Render's. bex has the deploy _machinery_ (w2/m5 deploy history + trigger, w2/m10 cancel/rollback, w2/m30 manual-deploy body, w7/m28 build logs, w1/m33 pre-deploy logs) but not Render's deploy _experience_ — the per-deploy page every deploy action lands on. Ordered UX-first: the deploy detail page is the anchor other deploy-experience work (history tab, richer statuses) would hang off.

## Local dev environment

Develop against `.pm/w9/dev-9/`, this worker's own isolated stack on the shared local kind/CAPD cluster — never the shared cluster's default `auth`/`bex-system` namespaces or standard ports (5173/4433/4445/8090/8091/5432), which any other worker's session may also be using. `dev-9` gets its own Kratos + Hydra + Mailpit (namespace `dev-9-auth`) and app namespace (`dev-9`), reusing the shared cluster's CNPG operator and bex operator, plus a locally-built `bex-api` on dedicated ports derived from N=9 (`dev-9/ports.env`) so it never collides with any other workstream's `dev-N`.

- `bash .pm/w9/dev-9/up.sh` — bring it up (idempotent — safe to re-run)
- `bash .pm/w9/dev-9/status.sh` — health check (processes, pods, HTTP)
- `bash .pm/w9/dev-9/down.sh` — tear it down (leaves the shared cluster and every other workstream's `dev-N` untouched)

`up.sh` prints the dashboard command to point at it once bex-api is running.

## Milestones

- [x] **m1** — Deploy detail page: Manual Deploy jumps to a per-deploy page with its logs (9 tasks) ← from user request 2026-07-14
- [x] **m2** — Render CLI compatibility: run the official CLI against bex-api → `docs/cli-compatibility-checklist.md` (7 tasks) ← user decision 2026-07-14 (`/pm-brainstorm` round 8): never build a CLI from scratch (new `.pm/DO_NOT_DO.md` entry); verify `render-oss/cli` as the fifth surface instead
- [x] **m3** — Managed Postgres rename: stable `dpg-…` identity + mutable name, rolled through prod and every `dev-*` environment (12 tasks) ← user request 2026-07-14 + the Postgres half of `.pm/w1/done/021.md` / `docs/cli-compatibility-checklist.md` — done 2026-07-15, moved to `done/m3/`
- [x] **m10** — Env vars: `generateValue` + cursor pagination (8 tasks) ← from `/pm-brainstorm more milestones for each worker` 2026-07-14 (the ADR006/ADR018 env-vars row's two documented omissions; `w1/m35`'s generateValue prerequisite). Reassigned from w8 to w9 for capacity before closeout
- [x] **m37** — Maintenance mode: user-toggled interstitial + custom page (10 tasks) ← from `/pm-brainstorm more milestones for each worker` round 6, 2026-07-14; filed under worker1's ownership (`Worker: worker1` in its own README) but physically queued in w9's directory for capacity. `App.spec.maintenanceMode {enabled, uri}` — REST/GraphQL/MCP/dashboard, operator reuses the w1/m4 activator as the responder. DoD verified live on the mock cluster 2026-07-15.
