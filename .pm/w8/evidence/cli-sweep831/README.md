# CLI sweep 831 — blueprint validate: located errors + pricing plan (live, 2026-10-20)

Validation-only probes; nothing created. No new bug filed.

- CLI `blueprints validate` on a valid image-backed manifest → `{"valid":true,"plan":{"services":["qa831-ok"],"totalActions":1}}`.
- Bogus plan → `valid:false` with a located error (`services[0].plan`, line 5, column 11, full enum in the message); malformed YAML → `Blueprint is not valid YAML` at line 1. Errors carry path/line/column as the pinned validate schema promises.
- REST multipart `POST /blueprints/validate` (curl) → 200 with `plan.mode: "current_state"`, per-resource `actions`, and `estimatedPricing` (free manifest → `totalUsd "0.00"`, no lines — the all-free no-panel rule from w8/m18). A first Python-multipart attempt answered 401 because the harness omitted the workspace header; the curl control with the header passes, so that was harness error, not a product bug.
