# CLI sweep 561 — blueprint `maintenanceMode` dialects (validate-only)

**Verdict: no bug filed.** Same isolated QA session (`puncsky@gmail.com`), `bex-canary`, HEAD-built CLI at `77ec448ae`, production pin `d98d07f62`, 2026-10-19T15:22Z. Validate-only; nothing created or mutated (baseline 8 throughout). Contract oracle: `docs/render-artifacts/maintenance-mode.md` plus the blueprint schema in `lego/backend/internal/apps/schema/render.yaml.json`.

Thirteen manifests (image web service unless noted) through `bex blueprints validate -o json`:

- **Valid:** `enabled: true` on `starter`; `enabled: true` with no plan (bex selects `starter` at apply for a maintenance manifest, per the artifact); `enabled: false` on `free`; a custom absolute `https://example.com/m.html` URI; `maintenanceMode: {}` and `{uri: …}` without `enabled` — the blueprint schema declares no `required` keys on the object (unlike the REST two-key contract), so an explicit object canonicalizes to disabled/default-URI at apply, matching bex's "explicit object replaces both values" rule.
- **Refused, located at `services[0].maintenanceMode`:** `enabled: true` with `plan: free` → "maintenanceMode requires a paid web service plan"; worker type → "maintenanceMode is available only for web services"; static runtime → schema "additional properties 'maintenanceMode' not allowed" (static uses a different schema branch — same outcome, one layer up); extra key → additional-properties refusal.
- **URI rules, each named:** relative `/m.html` and `ftp://…` → "must be an absolute HTTP(S) URL"; embedded credentials → "must not contain credentials". These are the artifact's SSRF-hardening rules enforced at validate, before any fetch exists.

## Unrun

- Live maintenance serving (503 default/custom page, private-network reachability, downgrade-while-enabled ordering): requires an owned **paid** web-service fixture — maintenance is paid-only by contract, so no live toggle from this QA loop; the create/PATCH ordering rules rest on source tests.
- URI pointing at the maintained service's own URL (needs a real service URL to be meaningful).
