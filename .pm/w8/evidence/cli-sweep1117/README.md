# CLI sweep 1117 — list limit bounds are exact (live, 2026-10-20)

One disposable free web service created and deleted (204). No new bug filed.

- `limit=1` and `limit=100` → 200; `limit=101` → 400. The 1–100 window from sweep 955 holds at both edges on the service list.
