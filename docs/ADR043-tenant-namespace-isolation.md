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

### D8 — Datastores land in `<ws>` too (closing the App-only implementation defect)

_Added 2026-08-08 by `w7/m77` after a production incident. D1 has always said "App / Database / KeyValue land in `<ws>`" — but only the App half was ever implemented (`aca23a05`, `db7d95fc`). `lego/backend/internal/core/base.go` recorded the divergence in a doc comment ("Only Apps live in the per-tenant namespace; Databases/KeyValues/Secrets stay in `b.Namespace`") rather than as a known gap, so it read as design._

**The defect.** With Apps in `<ws>` and their datastores in the shared apps namespace, **every `fromDatabase`/`fromService` link created after the migration is broken**, in three independent ways that surface one at a time:

1. `resolveDatabaseRef` emits a bare `SecretKeyRef`. Kubernetes resolves `secretKeyRef` **same-namespace only**, and nothing mirrors datastore Secrets — so the pod fails `CreateContainerConfigError: secret "<dpg-id>-app" not found`.
2. `<ws>`'s allow set (D3) grants same-namespace, kube-system DNS, Traefik ingress, and internet-minus-RFC1918. A datastore ClusterIP is RFC1918, so it is **denied** — connection timeouts.
3. CNPG's `host` key holds the bare Service name (`<dpg-id>-rw`), resolvable only in-namespace — `could not translate host name`.

Pre-migration tenants silently worked because App and datastore were co-located in `default`, where none of the three applies. That is why the fleet migration (2026-07-28) appeared clean: it broke only links created _afterwards_.

**Decision.** A `Database`/`KeyValue` CR — and every object derived from it (CNPG `Cluster`, Valkey `StatefulSet`, connection/auth Secrets, `Service`s, PVCs, backup `CronJob`s) — lives in the creating workspace's `<ws>` hosting namespace, alongside that workspace's Apps. Co-location is what fixes all three legs at once: `secretKeyRef` resolves, `allow-same-namespace` suffices, and the CNPG short name is correct again. No secret mirroring, no cross-namespace network allow, and no FQDN rewriting is introduced — each of those would be a mechanism that exists only to paper over the split.

**D8.1 — Namespace resolution is by lookup, not by computation.** Creation computes the target (`AppNamespace(tenantID)`); every _read_ resolves the CR's actual namespace, exactly as Apps already do (`Base.appNamespaceByName` + `preferOwnWorkspaceNamespace`). A caller holding the CR uses `d.Namespace`/`kv.Namespace` directly; only a caller holding just a name performs the lookup.

This is the permanent contract, not a transitional fallback — so there is no removal condition and no flag day. It also makes the legacy set free: a CR still in the shared namespace resolves to the shared namespace through the same code path, with no `if legacy` branch anywhere. bex-api already holds cluster-wide `get/list/watch` on `databases`/`keyvalues` (`lego/operator/config/api/rbac.yaml`), and the per-tenant `bex-tenant-api` ClusterRole already grants Secrets/pods/pods-log in each `<ws>`, so **no RBAC change is required**.

**D8.2 — The operator's Secret informer is the blocking constraint.** `NamespacedSecretCacheOptions` pins the manager's Secret informer to exactly one namespace, and both datastore reconcilers touch Secrets through the _cached_ client (`keyvalue_controller.go` creates the connection + auth Secrets and declares `Owns(&corev1.Secret{})`; `database_controller.go` copies the pooler Secret). Widening that informer cluster-wide is **not** available: the operator's ClusterRole deliberately omits Secrets (w7/m7), so a cluster-wide Secret list fails and prevents the entire shared cache — including the App controller — from starting.

Resolution: tenant-namespace Secret access moves to the **uncached** client, which is the precedent the App path already set (`buildPlaneClient()`), and the KeyValue `Owns(Secret)` watch is replaced with one that does not require a cluster-wide Secret informer. Authority is already in place — the per-tenant `bex-tenant-operator` ClusterRole grants Secrets in each `<ws>`.

**D8.3 — The hosting allow set must grow for datastore workloads.** D3's four allows were written for application pods. CNPG and Valkey need paths none of them grant, and omitting any one reproduces w7/m33's failure mode (CNPG stalling in bootstrap because a policy blocked its control traffic) inside the tenant namespace:

| Direction | Peer | Why | Where |
| --- | --- | --- | --- |
| ingress | `cnpg-system` | the CNPG operator reconciling its Cluster | `allow-datastore-control-ingress` |
| ingress | `bex-system` | pg-/kv-sni-proxy public access + bex-api's direct query path (`postgres/query.go`) | `allow-datastore-control-ingress` |
| ingress | `monitoring` | Prometheus scrape of datastore metrics | `allow-datastore-control-ingress` |
| egress | kube-apiserver | CNPG's instance manager; the w7/m33 lesson | `allow-cnpg-apiserver-egress` (GitOps) |

The three ingress peers are one additive hosting-regime k8s NetworkPolicy in `allowNetworkPolicies`, scoped by peer rather than by port: these are platform namespaces that already carry their own default-deny ingress, so enumerating ports buys little, while getting the list wrong fails silently as a stalled bootstrap or an empty metrics series.

**The apiserver egress is deliberately not bex-api's to grant.** It has no portable k8s NetworkPolicy representation (the apiserver is not a pod, and its address is a cluster-specific ClusterIP-to-node mapping), and D6's admission confines bex-api's CiliumNetworkPolicy authority to `<ws>-sandbox` namespaces. So it is a cluster-wide `CiliumClusterwideNetworkPolicy` in GitOps — the same split the sandbox regime already uses for the one sanctioned inbound path, and the right one on the merits: reachability to the control plane should not be delegated to the component that provisions tenant namespaces.

That policy must set **`enableDefaultDeny.egress: false`**. In Cilium any policy carrying a positive egress rule enables default-deny egress for every endpoint it selects, and this one selects CNPG pods cluster-wide — including un-migrated clusters in the shared namespace, which has no default-deny. Without the flag it would restrict those clusters to the single apiserver rule, cutting DNS and S3 backups: the cutover would break precisely the tenants it had not touched yet. `scripts/gitops-validate.sh` pins the shape.

**D8.4 — Backups: the operator owns a per-namespace ObjectStore.** The Barman Cloud `ObjectStore` is namespaced and today exists as a single GitOps-installed object (`deploy/gitops/charts/barman-cloud-objectstores/tenant-postgres.yaml`), referenced by name from `database_controller.go`. GitOps cannot enumerate namespaces created at workspace-create time, so the **operator** converges the ObjectStore in each namespace where it projects a Database, and mirrors the `BEX_DB_BACKUP_S3_SECRET` credential there. The operator already owns the whole backup contract (`BEX_DB_BACKUP_*` are its env), so this keeps one owner; giving it to the namespace reconciler instead would split the contract across two processes with two env sources.

The S3 destination-prefix contract is unchanged: the Barman plugin writes under `<destination>/<cluster-name>`, and cluster names are globally unique `dpg-<xid>` ids, so per-namespace ObjectStores share one prefix without collision. Placing the backup credential in the tenant namespace does not expose it to the tenant: tenant workloads have no Kubernetes API access and run with `automountServiceAccountToken: false` (w7/m2), so a namespace-local Secret is not reachable from tenant code.

**D8.5 — Existing tenants are fixed by migration, not by the code change.** Because resolution is by lookup, already-provisioned datastores keep working exactly as before — but an App in `<ws>` whose datastore is still in the shared namespace **stays broken** until that datastore moves. The code change fixes new resources; existing affected tenants are fixed only by the cutover in [`docs/runbooks/datastore-namespace-cutover.md`](runbooks/datastore-namespace-cutover.md). Datastore CRs cannot be moved in place (a PVC is namespace-bound), so the cutover recreates each CR **under its existing name** in `<ws>` and restores its data — preserving the name is what keeps the App's `secretKeyRef` valid with no App-side edit.

### D9 — The orphan prune is scoped to one control-plane identity

Namespace **provisioning** is inherently namespaced, but the orphan prune is **cluster-scoped**: `pruneOrphans` lists every namespace carrying the managed-by label and deletes any whose workspace is absent from **its own** database. The App-CR projector does the same thing (`Reconciler.ReconcileOnce` lists cluster-wide and deletes every App whose id is absent from its own database), so both projectors share the defect and both share the fix. With one control plane per cluster that is correct. With two, it is mutual destruction — and bex runs many, because every workstream's `.pm/wN/dev-N` harness raises its own bex-api with its own bex-db against the shared CAPD mock cluster. Observed live in both directions (`.pm/w3/017.md`, 2026-08-08): each harness deleted the other's freshly provisioned `tea-…` namespaces within one 60-second resync. The `dev-N` isolation design predates this pruner and was silently broken by it.

Every managed namespace **and every projected App CR** therefore carries `app.bex.co/control-plane`, set from `BEX_CP_IDENTITY` (default `production`), and a control plane prunes only what it owns. The App half matters at least as much as the namespace half: app ids are per-database xids, so another control plane's Apps are never accounted for, and before the fix two harnesses deleted and recreated each other's App CRs on **every** 30-second resync — a perpetual flap of the tenant Deployments behind them rather than a one-shot wipe.

A malformed `BEX_CP_IDENTITY` is fatal at startup. It is projected into a label, and `ReconcileOnce` collects per-workspace apply errors without ever exiting — so an invalid value would leave the process healthy, provisioning nothing, while its prune kept running.

Ownership is deliberately **asymmetric** about the unlabeled case. A namespace labelled with our identity is ours. A namespace with **no** identity label is ours only when we run under the default identity — namespaces provisioned before this change carry no label, and production must still reclaim those orphans, while a `dev-N` harness must never delete a namespace it cannot prove it created. Both halves fail in the safe direction: a dev harness leaks rather than deletes, and production keeps its pre-existing reclamation behavior. The unlabeled set is transitional — `ensureNamespace` re-stamps every namespace it owns on the next pass, so it only shrinks.

A namespace labelled for **another** identity is never pruned, including by production. Leaking a stale dev namespace is recoverable; deleting a live tenant's is not.

**Rollout requires no production change.** `BEX_CP_IDENTITY` is unset in production — the default is `production`, and the unlabeled-is-mine rule means the prune keeps working on pre-existing namespaces from the first pass, before any of them has been re-stamped. Only the `dev-N` harnesses set the variable, so the blast radius of this change on production is one additional label converging onto namespaces the reconciler already rewrites every pass.

**Ownership is asymmetric between adoption and pruning, deliberately.** An unlabeled object must stay _adoptable_ by any identity — that is how a pre-m39 object gains its label at all — while it is only _prunable_ by the default identity. Converging an object owned by a **different** identity is refused outright on both projectors: without that, a harness raised from another control plane's `bex-db` dump would inherit its tenant rows, re-stamp those objects to its own identity, and then legitimately prune them later, bypassing the ownership guard in one hop.

This is a scoping fix, not a security boundary: the identity label is tenant-unreachable but is not an authorization mechanism. D6's admission policy remains what confines the reconciler's authority — and it admits the new label, because [`bex-api-namespace-admission.yaml`](../deploy/gitops/base/bex-api-namespace-admission.yaml) asserts specific label **values** (`managed-by == bex-controlplane`, the Pod Security set, the workspace/regime/name agreement) and never a closed label **set**. Adding a label to a namespace the reconciler already owns is therefore admitted; a policy that had enumerated the exact label set would have failed every namespace write in production the moment this shipped.

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
- **Per-tenant `ResourceQuota`/`LimitRange`** replace the `BEX_MAX_SERVICES` / `BEX_MAX_POSTGRES` / `BEX_MAX_KEYVALUES` app-code caps — same intent, enforced one layer down. _Correction (2026-08-08, D8):_ this held only for Services until `w7/m77`. While datastore CRs lived outside `<ws>`, the `count/databases.app.bex.co` and `count/keyvalues.app.bex.co` quota dimensions were never charged, so Postgres/Key Value creation was uncapped in practice — an honest gap recorded at the time in `docs/ADR006-bex-api.md` § Per-workspace resource caps and `store.quotaForPlan`. D8 makes the dimensions bite.
- **ADR022 is marked deprecated** (banner added, linking here). Its label-scoped per-App policies were removed from the operator once migration completed (w3/m34); its other mechanisms are retained.
- **ADR042 D3 is refined**: the shared `bex-sandbox` namespace becomes per-tenant `<ws>-sandbox` (consistent with D1). ADR042 D3/D4 were amended to match on 2026-07-27.
- **Migration**: new workspaces provision `<ws>` + `<ws>-sandbox` (both eagerly at workspace create — pillar 5 is re-opened and `w3/m32` consumes the sandbox namespaces; an empty namespace costs nothing and keeps one uniform lifecycle); existing workspaces migrated. The label `app.bex.co/workspace` is retained as a pod label within namespaces (useful for selection/metrics), it just stops being the _boundary_. **Migration complete**: the fleet was fully migrated by 2026-07-28, and w3/m34 (2026-08-01) retired the `BEX_TENANT_NAMESPACES`/`BEX_TENANT_SANDBOX_NAMESPACES` rollout gates plus the app-code `BEX_MAX_SERVICES`/`BEX_MAX_POSTGRES`/`BEX_MAX_KEYVALUES` caps, deleted the operator's legacy per-App NetworkPolicy construction, and made per-tenant namespaces the sole, unconditional model. _Correction (2026-08-08):_ "migrated" covered **Apps only**. Datastores were left in the shared namespace, which silently broke every `fromDatabase`/`fromService` link created after the cutover — see D8 and [`docs/runbooks/datastore-namespace-cutover.md`](runbooks/datastore-namespace-cutover.md).
