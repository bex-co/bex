# w1 · m7 — Prod hardening: network · secrets · images

**Worker:** worker1 **Goal:** Close the deliberate demo shortcuts so the cluster is more than a demo: firewall the API, pin the operator image, encrypt secrets + node traffic, and get a stable LB IP on a private network. **Status:** todo

## Tasks (any order — mostly independent)

| id   | title                                             | est | depends_on |
| ---- | ------------------------------------------------- | --- | ---------- |
| t001 | Firewall the app nodes (Hetzner firewall, TF)     | 30m | —          |
| t002 | Immutable operator image (SHA-pin, not `:latest`) | 30m | —          |
| t003 | Secrets at rest (sealed-secrets / SOPS)           | 30m | —          |
| t004 | Encrypt node-to-node (Cilium WireGuard)           | 25m | —          |
| t005 | Private network + stable LB IP (Traefik LB)       | 30m | t004       |
| t006 | OpenBao Raft snapshot backup (etcd-backup CronJob pattern) | 30m | —          |

## Definition of done

The kube-API is firewalled (not internet-exposed); the operator runs a SHA-pinned image; repo/registry/Hetzner creds are encrypted at rest; node traffic is WireGuard-encrypted; Traefik has a stable LB IP on a Hetzner private network; OpenBao raft snapshots land nightly in object storage with a verified restore.

## Current state (2026-07-07)

- **t001 (firewall app nodes)**: `infra/terraform/main.tf` has `hcloud_firewall.infra` for the single infra node only. CAPH-managed app-cluster nodes have no Hetzner Firewall. Approach: add a second `hcloud_firewall` resource in Terraform targeting servers by label (CAPH stamps `capi-cluster-name=bex` labels) and attach via `hcloud_firewall_attachment.servers[label_selector]`.
- **t002 (SHA-pin)**: `deploy/gitops/base/bex.yaml` still uses `:latest` tag (mutable). Pin via Argo's image override in the overlay, or by writing the digest into `bex.yaml` as part of the CI release step.
- **t003 (secrets at rest)**: This is about **infrastructure secrets** (Hetzner API key, Argo deploy key, GHCR token) currently applied imperatively via `kubectl` / CI secrets — NOT tenant secrets, which are already handled by OpenBao (`lego/backend/internal/secrets`, shipped in `feat(secrets)` commit). The gap is moving infra creds into encrypted GitOps (sealed-secrets controller + `SealedSecret` manifests in `deploy/gitops/`, or SOPS-age over the plaintext Secret YAMLs).
- **t004 (WireGuard via Cilium)**: `infra/clusterapi/overlays/hetzner-caph/cluster.yaml` has sysctl prep for Cilium already. Cilium itself needs to be deployed as the CNI (Helm chart via Argo) before enabling `encryption.enabled: true / type: wireguard` in its values. Depends on replacing whatever CNI kubeadm installs by default.
- **t005 (private network)**: `infra/terraform/main.tf` creates `hcloud_network.main`; infra node is attached. CAPH-managed nodes need to join this network via `HetznerCluster.spec.network.id`.

## Source

Converted from `.tmp/010-prod-hardening.md`.
