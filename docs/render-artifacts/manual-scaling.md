# Render artifact — Manual scaling (instance count)

**Captured:** official docs/API revalidated 2026-07-18 plus authenticated dashboard capture 2026-07-16. Primary sources: [Scaling Render Services](https://render.com/docs/scaling), [Scale instance count API](https://api-docs.render.com/reference/scale-service), and [Render webhook event types](https://render.com/docs/webhooks#event-types).

## What Render ships

Render's Scaling page (web/private/background services only—not cron jobs) contains separate autoscaling, manual scaling, and recent-metrics sections.

### Manual scaling (default)

A 1–100 slider, numeric input, and **Save Changes** button set a fixed instance count. The API is `POST /v1/services/{id}/scale` with `{ "numInstances": N }` and returns **202**, so success acknowledges accepted desired state while instance provisioning/deprovisioning converges asynchronously. Render documents manual scale as an `instance_count_changed` event, separately from deploy/redeploy events; it does not require a source build or pre-deploy command.

### Autoscaling (toggle)

A separate card (`w5/m` autoscaling tab) — out of scope for this milestone.

## bex parity decisions

| Decision | bex |
| --- | --- |
| Section placement | Scaling tab, below Autoscaling (non-cron/static only) |
| Min / max | 1 – 100 (`store.MaxReplicas`; backend rejects 0 and >100) |
| Save affordance | Save button enabled when draft ≠ current, disabled while loading |
| Suspended service | Control is enabled — spec.replicas is stored; manifests after resume |
| Cron jobs | Hidden (no replica concept; consistent with m11 type-aware pattern) |
| Mutation | `scaleService(id, numInstances)` GraphQL (REST `POST /scale` equivalent) |
| Acknowledgement | “Scaling to N…” after 202/GraphQL acceptance; never completed tense before workload convergence |
| Deploy side effects | None: reuse active image/revision, no build/pre-deploy/deploy-history row |

## Historical correction (2026-07-16, applied w7/m43)

The docs-fallback reconstruction above guessed the wrong page. A live authenticated walk of `dashboard.render.com/web/srv-…/scaling` shows Render's manual scaling does **not** live under Settings — the **Scaling tab** carries three sections:

1. **Autoscaling** — toggle + config (min/max instances dual-slider **1–100**, Target CPU / Target Memory each with their own enable switch + 1–90% slider, **default 60%**), with confirm modals on both enable ("each running instance … is billed accordingly") and disable ("Your service will run the fixed number of instances specified under **Manual Scaling**").
2. **Manual Scaling** — "Run multiple instances that are automatically load balanced. All instances use the same instance type and are billed accordingly." + an Instances slider (1–100) with a numeric input and a Save Changes button. **Mutually exclusive with autoscaling**: the card is hidden while autoscaling is on.
3. **Recent Metrics** — "Showing metrics for the past 48 hours. View all metrics." + Average Memory Utilization / Average CPU Utilization ("Across all instances") / Total Instances, each with a "No data captured in the past 48 hours" empty state.

Render's Settings page has **no instance-count control**. bex adopted this placement in **w7/m43**: the w5/m16 Settings stepper was removed, a Manual Scaling card (slider 1–100 + input + Save, same `scaleService` mutation) joined the Scaling tab under the same mutual exclusion, the Recent Metrics section was added over the existing `metrics` query, and the autoscaling card's deltas were corrected (slider cap 25→100, default target 75→60, en copy "Render"→"bex", disable confirm with Render's fixed-count explanation).

## Production mechanism correction (2026-07-18, w2/m56)

The API/UI shape was already correct, but bex's operator treated the scale mutation's Kubernetes generation bump as a release. On source-backed `srv-d9dd16roviqs738quds0`, scaling 1→2 launched `bld-…-gen-2`; the copied one-hour GitHub installation token was already expired, so the build failed and the generation-1 Deployment remained at one replica. w2/m56 separates artifact/release identity from generic generation: replicas now reconcile against the active image and revision, and the dashboard acknowledges accepted intent with in-progress wording.
