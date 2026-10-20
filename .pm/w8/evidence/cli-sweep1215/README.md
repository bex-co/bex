# CLI sweep 1215 — workspace membership reads (live, 2026-10-20)

Read-only sweep. No new bug filed.

- `/users/me` and `/me` → 404: identity comes from the CLI's whoami path, not a REST me-route.
- Membership answers under both spellings: `GET /owners/{id}/members` and `GET /workspaces/{id}/members` → 200, each showing the QA user as the sole `ADMIN` member (with `status: active` on one shape, `subject`/`createdAt` on the other — two projections of the same membership).
