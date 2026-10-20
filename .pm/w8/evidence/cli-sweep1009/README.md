# CLI sweep 1009 — blueprint validate is inline-manifest, coded errors (live, 2026-10-20)

Validation-only probes; nothing created (name list confirms). No new bug filed.

- `POST /blueprints/validate` takes `{bexYaml: "<manifest>"}` — it validates an inline manifest, not a repo fetch (a repo-shaped body gets the schema-mismatch 400).
- A valid free image-service manifest → 200 `{valid:true}` with the dry-run plan (`totalActions: 1`, a `create` action with its source path) and `$0.00` estimated pricing.
- Invalid inputs still answer 200 with `{valid:false, errors:[…]}` where each error is coded and located: `BLUEPRINT_SCHEMA_INVALID` with line/column/path, `BLUEPRINT_YAML_SYNTAX` for non-YAML, `BLUEPRINT_YAML_EMPTY` for an empty document. Validation failures are data, not HTTP errors — the correct shape for a validate endpoint.
