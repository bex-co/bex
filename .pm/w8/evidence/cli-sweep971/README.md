# CLI sweep 971 — disks fail-closed on free services (live, 2026-10-20)

Two disposable free web services created and deleted (204s). No new bug filed.

- Disks are a top-level resource (`POST /disks`), not nested under the service — the nested path correctly 404s.
- Creating a 1GB disk for a free service → coded 400 `disks require a paid instance type`: the paid gate refuses on the free side, creates nothing (the follow-up list for the service is empty), and no paid transition was attempted.
