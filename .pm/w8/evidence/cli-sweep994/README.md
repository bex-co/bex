# CLI sweep 994 — rollback of an unknown deploy (live, 2026-10-20)

One disposable free web service created and deleted (204). No new bug filed.

- `POST /services/{id}/deploys/{unknown-dep}/rollback` → coded 404 `not found` (the deploy lookup fails before any rollback state machine runs).
- The fixture's own initial deploy was still in `created` state at cleanup; deleting the service removed both.
- Housekeeping note: this sweep's `git fetch` hit a transient SSH connection reset; the pin check read the cached `origin/main` (`3ede47d3c5a9`, unchanged) and was not treated as fresh evidence of upstream state.
