# CLI sweep 830 — env-var single-key operations (live, 2026-10-20)

Disposable free web service created, exercised, deleted (204, then GET 404). No new bug filed.

- `PUT …/env-vars/{key}` with both `value` and `generateValue:true` → 400 `request body does not match the Render API schema` (the oneOf in the pinned schema refuses the combination before any write).
- Single PUT plain → 200 echo; DELETE → 204; GET after → coded 404 `env var not found`; DELETE of a missing key → the same named 404 (not a bare or 500 answer).
