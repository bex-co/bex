# Datastore pages

## Page-by-page verdicts

| Page | Live Render evidence | bex evidence | Verdict | Disposition |
| --- | --- | --- | --- | --- |
| Postgres info | Connection information, plan/status, access, users, and HA sections | Consolidated detail page with on-demand connection reveal, plan/status, access, users, HA, pooling, SQL console, and insights | Match; bex is a functional superset | Not a gap. `render-walk-postgres-info.png` / `bex-walk-postgres.png`. |
| Postgres logs | Dedicated Logs tab with database log history | No datastore log source or Postgres Logs page | Missing cross-layer capability | Real gap, sized above an inbox note: [w3/m28](../../../.pm/w3/m28/README.md). `render-walk-postgres-logs.png` / `bex-walk-postgres.png`. |
| Postgres metrics | Dedicated datastore charts | Datastore memory/disk/connections/replication metrics are embedded in detail | Functional match; information architecture differs | Not a gap. `render-walk-postgres-metrics.png` / `bex-walk-postgres.png`. |
| Postgres recovery | Recovery/backups view | Backup, PITR, export, failover, and recovery controls embedded in detail | Match; bex is a functional superset | Not a gap. `render-walk-postgres-recovery.png` / `bex-walk-postgres.png`. |
| Postgres Apps | Render lists linked services | bex deliberately does not model a database-to-service link picker | Deliberate divergence | Not a gap: the board's explicit anti-goal keeps secrets/resource assignment environment-driven. |
| Key Value create | Live create form with name, plan, region, policy, and persistence controls | The shipped bex create form exposes its supported plan/version/public/policy/persistence controls | Match for supported scope | Not a gap. `render-walk-key-value-create.png`. |
| Key Value detail | No safe existing Key Value fixture in the live Render account | Seeded bex detail covers connection reveal, networking, plan, metrics, suspend/resume, and delete | Comparator unreachable | No parity judgment. The page was not skipped silently. `bex-walk-key-value.png`. |

The local managed resources remained in their provisioning/degraded state because the shared operator was unhealthy during the capture. The pages and controls were still reachable, and cluster readiness is not classified as a dashboard gap.
