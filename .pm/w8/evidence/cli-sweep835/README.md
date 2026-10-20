# CLI sweep 835 — environment protection toggle (live, 2026-10-20)

Disposable project+environment created, toggled, deleted (204s; GET 404). No new bug filed.

- `PATCH /environments/{id} {"protectedStatus":"protected"}` → applied; bogus value → 400 `invalid request body at /protectedStatus` (enum named); back to `unprotected` → applied. The environment was emptied and unprotected before deletion, so no protected-delete guard was exercised against a populated environment (that guard family is ADR032's confirm-phrase surface, covered in earlier rotations).
