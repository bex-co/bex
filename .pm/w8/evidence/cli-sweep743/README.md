# CLI sweep 743 — env-group clone (live, 2026-10-19)

Both groups deleted (204 ×2) and list-verified gone from both workspaces (the default workspace still holds one pre-existing October group, untouched).

- `POST /env-groups/{id}/clone {}` → 400 `env group name is required`.
- `{"name":"qa743-clone"}` → 201, a distinct group carrying the source's env-var keys.
- Observation: with no `ownerId` in the clone body, the clone landed in the caller's **default** workspace while the source lives in `bex-canary` — default-workspace resolution again (sweep 703), here copying across the caller's own workspaces. Both are the same account's, so no isolation issue, but the landing workspace is worth knowing before scripting clones.

No bug filed.
