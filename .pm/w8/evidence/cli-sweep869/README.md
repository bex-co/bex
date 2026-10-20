# CLI sweep 869 — `preDeployCommand` set/clear on an image service (live, 2026-10-20)

Disposable free image-backed web service created, patched, deleted (204, then GET 404). No new bug filed.

- `PATCH {"serviceDetails":{"preDeployCommand":"echo qa869-migrate"}}` → 200; GET reads it back inside `envSpecificDetails` (the w2/042 image-clone spelling, so `create --from` would carry the migration step).
- Clearing with an empty string → 200 and the whole `envSpecificDetails` block disappears again (absent-when-empty, so the image entrypoint default is never shadowed by an empty override).
