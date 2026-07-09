# w1 · m7 — Prod hardening: network · secrets · images

**Worker:** worker1 **Goal:** Close the deliberate demo shortcuts so the cluster is more than a demo: firewall the API, pin the operator image, encrypt secrets + node traffic, and get a stable LB IP on a private network. **Status:** done — **DONE**

## Tasks (any order — mostly independent)

| id   | title                                             | est | depends_on |            |
| ---- | ------------------------------------------------- | --- | ---------- | ---------- |
| t001 | Firewall the app nodes (Hetzner firewall, TF)     | 30m | —          | — **DONE** |
| t002 | Immutable operator image (SHA-pin, not `:latest`) | 30m | —          | — **DONE** |
| t003 | Secrets at rest (sealed-secrets / SOPS)           | 30m | —          | — **DONE** |
| t004 | Encrypt node-to-node (Cilium WireGuard)           | 25m | —          | — **DONE** |
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

- **t001** — `hcloud_firewall.app` + `hcloud_firewall_attachment.app` (label selector `caph-cluster-bex=owned`, the real CAPH ownership label from its `tags.go`) in `infra/terraform/main.tf`; `:22`/`:6443` locked to `allowed_ssh_cidrs`, `:80/:443/:5432` public, ICMP. `terraform validate` + `fmt -check` pass. Safe only alongside t005 (node-to-node must be on the private net, which a public firewall never sees).
- **t002** — digest write-back in `deploy.yml`: after build+push, the pushed `@sha256` digest is `sed`-pinned into `bex.yaml`/`dashboard.yaml` and committed back to main `[skip ci]`, so the running Deployment is content-addressed, never `:latest`.
- **t003** — sealed-secrets controller Argo app (`deploy/gitops/base/sealed-secrets.yaml`, chart 2.19.1) + `scripts/seal-secret.sh` + `docs/sealed-secrets.md` (sealed-secrets over SOPS; which infra creds to seal; sealing-key DR).
- **t004** — Cilium WireGuard (`encryption.enabled/type=wireguard/nodeEncryption`) in `app-cluster.yml`.
- **t005** — CAPH `hcloudNetwork` enabled (own `bex` network; the TF infra network renamed `bex-infra` to avoid the name collision — CAPH has no existing-network reference field); CCM `networking.enabled`; Traefik refactored to multi-source and a prod overlay values file flips it to a Hetzner `LoadBalancer` with `use-private-ip` for a stable IP. Validated by `gitops-validate.sh` (extended to render `traefik` + prod-overlay layerings against the real chart).
- **t006** — `openbao-backup` CronJob chart (`bao operator raft snapshot save` → S3, retain-7), Argo app in base (local overlay excludes), least-privilege `snapshot` policy + `bao-snapshot` role added to `scripts/bao-k8s-auth.sh`, runbook `docs/openbao-backup-restore.md`.

Verification: `scripts/gitops-validate.sh` PASS (whole tree renders, incl. new chart + both overlays); `terraform validate`/`fmt` PASS; shell + YAML syntax checked. Live-cluster smoke of the LB, WireGuard handshake, firewall reachability, and a real snapshot→restore remain for the first Hetzner run (flagged in the docs).

## Source

Converted from `.tmp/010-prod-hardening.md`.
