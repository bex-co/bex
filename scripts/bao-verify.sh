#!/usr/bin/env bash
# E2E verify of the OpenBao substrate (docs/secrets.md) against the current
# kubeconfig cluster:
#   1. init/unseal is idempotent — re-running bao-init.sh changes nothing.
#   2. KV v2 write/read under tenants/.
#   3. Scoped SA login: the bex-api ServiceAccount logs in via the Kubernetes
#      auth method, can read/write tenants/*, but is denied a sys/* path
#      (sys/mounts) its policy doesn't grant.
#   4. Restart durability: after deleting the pod (the chart's StatefulSet uses
#      updateStrategyType OnDelete, so this — not `rollout restart` — is how it
#      comes back) it returns SEALED (no auto-unseal, by design) — the KV value
#      is unreadable until bao-init.sh's unseal step runs again with the same
#      three keys, which restores it exactly (state lives in the raft PVC, not
#      the seal state).
#
# Usage: scripts/bao-verify.sh    # respects $KUBECONFIG; exits 0 on pass
# Requires: kubectl, curl, yq v4.
set -euo pipefail
cd "$(dirname "$0")/.."

NS=secrets
BAO=127.0.0.1:38200
PF_PID=""

fail() { echo "FAIL: $*" >&2; exit 1; }

stop_forward() {
  if [ -n "$PF_PID" ]; then
    kill "$PF_PID" 2>/dev/null || true
    wait "$PF_PID" 2>/dev/null || true
  fi
  PF_PID=""
}
trap stop_forward EXIT

start_forward() {
  kubectl -n "$NS" port-forward service/openbao 38200:8200 >/dev/null 2>&1 &
  PF_PID=$!
  for _ in $(seq 1 30); do
    code="$(curl -s -o /dev/null -w '%{http_code}' "http://$BAO/v1/sys/seal-status" || true)"
    [ "$code" != "000" ] && return 0
    sleep 2
  done
  fail "http://$BAO did not become reachable"
}

# wait_pod_running — the readiness probe (`bao status`) fails while sealed by
# design, so this waits for the Pod to start rather than for k8s "Ready".
wait_pod_running() {
  kubectl -n "$NS" wait --for=jsonpath='{.status.phase}'=Running pod/openbao-0 --timeout=300s
}

echo "==> waiting for openbao to start"
wait_pod_running

# The bex-api ServiceAccount is normally deployed by `make deploy`
# (operator/config/api); ensure it exists here too so this script also works
# on a cluster that only has the OpenBao substrate up.
kubectl create namespace bex-system --dry-run=client -o yaml | kubectl apply -f - >/dev/null
kubectl -n bex-system create serviceaccount bex-api --dry-run=client -o yaml | kubectl apply -f - >/dev/null

echo "==> ensuring init/unseal/tenants mount (idempotent baseline)"
bash scripts/bao-init.sh

set -a
# shellcheck disable=SC1091
source ./.env
set +a
root_token="$BAO_ROOT_TOKEN"

start_forward

echo "==> init/unseal is idempotent — re-running changes nothing"
before="$(curl -sf "http://$BAO/v1/sys/seal-status")"
BAO_ADDR="http://$BAO" bash scripts/bao-init.sh >/dev/null
after="$(curl -sf "http://$BAO/v1/sys/seal-status")"
[ "$before" = "$after" ] || fail "seal-status changed on a repeat bao-init.sh run"

echo "==> ensuring kubernetes auth + bex-api role"
BAO_ADDR="http://$BAO" bash scripts/bao-k8s-auth.sh

echo "==> KV v2 write/read under tenants/"
VALUE="verify-$(date +%s)"
curl -sf -H "X-Vault-Token: $root_token" -X POST "http://$BAO/v1/tenants/data/verify" \
  -d "{\"data\":{\"secret\":\"$VALUE\"}}" >/dev/null || fail "KV write failed"
readback="$(curl -sf -H "X-Vault-Token: $root_token" "http://$BAO/v1/tenants/data/verify" | yq '.data.data.secret' -)"
[ "$readback" = "$VALUE" ] || fail "KV read-back mismatch: got $readback want $VALUE"

echo "==> scoped bex-api ServiceAccount login"
sa_jwt="$(kubectl -n bex-system create token bex-api --duration=10m)"
[ -n "$sa_jwt" ] || fail "could not mint a token for ServiceAccount bex-api/bex-system"
login_resp="$(curl -sf -X POST "http://$BAO/v1/auth/kubernetes/login" -d "{\"role\":\"bex-api\",\"jwt\":\"$sa_jwt\"}")"
scoped_token="$(printf '%s' "$login_resp" | yq '.auth.client_token' -)"
[ -n "$scoped_token" ] && [ "$scoped_token" != "null" ] || fail "bex-api SA login did not return a client_token"

echo "==> scoped token can read/write tenants/*"
curl -sf -H "X-Vault-Token: $scoped_token" -X POST "http://$BAO/v1/tenants/data/verify-scoped" \
  -d '{"data":{"secret":"scoped-write-ok"}}' >/dev/null || fail "scoped token could not write under tenants/"
scoped_readback="$(curl -sf -H "X-Vault-Token: $scoped_token" "http://$BAO/v1/tenants/data/verify-scoped" | yq '.data.data.secret' -)"
[ "$scoped_readback" = "scoped-write-ok" ] || fail "scoped token read-back mismatch"

echo "==> scoped token is denied outside tenants/*"
denied_code="$(curl -s -o /dev/null -w '%{http_code}' -H "X-Vault-Token: $scoped_token" "http://$BAO/v1/sys/mounts")"
[ "$denied_code" = "403" ] || fail "expected 403 reading sys/mounts with the scoped token, got $denied_code"

echo "==> restarting openbao (delete pod — StatefulSet strategy is OnDelete) — must come back sealed"
stop_forward
kubectl -n "$NS" delete pod openbao-0
wait_pod_running
start_forward

sealed_after_restart="$(curl -sf "http://$BAO/v1/sys/seal-status" | yq '.sealed' -)"
[ "$sealed_after_restart" = "true" ] || fail "expected OpenBao to come back SEALED after a restart (no auto-unseal configured)"
echo "    confirmed sealed (expected — no auto-unseal)"

echo "==> sealed OpenBao must refuse KV reads"
sealed_read_code="$(curl -s -o /dev/null -w '%{http_code}' -H "X-Vault-Token: $root_token" "http://$BAO/v1/tenants/data/verify")"
[ "$sealed_read_code" = "503" ] || fail "expected 503 reading tenants/ while sealed, got $sealed_read_code"

echo "==> re-running bao-init.sh restores access with the same three keys"
BAO_ADDR="http://$BAO" bash scripts/bao-init.sh
restored="$(curl -sf -H "X-Vault-Token: $root_token" "http://$BAO/v1/tenants/data/verify" | yq '.data.data.secret' -)"
[ "$restored" = "$VALUE" ] || fail "value lost across restart+unseal: got $restored want $VALUE"

echo "PASS: init/unseal idempotent, KV v2 works, bex-api SA scoped correctly, state survives restart"
