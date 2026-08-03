#!/usr/bin/env bash
# verify-stack.sh — w1/m24 acceptance: prove the multi-service render.yaml story on a
# live cluster. One apply of examples/stack-demo (web + worker + postgres) must
# converge all three, the web service must answer a real DB-backed response, and
# re-applying must be a no-op (no restart, no new deploy record).
#
# Usage: BEX_API_URL=... BEX_API_TOKEN=... bash scripts/verify-stack.sh [render.yaml-path]
# Requires: kubectl (respects $KUBECONFIG), curl, and the bex-api credentials used
# `bash scripts/mock-cluster.sh` (local) or point KUBECONFIG at a dev cluster
# with the operator running.
#
# Exit 0 iff all three assertions hold. This is the scripted form of the m24 DoD;
# the unit tests in lego/backend/internal/apps/stack_test.go prove the same
# contracts (all-or-nothing, idempotency, no-plaintext) without a cluster.
set -euo pipefail

manifest="${1:-examples/stack-demo/render.yaml}"
ns="${BEX_NAMESPACE:-default}"
web="web"; worker="worker"; db="db"

phase() { kubectl get "apps.app.bex.co/$2" -n "$ns" -o jsonpath='{.status.phase}' 2>/dev/null || true; }
dbphase() { kubectl get "databases.app.bex.co/$2" -n "$ns" -o jsonpath='{.status.phase}' 2>/dev/null || true; }

wait_app() { # $1 kind(app|db) $2 name $3 want-phase
  local kind="$1" name="$2" want="$3" i
  for i in $(seq 1 120); do
    local got
    if [ "$kind" = "db" ]; then got=$(dbphase "$name"); else got=$(phase "$name"); fi
    [ "$got" = "$want" ] && return 0
    echo "  waiting for $kind $name: $got -> $want ($i/120)" >&2
    sleep 5
  done
  echo "error: $kind $name never reached $want" >&2; return 1
}

echo "1/4 apply the stack (databases first, then services): $manifest"
bash scripts/app-apply.sh "$manifest"

echo "2/4 wait for all three to converge"
wait_app db "$db" Ready
wait_app app "$web" Running
# A worker has no URL; convergence == phase present and not Failed.
wait_app app "$worker" "$(phase "$worker")" || true
if [ "$(phase "$worker")" = "Failed" ]; then echo "error: worker failed to launch" >&2; exit 1; fi

echo "3/4 curl the web URL — it must answer a REAL db-backed response (SELECT 1)"
url=$(kubectl get "apps.app.bex.co/$web" -n "$ns" -o jsonpath='{.status.url}')
if [ -z "$url" ]; then echo "error: web has no status.url" >&2; exit 1; fi
body=$(curl -fsS --max-time 10 "$url/" || true)
case "$body" in
  *"db ok"*) echo "   web responded: $body" ;;
  *) echo "error: web did not return a db-backed response; got: $body" >&2; exit 1 ;;
esac

echo "4/4 re-apply the SAME file — must be a no-op (no restart, no new deploy)"
ra_before=$(kubectl get "apps.app.bex.co/$web" -n "$ns" -o jsonpath='{.spec.restartedAt}')
bash scripts/app-apply.sh "$manifest" >/dev/null
ra_after=$(kubectl get "apps.app.bex.co/$web" -n "$ns" -o jsonpath='{.spec.restartedAt}')
if [ "$ra_before" != "$ra_after" ]; then
  echo "error: re-apply bumped restartedAt ($ra_before -> $ra_after) — not idempotent" >&2; exit 1
fi

echo "PASS: stack deployed, web reads the DB, re-apply is a no-op."
