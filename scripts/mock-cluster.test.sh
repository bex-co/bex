#!/usr/bin/env bash
# Contract tests for scripts/mock-cluster.sh (w7/m148 t006).
#
# These cover the two behaviours that used to fail silently and destructively:
# a second run racing the first, and a failed kubeconfig generation truncating
# the shared config every other session reads. Both are exercised against the
# real functions, with clusterctl/docker/kubectl stubbed — provisioning a CAPD
# cluster is not something a test suite should do.
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

fails=0
ok()   { printf '  ok    %s\n' "$1"; }
bad()  { printf '  FAIL  %s\n' "$1"; fails=$((fails + 1)); }
check(){ if [ "$2" = "$3" ]; then ok "$1"; else bad "$1 (want '$3', got '$2')"; fi; }

# Source definitions only: the guard returns before any provisioning runs.
BEX_MOCK_CLUSTER_LIB=1
export BEX_MOCK_CLUSTER_LIB
# shellcheck source=/dev/null
. scripts/mock-cluster.sh

LOCK_DIR="$tmp/lock"
WL_KUBECONFIG="$tmp/bex.kubeconfig"

echo "lock ownership:"

acquire_lock >/dev/null
check "acquiring an unheld lock succeeds" "$LOCK_HELD" "1"
check "the lock records the owning pid" "$(cat "$LOCK_DIR/pid")" "$$"

# A live owner must not be displaced. The subshell inherits the held lock dir
# but pretends to be a different process by re-running acquire_lock, which must
# refuse because our own pid is alive.
set +e
( LOCK_HELD=; acquire_lock >/dev/null 2>&1 ) ; rc=$?
set -e
check "a second run refuses while a live owner holds it" "$rc" "1"
check "the refusal left the original owner in place" "$(cat "$LOCK_DIR/pid")" "$$"

# A crashed run leaves a lock whose pid is gone; that must be reclaimable, or
# one crash wedges every future run.
printf '%s\n' "999999" >"$LOCK_DIR/pid"
LOCK_HELD=
acquire_lock >/dev/null
check "a stale lock (dead owner) is reclaimed" "$(cat "$LOCK_DIR/pid")" "$$"

# release_lock must be ownership-aware: not holding it means not removing it.
LOCK_HELD=
release_lock
check "release does nothing when this process does not hold it" \
  "$([ -d "$LOCK_DIR" ] && echo present || echo gone)" "present"
LOCK_HELD=1
release_lock
check "release removes the lock this process holds" \
  "$([ -d "$LOCK_DIR" ] && echo present || echo gone)" "gone"

echo
echo "kubeconfig publication:"

printf 'PRIOR-CONFIG\n' >"$WL_KUBECONFIG"
prior_sum="$(shasum "$WL_KUBECONFIG" | awk '{print $1}')"

# 1. clusterctl fails -> the shared file must be untouched, not truncated.
clusterctl() { return 1; }
docker() { echo "0.0.0.0:6443"; }
kubectl() { return 0; }
set +e
publish_kubeconfig >/dev/null 2>&1; rc=$?
set -e
check "clusterctl failure is reported" "$rc" "1"
check "clusterctl failure leaves the prior kubeconfig byte-identical" \
  "$(shasum "$WL_KUBECONFIG" | awk '{print $1}')" "$prior_sum"

# 2. A config that disables TLS verification must never be published.
clusterctl() { printf 'server: https://10.0.0.1:6443\ninsecure-skip-tls-verify: true\n'; }
set +e
publish_kubeconfig >/dev/null 2>&1; rc=$?
set -e
check "a TLS-skipping kubeconfig is refused" "$rc" "1"
check "the refusal leaves the prior kubeconfig byte-identical" \
  "$(shasum "$WL_KUBECONFIG" | awk '{print $1}')" "$prior_sum"

# 3. An unreachable apiserver must not be published either.
clusterctl() { printf 'server: https://10.0.0.1:6443\n'; }
kubectl() { return 1; }
set +e
publish_kubeconfig >/dev/null 2>&1; rc=$?
set -e
check "an unreachable apiserver is refused" "$rc" "1"
check "the unreachable refusal leaves the prior kubeconfig byte-identical" \
  "$(shasum "$WL_KUBECONFIG" | awk '{print $1}')" "$prior_sum"

# 4. The happy path publishes, and rewrites the server to the published port.
docker() { echo "127.0.0.1:34567"; }
kubectl() { return 0; }
set +e
publish_kubeconfig >/dev/null 2>&1; rc=$?
set -e
check "a valid kubeconfig publishes" "$rc" "0"
check "the server is rewritten to the host-published port" \
  "$(grep -c 'server: https://127.0.0.1:34567' "$WL_KUBECONFIG")" "1"

# 5. No credential material may reach stdout/stderr.
clusterctl() { printf 'server: https://10.0.0.1:6443\nclient-key-data: SUPERSECRETKEYMATERIAL\n'; }
out="$(publish_kubeconfig 2>&1 || true)"
check "publication never prints credential material" \
  "$(printf '%s' "$out" | grep -c 'SUPERSECRETKEYMATERIAL' || true)" "0"

echo
if [ "$fails" -ne 0 ]; then
  echo "FAILED: $fails check(s)" >&2
  exit 1
fi
echo "mock-cluster.sh contracts: all checks passed"
