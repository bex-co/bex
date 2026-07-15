# w2 · m41 — Blueprint Key Value resources + keyvalue `fromService` refs

**Worker:** worker2 **Goal:** a `bex.yml`/render.yaml Blueprint with `keyValue` entries validates, syncs, and provisions live Valkey stores, and env vars can reference a key-value connection via `fromService` — closing the Blueprint family's last resource-type hole. **Status:** todo

## Tasks (in order)

| id   | title                                                              | est | depends_on |
| ---- | ------------------------------------------------------------------ | --- | ---------- |
| t001 | Pin Render's Blueprint `keyValue` schema → render-artifacts        | 30m | —          |
| t002 | Shared parser: `keyValue` entries                                  | 45m | t001       |
| t003 | Core wiring: create/update via keyvalue service; validate/list/sync | 45m | t002       |
| t004 | `fromService` → keyvalue connection env refs                       | 30m | t003       |
| t005 | Checklist line 84 + ADR018 Blueprint row update                    | 15m | t004       |
| t006 | Render parity                                                      | 30m | t005       |
| t007 | Simplify                                                           | 30m | t006       |
| t008 | Test coverage                                                      | 45m | t006       |
| t009 | Closeout                                                           | 15m | t008       |

## Definition of done

A Blueprint containing a `keyValue` entry passes `validate_bex_yml`, and applying it provisions a live Valkey store; an env var `fromService` referencing that store's connection resolves at deploy; `docs/cli-compatibility-checklist.md:84`'s "keyValue is omitted" divergence and `lego/backend/internal/apps/blueprint.go:73`'s wire-compat placeholder are gone, recorded with evidence.

## Source + Goal linkage

- **Source:** `/pm-brainstorm` round 12, 2026-07-15 — code miner (`blueprint.go:73`, `deploy.go:1200`) + CLI checklist line 84.
- **Goal linkage:** pillar 4 (deploy-from-chat / agents deploy whole stacks declaratively); Blueprint parity (extends w1/m24 + w2/m15/m37).
- **Expected outcome:** all three managed resource types (services, Postgres, Key Value) are Blueprint-first-class; the checklist divergence closes.
- **Why now:** w2/m40 is about to touch the same Blueprint grouping code — doing Key Value resources adjacent avoids two passes over the parser. Render parity closing task included — REST/GraphQL/MCP Blueprint verbs change.
