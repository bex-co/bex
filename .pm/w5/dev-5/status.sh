#!/usr/bin/env bash
# Quick health check for the dev-5 isolated environment (workstream w5).
# Usage: bash .pm/w5/dev-5/status.sh
set -euo pipefail
cd "$(dirname "$0")/../../.." # repo root
ENVDIR=".pm/w5/dev-5"
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
  echo "== $DEV_AUTH_NS pods =="
  kubectl -n "$DEV_AUTH_NS" get pods 2>&1 || true
  echo
  echo "== $DEV_NS resources =="
  kubectl -n "$DEV_NS" get apps.app.bex.co,keyvalues.app.bex.co,databases.app.bex.co 2>&1 || true
fi

echo
echo "== http checks =="
curl -s -o /dev/null -w "  kratos   (:$KRATOS_PUBLIC_PORT): %{http_code}\n" "http://localhost:$KRATOS_PUBLIC_PORT/health/alive" || true
curl -s -o /dev/null -w "  kratos-adm(:$KRATOS_ADMIN_PORT): %{http_code}\n" "http://localhost:$KRATOS_ADMIN_PORT/admin/health/alive" || true
curl -s -o /dev/null -w "  hydra    (:$HYDRA_ADMIN_PORT):   %{http_code}\n" "http://localhost:$HYDRA_ADMIN_PORT/health/ready" || true
curl -s -o /dev/null -w "  bex-api  (:$BEX_API_PORT):       %{http_code}\n" "http://localhost:$BEX_API_PORT/healthz" || true
curl -s -o /dev/null -w "  dashboard(:$DASHBOARD_PORT):     %{http_code}\n" "http://localhost:$DASHBOARD_PORT/" || true

# == verification inventory (w5/m62) ==
# Unlike the loose health output above, these are ASSERTIONS: each prints
# [ok]/[FAIL] and the whole section exits non-zero if any fails, so a broken
# substrate (no StorageClass, CNPG down, Loki down, log store unreachable) is a
# hard, actionable signal — not a status a reader has to interpret. Run against a
# deliberately-broken stack, these turn red (that's the point).
echo
echo "== verification inventory (w5/m62) =="
INV_FAIL=0
ok() { echo "  [ok]   $1"; }
no() { echo "  [FAIL] $1"; INV_FAIL=$((INV_FAIL + 1)); }

if [ -f "$ENVDIR/.kubeconfig" ]; then
  export KUBECONFIG="$PWD/$ENVDIR/.kubeconfig"
  # A default StorageClass — every PVC (3 CNPG DBs + Loki) needs one.
  if kubectl get storageclass -o jsonpath='{range .items[*]}{.metadata.annotations.storageclass\.kubernetes\.io/is-default-class}{"\n"}{end}' 2>/dev/null | grep -qx true; then
    ok "default StorageClass present"
  else
    no "no default StorageClass — CNPG DBs + Loki PVCs cannot bind (reprovision the cluster)"
  fi
  # CNPG operator installed AND ready (not just the CRD). jsonpath yields ""
  # (not 0) when the field is absent, so default it before the numeric compare.
  cnpg_avail="$(kubectl -n cnpg-system get deploy cnpg-cloudnative-pg -o jsonpath='{.status.availableReplicas}' 2>/dev/null)"
  if kubectl get crd clusters.postgresql.cnpg.io >/dev/null 2>&1 && [ "${cnpg_avail:-0}" -ge 1 ]; then
    ok "CNPG operator ready"
  else
    no "CNPG operator not ready (absent CRD, or the manager's :9443 webhook won't bind)"
  fi
  # Loki ready.
  loki_ready="$(kubectl -n monitoring get statefulset loki -o jsonpath='{.status.readyReplicas}' 2>/dev/null)"
  if [ "${loki_ready:-0}" -ge 1 ]; then
    ok "Loki ready"
  else
    no "Loki not ready (check 'kubectl -n monitoring describe pod loki-0')"
  fi
fi

# Loki ingest -> query round-trip through the same forward bex-api uses
# (BEX_LOKI_URL). The real proof a log line survives into dev-5's store.
LOKI_BASE="http://localhost:$LOKI_PORT"
probe_ns="$(date +%s)000000000"
push_code="$(curl -s -o /dev/null -w '%{http_code}' -XPOST "$LOKI_BASE/loki/api/v1/push" -H 'Content-Type: application/json' \
  --data-binary "{\"streams\":[{\"stream\":{\"namespace\":\"$DEV_NS\",\"app\":\"status-probe\",\"type\":\"app\"},\"values\":[[\"$probe_ns\",\"status.sh loki inventory probe\"]]}]}" 2>/dev/null)"
sleep 1
hits="$(curl -s -G "$LOKI_BASE/loki/api/v1/query_range" --data-urlencode 'query={app="status-probe"}' --data-urlencode 'limit=1' 2>/dev/null | grep -c 'status.sh loki inventory probe' || true)"
if [ "$push_code" = "204" ] && [ "${hits:-0}" -ge 1 ]; then
  ok "Loki ingest->query round-trip (pushed a line at :$LOKI_PORT, read it back)"
else
  no "Loki ingest->query failed (push=$push_code, query hits=${hits:-0}) — is the loki forward up?"
fi

# bex-api reachable (the authenticated non-503 log read is the live walk's job —
# it needs a Kratos session status.sh has no business minting).
api_code="$(curl -s -o /dev/null -w '%{http_code}' "http://localhost:$BEX_API_PORT/healthz" 2>/dev/null)"
if [ "$api_code" = "200" ]; then
  ok "bex-api reachable (:$BEX_API_PORT)"
else
  no "bex-api not reachable (:$BEX_API_PORT healthz=$api_code)"
fi

echo
if [ "$INV_FAIL" -eq 0 ]; then
  echo "verification inventory: ALL GREEN"
else
  echo "verification inventory: $INV_FAIL check(s) FAILED"
  exit 1
fi
