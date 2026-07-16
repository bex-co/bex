# Datastore pages

## Page-by-page verdicts

| Page | Live Render evidence | bex evidence | Verdict | Disposition |
| --- | --- | --- | --- | --- |
| Postgres info | Connection information, plan/status, access, users, and HA sections | Consolidated detail page with on-demand connection reveal, plan/status, access, users, HA, pooling, SQL console, and insights | Match; bex is a functional superset | Not a gap. `render-walk-postgres-info.png` / `bex-walk-postgres.png`. |
| Postgres logs | Dedicated Logs tab with database log history, time range, search, and instance filtering | Directly linkable `?tab=logs` view over the generic logs query; Last hour/6h/24h, search, observed instance, timestamped lines, and distinct empty/403/503/error states | Functional match for captured history workflow | Closed in w3/m28. bex deliberately leaves database live subscription out of this historical-view milestone instead of routing a dpg id through the App SSE path. Baseline: `render-walk-postgres-logs.png` / `bex-walk-postgres.png`; current bex: `.playwright-mcp/bex-walk-postgres-logs-current.png`; [post-walk contract and implementation evidence](../postgres-logs.md). |
| Postgres metrics | Dedicated datastore charts | Datastore memory/disk/connections/replication metrics are embedded in detail | Functional match; information architecture differs | Not a gap. `render-walk-postgres-metrics.png` / `bex-walk-postgres.png`. |
| Postgres recovery | Recovery/backups view | Backup, PITR, export, failover, and recovery controls embedded in detail | Match; bex is a functional superset | Not a gap. `render-walk-postgres-recovery.png` / `bex-walk-postgres.png`. |
| Postgres Apps | Render lists linked services | bex deliberately does not model a database-to-service link picker | Deliberate divergence | Not a gap: the board's explicit anti-goal keeps secrets/resource assignment environment-driven. |
| Key Value create | Live create form with name, plan, region, policy, and persistence controls | The shipped bex create form exposes its supported plan/version/public/policy/persistence controls | Match for supported scope | Not a gap. `render-walk-key-value-create.png`. |
| Key Value detail | No safe existing Key Value fixture in the live Render account | Seeded bex detail covers connection reveal, networking, plan, metrics, suspend/resume, and delete | Comparator unreachable | No parity judgment. The page was not skipped silently. `bex-walk-key-value.png`. |

The local managed resources remained in their provisioning/degraded state because the shared operator was unhealthy during the capture. The pages and controls were still reachable, and cluster readiness is not classified as a dashboard gap.
