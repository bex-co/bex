# Runbook — Metronome billing setup (ADR040 Phase 0)

**Owner:** billing workstream **Source:** [ADR040 §5, §8 Phase 0](../ADR040-billing-metronome.md) **Status:** live

This runbook stands up the Metronome-side configuration the bex emitter targets: the **billable metrics**, **products**, and a **rate card** that turn bex's exported usage events into rated invoice line items. bex sends events (`internal/billing`); Metronome aggregates and rates them. **No bex code changes here** — this is dashboard/API configuration plus the out-of-band API token.

Do it once per Metronome org (production + any sandbox). Re-run it against a fresh org to reproduce the setup exactly.

---

## 0. Prerequisites

- A Metronome org (or sandbox). Sign in at <https://app.metronome.com>.
- The bex price sheet: [`lego/backend/internal/pricing/pricing.yaml`](../../lego/backend/internal/pricing/pricing.yaml) — the single source of the rates configured below (30 % off Render on compute/Postgres/Key Value/build/storage, 90 % off bandwidth; see [ADR030](../ADR030-pricing.md)).
- `curl` + `jq` if you prefer the API to the dashboard.

## 1. API token → `BEX_METRONOME_TOKEN` (out-of-band secret)

1. Metronome **Settings → API Tokens → Create**. Scope it to ingest + customer + read (contracts/usage). Copy the bearer token.
2. Hand it to ops as the out-of-band secret `BEX_METRONOME_TOKEN` ([ADR019](../ADR019-infra-credentials.md) custody pattern). **Never commit it**; it is not in `.env.example` (a value-less mirror only).
3. Setting it (with `BEX_CP_DB_URI` present) turns on the emitter. Leave it unset everywhere until the metrics/rate card below exist, or the first sealed rows will ingest against an org that cannot rate them.

## 2. Billable metrics (one per meter kind)

bex exports one `event_type` per meter kind. The numeric quantity travels as the string property `value`; the dimensions travel as `tier`, `resource_kind`, `service_id` (ADR040 §5). Create **four** billable metrics, each an aggregation `SUM` over `properties.value`, filtered by `event_type`, and grouped by `tier` and `resource_kind`:

| Metric name | `event_type` filter | Aggregate | Group by |
| --- | --- | --- | --- |
| bex Instance Seconds | `instance_seconds` | `sum(properties.value)` | `tier`, `resource_kind` |
| bex Egress Bytes | `egress_bytes` | `sum(properties.value)` | `resource_kind` |
| bex Build Seconds | `build_seconds` | `sum(properties.value)` | — |
| bex Storage GB-seconds | `storage_gb_seconds` | `sum(properties.value)` | `resource_kind` |

API shape (`POST /v1/billable-metrics/create`), instance-seconds example:

```jsonc
{
  "name": "bex Instance Seconds",
  "aggregation_type": "sum",
  "aggregation_key": "value",
  "event_type_filter": { "in_values": ["instance_seconds"] },
  "group_keys": [["tier"], ["resource_kind"]],
}
```

Record each returned `id` — the reconciliation harness (below) and the rate card reference them.

## 3. Products + rate card (mirror `pricing.yaml`)

Create one **product** per meter family and one **rate card** binding each product to its per-unit rate. The rates are copied verbatim from `pricing.yaml`; the mapping is:

| `pricing.yaml` key | Rate (USD) | Metronome rate on… |
| --- | --- | --- |
| `compute[].usdPerSecond` | per tier (see file) | Instance Seconds, dimension `resource_kind=service`, per `tier` |
| `postgres[].usdPerSecond` | per tier | Instance Seconds, `resource_kind=postgres`, per `tier` |
| `keyvalue[].usdPerSecond` | per tier | Instance Seconds, `resource_kind=key_value`, per `tier` |
| `bandwidth.usdPerByte` | `0.000000000013969839` | Egress Bytes (flat) |
| `build.usdPerSecond` | `0.000058333333` | Build Seconds (flat) |
| `storage.usdPerGBSecond` | `0.000000079908676` | Storage GB-seconds (flat) |

So every `usdPerSecond` / `usdPerByte` / `usdPerGBSecond` in the sheet becomes one rate on the matching metric. For instance-seconds, the rate is keyed by the `(resource_kind, tier)` group so `free` tiers rate to `$0`.

Steps (dashboard or API):

1. `POST /v1/contract-pricing/products/create` — one product per metric family (Compute, Bandwidth, Build, Storage), each `type: "usage"` referencing its billable-metric id.
2. `.../rate-cards/create` — create the rate card (e.g. "bex Standard").
3. `.../rate-cards/addRates` — add one rate per row above. For instance-seconds, add a rate per `(resource_kind, tier)` pricing-group value; use the exact `usdPerSecond` from `pricing.yaml`.

> **Re-deriving rates from `pricing.yaml`.** The file is the source of truth for numbers. Its header documents the derivation (`usdPerSecond = render_monthly × discount ÷ 2,628,000 s/mo`). When `pricing.yaml` changes, update the matching rate-card rates here — the two are kept in lockstep by convention, not code (ADR040 §1 keeps money out of the mechanism layer). `pricing_test.go` guards the sheet; this runbook guards the mirror.

## 4. Verify the mapping (reconciliation)

Once the emitter has exported at least one sealed period, run the reconciliation harness to prove Metronome's computed billable-metric totals equal the `usage_hourly` quantities bex shipped:

```bash
BEX_CP_DB_URI=… BEX_METRONOME_TOKEN=… \
  scripts/metronome-reconcile.sh --start 2026-07-01T00:00:00Z --end 2026-07-02T00:00:00Z \
  --metrics metrics.json     # {"instance_seconds":"<billable_metric_id>", …}
```

It prints a per-meter table (`usage_hourly` vs Metronome) and a `PASS`/`FAIL`. A correctly-mapped export passes within rounding; a wrong metric id or filter fails loudly. See the harness header for details.

## 5. Enable

With metrics + rate card in place, follow ADR040 §7 "Enable the feature": set `BEX_METRONOME_TOKEN` (+ an explicit `BEX_METRONOME_EPOCH` = billing-start instant) and deploy. The emitter backfills sealed rows from the epoch forward. Per-workspace **charging** is a Metronome contract (m48), never a bex switch; Phase 1 (this milestone) only shadow-exports and reconciles.
