# CLI sweep 1043 — workflows REST list exists; no per-service tasks route (live, 2026-10-20)

One disposable free web service created and deleted (204). No new bug filed.

- `GET /workflows?ownerId=…` → 200 (the workflow-services list behind the CLI's `workflows list`).
- `GET /services/{id}/tasks` → 404: task runs are addressed through the workflow surface, not nested under arbitrary services.
