#!/usr/bin/env bash
# Out-of-band init/unseal of OpenBao (docs/secrets.md#3), idempotent, plus the
# tenants/ KV v2 mount (docs/secrets.md#4).
#
# First run: initializes OpenBao (5 Shamir shares / 3 threshold — the OpenBao
# default), then writes BAO_UNSEAL_KEY_1/2/3 + BAO_ROOT_TOKEN into .env
# (gitignored — never printed to stdout/logs, same convention as
# auth-secrets.sh). Every run after that reads those same keys back out of
# .env and unseals — including after a pod restart, which always comes back
# sealed (no auto-unseal, by design: docs/secrets.md#3).
#
# Usage: scripts/bao-init.sh             # init (first run) / unseal / ensure tenants/ mount
#        BAO_ADDR=http://... ...         # use an already-reachable OpenBao
#        DRY_RUN=1 scripts/bao-init.sh   # print intent, change nothing
# Requires: kubectl (respects $KUBECONFIG) unless BAO_ADDR is set, curl, yq v4.
set -euo pipefail
cd "$(dirname "$0")/.."

NS=secrets
PF_PID=""

if [ -f .env ]; then
  set -a
  # shellcheck disable=SC1091
  source ./.env
  set +a
fi

cleanup() {
  if [ -n "$PF_PID" ]; then
    kill "$PF_PID" 2>/dev/null || true
    wait "$PF_PID" 2>/dev/null || true
  fi
}
trap cleanup EXIT

url="${BAO_ADDR:-}"
if [ -z "$url" ]; then
  kubectl -n "$NS" port-forward service/openbao 38200:8200 >/dev/null 2>&1 &
  PF_PID=$!
  url=http://127.0.0.1:38200
  for _ in $(seq 1 30); do
    code="$(curl -s -o /dev/null -w '%{http_code}' "$url/v1/sys/seal-status" || true)"
    [ "$code" != "000" ] && break
    sleep 2
  done
fi

if [ "${DRY_RUN:-}" = "1" ]; then
  echo "would init (if needed), unseal (if needed), ensure tenants/ kv-v2 mount at $url"
  exit 0
fi

# set_env_var NAME VALUE — set/replace a key in .env without ever printing VALUE.
set_env_var() {
  local name="$1" val="$2"
  if [ -f .env ] && grep -q "^${name}=" .env; then
    awk -v n="$name" -v v="$val" -F= 'BEGIN{OFS="="} $1==n{print n,v; next} {print}' .env >.env.tmp
    mv .env.tmp .env
  else
    printf '%s=%s\n' "$name" "$val" >>.env
  fi
}

# --- 1. init (once) -----------------------------------------------------------
initialized="$(curl -sf "$url/v1/sys/seal-status" | yq '.initialized' -)"
if [ "$initialized" != "true" ]; then
  echo "==> initializing (5 shares / 3 threshold)"
  init_resp="$(curl -sf -X PUT "$url/v1/sys/init" -d '{"secret_shares":5,"secret_threshold":3}')"
  set_env_var BAO_UNSEAL_KEY_1 "$(printf '%s' "$init_resp" | yq '.keys_base64[0]' -)"
  set_env_var BAO_UNSEAL_KEY_2 "$(printf '%s' "$init_resp" | yq '.keys_base64[1]' -)"
  set_env_var BAO_UNSEAL_KEY_3 "$(printf '%s' "$init_resp" | yq '.keys_base64[2]' -)"
  set_env_var BAO_ROOT_TOKEN "$(printf '%s' "$init_resp" | yq '.root_token' -)"
  echo "initialized — unseal keys + root token written to .env (never printed)"
  set -a
  source ./.env
  set +a
else
  echo "already initialized"
fi

for name in BAO_UNSEAL_KEY_1 BAO_UNSEAL_KEY_2 BAO_UNSEAL_KEY_3 BAO_ROOT_TOKEN; do
  [ -n "${!name:-}" ] || { echo "error: $name is missing from .env (delete the OpenBao PVC to re-init, or restore .env from backup)" >&2; exit 1; }
done

# --- 2. unseal (every run) -----------------------------------------------------
sealed="$(curl -sf "$url/v1/sys/seal-status" | yq '.sealed' -)"
if [ "$sealed" = "true" ]; then
  echo "==> unsealing"
  curl -sf -X PUT "$url/v1/sys/unseal" -d "{\"key\":\"$BAO_UNSEAL_KEY_1\"}" >/dev/null
  curl -sf -X PUT "$url/v1/sys/unseal" -d "{\"key\":\"$BAO_UNSEAL_KEY_2\"}" >/dev/null
  result="$(curl -sf -X PUT "$url/v1/sys/unseal" -d "{\"key\":\"$BAO_UNSEAL_KEY_3\"}")"
  [ "$(printf '%s' "$result" | yq '.sealed' -)" = "false" ] || { echo "error: still sealed after presenting 3 keys" >&2; exit 1; }
  echo "unsealed"
else
  echo "already unsealed"
fi

# --- 3. tenants/ kv-v2 mount ----------------------------------------------------
mounts="$(curl -sf -H "X-Vault-Token: $BAO_ROOT_TOKEN" "$url/v1/sys/mounts")"
if [ "$(printf '%s' "$mounts" | yq 'has("tenants/")' -)" = "true" ]; then
  echo "tenants/ mount exists"
else
  curl -sf -H "X-Vault-Token: $BAO_ROOT_TOKEN" -X POST "$url/v1/sys/mounts/tenants" \
    -d '{"type":"kv","options":{"version":"2"}}' >/dev/null
  echo "created tenants/ kv-v2 mount"
fi
