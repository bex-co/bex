# CLI sweep 1018 — headers routes refuse non-static services by name (live, 2026-10-20)

One disposable free web service created and deleted (204). No new bug filed.

- `GET /services/{web-service}/headers` → coded 400 `service "…" is not a static_site`. The bulk headers/routes family is static-only and says so, rather than returning an empty list that would imply the feature applies. (The missing single-rule operations on static sites remain [w8/137](../../137.md).)
