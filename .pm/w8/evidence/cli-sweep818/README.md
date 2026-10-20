# CLI sweep 818 — cron create refusals: never-fires schedule + paid gate (live, 2026-10-20)

No fixture survived (both creates refused; nothing to clean). No new bug filed.

- `--cron-schedule "0 0 31 2 *"` → 400 coded `SCHEDULE_NEVER_FIRES`: "schedule … never fires: no date and time matches all of its fields (e.g. February 31)" — the schedule validator proves unsatisfiability, not just syntax (the w9/m91 robfig-parser hardening family).
- A valid rare schedule (`30 3 29 2 *`) → named 400 `a cron_job requires a paid plan; plan must be one of starter|standard|pro|pro_plus|pro_max|pro_ultra`. Cron jobs have no free tier on bex, so the manual-run cycle from the ADR018 cron row could not be exercised without a paid fixture (never created, per loop rules); the run-history read surface was covered in earlier rotations.
