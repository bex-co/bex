# CLI sweep 883 — env-var key validation (live, 2026-10-20)

Disposable free web service created, exercised, deleted (204, then GET 404). No new bug filed.

- Traversal/slash keys (`../QA883`, `QA/883`, URL-encoded forms decoded first) → coded 400 `ENVIRONMENT_VARIABLE_INVALID` with the key rule spelled out (letters/digits/underscores, no leading digit, ≤253) and `params.field`/`maxLength`.
- An empty-string value is a legitimate set (`{"key":"QA883_E","value":""}` round-trips with its revision) — empty is a value, not a delete.
