#!/usr/bin/env bash
# Regression tests for scripts/private-ingress-deny-check.sh (w6/m640): the
# current tree passes, and each change that would cut platform traffic, refuse
# a paid service its private traffic or let a sibling through fails.
set -euo pipefail
cd "$(dirname "$0")/.."

src=deploy/gitops/base/tenant-node-egress.yaml
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
fail=0

if ! bash scripts/private-ingress-deny-check.sh "$src" >"$tmp/pass.out" 2>&1; then
  echo "FAIL: the current tree should pass:" >&2
  cat "$tmp/pass.out" >&2
  fail=1
fi

# refuses NAME YQ-EDIT — the check fails once the policy is edited so.
refuses() {
  local name="$1" edit="$2"
  yq "(select(.kind == \"CiliumClusterwideNetworkPolicy\" and .metadata.name == \"deny-send-only-private-ingress\") | .spec) |= ($edit)" "$src" >"$tmp/edited.yaml"
  if cmp -s "$src" "$tmp/edited.yaml"; then
    echo "FAIL [$name]: the edit did not apply; update this test with the policy" >&2
    fail=1
  elif bash scripts/private-ingress-deny-check.sh "$tmp/edited.yaml" >"$tmp/out" 2>&1; then
    echo "FAIL [$name]: the check passed an edited policy" >&2
    fail=1
  elif ! grep -q '^FAIL: deny-send-only-private-ingress' "$tmp/out"; then
    echo "FAIL [$name]: the check failed for another reason:" >&2
    cat "$tmp/out" >&2
    fail=1
  else
    echo "ok   [$name]: refused"
  fi
}

refuses "default deny ingress" '.enableDefaultDeny.ingress = true'
refuses "default deny egress" '.enableDefaultDeny.egress = true'
refuses "selector dropped" '.endpointSelector = {}'
refuses "selector widened to every workspace pod" '.endpointSelector = {"matchExpressions": [{"key": "app.bex.co/workspace", "operator": "Exists"}]}'
refuses "selector on the free-plan label" '.endpointSelector.matchLabels = {"app.bex.co/free-plan": "true"}'
refuses "selector on another value" '.endpointSelector.matchLabels."app.bex.co/private-network" = "true"'
refuses "every endpoint denied" '.ingressDeny[0].fromEndpoints = [{}]'
refuses "host entity denied" '.ingressDeny[0].fromEntities = ["host"]'
refuses "second deny rule" '.ingressDeny += [{"fromEntities": ["world"]}]'
refuses "source narrowed to one namespace" '.ingressDeny[0].fromEndpoints[0].matchLabels = {"k8s:io.kubernetes.pod.namespace": "default"}'
refuses "source key changed" '.ingressDeny[0].fromEndpoints[0].matchExpressions[0].key = "app.bex.co/app"'
refuses "source operator inverted" '.ingressDeny[0].fromEndpoints[0].matchExpressions[0].operator = "DoesNotExist"'
refuses "an allow added" '.ingress = [{"fromEntities": ["world"]}]'
refuses "an egress deny added" '.egressDeny = [{"toEntities": ["world"]}]'

yq 'select(.metadata.name != "deny-send-only-private-ingress")' "$src" >"$tmp/missing.yaml"
if bash scripts/private-ingress-deny-check.sh "$tmp/missing.yaml" >"$tmp/out" 2>&1; then
  echo "FAIL [policy removed]: the check passed a manifest without the policy" >&2
  fail=1
else
  echo "ok   [policy removed]: refused"
fi

[ "$fail" = 0 ] && echo "PASS: private-ingress-deny-check tests"
exit "$fail"
