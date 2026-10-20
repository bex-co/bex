# CLI sweep 811 — env-group values are write-only in lists, readable per-key (live, 2026-10-20)

Two disposable env groups created, cycled, deleted (204s, then GETs 404). No new bug filed.

- Group create/GET echo env-var **keys only** and secret-file **names only** — values/contents never appear in the group object (write-only masking by design).
- `GET /env-groups/{id}/env-vars` (list) → 405: the pinned OpenAPI defines no such list operation (only per-key `GET`/`PUT`/`DELETE` under `…/env-vars/{key}`), so the refusal matches the contract. bex additionally serves a bulk `PUT …/env-vars` replace (200) as an extension.
- Per-key cycle: GET reveals the value to the owner (`{"key":"QA811_A","value":…}`), PUT creates a key (response masks the value), DELETE → 204, GET after → coded 404; secret-file per-key GET on a missing name → coded 404.
