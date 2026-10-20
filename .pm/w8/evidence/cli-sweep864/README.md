# CLI sweep 864 — Key Value enum separators split REST from GraphQL → filed w8/139 (live, 2026-10-20)

Five disposable free Key Value instances created across REST/GraphQL, patched, deleted (204s; GETs 404). One bug filed: [w8/139](../../139.md).

- REST create/PATCH with hyphenated `maxmemoryPolicy: "allkeys-lru"` or `persistenceMode: "journal-snapshot"` → 400 `invalid request body at /<field>`; the underscore spellings → 201/200. The handler's shared check (`keyvalue/service.go:213,222`) normalizes both forms by design, and GraphQL `createKeyValue(maxmemoryPolicy: "allkeys-lru")` → 200 read back as `allkeys_lru` — the REST Render-schema validator is the only surface refusing the hyphen dialect ADR021 promises.
- Also verified en route: KV suspend/resume → 202s; bogus plan/policy values refuse by name; a nested `options` object at create → named unknown-field 400 (the create spelling is the flat field, reads nest it under `options`).
