# Tenant isolation: east-west network enforcement

**Status:** implemented (w7/m1)

## Threat model

Before this milestone all tenant pods lived in a flat pod network. A tenant pod in workspace A could reach:

| Target | Risk |
| --- | --- |
| Another workspace's app pods (any port) | Lateral movement, data exfiltration |
| Another workspace's CNPG Postgres service | Direct database access, credential theft |
| Another workspace's Valkey service | Cache poisoning, secret leakage |
| bex-api internal tenant API (`:8091`) | Tenant enumeration, cross-tenant mutations |
| OpenBao (`:8200`) | Secret store access (auth-scoped, but reachable) |
| Prometheus (`:9090`) | Metric scraping across all tenants |
| zot registry (`:5000`) | Arbitrary image push/pull |

The API front door (OpenFGA, Hydra introspection) was enforced by w1/m9. The pod network remained open — a compromised tenant container could bypass it entirely.

## Mechanism choice

Two options were evaluated:

### Option A — Namespace-per-workspace

Each workspace gets its own namespace; NetworkPolicies operate at the namespace boundary (the CNI's native unit). Pros: clean namespace-level quota and PSS; easy `networkPolicy.ingress.namespaceSelector`. Cons: major projector churn (every App CR moves namespace on first-login); URL/metrics/logs blast radius (bex-api namespace-scopes its watch); doubles operational surface without delivering a meaningfully stronger boundary than Option B for the threat above (kernel still shared). Deferred: if isolation demands grow beyond network, the next tier is microVM (Kata/Firecracker) — see [DO_NOT_DO.md](../.pm/DO_NOT_DO.md) isolation ladder.

### Option B — Label-scoped NetworkPolicies in a shared apps namespace ✓

A per-App `networking.k8s.io/v1` NetworkPolicy in the shared apps namespace (`BEX_CP_APPS_NAMESPACE`, default `default`) selects its app's pods by `app.bex.co/app` and uses `app.bex.co/workspace` label selectors for same-workspace allow rules.

**Chosen: Option B.** The projector already stamps tenant ids on App CRs; the operator can propagate a workspace label to pod templates with one additional line. No namespace changes, no URL/metrics/logs blast radius, no projector churn. The workspace label doubles as the cross-workspace deny selector — an app in workspace `tea-abc` can only reach other pods that also carry `app.bex.co/workspace: tea-abc`.

## Policy dialect

Standard `networking.k8s.io/v1` NetworkPolicy. Cilium (the live cluster's CNI) enforces these natively. Cilium-native `CiliumNetworkPolicy` is not used to preserve portability and keep envtest (which cannot load Cilium CRDs) able to validate the policies. The one exception — "internet yes, cluster no" egress — is expressed with `ipBlock cidr: 0.0.0.0/0 except: [RFC1918]` which Cilium also enforces correctly.

## Label contract

| Label | Value | Stamped by | Propagated to |
| --- | --- | --- | --- |
| `app.bex.co/workspace` | `tea-<xid>` | control-plane projector (reconciler); postgres service (on Database CR create) | operator → Deployment pod template; database controller → CNPG `inheritedMetadata.labels`; keyvalue controller → StatefulSet pod template |
| `app.bex.co/app` | `<app-name>` | operator (existing) | Deployment pod template (existing) |

Apps created without the control-plane store (hand-applied App CRs, legacy mode) carry no workspace label. The operator skips the NetworkPolicy for these apps — they communicate freely, consistent with prior behavior. This is a documented legacy gap; all new Apps provisioned through bex-api carry the workspace label.

## Per-App NetworkPolicy shape (t003)

The operator creates a `NetworkPolicy` object (same name as the App, owned by the App CR) whenever an App carries the `app.bex.co/workspace` label.

```
podSelector:
  matchLabels:
    app.bex.co/app: <app-name>
policyTypes: [Ingress, Egress]
ingress:
  - from:
      # same-workspace pods (private services between apps in a workspace)
      - podSelector:
          matchLabels:
            app.bex.co/workspace: <workspace-id>
      # Traefik ingress controller (external traffic enters here)
      - namespaceSelector:
          matchLabels:
            kubernetes.io/metadata.name: traefik
egress:
  - ports: [{port: 53, protocol: UDP}, {port: 53, protocol: TCP}]
    to:
      - namespaceSelector:
          matchLabels:
            kubernetes.io/metadata.name: kube-system
  - to:
      # same-workspace pods and services
      - podSelector:
          matchLabels:
            app.bex.co/workspace: <workspace-id>
  - to:
      # public internet (not RFC1918 / CGNAT — blocks in-cluster platform services)
      - ipBlock:
          cidr: 0.0.0.0/0
          except: [10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16, 100.64.0.0/10]
```

The activator service (auto-sleep wake path) lives in `bex-system` and talks to the Kubernetes API only — it never initiates a direct connection to tenant pod IPs, so no activator-specific allow rule is needed.

## Platform-side lockdown (t004)

Defense-in-depth: even if the tenant-side policy regresses, platform namespaces refuse traffic from the tenant apps namespace.

| Platform namespace | Protected service | Explicitly denied from |
| --- | --- | --- |
| `bex-system` | operator + bex-api :8090/:8091 | apps namespace (tenant pods) |
| `bex-registry` | zot :5000 | apps namespace tenant pods (build Jobs allowed via label) |
| `secrets` | OpenBao :8200 | apps namespace (tenant pods) |
| `monitoring` | Prometheus :9090 | apps namespace (tenant pods) |

Each namespace gets an ingress NetworkPolicy that allows the known-legitimate callers and implicitly blocks everything else (including tenant apps pods) by virtue of `policyTypes: [Ingress]`.

## Datastore pod labels

- **CNPG Postgres**: The `DatabaseReconciler` reads `db.Labels["app.bex.co/workspace"]` (stamped by bex-api's postgres service on create) and sets it in the CNPG Cluster's `spec.inheritedMetadata.labels`, causing CNPG to propagate the label to all postgres pods. Same-workspace pods can then reach the database service via the egress allow rule.

- **Valkey**: The `KeyValueReconciler` reads `kv.Labels["app.bex.co/workspace"]` (stamped by bex-api's keyvalue service on create, w6/m4/t002) and adds it to the StatefulSet pod template labels, so a tenant's own App can reach its own managed Valkey instance over the same-workspace egress rule. If the KeyValue CR has no workspace label (hand-applied), no label is propagated — the Valkey pod is not reachable from tenant apps (default-deny), but it is also not accessible as a same-workspace service. This is the correct safe default for that case.

## Reachability matrix (t005)

| Probe                                     | Direction | Expected |
| ----------------------------------------- | --------- | -------- |
| workspace-A pod → workspace-B pod         | deny      | BLOCKED  |
| workspace-A pod → workspace-B Postgres    | deny      | BLOCKED  |
| workspace-A pod → workspace-B Valkey      | deny      | BLOCKED  |
| workspace-A pod → bex-api :8091           | deny      | BLOCKED  |
| workspace-A pod → OpenBao :8200           | deny      | BLOCKED  |
| workspace-A pod → Prometheus :9090        | deny      | BLOCKED  |
| workspace-A pod → zot :5000               | deny      | BLOCKED  |
| workspace-A pod → workspace-A private svc | allow     | PASSES   |
| workspace-A pod → its own Postgres        | allow     | PASSES   |
| workspace-A pod → https://example.com     | allow     | PASSES   |
| Traefik → workspace-A pod via Ingress URL | allow     | PASSES   |

## Rejected options

- **vcluster**: rejected (DO_NOT_DO.md) — adds control-plane overhead without a meaningful network boundary.
- **CiliumNetworkPolicy**: provides `toEntities: world` cleanly but breaks envtest portability and creates a hard Cilium dependency. Deferred unless the `ipBlock except` approach proves insufficient in practice.
- **Node-level firewall (iptables/nftables)**: outside w7 scope per DO_NOT_DO.md; would require root access and is fragile across upgrades.
