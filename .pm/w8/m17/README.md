# w8 · m17 — Disk-autoscaling hardening: loud sample-failure signal + single-sourced 16 TB cap

**Worker:** worker8 **Goal:** the disk-autoscaling loop fails loudly when it can't sample disk usage — instead of silently no-oping while a tenant database fills to 100% — and the 16 TB cap stops living as four independent literals. **Status:** todo

## Tasks (in order)

| id   | title                                                                                     | est | depends_on |
| ---- | ----------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Design the persistent sample-failure signal: Event vs status condition, N-failure debounce | 30m | —          |
| t002 | Implement + test the sample-failure signal                                                 | 45m | t001       |
| t003 | Single-source the 16 TB cap across operator, dashboard, and MCP description                | 30m | —          |
| t004 | Simplify — `/simplify` over the milestone's diff                                           | 20m | t002, t003 |
| t005 | Test coverage — meaningful tests for the shipped behavior                                  | 30m | t002, t003 |
| t006 | Closeout — verify DoD, sync status, move to `done/`                                        | 15m | t005       |

## Definition of done

With `diskAutoscalingEnabled: true` and the disk-usage reader failing N consecutive reconciles (Prometheus unreachable or PVC series absent), the Database emits a durable Warning signal observable via `kubectl describe database` — distinguishable from benign first-reconcile "no PVC yet" noise — and a test proves it. A guard test fails the build if the operator cap constant, the dashboard display constant, and the MCP tool-description "16 TB" drift from one another.

## Source + Goal linkage

- **Source:** `/pm-brainstorm` round 16, 2026-07-15 — consistency-mine over the day-old w8/m14 diff (commit `4cb48836`): `lego/operator/internal/controller/database_autoscale.go:176-179` swallows sample-unavailable at Info level and no-ops (while genuine persist failures do surface via `DiskAutoscalingFailed`); the 16 TB cap literal is duplicated across `database_autoscale.go:38`, `dashboard/src/features/databases/components/database-disk-autoscaling-control.tsx:5`, `lego/backend/internal/postgres/mcp.go:186`, and `docs/render-artifacts/postgres-disk-autoscaling.md`.
- **Goal linkage:** managed-Postgres reliability (`docs/ADR009-postgresql-management.md`) — polish on the surface w8/m14 shipped; a feature whose purpose is preventing disk-full outages must not have a silent failure mode.
- **Expected outcome:** an operator sees a Warning on the Database before a tenant hits disk-full during a Prometheus outage; the four cap sites cannot drift silently.
- **Why now:** day-old code — the proven mine-while-fresh well; the failure mode is invisible until the first real Prometheus outage, which is exactly when it's too late.
- **Render parity omitted:** no wire-shape change — the signal is operator-internal and kubectl-only, mirroring `status.diskResizeHistory`'s documented operator-status-trail choice (Render publishes no notification contract for this). If t003 instead opts to expose the cap via the API, file a parity follow-up note then.
