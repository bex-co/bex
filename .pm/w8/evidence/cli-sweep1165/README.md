# CLI sweep 1165 — single env-key delete is exact (live, 2026-10-20)

One disposable free web service created and deleted (204, verified 404). No new bug filed.

- DELETE of an existing env key → 204, and the list read-back is empty.
- DELETE or GET of the now-missing key → 404 (not a silent 204): key-level deletes are not idempotent-blurred; a second delete reports the absence.
