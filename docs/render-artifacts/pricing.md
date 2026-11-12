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

## Notes

- All prices in USD.
- Render bills on a calendar-month cycle.
- "Included allowances": bex's meters record raw quantities. Build minutes per plan (Hobby 500, Pro 1,000, Scale 5,000) are applied on top since 2026-11-10 (w6/m308, [ADR030 §13](../ADR030-pricing.md)), and so is included bandwidth per plan (Hobby 5 GB, Pro 25 GB, Scale 1 TB; w6/m319, [ADR030 §14](../ADR030-pricing.md)).
- Snapshot taken 2026-07-13 from public Render docs; prices may change.
