# CLI sweep 1071 — env-group clone: values copy, owner resolves to default workspace (live, 2026-10-20)

Three env groups existed briefly (original, clone, duplicate-name clone); all deleted and verified absent in both workspaces. No new bug filed.

- `POST /env-groups/{id}/clone {name}` → 201, and the clone carries the var values (per-key read on the clone returns the original's value).
- The clone's `ownerId` resolves to the caller's default workspace rather than the source group's workspace (the request carries no owner field). This matches the clone owner-resolution behavior documented in earlier sweeps; it is recorded again because it is surprising, and — noted for loop hygiene — it means clone fixtures can land outside `bex-canary`. The duplicate-name clone this sweep created there was found by census and deleted (204; GET 404 in both workspaces).
- Env-group names are not unique: cloning under the original's exact name → 201 (unlike service names, sweep 988).
