# CLI sweep 1140 — env-group lifecycle is clean and idempotent-shaped (live, 2026-10-20)

One disposable env group created and deleted (204, verified 404). No new bug filed.

- Create → 201 with empty `envVars`/`secretFiles` arrays inline; batch var PUT → 200.
- Linking an unknown service id → 404 (the link route validates the service, it does not store a dangling reference).
- DELETE → 204, GET after → 404, second DELETE → 404.
