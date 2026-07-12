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
| Cloud instance-metadata `169.254.169.254` | SSRF → node identity + cloud-init user-data theft (w7/m4) |
| The node's own IPs (kubelet `:10250`, nodePorts) | Pod→node access: kubelet API, host-bound services (w7/m4) |

The API front door (OpenFGA, Hydra introspection) was enforced by w1/m9. The pod network remained open — a compromised tenant container could bypass it entirely. The last two rows (cloud metadata, node-local endpoints) were left open by the m1 egress policy — its `0.0.0.0/0 except RFC1918` allow still reached link-local `169.254.0.0/16` and the nodes' public IPs; both are closed by w7/m4 (see [§node and cloud-metadata egress](#node-and-cloud-metadata-egress-w7m4)).

## Mechanism choice

Two options were evaluated:

### Option A — Namespace-per-workspace

Each workspace gets its own namespace; NetworkPolicies operate at the namespace boundary (the CNI's native unit). Pros: clean namespace-level quota and PSS; easy `networkPolicy.ingress.namespaceSelector`. Cons: major projector churn (every App CR moves namespace on first-login); URL/metrics/logs blast radius (bex-api namespace-scopes its watch); doubles operational surface without delivering a meaningfully stronger boundary than Option B for the threat above (kernel still shared). Deferred: if isolation demands grow beyond network, the next tier is microVM (Kata/Firecracker) — see [DO_NOT_DO.md](../.pm/DO_NOT_DO.md) isolation ladder.

### Option B — Label-scoped NetworkPolicies in a shared apps namespace ✓

A per-App `networking.k8s.io/v1` NetworkPolicy in the shared apps namespace (`BEX_CP_APPS_NAMESPACE`, default `default`) selects its app's pods by `app.bex.co/app` and uses `app.bex.co/workspace` label selectors for same-workspace allow rules.

**Chosen: Option B.** The projector already stamps tenant ids on App CRs; the operator can propagate a workspace label to pod templates with one additional line. No namespace changes, no URL/metrics/logs blast radius, no projector churn. The workspace label doubles as the cross-workspace deny selector — an app in workspace `tea-abc` can only reach other pods that also carry `app.bex.co/workspace: tea-abc`.

## Policy dialect

Standard `networking.k8s.io/v1` NetworkPolicy. Cilium (the live cluster's CNI) enforces these natively. Cilium-native `CiliumNetworkPolicy` is not used for the per-App policies to preserve portability and keep envtest (which cannot load Cilium CRDs) able to validate them. The "internet yes, cluster no" egress is expressed with `ipBlock cidr: 0.0.0.0/0 except: [RFC1918, link-local]` which Cilium also enforces correctly.

The **one** Cilium-native policy is the cluster-wide node/metadata egress deny (w7/m4, [§node and cloud-metadata egress](#node-and-cloud-metadata-egress-w7m4)): a portable `ipBlock` cannot name "the nodes" (their public IPs are not a contiguous owned CIDR and rotate with the autoscaler), so that one guard uses Cilium's `host`/`remote-node` entities. It is a single static platform manifest, not per-App, so it does not affect per-App portability or envtest.

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
      # public internet, minus in-cluster platforms (RFC1918 / CGNAT) and the
      # link-local cloud-metadata range (169.254.0.0/16)
      - ipBlock:
          cidr: 0.0.0.0/0
          except: [10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16, 100.64.0.0/10, 169.254.0.0/16]
```

The activator service (auto-sleep wake path) lives in `bex-system` and talks to the Kubernetes API only — it never initiates a direct connection to tenant pod IPs, so no activator-specific allow rule is needed.

## Node and cloud-metadata egress (w7/m4)

The m1 per-App egress rule above allows `0.0.0.0/0` minus RFC1918/CGNAT, which left two paths open on the prod Hetzner cluster. Both are SSRF/pod-escape primitives, and both were confirmed reachable from a live tenant pod (2026-07-11) before this milestone.

### Cloud instance-metadata (link-local `169.254.169.254`)

`169.254.0.0/16` is link-local and was not in the m1 `except` list, so a tenant pod could reach the cloud instance-metadata service. On Hetzner, `http://169.254.169.254/hetzner/v1/metadata` returns the node's instance-id, hostname, region/AZ, MAC address, and IPv6 config, and `.../hetzner/v1/userdata` returns the cloud-init user-data (which can carry bootstrap secrets) — both served HTTP 200 to a tenant pod. This is the Capital-One-class SSRF → metadata-theft path. It is closed two ways: (1) `169.254.0.0/16` is now in the per-App egress `except` list (portable, applies on any CNI), and (2) the platform CiliumNetworkPolicy below `egressDeny`-s the range (defense-in-depth if the per-App policy regresses).

### The node's own IPs (kubelet `:10250`, nodePorts)

A node's public IP is not RFC1918, so the m1 `0.0.0.0/0` allow reached it — a tenant pod could open kubelet `:10250` and any nodePort bound on the node's public interface (both confirmed reachable, on the node's public **and** private IP). The Hetzner cloud firewall only filters ingress from the internet; it does not see pod→node traffic that originates inside the cluster. A portable `ipBlock` cannot express "the nodes": their public IPs are not a contiguous owned CIDR and rotate as the autoscaler replaces machines. Cilium models the nodes as the `host` (the pod's own node) and `remote-node` (every other node) identities, so this is denied with a Cilium-native policy.

### Mechanism: a platform `CiliumNetworkPolicy` egressDeny

`deploy/gitops/base/tenant-node-egress.yaml` is a single cluster-wide `CiliumNetworkPolicy` in the apps namespace that selects every workspace-labeled tenant pod (`app.bex.co/workspace` Exists) and `egressDeny`-s:

- `toEntities: [host, remote-node]` — the nodes themselves (kubelet, nodePorts).
- `toCIDR: [169.254.0.0/16]` — the link-local / metadata range (belt-and-suspenders with the per-App `except`).

Two properties make this the right shape:

- **Deny precedence.** A Cilium `egressDeny` overrides any allow, including the per-App policy's `0.0.0.0/0` egress — so the block holds even though the per-App policy still allows the public internet.
- **Cilium-native is acceptable here because it is not per-App.** The per-App policies stay portable vanilla `networking.k8s.io/v1` (envtest can validate them). Only this one platform guard is Cilium-specific — acceptable because Cilium is the CNI on every bex cluster, and the guard needs the node entities no portable policy can name.

**Composition, verified live (2026-07-11).** A tenant pod carries both the per-App k8s NetworkPolicy (allow: DNS, same-workspace, internet-minus-RFC1918/link-local) and this CNP (deny: node, metadata). Together: DNS resolves, same-workspace and genuine external egress (`https://example.com` → 200) work, while `169.254.169.254` and the node's IP on `:10250` are blocked. Because `egressDeny` selecting an endpoint puts it into egress default-deny, this composition requires the per-App allow policy to be present — which every operator-provisioned tenant pod has. The block does **not** depend on the per-App policy carrying the link-local `except`: the CNP `egressDeny` blocks metadata on its own (confirmed against a pod with the pre-m4 per-App policy), so the prod cluster is protected the moment the CNP is applied, independent of the operator rollout.

**Local overlay.** The local CAPD mock cluster runs Calico, not Cilium, so the `cilium.io/v2` CRD is absent; the local kustomize overlay drops this manifest (`$patch: delete`). There, node IPs are Docker-network RFC1918 addresses already covered by the per-App `except`, and there is no cloud-metadata service, so the reachability matrix still holds without the CNP.

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

| Probe                                          | Direction | Expected |
| ---------------------------------------------- | --------- | -------- |
| workspace-A pod → workspace-B pod              | deny      | BLOCKED  |
| workspace-A pod → workspace-B Postgres         | deny      | BLOCKED  |
| workspace-A pod → workspace-B Valkey           | deny      | BLOCKED  |
| workspace-A pod → bex-api :8091                | deny      | BLOCKED  |
| workspace-A pod → OpenBao :8200                | deny      | BLOCKED  |
| workspace-A pod → Prometheus :9090             | deny      | BLOCKED  |
| workspace-A pod → zot :5000                    | deny      | BLOCKED  |
| workspace-A pod → `169.254.169.254` (metadata) | deny      | BLOCKED  |
| workspace-A pod → node IP `:10250` (kubelet)   | deny      | BLOCKED  |
| workspace-A pod → workspace-A private svc      | allow     | PASSES   |
| workspace-A pod → its own Postgres             | allow     | PASSES   |
| workspace-A pod → https://example.com          | allow     | PASSES   |
| Traefik → workspace-A pod via Ingress URL      | allow     | PASSES   |

The metadata + node DENY probes and the external-egress ALLOW counter-probe are asserted by `scripts/verify-tenant-isolation.sh` (w7/m4); its egress-probe pod carries a per-App policy so it faithfully models a real tenant pod under the CNP.

## Rejected options

- **vcluster**: rejected (DO_NOT_DO.md) — adds control-plane overhead without a meaningful network boundary.
- **CiliumNetworkPolicy for the per-App policies**: provides `toEntities: world` cleanly but breaks envtest portability and creates a hard Cilium dependency, so the per-App policies stay vanilla. A CNP _is_ used for the single cluster-wide node/metadata egress deny (w7/m4), where the `host`/`remote-node` entities express something no portable `ipBlock` can — this is the scoped adoption anticipated here, not a reversal.
- **Node-level firewall (iptables/nftables)**: outside w7 scope per DO_NOT_DO.md; would require root access and is fragile across upgrades.
