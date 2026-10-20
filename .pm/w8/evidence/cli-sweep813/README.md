# CLI sweep 813 — workflows/jobs read surfaces + one-off-job refusal (live, 2026-10-20)

Read-only probes plus two refused creates on the baseline `hello-go` (nothing created; baseline untouched). No new bug filed.

- `GET /workflows?ownerId=…` → 200 `[]`; `GET /services/{hello-go}/jobs` → 200 `[]`.
- `POST /services/{hello-go}/jobs {"startCommand":"echo qa813"}` → 410 coded `ONE_OFF_JOBS_UNSUPPORTED` with actionable alternatives (`bex ssh`, pre-deploy command, cron job) — by design.
- Empty `startCommand` → bare 400 `bad request`: the minor wart already recorded in sweep 655, still not filed (the coded 410 carries the contract; the empty-body spelling is a validation-order nit).
- Pin re-checked: still `9d7ac8afa` → `ca30db3a3646`; w8/135's fix remains unpinned.
