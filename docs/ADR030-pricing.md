# ADR030 — Price sheet + estimated spend

**Status:** Accepted · 2026-07-13 · revised 2026-08-19 (workspace plan fees at 30% off Render) · revised 2026-07-26 by w7/m50 · revised 2026-09-06 (§7 paid-only Background Workers, w6/025) · revised 2026-10-15 (§7 paid-only Cron Jobs, w8/048) · revised 2026-11-16 (§18 cron job minimum, w9/m448) · revised 2026-11-26 (§19 the idle plan-fee waiver, w4/m689) · revised 2026-11-26 (§9 Render's price tables guarded weekly, w4/m690) **Author:** w8/m7

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

**Public read (2026-11-14).** bex-api serves the customer-facing part of the sheet without a credential at `GET /pricing` (`internal/pricing/public.go`). That part is every rate and the plan fees and included amounts. The agent model section (markup, caps, trust tiers, upstream costs) stays private. Its shape is pinned by `internal/pricing/testdata/public_sheet.json`. bex.co (eden-cms-v2) fetches it on `yarn dev`, and its tests check the pricing page against it, so a price change that forgets bex.co fails there.

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
| Sandbox snapshot storage | $0.08/GB-month | $0.056/GB-month | 30% off |
| Bandwidth | $0.15/GB | $0.015/GiB | 90% off |

**Agent model tokens (2026-10-30, [ADR091](ADR091-agent-model-pricing.md)).** Render has no model-token SKU, so the "Render × 0.70" rule cannot apply. Platform-route agent model usage is cost-plus instead: the provider's actual per-request cost × 1.30, metered as `agent_model_cost_microusd` and billed through one model-agnostic Stripe meter.

Workspace plan fees are **licensed monthly SKUs**, not usage meters. They appear on the dashboard plan picker and in `pricing.yaml`; they are **not** in `BillableMeterNames` and the Stripe setup script does not provision them as metered Prices. Enterprise remains custom (no catalog rate). Resource-tier usage is billed on top of the workspace fee.

**Scale and Enterprise are charged once per organization (w7/m464, [ADR115](ADR115-organizations.md) D2).** On Render, "An org can contain unlimited workspaces at no additional cost" (render.com/docs/organizations, read 2026-11-17), and the primary workspace "manages the billing for the organization". So the Scale or Enterprise fee is listed once per org, on its primary workspace: the plan picker and `GET /pricing` say "per organization", and a workspace created in the org shows "Included in your organization's plan". Usage stays billed per workspace. bex charges no licensed plan fee today, so no customer was charged twice; a future licensed SKU attaches once per org, to the primary's subscription. This is a recommendation under Render parity first, not a user decision.

Source: `docs/render-artifacts/pricing.md` (captured 2026-07-13; bandwidth re-verified 2026-07-15 after Render's new workspace-plan rollout). **Guarded since 2026-11-26 (w4/m690):** render.com/pricing's price tables are captured row by row in `lego/backend/internal/pricing/testdata/`, `price_table_test.go` holds each plan fee and each compute, Postgres, Key Value, cron and sandbox rate on this sheet to Render's × 0.70 to the cent, and a weekly job fails when Render's page changes a captured row (§9).

**Postgres tiers (w2/m171, 2026-10-29).** Render's Postgres prices were re-fetched on 2026-10-19 (render.com/pricing). The two small tiers had been recorded at $7 / $20 while Render lists $6 / $19, so they move onto the rule (t012, absorbing w2/047). The three ≥1-CPU tiers are the first plans that can run high availability. A high-availability database runs a primary and a standby and meters two instances, because Render bills its standby like the primary.

| bex plan (Render id) | CPU / RAM | Render | bex per instance-month | per second |
| --- | --- | --- | --- | --- |
| `basic-256mb` (`0.1c-256mb`) | 0.1 / 256 MB | $6 | $4.20 (was $4.90) | 0.000001598173515982 |
| `basic-1gb` (`0.5c-1g`) | 0.5 / 1 GB | $19 | $13.30 (was $14.00) | 0.000005060882800609 |
| `1c-2g` (`1c-2g`) | 1 / 2 GB | $40 | $28.00 | 0.000010654490106545 |
| `pro-4gb` (`1c-4g`) | 1 / 4 GB | $55 | $38.50 | 0.000014649923896499 |
| `basic-4gb` (`2c-4g`) | 2 / 4 GB | $75 | $52.50 | 0.000019977168949772 |

Per-second rates are `monthly / 2,628,000` (a 730-hour month). Live Stripe catalog provisioning for the new and corrected prices is still pending; once done, existing Stripe Subscriptions move to the corrected prices at their next billing period ([runbook](runbooks/stripe-billing-setup.md#repricing-a-dimension-the-2026-10-postgres-catalog-change-w2m171)).

The storage estimate prices `storage_gb_seconds`, using a 730-hour pricing month. Render bills provisioned Postgres capacity; bex's collector measures actual used PVC bytes. Render does not list a separate Key Value storage charge, so applying this same used-storage rate to Valkey is a deliberate bex extension rather than a claim of exact Render shape. **A Free Postgres or Key Value is exempt (w4/427, 2026-10-28):** its fixed included storage costs $0, as Render's Free Postgres does (1 GB included, [ADR009](ADR009-postgresql-management.md)). Storage usage rows carry the datastore's plan, the estimate prices a `free` row at $0, and the Stripe emitter sends no `storage_gb_hours` meter event for one. The usage row stays, so the Billing page still shows the GB used. Rows written before this change carried no plan; migration 0171 (w4/431) re-keys this month's (October 2026) untiered storage rows from the same datastore's `instance_seconds` row for that hour, so October's Free storage prices at $0 and any not-yet-emitted Free row sends no meter event. Earlier months stay as invoiced. Meter events already sent to Stripe for Free storage are not reversed by code; whether to credit them is an open billing decision (w4/431).

**Service disk (`disk_gb_seconds`, [ADR082](ADR082-persistent-disks.md) D8/D9) is the exception, and deliberately so.** It is the one storage meter billed on **provisioned** GB rather than used bytes, because that is what the underlying Hetzner volume actually costs: a 100 GB volume is billed at 100 GB whether the tenant wrote one byte or ninety. Metering used bytes would put bex on the wrong side of the margin on every under-filled disk, and — more importantly — would make the number the tenant sees on the Disk tab ("100 GB") disagree with the number on the invoice. Two consequences follow, both intentional: a disk bills from attach to delete regardless of whether the service is running (deleting the service releases the disk; stopping it does not), and the Blueprint/pricing **estimate equals the invoice** for this SKU — the only one where it does, since every other storage line estimates provisioned floor while metering used bytes. The rate is Render's $0.25/GB-month at the standard 30% discount ($0.175). bex's measured Hetzner cost on the production USD-billed account is **$0.0767/GB-month** (~56.2% gross margin — kept deliberately for sheet consistency; see ADR082 D8 / w2/034); snapshot object storage and its egress remain in that margin.

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

### 6. The free compute plan is capped at one running instance

The compute `free` tier is rated `usdPerSecond: 0.0` in `pricing.yaml`. A zero rate has an edge no paid tier has: N free instances would deliver N× the capacity for $0, so an uncapped free plan is effectively unlimited free compute. The free plan is therefore capped at **one running instance**.

The cap is a resource-sizing fact, not a price, so — consistent with the "no prices in `tiers.yaml`" invariant of §1 — it lives in the reviewed tier catalog rather than here: `lego/types/tiers/tiers.yaml` gives the compute `free` tier `maxInstances: 1` (paid tiers omit the field, so they carry no plan-specific cap, only the platform ceiling `MaxReplicas = 100`). The backend enforces it on every write path that sets an instance count (`Scale`, service create, autoscaling min/max), refuses a plan downgrade that would leave a service above the new cap ("scale down first", never a silent shrink), and the operator's plan-aware `clampReplicas` is the defense-in-depth backstop. See [ADR018 § Manual scale](ADR018-render-parity.md).

This is exact Render parity: Render's free instance types do not support horizontal scaling — running multiple instances is a paid-only feature, and Render's "billed accordingly" per-instance scaling copy applies only to paid instance types. Source: [render-artifacts/free-tier-scaling.md](render-artifacts/free-tier-scaling.md).

### 7. Background Workers, Private Services and Cron Jobs are paid-only (no free tier)

**Decided 2026-09-06 (w6/025), superseding the 2026-09-03 keep-Free triage recommendation.** A `background_worker` never runs on the compute `free` tier. bex's picker used to pre-select **Free — 512 MB / 0.1 CPU** for workers while Render's equivalent picker starts at a paid tier (2026-08-21 live QA pass); rather than record the zero-cost worker capacity as a deliberate differentiator, the product call is Render parity: workers must use paid compute.

**Extended to Private Services 2026-09-28 (w1/111).** Render sells a private service only on paid instances too ([explore other service types](https://render.com/docs/your-first-deploy#explore-other-service-types), checked 2026-09-28), and the 2026-09-06 decision left the sibling type accepting free purely because `normalizeTierForType` special-cased one constant. Live evidence of the resulting split: on 2026-09-26 a plan-less `private_service` was created on **`free`** while a plan-less `background_worker` was created on **`starter`**, from the same function, in the same pass. The rule is an **allowlist** (`paidOnlyServiceType`), not "everything but web"; a static site runs no instance at all.

**Extended to Cron Jobs 2026-10-15 (w8/048, user decision: match Render).** The earlier premise that "Render sells both that way" for web services and cron jobs was false for cron. Render's public API types `cronJobDetailsPOST.plan` and `cronJobDetailsPATCH.plan` as `paidPlan`, with no `free`, exactly as for workers and private services. render.com/docs/free lists no free cron instance, and render.com/docs/cronjobs states a $1/month minimum charge per cron job. A cron job is now refused `free` on create, plan change and Blueprint. A plan-less cron already defaulted to `starter` under §8. Blueprint `plan: free` on a cron is refused by the pinned upstream `cronPlan` enum: the bex schema overlay that admitted it (ADR049) is removed. Only web services keep Free. bex's per-second compute billing applies as for any paid service. **Amended 2026-11-16 (w9/m448):** the earlier closing clause, that bex added no monthly floor, was a divergence ADR018 never recorded and the user never signed. A cron job now has a monthly minimum, §18.

**A cron job's price is per minute of run time (w4/546, 2026-11-11).** A cron job is billed per second only while a run executes ("Billing is prorated by the second, based on active running time during a given month", render.com/docs/cronjobs), so its create form, its Plan page and the plan-change confirm show each instance type's rate per minute of run time ("$0.00011/min of run time" for Starter), with "billed per second while a run executes". The always-on monthly figure would overstate a nightly 30-second job about a thousandfold. The rate comes from the same price sheet (`pricing.Sheet.InstanceUSDPerMinute`, GraphQL `InstanceType.usdPerMinute`, two significant digits), so the client never divides a rounded monthly string. Other service types keep $/month, and the Blueprint estimate keeps excluding cron as a variable cost. Render's own per-minute wording is unverified.

Enforcement is backend-first, at the two choke points every surface funnels through (`lego/backend/internal/apps/service.go`):

- **Create** — `normalizeTierForType`, called from `specFromCreate`, which REST `POST /v1/services`, GraphQL `createService`, MCP `create_web_service` (its `type` accepts `background_worker`), MCP `create_cron_job`, and the Blueprint apply/validate paths all share. An **omitted** plan on a paid-only type defaults to the catalog's cheapest **paid** rung (`starter`, via `defaultPaidTierID`) instead of the catalog default `free`; an **explicit** free plan is refused with a 400 naming the paid plans that type may use — and naming **the type the caller asked for**, so the message does not send a private-service author to the worker docs.
- **Plan change** — `SetPlan` and its dry-run `SetPlanDryRun` (REST `PATCH /v1/services/{id}`, GraphQL `updateServicePlan`, and MCP `update_service` all fold into them) refuse a downgrade to free on any paid-only type with the same error.
- **Blueprint probes** — `stackHasPaidPlan` (the ADR046 payment-method gate) and `blueprintEstimatedPricing` resolve plans type-aware, so a plan-less paid-only service trips the paid gate and is priced at the paid default rather than $0.

The dashboard mirrors this as presentation only (`offeredInstanceTypes` in `dashboard/src/features/services/lib/instance-type.ts`): the create form's plan grid and the instance-type picker never offer Free to a paid-only type, the create default is the first paid tier, and a Free selection made under another service type does not survive a switch into one. The `tiers.yaml` catalog and the `instanceTypes` query are unchanged — free remains in the catalog for the type that keeps it (web; static sites run no instance).

**Existing free workers, private services and cron jobs — transition, no silent conversion.** Enforcement applies to new writes only: a worker already on `spec.tier: free` (or untiered) keeps running — the operator converts tier → pod resources mechanically and refuses nothing — and is **not** auto-converted to a billed plan. Identify them in the control-plane store with `SELECT id, name, tenant_id, type FROM apps WHERE type IN ('background_worker', 'private_service', 'cron_job') AND (tier = '' OR tier = 'free')` — run this against production before rollout and notify the owners of any hits. Their owners choose the transition: move to a paid plan (the picker now offers only paid tiers), change the service type, or delete the resource. For private services this matters more than it did for workers — QA history alone carries several free ones (for example `w4/139`), so the query is expected to return hits and the notify step is not a formality. The one-way door is deliberate: once such a resource leaves free it cannot return, and no new free worker, private service or cron job can be created on any surface. A Blueprint that still declares `plan: free` on an existing cron fails validation at that entry until its owner changes the plan, so the cron owners need the notice before the next sync. The cron demo example now declares `starter`. The internal control-plane create API (`store/api.go`) carries no service type and cannot enforce this rule; like the disk and maintenance-mode paid gates, enforcement is a bex-api boundary concern.

### 8. A plan-less service defaults to the paid starter tier, as on Render

**Decided 2026-10-15 (user, w1/122).** Render's Blueprint spec gives a service created without a `plan` the `0.5c-512mb` compute plan, which is bex's `starter` rung; bex used to create it on `free`, so a manifest or API call that omitted the plan landed on a different (free, sleeping) tier on bex than on Render. Every surface now follows Render: `normalizeTierForType` resolves an omitted plan to the cheapest paid rung for every type that runs an instance, so REST `POST /v1/services`, GraphQL `createService`, MCP `create_web_service`/`create_cron_job`, and Blueprint apply/validate (including `stackHasPaidPlan` and `blueprintEstimatedPricing`) all agree, and a plan-less create is paid intent for the ADR046 payment-method gate. A static site has no plan and keeps the catalog default. An explicit `plan: free` stays valid on a web service; a cron job became paid-only in §7 (w8/048). The dashboard is unaffected: its create form always sends the plan the user picked. Existing services keep their tier.

**A plan-less Blueprint Key Value creates on Starter (w33/m436, 2026-11-16). This is a recommendation under ADR008 Render parity first, awaiting the user's sign-off, not a recorded user decision.** Render's blueprint-spec § `plan` says "Render uses `256mb` for a new Key Value instance", and `256mb` is bex's `starter`. bex used to leave the plan empty, so the KeyValue CRD's `free` default won: one file gave a paid 256 MB store on Render and a free one on bex, priced at $0 in the review. `parseKeyValue` now writes `starter` into a plan-less entry. So `blueprintEstimatedPricing` prices it as Starter, `stackHasPaidPlan` counts it as paid intent, and the free-instance limit is not consulted for it; an explicit `plan: free` still creates a free store under that limit. As for databases, it is a create default: a sync that omits `plan` keeps the store's plan. The direct creates (REST, GraphQL, MCP) keep the CRD's `free` default, because Render's MCP server defaults to `free` and its REST API requires the field ([ADR021 §4](ADR021-keyvalue-management.md#4-plans--renders-key-value-vocabulary)). Keeping `free` for Blueprints would be a divergence, and that would need the user's sign-off in ADR018.

**An existing resource whose entry omits `plan` is priced and gated at the plan it keeps (w33/006, 2026-11-16).** The defaults above apply to creates. A sync or adopt keeps an existing service's, Postgres's or Key Value's plan when its entry omits `plan`, as Render "retains the current compute plan for an existing service". Until w33/006 the estimate and the payment-method gate still read the parse's create default for such an entry. So a file that only kept a free resource showed a Starter line, and under `BEX_REQUIRE_PAYMENT_METHOD=1` it asked a cardless workspace for a card. `stackAtKeptPlans` now puts each such entry on its live plan before `blueprintEstimatedPricing` prices the file and before `stackHasPaidPlan` judges its paid intent. It finds the existing resource the way the apply does. The validation dry-run, the Blueprint create and sync preflights and the stack apply all read it, so the gate and the estimate still agree. A create in the same file is still priced and gated at its default. Render does not document what its Estimated pricing panel shows for a resource it already has, so bex prices what the sync leaves running.

---

### 9. Workspace plans gate Render's plan features (w8/m137, w8/m138)

Render's [features-by-plan page](https://render.com/docs/platform-features-by-plan) is the source of truth, implemented as one table in `core/plan_gate.go` (`PlanAllows(plan, feature)`, one refusal sentence per feature) read through `core.Base.Plans`:

| Feature | Minimum plan | Notes |
| --- | --- | --- |
| Environments per project | Hobby: 2 | Pro and higher: unlimited |
| Network-isolated environments | Pro | turning it on is refused; an isolated environment can turn it off |
| Webhooks | Pro | one destination on Pro, up to 100 on Scale and Enterprise |
| HTTP request logs | Pro | explicit `type=request` refused; all-types queries and label discovery drop request lines and request-only values |
| Audit logs | Pro | events are still recorded on Hobby; reading them is refused, and after an upgrade the history starts at the upgrade (w9/098) |
| Response latency metrics | Pro | `http_latency` refused; request counts and bandwidth stay open |
| Autoscaling | Pro | turning it on is refused on every write path (w7/m429); a policy already on keeps running, re-sends unchanged and can be turned off |
| Inbound IP rules on services, static sites and environments | Scale | Postgres and Key Value rules stay on every plan |
| Enforcing 2FA (Workspace Settings → Two-factor authentication) | Pro | turning it on is refused (w7/070); a workspace that already enforces it keeps it, re-sends it unchanged and can turn it off |
| A billing email other than the owner's account email (Billing page → Billing email) | Pro | every write is refused on Hobby, whose billing email reads as the owner's account email; a move to Hobby clears the stored address and gives the Stripe Customer the owner's back (w6/m627, [billing-email.md](render-artifacts/billing-email.md)) |

**Free datastores (w4/m241):** every workspace, on any plan, may hold one free Postgres and one free Key Value (Render's free rule); paid instances never count toward it. Admission enforces it by instance plan (`FREE_INSTANCE_LIMIT`), and Hobby's total datastore quota is the paid ceiling (25/25) so a paid create is never refused for holding a free one. A terminating free instance still holds its free slot until deletion finishes. Every write that puts an instance on the free plan (a create, a recovery, or a plan change through the API or a Blueprint sync) checks the rule under the workspace's tenant lock, so concurrent writes cannot both pass it. A Blueprint sync judges the workspace its whole file leaves: two databases moving onto free are refused, and a file that hands the slot from one database to another is admitted in either order.

**Machine codes (w8/m138 t013):** a count cap answers `PLAN_LIMIT` (`params {plan, limit, field}`: members, environments per project) and a missing feature `PLAN_FEATURE_REQUIRED` (`params {plan, feature, requiredPlan[, field]}`, `feature` one of `networkIsolation`, `requestLogs`, `auditLogs`, `latencyMetrics`, `autoscaling`, `twoFactorEnforcement`, `billingEmail`), on REST `code`/`params`, GraphQL `extensions` and the MCP message prefix. Webhooks keep their earlier `WEBHOOK_PLAN_LIMIT`. Messages are unchanged. Blueprint validate reports the environment cap and network isolation at the authored entry with the code, and autoscaling at the service's `scaling` (w7/m429).

**Inventory (w8/m138 t006, 2026-10-29):** production held 64 Hobby, 2 Scale and 1 Enterprise workspaces, and no Hobby workspace had a project with three or more environments, an isolated environment or a webhook endpoint, so no migration was needed.

Existing state is never silently removed: a Hobby workspace keeps its third environment, isolated environments, webhook endpoints and IP rules working, and only new uses are refused. An unreadable plan fails the write; an unwired plan reader (store-off) leaves every feature open. Render's exact refusal wording and its request-log behavior on Hobby are unverified (no Render account); bex takes the documented 400 with the plan named.

**Every row of Render's plan tables has a disposition (w7/m429 t005, 2026-11-16).** Render publishes its plan differences twice: the comparison table on render.com/pricing (92 rows; the HTML table's first row, Compute, only links to the compute prices and is not in the page's markdown copy, so it left the fixture on 2026-11-26) and [platform-features-by-plan](https://render.com/docs/platform-features-by-plan) (27 lines). Both are captured row by row in [render-artifacts/pricing.md](render-artifacts/pricing.md) and as fixtures in `lego/backend/internal/pricing/testdata/`. `TestEveryRenderPlanRowHasADisposition` (`plan_table_test.go`) maps each row to exactly one disposition below. It fails on a captured row with none and on a disposition whose row is gone. Where the disposition is a number or a gate, it compares Render's value with the code's on every plan. Since w4/m690 a weekly job re-reads both pages and fails on a changed row (below). To take in a change, re-capture the fixture, then give every new row its disposition here and in the test.

Besides the gates in the table above, two limits follow the plan since w7/m429: the builds kept for rollback (Hobby 5, Pro 15, Scale and Enterprise 30, `core.KeptBuilds`, [ADR060](ADR060-build-worker-reliability-and-performance.md)) and the custom domains included before they are metered (Hobby 2, Pro 15, Scale 25, Enterprise all, `core.IncludedCustomDomains`). A third since w7/069: the sandboxes a workspace may have active at once across its groups (Hobby 100, Pro and higher 1,000, `core.ConcurrentSandboxes`, [ADR042 § Sandbox groups](ADR042-sandbox-cluster-substrate.md#sandbox-groups-w22m311)).

**The Workspaces and Enforce 2FA rows (w7/070, 2026-11-17).** Render's own pages disagree with its pricing table on both rows, so each was read against them. These are recommendations under [Render parity first](ADR008-vision.md#render-parity-first), not user decisions.

- **Enforce 2FA & Google login: "— | ✓ | ✓ | ✓".** [login-settings](https://render.com/docs/login-settings) (read 2026-11-17) names a plan only for SAML SSO and says nothing about Hobby. The table is the only Render source that states a plan for enforcement, so it wins, and enforcing 2FA is the Pro gate in the table above. A Hobby workspace's settings page could not be checked: bex has no Render account. Production held no workspace enforcing 2FA on any plan on 2026-11-17 (74 Hobby, 4 Pro, 3 Scale, 1 Enterprise), so the gate stops nothing that runs. Render's row also covers enforcing Google login, which bex gates the same way under its own key, `core.FeatureGoogleLoginEnforcement` (`googleLoginEnforcement`, Pro and higher, w7/m465, [ADR012 §9b](ADR012-auth.md#9b-workspace-enforced-google-login-w7m465)), so a refusal names the setting it refuses; a workspace that already enforces it keeps it.
- **Workspaces: "1 | 1 | Unlimited | Unlimited".** This row counts the workspaces one plan's account holds. Hobby and Pro are standalone workspaces. Scale and Enterprise come with an organization, and "An org can contain unlimited workspaces at no additional cost" ([organizations](https://render.com/docs/organizations), read 2026-11-17). How many workspaces one user may create is [team-members](https://render.com/docs/team-members)' rule: "You can have up to five Hobby workspaces" and "You can create an unlimited number of paid workspaces". bex applies that rule (`store.LimitsFor` `MaxWorkspacesPerUser`: 5 on Hobby, unlimited on paid plans). **Organizations (w7/m464, [ADR115](ADR115-organizations.md)), found by w7/070 as w7/077:** a Scale or Enterprise workspace is the primary of an org whose Owners add any number of workspaces under the primary's one plan fee, and Hobby and Pro have no org (`core.OrganizationPlan`). That is a recommendation under Render parity first, not a user decision. `checkWorkspaces` in `plan_table_test.go` fails if Render changes the row, if bex's org plans move, or if bex's per-user caps move off team-members' numbers.

| Disposition | Rows (table) | Where bex records it |
| --- | --- | --- |
| Gate (`core.planFeatures`) | Horizontal autoscaling (pricing), Autoscaling (features by plan) | core.planFeatures autoscaling |
| Gate (`core.planFeatures`) | Inbound IP rules (pricing), Expanded inbound IP rules (features by plan) | core.planFeatures inbound IP rules |
| Gate (`core.planFeatures`) | Network-isolated environments (pricing), Network-isolated environments (features by plan) | core.planFeatures network-isolated environments |
| Gate (`core.planFeatures`) | HTTP request logs (pricing), HTTP request logs (features by plan) | core.planFeatures HTTP request logs |
| Gate (`core.planFeatures`) | Audit logs (pricing), Audit logs (features by plan) | core.planFeatures audit logs |
| Gate (`core.planFeatures`) | Response latency metrics (features by plan) | core.planFeatures response latency metrics |
| Gate (`core.planFeatures`) | Webhooks (features by plan) | core.planFeatures webhooks, core.WebhookDestinationLimit |
| Gate (`core.planFeatures`) | Org-level audit logs (features by plan) | core.planFeatures organizations: an org exists on Scale and higher, and its audit log is its Owners' (ADR115 D7) |
| Gate (`core.planFeatures`) | Enforce 2FA & Google login (pricing) | core.planFeatures 2FA enforcement (w7/070) and Google login enforcement (w7/m465), both Pro and higher |
| Limit | Maximum services (pricing), Unlimited services (features by plan) | store.LimitsFor MaxServices |
| Limit | Instant rollbacks (pricing) | ADR060, core.KeptBuilds |
| Limit | Environments (pricing), Unlimited projects and environments (features by plan) | core.MaxEnvironmentsPerProject |
| Limit | Team seats (pricing), Unlimited workspace members (features by plan) | store.LimitsFor MaxMembers |
| Limit | Workspace user roles (pricing), Additional member roles (features by plan) | store.LimitsFor AllowedRoles |
| Limit | Workspaces (pricing) | core.OrganizationPlan: a Scale or Enterprise workspace is an organization's primary, whose Owners add any number of workspaces under its one plan fee; Hobby and Pro have no org (ADR115); per user, store.LimitsFor MaxWorkspacesPerUser |
| Limit | Organization user roles (pricing) | core.OrganizationRoles, store.OrgRoles: Owner, Member and Guest on Scale and Enterprise, none below (ADR115 D4) |
| Limit | Increased number of included custom domains (features by plan) | core.IncludedCustomDomains |
| Limit | Increased retention of past builds for rollbacks (features by plan) | core.KeptBuilds |
| Limit | Default concurrent sandboxes (pricing) | core.ConcurrentSandboxes (w7/069): 100 on Hobby, 1,000 on Pro and higher, across a workspace's sandbox groups |
| Included, then metered | Bandwidth (pricing) | ADR030 §14, pricing.yaml bandwidth.includedGB |
| Included, then metered | Custom domains (pricing) | ADR030 §16, core.IncludedCustomDomains, pricing.yaml customDomains |
| Included, then metered | Standard build pipeline (pricing) | ADR030 §13, pricing.yaml build.includedMinutes |
| Included, then metered | Increased monthly included amounts of pipeline minutes and bandwidth usage (features by plan) | ADR030 §13, §14 |
| Retention window | Log retention (pricing) | core.logRetention (w9/m389) |
| Retention window | Increased retention of historical service metrics and logs (features by plan) | core.logRetention, core.metricsRetention |
| Retention window | Point-in-time recovery window (pricing) | core.pitrRetention (w7/m453) |
| Same on every plan | Static Sites (pricing), Standard services (pricing), Web services (pricing), Private services (pricing), Background workers (pricing), Persistent disks (pricing), Postgres databases (pricing), Expandable storage (pricing), Read-only replicas (pricing), High availability (pricing), Key-value (Redis-compatible) (pricing), Cron jobs (pricing), Sandboxes (pricing), Maximum session length (pricing), Snapshot storage (pricing), Service scaling (pricing), Vertical scaling (pricing), Automatic private networking (pricing), Global regions (pricing), Zero-config CDN (pricing), Edge caching for web services (pricing), Automatic TLS (pricing), Wildcard domains (pricing), WebSockets (pricing), Service discovery (pricing), Automatic DDoS mitigation (pricing), Firewall (pricing), HTTPS only (pricing), Automatic builds (pricing), Native runtimes (pricing), Docker builds (pricing), Environment variables & secret files (pricing), Monorepo support (pricing), Deploy orchestration (pricing), Auto-deploys (pricing), Zero-downtime deploys (pricing), Deploy hooks (pricing), Pre-deploy command (pricing), Projects (pricing), Protected environments (pricing), Infrastructure as code (pricing), SSH access (pricing), Health checks (pricing), Service metrics (pricing), Notifications (pricing), 2FA & Google login (pricing) | per row in `plan_table_test.go` |
| Non-goal | Workflows (pricing) | DO_NOT_DO: Workflows & tasks; ADR018 Workflows & tasks |
| Non-goal | Parallel task compute (pricing), New task compute per minute (pricing), Increased per-minute task provisioning for Render Workflows (features by plan) | DO_NOT_DO: Workflows & tasks |
| Non-goal | Dedicated IPs (pricing) | DO_NOT_DO: dedicated outbound IPs; ADR018 Dedicated outbound IPs |
| Non-goal | Previews (pricing), Single-service previews (pricing), Full-stack preview environments (pricing), Preview environments (features by plan) | DO_NOT_DO: PR preview environments |
| Non-goal | Log streams (pricing) | DO_NOT_DO: external log/metric drains; ADR018 Log streams |
| Non-goal | OpenTelemetry metrics stream (pricing) | DO_NOT_DO: external log/metric drains; ADR018 Metric streams |
| Non-goal | SAML SSO (pricing), SCIM (pricing) | DO_NOT_DO: SSO/SAML · SCIM; ADR018 SSO / SAML · SCIM |
| Non-goal | Dedicated outbound IPs (features by plan) | DO_NOT_DO: dedicated outbound IPs |
| Non-goal | Metrics streaming (features by plan), Log stream overrides (features by plan) | DO_NOT_DO: external log/metric drains |
| Non-goal | SAML SSO and SCIM (features by plan) | DO_NOT_DO: SSO/SAML · SCIM |
| Open gap | Performance build pipeline (pricing), Performance build pipeline (features by plan) | ADR018 Performance pipeline tier, w6/m310 |
| Open gap | OIDC authentication (pricing), OIDC (features by plan) | ADR018 Managed OIDC for AI providers, w2/m476–m480 |
| Not applied yet: filed | AWS private links (pricing), Private link outbound bandwidth (pricing), HIPAA BAA (pricing), Private links (features by plan), HIPAA-enabled workspaces (features by plan) | w7/071 (HIPAA-enabled workspaces and AWS private links have no recorded disposition) |
| Commercial term, not a product surface | GDPR DPA (pricing), SOC 2 Type II (pricing), ISO 27001 (pricing), SOC 2 & ISO documentation (pricing), Email (pricing), Chat (pricing), Premium support (pricing), Private Slack channel (pricing), Dedicated technical account team (pricing), Support response SLAs (pricing), Migration assistance & architecture review (pricing), Compliance documentation (features by plan) | per row in `plan_table_test.go` |

**Render's price tables, its pricing FAQ, and the weekly job (w4/m690, 2026-11-26).** The tables above guard Render's plan differences. Its prices, which §2 discounts, were kept only as `pricing.yaml` comments and hard-coded test values, and nothing re-read either page: a Render change waited for someone to notice it.

- **Fixtures.** Seven price tables from render.com/pricing (read 2026-11-26), every row and column as displayed: the workspace plans (`render-plan-prices.tsv`, 4 rows), compute (`render-compute-prices.tsv`, 15), Postgres (`render-postgres-prices.tsv`, 24), Key Value (`render-keyvalue-prices.tsv`, 9), cron jobs (`render-cron-prices.tsv`, 11, per minute), Workflows (`render-workflow-prices.tsv`, 6) and Sandboxes (`render-sandbox-prices.tsv`, 4), plus the 10 questions of its FAQ (`render-pricing-faq.tsv`), all in `lego/backend/internal/pricing/testdata/`.
- **The test.** `TestEveryRenderPriceRowHasADisposition` (`price_table_test.go`) maps every row to one disposition, and fails on a row with none, a disposition whose row is gone, and a gap that names no board item. A tier's monthly price must equal Render's × 0.70 to the cent, so a Render price one cent away fails and names the `pricing.yaml` entry to move. A tier must also have Render's RAM, a datastore Render's connection limit, and the tier catalog's aliases must resolve Render's plan ID to it. `TestEveryPricedTierAnswersARenderRow` fails on a tier the sheet prices that no Render row names.

| Disposition | Rows | Where bex records it |
| --- | --- | --- |
| Tier at Render × 0.70 | Hobby, Pro, Scale (plans) | `pricing.yaml` `workspace` |
| Tier at Render × 0.70 | `free`, `0.5c-512mb`, `1c-2g`, `2c-4g`, `4c-8g`, `4c-16g`, `8c-32g` (compute) | `pricing.yaml` `compute` free, starter, standard, pro, pro-plus, pro-max, pro-ultra |
| Tier at Render × 0.70 | `free`, `0.1c-256mb`, `0.5c-1g`, `1c-2g`, `1c-4g`, `2c-4g` (Postgres) | `pricing.yaml` `postgres` free, basic-256mb, basic-1gb, 1c-2g, pro-4gb, basic-4gb |
| Tier at Render × 0.70 | all 9 (Key Value) | `pricing.yaml` `keyvalue` free, starter, standard, 3g, pro, pro_plus, pro_max, pro_ultra, 50g |
| Tier, per minute of run time | `0.5c-512mb`, `1c-2g`, `2c-4g`, `4c-8g`, `4c-16g`, `8c-32g` (cron) | the same `pricing.yaml` `compute` tiers, billed per second while a run executes (§7) |
| Rate at Render × 0.70 | all 4 (Sandboxes) | `pricing.yaml` `sandboxUsage` ($0.084 per CPU-hour, $0.0133 per GiB-hour, §17) |
| Open gap | `2c-8g`, `2c-16g`, `8c-16g`, `12c-24g` (compute and cron) | w6/blocked/m332 (cx53 capacity) |
| Open gap | `4c-32g`, `8c-64g`, `12c-48g`, `12c-96g` (compute and cron) | w6/blocked/114 (dedicated-vCPU limit) |
| Open gap | the 18 plans from `2c-8g` to `128c-1024g` (Postgres) | w6/192 |
| Non-goal | all 6 (Workflows) | DO_NOT_DO: Workflows & tasks; ADR018 Workflows & tasks |
| Commercial | Enterprise (plans); the migration-services and support questions (FAQ) | priced by contract, no catalog fee; support |
| Rule | the other 8 FAQ questions | per question in `price_table_test.go`: the plan fee waiver is §19 (bex charges no plan fee yet), proration is per second, the $1 card check has no counterpart (a setup-mode SetupIntent saves the card with no charge, ADR040) |

- **Key Value connection limits.** Render's pages disagree on two plans: render.com/pricing lists `256mb` at 200 connections and `1g` at 250, and [compute-plans](https://render.com/docs/compute-plans) (both read 2026-11-26) lists 250 and 1,000. bex enforces compute-plans' figures (`connlimit.Valkey`, w6/m456). That page is Render's plan reference, listing each plan ID with its legacy name and limit. Its figures follow the one connection per megabyte that every larger plan has on both pages. The higher limit also never refuses a client Render would admit. `keyValueConnectionDisagreements` in the test fails when the pricing page changes either figure, so the choice is re-read then. This is a recommendation under [Render parity first](ADR008-vision.md#render-parity-first), not a user decision.
- **A cron minute.** Each row of Render's cron table is the same plan's monthly compute price over a 30-day month (43,200 minutes), shown to five decimals: $7 gives $0.00016, $85 gives $0.00197. bex bills a cron run per second at the plan's compute rate, which `pricing.yaml` derives over 730 hours. So a bex cron minute is Render's × 0.70 × 720/730, 1.4% under this ADR's promise. The test holds that relation for each cron tier, so a cron price Render moves apart from its compute price fails. Pricing a cron minute at Render's own rate is w4/752.
- **Sandbox prices are on the page.** render.com/pricing § Sandboxes now lists $0.12 per active CPU hour and $0.038 an hour for each 2 GB of provisioned memory, the rates §17 took from Render's comparison article, so the guard holds `sandboxUsage` to them.
- **The weekly job.** The `render-pricing` job in `.github/workflows/render-schema-drift.yml` runs `scripts/render-pricing-drift.sh`. `scripts/lib/render_pricing_page.py` reads render.com/pricing.md and platform-features-by-plan.md into the fixtures' form, and fails, naming the table, on a section, table or column it does not recognise. `TestRenderPricingDrift` then compares the rows with all ten fixtures (the two plan tables above, the seven price tables and the FAQ). It fails on any changed, added or removed row, naming the disposition it moves, and holds each live row to its disposition's check, which names the price a moved tier needs. `scripts/render-pricing-drift.sh parse <dir>` writes the live rows in the fixtures' form for a re-capture.

### 10. Free web services share 750 instance hours per workspace per month (w8/m139)

[Render's Free plan](https://render.com/docs/free) grants each workspace 750 Free instance hours per calendar month, consumed only while a Free web service runs (spun-down time is free); once spent, "Render suspends all of your Free web services until the start of the next month". bex matches it:

- **Accounting.** Hours are month-to-date `instance_seconds` rows of tier `free` for service resources, from the running-pod meter (so hibernated time does not count). Postgres, Key Value, static sites and paid tiers do not count. The meter rolls up hourly, so enforcement lags by up to an hour plus the sweep interval.
- **Enforcement.** `freehours.Sweeper` (every five minutes, replica-safe) suspends every Free web service of a workspace at 750 h. The suspension is the App's `spec.suspended` with the `app.bex.co/free-hours-suspended: <YYYY-MM>` annotation, and the service reports the `billing` suspender. A service its owner suspended is never touched. Manual resume is refused (`FREE_INSTANCE_HOURS_EXHAUSTED`, 400) that month while the service is still free; moving it to a paid plan lifts the suspension immediately, and the sweeper lifts all of them at the start of the next UTC month. **Free means the plan the service runs (w6/m609, 2026-11-23).** The sweeper and the resume check read `App.RunningTier()` (`status.servedTier`), not the plan saved for the next deploy, since a REST or MCP plan change waits for a deploy. A paid plan saved on a running Free service neither exempts it nor resumes it once suspended: the resume refusal names the deploy that applies the saved plan, and that deploy lifts the suspension. The dashboard's Compute page deploys a plan change at once, so it lifts the suspension immediately as before.
- **Emails.** The workspace's billing owners get one warning at 80% and one when the hours are spent, each at most once per month (`free_hours_notices` claims the stage before sending).
- **Surfaces.** `freeInstanceHours {used, included}` on REST `GET /v1/usage`, GraphQL `usage` and MCP `get_usage`, and a "Monthly included usage" card on the dashboard Usage page.
- **Unverified.** Whether Render resumes automatically at the month start or needs a manual resume (bex resumes automatically, the parity-sufficient reading), the exact emails, and Render's `billing` suspender mapping. `BEX_FREE_INSTANCE_HOURS` overrides the allowance for dev and tests.

### 11. Pro Ultra is temporarily not sellable (w2/m250, 2026-11-02)

**Decided 2026-11-02 (user, w2/m250).** Pro Ultra (8 CPU / 32 GB) runs only on a Hetzner `ccx43` node, and the Hetzner project's dedicated vCPU limit refuses to create one ("dedicated core limit exceeded"). Until that limit is raised, `pro-ultra` carries `paused: true` in `lego/types/tiers/tiers.yaml`. A new selection is refused with 400 `INSTANCE_TYPE_UNAVAILABLE` on every create and plan-change path. Services already on it keep running, and their own plan can be re-sent unchanged. Its `pricing.yaml` price stays, so those services are still estimated and billed. The dashboard lists it disabled, and no unknown-plan hint names it. This is a temporary divergence from Render, recorded in [ADR018](ADR018-render-parity.md). **Release:** once Hetzner raises the dedicated vCPU limit, delete the `paused` field.

### 12. Key Value follows Render's 2026-10-01 ladder (w5/m266, 2026-11-05)

On 2026-10-01 Render added 3 GB and 50 GB Key Value plans and cut prices for the plans with 1 GB or more. Its current size plans are 256mb $10, 1g $20, 3g $60, 5g $100, 10g $200, 20g $350, 40g $600 and 50g $750 per month ([render.com/pricing](https://render.com/pricing#key-value), read 2026-11-05). bex sells the same ladder at Render × 0.70, under Render's legacy names where they exist:

| Plan        | bex $/mo                                           |
| ----------- | -------------------------------------------------- |
| `starter`   | $7                                                 |
| `standard`  | **$14** (was $21, priced against Render's old $30) |
| `3g`        | $42                                                |
| `pro`       | $70                                                |
| `pro_plus`  | $140                                               |
| `pro_max`   | $245                                               |
| `pro_ultra` | $420                                               |
| `50g`       | $525                                               |

Existing `standard` subscriptions move at their next billing period (user decision D2). `pro_ultra` and `50g` share Pro Ultra's temporary pause (§11, user decision D3). All Key Value tiers bill through one cost-denominated Stripe dimension ([ADR040](ADR040-billing-metronome.md), D5).

### 13. Each plan includes pipeline minutes, as on Render (decided 2026-11-10; shipped in `w6/m307`, `w6/m308`, `w6/m309`)

On hosted bex.co every workspace binds a card at sign-up (ADR075 D7), so the card-less path below runs only on self-hosted installs; [ADR018](ADR018-render-parity.md) records that divergence (w6/118).

Render's [build pipeline](https://render.com/docs/build-pipeline) (read 2026-11-10) gives each workspace Starter-tier pipeline minutes every month: Hobby 500, Pro 1,000, Scale 5,000. Every pipeline task counts: the build command, a Dockerfile image build, and the pre-deploy command. Past the allowance, Render buys more minutes automatically at $5 per 1,000 (the $0.005/min in §2). The exceptions are a workspace that has reached its pipeline spend limit (set under Workspace Settings → Build Pipeline) or has no payment method. Render then stops pipeline tasks, builds included, for the rest of the month ("Build Minutes Exhausted"), until the limit is raised or a payment method is added.

Until `w6/m308`, bex had no allowance: `estimatedCost` and the Stripe `build_seconds` meter priced every build minute, including the ones Render includes. With the per-workspace build cap gone ([ADR060 §D6](ADR060-build-worker-reliability-and-performance.md)), minutes are the only bound on usage, as on Render. bex now follows Render:

- **Allowance.** `pricing.yaml` `build.includedMinutes`: the same minutes per plan, and Enterprise takes Scale's 5,000 until a contract says otherwise. A workspace without a plan (self-hosted, the default tenant) includes none. Platform-initiated rebuilds stay exempt (w1/m184).
- **Accounting.** Month-to-date `build_seconds`, on Render's basis (`w6/m307`): pre-deploy time and failed builds count; a platform-caused failure, a wait for a build node and the layer-cache upload after a build's image push (w33/012) do not ([ADR023 § `build_seconds` against Render's pipeline minutes](ADR023-usage-metering.md)).
- **Estimate.** `EstimateIncluding` spends the allowance on the month's build rows in order and prices only the rest at $0.0035/min. Each covered charge line keeps its used quantity and reports the covered part as `included`.
- **Stripe.** The emitter sends each sealed build hour's part above the allowance. That part is computed from the month's build seconds that sort before it (`UsageBefore`: window, resource kind, service), so every replica and every retry sends the same value. An hour the allowance covers is accounted locally and never reaches Stripe. The sent part is kept as `usage_hourly.billing_export_quantity` and reported as `billedQuantity` by the billing-export report, which reconciliation compares with Stripe. A late-arriving earlier hour can only under-bill, never over-bill. The allowance is the plan in force when each hour is exported, and the first attempt freezes the value, so a retry after a plan change resends it. The estimate applies the current plan to the whole month, so after a mid-month plan change it can differ from the invoice; it is advisory (§4).
- **Exhaustion.** With Stripe billing on, the `buildminutes` sweeper (every 5 minutes) marks every App of a workspace that has spent its allowance and can pay for nothing more (no payment method, not comped, not excluded) with `app.bex.co/build-minutes-exhausted: <YYYY-MM>`. While the mark names the current UTC month, the operator starts no new build (the deploy ends `build_failed`, reason `BuildMinutesExhausted`, "Build Minutes Exhausted: …") and no pre-deploy run. Builds already running finish and running services are untouched. The mark lifts on the next sweep after a payment method is bound (within five minutes), and a new month ignores it. The estimate, the emitter and the sweeper read the allowance through one resolver (`pricing.IncludedForWorkspace`), so a workspace without a plan (the default tenant) is neither given minutes nor stopped. Without billing (self-hosted), nothing is priced and nothing stops. A workspace with a payment method buys overage up to its pipeline spend limit (below).
- **Spend limit (`w6/m309`).** As on Render (Workspace Settings → Build Pipeline → Set spend limit), a billing manager may set a monthly dollar limit on the build minutes bought beyond the included ones (`tenants.pipeline_spend_limit_cents`; no limit by default). When the month's overage reaches it, in whole cents spent, the sweeper stops the workspace exactly as for a missing payment method, and emails the `limited` notice. Raising or removing the limit lifts the stop on the next sweep. Render's public API and MCP have no spend-limit operation, so bex offers it on GraphQL (`pipelineSettings`, `setPipelineSpendLimit`) and the dashboard's Workspace Settings → Build Pipeline only.
- **Surfaces.** `buildMinutes {used, included}` next to `freeInstanceHours` (§10) on REST `GET /v1/usage`, GraphQL `usage` and MCP `get_usage`, and a "Pipeline minutes" card in the billing page's included usage. Whether the builds are stopped is `pipelineMinutesExceeded` on the workspace's limits read (REST `GET /v1/owners/{ownerId}/limits`, GraphQL `workspaceLimits`, MCP `get_workspace_limits`; w3/108): true while one of its Apps, not being deleted, carries the current month's mark. With it the dashboard disables Manual Build, Manual Deploy and a build source's Trigger Build before the click, with Render's "You’ve used up the maximum number of pipeline minutes specified by your spend limit. Learn more.", as Render's dashboard does. Billing owners get one email at 80% and one at exhaustion each month (`build_minutes_notices` claims the stage first), worded for whether the workspace can pay.
- **Unverified.** Render's rounding of partial minutes, whether failed or canceled builds consume minutes, and its exact emails. bex counts whole seconds, rounds once per month, and counts every build that ran, failed ones included: the parity-sufficient reading of "all pipeline tasks consume pipeline minutes".

### 14. Each plan includes outbound bandwidth, as on Render (decided 2026-11-10; shipped in `w6/m319`)

On hosted bex.co every workspace binds a card at sign-up (ADR075 D7), so the card-less path below runs only on self-hosted installs; [ADR018](ADR018-render-parity.md) records that divergence (w6/118).

Render's [outbound bandwidth](https://render.com/docs/outbound-bandwidth) (read 2026-11-10) is included per workspace plan: Hobby 5 GB, Pro 25 GB, Scale 1 TB, Enterprise custom. Beyond it, public-internet bandwidth costs $0.15/GB (§2). "If you haven't added a payment method and you exceed your included bandwidth, Render spins down your workspace's services until the start of the next month."

Until `w6/m319`, bex priced every egress byte from the first. bex now follows Render, on the same machinery as §13:

- **Allowance.** `pricing.yaml` `bandwidth.includedGB`: Render's figures per plan; Enterprise takes Scale's 1,000 until a contract says otherwise. The GB is the one bandwidth is billed and shown in, 2^30 bytes (the Stripe `egress_gib` meter and the estimate's "GB" line). That includes slightly more than Render's decimal GB, never less, and keeps the allowance, the charge line and the invoice in one unit. A workspace without a plan includes none. bex has no private-link traffic, so the allowance covers the one public `egress_bytes` meter.
- **Estimate and Stripe.** `EstimateIncluding` and the emitter apply a per-meter allowance (`pricing.Included`: `build_seconds`, `egress_bytes`). Egress hours follow §13's Stripe rule: the part above the allowance, from the month's earlier egress in a fixed order (`UsageBefore`), frozen with the first attempt, and an hour the allowance covers never reaches Stripe.
- **Spin-down.** With Stripe billing on, the `bandwidth` sweeper (every 5 minutes) suspends every service of a workspace that has spent its allowance and cannot pay (no payment method, not comped, not excluded). It uses the Free-hours suspension path (§10): `app.bex.co/bandwidth-suspended: <YYYY-MM>`, Render's `billing` suspender, and services their owner suspended are left alone. A manual resume that month is refused with `OUTBOUND_BANDWIDTH_EXHAUSTED` until a payment method is bound; then it succeeds, and the sweeper resumes the rest within five minutes. A new month resumes them all. Managed datastores keep running: Render's sentence names services. A workspace whose plan cannot be read keeps its state until the read recovers.
- **Surfaces.** `includedBandwidth {used, included}` (GB) next to `buildMinutes` on REST `GET /v1/usage`, GraphQL `usage` and MCP `get_usage`, and an "Outbound bandwidth" card in the billing page's included usage. Billing owners get one email at 80% and one at exhaustion each month (`bandwidth_notices`), worded for whether the workspace can pay.
- **Unverified.** Render's exact emails and how fast it spins services down. bex sweeps every five minutes, so a card-less workspace can overrun its allowance by up to five minutes of traffic, which is never billed.
- **User decisions (2026-11-10).** Three choices `w6/m319` shipped as recommendations were approved: the allowance in 2^30-byte GB; datastores keep running past the allowance; the sweeper runs only where the payment-method gate (`BEX_REQUIRE_PAYMENT_METHOD`) is on, since the gate refuses the manual resume. On bex's hosted service the gate is `all` (ADR075 D7: every workspace binds a card before any usage), so a card-less workspace cannot run services there and the spin-down binds self-hosted installs that run the gate in `1` mode.

### 15. A suspended sandbox pays snapshot storage, not compute (decided 2026-11-10, [ADR104](ADR104-sandbox-render-parity.md) D8; storage shipped in `w22/m313`)

Render bills a running sandbox for used CPU plus allocated memory and a suspended one for its snapshot storage only. Render's launch post and its comparison article (render.com/articles/render-sandboxes-vs-vercel-sandboxes, published 2026-11-06) give the model and the rate. Since 2026-11-26 render.com/pricing § Sandboxes lists the same rates and snapshot storage at $0.08 per GB-month, and the guard in §9 holds the sheet to them. bex adopts the model in two steps:

- **Storage, now (`w22/m313`).** `sandbox_snapshot_gb_seconds` is decimal GB × seconds a suspended sandbox's retained image is kept, at **$0.056 per GB-month** (Render's $0.08 × 0.70, `pricing.yaml` `sandboxSnapshot`, 730-hour month), re-based to GB-hours for Stripe (`sandbox_snapshot_gb_hours`). A workspace whose only sandbox sat suspended for a month with a 10 GB image previews $0.56 and no compute. The bytes are the compressed OCI manifest size read from the registry, and what is double-counted is stated in [ADR023 § Sandbox snapshot storage](ADR023-usage-metering.md#sandbox-snapshot-storage-w22m313-adr104-d8). The Price exists on the live Stripe account (created 2026-11-10 by `scripts/stripe-billing-setup.py --live`) and the meter is not held: a new Stripe account must run that script before bex-api starts against it. Agent-session sandboxes are not metered for storage (their cost is the session's, ADR047 D6).
- **Memory, recorded, not billed until the switch (`w22/m313`).** `sandbox_memory_gb_seconds` (allocated GiB × running seconds) was recorded beside the weighted `sandbox_compute_seconds` charge, which already includes memory at the AgentCore reference ratio, and shown on the usage surfaces at a rate of $0. Billing switched to memory plus used CPU in one step with the used-CPU meter (`w22/m314`, §17).
- **Hobby.** Whether Hobby workspaces pay for sandbox usage is still open ([ADR104](ADR104-sandbox-render-parity.md) D8): `pricing.yaml` keeps Hobby plans free, and Render's pages do not say.

### 16. Included custom domains: 2, 15 and 25 by plan, then metered, as on Render (w7/m429 t004)

Render's plan table (render.com/pricing, read 2026-11-16) has the row "Custom domains: 2 included then $0.25/domain/month" on Hobby, 15 on Pro, 25 on Scale and "Custom" on Enterprise. [render.com/docs/custom-domains](https://render.com/docs/custom-domains) (read 2026-11-16): "Your workspace includes an amount of custom domains based on its plan" and "Each custom domain you add beyond this amount costs $0.25 per month." Render charges past the count; it does not refuse. Until w7/m429 bex had no included count and no price. bex now follows Render:

- **Included count.** One table, `core.IncludedCustomDomains`: Hobby 2, Pro 15, Scale 25. Enterprise includes every domain until a contract says otherwise. As for every allowance (§13), a workspace without a plan (the default tenant) is not metered.
- **What counts.** One counted domain is one verified `domains` row that bex did not generate:
  - _Verified only._ An unverified claim never routes and is reclaimed after 48 hours (ADR005), so it is not counted.
  - _A `www`/apex pair counts once._ bex stores the pair as two rows, and the half bex adds (Render adds it automatically, `redirect_for_name` set) is not counted. A sibling the customer adds explicitly loses its redirect and counts as its own domain.
  - _A wildcard counts as one domain._ `*.example.com` is one row.
  - _Suspended services count_: their domains stay configured. The service's own `<slug>.onbex.co` host is never a claim, so it never counts.
- **Meter.** `custom_domain_seconds` ([ADR023 § Custom domains above the included count](ADR023-usage-metering.md#custom-domains-above-the-included-count-w7m429)): each hour, one row per workspace with the domains above its count × the seconds each was verified in that hour. At any moment the domains above the count are the most recently verified ones. Only those are written, so every unit is charged. The hour uses the plan in force when it is metered.
- **Price.** **$0.175 per domain-month** (Render's $0.25 × 0.70, §2), prorated by the second over the 730-hour month: `pricing.yaml` `customDomains.usdPerDomainSecond`. A Hobby workspace with three verified domains all month previews $0.18. The section stays out of the public `GET /pricing` sheet until bex.co quotes it.
- **Never refused for the count.** Only the abuse caps refuse a domain (`BEX_MAX_CUSTOM_DOMAINS_PER_SERVICE` 100 and `_PER_WORKSPACE` 500, `CUSTOM_DOMAIN_LIMIT`). The included count gates nothing.
- **Surfaces.** For the current month, REST `GET /v1/usage`, GraphQL `usage` and MCP `get_usage` carry `customDomains {used, included, unlimited}` (`unlimited` on Enterprise). It is omitted for a past month, because the count is the present inventory, and where no plan applies. `estimatedCost` carries the `custom_domain_seconds` line under resource kind `custom_domains` (one per workspace, keyed by its id, named "Custom domains"), in domain-months at $0.175. The Billing page's included usage shows a "Custom domains" card ("3 of 2 included custom domains in use"), and the charge tree shows the line.
- **Stripe.** The `custom_domain_hours` meter carries domain-seconds ÷ 3,600 at 0.023972602740¢ per domain-hour. The live Price does not exist yet: the user creates it with `python3 scripts/stripe-billing-setup.py --live` (the agent may not write live Stripe, `w2/blocked/070`). Until then bex-api holds the meter, as it held the agent model meter (ADR091 D3): it is on no Subscription and its rows stay pending in the outbox. No card is charged for domains, though the estimate shows them. After the live run, `BEX_STRIPE_CUSTOM_DOMAIN_METER` goes to `"1"` in `lego/operator/config/api/deployment.yaml`. Held rows inside the 34-day backfill horizon then ship, and `EnsureContract` adds the item to existing Subscriptions. The meter brings the catalog to 20 dimensions, Stripe's maximum per Subscription: one more needs a contract redesign (`TestCatalogFitsOneSubscription`).
- **Decision state.** This charge is a recommendation resting on ADR008's Render parity first, not a recorded user decision. Keeping extra domains free would be a divergence that needs the user's sign-off in ADR018.
- **Unverified against Render.**
  - Whether the automatically added `www`/apex sibling counts. Render's docs say "each custom domain you add".
  - Whether an unverified domain counts.
  - How a wildcard counts. Render's plan-table data carries a hidden "$0.25 per domain per month" in the check cells of its "Wildcard domains" row on every plan, which the page does not normally show. Whether Render bills wildcards outside the included count is unknown; bex counts a wildcard as one domain toward it.
  - What a plan downgrade does to domains already above the new count. bex keeps them and charges them, by ADR030 §9's rule that existing state is never removed.
  - How Render prorates within a month. bex prorates by the second, like its other monthly prices.

### 17. A running sandbox pays for used CPU and allocated memory, as on Render (effective 2026-11-18T00:00:00Z; [ADR104](ADR104-sandbox-render-parity.md) D8, shipped in `w22/m314`)

From **2026-11-18T00:00:00Z**, in every workspace and with no allowlist, a running sandbox is billed as Render bills one:

| Meter | Render | bex (× 0.70) | `pricing.yaml` |
| --- | --- | --- | --- |
| `sandbox_cpu_seconds`, CPU actually used | $0.12 per active CPU hour | **$0.084 per CPU-hour** | `sandboxUsage.usdPerCPUSecond` |
| `sandbox_memory_gb_seconds`, memory allocated while running | $0.038 per hour per 2 GB, about $0.019 per GB-hour | **$0.0133 per GiB-hour** | `sandboxUsage.usdPerMemoryGBSecond` |

- **Source and date of the read.** Render's launch post (render.com/blog/introducing-render-sandboxes, published 2026-11-04) gives the model: "billed for actively-used CPU (not simply provisioned CPU)" and for "memory allocated to it", and nothing for CPU or memory while suspended. Its comparison article (render.com/articles/render-sandboxes-vs-vercel-sandboxes, published 2026-11-06) gives the rates. **Both were re-read on 2026-11-17** and say what they said on 2026-11-10. **render.com/pricing lists the same rates since 2026-11-26** (§ Sandboxes, $0.12 per active CPU hour and $0.038 an hour per 2 GB), and `price_table_test.go` holds `sandboxUsage` to them (§9).
- **What a sandbox costs.** One that ran for an hour with 2 GiB and used 600 CPU-seconds previews $0.014 of CPU and $0.027 of memory. An idle running hour of the same sandbox previews its memory, $0.027, and about no CPU (0.45% of a core measured, $0.0004). The weighted charge it replaces was $0.108 for that hour whatever the sandbox did.
- **The measured CPU is billed, with no subtraction.** The figure is the pod's cgroup counter, which under gVisor includes the sentry: +9.8% over what a pure CPU load itself used (w22/m314 t001). At 0.70 of Render's rate a unit of real work still costs about 23% less than on Render ($0.084 × 1.098 ≈ $0.092 against $0.12). A syscall-heavy load reads higher, up to the pod's CPU quota, and **that quota is the plan's CPU** (w22/020, ADR104 D8): on `1c-2g` the worst hour is $0.084 + $0.027 = $0.111, against $0.108 before. Until w22/020 the quota was the larger of the plan's CPU and 2 cores, because the pod's init container took the namespace's default limit, and the same hour could cost $0.195; a pod that was already running when the fix deployed keeps that quota until it is next started. A variant that divides by an overhead factor is a separate follow-up (w22/019). **Measured against runc on the same node at the plan's quota (2026-11-24, ADR104 D8):** a CPU-bound load costs 1.02–1.05× the CPU per unit of work, about $0.086–0.088 per CPU-hour of work against Render's $0.12. A one-byte `dd` loop costs 15.5×, about $1.30. Whether to divide is the user's choice, still open in w22/019.
- **The switch.** `sandboxUsage.billedFrom` is one instant on a UTC hour boundary. Sandbox usage before it stays on `sandbox_compute_seconds` and is not re-priced; the CPU and memory recorded before it are shown as unbilled lines. `estimatedCost.sandboxUsageBilledFrom` on REST, GraphQL and MCP carries the instant whenever a period has sandbox usage, beside `estimatedCost.agentSandboxUsageBilledFrom` (below), and the dashboard prints both under the charge tree.
- **Stripe.** No new Price: the catalog is at Stripe's 20 items per Subscription, so billed CPU and memory are exported on the existing `sandbox_compute_seconds` meter as the weighted units that cost the same ([ADR023 § Sandbox CPU billed on use](ADR023-usage-metering.md#sandbox-cpu-billed-on-use-w22m314-adr104-d8)). The invoice shows one sandbox compute line; the usage page shows CPU and memory apart.
- **Agent sessions, from 2026-11-26T00:00:00Z (`w6/m566`, the user's approval of 2026-11-22).** An agent session's sandbox kept `sandbox_compute_seconds` until `sandboxUsage.agentBilledFrom`, **2026-11-26T00:00:00Z**, and from that instant it is billed the same way, in every workspace and with no allowlist.
  - **What it costs.** An idle running hour of an `agent-4c-8g` sandbox (4 CPU, 8 GiB) previews $0.1064 of memory and almost no CPU, against $0.434 on the weighted meter. A busy hour adds the CPU it used, at most 4 CPU-hours ($0.336).
  - **Before the instant.** Its usage stays on the weighted meter and is not re-priced. Its CPU and memory, recorded since `w6/m566` t001 (2026-11-23), are unbilled lines.
  - **The session's cost.** `outcome.cost.computeUSD` prices the session's sandboxes the same way: weighted compute before the instant, used CPU and allocated memory from it, to the micro-dollar of their rows (ADR094 D7).
  - **The instant on the API.** `estimatedCost.agentSandboxUsageBilledFrom` carries it on REST, GraphQL and MCP.
  - **Reference.** Render bills a running sandbox this way (above). Devin bills a session's awake time and lets it sleep after 5 minutes; since `w6/m563` an agent sandbox closes 5 minutes after its turn, so the two models give nearly the same idle charge.
- **Not on bex.co yet.** `sandboxUsage` is withheld from `GET /pricing` until bex.co quotes it, like `customDomains`.
- **Hobby** stays open, as in §15.

### 18. Each cron job has a monthly minimum, as on Render (w9/m448, 2026-11-16)

[render.com/docs/cronjobs](https://render.com/docs/cronjobs) (read 2026-11-16): "Billing is prorated by the second, based on active running time during a given month." and "There is a minimum monthly charge of $1 per cron job service." bex now charges the same floor:

- **Amount.** **$0.70 per cron job per calendar month**: Render's $1 × 0.70 under §2, as every Render-derived price on this sheet is, flat fees included (disk, snapshot storage, custom domains). A cron job's bex charge is then Render's charge for the same runs × 0.70, the minimum included, but for the month a cron minute is priced over: Render's is 30 days, bex's 730 hours, so bex's run charge is 1.4% lower (§9, w4/752). `pricing.yaml` `cronJob.minimumUsdPerMonth`.
- **Rule.** A cron job active in a month whose runs there cost less than the minimum is charged the difference: max(0, $0.70 − its run charge), where the run charge is its `instance_seconds` in the month at each tier's rate, in exact micro-USD. A cron job whose runs cost $0.70 or more owes nothing extra.
- **Who owes it.** Every cron job the hourly sweep sees active (created and not suspended) at any point in the month owes it once. The sweep records each one (`cron_minimum_months`), because a deleted cron job's row is gone when its month is charged.
  - _Deleted mid-month:_ it owes the full minimum, unprorated.
  - _Created mid-month:_ the same.
  - _Suspended all month:_ it owes nothing. Render staff say suspending a service stops its billing.
  - _The default tenant_ (no plan) is not charged, as for every plan charge.
- **Period.** The calendar month (UTC), Render's "given month" and the month bex's included allowances use (§13, §14). It is not the Stripe subscription period.
- **When.** Once the month's run time is final: 49 hours after the month ends, past the usage rollup's 48-hour rewrite window. Each top-up is one `cron_minimum_microusd` usage row under the cron job (resource kind `service`, window = the month's last hour). Its own meter cursor records the month, and a row is never rewritten. A second pass, a second replica or a manual `SettleWorkspace` charges nothing more. Exclusions follow the outbox: a `billing_excluded` workspace is never exported, and a card-less one waits for its card.
- **Stripe.** The minimum is **not a meter**. The catalog already has 20 dimensions, Stripe's maximum per Subscription (§16). The emitter therefore sends each row as one pending invoice item on the workspace's live Subscription, described "Cron job minimum: <cron job name>, <Month YYYY>", in cents rounded half up. The row's deterministic identifier is the idempotency key. Settle before delete awaits no meter for it. It needs no new Price and no live-Stripe step: the item lands on the Subscription's next invoice.
- **Surfaces.** REST `GET /v1/usage`, GraphQL `usage` and MCP `get_usage` carry the row as a `cron_minimum_microusd` line of `estimatedCost` under the cron job, priced at $0.000001 a unit, once the month is charged. The Billing page's charge tree labels it "Cron job minimum" with the rate "$0.70/month minimum". The New Cron Job form, a cron job's Plan page and the plan-change confirm say a cron job is billed per second while a run executes, with a $0.70 monthly minimum, in all eight dashboard languages. The current month's estimate does not project the minimum before the month ends.
- **Decision state.** This is a recommendation under ADR008's Render parity first, not a recorded user decision. Two variants would need the user:
  - _No floor at all_ would be a divergence, and it needs the user's sign-off in ADR018.
  - _Charging Render's $1 unscaled_ would make this the one Render price §2 does not discount. Changing it is a one-line `pricing.yaml` edit.
- **Unverified against Render.** Render publishes nothing on four points, and bex takes the literal reading of each:
  - whether the minimum is prorated for a cron job created or deleted mid-month (bex: no);
  - whether a suspended cron job owes it (bex: not if it was suspended the whole month);
  - what the invoice line is called (bex: "Cron job minimum");
  - whether it counts against any included usage (bex: no; no plan includes compute).

### 19. A paid workspace with no services and no activity in a month pays no plan fee for it, as on Render (w4/m689, 2026-11-26)

Render's pricing FAQ (render.com/pricing, read 2026-11-26): "If a workspace has no services (live or suspended) and no activity during a month, the workspace subscription fee for that month will be waived." No Render docs page states the rule (`render.com/docs/llms.txt` has no "waive", read 2026-11-26). Render defines neither a service nor activity for it, so the definitions below are bex's choices where Render publishes nothing.

- **Today there is no fee to waive.** §2's plan fees are listed (the plan picker, `GET /pricing`) but never charged. A workspace's Subscription carries metered Prices only (`internal/billing/contracts.go` refuses any other), and no item attaches the licensed plan-fee SKU. So every Pro and Scale workspace pays no plan fee today, empty or not, and an empty one already pays what Render's rule leaves it to pay. bex writes no waiver line while no fee is charged: a negative line on an invoice that carries no fee would discount the workspace's usage, which is a credit, not a waiver. The rest of this section is the rule the plan-fee SKU ships with. Its code, surfaces and Stripe check are w4/m689 t002–t004, parked on the user's decision to charge the fee.
- **Scope.** Pro and Scale, and Enterprise where its contract carries a catalog fee. Hobby has no fee. A workspace in an organization pays no fee of its own (§2), so an organization's one fee is waived only when every workspace in it qualifies, the primary included.
- **A service.** A web service, private service, background worker, cron job, static site, Postgres or Key Value that existed at any moment of the month, in any state: live, suspended, failed or still deploying. One created and deleted inside the month counts. Sandboxes, object storage buckets and environment groups are not services, though creating or changing one is activity.
- **Activity.** An `allowed` event in the workspace's audit log (`audit_events`, one row per authorized write, [ADR006](ADR006-bex-api.md) § Audit log) whose caller is one of the workspace's members, by any sign-in or token of theirs, or one of its API keys, other than a sign-in or sign-out (`auth.Login`, `auth.Logout`). Reads are never logged, billing-page reads included, so they never count. A denied attempt changed nothing and does not count. bex's own actions are not the workspace's activity: the billing worker, the operator, a bex admin's comp or exclusion.
- **Month.** The calendar month (UTC), the period §13, §14 and §18 use. It is not the Stripe subscription period.
- **Which invoice carries it.** Stripe bills a licensed recurring Price in advance, at the start of its period, and bex's metered usage in arrears. A month's fee is therefore already invoiced when the month ends. Its waiver is one negative pending invoice item on the workspace's (or the organization primary's) live Subscription, the mechanism §18 uses, and it lands on the next invoice, the one that bills that month's usage. It is decided 49 hours after the month ends, as §18 is. The line reads "<Plan> plan fee waived for <Month YYYY>: no services or activity" and equals the fee billed for that month, prorated as it was billed. It is idempotent per workspace (or organization) and month. A workspace whose fee was comped or not billed gets none. The plan-fee SKU aligns its charge with the calendar month or amends this paragraph. The waiver is never an [ADR071](ADR071-tenant-billing-credits.md) credit (`.pm/DO_NOT_DO.md`).
- **Surfaces.** Once a fee is charged, REST `GET /v1/usage`, GraphQL `usage`, MCP `get_usage` and the Billing page show a waived month with its plan and amount, from one core read. The field is absent where the workspace owes no fee.
- **Decision state.** A recommendation under ADR008's Render parity first, not a recorded user decision, and recorded unsigned in [ADR018 § Billing & invoicing](ADR018/billing-invoicing.md). Charging the listed plan fee at all is the user's decision. Until it is made, bex charges no workspace a fee, which is itself a difference from Render that has no sign-off.
- **Unverified against Render.**
  - Which resources count as services. bex counts its datastores, which Render lists beside its services.
  - What counts as activity. bex counts every member or API-key write and no read.
  - Whether an organization's fee follows the same rule, and whether it needs every workspace to qualify.
  - Whether a partial month's prorated fee is waived as billed, and what the invoice line is called.

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
