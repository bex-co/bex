# w9 · m389 — Log retention follows the workspace plan: 7, 14 or 30 days, as on Render

**Worker:** worker9 **Goal:** every log read — app, request, build and datastore logs on REST, GraphQL, MCP and the dashboard — reaches back exactly as far as the workspace's plan allows (Hobby 7 days, Pro 14, Scale and Enterprise 30), and an upgrade does not reveal logs older than the old plan kept. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | Measure Loki's daily volume and size 30 days; record the decision in ADR010; mark the ADR018 row ◐ until done | 45m | — |
| [t002](t002.md) | GitOps: Loki keeps 30 days; grow the volume | 60m | t001 |
| [t003](t003.md) | Per-plan read floor on every log read path | 120m | t001 |
| [t004](t004.md) | Plan changes: an upgrade does not reveal logs older than the old plan's window | 60m | t003 |
| [t005](t005.md) | Dashboard: range presets and the custom range bounded by plan | 60m | t003 |
| [t006](t006.md) | Live check on production: each plan's window holds on every surface | 45m | t002, t004, t005 |
| [t007](t007.md) | Render parity: every surface this milestone touched agrees, and matches render.com | 20m | t006 |
| [t008](t008.md) | Simplify | 15m | t007 |
| [t009](t009.md) | Test coverage | 45m | t007, t008 |
| [t010](t010.md) | Closeout | 10m | t009 |

## Definition of done

On production (QA user):

- A Hobby workspace's logs (REST `/v1/logs`, GraphQL, MCP `list_logs`, the dashboard, datastore and build logs) return nothing older than 7 days; Pro 14 days; Scale and Enterprise 30 days, with Loki configured for 30 days and sized from measured volume.
- Upgrading does not reveal lines older than the old plan's window at the upgrade time (tests).
- ADR010 and ADR018 describe the per-plan windows; ADR018 no longer calls the window 7 days.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w9` (2026-11-13), item 5, filed by `/pmx` 2026-11-13.
  - Render: Hobby 7 days, Pro 14, Scale and Enterprise 30; "Logs older than your current retention period are no longer available, even if you upgrade your plan to extend the period" ([Logging](https://render.com/docs/logging), checked 2026-11-13).
  - bex: Loki keeps 21 days for every plan (`deploy/gitops/base/loki.yaml:55-61`, 60Gi at `:78`), a window chosen for w2/m93's migration, which is done; the default lookback is 7 days (`lego/backend/internal/logs/loki.go:46`) and `lokiRange` (`:361`) applies no plan floor. ADR010:34 says 21 days; ADR018:262 says 7.
- **Goal linkage:** ADR008 Render parity first; observability (V0 roadmap item 2).
- **Expected outcome:** Scale and Enterprise workspaces keep days 22–30; Hobby and Pro windows match Render.
- **Why now:** the 21-day reason expired with w2/m93, and ADR018 misstates the window today.
- **Visible change to note:** Hobby workspaces go from 21 days to 7. That is a reduction users can see, required by the parity rule (ADR008), recorded here so the user is not surprised.
- **Render parity:** included (t007), because the change touches REST, GraphQL, MCP and the dashboard.
