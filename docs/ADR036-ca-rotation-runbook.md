# ADR: Kubernetes CA rotation and admin-cert renewal

**Status:** accepted; operational runbook (w7/m37). Companion to [ADR019-infra-credentials.md](ADR019-infra-credentials.md) (the credential inventory) and [ADR011-etcd-backup-restore.md](ADR011-etcd-backup-restore.md) (etcd backup). The admin cert expiry alert ([AdminCertExpiringSoon](../deploy/gitops/base/prometheus.yaml)) fires 30 days before the expected expiry.

## Context

The app cluster's kubeadm-issued certificates fall into two categories with different rotation costs:

| Category | Examples | Rotation cost | Trigger |
| --- | --- | --- | --- |
| **Client certs** | `kubernetes-admin`, `kube-scheduler`, `kube-controller-manager` | Low — kubeadm renews without downtime | Annual expiry (1-year default) |
| **Cluster CA** | `bex-ca`, `bex-etcd`, `bex-proxy` | High — all components must be restarted; all downstream certs reissued | Only on compromise or proactive hardening |

This document covers both paths: routine annual renewal (§1, expected path) and full CA rotation (§2, emergency path). The admin cert is the certificate embedded in the kubeconfig fetched by [`scripts/fetch-app-kubeconfig.sh`](../scripts/fetch-app-kubeconfig.sh) and stored in the in-cluster `bex-capi/bex-kubeconfig` Secret.

## 1. Annual client-cert renewal (expected path)

kubeadm renews all component client certs in place. No downtime for running workloads; kube-scheduler and kube-controller-manager restart briefly.

### Prerequisites

- Admin kubeconfig (`KUBECONFIG`) pointing at the app cluster.
- SSH access to each control-plane node via the `bex` key.
- `BAO_ROOT_TOKEN` in `.env` (the unseal step below is only needed if control-plane nodes are rebooted as part of the maintenance window).

### Procedure

```sh
# 1. Check current expiry on the primary CP node.
CP_IP=$(curl -sf -H "Authorization: Bearer $HCLOUD_TOKEN" \
  "https://api.hetzner.cloud/v1/servers?label_selector=caph-cluster-bex" \
  | jq -r '[.servers[] | select(.name | startswith("bex-control-plane"))][0].public_net.ipv4.ip')
ssh -i ~/.ssh/id_bex "root@${CP_IP}" \
  "kubeadm certs check-expiration"
```

```sh
# 2. Renew all client certs on each CP node (rerun this for each CP node).
ssh -i ~/.ssh/id_bex "root@${CP_IP}" \
  "kubeadm certs renew all"
```

```sh
# 3. Restart kube-scheduler and kube-controller-manager on each CP node
#    so they pick up the new certs (apiserver does not need a restart).
ssh -i ~/.ssh/id_bex "root@${CP_IP}" \
  "crictl rm -f $(crictl ps -q --label io.kubernetes.container.name=kube-scheduler 2>/dev/null) 2>/dev/null; \
   crictl rm -f $(crictl ps -q --label io.kubernetes.container.name=kube-controller-manager 2>/dev/null) 2>/dev/null; \
   true"
```

```sh
# 4. Refresh the admin kubeconfig (the renewed admin.conf is on the CP node).
scripts/fetch-app-kubeconfig.sh infra/local/bex.kubeconfig
```

```sh
# 5. Re-mint the scoped day-to-day kubeconfig with the updated cluster CA.
scripts/operator-kubeconfig.sh ~/.kube/bex-operator.kubeconfig
```

```sh
# 6. Verify cert expiry is now ~1 year out.
ssh -i ~/.ssh/id_bex "root@${CP_IP}" \
  "kubeadm certs check-expiration"
```

The CAPI in-cluster Secret `bex-capi/bex-kubeconfig` is updated automatically by CAPI after kubeadm renews the cert (CAPI watches and re-syncs). The `AdminCertExpiringSoon` alert clears within the next Prometheus scrape cycle once the Secret creation timestamp is refreshed.

### After renewal: push new kubeconfig to GitHub Actions

```sh
# HCLOUD_TOKEN, TF_STATE_* etc. are unchanged — only the kubeconfig changed.
# GitHub Actions fetches a fresh kubeconfig on each deploy run (fetch-app-kubeconfig.sh),
# so no separate GitHub secret update is needed.
echo "No GitHub secret update required — deploy.yml fetches the kubeconfig at run time."
```

---

## 2. Cluster CA rotation (emergency path)

Rotate the cluster CA when the `bex-ca` private key is suspected to be compromised. This is a disruptive operation: every component cert signed by the old CA must be reissued, and all clients that pinned the old CA must be updated. Plan for a maintenance window.

> **Important:** Before starting, take an etcd snapshot (see [ADR011](ADR011-etcd-backup-restore.md)). A failed CA rotation that leaves the cluster in an inconsistent state is much harder to recover from than a snapshot-restores one.

### What changes

- `bex-ca` key + cert → CAPI Secret `bex-capi/bex-ca`
- All certs signed by `bex-ca`: `kubernetes-admin`, `kube-apiserver`, `kube-apiserver-kubelet-client`, `kube-controller-manager`, `kube-scheduler`, and all kubelet client/server certs
- All clients that trust the old CA: kubectl, kube-proxy on worker nodes, Argo CD's in-cluster SA tokens (trust the apiserver via the service-account CA, not the cluster CA directly — unaffected), and any external systems pinning the CA cert

A separate `bex-etcd` CA covers etcd peer/client auth and is rotated independently using the same procedure with the etcd-specific cert names.

### Procedure

kubeadm's rotation path for the cluster CA uses a two-phase approach: add the new CA alongside the old one, restart all components, then remove the old CA. This avoids a hard cutover that would drop existing connections.

```sh
# Phase 0. Prerequisites on the primary CP node.
ssh -i ~/.ssh/id_bex "root@${CP_IP}" bash -s <<'EOF'
  # Snapshot etcd before touching anything.
  ETCDCTL_API=3 etcdctl snapshot save /tmp/pre-ca-rotation.db \
    --endpoints=https://127.0.0.1:2379 \
    --cacert=/etc/kubernetes/pki/etcd/ca.crt \
    --cert=/etc/kubernetes/pki/etcd/server.crt \
    --key=/etc/kubernetes/pki/etcd/server.key
  echo "snapshot saved: $(du -sh /tmp/pre-ca-rotation.db)"
EOF
```

```sh
# Phase 1. Generate a new CA key + cert alongside the old one.
ssh -i ~/.ssh/id_bex "root@${CP_IP}" bash -s <<'EOF'
  cd /etc/kubernetes/pki
  # Back up existing CA (do not delete yet).
  cp ca.crt ca.crt.old && cp ca.key ca.key.old
  # Generate new CA key.
  openssl genrsa -out ca.key.new 4096
  # Self-sign a new CA cert (10-year validity, same subject as the old one).
  openssl req -new -x509 -days 3650 -key ca.key.new \
    -subj "/CN=kubernetes" -out ca.crt.new
  # Bundle old + new CA into a combined cert (apiserver trusts both during rollover).
  cat ca.crt.old ca.crt.new > ca.crt
  # Leave ca.key.old untouched for now — Phase 3 removes it.
EOF
```

```sh
# Phase 2. Reissue all component certs signed by the new CA key.
#          kubeadm handles this via `certs renew` once we place the new key as ca.key.
ssh -i ~/.ssh/id_bex "root@${CP_IP}" bash -s <<'EOF'
  cd /etc/kubernetes/pki
  # Activate the new key.
  mv ca.key ca.key.old2 && cp ca.key.new ca.key
  # Renew all client and server certs.
  kubeadm certs renew all
  # Restart all CP components (apiserver, scheduler, controller-manager).
  for ctr in kube-apiserver kube-scheduler kube-controller-manager; do
    crictl rm -f "$(crictl ps -q --label io.kubernetes.container.name="${ctr}" 2>/dev/null)" \
      2>/dev/null || true
  done
  echo "CP components restarted with new CA-signed certs"
EOF
```

```sh
# Phase 2b. On each WORKER node: rotate the kubelet client cert.
for worker_ip in $(curl -sf -H "Authorization: Bearer $HCLOUD_TOKEN" \
    "https://api.hetzner.cloud/v1/servers?label_selector=caph-cluster-bex" \
    | jq -r '.servers[] | select(.name | startswith("bex-md-")) | .public_net.ipv4.ip'); do
  ssh -i ~/.ssh/id_bex "root@${worker_ip}" bash -s <<'EOF'
    # Copy the combined (old+new) CA cert from the CP.
    # (In practice: scp or a configmap-based distribution before this loop.)
    # Restart kubelet to pick up the new CA bundle and trigger cert rotation.
    systemctl restart kubelet
    echo "kubelet restarted on $(hostname)"
EOF
done
```

```sh
# Phase 3. Remove the old CA cert from the bundle (after all components verified healthy).
ssh -i ~/.ssh/id_bex "root@${CP_IP}" bash -s <<'EOF'
  cd /etc/kubernetes/pki
  # Replace the bundle with only the new CA cert.
  cp ca.crt.new ca.crt
  # Optionally delete the old key backup (keep it offline for a grace period).
  echo "Old CA key at: /etc/kubernetes/pki/ca.key.old (remove after verification)"
  # Renew admin cert against the new CA for a clean kubeconfig.
  kubeadm certs renew admin.conf
  cat /etc/kubernetes/admin.conf
EOF
```

```sh
# Phase 4. Update the CAPI in-cluster Secrets with the new CA material.
# CAPI stores the CA cert+key in bex-capi/bex-ca. After the rotation the
# in-cluster copy is stale and must be updated so CAPI can mint future node
# certs from the new CA.
kubectl -n bex-capi get secret bex-ca -o json \
  | jq --arg cert "$(ssh -i ~/.ssh/id_bex "root@${CP_IP}" cat /etc/kubernetes/pki/ca.crt | base64 -w0)" \
       --arg key  "$(ssh -i ~/.ssh/id_bex "root@${CP_IP}" cat /etc/kubernetes/pki/ca.key  | base64 -w0)" \
       '.data["tls.crt"] = $cert | .data["tls.key"] = $key' \
  | kubectl apply -f -
echo "CAPI bex-ca Secret updated"
```

```sh
# Phase 5. Re-fetch the admin kubeconfig and re-mint the day-to-day kubeconfig.
scripts/fetch-app-kubeconfig.sh infra/local/bex.kubeconfig
scripts/operator-kubeconfig.sh ~/.kube/bex-operator.kubeconfig
```

```sh
# Phase 6. Verify.
kubectl cluster-info
kubectl -n bex-system get pods
kubeadm certs check-expiration  # (on the CP node)
```

### External clients that pin the old CA

Any system that trusted the old CA cert explicitly (in a kubeconfig `certificate-authority-data` field, or in a `/etc/ssl/certs/` bundle) must be updated after Phase 3:

- GitHub Actions: not applicable — deploy.yml fetches the kubeconfig fresh at run time via `fetch-app-kubeconfig.sh`, which reads the live `admin.conf` from the CP node.
- Argo CD: uses the in-cluster `kubernetes.default.svc` endpoint (service account CA, not the cluster CA) — unaffected.
- The scoped `bex-operator.kubeconfig`: re-mint via `scripts/operator-kubeconfig.sh`.
- Any external monitoring or CI that hardcodes the CA cert: update those kubeconfigs.

## Alert: AdminCertExpiringSoon

A Prometheus alert ([deploy/gitops/base/prometheus.yaml](../deploy/gitops/base/prometheus.yaml)) fires 30 days before the expected admin-cert expiry based on the `bex-capi/bex-kubeconfig` Secret's creation timestamp (assuming 1-year kubeadm cert validity). When it fires:

1. Run `kubeadm certs check-expiration` (CP node SSH, §1 step 1) to confirm actual expiry.
2. If ≤30 days remaining, run the annual renewal procedure (§1).
3. The alert clears automatically once CAPI refreshes the Secret after kubeadm renewal.
