# CLI sweep 1004 — maxShutdownDelaySeconds bounds (live, 2026-10-20)

Two disposable free web services created and deleted (204s). No new bug filed.

- The accepted range is exactly 1–300 seconds: `-1`, `0`, `301`, `3600`, and `999999` → coded 400 `maxShutdownDelaySeconds must be 1-300`; boundary values `1` and `300` (and the default `30`) → 200 and read back.
