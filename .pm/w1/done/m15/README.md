# w1 · m15 — Additional service types: background worker + cron job

**Worker:** worker1 **Goal:** Add two of Render's service `type`s bex lacks — `background_worker` (runs with no HTTP port/ingress) and `cron_job` (runs a command on a schedule, with run history) — mechanism (operator + `lego/types`) then the create-surface `type` plumbing and dashboard type-awareness, tracking Render's names. **Status:** DONE 2026-07-09 — worker + cron shippable across REST/GraphQL/MCP/UI; `create_cron_job` + cron run trigger; parity ledger updated; `make test` + dashboard tests green.

## Tasks (in order)

| id   | title                                                                                | est | depends_on        |
| ---- | ------------------------------------------------------------------------------------ | --- | ----------------- |
| t001 | `background_worker` type — no-port / no-ingress reconcile path                         | 40m | — — **DONE**      |
| t002 | `cron_job` type — `schedule` field + CronJob reconcile + run-history status            | 45m | — — **DONE**      |
| t003 | Surface plumbing — create accepts `type`; MCP `create_cron_job`; cron run trigger       | 40m | t001, t002 — **DONE** |
| t004 | Dashboard — service-type awareness (badge; worker has no URL; cron shows schedule/runs) | 40m | t003 — **DONE**   |
| t005 | Render parity — worker/cron across REST/GraphQL/MCP/UI vs render.com                    | 20m | t004 — **DONE**   |
| t006 | Simplify — `/simplify` over what this milestone changed                                 | 20m | t005 — **DONE**   |
| t007 | Test coverage — reconcile + surface + UI tests for the new types                        | 30m | t005 — **DONE**   |
| t008 | Closeout                                                                                | 10m | t007 — **DONE**   |

## Definition of done

A user can create a `background_worker` (runs without an HTTP port/ingress — no URL, no HTTP health gate) and a `cron_job` (runs on a schedule; run history visible), via the create surface across REST/GraphQL/MCP, both type-badged in the dashboard (worker shows no URL; cron shows its schedule + recent runs); MCP `create_cron_job` and the cron run trigger (Render `/cron-jobs/{id}/runs`) work; parity checked vs render.com. `make test` + dashboard tests green. **Static sites are explicitly out of scope** (a larger build→CDN effort deferred to inbox note `w1/012`).

## Source + Goal linkage

- **Source:** inbox note `w1/009` (m13 audit), the ✖ "Static site · background worker · cron job" row in `docs/ADR018-render-parity.md` (→ w1/m15). Static-site half split out to `w1/012`.
- **Goal linkage:** pillar 1 (Render parity — service-type breadth).
- **Expected outcome:** bex covers the two most common non-web Render service types; `render.yaml`/`bex.yml` with `type: worker`/`cron` deploy correctly.
- **Why now:** the audit flagged bex serves only web/private services; background_worker is a near-free win (drop the Service/Ingress) and cron unlocks scheduled workloads without the off-roadmap exec surface.
- **Render parity INCLUDED:** this milestone changes the create surface (REST/GraphQL/MCP) and the dashboard — the standing Render-parity task checks worker/cron consistency across all four vs render.com.
