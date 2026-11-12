# w6 · m334 — An unmetered usage hour pages before it can no longer be metered

**Worker:** worker6 **Goal:** An hour the usage rollup has not metered pages an operator 24 h before the catch-up window closes on it, so the limit accepted in w4/496 is guarded rather than silent. **Status:** in progress — t001–t005 done (`bex_usage_oldest_unmetered_hour_timestamp_seconds` + `UsageHourNearlyUnmeterable`); t006 reads the series on production after the deploy

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Export the oldest unmetered hour's age per source](done/t001.md) — **DONE** | 1h | — |
| t002 | [Alert `UsageHourNearlyUnmeterable`, with tests, a panel and a runbook](done/t002.md) — **DONE** | 45m | t001 |
| t003 | [Record the guard in ADR023](done/t003.md) — **DONE** | 15m | t002 |
| t004 | [Simplify](done/t004.md) — **DONE** | 15m | t003 |
| t005 | [Test coverage](done/t005.md) — **DONE** | 30m | t004 |
| t006 | [Closeout](t006.md) | 10m | t005 |

## Definition of done

- A hole older than 24 h fires `UsageHourNearlyUnmeterable`, and with no hole it stays silent (promtool tests).
- `bex_usage_oldest_unmetered_hour_timestamp_seconds` is visible on production in Grafana (a start time, not the planned age gauge, so the alert ages even if publishing stops; t001).
- ADR023 records the guard.

## Source + Goal linkage

- **Source:** `/pm-brainstorm` 2026-11-11, item 3; the limit accepted in w4/496: when metering stops for more than 48 h, "Metered through" passes hours that were never metered, with no alert before it happens.
- **Goal linkage:** ADR008 Render parity first; billing at Render's prices (ADR030) needs every hour metered, and a lost hour is lost revenue and a wrong invoice.
- **Expected outcome:** a metering outage is fixed within its catch-up window instead of being skipped for good.
- **Why now:** w4/496 made settled holes pass automatically; without an alert that pass is silent.
- **Render parity task:** omitted, ops only: a metric, an alert and an ADR note, no REST/GraphQL/MCP/UI change.
