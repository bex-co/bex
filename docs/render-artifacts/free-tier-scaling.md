# Render artifact — Free instance types have no horizontal scaling

**Captured:** 2026-08-27 from Render's public docs. Primary sources: [Scaling Render Services](https://render.com/docs/scaling), [Free instance types](https://render.com/docs/free), [Compute plans](https://render.com/docs/compute-plans), [Render free tier blog](https://render.com/blog/free-tier).

## What Render ships

Horizontal scaling — running more than one instance of a service — is a **paid-only** capability on Render:

- **Scaling requires a paid instance type.** The Scaling docs state you can run multiple instances "on a paid instance type." Each additional instance uses the same instance type as the first and is billed accordingly.
- **Free web services run a single instance.** Render's free instance types do not support horizontal scaling; a free web service is exactly one running instance. Manual scaling and autoscaling are paid-tier features.
- **"Billed accordingly" is paid-only copy.** The per-instance "billed accordingly" language on the Scaling page (and in the autoscaling / manual-scaling confirm dialogs) can only apply to paid plans — a free instance has no per-instance charge to bill, and there is never more than one of it.

## bex parity decision (w6/m118)

bex's compute `free` tier is rated `usdPerSecond: 0.0` (`lego/backend/internal/pricing/pricing.yaml`). With a zero rate and no cap, N free instances would deliver N× the capacity for $0 — effectively unlimited free compute. To match Render exactly, the free plan is capped at **one running instance**:

- The cap lives in the reviewed tier catalog: `lego/types/tiers/tiers.yaml` gives the compute `free` tier `maxInstances: 1`. Paid tiers omit the field, so they carry no plan-specific cap — only the platform ceiling `MaxReplicas = 100`.
- bex-api enforces it on every write path that sets an instance count — `Scale` (REST `POST …/scale`, GraphQL `scaleService`, MCP `scale_service`), service create, and autoscaling `minInstances` / `maxInstances` — refusing over-cap requests with an error that names the plan and the limit (the `errNoPublicIngress` refusal shape, not a bare 400).
- Changing a service to a plan whose cap is below its current instance count (fixed replicas or autoscaling max) is refused with a "scale down first" message rather than a silent shrink, mirroring the "disable maintenance mode before changing to the free plan" guard.
- The operator's plan-aware `clampReplicas` is the defense-in-depth backstop, so a hand-applied CR or projector bug can never run more instances than the plan allows.

Design records: [ADR018 § Manual scale / Autoscaling config](../ADR018-render-parity.md) and [ADR030 § Free compute plan cap](../ADR030-pricing.md).
