# CLI sweep 1086 — displayName length cap (live, 2026-10-20)

One disposable free web service created and deleted (204). No new bug filed.

- A PATCH with a ~1M-character `displayName` → coded 400 `DISPLAY_NAME_TOO_LONG` (`display name must be at most 100 Unicode code points`). The cap is counted in code points, and the service answers normally afterwards.
