# CLI sweep 987 — delete lifecycle edges (live, 2026-10-20)

One disposable free web service created, suspended, and deleted. No new bug filed.

- Deleting a suspended service → 204 (suspension is not a delete guard).
- Deleting the same id again → 204 (idempotent delete), while GET and PATCH on the deleted id → 404. The delete/write asymmetry matches the custom-domain delete in sweep 970: deletes tolerate repetition, reads and writes do not resurrect.
