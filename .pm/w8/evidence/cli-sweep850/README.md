# CLI sweep 850 — env-group contents PATCH: merge semantics + `saveMode` gate (live, 2026-10-20)

Two disposable env groups created, patched, deleted (204s; GETs 404). No new bug filed.

- `PATCH /env-groups/{id}/contents` without `saveMode` → coded 400 `ENV_GROUP_SAVE_MODE_INVALID` ("must be save_only, deploy, or rebuild"); a bogus mode gets the same coded refusal.
- With `{"saveMode":"save_only","envVars":[{one new key}]}` → 200 merge/upsert: all three keys present afterwards, `revision` bumped (`egr1_…AAE` → `…AAI`), `rolledOut:false`, no affected services. A malformed `envVars` shape → 400 naming the Go field type.
