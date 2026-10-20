# w2 · m171 — Postgres plans large enough for high availability

**Worker:** worker2 **Goal:** a tenant can create or upgrade a managed Postgres on a ≥1-CPU plan, enable high availability, and survive a primary failure with Render's `postgres_cluster_leader_changed` event — on REST, GraphQL, MCP, the pinned Render CLI and the dashboard. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Fetch Render's live Postgres catalog and record which ≥1-CPU tiers bex adds, at Render × 0.70 (**user decision**) | 40m | — |
| t002 | Capacity and placement check: an HA pair of each chosen tier on production node types | 40m | t001 |
| t003 | Add the tiers to the catalog, pricing, Stripe catalog and schema enums, with Render's plan-change rules for HA | 1h | t002 |
| t004 | Emit `postgres_cluster_leader_changed` when the primary changes | 1h | t003, w5/m138/t003 |
| t005 | Dashboard: the new tiers in every plan picker, HA available on them, skeletons unchanged in geometry | 45m | t003 |
| t006 | Live HA acceptance on `dev-2`: create, fail the primary, observe failover and the event | 1h | t004, t005 |
| t007 | Correct ADR018, ADR009 §5, ADR030 and the CLI checklist | 20m | t006 |
| t008 | Render parity | 30m | t007 |
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
