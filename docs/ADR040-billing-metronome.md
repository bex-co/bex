# ADR040 — Billing via Metronome (usage export → rating → invoicing)

**Status:** Proposed · 2026-07-19 (Phase 1 shadow export implemented w7/m47; Phases 2–3 proposed) **Author:** (billing workstream)

---

## Context

[ADR023](ADR023-usage-metering.md) shipped the hard half of billing — **metering**. Four meters (`instance_seconds`, `egress_bytes`, `build_seconds`, `storage_gb_seconds`) roll up hourly from Prometheus/kubelet/k8s into the control-plane Postgres (`usage_hourly`, compacted into `usage_monthly`), surfaced over REST/GraphQL/MCP. [ADR030](ADR030-pricing.md) added a **static price sheet** (`internal/pricing/pricing.yaml`) that turns those quantities into an advisory `estimatedCost` — explicitly "estimate only", **no payment collection, no invoicing, no dunning**.

Both ADRs left a forward pointer to the deferred other half — **rating + invoicing + collection**. `lego/types/tiers/tiers.yaml` carries the comment `# prices are Metronome's (billing integration future work)`; `.pm/FUTURE-MAYBE.md` names the trigger ("a hosted bex offering becomes roadmap-worthy") and points at [Metronome](https://metronome.com) as the candidate.

This ADR designs that integration. The guiding constraint: **bex should not build a billing system.** Metronome is a usage-based billing platform that ingests usage events, aggregates them into billable metrics, rates them against per-customer contracts, and emits invoices. bex already produces the events; the integration is an **export seam**, not a new subsystem.

### What Metronome provides

Metronome separates _metering_ (measurement) from _rating_ (pricing). Its object model, in setup order:

| Object | Role | API |
| --- | --- | --- |
| **Customer** | Invoice recipient. One per bex workspace (`tenant`). | `POST /v1/customers` (with `ingest_aliases[]`) |
| **Billable metric** | Aggregates raw events (sum/count/max/unique) into a charge. | `POST /v1/billable-metrics/create` |
| **Product** | A line item on an invoice. | `POST /v1/contract-pricing/products/create` |
| **Rate card** | Centralized per-metric pricing. | `.../rate-cards/create` + `.../rate-cards/addRates` |
| **Contract** | Binds a customer to a rate card + billing period. | `POST /v1/contracts/create` |
| **Usage event** | One raw usage record. | `POST /v1/ingest` (batches ≤ 100) |
| **Invoice** | Auto-generated per period (Draft → Finalized → Sent). | read via API |

Three mechanics drive the design below:

1. **`transaction_id` is an idempotency key with a 34-day dedup window.** Re-sending the same `transaction_id` within 34 days is ignored — retries are always safe. Metronome recommends deriving it deterministically from business logic, not randomly.
2. **`ingest_aliases`** map _our_ identifiers onto a Metronome customer. Sending usage keyed on an alias auto-associates it — so bex uses `tenant.id` (`tea-…`) directly as the customer key and never has to store Metronome's own UUID on the hot path.
3. **Billable metrics are not retroactive, and an accepted event cannot be overwritten by re-sending its `transaction_id`.** This is the one real friction point — see Decision §3.

An official Go SDK exists: `github.com/Metronome-Industries/metronome-go/v3` (`client.V1.Usage.Ingest(...)`, Go 1.22+), which fits the backend module.

---

## Decision

### 1. Metronome is the billing source of truth; `usage_hourly` is the export ledger

bex keeps `usage_hourly`/`usage_monthly` as the **operational** record (dashboards, the ADR030 estimate, retention). A new emitter ships each sealed hourly row to Metronome's `/v1/ingest`. Metronome computes billable metrics, rates them, and issues invoices. **`pricing.yaml` is demoted** from "billing truth" to "fast real-time estimate for the dashboard"; the invoice truth moves to Metronome's rate cards.

The emitter lives in the **backend** module (bex-api process, which already runs `usage.Service` when `BEX_CP_DB_URI` is set). The operator never touches it — money must not reach a mechanism layer, exactly as ADR030 kept prices out of `tiers.yaml`.

### 2. One workspace = one Metronome customer, keyed by ingest alias

Each `tenant` (`tea-…`) maps 1:1 to a Metronome customer. On first export (or on tenant creation), bex calls `POST /v1/customers` with `ingest_aliases: ["tea-…"]`. Usage events then carry `customer_id: "tea-…"` and Metronome resolves the alias. bex optionally stores the returned Metronome customer UUID on `tenants` for admin/audit, but the ingest path depends only on the alias.

### 3. Seal-then-emit — export only rows past the rewrite horizon

**The problem.** bex's rollup rewrites an hour's `quantity` after the fact (catch-up gap-fill, deferred Prometheus windows). ADR023's compaction already clamps to **48 hours ago** precisely because windows younger than that may still change. But Metronome dedups by `transaction_id` for 34 days and metrics are not retroactive — so if bex emitted at the hour boundary and the value later changed, re-emitting the corrected value under the same `transaction_id` would be **silently dropped**.

**The decision.** Export only rows whose `window_start < now − BEX_METRONOME_SEAL_HOURS` (default **48h**, aligned with the compaction horizon). Past that point a row is final, so it is emitted **exactly once** with a deterministic `transaction_id`. Because Metronome invoices monthly, a ≤48h export latency is immaterial. This avoids all delta/correction complexity — no versioned transaction ids, no restatement events.

```
transaction_id = sha256("<resource_kind>|<service_id>|<kind>|<tier>|<window_start RFC3339>")
```

Deterministic ids make crash-recovery, retries, and accidental re-scans safe by construction.

### 4. Outbox via an `emitted_at` column

A migration adds a nullable `emitted_at timestamptz` to `usage_hourly`. The emitter loop selects `WHERE window_start < seal_horizon AND emitted_at IS NULL`, batches ≤ 100 rows, POSTs to `/v1/ingest`, then stamps `emitted_at`. This is a transactional outbox: it decouples ingest reliability from the metering loop, survives Metronome downtime, and — combined with the deterministic `transaction_id` — makes delivery safely at-least-once. Retry policy follows Metronome's guidance: exponential backoff on `429`, retry `5xx`, route non-429 `4xx` to a dead-letter log rather than blocking the loop.

### 5. Event and metric mapping

One `event_type` per meter kind; the numeric quantity and all dimensions travel as string `properties`:

```json
{
  "transaction_id": "…",
  "customer_id": "tea-abc123",
  "timestamp": "2026-07-19T01:00:00Z",
  "event_type": "instance_seconds",
  "properties": {
    "tier": "starter",
    "resource_kind": "service",
    "service_id": "srv-xyz",
    "value": "3600"
  }
}
```

In Metronome (dashboard config, not bex code), each billable metric is `sum(properties.value)` filtered by `event_type`, grouped by `tier` / `resource_kind`. `pricing.yaml`'s tiers seed the rate card: every `usdPerSecond` / `usdPerByte` becomes a rate on the matching metric, preserving ADR030's discount policy (30% off Render; bandwidth 90% off).

Volume is modest — one event per (resource × meter × hour). At 1000 resources × ~3 meters × 24h ≈ 72k events/day ≈ 720 batched requests/day.

### 6. Env-gated, byte-identical when off

| Variable | Meaning | Unset ⇒ |
| --- | --- | --- |
| `BEX_METRONOME_TOKEN` | Bearer token (out-of-band secret) for the ingest/customer API. | **export disabled — byte-identical to today** |
| `BEX_METRONOME_URL` | API base (default `https://api.metronome.com`). | default |
| `BEX_METRONOME_SEAL_HOURS` | Rewrite horizon before a row is exported (default `48`). | `48` |
| `BEX_METRONOME_EPOCH` | RFC3339 "billing starts here" floor — the emitter never ships a row whose `window_start` predates it (§7). | unset ⇒ effective floor is `now − backfill horizon` at first enable |

With `BEX_METRONOME_TOKEN` unset the emitter never starts; bex behaves exactly as it does under ADR030 (estimate-only). `.env.example` is updated per the repo rule.

### 7. Enable/disable & lifecycle

Disabling a _billing_ system naively causes real damage — lost revenue, double charges, silent gaps. bex avoids this by never using one switch. There are three, at three layers, and they must stay independent.

**The invariant that makes disable safe: metering is never gated by billing.** `usage_hourly` keeps filling regardless of Metronome's state. Billing only ever toggles _export_ and _rating_ — never _collection_. Gating the rollup on billing would lose usage permanently and make the period unbillable forever.

| Layer | Controls | Switch | Off ⇒ |
| --- | --- | --- | --- |
| **Metering** | Collecting `usage_hourly` | _none — always on_ | (never disabled) |
| **Emission** (global) | Whether bex exports at all | `BEX_METRONOME_TOKEN` (env, deploy-level) | byte-identical; no traffic; no customers created |
| **Rating** (per-workspace) | Whether a workspace is _charged_ | Metronome **contracts** (billing API, _not_ bex env) | usage lands in Metronome but rates to nothing |

Per-workspace billing is controlled in **Metronome (contracts + rate cards), not by gating emission per tenant in bex**: emit every real workspace uniformly and let the rating engine decide the price (a free/grandfathered/internal workspace = no contract or a $0 rate card ⇒ $0 invoice, but its usage stays complete for later). The one bex-side exception is a `tenants.billing_excluded` marker so genuinely internal/test workspaces never even create a Metronome customer.

**Operations:**

- **Enable the feature:** run Phase-0 rate-card setup → set `BEX_METRONOME_TOKEN` + `BEX_METRONOME_EPOCH` → deploy. The emitter backfills sealed rows _from the epoch forward_.
- **Pause (reversible):** unset `BEX_METRONOME_TOKEN`, redeploy. Metering continues, `emitted_at` stays NULL, zero charges; re-enable drains the outbox — within the horizon below.
- **Onboard one workspace:** its customer auto-creates on first emit; create a contract with `start = billing start`.
- **Offboard one workspace:** end its contract (Metronome finalizes a correct partial invoice); keep emitting for continuity, or set `billing_excluded` to also stop event creation.
- **Emergency kill:** unset the token — export stops on the next reconcile, Metronome retains what it has and finalizes on schedule. No half-charged state.

**Why it is correct:** (1) _decoupled_ — usage is never lost when billing is off; (2) _idempotent_ — deterministic `transaction_id` + the `emitted_at` outbox mean toggling off↔on can never double-bill (34-day dedup + the stamp) and never skips a sealed row (outbox re-scan); (3) _reversible within a horizon_; (4) _default-off & byte-identical_.

**Caveats that must be handled (not optional):**

- **The epoch guard is essential.** Without a floor, the _first_ enable would try to ship every sealed row ever — months of history, most older than the dedup window (⇒ `4xx` flood) and billing users for pre-billing usage. The emitter skips `window_start < max(BEX_METRONOME_EPOCH, now − backfill_horizon)` and logs a one-time gap warning. The epoch _is_ the platform's "billing starts here" line.
- **Long pause = permanent gap.** A disable lasting **> ~34 days** cannot be back-ingested — the off-period is a hole in the invoice. Decide the policy (accept + document, or manual reconciliation) before pausing that long.
- **Pause ≠ terminate.** Pausing global emission mid-month _undercounts_ the current invoice (missing tail); terminating a contract _finalizes_ it correctly. Use the operation that matches intent.

**Comped / exempt tenants (see spend, pay nothing).** Internal, superadmin, or comped workspaces must never be charged yet must still see their cost. This needs no special code path, because _seeing spend_ (the rating/estimate layer) and _paying_ (the collection layer) are already independent: `estimatedCost` (`pricing.yaml`, ADR030) is a **metering-layer read computed for every workspace regardless of Metronome state**, so cost visibility never depends on being billed. Two exemption modes:

- **Mode A — `billing_excluded` + estimate (recommended for superadmin/internal).** Set `tenants.billing_excluded=true` ⇒ the workspace never enters Metronome (no customer, no events, no contract, no invoice). Collection is therefore _structurally impossible_, not merely configured off — the strongest guarantee, and it survives Phase 3 automatically. The workspace still sees `estimatedCost` ("what this usage would cost at list price") like any other. This is the canonical use of the `billing_excluded` marker from §6/m18.
- **Mode B — comped contract, net $0 (for partners you want in the billing system).** Provision the customer + contract so Metronome _rates_ the usage (real line items), then apply a **100% discount** or a recurring **credit ≥ balance** ⇒ net invoice `$0`, and mark the customer **non-collectible / no payment method** as belt-and-suspenders so Phase 3 still charges nothing. The workspace sees a real invoice showing gross cost minus the comp = `$0` due.

Both switches decide _whether money is owed_, so they are **privileged: admin-only, set through the control-plane internal API (never tenant-editable), and written to `audit_events`.** (Distinct concern: a platform superadmin viewing _other_ tenants' spend is an authz/reporting question, not a billing-exemption one.)

### 8. Phased rollout

| Phase | Scope | Changes bex behavior? |
| --- | --- | --- |
| **0 · Metronome config** | Create billable metrics, products, rate cards mirroring `pricing.yaml` (dashboard/API; no bex code) — runbook: [`docs/runbooks/metronome-billing-setup.md`](runbooks/metronome-billing-setup.md). | no |
| **1 · Shadow export** | Emitter + `emitted_at` migration + customer provisioning, `BEX_METRONOME_TOKEN`-gated. Reconcile Metronome's computed usage against `usage_hourly`. | no (pure sidecar) |
| **2 · Billing surface** | Per-customer contracts; read real invoices/costs from Metronome; surface alongside/replacing `estimatedCost`. | display layer |
| **3 · Collection** | Metronome → Stripe (or a Metronome payment connector) for actual charging + dunning, plus bex's **non-payment enforcement** (§9): dunning → grace → suspend → terminate. | new |

Phase 1 is the MVP and can ship independently: it produces real billing data in Metronome without altering any tenant-visible behavior, letting us validate the mapping before anything is charged.

### 9. Non-payment enforcement — dunning → grace → suspend → terminate

Phase 3 turns computed invoices into collected revenue, which means bex finally needs an answer to _"an invoice was issued and never paid — then what?"_ This section designs that policy. **It is Phase-3 design, not yet implemented**, and it inherits ADR040's invariants rather than inventing new ones.

**Four principles the policy must not violate:**

1. **Metering is never gated (§7).** A delinquent — even suspended — workspace keeps metering whatever still runs; the debt is never silently forgiven. Enforcement pauses _resources_, never the _meter_.
2. **Metronome is the source of truth for payment status.** bex never computes "did they pay." It reacts to Metronome/Stripe invoice + payment events and owns only the _reaction_ (what to pause, when). Charging, retries, and proration stay in Metronome+Stripe — bex does not reimplement a dunning engine.
3. **Reversible until termination.** Every step short of deletion is undone by payment. Suspension preserves data, config, URLs, and certs (ADR007), so paying reinstates the workspace intact and instantly.
4. **Enforcement can never touch a non-billable workspace.** A `billing_excluded` (§7 Mode A) or comped-to-$0 (Mode B) workspace owes nothing, so it can never be delinquent — the state machine below structurally never selects it.

**The per-workspace delinquency state machine.** A new `tenants.billing_status` is driven by Metronome invoice/payment events: `active → past_due → suspended → terminated`, with recovery edges back to `active` on payment.

| State | Entered when (from Metronome/Stripe) | bex action | Metering | Reversible? |
| --- | --- | --- | --- | --- |
| **active** | no open past-due balance | none | on | — |
| **past_due** | a finalized invoice is unpaid past its due date while Stripe's dunning retries run | notify owner + admins (email + dashboard banner) with a billing-portal deep link; **services keep running** (grace) | on | auto, on payment |
| **suspended** | the grace window elapses with the balance still open | suspend the workspace: `spec.suspended=true` on every App (compute → 0), then hibernate managed Postgres/Key Value after a further window; data + config + certs kept (ADR007) | on (whatever still runs) | resume on payment |
| **terminated** | the long-delinquency window elapses | teardown via the existing audited delete path (w7/m12) after a final data-export offer | stops (resources gone) | no — re-onboard only |

**Timeline (defaults; all env-tunable, all measured from first `past_due`):**

- **Day 0** — invoice finalized and auto-charged (Metronome → Stripe). Paid ⇒ stays `active`.
- **Charge fails** ⇒ `past_due`. Stripe smart-retries over its own dunning window (~1–3 weeks); bex emails on entry and warns before each escalation. Nothing is paused yet — this is the grace period.
- **`BEX_BILLING_GRACE_DAYS`** (default 14) still unpaid ⇒ `suspended`: every App scales to 0. Owners are warned 72h and 24h ahead. Compute cost (`instance_seconds`) stops immediately; the workspace is instantly recoverable.
- **`BEX_BILLING_HIBERNATE_DAYS`** (default 30) ⇒ also hibernate managed Postgres/Key Value (stop their compute; the PVC/backup snapshot is retained) to stop datastore compute accrual while data stays recoverable.
- **`BEX_BILLING_TERMINATE_DAYS`** (default 60; must exceed the 34-day dedup horizon and the hibernate window) ⇒ `terminated`: the w7/m12 teardown runs after emailing a final export link.
- **Any payment before termination** ⇒ immediate `active`: resume Apps (`suspended=false` restores the saved replica count, readiness-gated per ADR007) and un-hibernate datastores. Idempotent.

**Where the code lives — backend only, never the operator.** Consistent with §1 (money never reaches the mechanism layer): the operator only ever sees `spec.suspended`, which it already honors (ADR007), and never learns _why_ a workspace is parked. A backend enforcement reconcile loop (beside the m47 emitter) maps Metronome invoice status → `tenants.billing_status` → the existing row-first suspend path (`SetAppSuspended` and the `Database`/`KeyValue` `Suspended` field) across every resource the tenant owns. Every transition is written to `audit_events` under a fixed verb (`billing.EnforcementTransition`, from→to), exactly the audited-privileged-write shape m47 established for `billing.SetExclusion`.

**Trigger: webhook first, poll as backstop.** bex adds an HMAC-verified `POST /v1/webhooks/metronome` (the git-webhook pattern) that maps `invoice.finalized` / `invoice.payment_failed` / `invoice.paid` to state transitions. A slow reconcile poll of open invoices is the belt-and-suspenders backstop for a missed webhook — the same outbox-style durability the emitter uses.

**Escape hatches — privileged, admin-only, audited (same custody as `billing_excluded`):**

- **`tenants.enforcement_hold`** (bool): pauses _automatic_ enforcement for one workspace — an enterprise account a human handles, a billing dispute, or a Stripe/Metronome outage. The workspace may sit `past_due` but is never auto-suspended while held. Set through the control-plane internal API, written to `audit_events`, never tenant-editable.
- **`BEX_BILLING_ENFORCEMENT`** global gate: unset/`off` ⇒ enforcement never acts. States are still tracked and surfaced (observe-only), but nothing is suspended — so Phase 3 can ship "watch the delinquency signal" before it ever parks a workspace, exactly as m47 shadow-exported before a cent was charged. **Default off ⇒ byte-identical to Phase 2.**

**Why suspend before delete.** Suspension is the reversible, data-preserving lever (ADR007): it stops the compute bleed (`instance_seconds` → 0) the moment a workspace is delinquent, while giving the customer a clean, instant recovery on payment. Deletion is irreversible and strictly last-resort, after a long well-notified window, and reuses the audited w7/m12 teardown rather than a bespoke destroy path — no new way to lose tenant data is introduced.

**Interactions:** billing-suspend and free-tier auto-sleep both scale to 0 but are different intents; billing-suspend **wins** and blocks wake (a delinquent free-tier workspace stays parked). Maintenance mode (ADR007) is orthogonal public-routing intent, not a payment state. Notifications reuse the existing email seam (deploy-failure/invite mailer, `BEX_DASHBOARD_URL` deep links).

**Deferred to Metronome/Stripe, not bex code:** retry cadence (Stripe dunning config), partial payments and proration (Metronome), and per-plan grace differences (enterprise longer) via contract metadata rather than bex branching.

---

## Consequences

- **Backend:** a new `internal/billing` (or `usage/metronome.go`) emitter runs inside bex-api, reading `usage_hourly`. `internal/usage` gains a migration for `emitted_at`. The Metronome Go SDK is added to the backend go.mod.
- **Store:** `usage_hourly` grows one nullable column; no change to the primary key or read paths. `usage_monthly` and the ADR023 retention/compaction loop are untouched.
- **Pricing:** `internal/pricing` stays as the dashboard's fast estimate. If Phase 2 surfaces Metronome's real cost, `estimatedCost` and the invoiced amount coexist (estimate for the current in-flight window, invoice for closed periods).
- **Operator:** unchanged. No dependency on billing, consistent with ADR030 §1.
- **Ops:** a new out-of-band secret (`BEX_METRONOME_TOKEN`) enters the credential inventory ([ADR019](ADR019-infra-credentials.md)); Metronome rate-card config becomes a runbook step. Metronome outages degrade gracefully — the outbox backs up and drains, and invoicing is monthly. Enable/disable is a three-layer model (§7): metering is always on, emission is the global env gate, and per-workspace charging is a Metronome contract — never a per-tenant emission gate.
- **Enforcement (Phase 3, §9):** non-payment drives an audited, reversible ladder — `past_due` (grace) → `suspended` (compute→0 via ADR007, data kept) → `terminated` (w7/m12 teardown) — that reuses existing mechanisms and never adds a new way to lose tenant data. It touches no non-billable workspace, keeps metering on throughout, and is globally gated (`BEX_BILLING_ENFORCEMENT` off ⇒ observe-only), so it can ship watching-before-acting. Adds one control-plane store column (`tenants.billing_status`), one admin escape hatch (`tenants.enforcement_hold`), and a Metronome webhook receiver.
- **Render parity:** none affected — Render exposes no usage/billing API; this is a bex-only extension already marked "bex ahead" in [ADR018](ADR018-render-parity.md).

---

## Alternatives considered

**Emit at the hour boundary + send correction deltas.** Rejected for the MVP. It gives sub-hour billing latency but forces versioned transaction ids, per-window last-emitted-value tracking, and signed delta events to work around Metronome's 34-day dedup. Monthly invoicing makes the ≤48h seal latency free, so the complexity buys nothing now. Revisit only if real-time (intra-day) billing analytics become a product requirement.

**Push events straight from the rollup loop (no outbox).** Rejected — couples metering correctness to an external API's availability. A Metronome 5xx or network blip would either stall the rollup or silently drop billing data. The `emitted_at` outbox isolates the two.

**Send raw per-request events instead of pre-aggregated hourly rows.** Rejected — bex has no per-request usage stream; the hourly rollup _is_ the source. Metronome explicitly supports pre-aggregated `sum` events, and hourly granularity keeps event volume trivial.

**Build invoicing/payments in-house.** Rejected — the whole point. bex would be rebuilding rating engines, tax, dunning, and payment reconciliation that Metronome (+ Stripe) already provide. `tiers.yaml`'s "prices are Metronome's" pointer commits to this direction.

**Keep `pricing.yaml` as billing truth and skip Metronome.** Rejected — `estimatedCost` is advisory by construction (ADR030 §4): no invoices, no collection, no audit trail, no contract/period model. A hosted offering needs real invoices, which is exactly Metronome's job.
