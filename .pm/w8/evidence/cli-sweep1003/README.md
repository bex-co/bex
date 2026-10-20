# CLI sweep 1003 — notifyOnFail enum and placement (live, 2026-10-20)

Three disposable free web services created and deleted (204s). No new bug filed.

- `notifyOnFail` is a top-level PATCH field with Render's enum `default|notify|ignore`. A made-up value (`never`, `failures_only`) → coded 400 enumerating the vocabulary; the `serviceDetails` placement → 400 as well.
- `ignore` → 200 and reads back; resetting to `default` → 200.
