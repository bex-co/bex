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

---

## Managed PostgreSQL

Source: render.com/docs/postgresql

| Plan        | USD/month | CPU | Memory | Storage |
| ----------- | --------- | --- | ------ | ------- |
| Free        | $0        | 0.1 | 256 MB | 1 GB    |
| Basic-256MB | $7        | 0.1 | 256 MB | 1 GB    |
| Basic-1GB   | $20       | 0.5 | 1 GB   | 5 GB    |

Free tier is ephemeral (expires after 90 days, no backups).

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

$0.15 per GB outbound above the current plan's included allowance. Re-verified 2026-07-15 after Render's April 23, 2026 workspace-plan change; legacy workspaces remain in a migration period until August 1, 2026.

---

## Build minutes

Source: render.com/docs/build-minutes

$0.005 per build-minute above the plan's included build-minute allowance.

---

## Notes

- All prices in USD.
- Render bills on a calendar-month cycle.
- "Included allowances" (free bandwidth, free build minutes per plan tier) are not tracked in bex's meter — bex meters raw quantities only.
- Snapshot taken 2026-07-13 from public Render docs; prices may change.
