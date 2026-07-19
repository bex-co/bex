#!/usr/bin/env bash
# verify-registry-auth.sh — proves the bootstrap registry-auth baseline
# (docs/ADR022-tenant-isolation.md § Registry access control).
#
# Runs from a build-labeled probe pod (so it clears the bex-registry NetworkPolicy
# the way a real tenant build does) and asserts the APPLICATION layer:
#   DENY  anonymous GET /v2/_catalog            (cross-tenant enumeration)  → 401
#   DENY  anonymous PUT /v2/.../manifests/...    (tag-overwrite poisoning)  → 401
#   ALLOW bex-builder  GET /v2/_catalog          (bootstrap cred works)    → 200
#
# This complements verify-tenant-isolation.sh (network policy) and
# verify-per-app-registry-isolation.sh / verify-build-isolation.sh (per-App
# repository scope and build-phase custody). A legacy shared bex-puller no
# longer exists.
#
# The credentialed probes need the plaintext passwords (to build the Basic auth
# header); they are read from .env (gitignored) — the same keys
# scripts/registry-secrets.sh mints the Secrets from.
#
# Usage:  bash scripts/verify-registry-auth.sh
# Exit 0 on full compliance; non-zero with the failing probe on stdout.
# Requires: kubectl, a pod image with curl (nicolaka/netshoot), .env with the
# BEX_REGISTRY_BUILDER_PASSWORD key.
set -euo pipefail
cd "$(dirname "$0")/.."

NS="${BUILD_NS:-bex-build}"
REGISTRY_NS="${REGISTRY_NS:-bex-registry}"
ZOT="zot.${REGISTRY_NS}.svc:5000"
PROBE_IMAGE="${PROBE_IMAGE:-nicolaka/netshoot}"
PROBE="verify-regauth"
TIMEOUT="${TIMEOUT:-5}"

if [ -f .env ]; then
  set -a
  # shellcheck disable=SC1091
  source ./.env
  set +a
fi
: "${BEX_REGISTRY_BUILDER_PASSWORD:?BEX_REGISTRY_BUILDER_PASSWORD required in .env for the credentialed probes}"

b64() { printf '%s' "$1" | base64 | tr -d '\n'; }
BUILDER_AUTH="$(b64 "bex-builder:$BEX_REGISTRY_BUILDER_PASSWORD")"

ok()   { echo "  PASS  $1"; }
fail() { echo "  FAIL  $1" >&2; exit 1; }

# http_code <pod> <curl-args...> — prints just the HTTP status code.
http_code() {
  local pod="$1"; shift
  kubectl exec -n "$NS" "$pod" -- curl -s -o /dev/null -w '%{http_code}' --max-time "$TIMEOUT" "$@"
}

cleanup() { kubectl delete pod "$PROBE" -n "$NS" --ignore-not-found &>/dev/null || true; }
trap cleanup EXIT

# A build-labeled pod is inside the bex-registry allow-list (network-policies.yaml)
# — exactly the position a tenant build pod is in. Auth must still refuse it
# without a credential.
kubectl run "$PROBE" -n "$NS" --image="$PROBE_IMAGE" --restart=Never \
  --overrides='{"metadata":{"labels":{"app.bex.co/component":"build"}}}' \
  --command -- sleep 600 >/dev/null
kubectl wait --for=condition=Ready pod/"$PROBE" -n "$NS" --timeout=120s >/dev/null \
  || fail "probe pod $PROBE never became Ready (image pull? netpol?)"

echo "registry-auth probes (from build-labeled pod $NS/$PROBE → $ZOT):"

# --- DENY: anonymous enumeration + poisoning ---
code="$(http_code "$PROBE" "http://$ZOT/v2/_catalog")"
[ "$code" = "401" ] && ok "anonymous GET /v2/_catalog → 401 (enumeration denied)" \
  || fail "anonymous GET /v2/_catalog → $code (expected 401 — auth is off or anonymous is allowed)"

code="$(http_code "$PROBE" -X PUT -H 'Content-Type: application/vnd.oci.image.manifest.v1+json' \
  -d '{}' "http://$ZOT/v2/authprobe/manifests/latest")"
case "$code" in 401|403) ok "anonymous PUT /v2/authprobe/manifests/latest → $code (push denied)" \
  ;; *) fail "anonymous push → $code (expected 401/403 — anyone can poison a tag)" ;; esac

# --- ALLOW: credentialed catalog access (proves the Secrets work) ---
code="$(http_code "$PROBE" -H "Authorization: Basic $BUILDER_AUTH" "http://$ZOT/v2/_catalog")"
[ "$code" = "200" ] && ok "bex-builder GET /v2/_catalog → 200" \
  || fail "bex-builder GET /v2/_catalog → $code (expected 200 — bootstrap credential / htpasswd drift)"

echo "registry-auth: ALL PROBES PASS"
