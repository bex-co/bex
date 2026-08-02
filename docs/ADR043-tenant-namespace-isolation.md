# ADR: Per-tenant namespace isolation — workspace = namespace

**Status:** accepted, implemented, and unconditional. The fleet was 100% migrated as of 2026-07-28 (0 store-managed App CRs remained in the shared `default` namespace); the transitional `BEX_TENANT_NAMESPACES` / `BEX_TENANT_SANDBOX_NAMESPACES` rollout gates and the now-dead shared-namespace code paths they guarded were retired in [w3/m34](../.pm/w3/done/m34/README.md), so per-tenant namespaces are the sole, permanent design. Supersedes the **tenant-boundary mechanism** of [ADR022-tenant-isolation.md](ADR022-tenant-isolation.md) (its "Option B": a shared apps namespace + label-scoped per-App NetworkPolicies). ADR022's other mechanisms remain in force and are re-scoped into the per-tenant namespace model. Also refines [ADR042-sandbox-cluster-substrate.md](ADR042-sandbox-cluster-substrate.md) D3.

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

All tenant workloads are mutually untrusted. Default-deny applies between namespaces (tenant↔tenant, hosting↔sandbox) **and between sandbox Pods inside one `<ws>-sandbox` namespace**. A workspace groups authorization and quota; it does not make two members' arbitrary code mutually trusted. Hosting retains its intentional same-namespace service allow, while the sandbox regime gets no same-namespace or label-only allow. Cilium admits only the exact `opensandbox-system/opensandbox-server` ServiceAccount and workload identity to sandbox execd TCP 44772.

Sandbox DNS and public egress are Cilium-enforced outside gVisor: only approved DNS names may be queried through CoreDNS UDP/TCP 53, and the matching public destinations are reachable only through `toFQDNs` plus exact TLS SNI on TCP 443. Public-IP traffic without an approved SNI, arbitrary DNS names, private/rebound targets, cluster/service CIDRs, API server, host/remote-node, and link-local metadata remain denied. A socket may use a learned public IP only while presenting the approved SNI; that is the destination identity Cilium can authenticate. The OpenSandbox iptables/nftables egress sidecar is deliberately absent because it is not a valid gVisor enforcement point.

Namespace-scoped Pod RBAC is not sufficient to preserve the host boundary: an authorized but compromised controller could submit a runc Pod. Native API-server admission therefore requires the gVisor RuntimeClass, sandbox node placement and identity, and disabled ServiceAccount token for every Pod in a sandbox-regime namespace, regardless of which controller creates it.

This is reachability control, not data-loss prevention: an approved writable HTTPS destination may still accept data into an attacker-controlled account. The guest receives no platform or tenant credentials; credential brokering and content-aware outbound proxying are separate future controls.

Other communication is opt-in, namespace-scoped so an allow physically cannot cross tenants, and narrowed to named services/ports. Plan-tier caps move out of bex-api code (`BEX_MAX_*`) into per-namespace `ResourceQuota`/`LimitRange` — enforced by the API server, not bypassable by an application bug.

**Per-plan quota dimensions (w7/m59, ADR045 Finding 4).** `quotaForPlan` (`lego/backend/internal/store/namespaces.go`) stamps each namespace's `tenant-quota` ResourceQuota with compute (cpu/memory requests+limits), `pods`, `count/jobs.batch`, the object-count caps that retire `BEX_MAX_*` (`count/{apps,databases,keyvalues}.app.bex.co`), and the storage/Service axis that bounds the cost-abuse vector:

| dimension | free | paid | rationale |
| --- | --- | --- | --- |
| `requests.storage` | 20Gi | 5Ti | aggregate tenant disk ceiling. The CNPG disk-autoscaler grows PVCs automatically, so without this an autoscaling Postgres is an unbounded per-GB-second bill. Derived from the tier catalog's largest datastore floor (5 GB) × the plan's Postgres+Key Value count caps × a generous autoscale-headroom factor, kept far below "every datastore at the 16 TiB disk-autoscale ceiling" (~400 TiB for a paid namespace). |
| `persistentvolumeclaims` | 4 | 200 | one PVC per datastore (HA replicas included) plus slack. |
| `services.loadbalancers` | 0 | 0 | bex only ever creates ClusterIP Services in a tenant namespace; deny billable cloud LBs outright (defense-in-depth against an operator bug or compromised principal). |
| `services.nodeports` | 0 | 0 | same — no NodePort exposure from a tenant namespace. |

A quota-blocked storage grow is **observable and self-clearing**, never a silent failure. The Postgres disk-autoscaler patches the CNPG Cluster CR, not the PVC — a `requests.storage` rejection would surface only inside CNPG as a silently-unfulfilled resize — so it pre-flights the namespace headroom, sets a `DiskGrowthBlockedByQuota` Database condition, and backs off until headroom returns (a plan upgrade or freed storage). The KeyValue reconciler patches its own PVC, so it catches the `exceeded quota` Forbidden directly and surfaces a `StorageBlockedByQuota` status instead of hot-looping. **Non-disruptive rollout:** a ResourceQuota set below current usage never evicts — Kubernetes only rejects new or increased requests — so the NamespaceReconciler's converge-on-reconcile update path adds these dimensions to already-provisioned `tea-*` quotas without disturbing running workloads.

### D4 — scale-to-zero is preserved (namespace is logical, node pool is physical)

The invariant that keeps "sleep = free" working: **per-tenant namespace (logical) + shared tenant node pool (physical)**. Pods from every `<ws>` bin-pack onto the same tenant nodes, so when all tenants sleep the pool scales to 0 exactly as today. The activator/operator/bex-api stay always-on in platform namespaces and manage wake across all `<ws>` namespaces (cluster-scoped or per-namespace RBAC). Namespace lifecycle is tied to **workspace** create/delete, not to App sleep/wake. The one thing that _would_ break scale-0 — confusing namespace with a per-tenant node pool — is explicitly rejected (see Alternatives).

### D5 — ADR022's mechanisms are retained, re-scoped

What ADR022 built beyond the boundary choice stays; it is applied per-namespace instead of label-scoped-in-one-namespace:

- The Cilium `egressDeny` for node/cloud-metadata ([ADR022 §node and cloud-metadata egress](ADR022-tenant-isolation.md#node-and-cloud-metadata-egress-w7m4)) — retained, but **not by leaving it alone**: today it is a _namespaced_ `CiliumNetworkPolicy` in the shared apps namespace (`deploy/gitops/base/tenant-node-egress.yaml` — a namespaced CNP only selects pods in its own namespace), so it must be promoted to a `CiliumClusterwideNetworkPolicy` (or stamped per `<ws>`) during migration, or migrated pods silently lose the kubelet/metadata guard.
- Platform-side lockdown ([ADR022 §Platform-side lockdown](ADR022-tenant-isolation.md#platform-side-lockdown-t004)) — still denies tenant→platform.
- Registry access control / per-App Zot ACLs ([ADR022 §Registry access control](ADR022-tenant-isolation.md#registry-access-control-w7m8)) — unchanged.
- Tenant container hardening ([ADR022 §Tenant container hardening](ADR022-tenant-isolation.md#tenant-container-hardening-w7m2)) — unchanged.

Only the **per-App label-scoped NetworkPolicy in a shared namespace as the tenant boundary** (ADR022 t003) is superseded.

### D6 — Dynamic namespace RBAC is admission-confined

The `bex-api` NamespaceReconciler must create and prune namespaces whose names do not exist when its RBAC is installed. Kubernetes RBAC cannot scope a ClusterRole to a namespace-name pattern, and creating a RoleBinding to a ClusterRole requires either holding that role or the `bind` verb. A bare cluster-wide `namespaces` mutation grant plus cluster-wide `rolebindings` mutation and named `bind` grants would therefore make the reconciler's application-code checks the only thing stopping a compromised public `bex-api` Pod from deleting a platform namespace or binding a Secret-bearing tenant role there.

The API server, not bex-api, enforces the missing scope. [`bex-api-namespace-admission.yaml`](../deploy/gitops/base/bex-api-namespace-admission.yaml) installs fail-closed `ValidatingAdmissionPolicy` rules keyed to the exact `system:serviceaccount:bex-system:bex-api` principal. Namespace operations are admitted only when the object is a canonical `tea-<xid>` hosting namespace or its `<ws>-sandbox` counterpart and retains the reconciler's workspace/regime/managed identity plus the exact baseline-enforce/restricted-warn-and-audit Pod Security labels. ResourceQuota, LimitRange, NetworkPolicy, and RoleBinding writes are admitted only inside those canonical namespaces and only for reconciler-owned objects. In a sandbox namespace, NetworkPolicy create/update is further limited to the exact empty-rule `default-deny`; delete is allowed only for the three known legacy broad allows that migration must prune. RoleBinding create/update requires the exact managed name→ClusterRole→ServiceAccount mapping for the regime, while delete accepts any known reconciler-owned binding name from either regime: revocation must still remove a malformed or stale pre-migration grant rather than admission preserving it. The RBAC grant itself contains only verbs the direct controller-runtime create/get/list/update/delete path actually invokes; `patch` and `watch` are absent, and ResourceQuota/LimitRange have no delete. Admission supplies the dynamic object scope RBAC cannot express. Argo sync waves install policy (`-3`) then binding (`-2`) before the authority (`-1`), so initial convergence has no unguarded privilege window. This confinement was adversarially verified on production 2026-07-30 (w3/m35): an impersonated `bex-api` attempt to delete/relabel a platform namespace, downgrade a sandbox namespace's Pod Security labels, install an arbitrary sandbox allow policy, or bind a Secret role into a platform namespace is denied, while the exact canonical tenant projection is admitted.

This is a containment boundary, not an authorization substitute. bex-api remains trusted to manage every real tenant namespace, but its projected ServiceAccount token cannot use that dynamic authority against `bex-system`, `opensandbox-system`, `kube-system`, or another noncanonical namespace.

### D7 — Tenant edge aliases are operator-only and destination-fixed

Hosting namespaces may contain operator-created ExternalName Services because Traefik needs cross-namespace static-server and maintenance routing. Namespace isolation alone does not make those aliases safe: Traefik is a trusted ingress principal that can reach platform Services, so a retargeted alias could bypass the tenant pod's own egress rules.

The namespace roles bound to `bex-api`, `bex-ssh-gateway`, and tenant workloads deliberately omit Service/Ingress mutation. The API server additionally applies [`operator-alias-admission.yaml`](../deploy/gitops/base/operator-alias-admission.yaml) only to `default` (legacy) or canonical `bex-controlplane` hosting namespaces. It requires the exact manager principal, matching App controller ownership/labels, and one of the static-server/activator DNS+port tuples. A transition away from ExternalName is matched too. Policy/binding sync waves `-3`/`-2` precede platform workload reconciliation, matching D6's no-unguarded-window rule.

This is separate from D6: D6 confines the namespace reconciler's dynamic authority; D7 confines the operator's deliberate tenant edge objects. Live proof is `scripts/verify-static-site-security.sh live`.

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
- **ADR022 is marked deprecated** (banner added, linking here). Its label-scoped per-App policies were removed from the operator once migration completed (w3/m34); its other mechanisms are retained.
- **ADR042 D3 is refined**: the shared `bex-sandbox` namespace becomes per-tenant `<ws>-sandbox` (consistent with D1). ADR042 D3/D4 were amended to match on 2026-07-27.
- **Migration**: new workspaces provision `<ws>` + `<ws>-sandbox` (both eagerly at workspace create — pillar 5 is re-opened and `w3/m32` consumes the sandbox namespaces; an empty namespace costs nothing and keeps one uniform lifecycle); existing workspaces migrated. The label `app.bex.co/workspace` is retained as a pod label within namespaces (useful for selection/metrics), it just stops being the _boundary_. **Migration complete**: the fleet was fully migrated by 2026-07-28, and w3/m34 (2026-08-01) retired the `BEX_TENANT_NAMESPACES`/`BEX_TENANT_SANDBOX_NAMESPACES` rollout gates plus the app-code `BEX_MAX_SERVICES`/`BEX_MAX_POSTGRES`/`BEX_MAX_KEYVALUES` caps, deleted the operator's legacy per-App NetworkPolicy construction, and made per-tenant namespaces the sole, unconditional model.
