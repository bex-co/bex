# CLI sweep 1014 — logs for a brand-new service (live, 2026-10-20)

One disposable free web service created and deleted (204). No new bug filed.

- `GET /logs?resource={new-service}` before any log lines exist → 200 with `logs: []`, `hasMore: false`, and a bounded default window (last 7 days) — an honest empty page, not an error.
- A bogus log-values label → 400.
