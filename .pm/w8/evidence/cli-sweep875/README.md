# CLI sweep 875 — build-only fields refuse on an image-backed service (live, 2026-10-20)

Disposable free image-backed web services created, patched (refused), deleted (204s). No new bug filed.

- `PATCH {"rootDir":…}` → named 400 `service "…" has no repo to build (root directory only applies to build-from-git)`; `PATCH {"buildFilter":{…}}` → the same named gate for build filters. Both are repo-build concepts and refuse on a prebuilt-image service instead of being silently stored — the ADR049 fail-closed pattern working as intended.
- A string-shaped `buildFilter` → validator 400 `invalid request body at /buildFilter` before the gate.
