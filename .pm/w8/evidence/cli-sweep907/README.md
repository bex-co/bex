# CLI sweep 907 — concurrent double-delete race (live, 2026-10-20)

Disposable free web service created, deleted by two simultaneous requests, verified absent. No new bug filed.

- Two parallel `DELETE /services/{id}` → exactly one 204 and one coded 404, no 500 and no stuck state (the sequential repeat-delete of sweep 889 under concurrency). Deletion is race-safe: the loser gets the honest not-found, not an error or a phantom success.
- Housekeeping note: the local tree is down to 8 uncommitted paths — the parallel worker has shipped the loop's recent evidence again.
