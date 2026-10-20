# CLI sweep 688 — env-group REST lifecycle (live, 2026-10-19)

Fixture `qa688-group` (`evg-…`) deleted (204) and verified absent (list `[]`, by-id 404).

- `POST /env-groups {name, ownerId, envVars:[{key,value}]}` → 201; the response (and by-id GET) lists env var **keys only** — values are write-only on this read shape.
- Duplicate name in the workspace → 409 `ENV_GROUP_NAME_EXISTS` with `params.name`/`workspaceId`.
- Contrast with sweep 676: this create passes the validator/handler stack cleanly (its `create-env-group` compat entry relaxes `ownerId`/`envVars` requiredness as documented in `render_openapi.go`).

No bug filed.
