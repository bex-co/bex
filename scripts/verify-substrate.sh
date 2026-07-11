#!/usr/bin/env bash
# verify-substrate.sh — proves the w1/m19.1 self-managed-substrate DoD:
#   1. ACCESS:   the cluster's kubeconfig is fetchable with NO mgmt cluster
#                (hcloud API -> CP node -> SSH admin.conf; scripts/fetch-app-kubeconfig.sh)
#   2. SELF-MGMT: CAPI objects live IN the cluster — machines all Running with
#                nodeRefs, KubeadmControlPlane initialized
#   3. PLACEMENT: CAPI controllers run on control-plane nodes (mgmt-plane home)
#   4. AUTOSCALER: the Argo-managed in-cluster autoscaler is Running
#   5. NO PET:    no bootstrap pet server exists on Hetzner
#
# Env: HCLOUD_TOKEN (required; sourced from ./.env if unset)
#      BEX_SSH_KEY_PATH   SSH key for the CP fetch (default ~/.ssh/bex)
#      WL_KUBECONFIG      skip the fetch and use this kubeconfig (check 1 still
#                         runs unless SKIP_FETCH=1)
# Exit 0 on full pass; non-zero naming the failed check. Requires kubectl, jq, curl.
set -euo pipefail
cd "$(dirname "$0")/.."

if [ -z "${HCLOUD_TOKEN:-}" ] && [ -f .env ]; then
  set -a; source ./.env; set +a
fi
: "${HCLOUD_TOKEN:?HCLOUD_TOKEN required (env or .env)}"
export BEX_SSH_KEY_PATH="${BEX_SSH_KEY_PATH:-$HOME/.ssh/bex}"

fail() { echo "FAIL: $*" >&2; exit 1; }
pass() { echo "PASS: $*"; }

# --- 1. ACCESS: mgmt-free kubeconfig fetch -----------------------------------
KUBECONFIG_FILE="${WL_KUBECONFIG:-}"
if [ "${SKIP_FETCH:-0}" != "1" ]; then
  TMP=$(mktemp)
  trap 'rm -f "$TMP"' EXIT
  bash scripts/fetch-app-kubeconfig.sh "$TMP" >/dev/null \
    || fail "access: fetch-app-kubeconfig.sh could not reach a CP node"
  KUBECONFIG_FILE="$TMP"
  pass "access: kubeconfig fetched via hcloud API + CP SSH (no mgmt cluster)"
fi
wl() { kubectl --kubeconfig "$KUBECONFIG_FILE" "$@"; }

# --- 2. SELF-MGMT: machines + KCP reconcile in-cluster ------------------------
MACHINES_JSON=$(wl get machines -n default -o json 2>/dev/null) \
  || fail "self-mgmt: 'kubectl get machines' failed in-cluster (CAPI absent?)"
TOTAL=$(echo "$MACHINES_JSON" | jq '.items | length')
[ "$TOTAL" -ge 1 ] || fail "self-mgmt: zero Machine objects in-cluster"
NOT_RUNNING=$(echo "$MACHINES_JSON" | jq '[.items[] | select(.status.phase != "Running")] | length')
[ "$NOT_RUNNING" -eq 0 ] || fail "self-mgmt: $NOT_RUNNING/$TOTAL machines not Running"
NO_NODE=$(echo "$MACHINES_JSON" | jq '[.items[] | select(.status.nodeRef == null)] | length')
[ "$NO_NODE" -eq 0 ] || fail "self-mgmt: $NO_NODE machines have no nodeRef"
KCP_INIT=$(wl get kcp -n default -o jsonpath='{.items[0].status.initialization.controlPlaneInitialized}' 2>/dev/null)
[ "$KCP_INIT" = "true" ] || fail "self-mgmt: KubeadmControlPlane not initialized in-cluster"
pass "self-mgmt: $TOTAL machines Running with nodeRefs, KCP initialized — in-cluster"

# --- 3. PLACEMENT: CAPI controllers on CP nodes -------------------------------
CP_NODES=$(wl get nodes -l node-role.kubernetes.io/control-plane -o jsonpath='{.items[*].metadata.name}')
for ns in capi-system caph-system; do
  POD_NODE=$(wl get pods -n "$ns" -o jsonpath='{.items[0].spec.nodeName}' 2>/dev/null) \
    || fail "placement: no pods in $ns"
  case " $CP_NODES " in
    *" $POD_NODE "*) ;;
    *) fail "placement: $ns pod on '$POD_NODE' (not a CP node)" ;;
  esac
done
pass "placement: capi-system + caph-system controllers on CP nodes"

# --- 4. AUTOSCALER: in-cluster, Running ---------------------------------------
CA_PHASE=$(wl get pods -n default -l app.kubernetes.io/instance=cluster-autoscaler \
  -o jsonpath='{.items[0].status.phase}' 2>/dev/null)
[ "$CA_PHASE" = "Running" ] || fail "autoscaler: in-cluster autoscaler pod not Running (${CA_PHASE:-absent})"
pass "autoscaler: in-cluster cluster-autoscaler Running"

# --- 5. NO PET: bootstrap server retired --------------------------------------
INFRA_COUNT=$(curl -sf -H "Authorization: Bearer $HCLOUD_TOKEN" \
  "https://api.hetzner.cloud/v1/servers?name=bex-bootstrap" | jq '.servers | length')
[ "$INFRA_COUNT" -eq 0 ] || fail "no-pet: bex-bootstrap server still exists"
pass "no-pet: no bex-bootstrap server on Hetzner"

echo "verify-substrate: ALL PASS"
