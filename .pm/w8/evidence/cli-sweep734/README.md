# CLI sweep 734 — job subresource not-found dialects (live, 2026-10-19)

Disposable free web service `qa734-web` (`srv-dbbch6i720pc73c0kltg`). Deleted and verified absent (REST 404).

- Malformed job id (22-char suffix) on GET/cancel → 400 `invalid path parameter "jobId"` at the validator.
- Well-formed synthetic (`job-` + 20) → 404 `job not found` on both GET and `POST …/jobs/{id}/cancel`.
- `GET /services/{id}/jobs?status=running` → 200 `[]` (list filter accepted; the create side remains the sweep-655 410).

No bug filed.
