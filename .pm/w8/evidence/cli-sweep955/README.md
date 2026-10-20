# CLI sweep 955 — deploy list pagination and cursor edges (live, 2026-10-20)

Read-only on the baseline `hello-go` service. No new bug filed.

- The list lives at `/services/{id}/deploys` (Render's shape); a top-level `/deploys` 404 is the correct absence, not a gap.
- `limit` outside 1–100 (`0`, `-1`, `9999`) → coded 400 `limit must be an integer between 1 and 100`, identical wording on every violation.
- Cursor pagination: page 1 (`limit=2`) returns a cursor equal to the last deploy id; page 2 returns the next two deploys with zero overlap.
- A bogus cursor returns `200 []` rather than an error. Render's pinned answer for an unknown cursor is unverified, so this is recorded, not filed.
- Unknown service id → 404.
