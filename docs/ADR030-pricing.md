# ADR030 — Price sheet + estimated spend

**Status:** Accepted · 2026-07-13 · revised 2026-08-19 (workspace plan fees at 30% off Render) · revised 2026-07-26 by w7/m50 **Author:** w8/m7

---

## Context

w8/m1–m6 ships a full metering pipeline: hourly Prometheus/k8s rollups of instance-seconds, egress bytes, and build seconds, with REST/GraphQL/MCP/UI surfaces that show _quantities_. Quantities alone are hard to reason about ("120 000 instance-seconds" means little without a dollar sign).

Render, bex's primary baseline, charges for these exact three meters. A 30% price advantage over Render is a pillar of bex's value proposition — but it was unquantifiable before this ADR.

The pricing/estimate trigger in `FUTURE-MAYBE.md` was fired 2026-07-13 (user request). The companion _subscription/invoices/payments_ trigger fired later: [ADR040](ADR040-billing-metronome.md), revised by w7/m50, now sends sealed usage directly to Stripe Billing. This ADR still governs the advisory estimate, not authoritative invoices.

---

## Decision

### 1. Price sheet lives in `lego/backend/internal/pricing/`

The operator already imports `lego/types/tiers/tiers.yaml` for pod resource-sizing data. That file carries a hard invariant: **no prices**. Money must not reach the operator; a mechanism layer must not depend on billing.

The price sheet therefore lives in the backend module only: `lego/backend/internal/pricing/pricing.yaml` (embedded at build time via `//go:embed`), loaded into an immutable `*Sheet` singleton at `package init`.

### 2. Discount policy

| Meter / SKU | Render rate | bex rate | Discount |
| --- | --- | --- | --- |
| Workspace plan (Hobby / Pro / Scale) | $0 / $25 / $499 per month | $0 / $17.50 / $349.30 per month | 30% off |
| Compute | per instance-month | Render × 0.70 | 30% off |
| Postgres | per instance-month | Render × 0.70 | 30% off |
| Key Value | per instance-month | Render × 0.70 | 30% off |
| Build minutes | $0.005/min | $0.0035/min | 30% off |
| Postgres storage | $0.30/GB-month | $0.21/GB-month | 30% off |
| Service disk | $0.25/GB-month | $0.175/GB-month | 30% off |
| Bandwidth | $0.15/GB | $0.015/GiB | 90% off |

Workspace plan fees are **licensed monthly SKUs**, not usage meters. They appear on the dashboard plan picker and in `pricing.yaml`; they are **not** in `BillableMeterNames` and the Stripe setup script does not provision them as metered Prices. Enterprise remains custom (no catalog rate). Resource-tier usage is billed on top of the workspace fee.

Source: `docs/render-artifacts/pricing.md` (captured 2026-07-13; bandwidth re-verified 2026-07-15 after Render's new workspace-plan rollout).

The storage estimate prices `storage_gb_seconds`, using a 730-hour pricing month. Render bills provisioned Postgres capacity; bex's collector measures actual used PVC bytes. Render does not list a separate Key Value storage charge, so applying this same used-storage rate to Valkey is a deliberate bex extension rather than a claim of exact Render shape.

**Service disk (`disk_gb_seconds`, [ADR082](ADR082-persistent-disks.md) D8/D9) is the exception, and deliberately so.** It is the one storage meter billed on **provisioned** GB rather than used bytes, because that is what the underlying Hetzner volume actually costs: a 100 GB volume is billed at 100 GB whether the tenant wrote one byte or ninety. Metering used bytes would put bex on the wrong side of the margin on every under-filled disk, and — more importantly — would make the number the tenant sees on the Disk tab ("100 GB") disagree with the number on the invoice. Two consequences follow, both intentional: a disk bills from attach to delete regardless of whether the service is running (deleting the service releases the disk; stopping it does not), and the Blueprint/pricing **estimate equals the invoice** for this SKU — the only one where it does, since every other storage line estimates provisioned floor while metering used bytes. The rate is Render's $0.25/GB-month at the standard 30% discount; bex's cost is Hetzner's €0.0440/GB-month list price for cloud volumes, so the margin absorbs the snapshot object storage and its egress.

Bandwidth is discounted 90% rather than 30% because:

1. bex is self-hosted on tenant-chosen infrastructure with much lower egress costs than Render's shared cloud.
2. Egress charging is a well-known friction point for self-hosted platforms.

### 3. `estimatedCost` field on usage responses

Every usage surface (REST, GraphQL, MCP, dashboard) is extended with a workspace-level `estimatedCost` object alongside the existing quantity meters. It contains:

- `totalUsd` — sum of all meter costs, formatted to the nearest cent.
- `meters` — per-(kind × tier × resourceKind) breakdown; entries below $0.01 are omitted.

The field is always present (never null); an empty or all-free workspace returns `totalUsd: "0.00"` and an empty meters array.

### 4. `estimatedCost` remains advisory beside real billing

`estimatedCost` is advisory even when Stripe Billing is enabled. The estimates are:

- Surfaced in the API alongside metered quantities.
- Labeled "estimate only" in the dashboard UI.
- Derived from a snapshot of Render's prices that may change.

ADR040 owns the real Stripe invoice preview/history and collection boundary. Stripe's amount is authoritative; it may differ because its billing period, discounts, credits, taxes, and rounding are not reproduced here. With Stripe disabled or degraded, this estimate remains available and the public `billing` object is absent.

### 5. Unknown tiers contribute $0

A tier ID not in `pricing.yaml` is priced at $0 (not an error). This means:

- A new compute/Postgres/KV tier that ships before it has a price entry will produce $0 estimates rather than crashing or returning a 500.
- Adding the price entry to `pricing.yaml` back-fills immediately; no migration needed.

---

## Consequences

- **Backend:** `internal/pricing` is an operator-independent package. `internal/usage.Service.monthToDateAt` calls `pricing.Default.Estimate` and attaches the result to `Summary.EstimatedCost`; `storage_gb_seconds` contributes at the embedded per-GB-second rate for Postgres and Key Value rows.
- **REST:** `GET /v1/usage` returns `estimatedCost` at the top level.
- **GraphQL:** `usage.estimatedCost { totalUsd meters { ... } }` is a new selection on `UsageSummary`.
- **MCP:** `get_usage` returns the same `estimatedCost` field; its description is updated to mention the estimate.
- **Dashboard:** an "Estimated Cost" card is added to the Usage page with a "(estimate only — not an invoice)" label.
- **Price updates:** edit `pricing.yaml`, rerun the Stripe catalog setup when billing is enabled (usage meters only), and redeploy. No schema migration is needed. Changing a workspace plan `usdPerMonth` updates the catalog/display immediately; collecting that licensed fee on a Stripe Subscription is a separate billing follow-up.
- **No Render parity conflict:** Render has no usage/billing API. The `estimatedCost` extension is bex-only; `docs/ADR018-render-parity.md` already marks the usage surface as "bex ahead of Render."

---

## Alternatives considered

**Compute the estimate in the dashboard only:** Rejected — the price sheet would then be duplicated in frontend code, untested, and unavailable to MCP agents that need to reason about cost.

**Store computed costs in Postgres:** Rejected — the price sheet changes rarely; materializing costs adds a migration burden every time it does. Compute on read is cheap.

**30% off bandwidth too:** Rejected — the 90% discount better reflects the real unit economics of self-hosted egress vs. Render's cloud markup.
