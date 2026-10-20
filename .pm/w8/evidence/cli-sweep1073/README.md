# CLI sweep 1073 — environment membership is not a service PATCH field (live, 2026-10-20)

One disposable free web service created and deleted (204). No new bug filed.

- `PATCH {environmentId}` → coded 400 `request body contains unknown field "environmentId"`. Environment membership moves through the project link endpoints (sweep 966) and the environment layer, not the service update body.
