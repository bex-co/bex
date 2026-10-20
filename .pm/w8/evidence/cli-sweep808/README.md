# CLI sweep 808 — workspace members/owner read surface (live, 2026-10-20)

Read-only probes; nothing created or mutated (the second workspace was only read). No new bug filed.

- `GET /owners/{tea-canary}/members` → 200 with the sole member (`role: ADMIN`, `status: active`, `mfaEnabled: false`).
- `GET /owners/{synthetic}` and `GET /owners/{synthetic}/members` → coded 403 `FORBIDDEN` (no existence leak split).
- `GET /owners/{own other workspace}` → 200 with its identity — membership scoping reads across the account's own workspaces as documented.
