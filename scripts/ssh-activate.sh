#!/usr/bin/env bash
set -euo pipefail

# Publish BEX_SSH_HOST to bex-api only after DNS/TCP/22 presents the stable key.
mode="${1:-}"
if [[ -n "$mode" && "$mode" != "--check" ]]; then
  echo "usage: $0 [--check]" >&2
  exit 2
fi
: "${BEX_SSH_HOST:?set the public gateway hostname}"
key_file="${BEX_SSH_HOST_KEY_FILE:-$HOME/.ssh/bex_gateway_host}"
namespace="${BEX_SYSTEM_NAMESPACE:-bex-system}"

if [[ ! "$BEX_SSH_HOST" =~ ^([A-Za-z0-9]([A-Za-z0-9-]*[A-Za-z0-9])?\.)+[A-Za-z0-9]([A-Za-z0-9-]*[A-Za-z0-9])?$ ]]; then
  echo "BEX_SSH_HOST must be a DNS hostname without a scheme, path, user, or port" >&2
  exit 1
fi

for command in dig kubectl ssh-keygen ssh-keyscan; do
  command -v "$command" >/dev/null || { echo "missing required command: $command" >&2; exit 1; }
done
[[ -f "$key_file" ]] || { echo "missing host key file: $key_file" >&2; exit 1; }

if ! public_key="$(ssh-keygen -y -f "$key_file" 2>/dev/null)"; then
  echo "invalid SSH gateway private key: $key_file" >&2
  exit 1
fi
if [[ "${public_key%% *}" != "ssh-ed25519" ]]; then
  echo "SSH gateway host key must be Ed25519" >&2
  exit 1
fi
expected="$(printf '%s\n' "$public_key" | ssh-keygen -lf - | awk '{print $2}')"

# Match the complete DNS address set, not just whichever family ssh-keyscan
# happens to reach first. A stale Cloudflare-proxied AAAA record can otherwise
# break IPv6-preferred OpenSSH clients even after the A record points directly
# at the Hetzner LoadBalancer.
edge_addresses="$(kubectl -n traefik get service traefik -o jsonpath='{range .status.loadBalancer.ingress[*]}{.ip}{"\n"}{end}' | awk 'NF' | sort -u)"
dns_addresses="$({ dig +short A "$BEX_SSH_HOST" || true; dig +short AAAA "$BEX_SSH_HOST" || true; } | awk '/^[0-9a-fA-F:.]+$/' | sort -u)"
if [[ -z "$edge_addresses" || "$dns_addresses" != "$edge_addresses" ]]; then
  echo "refusing activation: public A/AAAA records must equal the Traefik LoadBalancer ingress addresses" >&2
  echo "edge addresses: ${edge_addresses:-<none>}" >&2
  echo "DNS addresses: ${dns_addresses:-<none>}" >&2
  exit 1
fi

actual="$(ssh-keyscan -T 10 -p 22 "$BEX_SSH_HOST" 2>/dev/null | ssh-keygen -lf - 2>/dev/null | awk '{print $2}' | sort -u || true)"
if [[ -z "$actual" || "$actual" != "$expected" ]]; then
  echo "refusing activation: public host fingerprint does not match the stable gateway key" >&2
  exit 1
fi

if [[ "$mode" == "--check" ]]; then
  echo "PASS app SSH activation preflight host=$BEX_SSH_HOST fingerprint=$actual"
  exit 0
fi

kubectl -n "$namespace" create configmap bex-ssh --from-literal=host="$BEX_SSH_HOST" \
  --dry-run=client -o yaml | kubectl apply -f - >/dev/null
kubectl -n "$namespace" rollout restart deployment/bex-api >/dev/null
kubectl -n "$namespace" rollout status deployment/bex-api --timeout=180s >/dev/null
echo "activated app SSH host=$BEX_SSH_HOST fingerprint=$actual"
