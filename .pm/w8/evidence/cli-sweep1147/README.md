# CLI sweep 1147 — owners list is membership-scoped; unknown owners 403 (live, 2026-10-20)

Read-only sweep. No new bug filed.

- `GET /owners` lists exactly the workspaces the QA user belongs to (`bex`, `tian-personal`, `bex-canary`) in the `{owner, cursor}` envelope.
- A well-formed but non-member owner id → 403 `FORBIDDEN`, and a malformed id → the same 403: the boundary refuses without distinguishing nonexistent from merely inaccessible, so workspace existence is not leaked.
