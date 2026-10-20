# CLI sweep 1150 — deploy list pagination envelope (live, 2026-10-20)

One disposable free web service created and deleted (204, verified 404). No new bug filed.

- The deploys list wraps each row as `{deploy, cursor}` with the cursor equal to the deploy id; `limit=1` returns exactly one row (the newest trigger).
- An unknown cursor → 200 `[]` rather than an error — the same tolerate-unknown-cursor behavior recorded earlier for service lists; Render's own answer remains unverified against the pinned contract, so this is recorded, not filed.
