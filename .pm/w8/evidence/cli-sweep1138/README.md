# CLI sweep 1138 — logs live at `/v1/logs`, not nested under services (live, 2026-10-20)

Three disposable free web services created and deleted (204, verified 404). No new bug filed.

- `GET /services/{id}/logs` → 404: services have no nested logs route (only Postgres/Key Value do). App logs are the flat query surface the CLI's `logs` command uses.
- `GET /v1/logs?resources=…` → 400 naming the unsupported parameter; the declared parameter is the singular `resource`. With `?resource={id}` → 200 with an empty page (`logs: []`, `hasMore: false`) for a just-created service.
- `?resource=srv-000…` (unknown) → coded 404. `/logs/values` without its required `label` parameter → 400 naming it.

Parameter discipline on the logs surface matches the rest of the API: unsupported names refuse, required names are named, unknown resources 404.
