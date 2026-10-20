# CLI sweep 1093 — env-group link accepts a suspended service (live, 2026-10-20)

One disposable free web service (suspended during the probe) and one env group created; both deleted (204s). No new bug filed.

- Linking an env group to a suspended service → 200 (membership is configuration, and configuration stays open while suspended — sweep 1032's pattern). Unlink → 204.
