# CLI sweep 644 — cron schedule validation and plan gate (live, 2026-10-19)

No fixture survived: every probe was refused before creation; zero `qa644` services in list.

Via the pinned CLI (`services create --type cron_job --plan free --cron-command "echo hi"`):

| Schedule | Result |
| --- | --- |
| `bogus` | 400 `SCHEDULE_INVALID: schedule must be a valid 5-field cron expression (e.g. '0 * * * *')` |
| `99 99 99 99 99` | 400 same `SCHEDULE_INVALID` refusal |
| `*/5 * * * *` | 400 `a cron_job requires a paid plan; plan must be one of starter\|standard\|pro\|pro_plus\|pro_max\|pro_ultra` |
| `0 0 * * *` | 400 same paid-plan gate |

Schedule syntax is validated (with a named, coded refusal) before the plan gate; a valid schedule on free hits the paid-plan gate. No paid plan was submitted. No bug filed.
