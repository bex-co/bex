#!/usr/bin/env bash
# Create the Zot registry credentials OUT-OF-BAND of GitOps (w7/m8,
# docs/ADR022-tenant-isolation.md § Registry access control) — no secret material
# in git or Argo-managed manifests (repo rule; same posture as auth-secrets.sh).
#
# Zot denies anonymous catalog/pull/push (accessControl defaultPolicy []). Two
# bcrypt-hashed users live in the htpasswd Secret Zot mounts:
#   bex-builder  read/create/update/delete  — the build Job's push (buildkit + cosign)
#   bex-puller   read                       — tenant pods' imagePullSecret (kubelet pulls)
# The htpasswd holds only bcrypt HASHES (safe); the matching plaintext rides two
# docker-config Secrets so buildkitd authenticates the push and kubelet the pull.
#
# Reads the repo-local .env (gitignored — never commit or print it). Required keys
# (names only; values are never echoed):
#   BEX_REGISTRY_BUILDER_PASSWORD  push credential         (>= 12 chars; hex/alnum is safest)
#   BEX_REGISTRY_PULLER_PASSWORD   pull credential         (>= 12 chars; hex/alnum is safest)
# Optional (defaults match deploy/gitops/base + lego/operator/config/manager):
#   BEX_REGISTRY             registry host the docker-config `auths` key targets
#                             (default zot.bex-registry.svc:5000)
#   BEX_KPACK_REGISTRY       kpack alias of the same registry; *.local selects
#                             plain HTTP in upstream kpack (default zot.local:5000)
#   BEX_BUILD_NAMESPACE      ns the build Job runs in → where the push Secret lives
#                             (default bex-system; manager.yaml BEX_BUILD_NAMESPACE)
#   BEX_CP_APPS_NAMESPACE    ns tenant pods run in → where the pull Secret lives
#                             (default default)
#
# Secrets created (idempotent — re-run to rotate):
#   bex-registry/zot-htpasswd        key htpasswd         (bcrypt, both users)
#   <build-ns>/bex-registry-push     key config.json      (docker-config, bex-builder)
#   <build-ns>/bex-registry-push-kpack type dockerconfigjson (same credential + alias)
#   <apps-ns>/bex-registry-pull      type dockerconfigjson (bex-puller)
#
# Usage: scripts/registry-secrets.sh             # create/update the Secrets
#        DRY_RUN=1 scripts/registry-secrets.sh   # print what would be applied (names only)
# Requires: kubectl (respects $KUBECONFIG), htpasswd (apache2-utils; /usr/sbin/htpasswd on macOS).
# Generate passwords: openssl rand -hex 16
set -euo pipefail
cd "$(dirname "$0")/.."

REGISTRY_NS="${REGISTRY_NS:-bex-registry}"

# Load .env when present (local use); in CI the keys arrive as environment vars.
if [ -f .env ]; then
  set -a
  # shellcheck disable=SC1091
  source ./.env
  set +a
fi

# require NAME LEN — assert the .env key exists and is at least LEN characters.
# Never prints the value.
require() {
  local name="$1" len="$2" val="${!1:-}"
  [ -n "$val" ] || { echo "error: $name is missing or empty (.env or environment)" >&2; exit 1; }
  [ "${#val}" -ge "$len" ] || { echo "error: $name must be at least $len characters (got ${#val})" >&2; exit 1; }
}

require BEX_REGISTRY_BUILDER_PASSWORD 12
require BEX_REGISTRY_PULLER_PASSWORD 12

REGISTRY="${BEX_REGISTRY:-zot.bex-registry.svc:5000}"
KPACK_REGISTRY="${BEX_KPACK_REGISTRY:-zot.local:5000}"
BUILD_NS="${BEX_BUILD_NAMESPACE:-bex-system}"
APPS_NS="${BEX_CP_APPS_NAMESPACE:-default}"

command -v kubectl >/dev/null || { echo "error: kubectl not found" >&2; exit 1; }
command -v jq >/dev/null || { echo "error: jq not found" >&2; exit 1; }
# htpasswd ships at /usr/sbin/htpasswd on macOS; apache2-utils on Linux.
HTPASSWD_BIN="$(command -v htpasswd || echo /usr/sbin/htpasswd)"
command -v "$HTPASSWD_BIN" >/dev/null || {
  echo "error: htpasswd not found — install apache2-utils (Linux) or it ships at /usr/sbin/htpasswd on macOS" >&2
  exit 1
}

# bcrypt is the only hash algorithm Zot's htpasswd accepts (-B). -b reads the
# password from argv, -n prints `user:$2y$…` to stdout (no file written).
htpasswd_line() {
  local user="$1" pass="$2"
  "$HTPASSWD_BIN" -bBn "$user" "$pass" 2>/dev/null
}

# Docker config with both names of the same Zot endpoint. BuildKit uses the
# canonical Service name; kpack uses the *.local alias so its upstream registry
# client selects HTTP. jq handles arbitrary password characters safely.
registry_config() {
  local username="$1" password="$2"
  jq -cn \
    --arg registry "$REGISTRY" \
    --arg kpackRegistry "$KPACK_REGISTRY" \
    --arg username "$username" \
    --arg password "$password" \
    '{auths: {($registry): {username: $username, password: $password}}}
     | if $kpackRegistry == $registry then .
       else .auths[$kpackRegistry] = {username: $username, password: $password}
       end'
}

if [ "${DRY_RUN:-}" = "1" ]; then
  echo "would ensure namespace $REGISTRY_NS"
  echo "would apply secret $REGISTRY_NS/zot-htpasswd (key htpasswd, users: bex-builder bex-puller)"
  echo "would apply secret $BUILD_NS/bex-registry-push (key config.json, user bex-builder → $REGISTRY)"
  echo "would apply secret $BUILD_NS/bex-registry-push-kpack (dockerconfigjson, user bex-builder → $KPACK_REGISTRY)"
  echo "would apply secret $APPS_NS/bex-registry-pull (dockerconfigjson, user bex-puller → $REGISTRY + $KPACK_REGISTRY)"
  echo "would apply secret $BUILD_NS/bex-registry-pull (dockerconfigjson, for the publish Job)"
  exit 0
fi

# bex-registry + the build namespace must exist (the zot Application creates
# bex-registry with CreateNamespace=true, but the build namespace — where the
# push Secret lands — may not yet; this script may run first locally).
kubectl get namespace "$REGISTRY_NS" >/dev/null 2>&1 || kubectl create namespace "$REGISTRY_NS" >/dev/null
kubectl get namespace "$BUILD_NS" >/dev/null 2>&1 || kubectl create namespace "$BUILD_NS" >/dev/null

# 1. htpasswd (bcrypt) Zot mounts via externalSecrets (deploy/gitops/base/zot.yaml).
HTPASSWD="$(htpasswd_line bex-builder "$BEX_REGISTRY_BUILDER_PASSWORD")
$(htpasswd_line bex-puller "$BEX_REGISTRY_PULLER_PASSWORD")"
kubectl create secret generic zot-htpasswd -n "$REGISTRY_NS" \
  --from-literal=htpasswd="$HTPASSWD" \
  --dry-run=client -o yaml | kubectl apply -f - >/dev/null

# 2. Push credential (build namespace) — docker-config at key config.json.
kubectl create secret generic bex-registry-push -n "$BUILD_NS" \
  --from-literal=config.json="$(registry_config bex-builder "$BEX_REGISTRY_BUILDER_PASSWORD")" \
  --dry-run=client -o yaml | kubectl apply -f - >/dev/null

# kpack reads only the standard dockerconfigjson Secret type/key. The operator
# also refreshes this derived copy at dispatch time; creating it here unblocks
# the cluster-scoped builder before any App exists.
kubectl create secret generic bex-registry-push-kpack -n "$BUILD_NS" \
  --type=kubernetes.io/dockerconfigjson \
  --from-literal=.dockerconfigjson="$(registry_config bex-builder "$BEX_REGISTRY_BUILDER_PASSWORD")" \
  --dry-run=client -o yaml | kubectl apply -f - >/dev/null

# 3. Pull credential (imagePullSecret, kubelet reads it). Created in BOTH the
# apps namespace (tenant Deployments/CronJobs pull here) AND the build namespace
# (the static-site publish Job's extract initContainer pulls the built image here).
for pull_ns in "$APPS_NS" "$BUILD_NS"; do
  kubectl get namespace "$pull_ns" >/dev/null 2>&1 || kubectl create namespace "$pull_ns" >/dev/null
  kubectl create secret generic bex-registry-pull -n "$pull_ns" \
    --type=kubernetes.io/dockerconfigjson \
    --from-literal=.dockerconfigjson="$(registry_config bex-puller "$BEX_REGISTRY_PULLER_PASSWORD")" \
    --dry-run=client -o yaml | kubectl apply -f - >/dev/null
done

echo "applied: $REGISTRY_NS/zot-htpasswd, $BUILD_NS/bex-registry-push{,-kpack}, $APPS_NS/bex-registry-pull"
echo "remember to rollout restart zot so it re-reads the htpasswd:  kubectl rollout restart statefulset zot -n $REGISTRY_NS"
