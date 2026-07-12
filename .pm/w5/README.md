# w5 — Dashboard UI (worker5)

**Worker:** worker5 Scaffolds bex's human-facing dashboard: the client `bex-api`'s GraphQL adapter was already built to serve (`docs/ADR006-bex-api.md` calls it "Render dashboard compatible"). Ordered: stand up an empty, rebranded app shell first so a later milestone can wire it to `bex-api`'s queries/mutations without inheriting beancount's domain code.

## Milestones

- [x] **m1** — Scaffold dashboard from beancount-dashboard, stripped to sample content (7 tasks) ← from user request 2026-07-05 — done 2026-07-06, moved to `done/m1/`
- [x] **m2** — Polish dashboard UI: beancount-style layout, remove Ory branding (7 tasks) ← from user request 2026-07-06 — done 2026-07-06, moved to `done/m2/`
- [x] **m3** — Internationalization (i18n), including Ory Elements (8 tasks) ← from user request 2026-07-06 — done 2026-07-06, moved to `done/m3/`
- [x] **m4** — Services list + lifecycle actions, Render-consistent, wired to bex-api (7 tasks) ← from `/pm-brainstorm` 2026-07-06 — done 2026-07-08, moved to `done/m4/`
- [x] **m5** — Service overview page + Render-style service IA (Overview / Logs) (6 tasks) ← from `/pm-brainstorm` 2026-07-06 — done 2026-07-06, moved to `done/m5/`
- [x] **m6** — Live logs page (Render-consistent: historical query + SSE live-tail) (6 tasks) ← from `/pm-brainstorm` 2026-07-06 — done 2026-07-08, moved to `done/m6/`
- [x] **m7** — Service Settings + instance-type picker (Render parity) (6 tasks) ← from user request 2026-07-08, Render settings/plan pages captured live; UI half of w1/m8's plan API — done 2026-07-08, moved to `done/m7/`
- [x] **m8** — Databases page (managed Postgres, Render-consistent) (8 tasks) ← promoted from `001` via `/pm-brainstorm` 2026-07-08 — done 2026-07-08, moved to `done/m8/`
- [x] **m9** — MCP `query_render_postgres` — read-only SQL for agents (5 tasks) ← from `/pm-brainstorm for w2` 2026-07-08, reassigned to w5 (original `w2/m6` label collided with w5's done m6) — done 2026-07-08, moved to `done/m9/`
- [x] **m10** — Trustworthy dev stub (multi-service) + custom-domain DNS instructions end to end (9 tasks) ← from user report 2026-07-09 (phantom service in `local-bex`) + promoted `006` — done 2026-07-09, moved to `done/m10/`
- [x] **m11** — Type-aware service settings (cron jobs: hide Custom Domains + Idle timeout; show Schedule + Command) (6 tasks) ← from Playwright comparison 2026-07-09 — done 2026-07-09, moved to `done/m11/`
- [x] **m12** — Key Value dashboard (create / list / detail, Render-consistent) (9 tasks) ← from user parity report 2026-07-09 (dashboard.render.com/new/redis); UI half of `w2/m7`, on the w1/m14 mechanism — done 2026-07-09, moved to `done/m12/`
- [x] **m13** — Build & Deploy settings section (Root Directory) (8 tasks) ← from `/pm-brainstorm for w1` 2026-07-09 (Root Directory topic); UI half of `w1/m18`, gated on it — done 2026-07-10, moved to `done/m13/`
- [x] **m14** — Delete service: dashboard danger-zone action (8 tasks) ← from `/pm-brainstorm new milestones` 2026-07-09, unblocked — w2/m4 done 2026-07-09; done 2026-07-11, moved to `done/m14/`
- [x] **m15** — New-service create wizard (source picker · repo picker · deploy) (9 tasks) ← from `/pm-brainstorm for w2` 2026-07-11 (Render `/web/new` parity); done 2026-07-11, moved to `done/m15/`
- [x] **m16** — Manual-scaling section in service Settings (9 tasks) ← promoted from `004` 2026-07-11, unblocked by `w2/m12` (backend scale API shipped 2026-07-08) — done 2026-07-12, moved to `done/m16/`

## Inbox

- `007.md` — Events tab + deploy list UI (Rollback/Cancel) — gated on `w3/m7` + `w2/m10` (backends materialized 2026-07-12)

> `001.md` promoted to m8; `002.md` retired as superseded by open `w4/m8` (API keys in the dashboard) — both moved to `done/` 2026-07-08. `005.md` (ADR008-vision.md non-goal fix) done alongside m8 — moved to `done/` 2026-07-08. `003.md` promoted to `w1/m11.5` (custom-domains dashboard) on 2026-07-09 — moved to `done/`. `006.md` (post-add DNS/CNAME instructions) shipped in m10 — moved to `done/` 2026-07-09. `004.md` promoted to **m16** (manual-scaling settings) 2026-07-11 — moved to `done/`.
