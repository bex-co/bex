#!/usr/bin/env bash
# Exercise certificate isolation through the pinned real Kubernetes Ingress provider.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
TEST_DIR="$(mktemp -d)"
CLUSTER="bex-tls-$$"
TEST_KUBECONFIG="$TEST_DIR/cluster.kubeconfig"
FORWARD_PID=""
CLUSTER_OWNED=0
cleanup() {
  local status=$?
  trap - EXIT
  if [ "$status" -ne 0 ] && [ "$CLUSTER_OWNED" -eq 1 ]; then
    cat "$TEST_DIR/forward.log" 2>/dev/null || true
    kubectl --kubeconfig "$TEST_KUBECONFIG" --request-timeout=10s -n traefik get pods -o wide || true
    kubectl --kubeconfig "$TEST_KUBECONFIG" --request-timeout=10s -n traefik logs deployment/traefik --tail=35 || true
  fi
  if [ -n "$FORWARD_PID" ]; then kill "$FORWARD_PID" 2>/dev/null || true; fi
  if [ "$CLUSTER_OWNED" -eq 1 ] && ! kind delete cluster --name "$CLUSTER" --kubeconfig "$TEST_KUBECONFIG"; then
    echo "Cleanup failed: cluster $CLUSTER remains; diagnostics: $TEST_DIR" >&2
    exit 1
  fi
  rm -rf "$TEST_DIR"
  exit "$status"
}
trap cleanup EXIT
for cmd in kind kubectl helm go; do command -v "$cmd" >/dev/null; done
if kind get clusters | grep -Fxq "$CLUSTER"; then
  echo "Refusing to reuse existing cluster $CLUSTER" >&2
  exit 1
fi
CLUSTER_OWNED=1
kind create cluster --name "$CLUSTER" --image kindest/node:v1.36.1 --kubeconfig "$TEST_KUBECONFIG" --wait 120s
kubectl --kubeconfig "$TEST_KUBECONFIG" label node "$CLUSTER-control-plane" bex.co/pool=platform
kubectl --kubeconfig "$TEST_KUBECONFIG" create namespace traefik
kubectl --kubeconfig "$TEST_KUBECONFIG" apply -k "$ROOT/deploy/gitops/charts/traefik-plugins"
kubectl --kubeconfig "$TEST_KUBECONFIG" apply -f "$ROOT/lego/operator/config/crd/bases/app.bex.co_apps.yaml"
helm --kubeconfig "$TEST_KUBECONFIG" upgrade --install traefik traefik \
  --repo https://traefik.github.io/charts --version 41.0.0 --namespace traefik \
  -f "$ROOT/deploy/gitops/base/values/traefik.values.yaml" --wait --timeout 4m
kubectl --kubeconfig "$TEST_KUBECONFIG" -n traefik port-forward service/traefik :80 :443 > "$TEST_DIR/forward.log" 2>&1 &
FORWARD_PID=$!
for _ in $(seq 1 60); do
  if [ "$(grep -c 'Forwarding from 127.0.0.1:' "$TEST_DIR/forward.log" || true)" -eq 2 ]; then break; fi
  if ! kill -0 "$FORWARD_PID" 2>/dev/null; then cat "$TEST_DIR/forward.log"; exit 1; fi
  sleep 1
done
BEX_TEST_TRAEFIK_HTTP="$(sed -n 's/Forwarding from 127.0.0.1:\([0-9]*\) -> 80$/127.0.0.1:\1/p' "$TEST_DIR/forward.log")"
BEX_TEST_TRAEFIK_HTTPS="$(sed -n 's/Forwarding from 127.0.0.1:\([0-9]*\) -> 443$/127.0.0.1:\1/p' "$TEST_DIR/forward.log")"
test -n "$BEX_TEST_TRAEFIK_HTTP" && test -n "$BEX_TEST_TRAEFIK_HTTPS"
export BEX_TEST_TRAEFIK_HTTP BEX_TEST_TRAEFIK_HTTPS
export BEX_TEST_TRAEFIK_KUBECONFIG="$TEST_KUBECONFIG"
cd "$ROOT/lego/operator"
GOWORK=off go test ./internal/controller -run '^TestIngressTLSIsolationWithTraefik$' -count=1 -v -timeout 5m
