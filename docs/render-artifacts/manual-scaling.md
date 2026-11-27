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

## Compute page (2026-11-19, w6/m483)

Render moved these sections onto a service's **Compute** page on 2026-08-26 ("open your service's Compute page and scroll down to the Manual Scaling section", render.com/docs/scaling). bex followed: the Scaling tab's three sections now sit under the Compute section on `/services/{id}/compute`, in the order above, and `/scaling` redirects to `/compute#manual-scaling` ([compute-page.md](compute-page.md)).

## Autoscaling's confirmations, read from Render's dashboard bundle (2026-11-26, w4/m696)

The 2026-07-16 capture above elides the enable dialog's body and does not say what a save with both target toggles off does. Both were read on 2026-11-26 from Render's public dashboard bundle (the entry chunk `index-D5V4ZgbE.js`, by `scripts/render-dashboard-ops.sh`'s crawl, 459 chunks; the method is [dashboard-operations.md](dashboard-operations.md)'s). Chunk names change with each Render deploy: search for the strings below.

- **The Autoscaling switch saves nothing by itself.** Flipping it opens a confirmation at once, and only the confirmation's primary button sends `updateAutoscalingConfig`. Cancel (or closing the dialog) flips the switch back and sends nothing.
- **Enable**, on a service that does not autoscale: title "Enable Autoscaling?", body "When your service scales up, each running instance uses the same compute plan and is billed accordingly.", buttons **Cancel** (secondary) and **Enable** (primary). Enable turns autoscaling on with the service's stored autoscaling settings, or, when it has none, with the current instance count as the minimum, twice that as the maximum, and Target CPU and Target Memory both on at 60%.
- **The settings form shows only while autoscaling is on**: Number of Instances, Target CPU Utilization, Target Memory Utilization, then **Save Changes**, which stays disabled until a value changes, and **Cancel**, shown only then. Editing therefore comes after the enable, and Save Changes asks nothing.
- **Disable**, from the switch: title "Disable Autoscaling?", body "Your service will run the fixed number of instances specified under **Manual Scaling**.", buttons **Cancel** and **Disable** (primary, not styled as destructive).
- **Both toggles off never reaches Save.** Turning off the last enabled target (Target CPU while Target Memory is off, or the reverse) opens the same "Disable Autoscaling?" dialog at once, with the body "Disabling CPU-driven autoscaling will disable autoscaling entirely for this service. Are you sure?" (for memory, "Disabling memory-driven autoscaling will disable autoscaling entirely for this service. Are you sure?"). Cancel turns the toggle back on. Disable turns autoscaling off with the stored settings, leaving the form's unsaved edits unsent. Nothing on the page refuses a save for want of a target. This is the dashboard's side of render.com/docs/scaling's "If you enable _neither_ toggle, autoscaling is _disabled_ for the service."
- **A disk-backed service** gets neither card: the page says "Scaling is not supported for servers with disks." with a **Change disk settings** link. The dialog's disk variant ("Are you sure?", "New instances will have empty disks when they're started.", **Turn autoscaling on**) cannot open from that page, and render.com/docs/disks still says "You can't scale a service to multiple instances if it has a disk attached" (read 2026-11-26).

bex follows this since w4/m696: the Scaling section's switch opens the enable or disable confirmation before any save, the form shows only once autoscaling is on, and turning off the last target asks to disable autoscaling.
