# CLI sweep 1166 — env-var verb discipline (live, 2026-10-20)

One disposable free web service created and deleted (204, verified 404). No new bug filed.

- PATCH on a single env key, POST on the env-vars collection, and DELETE on the collection all → 405. The collection speaks GET/PUT (replace-all batch) and the single key speaks GET/PUT/DELETE (sweeps 1131–1132, 1165); every other verb refuses as method-not-allowed rather than guessing an intent.
