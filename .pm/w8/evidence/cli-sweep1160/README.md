# CLI sweep 1160 — notifications inbox is empty and id-gated (live, 2026-10-20)

Read-only plus refused writes. No new bug filed.

- `GET /notifications?limit=3` → 200 `[]`: the QA workspace has no inbox notifications.
- Mark-read against a malformed notification id → 400 (path-parameter form gate) with both an empty-object and an absent body: the id is rejected before body handling.
