# CLI sweep 1134 — deploy trigger requires a body object (live, 2026-10-20)

One disposable free web service created and deleted (204, verified 404). No new bug filed.

- `POST /services/{id}/deploys` with `{}` → 201; with `{"clearCache":"do_not_clear"}` → 201; with `{"clearCache":"maybe"}` → 400 (enum validated); with no body at all → 400.

The deploy trigger is the mirror of the lifecycle verbs from sweep 1126: it decodes a body object (an empty object is the canonical "no options" form the CLI sends), while suspend/resume/restart accept no payload. Each endpoint enforces its own declared shape; three deploys total were recorded (initial + the two accepted triggers).
