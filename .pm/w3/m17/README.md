# w3 · m17 — Wire replication-lag now that Postgres HA ships

**Worker:** worker3 **Goal:** `replication_lag` returns real data for an HA-enabled managed Postgres — the metric `w3/m10` advertised but parked behind a nil source "once HA makes it reachable", a precondition `w1/m22` has since satisfied. **Status:** todo

## Tasks (in order)

| id   | title                                                                                                  | est | depends_on |
| ---- | ------------------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Implement `ReplicationLagSource` against Prometheus `cnpg_pg_replication_lag` (the m10 source-func pattern) | 45m | —          |
| t002 | The `DatastoreMetrics` verb calls it when `Database.status.highAvailabilityEnabled` is true; omitted (not a fake zero) when HA is off | 30m | t001       |
| t003 | Dashboard `DatastoreMetricsPanel` renders the series for HA Postgres; update ADR006/ADR009's "still deferred" note | 30m | t002       |
| t004 | Render parity — three-surface consistency for `replication_lag`; refresh the ADR018/metrics rows          | 20m | t003       |
| t005 | Simplify — `/simplify` over the code this milestone changed                                              | 20m | t004       |
| t006 | Test coverage — HA returns a series, non-HA omits it, source-down maps to `ErrMetricsUnavailable`         | 30m | t004       |
| t007 | Closeout — DoD met → move milestone to `done/`                                                           | 10m | t006       |

## Definition of done

An HA-enabled managed Postgres (`status.highAvailabilityEnabled == true`) returns a real `replication_lag` `MetricSeries` across REST/GraphQL/MCP and renders in the dashboard `DatastoreMetricsPanel`; a non-HA Postgres omits the metric (unchanged, not a fabricated zero); a Prometheus outage maps to `core.ErrMetricsUnavailable`; the "today the verb never calls it / once HA makes it reachable" comment in `datastore.go` is deleted and the ADR006/ADR009 "still deferred" note removed.

## Source + Goal linkage

- **Source:** `lego/backend/internal/metrics/datastore.go` — `ReplicationLagSource` is nil and `MetricReplicationLag` documents the verb never calls it "once HA makes it reachable (w1/m22)"; `w1/m22` shipped HA and the operator now sets `Database.status.highAvailabilityEnabled` (`lego/types/v1alpha1/database_types.go:357`, controller-set when ≥2 instances ready). The gating precondition is met but the source was never wired. `/pm-brainstorm` round 11, 2026-07-15.
- **Goal linkage:** GOAL.md #2 observability; Render metrics parity — `w3/m10`'s `DatastoreMetrics` family, the one dimension left inert.
- **Expected outcome:** operators of an HA Postgres can actually see replica lag, which the MCP schema already promises.
- **Why now:** the metric is advertised in the MCP `metricTypes` schema but returns nothing; m22 removed the only blocker, so this is pure wiring of an existing, documented seam.
- **Render parity closing task: included** (t004) — REST/GraphQL/MCP + dashboard metric surface.
