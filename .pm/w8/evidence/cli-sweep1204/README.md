# CLI sweep 1204 — CLI deploys commands surface API errors faithfully (live, 2026-10-20)

One disposable free web service created and deleted (204, verified 404). No new bug filed.

- `bex deploys list <id> -o json` → exit 0 showing the initial deploy. `bex deploys cancel` on an unknown deploy → exit 1 carrying the API's coded 404 (`deploy not found`). `bex deploys list` on an unknown service → exit 1 with the 404 surfaced.

The CLI does not mask or re-code server errors; each failure reaches the user with its status and message intact.
