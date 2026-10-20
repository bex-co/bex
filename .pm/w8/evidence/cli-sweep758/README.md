# CLI sweep 758 — `blueprints validate --workspace` authorization (live, 2026-10-19)

Same local manifest as sweep 692; nothing deployed.

- `-w tea-0000000000000000000000` (synthetic) → server 403 `FORBIDDEN` surfaced as `validation request failed with status 403: …` — workspace selection is authorization-checked server-side, not just a client label.
- `-w <canary>` explicitly → the same `{"valid":true, "plan":{…}}` as the default-workspace run.

No bug filed.
