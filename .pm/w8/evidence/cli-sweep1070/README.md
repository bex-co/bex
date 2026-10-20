# CLI sweep 1070 — instances are list-only (live, 2026-10-20)

One disposable free web service created and deleted (204). No new bug filed.

- `GET /services/{id}/instances` lists the running instance (sweep 996). Per-instance GET and DELETE → 404: instances have no individual REST resource — lifecycle is managed through the service (restart/scale/suspend), not by addressing instances directly.
