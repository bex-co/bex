#!/usr/bin/env bash
# Tear down the dev-alpha isolated environment: kills the local processes
# up.sh forked (port-forwards, bex-api) and deletes the dev-alpha-auth /
# dev-alpha namespaces from the shared cluster. Leaves the shared cluster
# itself (and any other session's resources in other namespaces) untouched.
#
# Usage: bash env/dev-alpha/down.sh
set -euo pipefail
cd "$(dirname "$0")/../.." # repo root
ENVDIR="env/dev-alpha"
# shellcheck disable=SC1091
source "$ENVDIR/ports.env"

echo "==> killing local processes (bex-api, port-forwards)"
for pidfile in "$ENVDIR"/.pids/*.pid; do
  [ -f "$pidfile" ] || continue
  pid="$(cat "$pidfile")"
  pkill -P "$pid" 2>/dev/null || true # forward()'s loop's current kubectl child, if any
  kill "$pid" 2>/dev/null || true
  rm -f "$pidfile"
done

if [ -f "$ENVDIR/.kubeconfig" ]; then
  export KUBECONFIG="$PWD/$ENVDIR/.kubeconfig"
  echo "==> helm uninstall kratos/hydra"
  helm uninstall kratos hydra -n "$DEV_ALPHA_AUTH_NS" >/dev/null 2>&1 || true
  echo "==> deleting namespaces $DEV_ALPHA_AUTH_NS $DEV_ALPHA_NS"
  kubectl delete namespace "$DEV_ALPHA_AUTH_NS" "$DEV_ALPHA_NS" --ignore-not-found --wait=false
else
  echo "no kubeconfig at $ENVDIR/.kubeconfig — nothing to delete in-cluster"
fi

echo "dev-alpha down. Namespace deletion continues in the background (kubectl get ns to watch)."
