# CLI sweep 958 — owners list/detail authz boundary (live, 2026-10-20)

Read-only probes. No fixture created. No new bug filed.

- `GET /owners` lists exactly the workspaces the QA user belongs to, each with a cursor; `limit=0` gets the same coded 1–100 refusal as the deploy list.
- `GET /owners/{own-id}` → 200 for both of the user's workspaces.
- `GET /owners/{unknown tea- id}` → coded 403 `FORBIDDEN`, not a bare 404 and not data: the boundary refuses without serving the resource. Recorded as correct authz discipline (a 404-vs-403 existence-oracle preference is not pinned by the Render contract, so not filed).
- `/users/{own-user-id}` → 404: no user-detail REST route is registered; identity comes from the session/`whoami` path, consistent with earlier sweeps.
