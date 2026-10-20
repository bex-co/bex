# CLI sweep 1195 — IP allow lists belong to datastores, not services (live, 2026-10-20)

One disposable service created and deleted (204, verified 404); baseline Postgres read-only. No new bug filed.

- `/services/{id}/ip-allow-list` → 404 for a fresh fixture and for the baseline service alike: services have no allow-list subresource.
- The datastore surface answers: baseline Postgres `GET /postgres/{id}/ip-allow-list` → 200 `{cidrs: ["0.0.0.0/0", "::/0"]}` (the default allow-all), read-only and unmutated. An unknown `dpg-` id on the same route → 404.
