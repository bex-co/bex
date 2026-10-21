# w2 · m171 — Postgres plans large enough for high availability

**Worker:** worker2 **Goal:** a tenant can create or upgrade a managed Postgres on a ≥1-CPU plan, enable high availability, and survive a primary failure with Render's `postgres_cluster_leader_changed` event — on REST, GraphQL, MCP, the pinned Render CLI and the dashboard. **Status:** todo — t001 done 2026-10-20 (user approved the three tiers and prices); t002 next; `w5/m138/t003` (t004's other dependency) is done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Fetch Render's live Postgres catalog and record which ≥1-CPU tiers bex adds, at Render × 0.70 — **DONE** | 40m | — |
| t002 | Capacity and placement check: an HA pair of each chosen tier on production node types | 40m | t001 |
| t003 | Add the tiers to the catalog, pricing, Stripe catalog and schema enums, with Render's plan-change rules for HA | 1h | t002 |
| t004 | Emit `postgres_cluster_leader_changed` when the primary changes | 1h | t003, w5/m138/t003 |
| t005 | Dashboard: the new tiers in every plan picker, HA available on them, skeletons unchanged in geometry | 45m | t003 |
| t006 | Live HA acceptance on `dev-2`: create, fail the primary, observe failover and the event | 1h | t004, t005 |
| t007 | Correct ADR018, ADR009 §5, ADR030 and the CLI checklist | 20m | t006 |
| t012 | Correct the `basic-256mb` / `basic-1gb` prices to Render × 0.70 ($4.20 / $13.30), moving existing subscriptions at their next period (absorbs `w2/047`) | 30m | t003 |
| t008 | Render parity | 30m | t007, t012 |
| t009 | Simplify | 15m | t008 |
| t010 | Test coverage | 45m | t008 |
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
- **Goal linkage:** [ADR008](../../../docs/ADR008-vision.md) pillar 1 (Render-compatible API) and [ADR009](../../../docs/ADR009-postgresql-management.md) managed Postgres — its §5 maps Render's Pro tier to "availability + PITR".
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
