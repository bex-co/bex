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
#         BEX_SSH_KNOWN_HOSTS_FILE  optional isolated known-hosts file for
#                                  immutable-node maintenance
#         CLUSTER_NAME        cluster name (default bex)
# Callers: .github/workflows/app-cluster.yml, .github/workflows/deploy.yml,
#          the deploy-app-from-local runbook, and local operators.
set -euo pipefail

OUT="${1:?usage: fetch-app-kubeconfig.sh <output-path>}"
: "${HCLOUD_TOKEN:?HCLOUD_TOKEN is required}"
KEY="${BEX_SSH_KEY_PATH:-$HOME/.ssh/id_bex}"
CLUSTER="${CLUSTER_NAME:-bex}"
SSH_KNOWN_HOSTS_ARGS=()
if [ -n "${BEX_SSH_KNOWN_HOSTS_FILE:-}" ]; then
  SSH_KNOWN_HOSTS_ARGS=(-o "UserKnownHostsFile=$BEX_SSH_KNOWN_HOSTS_FILE")
fi

CP_IPS=$(curl -sf -H "Authorization: Bearer $HCLOUD_TOKEN" \
  "https://api.hetzner.cloud/v1/servers?label_selector=caph-cluster-${CLUSTER}" \
  | jq -r --arg p "${CLUSTER}-control-plane" '
      [.servers[]
        | select(.status == "running")
        | select(.name | startswith($p))]
      | sort_by(.created) | reverse[]
      | .public_net.ipv4.ip // empty')
[ -n "$CP_IPS" ] || { echo "no running control-plane servers found for $CLUSTER" >&2; exit 1; }

TMP=$(mktemp "${OUT}.tmp.XXXXXX")
cleanup() { [ -z "${TMP:-}" ] || rm -f -- "$TMP"; }
trap cleanup EXIT

while IFS= read -r CP_IP; do
  if ssh -o BatchMode=yes -o ConnectTimeout=8 -o StrictHostKeyChecking=accept-new \
      "${SSH_KNOWN_HOSTS_ARGS[@]}" -i "$KEY" "root@${CP_IP}" \
      'cat /etc/kubernetes/admin.conf' > "$TMP" \
    && kubectl --kubeconfig "$TMP" --request-timeout=10s cluster-info >/dev/null; then
    chmod 600 "$TMP"
    mv "$TMP" "$OUT"
    TMP=""
    echo "app kubeconfig -> $OUT (via CP $CP_IP)"
    exit 0
  fi
  echo "control-plane $CP_IP unavailable; trying the next candidate" >&2
done <<< "$CP_IPS"

echo "no reachable, healthy control-plane server found for $CLUSTER" >&2
exit 1
