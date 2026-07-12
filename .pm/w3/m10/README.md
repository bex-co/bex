# w3 · m10 — Extended resource metrics: autoscale target, disk, DB connections

**Worker:** worker3 **Goal:** the metrics page/API shows autoscale-target, disk usage, and DB active-connections series alongside the existing CPU/mem-as-%-of-limit charts, closing the last open row of the metrics-parity work. **Status:** todo

## Tasks (in order)

| id   | title                                                                                                        | est | depends_on              |
| ---- | -------------------------------------------------------------------------------------------------------------- | --- | ------------------------ |
| t001 | Autoscale-target line: surface the HPA/autoscaling target utilization alongside current CPU/mem usage (`w1/m20`'s config) | 45m | —                         |
| t002 | Disk usage series: PVC usage for managed Postgres/Key Value volumes via cAdvisor/CSI metrics                    | 45m | —                         |
| t003 | DB active-connections series: CNPG/Postgres exporter metric on the metrics API                                  | 45m | —                         |
| t004 | DB replication-lag series — gated on `w1/m22` (Postgres HA); implement the query path now, surface `N/A`/hidden until HA ships | 30m | —                         |
| t005 | Dashboard: add the four series to the metrics page charts (Render-consistent)                                   | 1h  | t001,t002,t003,t004       |
| t006 | Render parity — verify the new metrics fields/semantics are consistent across REST/GraphQL/MCP/UI vs render.com | 30m | t005                      |
| t007 | Simplify — `/simplify` over the code this milestone changed                                                      | 15m | t006                      |
| t008 | Test coverage — meaningful tests for each new series' real behavior (real data, missing-HA fallback for t004)    | 30m | t006                      |
| t009 | Closeout — verify DoD met, then move the milestone to `done/`                                                    | 10m | t007,t008                 |

## Definition of done

The metrics page/API shows autoscale-target, disk usage, and DB active-connections series for real services/databases; replication-lag is wired but inert until `w1/m22` ships (documented, not silently missing).

## Source + Goal linkage

- **Source:** `/pm-brainstorm for more` 2026-07-12, `docs/ADR018-render-parity.md` "Extended metrics" row — explicitly deferred pending `w1/008` (autoscaling) and `w1/m17` (Postgres), both now shipped (as `w1/m20` and `w1/m17` respectively).
- **Goal linkage:** pillar 1 (Render-compatible REST/GraphQL/MCP), closing the last open row of `w3/m4.5`'s metrics-parity work.
- **Expected outcome:** the metrics page matches Render's autoscale-target, disk, and DB-connection series; replication-lag is scaffolded so it activates automatically once `w1/m22` ships, rather than needing a second milestone.
- **Why now:** both blockers this row was explicitly waiting on are done; this was the last open piece of extended-metrics parity.
- **Render parity closing task: included** — new metrics-API fields + dashboard chart surface.
