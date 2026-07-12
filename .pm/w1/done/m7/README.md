# w1 · m7 — Prod hardening: network · secrets · images

**Worker:** worker1 **Goal:** Close the deliberate demo shortcuts so the cluster is more than a demo: firewall the API, pin the operator image, encrypt secrets + node traffic, and get a stable LB IP on a private network. **Status:** done — **DONE**

## Tasks (any order — mostly independent)

| id   | title                                             | est | depends_on |            |
| ---- | ------------------------------------------------- | --- | ---------- | ---------- |
| t001 | Firewall the app nodes (Hetzner firewall, TF)     | 30m | —          | — **REMOVED** → `.pm/DO_NOT_DO.md` |
| t002 | Immutable operator image (SHA-pin, not `:latest`) | 30m | —          | — **DONE** |
| t003 | Secrets at rest (sealed-secrets / SOPS)           | 30m | —          | — **DONE** |
| t004 | Encrypt node-to-node (Cilium WireGuard)           | 25m | —          | — **DEFERRED** → `.pm/FUTURE-MAYBE.md` |
| t005 | Private network + stable LB IP (Traefik LB)       | 30m | t004       | — **DONE** |
| t006 | OpenBao Raft snapshot backup (etcd-backup CronJob pattern) | 30m | —          | — **DONE** |

## Definition of done

The kube-API is firewalled (not internet-exposed); the operator runs a SHA-pinned image; repo/registry/Hetzner creds are encrypted at rest; node traffic is WireGuard-encrypted; Traefik has a stable LB IP on a Hetzner private network; OpenBao raft snapshots land nightly in object storage with a verified restore.

## Current state (2026-07-07)

- **t001 (firewall app nodes)**: `infra/terraform/main.tf` has `hcloud_firewall.infra` for the single infra node only. CAPH-managed app-cluster nodes have no Hetzner Firewall. Approach: add a second `hcloud_firewall` resource in Terraform targeting servers by label (CAPH stamps `capi-cluster-name=bex` labels) and attach via `hcloud_firewall_attachment.servers[label_selector]`.
- **t002 (SHA-pin)**: `deploy/gitops/base/bex.yaml` still uses `:latest` tag (mutable). Pin via Argo's image override in the overlay, or by writing the digest into `bex.yaml` as part of the CI release step.
- **t003 (secrets at rest)**: This is about **infrastructure secrets** (Hetzner API key, Argo deploy key, GHCR token) currently applied imperatively via `kubectl` / CI secrets — NOT tenant secrets, which are already handled by OpenBao (`lego/backend/internal/secrets`, shipped in `feat(secrets)` commit). The gap is moving infra creds into encrypted GitOps (sealed-secrets controller + `SealedSecret` manifests in `deploy/gitops/`, or SOPS-age over the plaintext Secret YAMLs).
- **t004 (WireGuard via Cilium)**: `infra/clusterapi/overlays/hetzner-caph/cluster.yaml` has sysctl prep for Cilium already. Cilium itself needs to be deployed as the CNI (Helm chart via Argo) before enabling `encryption.enabled: true / type: wireguard` in its values. Depends on replacing whatever CNI kubeadm installs by default.
- **t005 (private network)**: `infra/terraform/main.tf` creates `hcloud_network.main`; infra node is attached. CAPH-managed nodes need to join this network via `HetznerCluster.spec.network.id`.

## Delivered (2026-07-08)

All six shipped as declarative config (the Hetzner/CAPH surfaces are IaC that can't be applied without a live account — validated by rendering/`terraform validate`, not a live run, matching the repo's existing untested-infra convention).

- **t001** — ~~`hcloud_firewall.app` + attachment locking `:22`/`:6443` to `allowed_ssh_cidrs`~~ **REMOVED 2026-07-09** (Terraform code deleted; see [.pm/DO_NOT_DO.md](../../../DO_NOT_DO.md) + [docs/ADR019-infra-credentials.md](../../../../docs/ADR019-infra-credentials.md)). It never applied to prod (validated-only, like the rest of m7), and a static source-IP allowlist fits neither a dynamic-IP operator nor GitHub-hosted CI. `:22`/`:6443` stay authentication-only; a network second layer, if ever wanted, is Tailscale/WireGuard, not a static CIDR.
- **t002** — digest write-back in `deploy.yml`: after build+push, the pushed `@sha256` digest is `sed`-pinned into `bex.yaml`/`dashboard.yaml` and committed back to main `[skip ci]`, so the running Deployment is content-addressed, never `:latest`.
- **t003** — sealed-secrets controller Argo app (`deploy/gitops/base/sealed-secrets.yaml`, chart 2.19.1) + `scripts/seal-secret.sh` + `docs/ADR016-sealed-secrets.md` (sealed-secrets over SOPS; which infra creds to seal; sealing-key DR).
- **t004** — Cilium WireGuard (`encryption.enabled/type=wireguard/nodeEncryption`) flags in `app-cluster.yml`, but **DEFERRED 2026-07-09** (live cluster: `Encryption: Disabled`). A no-op on a single node (no inter-node hop to encrypt; node kernel lacks the `wireguard` module) — revisit at the multi-node buildout. See [.pm/FUTURE-MAYBE.md](../../../FUTURE-MAYBE.md).
- **t005** — CAPH `hcloudNetwork` (own `bex` network); Traefik refactored to multi-source + a prod overlay values file flips it to a Hetzner `LoadBalancer` for a stable IP. **Delivered live 2026-07-09** (the first attempt bundled `use-private-ip` + the private net and took prod down — 521 — because the node wasn't on the network; reverted, then re-done correctly): LB `bex-traefik` (`142.132.241.247`), public-IP first, then `use-private-ip: "true"` after attaching the single node to the `bex` network out-of-band (`10.0.1.2`) and making the CCM network-aware with the route-controller off (`HCLOUD_NETWORK_ROUTES_ENABLED=false`). All targets healthy over the private net; verified serving. See `deploy/gitops/overlays/prod/values/traefik.values.yaml`, `app-cluster.yml`, docs/ADR019-infra-credentials.md.
- **t006** — `openbao-backup` CronJob chart (`bao operator raft snapshot save` → S3, retain-7), Argo app in base (local overlay excludes), least-privilege `snapshot` policy + `bao-snapshot` role added to `scripts/bao-k8s-auth.sh`, runbook `docs/ADR015-openbao-backup-restore.md`.

Verification: `scripts/gitops-validate.sh` PASS (whole tree renders, incl. new chart + both overlays); `terraform validate`/`fmt` PASS; shell + YAML syntax checked. Live-cluster smoke of the LB, WireGuard handshake, firewall reachability, and a real snapshot→restore remain for the first Hetzner run (flagged in the docs).

## Source

Converted from `.tmp/010-prod-hardening.md`.
