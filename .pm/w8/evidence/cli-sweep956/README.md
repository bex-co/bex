# CLI sweep 956 — jobs surface + w8/139 fix landed on main, not yet pinned (live, 2026-10-20)

Read-only probes on the baseline `hello-go` service. No new bug filed. No fixture created.

- Jobs list on a service with no jobs → `200 []`.
- Unknown job id with a malformed prefix → coded 400 `invalid path parameter "jobId"` (format validated before lookup).
- One-off job create → 410 `ONE_OFF_JOBS_UNSUPPORTED` with remediation text (documented by-design refusal; existing jobs remain listable).
- `/cron-jobs` top-level → 404; cron services list correctly via `/services?type=cron_job`.
- Status note: the w8/139 fix (`9e9c28e82`, REST accepts hyphenated KV spellings) is on `origin/main` but is **not** an ancestor of the production pin `3ede47d3c5a9` — live closure waits for the next pin, same as w8/137.
