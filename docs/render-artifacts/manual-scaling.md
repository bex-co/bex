# Render artifact — Manual scaling (instance count)

**Captured:** docs-fallback (render.com login required; layout reconstructed from Render's public docs and the REST/MCP surfaces — `POST /v1/services/{id}/scale`, field `numInstances`).

## What Render ships

Render's Settings tab (web/private/background services only — not cron jobs) contains a **Scaling** section with two sub-modes toggled by the user:

### Manual scaling (default)

A numeric stepper with **−** and **+** buttons flanking the current instance count. The input accepts free-form entry; bounds are 1–∞ (Render's upper limit is plan-dependent). A **Save Changes** button appears when the draft differs from the live value; clicking it fires `POST /v1/services/{id}/scale` with body `{ "numInstances": N }`. The mutation is synchronous — the response returns immediately and the pods converge in the background.

### Autoscaling (toggle)

A separate card (`w5/m` autoscaling tab) — out of scope for this milestone.

## bex parity decisions

| Decision | bex |
| --- | --- |
| Section placement | Inside the existing Settings Card, below Idle Timeout (non-cron only) |
| Min / max | 1 – 100 (`store.MaxReplicas`; backend rejects 0 and >100) |
| Save affordance | Save button enabled when draft ≠ current, disabled while loading |
| Suspended service | Control is enabled — spec.replicas is stored; manifests after resume |
| Cron jobs | Hidden (no replica concept; consistent with m11 type-aware pattern) |
| Mutation | `scaleService(id, numInstances)` GraphQL (REST `POST /scale` equivalent) |

## Live-capture correction (2026-07-16, applied w7/m43)

The docs-fallback reconstruction above guessed the wrong page. A live authenticated walk of `dashboard.render.com/web/srv-…/scaling` shows Render's manual scaling does **not** live under Settings — the **Scaling tab** carries three sections:

1. **Autoscaling** — toggle + config (min/max instances dual-slider **1–100**, Target CPU / Target Memory each with their own enable switch + 1–90% slider, **default 60%**), with confirm modals on both enable ("each running instance … is billed accordingly") and disable ("Your service will run the fixed number of instances specified under **Manual Scaling**").
2. **Manual Scaling** — "Run multiple instances that are automatically load balanced. All instances use the same instance type and are billed accordingly." + an Instances slider (1–100) with a numeric input and a Save Changes button. **Mutually exclusive with autoscaling**: the card is hidden while autoscaling is on.
3. **Recent Metrics** — "Showing metrics for the past 48 hours. View all metrics." + Average Memory Utilization / Average CPU Utilization ("Across all instances") / Total Instances, each with a "No data captured in the past 48 hours" empty state.

Render's Settings page has **no instance-count control**. bex adopted this placement in **w7/m43**: the w5/m16 Settings stepper was removed, a Manual Scaling card (slider 1–100 + input + Save, same `scaleService` mutation) joined the Scaling tab under the same mutual exclusion, the Recent Metrics section was added over the existing `metrics` query, and the autoscaling card's deltas were corrected (slider cap 25→100, default target 75→60, en copy "Render"→"bex", disable confirm with Render's fixed-count explanation).
