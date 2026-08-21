# w9 · m91 — Cron job schedule validation + Render form consistency

**Worker:** worker9 **Goal:** a cron job can never be saved with an invalid schedule, and the cron create/edit forms match Render's cron UX **Status:** done

## Tasks (in order)

| id   | title                                                            | est | depends_on          |
| ---- | ---------------------------------------------------------------- | --- | ------------------- |
| t001 | Server-side schedule validation on the cron update path          | 45m | — — **DONE**        |
| t002 | Dashboard Settings edit-form schedule validation parity          | 40m | t001 — **DONE**     |
| t003 | Cron create-form chrome: "New Cron Job" title/heading/subtitle   | 30m | — — **DONE**        |
| t004 | Schedule UX parity: human-readable preview + pre-fill + UTC note | 45m | t002 — **DONE**     |
| t005 | Render parity check (REST/GraphQL/MCP + dashboard)               | 30m | t001–t004 — **DONE** |
| t006 | Simplify                                                         | 20m | t005 — **DONE**     |
| t007 | Test coverage                                                    | 40m | t005 — **DONE**     |
| t008 | Closeout                                                         | 10m | t007 — **DONE**     |

## Outcome (2026-08-20)

Root cause found (broader than the QA hypothesis): both the create and update
paths validated the schedule with a **field-count-only** check (`validCronSchedule`
server-side, `isValidCron` client-side), so `99 99 * * *` — 5 fields, out of range
— passed everywhere and only the Kubernetes apiserver rejected it, flipping the App
to `Failed`. Fix strengthened both validators: bex-api now parses with
`github.com/robfig/cron/v3` `cron.ParseStandard` (the exact parser the k8s CronJob
controller uses) on **create and update** across REST/GraphQL/MCP; the dashboard
`isValidCron` range-checks every field so both the create form and the Settings
editor refuse it up front. Plus the Render UI-parity polish: cron-aware "New Cron
Job" chrome, a live `describeCron` human-readable preview + "runs in UTC" note on
both forms, and a pre-filled `*/5 * * * *` default on the cron deep link.

Verified: backend `go test ./internal/apps/` + `make lint-backend` (0 issues) +
`go vet`; dashboard `yarn typecheck` + `yarn lint` + full `yarn test` (2397 green).
Tests added: `service_types_test.go` (create reject/accept), `cron_settings_test.go`
(update range-invalids), `cron.test.ts`, `services.new.test.tsx`,
`cron-deploy-section.test.tsx`. Docs: ADR038 + ADR018 cron row. Residual: live
prod-cluster acceptance (save `99 99 * * *` → 400, App stays healthy) was not run
in-session (no cluster); the parser-parity with k8s makes it a formality.

## Definition of done

- Saving an invalid crontab (e.g. `99 99 * * *`, `not a cron`) on the cron **Settings** page is rejected **before** any App CR is written — the service never transitions to `Failed` from a bad user-supplied schedule. Rejection uses bex-api's Render-compatible error shape (`core.WriteErr`, `message` field) on REST/GraphQL/MCP, and the dashboard edit form disables Save + shows an inline error exactly like the create form does today.
- The cron create page reads "New Cron Job" (page title, `<h1>`, and subtitle) instead of the generic "New Service" / "Deploy a web service…" copy.
- The Schedule field on both create and edit shows a live human-readable translation of a valid cron expression (e.g. `*/5 * * * *` → "every 5 minutes"), pre-fills a sensible default on the create form, and states that schedules run in **UTC**.
- Regression tests assert the server rejects invalid schedules on the update path and accepts valid ones; a dashboard test asserts the edit form blocks save on an invalid schedule.

## Source + Goal linkage

- **Source:** QA pass on `dashboard.bex.co` cron jobs (2026-08-20, Playwright) + a side-by-side comparison against `dashboard.render.com/cron/new`. Confirmed bug: the create form validates the crontab (inline error, Deploy disabled) but the **Settings edit form** accepts `99 99 * * *`, saves it ("Cron job settings saved"), and the App flips **Running → Failed**. Render validates the schedule on both create and edit (`Invalid cron spec` + "There are errors above"). Render also shows a live "every 5 minutes" preview, pre-fills `*/5 * * * *`, and titles the page "New Cron Job"; bex shows none of these and reuses "New Service" chrome. See `docs/ADR038-cron-jobs.md`, `docs/ADR018-render-parity.md`.
- **Goal linkage:** Render parity / correctness — bex-api is meant to be Render-compatible (`docs/ADR006-bex-api.md`), and a user-supplied value that silently breaks a running resource is a correctness defect, not just a cosmetic gap.
- **Expected outcome:** invalid cron schedules are impossible to persist from any surface; the cron form is visibly a cron form and reduces crontab-syntax mistakes the way Render's does.
- **Why now:** the validation gap is a live, reproduced bug that puts a service into `Failed` with no user feedback — the highest-severity item from the QA pass. The consistency polish rides the same forms and error path, so fixing them together avoids touching the cron form twice.
- **Render parity task included** because this changes user-facing behavior across REST/GraphQL/MCP (schedule validation on update) and the dashboard (edit-form validation + create-form chrome).
