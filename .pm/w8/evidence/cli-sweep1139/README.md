# CLI sweep 1139 — secret files are named PUT/GET/DELETE resources (live, 2026-10-20)

Two disposable free web services created and deleted (204, verified 404). No new bug filed.

- `POST /services/{id}/secret-files` → 405: there is no collection-create. The list reads `[]` on a fresh service and 404s after the service is deleted.
- `PUT /secret-files/{name}` → 201 and returns `{name, content}`; the named GET returns the same content to the authenticated owner (the dedicated read path, unlike general service reads which never carry secrets). Re-PUT of the same name → 201 again (upsert).
- `DELETE /secret-files/{name}` → 204; GET after → 404; a second DELETE → 404.
