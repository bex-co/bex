# Render Pricing Snapshot — 2026-07-13

Captured from render.com public pricing pages and docs. Used as the baseline for bex's price sheet (docs/ADR030-pricing.md): 30% off workspace-plan fees, compute / Postgres / KeyValue / build-minute / Postgres-storage lines, 90% off bandwidth.

---

## Workspace plans

Source: render.com/pricing and render.com/docs/new-workspace-plans (April 23, 2026 lineup).

| Plan       | Render USD/month | bex USD/month (× 0.70)  |
| ---------- | ---------------- | ----------------------- |
| Hobby      | $0               | $0                      |
| Pro        | $25              | $17.50                  |
| Scale      | $499             | $349.30                 |
| Enterprise | Custom           | Custom (no catalog SKU) |

These are flat workspace subscriptions, billed in addition to compute/usage. bex keeps the same capability ladder and applies the 30% compute-family discount to the listed fees.

---

## Compute (web service instances)

Source: render.com/docs/compute-plans

| Plan      | USD/month | CPU | Memory |
| --------- | --------- | --- | ------ |
| Free      | $0        | 0.1 | 512 MB |
| Starter   | $7        | 0.5 | 512 MB |
| Standard  | $25       | 1   | 2 GB   |
| Pro       | $85       | 2   | 4 GB   |
| Pro Plus  | $175      | 4   | 8 GB   |
| Pro Max   | $225      | 4   | 16 GB  |
| Pro Ultra | $450      | 8   | 32 GB  |

Monthly pricing covers a continuously-running instance; per-hour rate = monthly / 730 h.

### Spec-named compute plans (read 2026-11-12)

Source: render.com/pricing#compute, the compute table embedded in the page as of 2026-11-12. render.com/docs/compute-plans lists the same IDs with CPU and RAM but no prices. Since 2026-10, Render names plans by size; the six legacy names above map onto `0.5c-512mb`, `1c-2g`, `2c-4g`, `4c-8g`, `4c-16g` and `8c-32g`.

| Plan ID | USD/month | CPU | Memory | bex rung | bex USD/month (× 0.70) |
| --- | --- | --- | --- | --- | --- |
| `2c-8g` | $135 | 2 | 8 GB | none yet (w6/m332) | $94.50 |
| `2c-16g` | $200 | 2 | 16 GB | none yet (w6/m332) | $140.00 |
| `4c-32g` | $350 | 4 | 32 GB | none yet (w6/blocked/114) | $245.00 |
| `8c-16g` | $300 | 8 | 16 GB | none yet (w6/m332) | $210.00 |
| `8c-64g` | $1,000 | 8 | 64 GB | none yet (w6/blocked/114) | $700.00 |
| `12c-24g` | $450 | 12 | 24 GB | none yet (w6/m332) | $315.00 |
| `12c-48g` | $800 | 12 | 48 GB | none yet (w6/blocked/114) | $560.00 |
| `12c-96g` | $1,500 | 12 | 96 GB | none yet (w6/blocked/114) | $1,050.00 |

Web services may use every ID above. For private services and background workers the two Render pages differ. render.com/docs/compute-plans allows every web ID except `free`, while the Blueprint spec's `plan` list stops at `8c-64g`. Cron jobs stop at `8c-64g` on both pages.

---

## Managed PostgreSQL

Source: render.com/docs/postgresql

| Plan        | USD/month | CPU | Memory | Storage |
| ----------- | --------- | --- | ------ | ------- |
| Free        | $0        | 0.1 | 256 MB | 1 GB    |
| Basic-256MB | $6        | 0.1 | 256 MB | 1 GB    |
| Basic-1GB   | $19       | 0.5 | 1 GB   | 5 GB    |

Free tier is ephemeral (expires after 90 days, no backups).

Re-fetched 2026-10-19 (render.com/pricing page data, w2/m171): Basic-256MB / `0.1c-256mb` is $6 and Basic-1GB / `0.5c-1g` is $19 (previously recorded here as $7 / $20). The ≥1-CPU plans bex adds, by Render's flexible id (legacy name): `1c-2g` (none) 1 CPU / 2 GB $40; `1c-4g` (`pro-4gb`) 1 CPU / 4 GB $55; `2c-4g` (`basic-4gb`) 2 CPU / 4 GB $75. On flexible plans storage is sold separately (below). [High availability](https://render.com/docs/postgresql-high-availability) needs "a compute plan with at least 1 CPU", and the standby "is billed accordingly", the same as the primary.

### Flexible-plan storage (re-verified 2026-07-14)

Source: render.com/docs/postgresql-refresh

- `$0.30` per provisioned GB per month, prorated to the second.
- Storage is selected independently from compute on paid flexible plans.
- bex's 30%-lower reference rate is `$0.21/GB-month`.

---

## Managed Key Value (Redis-compatible)

Source: render.com/docs/redis

| Plan     | USD/month | Memory |
| -------- | --------- | ------ |
| Free     | $0        | 25 MB  |
| Starter  | $10       | 256 MB |
| Standard | $30       | 1 GB   |

Render does not publish a separate Key Value storage price: persistence is a plan feature. bex's separately-metered Valkey PVC usage is therefore a deliberate extension priced at the same transparent `$0.21/GB-month` used-storage rate as Postgres.

---

## Bandwidth (egress)

Source: render.com/docs/outbound-bandwidth and render.com/docs/new-workspace-plans

$0.15 per GB outbound above the current plan's included allowance: Hobby 5 GB, Pro 25 GB, Scale 1 TB, Enterprise custom (read 2026-11-10). Without a payment method, Render spins down a workspace's services past the allowance until the next month. Re-verified 2026-07-15 after Render's April 23, 2026 workspace-plan change; legacy workspaces remain in a migration period until August 1, 2026.

---

## Build minutes

Source: render.com/docs/build-minutes

$0.005 per build-minute above the plan's included build-minute allowance.

---

## Workspace plan comparison table (read 2026-11-16, w7/m429)

Source: render.com/pricing, the workspace plan comparison table, read from the page's embedded table data on 2026-11-16: every row and every column as the page displays it ("✓" a tick, "—" a blank cell, a cell's footnote after it). The same rows are the fixture `lego/backend/internal/pricing/testdata/render-plan-table.tsv`, and `plan_table_test.go` fails while any row has no disposition in bex ([ADR030 §9](../ADR030-pricing.md#9-workspace-plans-gate-renders-plan-features-w8m137-w8m138)). One cell the page does not display: the Wildcard domains row's ticks carry the text "$0.25 per domain per month" on every plan in the data, so whether Render bills a wildcard domain outside the included count is unverified.

| Section | Row | Hobby | Pro | Scale | Enterprise |
| --- | --- | --- | --- | --- | --- |
| Render Services & Storage | Compute | per instance type (render.com/pricing#compute) |  |  |  |
| Render Services & Storage | Maximum services | 25 | Unlimited | Unlimited | Unlimited |
| Render Services & Storage | Static Sites | ✓ | ✓ | ✓ | ✓ |
| Render Services & Storage | Standard services | ✓ | ✓ | ✓ | ✓ |
| Render Services & Storage | Web services | ✓ | ✓ | ✓ | ✓ |
| Render Services & Storage | Private services | ✓ | ✓ | ✓ | ✓ |
| Render Services & Storage | Background workers | ✓ | ✓ | ✓ | ✓ |
| Render Services & Storage | Persistent disks | $0.25 per GB per month | $0.25 per GB per month | $0.25 per GB per month | $0.25 per GB per month |
| Render Services & Storage | Postgres databases | ✓ | ✓ | ✓ | ✓ |
| Render Services & Storage | Expandable storage | $0.30 per GB | $0.30 per GB | $0.30 per GB | $0.30 per GB |
| Render Services & Storage | Point-in-time recovery window | 3 days | 7 days | 7 days | 7 days |
| Render Services & Storage | Read-only replicas | ✓ | ✓ | ✓ | ✓ |
| Render Services & Storage | High availability | ✓ | ✓ | ✓ | ✓ |
| Render Services & Storage | Key-value (Redis-compatible) | ✓ | ✓ | ✓ | ✓ |
| Render Services & Storage | Cron jobs | ✓ | ✓ | ✓ | ✓ |
| Render Services & Storage | Workflows | ✓ | ✓ | ✓ | ✓ |
| Render Services & Storage | Parallel task compute | 10,000 CPU 40,000 GB RAM | 10,000 CPU 40,000 GB RAM | 10,000 CPU 40,000 GB RAM | Custom |
| Render Services & Storage | New task compute per minute | 16 CPU 64 GB RAM | 32 CPU 128 GB RAM | 32 CPU 128 GB RAM | Custom |
| Render Services & Storage | Sandboxes | ✓ | ✓ | ✓ | ✓ |
| Render Services & Storage | Default concurrent sandboxes | 100 | 1,000 Up to 5,000 on request | 1,000 Up to 10,000 on request | Custom |
| Render Services & Storage | Maximum session length | No limit | No limit | No limit | No limit |
| Render Services & Storage | Snapshot storage | $0.08 per GB per month | $0.08 per GB per month | $0.08 per GB per month | $0.08 per GB per month |
| Render Services & Storage | Service scaling | ✓ | ✓ | ✓ | ✓ |
| Render Services & Storage | Vertical scaling | ✓ | ✓ | ✓ | ✓ |
| Render Services & Storage | Horizontal autoscaling | — | ✓ | ✓ | ✓ |
| Networking | Automatic private networking | ✓ | ✓ | ✓ | ✓ |
| Networking | Bandwidth | 5 GB included per month then $0.15 per GB | 25 GB included per month then $0.15 per GB | 1 TB included per month then $0.15 per GB | Custom |
| Networking | Global regions | ✓ | ✓ | ✓ | ✓ |
| Networking | Zero-config CDN | ✓ | ✓ | ✓ | ✓ |
| Networking | Edge caching for web services | ✓ | ✓ | ✓ | ✓ |
| Networking | Custom domains | 2 included then $0.25/domain/month | 15 included then $0.25/domain/month | 25 included then $0.25/domain/month | Custom |
| Networking | Automatic TLS | ✓ | ✓ | ✓ | ✓ |
| Networking | Wildcard domains | ✓ | ✓ | ✓ | ✓ |
| Networking | WebSockets | ✓ | ✓ | ✓ | ✓ |
| Networking | Service discovery | ✓ | ✓ | ✓ | ✓ |
| Networking | Dedicated IPs | — | $100 per month for each IP set | $100 per month for each IP set | Custom |
| Networking | AWS private links | — | $30 per month Includes up to 3 links | $30 per month Includes up to 3 links | Custom |
| Networking | Private link outbound bandwidth | — | $0.03 per GB | $0.03 per GB | Custom |
| Protection | Automatic DDoS mitigation | ✓ | ✓ | ✓ | ✓ |
| Protection | Firewall | ✓ | ✓ | ✓ | ✓ |
| Protection | HTTPS only | ✓ | ✓ | ✓ | ✓ |
| Protection | Inbound IP rules | Datastores only Postgres and Key Value | Datastores only Postgres and Key Value | ✓ | ✓ |
| Protection | Network-isolated environments | — | ✓ | ✓ | ✓ |
| Build & Deploy | Automatic builds | ✓ | ✓ | ✓ | ✓ |
| Build & Deploy | Standard build pipeline | 500 mins per month then $5 per 1K mins | 1K mins per month then $5 per 1K mins | 5K mins per workspace per mo. then $5 per 1K mins | Custom |
| Build & Deploy | Performance build pipeline | — | $25 per 1000 mins | $25 per 1000 mins | Custom |
| Build & Deploy | Native runtimes | ✓ | ✓ | ✓ | ✓ |
| Build & Deploy | Docker builds | ✓ | ✓ | ✓ | ✓ |
| Build & Deploy | Environment variables & secret files | ✓ | ✓ | ✓ | ✓ |
| Build & Deploy | Monorepo support | ✓ | ✓ | ✓ | ✓ |
| Build & Deploy | Previews | ✓ | ✓ | ✓ | ✓ |
| Build & Deploy | Single-service previews | ✓ | ✓ | ✓ | ✓ |
| Build & Deploy | Full-stack preview environments | — | ✓ | ✓ | ✓ |
| Build & Deploy | Deploy orchestration | ✓ | ✓ | ✓ | ✓ |
| Build & Deploy | Auto-deploys | ✓ | ✓ | ✓ | ✓ |
| Build & Deploy | Zero-downtime deploys | ✓ | ✓ | ✓ | ✓ |
| Build & Deploy | Deploy hooks | ✓ | ✓ | ✓ | ✓ |
| Build & Deploy | Pre-deploy command | ✓ | ✓ | ✓ | ✓ |
| Build & Deploy | Instant rollbacks | 5 builds retained | 15 builds retained | 30 builds retained | 30 builds retained |
| Build & Deploy | Projects | Unlimited | Unlimited | Unlimited | Unlimited |
| Build & Deploy | Environments | 2 per project | Unlimited | Unlimited | Unlimited |
| Build & Deploy | Protected environments | ✓ | ✓ | ✓ | ✓ |
| Build & Deploy | Infrastructure as code | ✓ | ✓ | ✓ | ✓ |
| Monitoring | SSH access | ✓ | ✓ | ✓ | ✓ |
| Monitoring | Health checks | ✓ | ✓ | ✓ | ✓ |
| Monitoring | Service metrics | ✓ | ✓ | ✓ | ✓ |
| Monitoring | Notifications | ✓ | ✓ | ✓ | ✓ |
| Monitoring | Log retention | 7 days | 14 days | 30 days | 30 days |
| Monitoring | Log streams | ✓ | ✓ | ✓ | ✓ |
| Monitoring | HTTP request logs | — | ✓ | ✓ | ✓ |
| Monitoring | OpenTelemetry metrics stream | — | ✓ | ✓ | ✓ |
| Platform Access & Security | Team seats | 1 | Unlimited | Unlimited | Unlimited |
| Platform Access & Security | Workspaces | 1 | 1 | Unlimited | Unlimited |
| Platform Access & Security | 2FA & Google login | ✓ | ✓ | ✓ | ✓ |
| Platform Access & Security | Enforce 2FA & Google login | — | ✓ | ✓ | ✓ |
| Platform Access & Security | Workspace user roles | — | Admin, Developer | Admin, Developer, Contributor, Viewer, Billing | Admin, Developer, Contributor, Viewer, Billing |
| Platform Access & Security | Organization user roles | — | — | Owner, Member, Guest | Owner, Member, Guest |
| Platform Access & Security | SAML SSO | — | — | ✓ | ✓ |
| Platform Access & Security | SCIM | — | — | ✓ | ✓ |
| Platform Access & Security | OIDC authentication | — | ✓ | ✓ | ✓ |
| Platform Access & Security | Audit logs | — | Workspace | Workspace and organization | Workspace and organization |
| Compliance | GDPR DPA | ✓ | ✓ | ✓ | ✓ |
| Compliance | SOC 2 Type II | ✓ | ✓ | ✓ | ✓ |
| Compliance | ISO 27001 | ✓ | ✓ | ✓ | ✓ |
| Compliance | SOC 2 & ISO documentation | — | ✓ | ✓ | ✓ |
| Compliance | HIPAA BAA | — | — | 20% compute premium for enabled workspaces | 20% compute premium for enabled workspaces |
| Support | Email | ✓ | ✓ | ✓ | ✓ |
| Support | Chat | ✓ | ✓ | ✓ | ✓ |
| Support | Premium support | — | Add-on | Add-on | ✓ |
| Support | Private Slack channel | — | Add-on | Add-on | ✓ |
| Support | Dedicated technical account team | — | Add-on | Add-on | ✓ |
| Support | Support response SLAs | — | Add-on | Add-on | ✓ |
| Support | Migration assistance & architecture review | — | Add-on | Add-on | ✓ |

## Platform features by plan (read 2026-11-16, w7/m429)

Source: render.com/docs/platform-features-by-plan, read 2026-11-16: every feature row and every "Increased limits and retention" line, with the lowest plan that includes it. Fixture: `lego/backend/internal/pricing/testdata/render-features-by-plan.tsv`, guarded by the same test.

| Category | Feature | Lowest plan |
| --- | --- | --- |
| Service ops | Autoscaling | Pro |
| Service ops | Preview environments | Pro |
| Service ops | Performance build pipeline | Pro |
| Networking | Dedicated outbound IPs | Pro |
| Networking | Network-isolated environments | Pro |
| Networking | Private links | Pro |
| Networking | Expanded inbound IP rules | Scale |
| Observability | HTTP request logs | Pro |
| Observability | Response latency metrics | Pro |
| Observability | Metrics streaming | Pro |
| Observability | Log stream overrides | Pro |
| Observability | Webhooks | Pro |
| Compliance and governance | Compliance documentation | Pro |
| Compliance and governance | Audit logs | Pro |
| Compliance and governance | OIDC | Pro |
| Compliance and governance | SAML SSO and SCIM | Scale |
| Compliance and governance | Org-level audit logs | Scale |
| Compliance and governance | Additional member roles | Scale |
| Compliance and governance | HIPAA-enabled workspaces | Scale |
| Increased limits and retention | Unlimited workspace members | Pro |
| Increased limits and retention | Unlimited services | Pro |
| Increased limits and retention | Unlimited projects and environments | Pro |
| Increased limits and retention | Increased number of included custom domains | Pro |
| Increased limits and retention | Increased monthly included amounts of pipeline minutes and bandwidth usage | Pro |
| Increased limits and retention | Increased retention of past builds for rollbacks | Pro |
| Increased limits and retention | Increased retention of historical service metrics and logs | Pro |
| Increased limits and retention | Increased per-minute task provisioning for Render Workflows | Pro |

---

## Notes

- All prices in USD.
- Render bills on a calendar-month cycle.
- "Included allowances": bex's meters record raw quantities. Build minutes per plan (Hobby 500, Pro 1,000, Scale 5,000) are applied on top since 2026-11-10 (w6/m308, [ADR030 §13](../ADR030-pricing.md)), and so is included bandwidth per plan (Hobby 5 GB, Pro 25 GB, Scale 1 TB; w6/m319, [ADR030 §14](../ADR030-pricing.md)).
- Snapshot taken 2026-07-13 from public Render docs; prices may change.
- **The current capture is the fixtures (w4/m690, 2026-11-26).** Every price table on render.com/pricing (workspace plans, compute, Postgres, Key Value, cron jobs, Workflows, Sandboxes), its FAQ and both plan tables are captured row by row in `lego/backend/internal/pricing/testdata/`. `price_table_test.go` and `plan_table_test.go` give each row a disposition, and the weekly `scripts/render-pricing-drift.sh` fails when Render's page changes one ([ADR030 §9](../ADR030-pricing.md)). Where this snapshot and a fixture differ, the fixture is the later read.
