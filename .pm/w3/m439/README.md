# w3 · m439 — The CLI usage board counts customers, product areas and agent outcomes (ADR113 D7 read side + D9)

**Worker:** worker3 **Goal:** the CLI usage view and board learn audience, command family, exit codes and long waits, and ADR018 records the telemetry surface. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | Append exit_code, cli_fault, command_family, started_at and audience to cli_analytics.events | 60m | — |
| [t002](t002.md) | SQL tests for the new columns | 45m | t001 |
| [t003](t003.md) | Audience and family filters and the business panels | 75m | t001 |
| [t004](t004.md) | Operations SQL panels: delivery lag, retained window, clients by release | 30m | t003 |
| [t005](t005.md) | Docs: runbook definitions, ADR018 row, ADR088 board row, ADR113 status | 30m | t003, t004 |
| [t006](t006.md) | Apply on production and classify bex's own workspaces | 45m | t005 |
| [t007](t007.md) | Simplify | 15m | t006 |
| [t008](t008.md) | Test coverage | 30m | t007 |
| [t009](t009.md) | Closeout | 10m | t008 |

## Definition of done

- Production `cli_analytics.events` carries `exit_code`, `cli_fault`, `command_family`, `started_at` and `audience`, applied by `scripts/cli-analytics-bootstrap.sh`.
- The CLI usage board filters by audience (default `customer` + `unclassified`) and command family, and its eleven new panels answer without query errors on production.
- `python3 -m unittest scripts/test_cli_analytics.py -v` executes every committed panel query, including the new ones.
- `docs/ADR018-render-parity.md` has a CLI usage telemetry row.

## Source + Goal linkage

- **Source:** [ADR113](../../../docs/ADR113-cli-usage-telemetry.md) D7 (read side) and D9 (gaps G1, G5, G8). `/pm-brainstorm` for ADR113's implementation, 2026-11-16, item 3; filed by `/pmx adr113's implementation to w3`. Re-checked 2026-11-16 against `origin/main` `9a66e98b5`.
- **bex today:** the view exposes only a `failed` boolean, so a failed agent turn (`bex agent` exit 3) counts as a CLI failure. Hour-long `agent run --wait`, `agent wait` and `agent chat` runs land in the non-interactive duration percentiles. No panel groups commands by product area. QA and CI usage from this repository counts as adoption, although `product_analytics_audiences` already classifies workspaces for the Product adoption board. ADR018 has no telemetry row.
- **Goal linkage:** ADR008's agent-native thesis needs honest CLI adoption numbers per product area, and ADR018 must record every Render surface.
- **Expected outcome:** customer-only CLI adoption by product area, and agent-run outcomes measured as outcomes rather than CLI faults.
- **Why now:** the extended CLI (`bex agent`, automations, coding-agent launchers) keeps growing, and every week without families and audience adds unreadable history to a 90-day window.
- **Recommendation recorded (not a user decision):** bex's own QA, canary and internal workspaces are classified through the existing administrator procedure, so they leave the default customer view.
- **Render parity omitted:** an internal, ops-only board and a docs row. No REST, GraphQL, MCP or dashboard surface changes.
