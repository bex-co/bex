# w5 · m17 — Create wizard: all service types (static site · cron job · background worker · private service)

**Worker:** worker5 **Goal:** The w5/m15 wizard creates web services only, while backend GraphQL `createService` already accepts `type`/`schedule`/`command`/`publishPath`/`routes`/`headers` (`lego/backend/internal/apps/graphql.go`) — the dashboard is the one surface where non-web service types can't be born. Add a Render-consistent type picker + per-type fields so `/services/new` covers all five types, closing the parity ledger's Static-site UI ◐ cell. **Status:** done

## Tasks (in order)

| id   | title                                                                                                                                            | est | depends_on |
| ---- | ------------------------------------------------------------------------------------------------------------------------------------------------ | --- | ---------- |
| t001 | Capture render.com's `/new` type entries + per-type create forms live via Playwright → `docs/render-artifacts/`                                    | 25m | —          | — **DONE** |
| t002 | Type picker (Web / Private / Static Site / Background Worker / Cron Job) wired into the wizard flow, matching the t001 capture                     | 30m | t001       | — **DONE** |
| t003 | Per-type conditional fields: `publishPath` + no instance grid (static), `schedule`+`command` with cron validation (cron), no-URL note (worker/private) | 35m | t002       | — **DONE** |
| t004 | Extend the `CreateService` GraphQL document + `use-create-service` with `type/schedule/command/publishPath`; per-type redirect & deploy progress   | 25m | t003       | — **DONE** |
| t005 | Render parity — wizard vocabulary/flow vs the t001 captures + cross-surface create consistency; ledger Static-site UI ◐→✅ with evidence            | 25m | t004       | — **DONE** |
| t006 | Simplify — `/simplify` over the code this milestone changed                                                                                        | 15m | t005       | — **DONE** |
| t007 | Test coverage — per-type submit payloads, conditional-field visibility, cron/publishPath validation failure modes                                   | 30m | t005       | — **DONE** |
| t008 | Closeout — DoD met → move milestone to `done/`                                                                                                     | 10m | t007       | — **DONE** |

## Definition of done

From `/services/new` a user can create each of the five service types; a created cron job lands with its schedule and command, a created static site publishes and serves, a worker/private service shows no public URL; the wizard's type names, field vocabulary, and flow match the live render.com `/new` capture (any drift documented in the parity ledger, never silent); `docs/ADR018-render-parity.md`'s Static-site row UI cell moves ◐→✅ and the Create-service row's UI evidence is refreshed.

## Source + Goal linkage

- **Source:** `/pm-brainstorm more for w5` 2026-07-12 (Proposal 1); parity-ledger rows "Static site" (UI ◐ — "not yet offered in the create wizard") and "Create service (web / private)"; `docs/render-artifacts/new-service-wizard.md` (the w5/m15 capture this extends).
- **Goal linkage:** pillar 1 (Render dashboard parity) — the last UI-only Services-section cell not gated on a backend; every field is already accepted by `createService` (GraphQL), so this is pure dashboard work.
- **Expected outcome:** the dashboard stops being the only surface that can't create non-web service types; the Static-site ledger cell flips to ✅.
- **Why now:** the wizard is one day old (w5/m15, done 2026-07-11) — extending it now is the cheapest it will ever be; zero backend work required.
- **Render parity closing task: included** — user-facing UI change; per user instruction at materialization ("ensure they are render.com consistent", 2026-07-12), the live render.com capture (t001) is the binding comparison baseline for every implementation task, not just the closing check.
