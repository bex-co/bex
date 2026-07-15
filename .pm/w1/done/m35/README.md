# w1 · m35 — Blueprint field completeness: envVarGroups · fromGroup · sync:false · fromService

**Worker:** worker1 **Goal:** The `bex.yml` fields currently rejected with a named error — `envVarGroups`/`fromGroup`, `sync: false`, `fromService.envVarKey`, plus `generateValue` acceptance — work, so real-world `render.yaml` files deploy unmodified. **Status:** done (2026-07-14)

## Tasks (in order)

| id   | title                                                        | est | depends_on                   |
| ---- | ------------------------------------------------------------ | --- | ---------------------------- |
| t001 | `envVarGroups:` blocks create/update env groups at apply     | 45m | — — **DONE**                 |
| t002 | `fromGroup` links a declared group to the service            | 30m | t001 — **DONE**              |
| t003 | `sync: false` semantics on blueprint sync                    | 40m | — — **DONE**                 |
| t004 | `fromService.envVarKey` cross-service references             | 45m | — — **DONE**                 |
| t005 | Accept `generateValue` in bex.yml (rides w8/m10's core verb) | 30m | t001 — **DONE**              |
| t006 | `validate_bex_yml` accepts all; ADR006 field ledger update   | 30m | t001, t002, t003, t004, t005 — **DONE** |
| t007 | Render parity                                                | 30m | t006 — **DONE**              |
| t008 | Simplify                                                     | 30m | t007 — **DONE**              |
| t009 | Test coverage                                                | 45m | t007 — **DONE**              |
| t010 | Closeout                                                     | 15m | t009 — **DONE**              |

## Definition of done

A `render.yaml`-shaped `bex.yml` using `envVarGroups` + `fromGroup`, `sync: false`, `fromService.envVarKey`, and `generateValue` validates (`validate_bex_yml` — no named-error rejection) and deploys end-to-end; a later `sync` honors `sync: false` (the var's live value is not overwritten). The ADR006 §bex.yml field ledger marks all five supported.

## Source + Goal linkage

- **Source:** `/pm-brainstorm more milestones for each worker to work on until feature parity` 2026-07-14 (item 4); `docs/ADR018-render-parity.md` §Blueprint row ("`envVarGroups`/`fromGroup`/`generateValue`/`sync:false`/`fromService.envVarKey` rejected with a named error").
- **Goal linkage:** vision pillar 4 (deploy-from-chat) — an agent handed a real Render blueprint should not need to edit it; every ingredient shipped (env groups w1/m16, stacks w1/m24, verbs w2/m15). Blueprint spec work routes to w1 per DO_NOT_DO's `fromDatabase` precedent.
- **Expected outcome:** the named-error rejection list is empty (or documented as deliberate per-field, e.g. if Render semantics turn out unimplementable); real blueprints apply unmodified.
- **Why now:** last blueprint gap after w1/m24 + w2/m15; w8/m10 (its t005 prerequisite) is being materialized alongside. Render parity task included — surface behavior change on `deploy`/validate/sync across REST/GraphQL/MCP.
