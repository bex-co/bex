# CLI sweep 922 — service placement cannot be PATCHed directly (live, 2026-10-20)

Disposable free web service created, probed, deleted (204, then GET 404). No new bug filed.

- `PATCH /services/{id} {"environmentId":…}` → 400 unknown field: placement is not a service-PATCH field; it changes only through the projects/environments link verbs (sweep 793), so there is one write path for membership.
- `PUT /projects/{synthetic}/service-links` → coded 404 `project not found` (the container resolves before any link write).
