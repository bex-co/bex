# w6 · m28 — Render resource metadata: owner · region · dashboard URL · updated timestamp

**Worker:** worker6 **Goal:** Make Service, Postgres, and Key Value REST objects carry truthful Render-compatible `owner`, `region`, `dashboardUrl`, and `updatedAt` metadata so the unmodified official Render CLI and other Render clients show complete resource identity and navigation details. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Pin the Render metadata contract and choose authoritative region, URL, and timestamp sources | 40m | — |
| t002 | Add a reusable workspace-owner lookup and wire it into all three resource services | 45m | t001 |
| t003 | Complete Service REST metadata without conflating app URLs, dashboard routes, or timestamps | 45m | t002 |
| t004 | Add the nested owner and remaining Render metadata to managed Postgres REST objects | 45m | t002 |
| t005 | Reconcile Key Value's partial owner adapter and add its remaining truthful metadata | 40m | t002 |
| t006 | Verify all resource shapes through the official CLI and update compatibility/architecture docs | 40m | t003, t004, t005 |
| t007 | Render parity — audit REST · GraphQL · MCP · dashboard · official CLI metadata semantics | 30m | t006 |
| t008 | Simplify — run `$simplify` over metadata projection, owner lookup, and server wiring | 30m | t007 |
| t009 | Test coverage — harden shape, omission, authorization, and timestamp regression coverage | 45m | t007 |
| t010 | Closeout — verify the DoD, move tasks/milestone to `done/`, and sync the w6 roadmap | 15m | t008, t009 |

## Definition of done

`GET` and list responses for `/v1/services`, `/v1/postgres`, and `/v1/key-value` expose Render-compatible metadata backed by real state: nested `owner: {id,name,email?,type}` values resolve the resource's own workspace without a cross-tenant leak; `region` comes from an explicit platform source and is omitted when unavailable rather than invented; `dashboardUrl` uses `BEX_DASHBOARD_URL` only when a real dashboard route exists and never substitutes an app/data-plane URL; and `updatedAt` advances from an authoritative mutation/reconciliation timestamp rather than copying `createdAt`. Existing partial Service and Key Value adapters are reconciled into this contract without changing their stable ids, cursors, native `ownerId` extensions, authorization behavior, or GraphQL/MCP shapes accidentally. The unmodified pinned Render CLI prints the workspace and populated metadata for all three resource types in JSON and text modes, while unset optional configuration degrades cleanly. Backend build/tests/lint, the CLI compatibility verifier, and relevant shell checks pass; ADR006, ADR018, and the CLI checklist record evidence and any deliberate divergence.

## Source + Goal linkage

- **Source:** Promoted from `.pm/w6/done/016.md`, captured 2026-07-15 while running the real `render-oss/cli` against the dev-9 harness. The note found a systemic omission across Service, Postgres, and Key Value and recorded that parallel CLI work had since landed only partial adapters: Service has placeholder `dashboardUrl`/`updatedAt`, while Key Value has an id-as-name owner but lacks the remaining metadata.
- **Goal linkage:** Advances bex's Render-compatibility contract and AI-native “one core, compatible clients” surface (`docs/ADR006-bex-api.md`, `docs/ADR018-render-parity.md`, `docs/cli-compatibility-checklist.md`) by making Render's own official CLI a complete, trustworthy client of bex-api.
- **Expected outcome:** Resource list/get output names the real workspace and gives clients usable navigation, placement, and freshness metadata across all three resource types; text-mode CLI output no longer silently omits Workspace or prints blank Region lines when the platform has authoritative values.
- **Why now:** The CLI audit exposed the cross-cutting gap while concurrent patches began solving it independently in Service and Key Value. Consolidating the contract now prevents three incompatible owner types, fake region strings, app URLs masquerading as dashboard links, and `updatedAt == createdAt` placeholders from becoming permanent compatibility behavior. Render parity is included as t007 because the change is user-visible REST/CLI work whose underlying metadata semantics must remain coherent with GraphQL, MCP, and dashboard consumers.
