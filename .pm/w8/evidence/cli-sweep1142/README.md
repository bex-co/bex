# CLI sweep 1142 — create validation matrix; omitted ownerId resolves to default workspace (live, 2026-10-20)

No new bug filed. The one successful create was deleted and verified absent in both workspaces.

- Missing `name` → 400 `invalid request body at /name`; bogus `type` → 400 at `/type`; bogus runtime → 400 `unsupported runtime "nope"`; missing both repo and image → 400 `one of repo or image is required`. Every refusal names the failing field.
- Omitting `ownerId` → 201, but the service lands in the caller's default workspace (`tea-d982…`, "bex"), not the `X-Bex-Workspace` header's workspace — the documented owner resolution already recorded for env-group clones (sweep 1071). The fixture was deleted from the default workspace (204; GET 404 in both workspaces; name list empty).

Lesson re-applied: create probes must always send `ownerId` explicitly, or the fixture lands outside `bex-canary`.
