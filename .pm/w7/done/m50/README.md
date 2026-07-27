# w7 · m50 — Pivot the billing sink: Metronome → Stripe Billing

**Worker:** worker7 **Goal:** re-target the m47/m48 billing pipeline from Metronome to **Stripe Billing** (native usage meters + prices + subscriptions + collection) after Stripe acquired Metronome (Dec 2025) — one vendor, native charging, the Stripe CLI as the config surface **Status:** done

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

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Stripe setup runbook: meters + tiered prices from `pricing.yaml`, webhook, restricted key — **DONE** | 1h | — |
| t002 | Stripe Go SDK client: `EnsureCustomer` (metadata-keyed) + `IngestBatch` (meter events) — **DONE** | 1h | — |
| t003 | Re-target the emitter/wiring to the Stripe client; `BEX_STRIPE_*` env; keep m47 outbox — **DONE** | 1h | t002 |
| t004 | Stripe read client: current-period (upcoming invoice) + finalized invoices → the m48 surface — **DONE** | 45m | t002 |
| t005 | Subscription provisioning (contract equivalent) + comp via 100%-off coupon (Mode B) — **DONE** | 45m | t002 |
| t006 | Retire the Metronome client/deps; env + CLAUDE.md + `.env.example` sync — **DONE** | 45m | t003, t004, t005 |
| t007 | ADR040 revision + runbook cross-links — **DONE** | 30m | t001, t006 |
| t008 | Render parity — **DONE** | 30m | t007 |
| t009 | Simplify — **DONE** | 30m | t008 |
| t010 | Test coverage (stub-transport unit tests; cross-surface parity holds) — **DONE** | 45m | t009 |
| t011 | Closeout — **DONE** | 10m | t010 |

## Definition of done

With `BEX_STRIPE_SECRET_KEY` set, bex emits every paid sealed `usage_hourly` row to Stripe with a deterministic meter-event `identifier`; ordinary retry ambiguity is deduplicated inside Stripe's documented rolling uniqueness window, and older ambiguity is explicitly monitored/reconciled rather than mislabeled strict exactly-once. Every non-excluded workspace entering billing has a Stripe `Customer` keyed by `bex_workspace=tea-…`, the usage surface reads back real cost from Stripe's invoice preview/finalized invoices, and comped tenants are handled (Mode A `billing_excluded` ⇒ no customer/subscription; Mode B ⇒ 100%-off coupon). **With `BEX_STRIPE_SECRET_KEY` unset, behavior is byte-identical to today (estimate-only).** The Metronome client + deps are removed. Collection is native Stripe; bex only verifies and records `invoice.payment_failed` webhooks for the ADR040 enforcement ladder (still deferred build).

## Provisioned in Stripe (test mode · acct `acct_1Ivbc5EqsEqs2tLV` "Stargately, Inc" · 2026-07-20)

The idempotent setup script `scripts/stripe-billing-setup.py` provisions or validates **13 meters + 13 metered prices** straight from `pricing.yaml` (the single source of truth), creates/validates the stable `bex-comp-100` coupon, and deactivates the earlier kind-level shadow meters. Each meter is `sum` over payload `value`, customer via `stripe_customer_id`; each price is monthly-metered, `unit_amount_decimal` in cents. Re-runs validate what exists and fail closed on drift.

- **`instance_seconds` is priced per `(resource_kind, tier)`** → event names `instance_seconds.service.starter`, `…service.pro-plus`, `…postgres.basic-1gb`, `…key_value.standard`, … (10 paid tiers; `free` tiers have no meter — nothing is owed). A Stripe meter is one scalar per customer, so tiered rating needs one meter per priced dimension.
- **`egress_bytes` → `egress_gib`** and **`storage_gb_seconds` → `storage_gb_hours`** — re-based so the per-unit cent rate fits Stripe's 12-decimal `unit_amount_decimal` cap (per-byte `$1.4e-11` is 16 decimals in cents; per-GiB is `1.5¢`). The billing client (`internal/billing/stripe.go` `stripeMeterEvent`) composes the identical event names and divides the value by `2^30` / `3600` in lockstep.
- **`build_seconds`** stays per-second.

Validated live: per-tier + re-based meter events **accepted**; `instance_seconds.service.starter` links to a metered price at `0.0001864554¢/unit` (= `pricing.yaml`'s `$0.000001864554/s`).

**Provisioned per environment during operator activation:**

- **Webhook endpoint** (`/v1/webhooks/stripe`) — created with the version/event in the Stripe setup runbook; its one-time signing secret goes directly into out-of-band custody.
- **Subscriptions** are per-customer (the "contract" equivalent) — provisioned by the code at first sight of a billable workspace (t005), not a global setup step.

## Notes

- **Supersedes** ADR040 Decisions §1–2 (Metronome as the sink); the metering/outbox/seal/enforcement design stands. t007 records this in the ADR.
- Stripe CLI config (meters/prices/webhook) creates real objects in a **live Stripe account** — an operator step in the t001 runbook, run with the operator's own restricted key, never committed.
- High volume: bex's ~72k events/day is well under the v1 `meterevent` limit; the v2 stream (`v2/billing/metereventstream`) is the escape hatch if ever needed.

## Source + Goal linkage

- **Source:** User request on 2026-07-20 to replace the two-vendor Metronome path after Stripe completed its acquisition; supersedes ADR040 Decisions §1–2.
- **Goal linkage:** Advances ADR008 pillar 2 (complete managed-service experience) by connecting the existing usage surface to native rating, invoicing, and collection without putting money logic in the operator.
- **Expected outcome:** Enabling the Stripe runtime key makes sealed usage produce rated Stripe invoices that the unchanged REST, GraphQL, MCP, and dashboard billing fields read back; disabling it leaves estimate-only behavior unchanged.
- **Why now:** m47/m48 already shipped the outbox and tenant-facing billing surface, so retaining the interim Metronome client now duplicates vendors, secrets, customer provisioning, and collection integration.
- **Render parity:** Included as t008 because the implementation changes the meaning and source of the tenant-facing usage billing fields even though it adds no new adapter fields.

## Resolution

Completed 2026-07-26. The m47 outbox now provisions metadata-keyed Stripe Customers and one complete metered Subscription, emits the paid catalog's meter events, reads Stripe invoice previews/history through the unchanged m48 result, applies the stable Mode-B comp coupon, and mounts version-compatible signature-verified `invoice.payment_failed` intake only when configured. Metronome runtime code, module dependencies, environment variables, setup runbook, and reconciliation script are gone. ADR040 and the activation/rollback runbook record Stripe's real 35-day timestamp bound and ≥24-hour identifier-uniqueness guarantee without claiming strict cross-system exactly-once delivery.

Verification: `cd lego/backend && go test ./... && go build ./...`; `cd lego/operator && make test && make lint`; `cd dashboard && yarn test` (262 files / 1,624 tests); `python3 -m unittest scripts/test_stripe_billing_setup.py` (6 tests); module/config searches found no active `BEX_METRONOME_*` or Metronome SDK reference. No live Stripe secret or account mutation is part of repository closeout; per-environment catalog/webhook activation follows `docs/runbooks/stripe-billing-setup.md`.
