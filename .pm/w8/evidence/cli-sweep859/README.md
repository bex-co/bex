# CLI sweep 859 — list pagination discipline (live, 2026-10-20)

Disposable free web service created and deleted (204, then GET 404); pagination probed against the workspace list. No new bug filed.

- `limit=1` + returned `cursor` → the next page holds a different service (keyset paging advances; cursors are opaque name/id keys as elsewhere).
- `limit=0` → named 400 `limit must be an integer between 1 and 100`; `limit=abc` → 400 `invalid query parameter "limit"`.
- A bogus cursor → 200 `[]` (keyset semantics: an unknown cursor sorts past the end rather than erroring) — recorded; Render's own bogus-cursor answer is unverified and an empty page is a safe reading.
