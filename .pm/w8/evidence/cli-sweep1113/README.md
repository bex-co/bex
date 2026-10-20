# CLI sweep 1113 — duplicate JSON keys: last wins (live, 2026-10-20)

One disposable free web service created and deleted (204). No new bug filed.

- A PATCH body with the same key twice (`displayName` first/second) → 200, and the second value is stored. Standard Go JSON decoding semantics (last occurrence wins); recorded, not filed.
