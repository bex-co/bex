# w5 — Dashboard UI (worker5)

**Worker:** worker5 Scaffolds bex's human-facing dashboard: the client `bex-api`'s GraphQL adapter was already built to serve (`docs/bex-api.md` calls it "Render dashboard compatible"). Ordered: stand up an empty, rebranded app shell first so a later milestone can wire it to `bex-api`'s queries/mutations without inheriting beancount's domain code.

## Milestones

- [x] **m1** — Scaffold dashboard from beancount-dashboard, stripped to sample content (7 tasks) ← from user request 2026-07-05 — done 2026-07-06, moved to `done/m1/`
- [x] **m2** — Polish dashboard UI: beancount-style layout, remove Ory branding (7 tasks) ← from user request 2026-07-06 — done 2026-07-06, moved to `done/m2/`
- [x] **m3** — Internationalization (i18n), including Ory Elements (8 tasks) ← from user request 2026-07-06 — done 2026-07-06, moved to `done/m3/`
- [ ] **m4** — Services list + lifecycle actions, Render-consistent, wired to bex-api (7 tasks) ← from `/pm-brainstorm` 2026-07-06
- [x] **m5** — Service overview page + Render-style service IA (Overview / Logs) (6 tasks) ← from `/pm-brainstorm` 2026-07-06 — done 2026-07-06, moved to `done/m5/`
- [ ] **m6** — Live logs page (Render-consistent: historical query + SSE live-tail) (6 tasks) ← from `/pm-brainstorm` 2026-07-06

## Inbox

- `001.md` — Databases page (managed Postgres) — candidate milestone
- `002.md` — API-keys settings page — candidate milestone
