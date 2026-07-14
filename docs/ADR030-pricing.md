# ADR030 — Price sheet + estimated spend

**Status:** Accepted · 2026-07-13 **Author:** w8/m7

---

## Context

w8/m1–m6 ships a full metering pipeline: hourly Prometheus/k8s rollups of instance-seconds, egress bytes, and build seconds, with REST/GraphQL/MCP/UI surfaces that show _quantities_. Quantities alone are hard to reason about ("120 000 instance-seconds" means little without a dollar sign).

Render, bex's primary baseline, charges for these exact three meters. A 30% price advantage over Render is a pillar of bex's value proposition — but it was unquantifiable before this ADR.

The pricing/estimate trigger in `FUTURE-MAYBE.md` was fired 2026-07-13 (user request). The companion _subscription/invoices/payments_ half remains deferred (trigger: hosted bex offering becomes roadmap-worthy).

---

## Decision

### 1. Price sheet lives in `lego/backend/internal/pricing/`

The operator already imports `lego/types/tiers/tiers.yaml` for pod resource-sizing data. That file carries a hard invariant: **no prices**. Money must not reach the operator; a mechanism layer must not depend on billing.

The price sheet therefore lives in the backend module only: `lego/backend/internal/pricing/pricing.yaml` (embedded at build time via `//go:embed`), loaded into an immutable `*Sheet` singleton at `package init`.

### 2. Discount policy

| Meter         | Render rate        | bex rate      | Discount |
| ------------- | ------------------ | ------------- | -------- |
| Compute       | per instance-month | Render × 0.70 | 30% off  |
| Postgres      | per instance-month | Render × 0.70 | 30% off  |
| Key Value     | per instance-month | Render × 0.70 | 30% off  |
| Build minutes | $0.005/min         | $0.0035/min   | 30% off  |
| Bandwidth     | $0.10/GB           | $0.01/GB      | 90% off  |

Source: `docs/render-artifacts/pricing.md` (captured 2026-07-13).

Bandwidth is discounted 90% rather than 30% because:

1. bex is self-hosted on tenant-chosen infrastructure with much lower egress costs than Render's shared cloud.
2. Egress charging is a well-known friction point for self-hosted platforms.

### 3. `estimatedCost` field on usage responses

Every usage surface (REST, GraphQL, MCP, dashboard) is extended with a workspace-level `estimatedCost` object alongside the existing quantity meters. It contains:

- `totalUsd` — sum of all meter costs, formatted to the nearest cent.
- `meters` — per-(kind × tier × resourceKind) breakdown; entries below $0.01 are omitted.

The field is always present (never null); an empty or all-free workspace returns `totalUsd: "0.00"` and an empty meters array.

### 4. "Estimate only" boundary — no payment collection

`estimatedCost` is advisory. bex has no billing system, no payment processor, no invoicing, and no dunning. The estimates are:

- Surfaced in the API alongside metered quantities.
- Labeled "estimate only" in the dashboard UI.
- Derived from a snapshot of Render's prices that may change.

The subscription/invoices/payments half of the `FUTURE-MAYBE.md` entry remains deferred. If/when bex adds a hosted offering, that work will use `lego/types/tiers/tiers.yaml`'s forward pointer ("prices are Metronome's") as the starting point and introduce a real billing surface — not this file.

### 5. Unknown tiers contribute $0

A tier ID not in `pricing.yaml` is priced at $0 (not an error). This means:

- A new compute/Postgres/KV tier that ships before it has a price entry will produce $0 estimates rather than crashing or returning a 500.
- Adding the price entry to `pricing.yaml` back-fills immediately; no migration needed.

---

## Consequences

- **Backend:** `internal/pricing` is a new, operator-independent package. `internal/usage.Service.monthToDateAt` calls `pricing.Default.Estimate` and attaches the result to `Summary.EstimatedCost`.
- **REST:** `GET /v1/usage` returns `estimatedCost` at the top level.
- **GraphQL:** `usage.estimatedCost { totalUsd meters { ... } }` is a new selection on `UsageSummary`.
- **MCP:** `get_usage` returns the same `estimatedCost` field; its description is updated to mention the estimate.
- **Dashboard:** an "Estimated Cost" card is added to the Usage page with a "(estimate only — not an invoice)" label.
- **Price updates:** edit `pricing.yaml` and re-deploy. No schema or migration change needed.
- **No Render parity conflict:** Render has no usage/billing API. The `estimatedCost` extension is bex-only; `docs/ADR018-render-parity.md` already marks the usage surface as "bex ahead of Render."

---

## Alternatives considered

**Compute the estimate in the dashboard only:** Rejected — the price sheet would then be duplicated in frontend code, untested, and unavailable to MCP agents that need to reason about cost.

**Store computed costs in Postgres:** Rejected — the price sheet changes rarely; materializing costs adds a migration burden every time it does. Compute on read is cheap.

**30% off bandwidth too:** Rejected — the 90% discount better reflects the real unit economics of self-hosted egress vs. Render's cloud markup.
