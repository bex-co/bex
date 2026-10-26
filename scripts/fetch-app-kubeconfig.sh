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
#         BEX_SSH_KEY_PATH    SSH private key override (otherwise
#                             BEX_SSH_PRIVATE_KEY_FILE or ~/.ssh/bex)
#         BEX_SSH_KNOWN_HOSTS_FILE  authoritative known-hosts file. Set => the
#                                  host key is AUTHENTICATED and an unknown or
#                                  changed key fails closed before admin.conf is
#                                  read (w1/m66 F7). Unset => trust-on-first-use,
#                                  as before, with a notice on stderr.
#         CLUSTER_NAME        cluster name (default bex)
# Callers: .github/workflows/app-cluster.yml, .github/workflows/deploy.yml,
#          the deploy-app-from-local runbook, and local operators.
set -euo pipefail

OUT="${1:?usage: fetch-app-kubeconfig.sh <output-path>}"
: "${HCLOUD_TOKEN:?HCLOUD_TOKEN is required}"
KEY="${BEX_SSH_KEY_PATH:-${BEX_SSH_PRIVATE_KEY_FILE:-$HOME/.ssh/bex}}"
CLUSTER="${CLUSTER_NAME:-bex}"
# shellcheck source=scripts/lib/ssh-hostkey.sh
source "$(dirname "${BASH_SOURCE[0]}")/lib/ssh-hostkey.sh"
bex_ssh_hostkey_args

# A control-plane roll replaces nodes (new IPs, new host keys) one at a time,
# so for a few minutes every candidate can fail strict host-key checking even
# though the pinned keys are still current for the nodes that will remain
# (w6/m153: a deploy and a drill failed mid-roll on 2026-10-24, then fetched
# fine once it settled). Rediscover the nodes and retry for a bounded time
# when — and only when — host-key verification is what failed. Strict checking
# is never relaxed.
ATTEMPTS="${BEX_KUBECONFIG_FETCH_ATTEMPTS:-6}"
RETRY_SECONDS="${BEX_KUBECONFIG_FETCH_RETRY_SECONDS:-60}"

discover() {
  curl -sf -H "Authorization: Bearer $HCLOUD_TOKEN" \
    "https://api.hetzner.cloud/v1/servers?label_selector=caph-cluster-${CLUSTER}" \
    | jq -r --arg p "${CLUSTER}-control-plane" '
        [.servers[]
          | select(.status == "running")
          | select(.name | startswith($p))]
        | sort_by(.created) | reverse[]
        | .public_net.ipv4.ip // empty'
}

TMP=$(mktemp "${OUT}.tmp.XXXXXX")
SSH_ERR=$(mktemp "${OUT}.ssherr.XXXXXX")
cleanup() {
  [ -z "${TMP:-}" ] || rm -f -- "$TMP"
  rm -f -- "$SSH_ERR"
}
trap cleanup EXIT

attempt=1
while :; do
  CP_IPS=$(discover)
  [ -n "$CP_IPS" ] || { echo "no running control-plane servers found for $CLUSTER" >&2; exit 1; }
  hostkey_failures=0
  while IFS= read -r CP_IP; do
    if ssh -o BatchMode=yes -o ConnectTimeout=8 \
        "${BEX_SSH_HOSTKEY_ARGS[@]}" -i "$KEY" "root@${CP_IP}" \
        'cat /etc/kubernetes/admin.conf' > "$TMP" 2> "$SSH_ERR" \
      && kubectl --kubeconfig "$TMP" --request-timeout=10s cluster-info >/dev/null; then
      chmod 600 "$TMP"
      mv "$TMP" "$OUT"
      TMP=""
      echo "app kubeconfig -> $OUT (via CP $CP_IP)"
      exit 0
    fi
    cat "$SSH_ERR" >&2
    if grep -q "Host key verification failed" "$SSH_ERR"; then
      hostkey_failures=$((hostkey_failures + 1))
    fi
    echo "control-plane $CP_IP unavailable; trying the next candidate" >&2
  done <<< "$CP_IPS"

  if [ "$hostkey_failures" -eq 0 ] || [ "$attempt" -ge "$ATTEMPTS" ]; then
    break
  fi
  echo "every reachable control plane failed host-key verification (attempt $attempt/$ATTEMPTS) — a roll may be in progress; retrying in ${RETRY_SECONDS}s" >&2
  attempt=$((attempt + 1))
  sleep "$RETRY_SECONDS"
done

if [ "$hostkey_failures" -gt 0 ]; then
  echo "error: control-plane host keys still do not match the pinned BEX_SSH_KNOWN_HOSTS after $attempt attempt(s)." >&2
  echo "       Either a control-plane roll is still running, or the pin is stale: re-pin the" >&2
  echo "       current control-plane host keys (BEX_SSH_KNOWN_HOSTS secret + .env) from an" >&2
  echo "       authenticated source (docs/ADR019-infra-credentials.md). Never trust on first use." >&2
fi
echo "no reachable, healthy control-plane server found for $CLUSTER" >&2
exit 1
