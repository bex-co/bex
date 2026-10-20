# CLI sweep 792 — project + environment CRUD cycle (live, 2026-10-20)

Disposable project and environment created, verified, deleted (204s; both GETs 404 after). No new bug filed.

- First probe used a nested `POST /projects/{id}/environments` route that does not exist (404) — harness error: environments are top-level (`POST /v1/environments` with `projectId`, `lego/backend/internal/environments/rest.go:180`).
- Correct cycle: project create 201 → environment create 201 with default `ipAllowList` (allow-all v4+v6), `protectedStatus: unprotected` → project GET lists the env in `environmentIds` → `GET /environments?projectId=` returns it → env DELETE 204 → project DELETE 204 → both GETs 404 with named messages (`project not found` / `environment not found`).
