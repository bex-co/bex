# CLI sweep 566 — autoscaling subresource dialects (owned fixture, REST boundary)

**Verdict: no bug filed.** Same isolated QA session (`puncsky@gmail.com`), `bex-canary`, HEAD-built CLI at `77ec448ae`, production pin `d98d07f62`, 2026-10-19T18:48–18:50Z. Owned free fixture `qa566-autoscale` (`srv-dbb6btdt4o5c73dqgc90`, reached `live`) via same-credential REST (`/tmp/qa501/autoscale566.py`; the pinned CLI exposes no autoscaling verbs). Deleted with service + subresource 404 proof; workspace back to 8. Contract: pinned `PUT|DELETE /services/{id}/autoscaling` (bex also serves `GET`) and `autoscalingSpec` in `lego/backend/internal/apps/service.go`.

- **Happy path + read-back:** initial GET is the zero object (`enabled:false, min/max 0, criteria 0`); PUT `{enabled,min:1,max:1,criteria 70/70}` → 200 echo, GET byte-equal. DELETE → 204, GET reverts to the zero object, second DELETE → 204 again (idempotent).
- **Bounds, each named:** free-plan cap (`max:2` → "the free plan is limited to 1 instance(s) and cannot scale to 2"); percentages outside 1–100 per target; missing `criteria` or `criteria.memory` at the boundary layer; both criteria disabled → "at least one of criteria.cpu or criteria.memory must have enabled:true". The bex-native dialect (`minInstances/…`) on this Render-shaped route is refused at the boundary, as w4/m104 contracts.
- **Two cells checked against code, both deliberate:** `min:0` is accepted and stored — the coded bound is `minInstances ≥ 0` (scale-to-zero floor; the pinned schema declares no numeric bounds, so this is bex policy, not a schema gap). `enabled:false` PUT with dormant bounds returns/stores the zero object — disabled ≡ absent spec, same canonical state as after DELETE.
- **Type eligibility (w4/m101):** baseline cron GET → 200 zero object (stale-config normalization), PUT → named 400 ("a cron_job cannot use autoscaling; autoscaling is supported on web services, private services and background workers"). Synthetic and Postgres ids → uniform 404 on GET and PUT; the baseline cron was not mutated (refusal precedes any write).

## Unrun

- The autoscaler actually moving replicas under load (free tier caps at 1 instance; a min<max range needs a paid fixture).
- Blueprint `scaling:` block dialects (same `autoscalingSpec` validator via `specFromCreate`).
