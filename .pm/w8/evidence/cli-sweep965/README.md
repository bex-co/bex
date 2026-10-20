# CLI sweep 965 — projects create shape and environment nesting (live, 2026-10-20)

Two disposable projects created and deleted (204s; GETs 404; name list empty). No new bug filed.

- Project create → 201 with `environmentIds: []`; bogus owner → 403; empty name → coded 400.
- Environments are created inline in the project create body (`environments: [{name}]` → the new env id appears in `environmentIds`). A nested `POST /projects/{id}/environments` 404s — that route family does not exist; the registered route table (`projects/rest.go`) confirms environments are create-inline plus project-level link endpoints (`service-links`, `database-links`, `keyvalue-links`), not a nested CRUD resource. Recorded as the designed shape, not a gap.
- Deleting the project removes it and its inline environment (project GET 404 afterwards).
