# CLI sweep 519 — blueprint plan-name matrix (validate-only)

**Verdict: no bug filed.** Plan names were probed through read-only `blueprints validate` (never apply), so paid/spec names could be tested with zero purchase risk. Same isolated session, `bex-canary`, HEAD build `218dffcd1`, 2026-10-19T09:23Z. Base manifest: `examples/hello-go/render.yaml` with only `plan:` varied.

| plan value | result |
| --- | --- |
| `free`, `starter`, `standard`, `pro`, `pro plus` | `valid:true`, exit `0`. |
| `0.5c-512mb`, `2c-4g` | `valid:true` — real rungs of Render's spec-based ladder. |
| `1c-1g`, `128c-1024g`, `bogusplan` | `valid:false`, exit `1`, located at `/services/0/plan` with the full enum: `free, starter, standard, pro, pro plus, pro max, pro ultra, 0.5c-512mb, 1c-2g, 2c-4g, 2c-8g, 2c-16g, 4c-8g, 4c-16g, 4c-32g, 8c-16g, 8c-32g, 8c-64g, 12c-24g, 12c-48g, 12c-96g` — the pinned Render schema's ladder (`lego/backend/internal/apps/schema/render.yaml.json`); `1c-1g`/`128c-1024g` are not Render rungs. |
| `pro_plus` | `valid:false` at the schema layer — blueprints spell multi-word plans with spaces (REST accepts the underscore form, sweep 518). That asymmetry is Render's own pinned schema, faithfully enforced, not a bex invention. |

## Safety and unrun checks

- Validation only; nothing created. Apply-time mapping of spec names onto bex's tier catalog (w8/011 aliases for overlapping rungs; larger Render rungs an intended divergence per the compat ledger) was deliberately **not** probed — an apply with a paid-equivalent plan could create a billable service, which hunt rules forbid.
- Unrun: `databases[].plan` and Key Value plan enums in blueprints (separate schema branches), plan names on `cron`/`worker` blueprint entries.
- Deploy lag means deployed revisions can precede local `main`.
