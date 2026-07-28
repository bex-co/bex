# Tenant isolation: east-west network enforcement

> ⚠️ **DEPRECATED tenant-boundary mechanism (2026-07-27) — superseded by [ADR043: per-tenant namespace isolation](ADR043-tenant-namespace-isolation.md).** The [§Mechanism choice](#mechanism-choice) decision below ("Option B": a **shared apps namespace + label-scoped per-App NetworkPolicies** as the tenant boundary) is deprecated in favor of namespace-per-workspace. The implemented policies remain in production until ADR043 is implemented. **The rest of this doc is retained** — the Cilium node/cloud-metadata `egressDeny` (w7/m4), platform-side lockdown (t004), registry access control (w7/m8), and tenant container hardening (w7/m2) move _into_ the per-tenant namespace model; only the label-scoped boundary primitive is superseded. See ADR043 for the reversal rationale and its point-by-point engagement of this doc's objections to namespace-per-workspace.

**Status:** implemented (w7/m1); tenant-boundary mechanism deprecated 2026-07-27 — see the banner above and [ADR043](ADR043-tenant-namespace-isolation.md).

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

`deploy/gitops/base/tenant-node-egress.yaml` is a single platform-wide guard implemented as a **namespaced** `CiliumNetworkPolicy` in the apps namespace (a namespaced CNP only selects pods in its own namespace — "every tenant pod" holds only while all tenant pods share that namespace; under ADR043's per-tenant namespaces it must become a `CiliumClusterwideNetworkPolicy` or be stamped per-namespace). It selects every workspace-labeled tenant pod (`app.bex.co/workspace` Exists) and `egressDeny`-s:

- `toEntities: [host, remote-node]` — the nodes themselves (kubelet, nodePorts).
- `toCIDR: [169.254.0.0/16]` — the link-local / metadata range (belt-and-suspenders with the per-App `except`).

Two properties make this the right shape:

- **Deny precedence.** A Cilium `egressDeny` overrides any allow, including the per-App policy's `0.0.0.0/0` egress — so the block holds even though the per-App policy still allows the public internet.
- **Cilium-native is acceptable here because it is not per-App.** The per-App policies stay portable vanilla `networking.k8s.io/v1` (envtest can validate them). Only this one platform guard is Cilium-specific — acceptable because Cilium is the CNI on every bex cluster, and the guard needs the node entities no portable policy can name.

**Composition, verified live (2026-07-11; namespace-wide policy correction 2026-07-28).** A tenant pod carries both the per-App k8s NetworkPolicy (allow: DNS, same-workspace, internet-minus-RFC1918/link-local) and this CNP (deny: node, metadata). Together: DNS resolves, same-workspace and genuine external egress (`https://example.com` → 200) work, while `169.254.169.254` and the node's IP on `:10250` are blocked. The label-independent metadata CNP explicitly sets `enableDefaultDeny.egress: false`: it adds the deny without forcing unrelated non-App workloads in the shared namespace (CNPG, backup-purge Jobs, autoscaler) into a deny-all state. App Pods remain default-denied by their operator-managed NetworkPolicy, and deny precedence still blocks metadata. The block does **not** depend on the per-App policy carrying the link-local `except`: the CNP `egressDeny` blocks metadata on its own (confirmed against a pod with the pre-m4 per-App policy), so the prod cluster is protected the moment the CNP is applied, independent of the operator rollout.

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

## Tenant container hardening (w7/m2)

Every tenant container (Deployments, CronJobs, pre-deploy Jobs) is stamped with a hardening SecurityContext (`tenantSecCtx()`, `app_controller.go`): `allowPrivilegeEscalation: false`, **all Linux capabilities dropped**, RuntimeDefault seccomp. `runAsNonRoot` is deliberately absent — tenant images may run as root (PSS baseline, not restricted). A user-visible consequence of dropping ALL: `NET_BIND_SERVICE` is gone, so **a tenant container cannot bind ports below 1024 even as root** — stock port-80 images (nginx, httpd, whoami) crash-loop with `bind: permission denied` unless pointed at a high port. Deliberate posture (kept over Render parity); the operator diagnoses the crash loop and stamps the actionable cause — listen on `$PORT`, no ports < 1024 — onto the failed deploy's `failureReason` (w9/011, [ADR004-app-deployment.md](ADR004-app-deployment.md)).

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

## Repeatable verification (w6/m6)

The live matrix that script asserts is wired for repeatability two ways, so a regression fails CI or an on-demand check, not at a live penetration:

- **On demand** — `make verify-tenant-isolation` (from `lego/operator/`) runs `scripts/verify-tenant-isolation.sh` against the current kubeconfig.
- **In CI** — `scripts/gitops-validate.sh` (cluster-less, runs in `.github/workflows/gitops.yml`) asserts every platform namespace carries a default-deny-ingress policy (`podSelector: {}` + `policyTypes: [Ingress]`) and that no allow-list peer names the tenant apps namespace (`default`) — a manifest regression in `deploy/gitops/base/network-policies.yaml` fails before Argo CD applies it.

## Registry access control (w7/m8)

The network layer (above) puts a build-labeled pod inside Zot's allow-list, because the build Job must push to it. That same pod runs tenant-authored Dockerfile/CNB `RUN` steps in its network namespace — so the network layer alone leaves every tenant build able to enumerate (`/v2/_catalog`), pull (source disclosure), or overwrite (tag poisoning) every other tenant's image. w7/m8 closes that hole at the **application** layer: Zot requires a credential the tenant `RUN` steps cannot read.

### Threat model

- **Cross-tenant image enumeration** — `GET /v2/_catalog`, `GET /v2/<repo>/tags/list`: leaks every tenant's image/repo names.
- **Cross-tenant pull (source disclosure)** — `GET /v2/<repo>/manifests/…` + blobs: reconstructs another tenant's image.
- **Tag-overwrite poisoning** — `PUT` over an existing tag: a fresh autoscaled node then pulls the poisoned layer on its first deploy.

All three existed before w7/m8; the current Zot configuration denies anonymous access.

### Credential and process scheme (decision, closed by w2/m59)

`accessControl.repositories["**"].defaultPolicy: []` denies anonymous catalog, list, pull, and push. `bex-builder` remains an out-of-band bootstrap/admin identity for registry seeding, legacy shared-auth fallback, and the manager's cross-repository admission verifier. Normal tenant builds use a generated `app-<name>` identity whose Zot rule grants `read, create, update, delete` on exactly `<name>` and nothing else. The one platform-input exception is the exact `bex-cnb-builder` repository: its `defaultPolicy: ["read"]` lets authenticated kpack builds pull the shared builder, while the global `bex-builder` admin policy remains the only write path. A colliding App name cannot replace or remove that read-only rule. The legacy Secret name `reg-pull-<name>` is retained, but the same repository-scoped credential now serves two non-overlapping custody paths: Skopeo/Cosign receives it as a mounted build-phase credential, while a runtime Pod names it only as an `imagePullSecret` consumed by kubelet.

The Dockerfile pipeline is deliberately serial:

```text
private Git token       private-base credential       App-repo credential      signing key
       |                         |                            |                      |
clone init container -> BuildKit init container -> Skopeo push init container -> Cosign container
       |                   OCI archive only              pushed image            signature
       +------------------------- no shared PID lifetime ----------------------------+
```

- `clone` alone receives `GIT_AUTH_TOKEN`; it exits before tenant build code starts.
- BuildKit alone receives an optional external-registry credential for a private `FROM`. It never receives the platform/App output credential or signing key and exports an OCI archive instead of pushing.
- Skopeo alone receives the per-App output credential and the archive after BuildKit exits.
- Cosign alone receives the same per-App output credential plus the signing key, after Skopeo exits. Private-key signing uses `--tlog-upload=false`, so private image metadata is not published to Rekor.

BuildKit runs as root inside a Kubernetes Pod user namespace (`hostUsers: false`) with the Kubernetes 1.34 `UserNamespacesSupport` gate and containerd 2.3/runc 1.5 production prerequisites. Namespaced capabilities let the OCI worker mount snapshots without host privilege. Its default OCI process sandbox is enabled; `--oci-worker-no-process-sandbox` is forbidden by tests. Dockerfile `RUN` processes therefore cannot see or ptrace the BuildKit daemon PID. An unconfined seccomp/AppArmor profile remains limited to the BuildKit container inside that Pod user namespace; clone, push, sign, publish, and pre-deploy containers retain restricted contexts.

Every direct execution Pod also disables service-account-token automount, carries App/workspace labels, selects `bex.co/pool=tenant`, and runs in `bex-build` behind ingress/egress default-deny. Static publish and pre-deploy use the same common hardening source. kpack's generated build identity disables token automount and carries the same labels/tenant placement through its supported pod-template fields.

Repository-backed Docker services may bind a workspace credential for a private `FROM` (w6/m34). The operator copies only that docker config plus the BuildKit HTTP-registry resolver config into a deterministic build Secret; it is no longer merged with the output credential. Native, buildpack, and direct static-site sources reject this field. The App finalizer deletes the derived Secret.

Closure evidence on 2026-07-19 used a private authenticated Git source, a private Zot base image, and an adversarial Dockerfile that checked credential paths, environment/token absence, and `/proc` visibility before producing a signed, authenticated image that reached `App.status.phase=Running`. [`verify-build-isolation.sh`](../scripts/verify-build-isolation.sh) then passed 38/38 live assertions: named positive controls plus denial of metadata, kubelet, the Kubernetes API, real `bex-api`, cross-workspace traffic, and another Zot repository; own-repository push/read; and signed allow, unsigned deny, post-signature tamper deny, and platform non-selection. Cluster-less counterparts live in `scripts/gitops-validate.sh` and generated-object unit tests. This closes [ADR039](ADR039-operator-audit-and-platform-reuse.md) O-01/O-02; the upstream rootless no-process-sandbox warning remains the reason that mode must not return.

### Read policy (decision)

**Authenticated read** (`defaultPolicy: []`, anonymous denied). Pulls authenticate via per-App pull credentials (w7/m36, see below). The earlier shared `bex-puller` credential (w7/m8) has been superseded; the shared-credential residual recorded at line 204 is now closed.

#### Per-App pull credentials (w7/m36)

Each App that builds and pushes an image to Zot receives its own Zot user `app-<name>` and a per-repo ACL entry that restricts that user to `read, create, update, delete` on only `<name>`. The operator manages:

- **`zot-htpasswd`** Secret (in `bex-registry`): bcrypt entry `app-<name>:hash` added on App reconcile, removed on App delete. `bex-puller` is no longer present. Because the running zot never re-reads the file (see Credential activation below), removal alone does not deactivate the entry — `RevokeCreds` ends with a best-effort rate-limited zot bounce, so a revoked credential stops authenticating at the next bounce or zot restart, not instantly.
- **`zot-config`** Secret (in `bex-registry`): full Zot `config.json` managed by the operator; per-App entry `"<name>": {"policies": [{"users": ["app-<name>"], "actions": ["read", "create", "update", "delete"]}]}` added on App reconcile, removed on App delete. The exact `bex-cnb-builder` entry grants authenticated identities read only so kpack can fetch its shared input; the global `adminPolicy` grants `bex-builder` bootstrap/verifier and builder-publish access even when an exact repository rule matches. No shared tenant read user is in the global ACL.
- **`reg-pull-<name>`** Secret: `kubernetes.io/dockerconfigjson` with the standard `auth` field plus explicit username/password. In the App namespace kubelet consumes it as an `imagePullSecret`; the operator mirrors it into `bex-build` with an additional `config.json` filename for the App's Skopeo/Cosign phases. Tenant runtime containers never mount the Secret. Both copies are deleted by finalization.

**Closed residual (was ADR022:204):** `app-foo` can only act on the `foo` repository. It cannot read or overwrite `bar` because it is absent from `bar`'s exact rule and the `**` wildcard `defaultPolicy: []` provides no fallback. A compromised runtime container still cannot read the credential because kubelet, not the container, consumes `imagePullSecrets`. Live proof: `scripts/verify-per-app-registry-isolation.sh` and the own-repository/cross-repository controls in `scripts/verify-build-isolation.sh`.

**Credential activation (w9/m43):** Zot loads `/secret/htpasswd` and the `accessControl` config **once at process start** — it has no working hot reload under Kubernetes Secret mounts (verified against v2.1.18: a kubelet-style atomic symlink swap of both mounts activates nothing, and SIGHUP shuts the server down instead of reloading). Writing the Secrets alone therefore leaves a brand-new App's credential unusable — every kubelet pull answers `401` until the zot pod restarts (the 2026-07-17 prod incident: first deploy of any new App failed `ImagePullBackOff` until a manual `rollout restart statefulset/zot`). The operator closes the gap itself: before rolling any workload to a registry-hosted image, `registry.EnsureActive` probes zot with the App's own credential against the App's own repository (`GET /v2/<name>/tags/list` basic-auth — exercising both the htpasswd user and the per-repo ACL); while rejected it requeues without touching the workload, and past a 30s grace (anchored to the htpasswd write when known) it deletes the zot pod (rate-limited to one bounce per 2m across all Apps — the StatefulSet restarts it and the restart loads the new htpasswd + ACL entries). An accepted credential is memoized until rotation/revocation, so the steady state adds no I/O. Rotation deterministically re-enters this window (zot holds the old hash until bounced); revocation triggers the same rate-limited bounce best-effort. The needed `pods: get/list/delete` grant is scoped to `bex-registry` only (`deploy/gitops/base/operator-registry-rbac.yaml`), so the operator gains no pod-delete on tenant or platform namespaces.

### TLS posture (residual)

`registry.insecure=true` on the build push and HTTP-only Zot means the `bex-builder` and per-App credentials cross the cluster network in plaintext. This is acceptable **inside** a trusted cluster network (the same trust boundary the in-cluster control plane relies on) but is the reason Zot must be `ClusterIP`-only (never exposed) — the platform-side NetworkPolicy already denies tenant pods; this residual is about east-west between platform namespaces, not north-south. Fronting Zot with TLS + a public face is a prod-hardening follow-up; documented here so it is not silently inherited.

### Verification

- **Live (auth)** — `scripts/verify-registry-auth.sh` asserts anonymous `GET /v2/_catalog` and push are both refused (`401`), and that a credentialed build-from-git App round-trips.
- **Live (per-App isolation)** — `scripts/verify-per-app-registry-isolation.sh` asserts App A's credential cannot pull App B's image (expected `401`), proves App A can pull its own image, and verifies credential revocation on App delete (htpasswd + config entries removed).
- **Live (private Docker base)** — the w6/m34 CAPD run built and deployed a repository Docker service from an authenticated private `FROM`, then observed `401 Unauthorized` with an intentionally wrong bound credential; all fixture resources were removed.
- **Live (m59 execution boundary)** — `scripts/verify-build-isolation.sh` proves the private-source/private-base build and its adversarial process checks, per-repository auth, explicit network allows/denies, placement/token defaults, and signed/unsigned/tampered admission controls without printing credentials.
- **CI** — `scripts/gitops-validate.sh` asserts `deploy/gitops/base/zot.yaml` carries the auth stanza (`auth.htpasswd`, `accessControl` with `defaultPolicy: []`) and a pinned chart; a regression that removes auth fails CI before Argo applies it.

## Rejected options

- **vcluster**: rejected (DO_NOT_DO.md) — adds control-plane overhead without a meaningful network boundary.
- **CiliumNetworkPolicy for the per-App policies**: provides `toEntities: world` cleanly but breaks envtest portability and creates a hard Cilium dependency, so the per-App policies stay vanilla. A CNP _is_ used for the single cluster-wide node/metadata egress deny (w7/m4), where the `host`/`remote-node` entities express something no portable `ipBlock` can — this is the scoped adoption anticipated here, not a reversal.
- **Node-level firewall (iptables/nftables)**: outside w7 scope per DO_NOT_DO.md; would require root access and is fragile across upgrades.
