# w5 — Dashboard UI (worker5)

**Worker:** worker5 Scaffolds bex's human-facing dashboard: the client `bex-api`'s GraphQL adapter was already built to serve (`docs/bex-api.md` calls it "Render dashboard compatible"). Ordered: stand up an empty, rebranded app shell first so a later milestone can wire it to `bex-api`'s queries/mutations without inheriting beancount's domain code.

## Milestones

- [x] **m1** — Scaffold dashboard from beancount-dashboard, stripped to sample content (7 tasks) ← from user request 2026-07-05 — done 2026-07-06, moved to `done/m1/`
- [x] **m2** — Polish dashboard UI: beancount-style layout, remove Ory branding (7 tasks) ← from user request 2026-07-06 — done 2026-07-06, moved to `done/m2/`
- [x] **m3** — Internationalization (i18n), including Ory Elements (8 tasks) ← from user request 2026-07-06 — done 2026-07-06, moved to `done/m3/`
- [x] **m4** — Services list + lifecycle actions, Render-consistent, wired to bex-api (7 tasks) ← from `/pm-brainstorm` 2026-07-06 — done 2026-07-08, moved to `done/m4/`
- [x] **m5** — Service overview page + Render-style service IA (Overview / Logs) (6 tasks) ← from `/pm-brainstorm` 2026-07-06 — done 2026-07-06, moved to `done/m5/`
- [x] **m6** — Live logs page (Render-consistent: historical query + SSE live-tail) (6 tasks) ← from `/pm-brainstorm` 2026-07-06 — done 2026-07-08, moved to `done/m6/`
- [x] **m7** — Service Settings + instance-type picker (Render parity) (6 tasks) ← from user request 2026-07-08, Render settings/plan pages captured live; UI half of w1/m8's plan API — done 2026-07-08, moved to `done/m7/`
- [ ] **m8** — Databases page (managed Postgres, Render-consistent) (8 tasks) ← promoted from `001` via `/pm-brainstorm` 2026-07-08

## Inbox

- `003.md` — Custom-domains section in service Settings — blocked on `w1/003` (backend API half)
- `004.md` — Manual-scaling section in service Settings — blocked on `w1/004` (backend API half)
- `005.md` — Fix `docs/vision.md` stale "managed databases" non-goal — sub-hour, can ride with m8/t001

> `001.md` promoted to m8; `002.md` retired as superseded by open `w4/m8` (API keys in the dashboard) — both moved to `done/` 2026-07-08.
