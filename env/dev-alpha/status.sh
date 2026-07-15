#!/usr/bin/env bash
# Quick health check for the dev-alpha isolated environment.
# Usage: bash env/dev-alpha/status.sh
set -euo pipefail
cd "$(dirname "$0")/../.." # repo root
ENVDIR="env/dev-alpha"
# shellcheck disable=SC1091
source "$ENVDIR/ports.env"

echo "== local processes =="
for pidfile in "$ENVDIR"/.pids/*.pid; do
  [ -f "$pidfile" ] || continue
  pid="$(cat "$pidfile")"
  name="$(basename "$pidfile" .pid)"
  if kill -0 "$pid" 2>/dev/null; then echo "  $name: running (pid $pid)"; else echo "  $name: NOT running (stale pid $pid)"; fi
done

if [ -f "$ENVDIR/.kubeconfig" ]; then
  export KUBECONFIG="$PWD/$ENVDIR/.kubeconfig"
  echo
  echo "== $DEV_ALPHA_AUTH_NS pods =="
  kubectl -n "$DEV_ALPHA_AUTH_NS" get pods 2>&1 || true
  echo
  echo "== $DEV_ALPHA_NS resources =="
  kubectl -n "$DEV_ALPHA_NS" get apps.app.bex.co,keyvalues.app.bex.co,databases.app.bex.co 2>&1 || true
fi

echo
echo "== http checks =="
curl -s -o /dev/null -w "  kratos   (:$KRATOS_PUBLIC_PORT): %{http_code}\n" "http://localhost:$KRATOS_PUBLIC_PORT/health/alive" || true
curl -s -o /dev/null -w "  hydra    (:$HYDRA_ADMIN_PORT):   %{http_code}\n" "http://localhost:$HYDRA_ADMIN_PORT/health/ready" || true
curl -s -o /dev/null -w "  bex-api  (:$BEX_API_PORT):       %{http_code}\n" "http://localhost:$BEX_API_PORT/healthz" || true
curl -s -o /dev/null -w "  dashboard(:$DASHBOARD_PORT):     %{http_code}\n" "http://localhost:$DASHBOARD_PORT/" || true
