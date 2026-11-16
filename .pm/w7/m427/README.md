# w7 · m427 — Service metrics reach back as far as the workspace plan keeps them: 7, 14 or 30 days, as on Render

**Worker:** worker7 **Goal:** every metrics read (services and datastores, on REST, GraphQL, MCP and the dashboard) reaches back exactly as far as the workspace's plan keeps metrics on Render: Hobby 7 days, Pro 14, Scale and Enterprise 30, with the store really holding 30 days. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Measure Prometheus's tenant series and choose how 30 days are kept; record it in ADR010; mark the ADR018 row ◐ | 60m | — |
| t002 | GitOps: metrics are kept 30 days on a volume sized from the measurement, with a fill alert | 90m | t001 |
| t003 | Per-plan read floor on every metrics read path | 90m | t001 |
| t004 | `metricsRetention` and dashboard ranges bounded by the plan | 60m | t003 |
| t005 | Live check on production, date-gated: eight days after t002 rolls out | 45m | t002, t004 |
| t006 | Render parity | 20m | t005 |
| t007 | Simplify | 15m | t006 |
| t008 | Test coverage | 45m | t006 |
| t009 | Closeout | 10m | t007, t008 |

## Definition of done

On production, as the QA user:

1. The metrics store keeps tenant-facing series for 30 days, on a volume sized from t001's measurement, and its fill alert is healthy.
2. Eight days after that rollout, for a service that has run the whole time: a Hobby workspace's metrics read with a start 14 days back (REST `/v1/metrics/cpu`, GraphQL `metrics`, MCP `get_metrics` and the dashboard) returns nothing older than 7 days, and a Scale workspace's returns all eight days. Pro's 14-day window is covered by tests.
3. The dashboard offers no preset and no custom range beyond the plan's window, and names the plan that extends it.
4. ADR010 and ADR018 state the per-plan windows.
5. `cd lego/backend && go test ./...`, `make lint`, `bash scripts/gitops-validate.sh`, and dashboard `yarn lint` + `yarn test` pass.

The full 30-day horizon can only be observed 30 days after the rollout. The closeout files a dated inbox note for that check instead of holding the milestone open.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for render parity`, 2026-11-16, item 2; filed by `/pmx them all to w7` the same day.
- **Render reference:** render.com/docs/service-metrics § Metrics retention period, read 2026-11-16: Hobby 7 days, Pro 14 days, Scale / Enterprise 30 days. render.com/pricing lists "Extended metrics retention" under Scale.
- **bex today:** `deploy/gitops/base/prometheus.yaml:289` sets `retention: 3d` on an 8Gi volume, and the production overlay patches only memory (`deploy/gitops/overlays/prod/kustomization.yaml`, w4/m144). The dashboard offers 7d, 14d and 30d presets on every plan (`dashboard/src/features/metrics/lib/range.ts`: "Ranges beyond Prometheus's retention … simply show the retained window"). `w6/done/065` measured a 720h read returning about three days and left the retention as "a capacity question … out of scope". `lego/backend/internal/core/plan_gate.go` has `LogRetention` and `LogFloor` (w9/m389) and no metrics counterpart. ADR018 § Metrics says nothing about retention.
- **Goal linkage:** ADR008 Render parity first; ADR010 (observability); ADR030 §9 (workspace plans gate Render's plan features).
- **Expected outcome:** a 7-day chart shows seven days of data, and metrics history is a real difference between plans, as on Render.
- **Why now:** every bex workspace keeps less metrics history than Render's lowest plan. Logs got the same rule two days ago in w9/m389, so the mechanism (a per-plan floor in the core verb and a bounded range picker) is fresh and can be copied.
- **Decision state:** the windows are Render's documented ones. One point is this proposal's recommendation, not a user decision: Render says an upgrade does not reveal older lines for logs, and says nothing for metrics, so this milestone applies the current plan's window with no memory of earlier plans. That is unverified against Render.
- **Sequencing:** `w7/m430` adds datastore series. Its t002 records the added series so this milestone's sizing can be re-checked.
- **Render parity:** included: metrics reads change on REST, GraphQL, MCP and the dashboard.
