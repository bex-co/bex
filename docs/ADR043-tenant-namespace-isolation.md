# ADR: Per-tenant namespace isolation — workspace = namespace

**Status:** proposed — supersedes the **tenant-boundary mechanism** of [ADR022-tenant-isolation.md](ADR022-tenant-isolation.md) (its "Option B": a shared apps namespace + label-scoped per-App NetworkPolicies). ADR022's _other_ mechanisms (platform-side lockdown, registry access control, tenant container hardening, node/cloud-metadata egress) remain in force — they move _into_ the per-tenant namespace model rather than being discarded. Also refines [ADR042-sandbox-cluster-substrate.md](ADR042-sandbox-cluster-substrate.md) D3 (a shared `bex-sandbox` namespace → per-tenant `<ws>-sandbox`).

## Context

bex's tenancy boundary today is [ADR022](ADR022-tenant-isolation.md)'s chosen design: **all tenant pods share one namespace** (`BEX_CP_APPS_NAMESPACE`, default `default`) and are isolated by **label-scoped NetworkPolicies** keyed on `app.bex.co/workspace`. ADR022 explicitly evaluated namespace-per-workspace and rejected it:

> ### Option A — Namespace-per-workspace
>
> Each workspace gets its own namespace … Cons: major projector churn (every App CR moves namespace on first-login); URL/metrics/logs blast radius (bex-api namespace-scopes its watch); doubles operational surface **without delivering a meaningfully stronger boundary than Option B for the threat above (kernel still shared)**. Deferred: if isolation demands grow beyond network, the next tier is microVM (Kata/Firecracker).
>
> ### Option B — Label-scoped NetworkPolicies in a shared apps namespace ✓
>
> … uses `app.bex.co/workspace` label selectors for same-workspace allow rules. **Chosen: Option B.**

ADR022 was **correct for its threat model**: that model was scoped to **east-west _network_ isolation only** (pod A must not reach pod B). For that single threat, namespace vs. label is indeed not meaningfully different, and Option B avoided churn. The decision is recorded at [ADR022 §Mechanism choice](ADR022-tenant-isolation.md#mechanism-choice).

The isolation _goals_ have since grown beyond east-west network. This ADR records that namespace-per-workspace is the correct boundary for the expanded goals, and deprecates Option B _as the tenant boundary_ (not ADR022's network mechanisms themselves).

## Why namespace, not labels — the expanded goals

| Goal | Label-scoped (ADR022 Option B) | Namespace-per-workspace |
| --- | --- | --- |
| East-west network deny | ✓ (equal) | ✓ (equal) |
| **Structural boundary** (can't be forgotten by a misconfigured policy) | ✗ conventional — depends on every policy carrying the label selector | ✓ structural — `namespaceSelector` default-deny |
| **Plan-tier quota per tenant** (no one tenant starves others) | ✗ one shared `tenant-apps-quota`; caps enforced in bex-api _code_ (`BEX_MAX_*`) | ✓ per-namespace `ResourceQuota`, enforced by the API server |
| **Entity alignment** (k8s boundary = tenancy boundary) | ✗ workspace is a label, not a k8s object | ✓ workspace = namespace, 1:1 |
| **Regime separation** (hosting vs sandbox) | ✗ forces label-split _within_ the namespace | ✓ one namespace per uniform regime |
| **Blast-radius / least-privilege containment** | partial | ✓ default-deny between any two untrusted workloads |

The structural, quota, alignment, and regime goals are things labels cannot deliver. That is the case for the reversal.

## Decision

### D1 — workspace = namespace

Each workspace maps to k8s namespace(s). Hosting workloads (App / Database / KeyValue) land in `<ws>`; sandbox workloads land in `<ws>-sandbox`. Environments (ADR032 staging/production) are a label _within_ `<ws>` (`app.bex.co/env`), because the **tenant** boundary is the structural one; a tenant-internal subdivision stays soft. k8s namespaces are not nestable, so the tenant layer takes the namespace.

### D2 — hosting and sandbox are separate namespaces (regime split), both untrusted

Hosting (`<ws>`, an addressable **service** that serves traffic) and sandbox (`<ws>-sandbox`, a sealed **exec-box** driven via the gateway) have opposite default network postures and different ownership (operator vs OpenSandbox per [ADR042](ADR042-sandbox-cluster-substrate.md)) — so they are separate namespaces. Both regimes are **equally untrusted** (both run tenant-submitted code); there is no trust tier between them. See [ADR042](ADR042-sandbox-cluster-substrate.md) for the sandbox substrate.

### D3 — zero-trust east-west; enforcement pushed to the lowest layer

All tenant workloads are mutually untrusted. Default-deny between namespaces (tenant↔tenant, hosting↔sandbox); communication is opt-in, namespace-scoped so an allow physically cannot cross tenants, and narrowed to named services/ports. Plan-tier caps move out of bex-api code (`BEX_MAX_*`) into per-namespace `ResourceQuota`/`LimitRange` — enforced by the API server, not bypassable by an application bug.

### D4 — scale-to-zero is preserved (namespace is logical, node pool is physical)

The invariant that keeps "sleep = free" working: **per-tenant namespace (logical) + shared tenant node pool (physical)**. Pods from every `<ws>` bin-pack onto the same tenant nodes, so when all tenants sleep the pool scales to 0 exactly as today. The activator/operator/bex-api stay always-on in platform namespaces and manage wake across all `<ws>` namespaces (cluster-scoped or per-namespace RBAC). Namespace lifecycle is tied to **workspace** create/delete, not to App sleep/wake. The one thing that _would_ break scale-0 — confusing namespace with a per-tenant node pool — is explicitly rejected (see Alternatives).

### D5 — ADR022's mechanisms are retained, re-scoped

What ADR022 built beyond the boundary choice stays; it is applied per-namespace instead of label-scoped-in-one-namespace:

- The Cilium `egressDeny` for node/cloud-metadata ([ADR022 §node and cloud-metadata egress](ADR022-tenant-isolation.md#node-and-cloud-metadata-egress-w7m4)) — still cluster-wide.
- Platform-side lockdown ([ADR022 §Platform-side lockdown](ADR022-tenant-isolation.md#platform-side-lockdown-t004)) — still denies tenant→platform.
- Registry access control / per-App Zot ACLs ([ADR022 §Registry access control](ADR022-tenant-isolation.md#registry-access-control-w7m8)) — unchanged.
- Tenant container hardening ([ADR022 §Tenant container hardening](ADR022-tenant-isolation.md#tenant-container-hardening-w7m2)) — unchanged.

Only the **per-App label-scoped NetworkPolicy in a shared namespace as the tenant boundary** (ADR022 t003) is superseded.

## Engaging ADR022's objections to Option A

ADR022 rejected namespace-per-workspace on four grounds. Each, re-examined:

1. **"Major projector churn (every App CR moves namespace on first-login)."** Real, but a one-time migration plus a wiring change — an effort cost, not an architectural principle. (Per the framing of this decision, effort does not weigh against architectural correctness.)
2. **"URL/metrics/logs blast radius (bex-api namespace-scopes its watch)."** bex-api's single-namespace watch (`BEX_API_NAMESPACE`) becomes cluster-scoped or multi-namespace. This is a watch-scope change, not a fundamental limit; many controllers already watch cluster-wide.
3. **"No meaningfully stronger boundary than Option B for the threat above (kernel still shared)."** True **for east-west network only** — ADR022's scope. False for the expanded goals (structural integrity, per-tenant quota, regime separation). The boundary _is_ stronger once isolation means more than network deny.
4. **"The next tier is microVM, not namespace."** This framed namespace and microVM as either/or escalation rungs. They are **orthogonal axes**: namespace is the _logical/administrative_ boundary; microVM is the _runtime/kernel_ boundary. Both are warranted; ADR022's "microVM is the escalation" was correct for the runtime axis but did not address the logical axis this ADR settles.

## Alternatives considered

- **ADR022 Option B — shared namespace + label-scoped policies.** The current, implemented choice. Deprecated as the tenant boundary by this ADR for the expanded goals above; retained as a network mechanism re-scoped to the namespace.
- **vcluster / per-tenant control plane.** Rejected ([`.pm/DO_NOT_DO.md`](../.pm/DO_NOT_DO.md) #28) — namespace is not a control plane; it shares one apiserver, so it does not violate that decision.
- **Per-tenant node pool.** Rejected — breaks scale-to-zero (a per-tenant pool with min≥1 never scales to 0) and cross-tenant bin-packing. Namespace is logical; node capacity stays shared.
- **microVM alone (no namespace change).** Insufficient — Kata raises the runtime bar but does not give per-tenant quota, structural network defaults, or entity alignment. Both, not either.

## Consequences

- **Refactor scope** (factual, not a reason against): the operator, bex-api, the control-plane projector, and RBAC move from single-namespace assumptions (`BEX_APPS_NAMESPACE`, `BEX_API_NAMESPACE`, the `bex-operator-apps` RoleBinding) to cluster-scoped/per-namespace over tenant namespaces. Namespace lifecycle becomes workspace lifecycle (create/delete on workspace create/delete).
- **Per-tenant `ResourceQuota`/`LimitRange`** replace the `BEX_MAX_SERVICES` / `BEX_MAX_POSTGRES` / `BEX_MAX_KEYVALUES` app-code caps — same intent, enforced one layer down.
- **ADR022 is marked deprecated** (banner added, linking here). Its implemented label-scoped policies remain in production until this ADR is implemented; its other mechanisms are retained.
- **ADR042 D3 is refined**: the shared `bex-sandbox` namespace becomes per-tenant `<ws>-sandbox` (consistent with D1). ADR042 should be updated to match on implementation.
- **Migration**: new workspaces provision `<ws>` (+`<ws>-sandbox` on pillar-5 enable); existing workspaces migrated. The label `app.bex.co/workspace` is retained as a pod label within namespaces (useful for selection/metrics), it just stops being the _boundary_.
