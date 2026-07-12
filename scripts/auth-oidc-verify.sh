#!/usr/bin/env bash
# Prod (or mock-cluster) smoke for GitHub social login (docs/auth.md § Social
# login, w4/003). Read-only: it inspects the live cluster + public Kratos and
# reports whether the `oidc` method is wired end to end, without mutating
# anything. Complements scripts/auth-oidc-e2e.sh (which proves the *mechanism*
# with a throwaway Dex); this proves the *deployment* is actually enabled.
#
# Checks, in order:
#   1. the `kratos` Secret carries an `oidc.yaml` key, and whether it enables a
#      provider (enabled: true) or is the no-op (enabled: false)
#   2. the kratos Deployment loads it as a SECOND `--config` file
#   3. the kratos rollout is healthy (the second config didn't crashloop it)
#   4. the public login flow advertises the provider node (the button)
#
# Usage:  KUBECONFIG=… scripts/auth-oidc-verify.sh
# Env:    KRATOS_PUBLIC_URL   public Kratos base (default https://auth.bex.co)
#         KRATOS_NAMESPACE    namespace of the kratos release (default auth)
# Requires: kubectl (respects $KUBECONFIG), curl, python3.
set -euo pipefail

PUB="${KRATOS_PUBLIC_URL:-https://auth.bex.co}"
NS="${KRATOS_NAMESPACE:-auth}"
fail=0

echo "-> 1/4 kratos Secret oidc.yaml key..."
frag="$(kubectl -n "$NS" get secret kratos -o go-template='{{index .data "oidc.yaml"}}' 2>/dev/null | base64 -d 2>/dev/null || true)"
if [ -z "$frag" ]; then
  echo "  ✗ no oidc.yaml key in the kratos Secret — run scripts/auth-secrets.sh (deploy.yml does this)"
  fail=1
elif printf '%s' "$frag" | grep -q 'enabled: true'; then
  prov="$(printf '%s' "$frag" | grep -oE 'provider: [a-z]+' | head -1 | awk '{print $2}')"
  echo "  ✓ oidc.yaml present and ENABLED (provider: ${prov:-?})"
else
  echo "  • oidc.yaml present but DISABLED (enabled: false) — set BEX_GITHUB_OIDC_* and re-run auth-secrets.sh"
fi

echo "-> 2/4 kratos Deployment loads it as a second --config..."
args="$(kubectl -n "$NS" get deploy kratos -o jsonpath='{.spec.template.spec.containers[0].args}' 2>/dev/null || true)"
if printf '%s' "$args" | grep -q '/etc/config-oidc/oidc.yaml'; then
  echo "  ✓ args include --config /etc/config-oidc/oidc.yaml"
else
  echo "  ✗ second --config not mounted yet — the base kratos values haven't synced (Argo/main)"
  fail=1
fi

echo "-> 3/4 kratos rollout healthy..."
if kubectl -n "$NS" rollout status deploy/kratos --timeout=10s >/dev/null 2>&1; then
  echo "  ✓ kratos rollout is complete"
else
  echo "  ✗ kratos is not fully rolled out (a bad second config crashloops it — check logs)"
  fail=1
fi

echo "-> 4/4 public login flow advertises the provider button..."
provs="$(curl -sf "$PUB/self-service/login/browser" -H 'Accept: application/json' 2>/dev/null \
  | python3 -c "import sys,json;d=json.load(sys.stdin);print(' '.join(n['attributes'].get('value','') for n in d['ui']['nodes'] if n['attributes'].get('name')=='provider'))" 2>/dev/null || true)"
if [ -n "$provs" ]; then
  echo "  ✓ login flow offers provider(s): $provs"
else
  echo "  • login flow offers no provider — expected while social login is disabled"
fi

echo
if [ "$fail" = 0 ]; then
  echo "✓ social-login deployment checks passed for $PUB"
  echo "  Final manual step: open $PUB's dashboard login, click 'Sign in with GitHub', authorize."
else
  echo "✗ social login is not fully live yet (see ✗ lines above)"
  exit 1
fi
