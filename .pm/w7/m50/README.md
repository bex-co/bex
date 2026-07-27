# w7 · m50 — Pivot the billing sink: Metronome → Stripe Billing

**Worker:** worker7 **Goal:** re-target the m47/m48 billing pipeline from Metronome to **Stripe Billing** (native usage meters + prices + subscriptions + collection) after Stripe acquired Metronome (Dec 2025) — one vendor, native charging, the Stripe CLI as the config surface **Status:** todo

## Why

Stripe **acquired Metronome** in Dec 2025 (~$1B, completed) to own usage-based billing for the AI era. Meanwhile **Stripe Billing** now natively covers what we used Metronome for: `billing/meter` + `meter_events` (v2 stream for >1k/s), tiered/graduated pricing on the attached `Price`, and — being Stripe — **native collection** (subscription → invoice → charge + Smart Retries dunning). So bex → Metronome → Stripe collapses to **bex → Stripe**: one vendor, Phase-3 collection nearly free, and the Stripe CLI becomes the real config tool. Sources: [Stripe completes Metronome acquisition](https://stripe.com/newsroom/news/stripe-completes-metronome-acquisition), [Stripe API — Meters](https://docs.stripe.com/api/billing/meter) / [Meter Events](https://docs.stripe.com/api/billing/meter-event).

The m47/m48 architecture is **~70% reusable** — the seal-then-emit outbox (`usage_hourly.emitted_at`), epoch floor, deterministic id, TTL cache, and the `Ingester`/`BillingReader` seams all stand. Only the client's **target** changes.

## Mapping (Metronome → Stripe)

| m47/m48 (Metronome) | → Stripe Billing |
| --- | --- |
| seal-then-emit outbox, epoch floor, TTL cache | unchanged |
| deterministic `transaction_id` | meter-event `identifier` (idempotency) — unchanged logic |
| `EnsureCustomer` (ingest alias `tea-…`) | Stripe `Customer` (metadata `bex_workspace=tea-…`), cache `tea-… → cus_…` |
| `POST /v1/ingest` | `meterevent.New` (`event_name`, `payload{stripe_customer_id,value}`, `identifier`) |
| billable metric | Stripe **Meter** (`sum` over payload `value`) |
| rate card | Stripe **Price** (tiered `unit_amount_decimal`) |
| contract | Stripe **Subscription** |
| read costs/invoices (m48) | Stripe upcoming + finalized `Invoice` reads |
| Phase 3 collection (unbuilt) | **native Stripe** — subscription auto-charge + dunning |

## Tasks (in order)

| id   | title                                                                                       | est   | depends_on |
| ---- | ------------------------------------------------------------------------------------------- | ----- | ---------- |
| t001 | Stripe setup runbook: meters + tiered prices from `pricing.yaml`, webhook, restricted key    | 1h    | —          |
| t002 | Stripe Go SDK client: `EnsureCustomer` (metadata-keyed) + `IngestBatch` (meter events)       | 1h    | —          |
| t003 | Re-target the emitter/wiring to the Stripe client; `BEX_STRIPE_*` env; keep m47 outbox        | 1h    | t002       |
| t004 | Stripe read client: current-period (upcoming invoice) + finalized invoices → the m48 surface  | 45m   | t002       |
| t005 | Subscription provisioning (contract equivalent) + comp via 100%-off coupon (Mode B)           | 45m   | t002       |
| t006 | Retire the Metronome client/deps; env + CLAUDE.md + `.env.example` sync                        | 45m   | t003, t004 |
| t007 | ADR040 revision + Render-parity note; runbook cross-links                                     | 30m   | t006       |
| t008 | Simplify                                                                                     | 30m   | t007       |
| t009 | Test coverage (stub-transport unit tests; cross-surface parity holds)                         | 45m   | t008       |
| t010 | Closeout                                                                                     | 10m   | t009       |

## Definition of done

With `BEX_STRIPE_SECRET_KEY` set, bex emits each sealed `usage_hourly` row to Stripe as a meter event exactly once (deterministic `identifier`), every non-excluded tenant has a Stripe `Customer` keyed by `bex_workspace=tea-…`, the usage surface reads back real cost from Stripe's upcoming/finalized invoices, and comped tenants are handled (Mode A `billing_excluded` ⇒ no customer/subscription; Mode B ⇒ 100%-off coupon). **With `BEX_STRIPE_SECRET_KEY` unset, behavior is byte-identical to today (estimate-only).** The Metronome client + deps are removed. Collection (Phase 3) is now native Stripe — bex only reacts to `invoice.payment_failed` webhooks for the ADR040 §9 enforcement ladder (still deferred build).

## Provisioned in Stripe (test mode · acct `acct_1Ivbc5EqsEqs2tLV` "Stargately, Inc" · 2026-07-20)

The idempotent setup script `scripts/stripe-billing-setup.py` provisions **13 meters + 13 metered prices** straight from `pricing.yaml` (the single source of truth), and deactivates the earlier kind-level shadow meters. Each meter is `sum` over payload `value`, customer via `stripe_customer_id`; each price is monthly-metered, `unit_amount_decimal` in cents. Re-run any time — it skips what exists.

- **`instance_seconds` is priced per `(resource_kind, tier)`** → event names `instance_seconds.service.starter`, `…service.pro-plus`, `…postgres.basic-1gb`, `…key_value.standard`, … (10 paid tiers; `free` tiers have no meter — nothing is owed). A Stripe meter is one scalar per customer, so tiered rating needs one meter per priced dimension.
- **`egress_bytes` → `egress_gib`** and **`storage_gb_seconds` → `storage_gb_hours`** — re-based so the per-unit cent rate fits Stripe's 12-decimal `unit_amount_decimal` cap (per-byte `$1.4e-11` is 16 decimals in cents; per-GiB is `1.5¢`). The billing client (`internal/billing/stripe.go` `stripeMeterEvent`) composes the identical event names and divides the value by `2^30` / `3600` in lockstep.
- **`build_seconds`** stays per-second.

Validated live: per-tier + re-based meter events **accepted**; `instance_seconds.service.starter` links to a metered price at `0.0001864554¢/unit` (= `pricing.yaml`'s `$0.000001864554/s`).

**Still to provision (deliberate):**

- **Webhook endpoint** (`/v1/webhooks/stripe`) — its signing secret is shown once at creation, so it's created together with the receiver (t003) to capture the secret straight into the env rather than losing/exposing it.
- **Subscriptions** are per-customer (the "contract" equivalent) — provisioned by the code at first sight of a billable workspace (t005), not a global setup step.

## Notes

- **Supersedes** ADR040 Decisions §1–2 (Metronome as the sink); the metering/outbox/seal/enforcement design stands. t007 records this in the ADR.
- Stripe CLI config (meters/prices/webhook) creates real objects in a **live Stripe account** — an operator step in the t001 runbook, run with the operator's own restricted key, never committed.
- High volume: bex's ~72k events/day is well under the v1 `meterevent` limit; the v2 stream (`v2/billing/metereventstream`) is the escape hatch if ever needed.
