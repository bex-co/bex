# Tenant isolation: east-west network enforcement

> ⚠️ **DEPRECATED tenant-boundary mechanism (2026-07-27) — superseded by [ADR043: per-tenant namespace isolation](ADR043-tenant-namespace-isolation.md).** The [§Mechanism choice](#mechanism-choice) decision below ("Option B": a **shared apps namespace + label-scoped per-App NetworkPolicies** as the tenant boundary) is deprecated in favor of namespace-per-workspace. **Removed, not just deprecated**: w3/m34 (2026-08-01) deleted the operator's per-App NetworkPolicy construction entirely once the fleet was fully migrated — the mechanism below is historical record, not live behavior. **The rest of this doc is retained** — the Cilium node/cloud-metadata `egressDeny` (w7/m4), platform-side lockdown (t004), registry access control (w7/m8), and tenant container hardening (w7/m2) moved _into_ the per-tenant namespace model; only the label-scoped boundary primitive is superseded. See ADR043 for the reversal rationale and its point-by-point engagement of this doc's objections to namespace-per-workspace.

**Status:** implemented (w7/m1); tenant-boundary mechanism deprecated 2026-07-27 and removed 2026-08-01 (w3/m34) — see the banner above and [ADR043](ADR043-tenant-namespace-isolation.md).

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

### Option B — Label-scoped NetworkPolicies in a shared apps namespace ✓ (superseded — see ADR043)

A per-App `networking.k8s.io/v1` NetworkPolicy in the shared apps namespace (`BEX_CP_APPS_NAMESPACE`, default `default`) selected its app's pods by `app.bex.co/app` and used `app.bex.co/workspace` label selectors for same-workspace allow rules.

**Chosen: Option B** (at the time). The projector already stamped tenant ids on App CRs; the operator propagated a workspace label to pod templates with one additional line. No namespace changes, no URL/metrics/logs blast radius, no projector churn. The workspace label doubled as the cross-workspace deny selector — an app in workspace `tea-abc` could only reach other pods that also carried `app.bex.co/workspace: tea-abc`. **Superseded by ADR043** (per-tenant namespaces); this mechanism no longer runs.

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

### Image pulls from the node network (w5/171)

A tenant image may come from any public registry, as on Render (`store.ValidateImage` refuses only private and reserved addresses, localhost, single-label names and cluster or local-network suffixes such as `.svc`, `.cluster.local`, `.local` and `.internal`). containerd pulls on the node network, where none of the pod egress policies above apply. A tenant-chosen registry name could therefore resolve, at pull time, to the metadata service, and a validation-time DNS check cannot stop that rebinding. So every node bootstrap (`infra/clusterapi`) loads an nftables table that rejects link-local (`169.254.0.0/16`) destinations from containerd's own cgroup (`system.slice/containerd.service`) only. Pod traffic, CSI, kubelet and cloud-init, which may read the metadata service, are unaffected. A containerd drop-in re-applies the table on every start, because the rule binds the cgroup of the start that loaded it, and a failed load fails the start. `scripts/clusterapi-validate.sh` refuses a bootstrap without it. containerd also uses HTTPS for any registry without its own `hosts.toml`, and only bex's Zot has one, so a plain-HTTP endpoint such as the metadata service fails at the TLS handshake even before this rule. Because CAPI applies a bootstrap only to new machines, the pools were rolled onto it before bex-api dropped the fixed allowlist (w5/174, 2026-10-24). `BEX_PUBLIC_IMAGE_REGISTRIES=false` remains available for a cluster whose nodes lack the deny.

## Free-plan SMTP egress (w8/m127)

Render refuses outbound TCP 25, 465 and 587 from free instances ([render.com/docs/free](https://render.com/docs/free)); bex does the same, decided 2026-10-20.

- **Label.** The operator stamps `app.bex.co/free-plan: "true"` (`execution.LabelFreePlan`) on every pod a free-plan App runs: the Deployment template (`appPodLabels` → `appPolicyLabels`), cron runs (`cronPodSpec`) and pre-deploy Jobs (`predeploy.Options.FreePlan`). A held release's served template follows a plan change too (`syncPolicyLabels`). An empty tier is the free plan, as for auto-hibernation (`isFreeApp`). Paid Apps carry no label, so their templates are byte-identical to before.
- **Deny.** `deny-free-plan-smtp-egress` in `deploy/gitops/base/tenant-node-egress.yaml` is a `CiliumClusterwideNetworkPolicy` selecting that label, with `egressDeny` to entity `world` (the internet, so an in-cluster same-workspace relay stays reachable) on TCP 25/465/587. Deny precedence beats the namespace's `allow-internet-egress`. `enableDefaultDeny.egress: false` keeps it subtractive: DNS, 443, 2525 and same-workspace traffic behave as before. `scripts/gitops-validate.sh` pins its exact shape.
- **Plan changes.** Upgrading drops the label and downgrading restores it; the pods roll with the new template, as a tier change already does for its resources. The operator rollout adds the label to existing free Apps once, rolling those Apps' pods; paid Apps do not roll.
- **Paid plans.** 587 and 2525 are open. 25 and 465 stay blocked for every plan by the hosting provider; the user decided on 2026-10-20 not to request the unblock, and no bex config implies they could open ([ADR018](ADR018-render-parity.md) "Outbound SMTP ports").

## Free-plan private-network ingress (w8/m130)

Render's Free plan does not support "Receiving private network traffic" ([render.com/docs/free](https://render.com/docs/free)); bex does the same, in every workspace.

- **Deny.** `deny-free-plan-private-ingress` in `deploy/gitops/base/tenant-node-egress.yaml` is a `CiliumClusterwideNetworkPolicy` selecting the `app.bex.co/free-plan: "true"` label (see above) with `ingressDeny` from every endpoint carrying `app.bex.co/workspace` (any namespace). Deny precedence beats the per-namespace same-workspace allow. Traefik, the activator, kubelet probes (`host`) and platform scrapers carry no workspace label and stay allowed, so the public URL, custom domains, the free-tier wake and health checks are untouched. `enableDefaultDeny.ingress: false` keeps it subtractive. `scripts/gitops-validate.sh` pins its exact shape.
- **Plan changes.** The label follows the plan (`syncPolicyLabels`), so an upgrade opens the internal address and a downgrade closes it with the pod roll a tier change already causes. No allowlist or canary: the rule applies to every workspace at once.
- **Surfaces.** [ADR041](ADR041-service-addresses.md) D5 and [ADR018](ADR018-render-parity.md) record that `internalAddress` stays readable on a free web service while the dashboard Connect → Internal section and Blueprint `fromService` `host`/`hostport` say the Free plan takes no private traffic.

## Platform-side lockdown (t004)

Defense-in-depth: even if the tenant-side policy regresses, platform namespaces refuse traffic from the tenant apps namespace.

| Platform namespace | Protected service | Explicitly denied from |
| --- | --- | --- |
| `bex-system` | operator + bex-api :8090/:8091 | apps namespace (tenant pods) |
| `bex-registry` | zot :5000 | apps namespace tenant pods (build Jobs allowed via label) |
| `secrets` | OpenBao :8200 | apps namespace (tenant pods) |
| `monitoring` | Prometheus :9090 | apps namespace (tenant pods) |

Each namespace gets an ingress NetworkPolicy that allows the known-legitimate callers and implicitly blocks everything else (including tenant apps pods) by virtue of `policyTypes: [Ingress]`.

### Static edge aliases are an authorization boundary (w7/m54)

NetworkPolicy does not constrain what Traefik itself may route. Traefik's global ExternalName support is required for the operator's cross-namespace static-server and maintenance aliases; if a tenant-facing identity could mutate a Service or Ingress, it could turn that feature into a platform/tenant-network routing primitive. The effective boundary is therefore layered:

- `bex-api`, `bex-ssh-gateway`, and tenant workload ServiceAccounts have no create/update/patch/delete on Services or Ingresses in hosting namespaces;
- the operator remains the trusted reconciler;
- fail-closed [`operator-alias-admission.yaml`](../deploy/gitops/base/operator-alias-admission.yaml) accepts ExternalName create/update only from that manager and only for an App-owned `bex-static-* → bex-static-server:8080` or `bex-maintenance-* → bex-activator:8888` shape.

This closes the control-plane bypass adjacent to network isolation. It does not add an arbitrary upstream proxy to static sites: their rewrites remain object-key rewrites inside the fixed S3 origin ([ADR029](ADR029-static-sites.md)). `scripts/verify-static-site-security.sh live` enumerates real ServiceAccounts/RoleBinding subjects, checks every write verb by impersonation, and submits both admitted and hostile server-side dry-run aliases.

### Client-identity headers a tenant app receives (w8/m123)

Render puts every web service behind Cloudflare, which overwrites `CF-Connecting-IP` and `True-Client-IP` with the real client, and Render tells apps to trust those headers for rate limits, allowlists and audit logs. Traefik rewrites only its own `X-Forwarded-*`/`X-Real-IP` set. Without more, a migrated app would trust caller-controlled values.

The `web` and `websecure` entrypoints therefore run the `bex-client-ip-headers` local plugin (`deploy/gitops/charts/traefik-plugins/clientipheaders/`) before every router. It covers every App, static site, custom domain and platform host. With the peer being the TCP source after PROXY protocol:

| Request | `True-Client-IP` | `CF-Connecting-IP`, `CF-IPCountry`, other `CF-*` | `X-Client-IP` |
| --- | --- | --- | --- |
| From any non-Cloudflare peer (every `*.onbex.co` host, a DNS-only custom domain, or a direct hit on the LB) | the peer | removed | removed |
| From a Cloudflare edge peer (a custom domain behind the tenant's own Cloudflare proxy) with a valid `CF-Connecting-IP` | that `CF-Connecting-IP` | kept, since Cloudflare set them | removed |
| From a Cloudflare edge peer whose `CF-Connecting-IP` is not an IP | the peer | removed | removed |

Underscore spellings (`True_Client_IP`, `CF_Connecting_IP`) are stripped too, because CGI-style frameworks (Rack, PHP, WSGI) read them as the same variable as the hyphenated header. A trusted edge's `CF-Connecting-IP` is reduced to its first value. Because the reference sits on the entrypoint, a missing `Middleware` fails every router on `web`/`websecure`, not one App. The `Middleware` therefore ships in the Traefik release itself (`extraObjects`), beside the CRDs it needs. `X-Forwarded-For`/`X-Real-IP` keep Traefik's behavior: they name the peer, and a forged value is replaced. The plugin's Cloudflare ranges equal `core/proxiedhosts.go`'s, and `scripts/gitops-validate.sh` fails when they drift. bex's own inbound-IP allowlist still matches the TCP source, never `CF-Connecting-IP` (m150 Decision 2). `X-Forwarded-Server` (the Traefik pod name) still reaches apps; it identifies no client.

## Tenant container hardening (w7/m2)

**Two contracts (amended w6/m149, 2026-10-20).** Render parity is the top product principle ([ADR008 § Render parity first](ADR008-vision.md#render-parity-first)), so drop-ALL is no longer a posture kept over parity. The policy is versioned and persisted per service ([ADR089](ADR089-render-container-compatibility.md) D2):

- **Legacy, existing services:** `strict-v1` drops all capabilities; `image-v1` drops all and adds `CHOWN, DAC_OVERRIDE, SETUID, SETGID`. These rows never change on upgrade, sync or redeploy. Moving one is an explicit operator-run migration (w6/m149 t008).
- **New Docker/image/Dockerfile services:** `render-v1` drops all and adds exactly Render's measured set, `CHOWN, DAC_OVERRIDE, FOWNER, SETGID, SETUID, SYS_CHROOT`. Docker-default capabilities that Render lacks (`NET_BIND_SERVICE`, `KILL`, `SETPCAP`, `SETFCAP`, `FSETID`, `MKNOD`, `AUDIT_WRITE`, `NET_RAW`) are not added. It is the creation default in every workspace (w6/m149 t009), including when `BEX_IMAGE_COMPATIBILITY_WORKSPACES` is unset (w6/090); `off` opts a deployment out. Static sites and native runtimes keep `strict-v1`.
- **Residuals:** seccomp stays RuntimeDefault on every policy, a named divergence because Render applies none (`io_uring`, unprivileged user namespaces); relaxation is w6/085. **2026-10-21 (w6/m150):** `render-v1` now runs bex's Localhost profile `profiles/bex-render-v1.json` (RuntimeDefault plus user-namespace and `io_uring` syscalls; [ADR089](ADR089-render-container-compatibility.md)). Tenant runtime pods run without a pod user namespace, as on Render; `hostUsers: false` as bex-only defense in depth is w6/086, gated on Hetzner `hcloud-volumes` idmap proof. **2026-10-21 (w6/m151):** `render-v1` runtime pods now run in a pod user namespace in every workspace ([ADR089](ADR089-render-container-compatibility.md)).

The original w7/m2 text follows; its "all Linux capabilities dropped" now describes `strict-v1` only. Every tenant container (Deployments, CronJobs, pre-deploy Jobs) is stamped with a hardening SecurityContext (`tenantSecCtx()`, `app_controller.go`): `allowPrivilegeEscalation: false`, **all Linux capabilities dropped**, RuntimeDefault seccomp. `runAsNonRoot` is deliberately absent — tenant images may run as root (PSS baseline, not restricted). **Ports below 1024 are bindable despite the dropped `NET_BIND_SERVICE` (corrected w6/m149, 2026-10-19).** Every pod gets `net.ipv4.ip_unprivileged_port_start=0` from the pinned containerd 2.x CRI default. A live `strict-v1` root process bound `:80`, as on Render ([container-privileges.md](render-artifacts/container-privileges.md)). The earlier claim that "a tenant container cannot bind ports below 1024 even as root" does not hold on this runtime. bex-api accepts any port 1-65535, and `scripts/clusterapi-validate.sh` keeps node bootstrap from overriding the default. The operator's crash-loop diagnosis still tells the user to listen on `$PORT` (w9/011, [ADR004-app-deployment.md](ADR004-app-deployment.md)).

## Build reach to the owning workspace's datastores (w8/m129, 2026-10-29)

**Decision source.** The user decision recorded in [.pm w8/m129's unblock record](../.pm/w8/m129/README.md) on 2026-10-29 (`/unblock w8`) applies the Render-parity rule (AGENTS.md, [ADR008](ADR008-vision.md#render-parity-first)). It adopts the scoped design. A build may reach only its own workspace's datastores. `fromDatabase`/`fromService` values enter the build only through the secret mount, never as Job-spec literals. Cross-workspace reach stays forbidden. That record also lets the worker write this threat review without waiting for a separate sign-off. It requires the worker to stop if the review finds that the scoped design cannot keep cross-workspace reach closed. The review below finds that it can.

**Why.** Render's free-plan guides run migrations at the end of the build command against the internal `DATABASE_URL` ([Deploy a Django App](https://render.com/docs/deploy-django) and `render-examples/django`'s `build.sh`; [Rails](https://render.com/docs/deploy-rails-6-7): "If you're running on the Free instance type, you can include a database migration step in your build command"). On a free service bex refuses the pre-deploy command, as Render does (w8/m131). Before this change bex dropped every `ValueFrom` variable from the build (`BuildLiterals`), and `bex-build`'s default-deny allowed only DNS, Zot and public ports. Every such app failed its first build ([w8/158](../.pm/w8/done/158.md)).

**Mechanism.**

1. **Values through the mount only.** `projectNativeBuildEnv` resolves each `spec.env` `SecretKeyRef` from the App's own namespace (`readBuildEnvRefs`). Only the referenced key is copied, into the App-owned merged env Secret the build already mounts (`<app>-native-env`, w4/m93). The refs merge after the envFrom sources, as a container-level env entry overrides envFrom at runtime. A native build reads that Secret through a BuildKit secret mount. A Dockerfile build turns its keys into build args inside the BuildKit container (w1/m178). The Job spec names only the Secret. Protected operator Secrets are refused at read time, as for every other build source. kpack is unchanged: it cannot take Secret refs (w1/121), so its builds get neither the values nor the reach.
2. **Wait for the database.** A Blueprint creates the database and the service together. A build therefore waits, as `BuildQueued`, while a referenced connection Secret does not exist yet or a referenced Database is still `Pending`/`Provisioning`. It does not run `migrate` against a server that is not up yet.
3. **Two operator-owned NetworkPolicies per building App.** They are converged at dispatch and removed at the build's terminal observation (`reconcileBuildDatastoreReach`):
   - `bex-build/bld-<app>-datastore-egress` selects this App lifetime's build pods only: `app.bex.co/build=<app>`, `app.bex.co/app-uid=<uid>`, `app.bex.co/component=build` and `app.bex.co/workspace=<ws>`. It allows egress to one peer: `namespaceSelector {kubernetes.io/metadata.name: <App namespace>}` AND the isolation-aware pod selector, on TCP 5432/6379/6380. The finalizer reclaims it like the other build artifacts (component `execution-network-policy`).
   - `<ws>/<app>-build-ingress` selects the same isolation-aware pod set in the workspace namespace. It admits only those exact build pods (`namespaceSelector {kubernetes.io/metadata.name: bex-build}` AND the same four labels) on the same ports. The App owns it (controller reference), so it is deleted with the App.
   - **Isolation-aware target.** An ordinary App's build reaches only pods without `app.bex.co/network-isolation`, as `allow-same-namespace` does. An App in a protected environment reaches only pods of its own environment (w8/m136).
   - **Eligibility.** The App namespace must be the workspace's canonical namespace (namespace name == `app.bex.co/workspace`, the rule `operator-workload-admission.yaml` enforces). The legacy shared namespace holds several workspaces, so it never gets a grant. A build that runs in the App namespace (`BEX_BUILD_NAMESPACE` unset) already has `allow-same-namespace`, so it gets nothing extra.

**Threat review.**

| Threat | Analysis | Verdict |
| --- | --- | --- |
| A build in workspace A reaches workspace B's datastore | A connection needs both halves. A's egress names exactly one namespace, A's own, through `kubernetes.io/metadata.name`, which the API server sets and nobody can relabel. B's namespace holds only B's own ingress grants, keyed to B's App UIDs. Neither policy can name the other workspace. | Closed by construction (`TestBuildDatastoreReachIsOwnWorkspaceOnly` evaluates both directions) |
| A build pod forges another App's labels | Only the operator (Job templates) and kpack (from operator-written Images) create pods in `bex-build`. bex-api has no create verb there, and tenants have no Kubernetes identity. The App UID is server-minted. `bex-build-job-shape` admission pins every Job author's template. | Closed |
| Tenant RUN steps spoof another pod's IP with `NET_RAW`/`NET_ADMIN` | Those capabilities stay in the pod's user and network namespaces. Cilium derives identity from pod labels, not packet contents, and drops egress whose source IP is not the endpoint's own. | Closed (Cilium invariant; `verify-build-isolation.sh` remains the live check) |
| A build reaches its own workspace's protected-environment datastores | The isolation-aware selector mirrors `allow-same-namespace`. | Closed |
| A build reaches own-workspace non-datastore services | Port-limited to 5432/6379/6380, so only a tenant's own pod listening on those ports in its own workspace is reachable. That is within the approved boundary (own workspace). Private services on other ports stay unreachable from builds. | Accepted (own workspace) |
| A build reaches platform namespaces, nodes or metadata | No peer names them. `allow-required-egress` still excludes private ranges, and `deny-build-node-and-metadata-egress` is a Cilium deny that overrides any allow. | Unchanged |
| The credential leaks through logs or the image | The operator never logs values; errors name variables and Secrets only. A native build mounts the value as a BuildKit secret, so it is not in any layer. A Dockerfile build gets it as a build arg, which Docker records in image history when a `RUN` uses it. That is Render's documented build-arg translation, and it already applied to the service's own Secret env (w1/m178). The image lives in the App's own ACL-scoped Zot repository. A build command that echoes the value prints it into the workspace's own build log, as on Render. | Accepted (Render parity, same workspace) |
| A stale grant outlives the build | Both policies select only this App lifetime's build pods, so they grant nothing while no build runs. They are removed at the terminal observation (best-effort, retried on the next terminal pass), and the finalizer removes them with the App. | Accepted |
| Same-named Apps in two workspaces collide on the build-namespace policy name | Service ids are globally unique. On collision `checkOwnedArtifact` refuses to overwrite another lifetime's policy and the build fails closed. | Closed |

**Residuals.** Two things are left open. First, private-service reach from a build (`fromService` `host`/`hostport` to a web or private service) is not granted; only datastores are. Second, kpack builds stay credential-less (w1/121). Both are recorded in ADR018.

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
| workspace-A build → its own Postgres (w8/m129) | allow     | PASSES   |
| workspace-A build → workspace-B Postgres       | deny      | BLOCKED  |

The metadata + node DENY probes and the external-egress ALLOW counter-probe are asserted by `scripts/verify-tenant-isolation.sh` (w7/m4); its egress-probe pod carries a per-App policy so it faithfully models a real tenant pod under the CNP.

## Repeatable verification (w6/m6)

The live matrix that script asserts is wired for repeatability two ways, so a regression fails CI or an on-demand check, not at a live penetration:

- **On demand** — `make verify-tenant-isolation` (from `lego/operator/`) runs `scripts/verify-tenant-isolation.sh` against the current kubeconfig.
- **In CI** — `scripts/gitops-validate.sh` (cluster-less, runs in `.github/workflows/gitops.yml`) asserts every platform namespace carries a default-deny-ingress policy (`podSelector: {}` + `policyTypes: [Ingress]`) and that no allow-list peer names the tenant apps namespace (`default`) — a manifest regression in `deploy/gitops/base/network-policies.yaml` fails before Argo CD applies it.
- **Static edge** — `scripts/gitops-validate.sh` pins the operator alias admission/RBAC shape; `scripts/verify-static-site-security.sh live` proves the API-server allow/deny behavior across every canonical hosting namespace and refuses Apps outside that protected set.

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

Every direct execution Pod also disables service-account-token automount, carries App/workspace labels, selects `bex.co/pool=tenant`, and runs in `bex-build` behind ingress/egress default-deny. **Amended 2026-10-29 (w8/m129):** the default-deny keeps one operator-generated exception. While a native or Dockerfile build runs, its own pods may reach datastore ports in the App's own workspace namespace, and nothing else. See [§ Build reach to the owning workspace's datastores](#build-reach-to-the-owning-workspaces-datastores-w8m129-2026-10-29). Static publish and pre-deploy use the same common hardening source. kpack's generated build identity disables token automount and carries the same labels/tenant placement through its supported pod-template fields.

Repository-backed Docker services may bind a workspace credential for a private `FROM` (w6/m34). The operator copies only that docker config plus the BuildKit HTTP-registry resolver config into a deterministic build Secret; it is no longer merged with the output credential. Native, buildpack, and direct static-site sources reject this field. The App finalizer deletes the derived Secret.

Closure evidence on 2026-07-19 used a private authenticated Git source, a private Zot base image, and an adversarial Dockerfile that checked credential paths, environment/token absence, and `/proc` visibility before producing a signed, authenticated image that reached `App.status.phase=Running`. [`verify-build-isolation.sh`](../scripts/verify-build-isolation.sh) then passed 38/38 live assertions: named positive controls plus denial of metadata, kubelet, the Kubernetes API, real `bex-api`, cross-workspace traffic, and another Zot repository; own-repository push/read; and signed allow, unsigned deny, post-signature tamper deny, and platform non-selection. Cluster-less counterparts live in `scripts/gitops-validate.sh` and generated-object unit tests. This closes [ADR039](ADR039-operator-audit-and-platform-reuse.md) O-01/O-02; the upstream rootless no-process-sandbox warning remains the reason that mode must not return.

### Read policy (decision)

**Authenticated read** (`defaultPolicy: []`, anonymous denied). Pulls authenticate via per-App pull credentials (w7/m36, see below). The earlier shared `bex-puller` credential (w7/m8) has been superseded; the shared-credential residual recorded at line 204 is now closed.

#### Per-App pull credentials (w7/m36)

Each App that builds and pushes an image to Zot receives its own Zot user and a per-repo ACL entry that restricts that user to `read, create, update, delete` on only its repository. Unlabeled/legacy Apps keep `app-<name>` on repo `<name>` and Secret `reg-pull-<name>` (this section's original contract). Apps carrying `app.bex.co/workspace` use `app-<tea-id>-<name>` on repo `<tea-id>/<name>` and Secret `reg-pull-<tea-id>-<name>` ([ADR074](ADR074-workspace-scoped-artifact-identity.md), w2/m75); dual-read grants the new user READ on the legacy repo until a verified migration tombstones it. The operator manages:

- **`zot-htpasswd`** Secret (in `bex-registry`): bcrypt entry `app-<name>:hash` added on App reconcile, removed on App delete. `bex-puller` is no longer present. The running zot hot-reloads the file once the kubelet projects the write (see Credential activation below), and `RevokeCreds` ends with a best-effort reload nudge, so a revoked credential stops authenticating within seconds, with no zot restart (at the kubelet's next periodic sync, about a minute, if the nudge fails).
- **`zot-config`** Secret (in `bex-registry`): full Zot `config.json` managed by the operator; per-App entry `"<name>": {"policies": [{"users": ["app-<name>"], "actions": ["read", "create", "update", "delete"]}]}` added on App reconcile, removed on App delete. The exact `bex-cnb-builder` entry grants authenticated identities read only so kpack can fetch its shared input; the global `adminPolicy` grants `bex-builder` bootstrap/verifier and builder-publish access even when an exact repository rule matches. No shared tenant read user is in the global ACL.
- **`reg-pull-<name>`** Secret: `kubernetes.io/dockerconfigjson` with the standard `auth` field plus explicit username/password. In the App namespace kubelet consumes it as an `imagePullSecret`; the operator mirrors it into `bex-build` with an additional `config.json` filename for the App's Skopeo/Cosign phases. Tenant runtime containers never mount the Secret. Both copies are deleted by finalization.

**Closed residual (was ADR022:204):** `app-foo` can only act on the `foo` repository. It cannot read or overwrite `bar` because it is absent from `bar`'s exact rule and the `**` wildcard `defaultPolicy: []` provides no fallback. A compromised runtime container still cannot read the credential because kubelet, not the container, consumes `imagePullSecrets`. Live proof: `scripts/verify-per-app-registry-isolation.sh` and the own-repository/cross-repository controls in `scripts/verify-build-isolation.sh`.

**Credential activation (w9/m43; amended by w4/m347, 2026-11-14):**

- **History (w9/m43).** Verified against zot v2.1.18: zot loaded `/secret/htpasswd` and the `accessControl` config only at process start. Its file watchers reacted only to in-place `Write` events, so a kubelet-style atomic `..data` symlink swap of either mount activated nothing, and SIGHUP shut the server down instead of reloading. A brand-new App's credential was therefore unusable until the zot pod restarted (the 2026-07-17 prod incident: every first deploy failed `ImagePullBackOff` until a manual `rollout restart statefulset/zot`). w9/m43 had the operator restart zot itself. Past a 30s grace it deleted the zot pod, rate-limited to one restart per 2m across all Apps.
- **Why it was replaced (w4/m347).** Restarting the only registry replica was the activation mechanism, so it ran for every service anyone created.
  - **Slow first builds.** Every new App waited at least the 30s grace plus whatever remained of the platform-wide cooldown. Production round 180 (2026-11-12, `bex-canary`) saw a first build sit `==> Preparing registry credentials` at least 4 m 46 s, and 6 m 30 s before it started at all. Its repeat build started in 59 s.
  - **Broken pushes for other tenants.** Each restart interrupted every tenant's pushes and pulls. Round 190 saw a static site's in-flight upload fail with `EOF` and then `connection refused` for at least 10 s, longer than skopeo's 3-attempt retry budget. The build passed only because the Job's backoff reran it whole, and the deploy took 3 m 14 s instead of about 1 m 20 s.
- **Decision: activate by hot reload, never by restart.**
  - **zot reloads both files itself.** The pinned zot v2.1.22 (`deploy/gitops/base/zot.yaml`) watches both files and also fingerprint-polls them every second (identity, size, mtime): `HTPasswdWatcher` in `pkg/api/htpasswd.go` and `HotReloader` in `pkg/cli/server/config_reloader.go`. The poll catches the kubelet's symlink swap. A config reload re-applies `http.accessControl` (`Config.UpdateReloadableConfig`) and recompiles the policy.
  - **Upstream tests prove the Secret-mount case.** `TestHTPasswdWatcherAtomicReplace` ("reload htpasswd after symlink retarget (k8s ..data style)") and `TestConfigReloaderKubernetesConfigMapUpdate` (build tag `search`) pass at tag `v2.1.22`, re-run for this decision. The same files at `v2.1.18` have no poll and react to `Write` only, which matches the original finding.
  - **What remains is kubelet projection.** The kubelet re-projects a Secret volume on its periodic pod sync, about a minute. It does so at once when the pod object changes: an UPDATE re-runs the pod sync, which remounts its Secret volumes. So every credential write ends with `registry.nudgeZot`, a merge patch of one annotation, `bex.co/registry-credentials-written: <time>`, on the zot pod. The registry keeps serving throughout.
  - **The nudge is best effort.** If it fails, for example during the rollout window before the RBAC grant below lands, the write still activates at the next periodic sync. Revocation, rotation and the snapshot and sandbox-meter users use the same nudge, so a revoked credential stops working within seconds instead of at the next restart.
- **The probe gate stays.** Before a build Job is created, or a workload rolls to a registry-hosted image, `registry.EnsureActive` probes zot with the App's own credential against the App's own repository. The probe is `GET /v2/<repo>/tags/list` with basic auth, which exercises both the htpasswd user and the per-repo ACL. While the credential is rejected, the App waits in `RegistryCredsPending` (`==> Preparing registry credentials`) and is re-probed every 3 s (was 10 s). Each rejected probe may re-nudge zot, at most every 5 s, which covers a nudge the kubelet handled before its Secret cache saw the write. An accepted credential is memoized until rotation or revocation, so the steady state adds no I/O.
- **The restart is a last resort that alerts.** A credential still rejected 3 min after its write is far past the kubelet's own sync period, which means hot reload is broken (for example a zot downgrade, or a chart change that mounts the Secrets by `subPath`).
  - Only then does the operator delete the zot pod, still rate-limited to one restart per 2 min. It logs `registry hot reload did not activate a per-app credential; restarted zot as a last resort` and increments `bex_registry_activation_restarts_total`.
  - `RegistryCredentialActivationRestarted` alerts on any increase. The Cluster capacity dashboard graphs it.
  - The restart is kept rather than removed so a broken reload degrades to the old slow path instead of a permanent `RegistryCredsPending`.
- **Storage settings reload live too (w4/649, 2026-11-14).** w8/m156 added a second restart trigger, `ensureStorageActive`: the operator restarted zot once per change to the config's `storage` block (GC, retention), including on a fresh install's first App, on the belief that zot reads it only at start. At v2.1.22 that is not so, and the restart is removed. A storage change rides the same `zot-config` write as the ACL, so it ends with the same reload nudge, and nothing restarts zot for it.
  - **What zot reloads.** `HotReloader.reloadConfig` (`pkg/cli/server/config_reloader.go`) stops the background tasks, calls `Controller.LoadNewConfig` and starts them again. `Config.UpdateReloadableConfig` (`pkg/api/config/config.go`) copies `storage.gc`, `dedupe`, `gcDelay`, `gcInterval` and `gcTimeWindow`, and `RunGCTasks` (`pkg/api/controller.go`) rebuilds the GC and retention task from the new values.
  - **The one condition: retention needs the metadata database zot opened at start.** `UpdateReloadableConfig` copies `storage.retention` only when the running config already enables retention (`isRetentionEnabledInternal`: some policy keeps tags by count or age), because `InitMetaDB` opens that database only at start. Turning retention on in a zot that started without it would need a restart. bex never asks for one: every operator-written config since the first (w7/m36, July 2026) keeps tags by `mostRecentlyPushedCount`, and zot cannot start before the operator has written `zot-config`, which it mounts. So no operator-made storage change needs a restart, and none is kept.
  - **`rootDirectory` is outside the reloadable set.** zot logs `config changes are outside the reloadable set and need a restart to take effect` for it. The operator writes a constant there and never changes it.
  - **A broken reload is still caught.** zot applies a config file all at once, so a reload that stopped working would also strand the next ACL write, which trips the credential last resort and `RegistryCredentialActivationRestarted` above.
  - **Measured on the local CAPD mock cluster (2026-11-14, zot `v2.1.22` at the production digest).** Writing the m156 rule order (`^live-.*` first), a retention count of 6 and `gcInterval: 2h` into the live `zot-config` and nudging `zot-0` reloaded within about 1 s: `loaded new configuration settings` showed the new retention and a `GCInterval` of 2 h, the next GC pass logged `retained by mostRecentlyPushedCount:6 policy` and `…:10 policy` for `live-` tags, and there was no "outside the reloadable set" warning. The pod's UID and restart count were unchanged. The original block was then written back the same way.
  - The `bex.co/zot-storage-active` annotation an earlier operator left on `zot-config` is no longer read.
- **RBAC.** `deploy/gitops/base/operator-registry-rbac.yaml` adds `pods: patch` with `resourceNames: [zot-0]` (the chart's single StatefulSet replica) to the existing `bex-registry`-scoped `pods: get/list/delete`. The operator still gains no pod verbs in tenant or platform namespaces.
- **Isolation is unchanged.** Hot reload changes when an entry takes effect, not what it grants. App A's credential still gets `401`/`403` on App B's repository (`scripts/verify-per-app-registry-isolation.sh`). `lego/operator/internal/registry/verify_reload_test.go` asserts this, plus first-probe activation, six concurrent new Apps, revocation without a restart, and the fallback, against a model of zot that serves only what the kubelet last projected.
- **Measured on the local CAPD mock cluster (2026-11-14, zot `v2.1.22` at the production digest).** A throwaway htpasswd user and ACL entry were written into the live `zot-htpasswd` and `zot-config` Secrets, and probed through NodePort 30500 every 0.5 s:
  - **No nudge:** accepted 16.6 s after the write (the kubelet's periodic sync, which can take up to about a minute).
  - **With the annotation nudge:** accepted 1.2 s after the write.
  - **Six users written back to back, each followed by a nudge:** all accepted within 4.9 s of the first write. That time includes the twelve serial Secret writes.
  - **Isolation:** one user's credential on another's repository answered `403`.
  - **Revocation:** removing an entry and nudging refused it (`401`) within 0.8–1.1 s.
  - **No restart:** the zot pod's UID and restart count were unchanged throughout. Its log shows `loaded htpasswd file` and `loaded new configuration settings` for each write, and no "outside the reloadable set" warning.
  - **Not re-measured:** a restart's effect on an in-flight push or pull, which would have meant restarting the shared mock registry. Round 190's production failure is the evidence.

### TLS posture (residual)

`registry.insecure=true` on the build push and HTTP-only Zot means the `bex-builder` and per-App credentials cross the cluster network in plaintext. This is acceptable **inside** a trusted cluster network (the same trust boundary the in-cluster control plane relies on) but is the reason Zot must be `ClusterIP`-only (never exposed) — the platform-side NetworkPolicy already denies tenant pods; this residual is about east-west between platform namespaces, not north-south. Fronting Zot with TLS + a public face is a prod-hardening follow-up; documented here so it is not silently inherited.

### Verification

- **Live (auth)** — `scripts/verify-registry-auth.sh` asserts anonymous `GET /v2/_catalog` and push are both refused (`401`), and that a credentialed build-from-git App round-trips.
- **Live (per-App isolation)** — `scripts/verify-per-app-registry-isolation.sh` asserts App A's credential cannot pull App B's image (expected `401`), proves App A can pull its own image, and verifies credential revocation on App delete (htpasswd + config entries removed).
- **Live (private Docker base)** — the w6/m34 CAPD run built and deployed a repository Docker service from an authenticated private `FROM`, then observed `401 Unauthorized` with an intentionally wrong bound credential; all fixture resources were removed.
- **Live (m59 execution boundary)** — `scripts/verify-build-isolation.sh` proves the private-source/private-base build and its adversarial process checks, per-repository auth, explicit network allows/denies, placement/token defaults, and signed/unsigned/tampered admission controls without printing credentials.
- **CI** — `scripts/gitops-validate.sh` asserts `deploy/gitops/base/zot.yaml` carries the auth stanza (`auth.htpasswd`, `accessControl` with `defaultPolicy: []`) and a pinned chart; a regression that removes auth fails CI before Argo applies it.

## Node image cache (w4/426, 2026-10-27)

Tenant pods share nodes, and so share each node's container image cache. With `imagePullPolicy: IfNotPresent`, a kubelet that already holds an image starts it without presenting the pod's `imagePullSecrets` to the registry. Re-verifying credentials for cached images (KEP-2535, `KubeletEnsureSecretPulledImages`) is alpha and off in Kubernetes v1.34. A workspace that names another workspace's private `image@sha256:…` could therefore run it on a node that had pulled it.

**Decision:** every tenant container (the runtime Deployment and cron pods, `tenantImagePullPolicy`, and the pre-deploy Job) uses `imagePullPolicy: Always`, digest-pinned or not. Tags already did, for codex-security #29. Each container start then has the registry check the pod's own credentials against the manifest. Layers stay cached, so the cost is a manifest round trip per start, and Docker Hub does not count manifest `HEAD` requests against its pull limit. The trade-off is the one tags already had: while a tenant's registry is unreachable, a restarting instance cannot start. Platform-owned images (exports, edge cache, health probe) keep `IfNotPresent`, since no tenant chooses them. Revisit when `KubeletEnsureSecretPulledImages` is on by default in the node fleet's Kubernetes version.

## Rejected options

- **vcluster**: rejected (DO_NOT_DO.md) — adds control-plane overhead without a meaningful network boundary.
- **CiliumNetworkPolicy for the per-App policies**: provides `toEntities: world` cleanly but breaks envtest portability and creates a hard Cilium dependency, so the per-App policies stay vanilla. A CNP _is_ used for the single cluster-wide node/metadata egress deny (w7/m4), where the `host`/`remote-node` entities express something no portable `ipBlock` can — this is the scoped adoption anticipated here, not a reversal.
- **Node-level firewall (iptables/nftables)**: outside w7 scope per DO_NOT_DO.md; would require root access and is fragile across upgrades.

## Bounded request reading at the tenant edge (w8/207, 2026-10-28)

Both HTTP entrypoints, `web` and `websecure`, explicitly set `transport.respondingTimeouts.readTimeout: 6000s` in base values, inherited by production. Traefik's [entrypoint contract](https://doc.traefik.io/traefik/reference/install-configuration/entrypoints/) applies this deadline to the entire incoming request, including its body; the 60-second default was truncating ordinary slow uploads. Render's [first-party comparison](https://render.com/docs/render-vs-vercel-comparison#long-running-requests-and-jobs) documents HTTP responses lasting up to 100 minutes. Using that same finite budget for incoming reads is Bex's parity-sufficient bound; the precise split between Render's header/body/response budgets has not been measured.

The longer deadline also lets a slow-header or slow-body client retain a connection longer. The decision for this parity fix is a finite bound of 100 minutes on both entrypoints without a new per-IP concurrency cap: the cap has no measured requirement here and could reject legitimate clients behind shared NAT. Existing edge resource limits and the finite deadline remain; per-client flood controls can be evaluated from observed traffic separately. Traefik exposes no independent header-read deadline in this configuration. `writeTimeout` and `idleTimeout` retain their existing behavior.

The entrypoints also carry platform Ingresses: `lego/operator/config/api/ingress.yaml` and the dashboard's Traefik Ingress use this edge. bex-api retains its own 10 second ReadHeaderTimeout and 30 second ReadTimeout (`cmd/api/main.go:newHTTPServer`), which continue limiting its API request bodies independently. The shared entrypoint change does not raise an application's own server limits or a customer's upstream proxy limits.

`scripts/gitops-validate.sh` checks the actual rendered flags for base and production on both entrypoints, so a chart-schema change or prod override cannot silently restore the 60 second default. Hosted upload acceptance follows GitOps sync.
