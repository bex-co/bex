#!/usr/bin/env bash
# Install the Expo Push Service runtime credential out of band as
# bex-system/bex-push. Secret bytes are never printed or passed as process
# arguments. Re-running atomically rotates the provider/token pair and rolls
# bex-api; keep the old Expo access token valid until rollout completion.
set -euo pipefail
cd "$(dirname "$0")/.."

namespace="${BEX_SYSTEM_NAMESPACE:-bex-system}"
secret_name="${BEX_PUSH_SECRET_NAME:-bex-push}"
env_file="${BEX_PUSH_ENV_FILE:-.env}"

if [ -f "$env_file" ]; then
  set -a
  # shellcheck disable=SC1091
  source "$env_file"
  set +a
fi

BEX_PUSH_PROVIDER="${BEX_PUSH_PROVIDER:-expo}"
case "$BEX_PUSH_PROVIDER" in
  expo) ;;
  *) echo "error: BEX_PUSH_PROVIDER must be expo" >&2; exit 1 ;;
esac

if [ -z "${BEX_EXPO_PUSH_ACCESS_TOKEN:-}" ]; then
  echo "error: BEX_EXPO_PUSH_ACCESS_TOKEN is missing or empty (.env or environment)" >&2
  exit 1
fi
case "$BEX_EXPO_PUSH_ACCESS_TOKEN" in
  *$'\n'*|*$'\r'*) echo "error: BEX_EXPO_PUSH_ACCESS_TOKEN is malformed" >&2; exit 1 ;;
esac
if [ "${#BEX_EXPO_PUSH_ACCESS_TOKEN}" -gt 4096 ]; then
  echo "error: BEX_EXPO_PUSH_ACCESS_TOKEN is too long" >&2
  exit 1
fi

if [ "${DRY_RUN:-0}" = "1" ]; then
  echo "would apply Secret $namespace/$secret_name (keys: BEX_PUSH_PROVIDER BEX_EXPO_PUSH_ACCESS_TOKEN) and roll bex-api"
  exit 0
fi

command -v kubectl >/dev/null || { echo "error: kubectl is required" >&2; exit 1; }

umask 077
secret_dir="$(mktemp -d)"
secret_env="$secret_dir/push.env"
cleanup() {
  rm -f "$secret_env"
  rmdir "$secret_dir" 2>/dev/null || true
}
trap cleanup EXIT

{
  printf 'BEX_PUSH_PROVIDER=%s\n' "$BEX_PUSH_PROVIDER"
  printf 'BEX_EXPO_PUSH_ACCESS_TOKEN=%s\n' "$BEX_EXPO_PUSH_ACCESS_TOKEN"
} >"$secret_env"

kubectl get namespace "$namespace" >/dev/null 2>&1 || kubectl create namespace "$namespace" >/dev/null
kubectl -n "$namespace" create secret generic "$secret_name" \
  --from-env-file="$secret_env" \
  --dry-run=client -o yaml | kubectl apply -f - >/dev/null

if kubectl -n "$namespace" get deployment/bex-api >/dev/null 2>&1; then
  kubectl -n "$namespace" rollout restart deployment/bex-api >/dev/null
  kubectl -n "$namespace" rollout status deployment/bex-api --timeout=300s >/dev/null
fi

echo "installed $namespace/$secret_name; bex-api rollout is ready"
