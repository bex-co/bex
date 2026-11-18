#!/usr/bin/env bash
# Prod (or mock-cluster) smoke for social login — GitHub, Google, GitLab and
# Bitbucket (docs/ADR012-auth.md § Social login, w4/003, w7/m465, w7/m468).
# Read-only: it inspects the live cluster + public Kratos and
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
#   5. when the fragment enables `bitbucket`: Dex, its OIDC bridge, is ready
#      and its public issuer serves discovery; when it does not, no Dex runs
#
# Usage:  KUBECONFIG=… scripts/auth-oidc-verify.sh
# Env:    KRATOS_PUBLIC_URL   public Kratos base (default https://auth.bex.co)
#         KRATOS_NAMESPACE    namespace of the kratos release (default auth)
#         DEX_ISSUER_URL      Dex's public issuer (default $KRATOS_PUBLIC_URL/dex)
# Requires: kubectl (respects $KUBECONFIG), curl, python3.
set -euo pipefail

PUB="${KRATOS_PUBLIC_URL:-https://auth.bex.co}"
NS="${KRATOS_NAMESPACE:-auth}"
fail=0

echo "-> 1/5 kratos Secret oidc.yaml key..."
frag="$(kubectl -n "$NS" get secret kratos -o go-template='{{index .data "oidc.yaml"}}' 2>/dev/null | base64 -d 2>/dev/null || true)"
if [ -z "$frag" ]; then
  echo "  ✗ no oidc.yaml key in the kratos Secret — run scripts/auth-secrets.sh (deploy.yml does this)"
  fail=1
elif printf '%s' "$frag" | grep -q 'enabled: true'; then
  prov="$(printf '%s' "$frag" | grep -oE '^ *- id: [a-z]+' | awk '{print $3}' | tr '\n' ' ')"
  echo "  ✓ oidc.yaml present and ENABLED (providers: ${prov:-?})"
else
  echo "  • oidc.yaml present but DISABLED (enabled: false) — set a BEX_{GITHUB,GOOGLE,GITLAB,BITBUCKET}_OIDC_* pair and re-run auth-secrets.sh"
fi
# The Google-login settings guard (w7/m465) rides the same fragment whether or
# not a provider is on.
if printf '%s' "$frag" | grep -q '/internal/kratos/settings-guard?method=profile'; then
  echo "  ✓ oidc.yaml wires the Google-login settings guard"
elif [ -n "$frag" ]; then
  echo "  ✗ oidc.yaml has no settings guard — re-run scripts/auth-secrets.sh"
  fail=1
fi

echo "-> 2/5 kratos Deployment loads it as a second --config..."
args="$(kubectl -n "$NS" get deploy kratos -o jsonpath='{.spec.template.spec.containers[0].args}' 2>/dev/null || true)"
if printf '%s' "$args" | grep -q '/etc/config-oidc/oidc.yaml'; then
  echo "  ✓ args include --config /etc/config-oidc/oidc.yaml"
else
  echo "  ✗ second --config not mounted yet — the base kratos values haven't synced (Argo/main)"
  fail=1
fi

echo "-> 3/5 kratos rollout healthy..."
if kubectl -n "$NS" rollout status deploy/kratos --timeout=10s >/dev/null 2>&1; then
  echo "  ✓ kratos rollout is complete"
else
  echo "  ✗ kratos is not fully rolled out (a bad second config crashloops it — check logs)"
  fail=1
fi

echo "-> 4/5 public login flow advertises the provider button..."
provs="$(curl -sf "$PUB/self-service/login/browser" -H 'Accept: application/json' 2>/dev/null \
  | python3 -c "import sys,json;d=json.load(sys.stdin);print(' '.join(n['attributes'].get('value','') for n in d['ui']['nodes'] if n['attributes'].get('name')=='provider'))" 2>/dev/null || true)"
if [ -n "$provs" ]; then
  echo "  ✓ login flow offers provider(s): $provs"
else
  echo "  • login flow offers no provider — expected while social login is disabled"
fi

echo "-> 5/5 Dex, Bitbucket's OIDC bridge..."
DEX="${DEX_ISSUER_URL:-${PUB%/}/dex}"
dex_ready="$(kubectl -n "$NS" get deploy dex -o jsonpath='{.status.readyReplicas}/{.spec.replicas}' 2>/dev/null || true)"
if [[ " ${prov:-} " == *" bitbucket "* ]]; then
  if [ "${dex_ready%%/*}" -ge 1 ] 2>/dev/null; then
    echo "  ✓ deploy/dex ready ($dex_ready)"
  else
    echo "  ✗ the fragment offers bitbucket but deploy/dex is not ready (${dex_ready:-absent}) — kubectl -n $NS logs deploy/dex"
    fail=1
  fi
  issuer="$(curl -sf "$DEX/.well-known/openid-configuration" 2>/dev/null \
    | python3 -c "import sys,json;print(json.load(sys.stdin)['issuer'])" 2>/dev/null || true)"
  if [ "$issuer" = "$DEX" ]; then
    echo "  ✓ $DEX serves Dex's discovery document"
  else
    echo "  ✗ $DEX/.well-known/openid-configuration does not answer with issuer $DEX (got '${issuer:-nothing}') — check the dex Ingress"
    fail=1
  fi
elif [ "${dex_ready#*/}" = "0" ] || [ -z "$dex_ready" ]; then
  echo "  • no Bitbucket sign-in, and no Dex runs — expected while BEX_BITBUCKET_OIDC_* is unset"
else
  echo "  ✗ Dex runs ($dex_ready) but the fragment offers no bitbucket — re-run scripts/auth-secrets.sh"
  fail=1
fi

echo
if [ "$fail" = 0 ]; then
  echo "✓ social-login deployment checks passed for $PUB"
  echo "  Final manual step: open $PUB's dashboard login, click each provider's button, authorize."
else
  echo "✗ social login is not fully live yet (see ✗ lines above)"
  exit 1
fi
