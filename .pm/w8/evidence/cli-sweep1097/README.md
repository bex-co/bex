# CLI sweep 1097 — env vars inline at create (live, 2026-10-20)

One disposable free web service created and deleted (204). No new bug filed.

- Create with an inline `envVars` array → 201, and the var is immediately readable through the env-vars endpoint with its first revision. Create-time env lands in the same store as post-create PUTs.
