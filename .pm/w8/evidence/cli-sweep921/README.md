# CLI sweep 921 — env-group environment assignment verbs (live, 2026-10-20)

Disposable env group created, moved/renamed, deleted (204, then GET 404). Pin re-checked (unchanged). No new bug filed.

- `PATCH /env-groups/{id}/environment {"environmentId": synthetic}` → coded 404 `environment not found` (the move target resolves before any write).
- `{"environmentId":""}` → 200 (detach to unassigned is a valid move, answered with the group object).
- Plain `PATCH {"name":…}` rename → 200 read back.
