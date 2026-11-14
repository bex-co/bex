#!/usr/bin/env bash
# Install (or update) the out-of-band bex-system/bex-agent-platform-model
# Secret that arms bex-api's workspace-allowlisted platform model route
# (docs/ADR090-agent-owned-delivery.md § Platform OpenRouter route) and, when
# .env carries META_MODEL_API_KEY, the muse profile's Meta route
# (docs/ADR108-muse-code-agent.md). The OpenRouter key is read from .env
# OPENROUTER_API_KEY; both keys reach kubectl only through 0600 temp files —
# never argv, stdout, Git, or a generated manifest. The Secret is rewritten
# whole, so a key absent from .env is removed from it.
#
# Usage:
#   KUBECONFIG=infra/hetzner/bex.kubeconfig scripts/agent-platform-model-secret.sh
#   BEX_KUBE_CONTEXT=hetzner-prod scripts/agent-platform-model-secret.sh
#   DRY_RUN=1 scripts/agent-platform-model-secret.sh
#
# bex-api reads the keys as BEX_AGENT_PLATFORM_MODEL_KEY and
# BEX_AGENT_PLATFORM_META_KEY (secretKeyRefs bex-agent-platform-model/
# openrouter-api-key and /meta-api-key, both optional). When bex-api already
# consumes it, the deployment is restarted so the new key takes effect.
set -euo pipefail
cd "$(dirname "$0")/.."

if [ -f .env ]; then
  . scripts/lib/env-file.sh && bex_load_env ./.env || exit 1
fi

KUBE=(kubectl)
if [ -n "${BEX_KUBE_CONTEXT:-}" ]; then
  KUBE=(kubectl --context "$BEX_KUBE_CONTEXT")
fi
namespace="${BEX_SYSTEM_NAMESPACE:-bex-system}"
secret_name="bex-agent-platform-model"
secret_key="openrouter-api-key"

[ -n "${OPENROUTER_API_KEY:-}" ] || {
  echo "error: OPENROUTER_API_KEY is missing or empty (.env or environment)" >&2
  exit 1
}

meta_key="meta-api-key"
if [ "${DRY_RUN:-0}" = "1" ]; then
  echo "would apply Secret $namespace/$secret_name (key: $secret_key, from OPENROUTER_API_KEY; value not printed)"
  if [ -n "${META_MODEL_API_KEY:-}" ]; then
    echo "  and key: $meta_key, from META_MODEL_API_KEY (value not printed)"
  else
    echo "  without $meta_key (META_MODEL_API_KEY unset: muse stays off)"
  fi
  echo "kubeconfig=${KUBECONFIG:-<default>} context=${BEX_KUBE_CONTEXT:-<current>}"
  exit 0
fi

command -v kubectl >/dev/null || { echo "error: kubectl is required" >&2; exit 1; }
command -v jq >/dev/null || { echo "error: jq is required" >&2; exit 1; }
umask 077
secret_dir="$(mktemp -d)"
secret_file="$secret_dir/$secret_key"
meta_file="$secret_dir/$meta_key"
cleanup() {
  rm -f "$secret_file" "$meta_file"
  rmdir "$secret_dir" 2>/dev/null || true
}
trap cleanup EXIT
printf '%s' "$OPENROUTER_API_KEY" >"$secret_file"
from_files=(--from-file="$secret_key=$secret_file")
if [ -n "${META_MODEL_API_KEY:-}" ]; then
  printf '%s' "$META_MODEL_API_KEY" >"$meta_file"
  from_files+=(--from-file="$meta_key=$meta_file")
fi
"${KUBE[@]}" get namespace "$namespace" >/dev/null 2>&1 || "${KUBE[@]}" create namespace "$namespace" >/dev/null
"${KUBE[@]}" -n "$namespace" create secret generic "$secret_name" \
  "${from_files[@]}" \
  --dry-run=client -o yaml | "${KUBE[@]}" apply -f - >/dev/null
cleanup
trap - EXIT

if "${KUBE[@]}" -n "$namespace" get deployment/bex-api >/dev/null 2>&1 \
    && "${KUBE[@]}" -n "$namespace" get deployment/bex-api -o json \
    | jq -e '.spec.template.spec.containers[].env[]? | select(.name == "BEX_AGENT_PLATFORM_MODEL_KEY")' >/dev/null; then
  "${KUBE[@]}" -n "$namespace" rollout restart deployment/bex-api >/dev/null
  "${KUBE[@]}" -n "$namespace" rollout status deployment/bex-api --timeout=300s >/dev/null
  echo "installed $namespace/$secret_name for the platform model route; bex-api rollout is ready"
else
  echo "installed $namespace/$secret_name for the platform model route (bex-api not yet consuming it; next deploy will arm)"
fi
