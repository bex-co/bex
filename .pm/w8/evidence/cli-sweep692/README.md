# CLI sweep 692 — `blueprints validate` via pinned CLI (live, 2026-10-19)

Local scratch manifests under `/tmp/qa501` only; nothing deployed.

- Valid minimal image service manifest → `{"valid":true, "plan":{"services":["qa692-app"],"totalActions":1}}`.
- Manifest with an unknown `bogusField` → `{"valid":false}` with a located error: `at '/services/0': additional properties 'bogusField' not allowed` at `services[0].bogusField`, line 5, column 17 (closed-world schema, ADR049).
- Missing file → client-side `file not found: …` before any request.

No bug filed.
