#!/usr/bin/env bash
set -euo pipefail

# Install the stable app-SSH gateway host key without ever rendering its bytes
# to stdout or checking them into Git. This is unrelated to node-admin SSH.
key_file="${BEX_SSH_HOST_KEY_FILE:-$HOME/.ssh/bex_gateway_host}"
namespace="${BEX_SYSTEM_NAMESPACE:-bex-system}"

for command in kubectl ssh-keygen; do
  command -v "$command" >/dev/null || { echo "missing required command: $command" >&2; exit 1; }
done

if [[ ! -f "$key_file" ]]; then
  echo "missing SSH gateway host key: $key_file" >&2
  echo "create it once with: ssh-keygen -t ed25519 -N '' -f '$key_file' -C bex-ssh-gateway" >&2
  exit 1
fi

if ! public_key="$(ssh-keygen -y -f "$key_file" 2>/dev/null)"; then
  echo "invalid SSH gateway private key: $key_file" >&2
  exit 1
fi
if [[ "${public_key%% *}" != "ssh-ed25519" ]]; then
  echo "SSH gateway host key must be Ed25519" >&2
  exit 1
fi

kubectl -n "$namespace" create secret generic bex-ssh-host-key \
  --from-file=ssh_host_ed25519_key="$key_file" \
  --dry-run=client -o yaml | kubectl apply -f - >/dev/null

# A running gateway loads its signer once at process startup. Roll it after
# replacement so rotation cannot report the new fingerprint while replicas are
# still serving the old key. On first install the Deployment may not exist yet.
if kubectl -n "$namespace" get deployment/bex-ssh-gateway >/dev/null 2>&1; then
  kubectl -n "$namespace" rollout restart deployment/bex-ssh-gateway >/dev/null
  kubectl -n "$namespace" rollout status deployment/bex-ssh-gateway --timeout=180s >/dev/null
fi

echo "installed bex-ssh-host-key in namespace $namespace"
printf '%s\n' "$public_key" | ssh-keygen -lf -
