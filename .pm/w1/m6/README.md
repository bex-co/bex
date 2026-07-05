# w1 · m6 — Multi-tenant isolation (namespace → vcluster)

**Worker:** worker1 **Goal:** Isolate tenants who share the worker pools — isolation is by namespace/vcluster/sandbox, **not** by pool. Start soft + cheap (namespace per tenant), leave stronger tiers as scoped spikes. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Namespace per tenant + RBAC + ResourceQuota + LimitRange | 30m | (m2) |
| t002 | Default-deny cross-tenant NetworkPolicy + deploy into ns | 25m | t001 |
| t003 | Spike: vcluster per tenant via Argo ApplicationSet | 30m | t001 |
| t004 | Design note: opt-in hard isolation (Kata/Firecracker) | 20m | — |

## Definition of done

The control plane provisions a per-tenant namespace with RBAC + quota + limits + default-deny netpol, and tenant Apps deploy into it. Stronger isolation (vcluster, microVM) is designed and de-risked as spikes.

## Source

Converted from `.tmp/006-multi-tenant-isolation.md` (see `docs/control-plane.md`: isolation units ≠ worker pools). Depends on the control plane (m2) creating tenants.
