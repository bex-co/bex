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
