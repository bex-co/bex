# CLI sweep 1161 — paid service types fail closed on free (live, 2026-10-20)

Refusal-side only; no service was created (fixture list verified empty). No new bug filed.

- `background_worker`, `private_service`, and `cron_job` creates with `plan: free` → 400 naming the requirement and the paid plan ladder (`starter|standard|pro|pro_plus|pro_max|pro_ultra`). The gate fires before provisioning; nothing was charged.
- A bogus cron schedule → 400 with code `SCHEDULE_INVALID` and an example of a valid 5-field expression — validated even though the plan gate would also refuse.
- A first cron probe with the command in the wrong envelope → 400 `unknown field "command"` (strict decoding, sweep 1141 discipline).
