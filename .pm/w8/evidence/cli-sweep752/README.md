# CLI sweep 752 — workflow-service creation surface (live, 2026-10-19)

Nothing created (`workflows list` stayed `null`).

- Missing-flag guards are sequential and named: no flags → `--name is required`; name only → `--repo is required`.
- A fully specified create (public repo, go runtime, build/run commands) → server 405 `method not allowed`: workflow-service creation is not open on this production surface yet, and it refuses as a clean 405 rather than half-creating.
- `workflows tasks list <synthetic>` → 404.

No bug filed.
