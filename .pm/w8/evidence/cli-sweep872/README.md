# CLI sweep 872 — service image PATCH discipline (live, 2026-10-20)

Disposable free image-backed web service created, image-patched, deleted (204, then GET 404). No new bug filed.

- `PATCH {"image":{"imagePath":"nginx:alpine","ownerId":""}}` → 200; GET reads back `nginx:alpine` (settings-level image change, distinct from the per-deploy `imageUrl` host-lock of sweep 833).
- A foreign `image.ownerId` → named 400 `image.ownerId does not match the service owner`; an empty `imagePath` → named 400 `image path is required`. Neither moved the stored image (final GET still `nginx:alpine`).
