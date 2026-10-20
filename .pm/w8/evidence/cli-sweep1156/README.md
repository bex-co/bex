# CLI sweep 1156 — project lifecycle; environments are not nested CRUD (live, 2026-10-20)

One disposable project created and deleted (204, verified 404). No new bug filed.

- Create → 201 with `environmentIds: []` inline; GET → 200; PATCH rename → 200; DELETE → 204; GET after → 404.
- `POST /projects/{id}/environments` → 404: there is no nested environment-create route under a project (environments are managed through their own surface and linked, matching the project link probes from earlier sweeps).
