#!/usr/bin/env bash
# Fetch the app cluster's admin kubeconfig via a control-plane node (w1/m19.1).
#
# Post-pivot there is no mgmt cluster: discover a CP node by its CAPH label
# through the hcloud API and SSH-fetch /etc/kubernetes/admin.conf (key-only
# :22 — the recorded auth-only baseline, docs/ADR019-infra-credentials.md). The
# admin.conf already points at the kube-api LB, so no rewriting is needed.
#
# Usage:  HCLOUD_TOKEN=... scripts/fetch-app-kubeconfig.sh <output-path>
# Env:    HCLOUD_TOKEN        (required) Hetzner Cloud API token
#         BEX_SSH_KEY_PATH    SSH private key (default ~/.ssh/id_bex)
#         CLUSTER_NAME        cluster name (default bex)
# Callers: .github/workflows/app-cluster.yml, .github/workflows/deploy.yml,
#          the deploy-app-from-local runbook, and local operators.
set -euo pipefail

OUT="${1:?usage: fetch-app-kubeconfig.sh <output-path>}"
: "${HCLOUD_TOKEN:?HCLOUD_TOKEN is required}"
KEY="${BEX_SSH_KEY_PATH:-$HOME/.ssh/id_bex}"
CLUSTER="${CLUSTER_NAME:-bex}"

CP_IP=$(curl -sf -H "Authorization: Bearer $HCLOUD_TOKEN" \
  "https://api.hetzner.cloud/v1/servers?label_selector=caph-cluster-${CLUSTER}" \
  | jq -r --arg p "${CLUSTER}-control-plane" \
      '[.servers[] | select(.name | startswith($p))][0].public_net.ipv4.ip')
test -n "$CP_IP" && test "$CP_IP" != "null"

ssh -o StrictHostKeyChecking=accept-new -i "$KEY" "root@${CP_IP}" \
  'cat /etc/kubernetes/admin.conf' > "$OUT"
chmod 600 "$OUT"
kubectl --kubeconfig "$OUT" cluster-info >/dev/null
echo "app kubeconfig -> $OUT (via CP ${CP_IP})"
