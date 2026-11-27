# ADR125 — Tenant egress leaves from a fixed, published set: Cilium's egress gateway on two Hetzner Floating IPv4s

**Status:** Proposed, 2026-11-23 (w21/m616 t001). The design rests on [ADR008 § Render parity first](ADR008-vision.md#render-parity-first); it is not a user decision. Building it on production waits for the user's go-ahead on two things t002 does: buying two Floating IPv4s (billed monthly to the Hetzner account) and changing every node's Cilium datapath (D9).

**Related decisions:** [ADR018 § Services & lifecycle](ADR018/services-lifecycle.md) holds the `Service outbound IPs` row this ADR corrects. [ADR022](ADR022-tenant-isolation.md) owns the tenant egress denies (node, metadata, free-plan SMTP) that must keep working. [ADR030](ADR030-pricing.md) §14 bills outbound bandwidth through the egress meter. [ADR019](ADR019-infra-credentials.md) is the credential inventory D7 adds to. Dedicated outbound IPs per workspace stay a non-goal ([`.pm/DO_NOT_DO.md`](../.pm/DO_NOT_DO.md), enterprise surfaces).

## Context

Render publishes a fixed set of addresses per region, and its customers put that set in external allowlists (Atlas, RDS, a partner API):

- "Render services send outbound traffic through specific sets of IP ranges depending on their region." "Each IP range uses CIDR notation." "Outbound IP ranges are shared across _all_ services in the same region." The one exception, "For workspaces created before January 23, 2022, services in the Oregon region do _not_ use a fixed set of outbound IP addresses", shows that the default is a fixed set ([outbound-ip-addresses](https://render.com/docs/outbound-ip-addresses), read 2026-11-23).
- When Render changes which addresses are in use, it "proactively resets affected TCP connections to public destinations" so the service "can reconnect using its new outbound IP". The causes include "Periodic rotation of in-use IP addresses in Render's default IP ranges"; they are "uncommon" ([outbound-connection-resets](https://render.com/docs/outbound-connection-resets), read 2026-11-23).
- Render announced its last change to the set 33 days before the new ranges went live and 68 days before the old set was retired ([changelog, 2025-09-24](https://render.com/changelog/upcoming-changes-to-outbound-ip-addresses)).

bex has no such set. `Service.OutboundIPs` (`lego/backend/internal/apps/outboundips.go`) answers the current ExternalIPs of the `bex.co/pool=tenant` nodes, because a tenant pod's egress is masqueraded to the public address of whichever node it runs on. Those nodes come and go with the autoscaler and with node rolls. On 2026-11-23, 8 of the 9 tenant nodes were less than 5 days old (read-only node listing, taken twice that day). An allowlist built from `outboundIps` a week earlier refused almost every instance. ADR018 marked the row ✅ on the premise that "Render's shared regional IPs drift the same way", which Render's pages contradict.

### Production as it stands (read 2026-11-23, read-only)

- **Where Cilium's values live.** No `cilium-config` and no chart values file is in `infra/` or `deploy/`. The install is the `install Cilium (CNI)` step of [`.github/workflows/app-cluster.yml`](../.github/workflows/app-cluster.yml): `helm upgrade --install cilium` with its settings as `--set` flags (cluster-pool IPAM, WireGuard with node encryption, `bpf.hostLegacyRouting=true`, `l7Proxy=true`, `devices=enp7s0`, `direct-routing-device=enp7s0`, the operator on the control plane). The chart is pinned in [`deploy/helm-artifacts.lock`](../deploy/helm-artifacts.lock) (cilium 1.19.5). The workflow runs on a push to `main` that touches `infra/clusterapi/overlays/hetzner-caph/**`, the Helm lock or the workflow itself, and on dispatch.
- **`cilium-config`:** `kube-proxy-replacement=false`, `routing-mode=tunnel` (VXLAN), `enable-ipv4-masquerade=true` with no BPF masquerading (iptables masquerade), `enable-host-legacy-routing=true`, `devices=enp7s0` (the private NIC only), `identity-allocation-mode=crd`, `enable-ipv6=false`.
- **kube-proxy** runs as a DaemonSet on all 16 nodes (3 control plane, 3 platform, 1 sandbox, 9 tenant).
- **No `ciliumegressgatewaypolicies.cilium.io` CRD** is installed, so no egress gateway policy can be applied, and a manifest of that kind in `deploy/gitops/` would fail Argo's sync.
- **Node pools:** `bex-platform` is fixed at 3 (min = max = 3), its nodes 7 to 11 days old. The tenant MachineDeployments autoscale (`bex-tenant-0` 1 to 8, `-xl` 0 to 4, `-xxl` 0 to 2). Every node is in `fsn1` (network zone `eu-central`).
- **Terraform applies on push.** `.github/workflows/infra.yml` plans and applies `infra/terraform/` on every push to `main` that touches it, so a Floating IP added there is bought by the push itself.

### What Cilium's egress gateway needs (Cilium 1.19 docs, read 2026-11-23)

[Egress Gateway](https://docs.cilium.io/en/v1.19/network/egress-gateway/egress-gateway/):

- "requires that both BPF masquerading and the kube-proxy replacement are enabled": `egressGateway.enabled=true`, `bpf.masquerade=true`, `kubeProxyReplacement=true`. It is not compatible with `kvstore` identity allocation (production uses `crd`), Cluster Mesh or CiliumEndpointSlice (neither is on).
- "The egress IP must be assigned to a network device on the node", and Cilium must run on that device (`--devices`).
- With one `egressGateway`, the first matching node by name is the gateway: one node, no failover. With `egressGateways`, each source endpoint is assigned to one gateway by its CiliumEndpoint UID, and any change to the list of matching nodes reassigns endpoints. "changing the gateway node will break existing egress connections."
- "When new pods are started, there is a delay before egress gateway policies are applied for those pods", during which their traffic can leave from the node's address.
- Cluster destinations (pods, nodes, the API server) inside `destinationCIDRs` are excluded from the SNAT automatically.

Hetzner Floating IPs (read 2026-11-23): €3.00 per month per IPv4, excluding VAT ([overview](https://docs.hetzner.com/cloud/floating-ips/overview)); assignable to one server at a time in the same network zone and reassignable at any time without powering the server off; configured on the server's interface as an extra `/32` ([persistent configuration](https://docs.hetzner.com/cloud/floating-ips/persistent-configuration/)).

## Decisions

### D1 — Mechanism: Cilium's egress gateway, with a Hetzner Floating IPv4 as the egress IP

A `CiliumEgressGatewayPolicy` sends tenant pods' traffic to public destinations through one gateway node, which source-NATs it to a Floating IPv4. bex already runs Cilium 1.19.5, and the policy selects pods by label, so it follows tenant workloads into every namespace without touching node routing.

Rejected:

- **A NAT VM behind a Hetzner network route (`0.0.0.0/0` via the VM).** It would move all node traffic (kubelet, image pulls, the control plane, the edge) onto one VM, and the nodes would have to give up their public addresses. That is a larger change to the cluster than a pod-selected policy, and the VM is a single point of failure that needs the same failover anyway.
- **Hetzner has no managed NAT gateway**, and keeping each tenant node's own address stable is impossible while the autoscaler creates and deletes nodes.
- **Cilium's multi-gateway list (`egressGateways`).** It spreads endpoints over both addresses, but every change to the list of gateways reassigns endpoints, so a gateway that comes back after a failure breaks a second set of connections. D4's switch moves endpoints once per failure.

### D2 — The set: two Floating IPv4s in `fsn1`, both published

- Terraform owns two `hcloud_floating_ip` (IPv4, home location `fsn1`) with delete protection and `prevent_destroy`, labelled with their role, as it owns the edge load balancer.
- One address is in use at a time. The other sits on the standby gateway (D3), so a failover switches to an address that is already routed to a live node, with no Hetzner API call in the incident path. This is Render's own model: a published set inside which the in-use address rotates.
- Both addresses are published from the start, so an allowlist built today admits either one. `outboundIps` answers `{"type": "shared", "ips": ["<a>/32", "<b>/32"]}` for every tenant service, read from configuration rather than from the node list (t005). Render's Outbound tab lists CIDR ranges; a `/32` is the same notation for a single address. A cluster with no set configured (the local mock) answers an empty `ips`.
- The set is shared by every tenant service, as Render's regional ranges are. Per-workspace dedicated addresses stay a non-goal.
- **Cost:** €6.00 per month excluding VAT (€7.14 with Germany's 19% VAT). No new servers.

### D3 — Gateways run on the platform pool

- The gateway nodes are two of the three `bex.co/pool=platform` nodes. That pool is not autoscaled (min = max = 3), and its nodes live about a week or more between rolls, against tenant nodes that turned over almost completely within 5 days. A gateway on the tenant pool would fail over, and reset connections, on most scale-ins.
- Every platform node carries both Floating IPs as `/32` on its public interface (`eth0`), set by the platform KubeadmConfigTemplate. Hetzner routes each address only to the server it is assigned to, so the extra address is inert on the other nodes, and any platform node can become a gateway without a per-node agent.
- Tenant packets transit the gateway's kernel only as forwarded traffic. Platform pods are not reachable through it: the policy matches only public destinations (D5), and the node, metadata and private-network denies of ADR022 still apply at the source.
- A dedicated gateway pool would isolate this traffic further, at about €4 to €8 per node per month. It is a follow-up if the platform pool's roll cadence proves too frequent.

### D4 — Failover: an `egress-gateway` controller switches the policy, then re-homes the address

Cilium's single-gateway policy has no failover. A small controller beside drain-guard (`lego/operator/cmd/egress-gateway`, a Deployment on the platform pool with leader election, its own ServiceAccount) owns the policy object:

1. It keeps one **active** gateway (a Ready platform node holding address A) and one **standby** (another Ready platform node holding address B), and writes the policy's `nodeSelector` and `egressIP` for the active one.
2. When the active node is NotReady for longer than a bound (60 s, t004 measures it), is being deleted, or is tainted for deletion or drain, it rewrites the policy to the standby node and address. This is a Kubernetes-only edit.
3. It then re-homes the lost address: it assigns it through the Hetzner Cloud API to another Ready platform node, which becomes the new standby. A failure here leaves egress working from the other address and is retried.
4. It publishes a failover generation, which the connection reset (D6) reads.

GitOps holds the controller's Deployment, RBAC and configuration (the addresses and their Hetzner ids, the pod selector, the excluded CIDRs). `scripts/gitops-validate.sh` guards that configuration, so the selector and the exclusions cannot be dropped silently.

### D5 — Which traffic uses the set

- **Sources:** pods carrying `app.bex.co/workspace`, in any namespace: App serving pods, cron runs and pre-deploy Jobs, which the operator labels in every namespace (the same selector as ADR022's node and metadata deny). Excluded: managed Postgres and Key Value pods (`cnpg.io/cluster`, `app.bex.co/keyvalue`), whose egress is bex's own backup traffic; `/v1/sandboxes` and agent-session sandboxes (`bex.co/sandbox-id`, `bex.co/agent-session`), which have their own egress policies ([ADR042](ADR042-sandbox-cluster-substrate.md), [ADR047](ADR047-cloud-coding-agent-sessions.md)). Static sites make no outbound traffic, as on Render.
- **Builds use the set too (amended 2026-11-27, w4/741).** Render's builds leave from the workspace's outbound addresses: "Services use dedicated IP sets for any outbound build traffic, such as package downloads. The exception is Docker image pulls, which use Render's shared outbound IP ranges", and "Workspaces without dedicated IPs use Render's default outbound IP ranges" ([dedicated-ips](https://render.com/docs/dedicated-ips), read 2026-11-27). bex has only the shared set, so on bex, as in a Render workspace without dedicated IPs, a build's requests leave from the published set, image pulls included. The build pods in `bex-build` already carry `app.bex.co/workspace`: BuildKit build Jobs (`build.AppBuildPodLabels`), kpack builds through their pod template ([ADR022](ADR022-tenant-isolation.md)) and static publish (`execution.PodLabels`). The same selector therefore covers them, with no second policy and no namespace exclusion. `bex-build`'s own NetworkPolicies (default deny, then DNS, Zot and public TCP 22, 80 and 443 only, plus the node and metadata denies) are still enforced at the source, before the redirect. BuildKit's base-image pulls run inside the build pod and leave from the set as well.
- **Open gap: the kubelet's image pulls.** The kubelet pulls an image-backed service's image on the tenant node as node traffic, which no pod-selected policy redirects, so that pull leaves from the node's own address. On Render it leaves from the shared ranges that a workspace without dedicated IPs publishes. A private registry that allowlists `outboundIps` therefore admits a build's pulls but refuses the kubelet's. Routing all node traffic through the set is the NAT design D1 rejected; a narrower fix for the pulls alone is the follow-up `w4/757`.
- **Destinations:** `0.0.0.0/0`, minus 10.0.0.0/8 (the private network, pods and services), 172.16.0.0/12, 192.168.0.0/16, 100.64.0.0/10 and 169.254.0.0/16. Cilium also excludes every cluster address by itself. Traffic to another service's public URL leaves through the set, as on Render; traffic over the private network does not, as on Render.

### D6 — Connections through a lost gateway are reset, not left hanging

A flow whose gateway changes is dead: the far end never sees its address again. A connection that is sending gets a reset from the far end on its next packet, but one that waits to receive (a database pool, a subscription) would hang until a keepalive expires. Render resets them. bex resets them on each tenant node: on a new failover generation, the node-local agent destroys the established TCP sockets of tenant pods whose remote address is a public destination (netlink `SOCK_DESTROY` in each pod's network namespace). The egress meter's DaemonSet is the candidate home, since it already runs on every tenant node with `NET_ADMIN` and `SYS_ADMIN` and maps each pod IP to its service. t004 confirms the mechanism on a real kernel and records the measured reset time.

### D7 — The Hetzner token

Reassigning a Floating IP needs a Hetzner Cloud API token, and Hetzner tokens are per project, Read or Read & Write, with no narrower scope. The controller reads the same `kube-system/hcloud` token the CCM and the CSI driver already hold, so the cluster gains a new reader of an existing credential, not a new credential. ADR019's inventory names the controller as its third reader. The controller calls only the Floating IP `assign` action, and its runbook says so.

### D8 — IPv4 only

The cluster runs with `enable-ipv6=false`, so tenant pods have no IPv6 egress to fix. Render's page names no IPv6 ranges. No Floating IPv6 is bought.

### D9 — What t002 changes on every node

The egress gateway needs datapath settings production does not have. All of them go into the `install Cilium (CNI)` step of `app-cluster.yml`, keeping every existing flag:

- `kubeProxyReplacement=true`, with `k8sServiceHost`/`k8sServicePort` set to the control-plane endpoint, and the kube-proxy DaemonSet removed once Cilium serves Services.
- `bpf.masquerade=true`, which replaces the iptables masquerade.
- `egressGateway.enabled=true`.
- `devices=eth0,enp7s0`. Today Cilium runs only on the private NIC. BPF masquerading and the egress IP both need the public one, or pod egress through `eth0` would not be masqueraded at all.

This is the riskiest part of the milestone: it changes how every node forwards Services and masquerades pods. It runs in its own `app-cluster.yml` run, with the previous flags as the rollback. The kind matrix (D10) passed with the same flags on 2026-11-23; the production run must still confirm Services, the edge NodePorts and the datastore listeners under kube-proxy replacement before kube-proxy is removed. `scripts/clusterapi-validate.sh` or `scripts/gitops-validate.sh` refuses a workflow that drops them.

### D10 — The egress meter and the SMTP deny

`scripts/verify-egress-gateway.sh` (w21/m616 t003) runs both checks on a three-node kind cluster with Cilium 1.19.5 and the D9 flags plus production's WireGuard node encryption and legacy host routing. The control-plane node plays the internet at `8.8.8.8`, the gateway node holds the egress IP as a `/32` on `eth0`, and the peers report the address they saw. It passed on 2026-11-23:

- A tenant pod on another node and one on the gateway node both left from the egress IP; a pod without `app.bex.co/workspace` kept its node's address, and with no policy a tenant pod left from its node.
- **SMTP deny.** `deny-free-plan-smtp-egress`, a `CiliumClusterwideNetworkPolicy` `egressDeny` to the `world` entity, is enforced at the source endpoint before the redirect: a free-plan pod could not open 587, reached 18001 from the egress IP, and a paid pod reached 587 from the egress IP.
- **Egress meter.** The meter (ADR030 §14) is a netfilter BPF program at `POST_ROUTING`, keyed by the local pods' IPs. Under legacy host routing the source node's hook still sees the redirected traffic with the pod's address, and BPF masquerading does its SNAT later, at the device. A 512-byte UDP payload counted exactly 544 bytes in total over every node's meter, on the source node and not again on the gateway, for a pod on another node and for one on the gateway node. The meter needs no change, and the gateway nodes need no meter of their own (it does not run on the platform pool today).
- **Build pods (w4/741).** A pod labelled as the operator labels a build pod, in `bex-build` under production's own `build-boundary.yaml` namespace and policies, left from its node before the policy and from the egress IP after it on TCP 443, and still could not open a port `allow-required-egress` does not list (18001). It passed on 2026-11-27, with every earlier check unchanged.

### D11 — Known divergence: a new pod's first packets

Cilium applies the policy to a new pod after a short delay (the agent's reconciliation, `egress-gateway-reconciliation-trigger-interval=1s` on production). A connection opened in that window leaves from the node's own address and is refused by an allowlist; the client's retry then leaves from the set. Render documents no such window. In the kind matrix a pod that connected as soon as its container started, 50 times at 50 ms intervals, left from the egress IP every time, the first attempt included. t006 measures it again on production, where nodes are busier. If it is long enough for an application's first connection to be refused, the follow-up is a policy that drops tenant pods' public egress that has not been redirected.

## Consequences

- An allowlist built from `outboundIps` keeps admitting a service through autoscaling and node rolls. It changes only when the set itself changes, which needs a Terraform change.
- One gateway node carries all tenant egress, build traffic included: package downloads, base-image pulls and BuildKit's layer-cache export to the `bex-build-cache` bucket. Its NIC bandwidth bounds tenant egress, and losing it costs one failover, which resets the connections it carried (D6). Load sharing across both addresses is out of scope (D1).
- The cluster gains a controller with write access to Floating IP assignment, through an existing credential (D7).
- The ledger row stays ◐ until t006 proves the set on production.

## Follow-ups (not in this ADR)

- Announcing a change to the set ahead of time, as Render did with 33 days' notice.
- A dedicated gateway pool, if the platform pool rolls too often (D3).
- The kubelet's image pulls for image-backed services through the set (D5's open gap, `w4/757`): node traffic, which a pod-selected policy cannot redirect.

## Tasks

`w21/m616`: t001 (this ADR and the ledger), t002 (Terraform and the Cilium flags, after the user's go-ahead), t003 (the policy, the kind matrix, the meter and the SMTP deny), t004 (the controller and the reset), t005 (`outboundIps` on every surface), t006 (production). `w4/741` adds build pods to D5 and to the kind matrix, then checks a build on production once `w21/m616` is live.
