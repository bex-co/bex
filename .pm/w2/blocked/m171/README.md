# w2 · m171 — Postgres plans large enough for high availability

**Worker:** worker2 **Goal:** a tenant can create or upgrade a managed Postgres on a ≥1-CPU plan, enable high availability, and survive a primary failure with Render's `postgres_cluster_leader_changed` event — on REST, GraphQL, MCP, the pinned Render CLI and the dashboard. **Status:** blocked on the live Stripe catalog — t001, t002, t004–t010 done; t003/t012 code shipped, their live Stripe steps need the Stripe admin; t011 Closeout after that

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Fetch Render's live Postgres catalog and record which ≥1-CPU tiers bex adds, at Render × 0.70 — **DONE** | 40m | — |
| t002 | Capacity and placement check: an HA pair of each chosen tier on production node types — **DONE** | 40m | t001 |
| t003 | Add the tiers to the catalog, pricing, Stripe catalog and schema enums, with Render's plan-change rules for HA | 1h | t002 |
| t004 | Emit `postgres_cluster_leader_changed` when the primary changes — **DONE** | 1h | t003, w5/m138/t003 |
| t005 | Dashboard: the new tiers in every plan picker, HA available on them, skeletons unchanged in geometry — **DONE** | 45m | t003 |
| t006 | Live HA acceptance on `dev-2`: create, fail the primary, observe failover and the event — **DONE** | 1h | t004, t005 |
| t007 | Correct ADR018, ADR009 §5, ADR030 and the CLI checklist — **DONE** | 20m | t006 |
| t012 | Correct the `basic-256mb` / `basic-1gb` prices to Render × 0.70 ($4.20 / $13.30), moving existing subscriptions at their next period (absorbs `w2/047`) | 30m | t003 |
| t008 | Render parity — **DONE** | 30m | t007, t012 |
| t009 | Simplify — **DONE** | 15m | t008 |
| t010 | Test coverage — **DONE** | 45m | t008 |
| t011 | Closeout | 10m | t009, t010 |

## Definition of done

- `bex postgres create --plan <new tier> --high-availability` (pinned Render CLI) succeeds, as do the REST (`enableHighAvailability`), GraphQL and MCP equivalents and the dashboard HA toggle; the database runs two instances on distinct nodes.
- Upgrading `basic-1gb` → a new tier and then enabling HA works on every surface; a change that would leave HA on a <1-CPU plan is handled exactly as Render handles it (recorded in t001), on every surface.
- Deleting the primary pod (and, separately, `POST /v1/postgres/{id}/failover`) promotes the standby, the internal connection URL keeps working, and one `postgres_cluster_leader_changed` event appears in the events feed and is deliverable to a subscribed webhook.
- The usage estimate prices the new tiers at Render × 0.70 (ADR030), and the Stripe catalog script knows them.
- ADR018's HA row no longer claims a capability no plan can reach; ADR009 §5 and the CLI checklist match what shipped.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w2` (2026-10-19), item 4, chosen by the user the same day (parity-ledger sweep). `w8/done/m43` shipped the HA plan gate and called a larger plan "deferred product work" but filed nothing; `w5/m138` (read replicas, in progress) explicitly leaves `postgres_cluster_leader_changed` out "because HA needs a ≥1-CPU plan and none ships yet (ADR009 §5's larger plans are an open pricing decision)" — this milestone owns that decision and that event.
- **Evidence:** the largest Postgres tier is `basic-1gb` (500m CPU, 1Gi, 5 GB) in `lego/types/tiers/tiers.yaml:78-104`; `CheckHighAvailabilityPlan` (`lego/backend/internal/postgres/tiers.go:115-140`) refuses HA below 1 CPU, and `PlanSupportsHighAvailability` is a variable "only so tests … can run while the shipped catalog has no ≥1-CPU Postgres plan". Render's pinned event list (`lego/backend/internal/api/openapi/render-public-api-1.json`) includes `postgres_cluster_leader_changed`; bex's event vocabulary does not. The operator already projects CNPG `status.currentPrimary` into `Database.status.CurrentPrimary` and `LastFailoverAt` (`lego/operator/internal/controller/database_controller.go:1503-1577`).
- **Goal linkage:** [ADR008](../../../../docs/ADR008-vision.md) pillar 1 (Render-compatible API) and [ADR009](../../../../docs/ADR009-postgresql-management.md) managed Postgres — its §5 maps Render's Pro tier to "availability + PITR".
- **Expected outcome:** a team on Render can move a production database that needs failover or more than 1 GiB of RAM; the HA and failover code bex already ships becomes reachable.
- **Why now:** shipped capability (HA gate, failover verb, HA panel) is unreachable behind a ledger row that marks it ✅, and `w5/m138` is extending the same Postgres plan family right now — landing the plan decision while that context is fresh avoids a second pass over admission and events.
- **Render parity included:** new plans and HA availability appear on REST, GraphQL, MCP, the CLI and the dashboard.
- **Coordination:** t004 waits for `w5/m138/t003` (which edits the same Postgres event vocabulary) to avoid conflicting changes; if w5/m138 stalls, coordinate rather than fork the vocabulary.

## Decisions

**t001 (2026-10-19 research; approved by the user 2026-10-20: "A — stay consistent with Render").** Add exactly the three tiers below at Render × 0.70; wherever Render's behavior is documented or observable, bex follows it rather than inventing its own rule.

Render primary sources, fetched 2026-10-19: [high availability](https://render.com/docs/postgresql-high-availability) — "Use a compute plan with at least 1 CPU"; "Your standby instance always has the same compute plan and storage as your primary instance and is billed accordingly"; failover "takes just a few seconds", connections are terminated and the new primary is reachable "at the exact same database URL". [Compute plans](https://render.com/docs/compute-plans) maps flexible ids to legacy names; prices from [render.com/pricing](https://render.com/pricing) (page data). The pinned OpenAPI enum (`lego/backend/internal/api/openapi/render-public-api-1.json`) already lists every id below.

| Render id (legacy) | CPU / RAM | Render $/mo | bex at × 0.70 (ADR030) | HA eligible |
| --- | --- | --- | --- | --- |
| `0.1c-256mb` (`basic-256mb`) | 0.1 / 256 MB | 6 | 4.20 | no |
| `0.5c-1g` (`basic-1gb`) | 0.5 / 1 GB | 19 | 13.30 | no |
| **`1c-2g`** (—) | 1 / 2 GB | 40 | **28.00** | yes |
| **`1c-4g`** (`pro-4gb`) | 1 / 4 GB | 55 | **38.50** | yes |
| **`2c-4g`** (`basic-4gb`) | 2 / 4 GB | 75 | **52.50** | yes |

- **Recommended tiers:** add `1c-2g` (the smallest HA-eligible plan), `pro-4gb` (alias `1c-4g`, `pro_4gb`) and `basic-4gb` (alias `2c-4g`, `basic_4gb`), keeping the catalog's legacy-name-as-id convention (`lego/types/tiers/aliases.go`); `1c-2g` has no legacy name and keeps its flexible id.
- **HA billing:** like Render, the standby costs the same as the primary — an HA database meters two instances.
- **Downgrade rule:** Render's documentation is silent, so t003 first determines Render's actual behavior (pinned OpenAPI / CLI / a live check where possible) and mirrors it; only if it cannot be determined, fall back to refusing a plan change to a <1-CPU plan while HA stays enabled, with the existing `POSTGRES_HA_PLAN_UNSUPPORTED` code, unless the same request disables HA (symmetric with the enable rule; nothing is silently turned off).
- **Capacity (read-only, 2026-10-19):** tenant pool = 3 × cx33 (4 vCPU, ~7.5 Gi allocatable) + 1 × cx43 (8 vCPU, ~15 Gi); each recommended tier's HA pair can be placed on distinct nodes. t002 confirms headroom and autoscaler behavior.
- **Side finding (filed separately as `w2/047`):** `pricing.yaml` records Render's small tiers at $7 and $20, but Render lists $6 and $19, so `basic-256mb` and `basic-1gb` are priced above bex's own Render × 0.70 rule.

**047 absorbed (decision 2026-10-20).** The user delegated the call with "the end result must be correct (consistent)". Decision: the two small tiers move to Render × 0.70 of Render's current prices ($6 → **$4.20**, $19 → **$13.30**), and existing subscriptions move at their next billing period (a price cut, so no customer is harmed and bex stays consistent with its own rule). Its Stripe work rides the same single catalog change as the three new tiers: create five prices, retire two, then `scripts/stripe-billing-setup.py` must report the live catalog in sync with `pricing.yaml`. The live Stripe step needs the Stripe admin (`.pm/DO_NOT_DO.md`, ADR071 entry). Original note: [source-w2-047.md](source-w2-047.md).

## Capacity (t002, read-only 2026-10-20)

- Serving capacity is only `bex-tenant-0`: 3 × cx33 (4 vCPU / ~7.5 Gi allocatable), autoscaler **min 1 / max 3 — at max**. CPU requested 91% / 81% / 85% (free ≈ 324m / 734m / 584m). `bex-tenant-lg` (cx43, ~5.8 CPU free) and `bex-tenant-burst` carry `bex.co/build-only=true:NoSchedule` — Postgres cannot use them.
- So a single `1c-2g` instance (1 CPU, Guaranteed) is unschedulable today, and the autoscaler cannot add a serving node. The same constraint left the `tenant-headroom` placeholder Pending (`w2/048`) and tenant QA pods Pending on 2026-10-20.
- HA placement: the operator sets CNPG `enablePodAntiAffinity: true` with the default **preferred** type (`database_controller.go:476-480`), so with capacity a pair *could* co-locate on one node — not Render's guarantee. t003 should make HA anti-affinity **required**.
- **Decision needed (user, spend):** raise `bex-tenant-0` `cluster-api-autoscaler-node-group-max-size` (`infra/clusterapi/overlays/hetzner-caph/cluster.yaml`) — recommended 3 → 6 so a 2c-4g HA pair plus rollout surge fit; nodes are added (and billed) only when pods need them. Applied by the `app-cluster.yml` workflow on merge.

## Unblock 2026-10-29 — serving-pool ceiling decided; resumed

- **Affected:** `w2/m171` (t003–t012). **Source:** explicit user decision in `/unblock w2` on 2026-10-29: "5 → 8". Capacity facts verified by `/unblock` (read-only, `hetzner-prod`, main `7fd6c9c4f`).
- **What changed since t002:** `2faa86f18` (2026-10-24, a separate beancount-cms outage) already raised `bex-tenant-0` max 3 → **5**, live as `max=5`. The pool runs **4** cx33 nodes. CPU requests are 2077m / 3917m / 3767m / 3267m of 4000m, so the most free on any node is ≈1.9 CPU. At max 5, a `1c-*` HA pair fits (one existing node + one new), but a **`2c-4g` HA pair cannot**: it needs two nodes with ≥2 CPU free, so two new nodes, which means ≥6.
- **Decision:** raise `bex-tenant-0` `cluster.x-k8s.io/cluster-api-autoscaler-node-group-max-size` **5 → 8**. The user chose 8 over the recommended 6 to leave headroom for rollout surge and more than one HA customer. Nodes are added and billed only when pods need them. Not adopted: 6 (no surge margin) and leaving it at 5 (2c-4g HA would be unplaceable).
- **Remaining gates:** none for the capacity decision. Known later step: t003's **live** Stripe catalog change (create five prices, retire two). The Stripe CLI on the operator machine is authenticated to the `Stargately, Inc` account. If live-mode access turns out to be missing when the worker gets there, park only that step and ask the Stripe admin. Do not re-park the whole milestone.
- **Worker next steps:** t003 now also lands the ceiling change in `infra/clusterapi/overlays/hetzner-caph/cluster.yaml`, plus the `scripts/clusterapi-validate.sh` guard and the ADR060 text if they pin the value (follow `2faa86f18`). The `app-cluster.yml` workflow applies it on merge. Verify live with `kubectl get md -n bex-capi bex-tenant-0` showing `max=8` before t006's HA acceptance. Then continue t003 → t012 as tabled.

## Progress — 2026-10-29 (`/loopx w2`)

Code for t003 and t012 shipped in `8b09490a3`, `ce346c22b` and `dffe10a88`. Backend `go test ./...` and the real-deps suite (`ship-gates.sh`) pass, as do operator `make test`, `make lint` (0 issues, all four modules), `clusterapi-validate.sh`, `test_stripe_billing_setup.py` (15) and dashboard `yarn test`.

- **Catalog:** `1c-2g` (1 CPU / 2Gi), `pro-4gb` (alias `1c-4g`, `pro_4gb`; 1 CPU / 4Gi) and `basic-4gb` (alias `2c-4g`, `basic_4gb`; 2 CPU / 4Gi). Each has a 10 GB default volume, backups on, and one instance (HA adds the standby). Blueprint `capabilities.json`, MCP descriptions, the CRD enum and the docs plan lists are updated. The dashboard pickers read the catalog from the API.
- **Prices** (monthly ÷ 2,628,000 s): `basic-256mb` $4.20, `basic-1gb` $13.30, `1c-2g` $28.00, `pro-4gb` $38.50, `basic-4gb` $52.50. HA standby billing was already correct: the usage query sums every `<name>-N` pod. A Prometheus test now pins primary + standby = 7200 s/h.
- **HA rule:** unchanged (refuse moving to a <1-CPU plan while HA stays on, unless the same request disables HA). The test-only `PlanSupportsHighAvailability` variable is gone, and tests use the real plans. The operator sets CNPG `podAntiAffinityType: required` for HA databases.
- **Ceiling:** `bex-tenant-0` max-size is 8 in `cluster.yaml`, with `clusterapi-validate.sh` and ADR060 matching. `app-cluster.yml` applies it.
- **Billing gap found and fixed:** `EnsureContract` attached Prices only when it created a Subscription, so existing workspaces would never have been invoiced for a new dimension. Existing Subscriptions now gain any catalog Price whose meter no item rates (`proration_behavior=none`, idempotent, once per process). Matching is by meter, so a repriced dimension is never double-rated. The retired → current swap for `basic-256mb`/`basic-1gb` stays a runbook step at the period boundary (`docs/runbooks/stripe-billing-setup.md` § Repricing a dimension). A Subscription Schedule would replay stale metadata, coupon and default-card settings. The sync is fail-soft: if the live Stripe catalog lacks a Price, it logs `contract_item_sync=catalog_unresolved` and never holds usage. `TestCatalogFitsOneSubscription` guards Stripe's 20-item limit (18 today).

**Still open in t003/t012 (live):** (1) verify `kubectl get md -n bex-capi bex-tenant-0` shows `max=8` after the `app-cluster.yml` roll; (2) run `scripts/stripe-billing-setup.py` in **live** mode to create the 5 prices and retire the 2 (Stripe admin authorization needed); (3) move existing subscriptions off the two retired prices at their next period per the runbook. Until (2) runs, usage of the new tiers is metered but not invoiced.

Not changed (follow-ups for t008 parity review): ADR049 says Render's Blueprints refuse retired legacy plans such as `pro-4gb`, while bex accepts `pro-4gb` per t001. The Blueprint pricing label renders `1c-2g` as "1c 2g". The CLI Heroku skill maps X-0 (2 vCPU / 4 GB) to `pro-4gb` rather than `basic-4gb`.

## t004 + t005 — 2026-10-29 (shipped `5aa58bfaf`, `396c8c0fc`)

- **t004:** `postgres_cluster_leader_changed` (details `{"leaderId": "<new primary instance>"}`, the only field in Render's pinned schema). It comes from `Database.status.currentPrimary` against a latched checkpoint (migration `0173`): none at create (the first primary only latches), one per promotion, and none on unrelated status updates. Idempotency: `SELECT … FOR UPDATE` on the checkpoint, plus a promotion-keyed fact (`<dpg>:leader:<primary>:<promotedAt>`, `ON CONFLICT DO NOTHING`). A stale read from another replica is ignored through CNPG's `currentPrimaryTimestamp`, which the operator now projects. The event is subscribable on REST/GraphQL/MCP webhooks and the dashboard picker, and readable by id on all three. Like every datastore event, it has no service-events list home (w3/m82 boundary). Real-Postgres test `TestPGClusterLeaderChangedOncePerPromotion`.
- **t005:** the pickers already read the catalog. Added: HA enable/disable on existing databases (GraphQL `updateDatabaseHighAvailability`, MCP `update_postgres.enableHighAvailability`, same rule as REST), a create-dialog HA switch shown only for HA-eligible plans, an HA panel switch with standby price confirmation, and the plan section's <1-CPU block using the requested state. Skeletons: the Postgres plan preview now shows 6 tier slots, pinned to `tiers.yaml`, and the HA preview gained a switch slot. Not done: a browser side-by-side of pending vs ready at 1440/390 px. t007 checks that.

## t006 — live HA acceptance on `dev-2`, 2026-10-30 (PASS)

Local CAPD cluster. The shared local operator was updated to main (`bex-operator:w2-m171`, built from `5b6ca4fb5`; CRDs reinstalled), and `dev-2`'s bex-api was rebuilt from the same tree. A throwaway Kratos identity and workspace (switched to Pro so webhooks are allowed) were deleted afterwards, along with the database and webhook.

- **Create:** `POST /v1/postgres {"plan":"1c-2g","enableHighAvailability":true}` → `dpg-dbi03ohjg4r95gm7765g`. CNPG `instances: 2`, `resources` 1 CPU / 2Gi (Guaranteed), `affinity: {enablePodAntiAffinity: true, podAntiAffinityType: required}`. The instances landed on **two different nodes** (`bex-worker-0…`, `bex-tenant-0…`). REST then reported `status: available`, `plan: 1c-2g`, `highAvailabilityEnabled: true`.
- **Primary loss:** deleting the primary Pod at 02:59:26Z promoted `-2` within about 5 s. A write through the unchanged `internalConnectionString` succeeded afterwards. `Database.status` showed `currentPrimary: …-2` and `currentPrimaryTimestamp: 02:59:28Z`.
- **`failover` verb:** `POST /v1/postgres/{id}/failover` → 202 at 03:00:49Z, promoted `-1` within about 5 s, and writes still worked (`before,after-kill,after-verb`).
- **Events:** exactly **two** `postgres_cluster_leader_changed` events, none at create and none on later reconciles (checked 2 min later): `evt-7amt1lb01e12k9pq5te8` (02:59:28Z, `{"leaderId":"…-2"}`) and `evt-cfr7bdtc2lr9p1iaqed4` (03:00:50Z, `{"leaderId":"…-1"}`), both readable via `GET /v1/events/{id}`.
- **Webhook:** each event produced a delivery (`whd-…`, `eventType: postgres_cluster_leader_changed`, `serviceName: dpg-…`) plus one retry of the same event. The target was a deliberately local `https://localhost` URL, which the SSRF guard refused ("destination resolves to a blocked address"), so the event is deliverable through the pipeline. The production check of an external receiver happens in the closing live walk.
- **Local-environment gap found (not product):** the local overlay deletes production's `allow-cnpg-apiserver-egress` Cilium policy without a Calico equivalent, so no tenant Postgres can initialize on the local cluster (initdb: `dial tcp 10.128.0.1:443: i/o timeout`). For this run, a namespaced NetworkPolicy was added in the throwaway tenant namespace. Filed as `w2/053`.

## t008 — Render parity, 2026-10-30

- **Plans on every surface:** REST, GraphQL, MCP, Blueprint and the dashboard all read the one catalog (t003/t005). Render's pinned `render.yaml` schema `postgresPlan` enum contains `1c-2g`, `1c-4g`, `2c-4g` **and** `pro-4gb`/`basic-4gb`, so accepting `pro-4gb` matches Render. ADR049 refuses only the plans bex does not sell (`pro-8gb` …).
- **Fixed:** the Blueprint estimate labeled Postgres plans with the compute title-caser ("1c 2g", "Basic 1gb"), while the Postgres surfaces said "1c-2g" / "Basic 1GB". Both now use `postgres.TierDisplayName`.
- **HA plan-change rule:** Render's HA documentation (re-read 2026-10-30) says only "at least 1 CPU" and nothing about changing plan while HA is on. Per the t001 decision, the fallback stands: refuse with `POSTGRES_HA_PLAN_UNSUPPORTED` unless the same request disables HA, so nothing is silently turned off. It is identical on REST, GraphQL, MCP and Blueprint (t003 tests).
- **Event:** `postgres_cluster_leader_changed` carries exactly Render's pinned details (`leaderId`). Like every datastore event, it has no service-events list home (an existing w3/m82 boundary, not new here).
- **Not a bex divergence:** the CLI's Heroku-migration skill table maps X-0 to `pro-4gb` and lists plans bex does not sell. It is the Render skill pack's own mapping, left as-is.

## t009 + t010 — 2026-10-30

- **t009:** GraphQL `updateDatabaseHighAvailability` now goes through `gqlutil.PatchMutation` (widened to boolean fields) instead of a hand-rolled copy of its dry-run handling. As a side effect it gains the standard optional `confirm` argument. Reviewed and left as is: the catalog-unresolved retry cost (2 list calls per workspace per hourly cycle, only while live Stripe lacks a price), and the leader checkpoint (no extra queries).
- **t010:** new tests cover the HA plan-change rule on GraphQL (`updateDatabasePlan` + dry run) and MCP `update_postgres`. Leader-changed is tested with missing, unparseable and mixed timestamps, including a real-Postgres NULL promotion-time case. Subscription sync is tested for retry after an unresolved catalog and for a failed item list. The dashboard create dialog is checked never to send HA after switching to a <1-CPU plan (a mutation check confirmed the test catches it), along with the HA panel price confirmation, refusal handling and the standby-being-removed state. Backend real-deps suite: 72 packages pass. Operator `make test`/`make lint` pass. Dashboard: 5685 tests pass.

## Parked 2026-10-30 — the only open gate is the live Stripe catalog

All implementable work is shipped. What remains needs the user (Stripe admin):

1. **t003/t012 live step:** run `scripts/stripe-billing-setup.py` in **live** mode, creating the `1c-2g`, `pro-4gb` and `basic-4gb` prices and the corrected `basic-256mb`/`basic-1gb` prices, then retire the two old ones. The script must then report the live catalog in sync with `pricing.yaml`. Asked in `/loopx w2` on 2026-10-29. No answer yet. Until this runs, usage of the new tiers is metered but not invoiced (fail-soft sync, so no other billing is affected).
2. **Then:** at each existing subscription's next period, swap the two retired price items per `docs/runbooks/stripe-billing-setup.md` § Repricing a dimension (list the affected subscriptions first).
3. **t011 Closeout:** verify the DoD on production with the pinned CLI (`bex postgres create --plan 1c-2g --high-availability`), REST, GraphQL, MCP and the dashboard (two instances on distinct nodes, failover, one event), the usage estimate, and `stripe-billing-setup.py` in sync. Then close.

Release condition: the user authorizes the live Stripe run (or runs it), and the script reports in sync.
