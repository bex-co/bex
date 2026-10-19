# CLI sweep 557 — cron schedule dialects through blueprint validate (live)

**Verdict: no bug filed.** Same isolated QA session (`puncsky@gmail.com`), `bex-canary`, HEAD-built CLI at `d98d07f62`, production pin still `f4e38371055f`, 2026-10-19T12:10–12:11Z. Validate-only: no production resource created or mutated, baseline remained 8 services.

Sixteen manifests (image-runtime cron, `busybox:1.36`, varying only `schedule`) were driven through `bex blueprints validate -o json` and compared against the repo's shared acceptance oracle — `checkCronSchedule` in `lego/backend/internal/apps/service.go` (robfig `ParseStandard` plus a `Next(now)` never-fires check) and `lego/backend/internal/apps/testdata/cron-schedule-vectors.json`. Live behavior matched the source contract in every cell:

- **Valid controls (exit 0):** `0 2 * * *`; `0 0 ? * *` (`?` reads as `*`); `0 0 * * MON-FRI`; leap-day `0 0 29 2 *` (robfig's five-year `Next` horizon reaches 2028, so it is not mistaken for never-firing).
- **Malformed (exit 1, located at `services[0].schedule`, "must be a valid 5-field cron expression"):** `@daily` (descriptors refused — exactly-5-fields, matching Render), six-field and four-field forms, day-of-week `7` (0–6 only, w1/m145), minute `99`, step `*/0`.
- **Never-fires (distinct located message echoing the schedule):** `0 0 31 2 *` (February 31), `0 0 31 4 *` (April 31), and the bare-comma minute `, * * * *` whose list parses to no values — all three refused up front instead of silently never running (w4/197).
- **Presence/shape dialects:** empty string → semantic "schedule is required for a cron_job"; omitted key → schema "missing property 'schedule'"; `schedule` on a `type: web` service → schema "additional properties 'schedule' not allowed".

Note: validate errors carry `path` + message text, not the `SCHEDULE_INVALID`/`SCHEDULE_NEVER_FIRES` machine codes — the same error shape every blueprint refusal uses, with the two cases still distinguishable by message.

## Unrun

- The full ~60-row vectors table live (spot-checked the load-bearing rows; the table is already asserted surface-by-surface in source tests).
- Create/update with these schedules on a real cron fixture (cron is a paid-only type, w8/048 — no paid submission from QA; validate exercises the same `checkCronSchedule`).
- Cron run trigger/cancel history (needs an owned cron; baseline crons are pre-existing and untouched).
- W8/129–131 live verification still waits for the production image pin to pass `5ae176831`.
