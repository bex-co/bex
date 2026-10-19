# CLI sweep 520 — blueprint datastore plan enums (validate-only)

**Verdict: no bug filed.** Companion to sweep 519 for the `databases[]` and `type: keyvalue` schema branches. Same isolated session, `bex-canary`, HEAD build `218dffcd1`, 2026-10-19T09:24Z. Validation only — the Key Value list stayed empty and no database was created.

| probe | observed result |
| --- | --- |
| Key Value entry without `ipAllowList` | `valid:false`, located schema error (`missing property 'ipAllowList'`) — Render's pinned schema requires the property on keyvalue entries; bex enforces it as-is. |
| `databases[].plan: bogus` | Schema-layer refusal naming Render's database enum (`free`, `starter`, …, `basic-256mb`, `basic-1gb`, `basic-4gb`, `pro-…`, `accelerated-…`). |
| `databases[].plan: basic-4gb` / `pro plus` | Pass Render's schema, then bex's semantic layer refuses by name: `plan "…" is not a supported Render Postgres plan (one of free\|basic-256mb\|basic-1gb)` — the compat ledger's intended catalog divergence, named with bex's real set instead of a schema echo. |
| Key Value plans (isolated manifest) | `free`, `starter`, `standard`, `256mb`, `1g` validate (spec names alias onto bex rungs, w8/011); `pro`, `pro plus`, `5g`, `50g` refuse — semantic message names bex's set (`free\|starter\|standard`) where the schema admits more than bex sells. |
| `databases: [free]` + `keyvalue: free` in this workspace | `valid:false` for a *current-state* reason: "workspace is limited to 1 Postgres database" (the baseline database holds the free slot) — validate runs the documented current-state plan, not schema alone, and says so per-entry. |

## Safety and unrun checks

- Validation only; nothing created (Key Value list re-verified empty after).
- The two-layer dialect (Render schema enum first, bex supported-set second) is consistent with sweep 519's service plans and the ledger's partially-closed divergence record; no layer contradicts the other.
- Unrun: `postgres` version fields, Key Value `maxmemoryPolicy` enum in blueprints, `databases[].diskSizeGB` bounds.
- Deploy lag means deployed revisions can precede local `main`.
