# CLI sweep 1075 — environment save with save_only (live, 2026-10-20)

One disposable free web service created and deleted (204). No new bug filed.

- `PATCH /services/{id}/environment {saveMode:"save_only", envVars:[…]}` → 200, and the var reads back through the standard env-vars endpoint. The dashboard-style save path and the REST env path share one store; `save_only` records the change without forcing a deploy.
