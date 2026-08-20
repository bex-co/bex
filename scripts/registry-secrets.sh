#!/usr/bin/env bash
# Create the Zot registry credentials OUT-OF-BAND of GitOps (w7/m8,
# docs/ADR022-tenant-isolation.md § Registry access control) — no secret material
# in git or Argo-managed manifests (repo rule; same posture as auth-secrets.sh).
#
# Per-App pull credentials (w7/m36): the bex-puller shared credential is no longer
# used. Each App gets its own htpasswd user and a per-repo Zot ACL
# entry, managed dynamically by the operator (legacy "app-<name>"; labeled
# Apps "app-<tea-id>-<name>" per docs/ADR073). This script:
#   - refreshes bex-builder while preserving operator-managed app-* users
#     already present in zot-htpasswd (bex-puller remains removed);
#   - does NOT create bex-registry-pull (deprecated; superseded by per-App
#     "reg-pull-<name>" / "reg-pull-<tea-id>-<name>" Secrets in the apps
#     namespace created by the operator).
#   - creates bex-registry-pull in the BUILD namespace only (for the static-site
#     publish Job's extract initContainer which uses the push credential path).
#
# Zot denies anonymous catalog/pull/push (accessControl defaultPolicy []). The
# htpasswd holds only bcrypt HASHES (safe); the matching plaintext rides:
#   bex-builder  bootstrap Secret — registry seeding, webhook reads, and the
#                                   legacy shared-auth fallback only
#   app-<name> / app-<tea-id>-<name>
#                per-App Secret   — post-build push/sign plus kubelet pulls,
#                                   scoped by Zot to that App's repository
#
# Reads the repo-local .env (gitignored — never commit or print it). Required keys
# (names only; values are never echoed):
#   BEX_REGISTRY_BUILDER_PASSWORD  push credential         (>= 12 chars; hex/alnum is safest)
# Optional (defaults match deploy/gitops/base + lego/operator/config/manager):
#   BEX_REGISTRY             registry host the docker-config `auths` key targets
#                             (default zot.bex-registry.svc:5000)
#   BEX_KPACK_REGISTRY       kpack alias of the same registry; *.local selects
#                             plain HTTP in upstream kpack (default zot.local:5000)
#   BEX_BUILD_NAMESPACE      ns the build Job runs in → where the push Secret lives
#                             (default bex-build; manager.yaml BEX_BUILD_NAMESPACE)
#   BEX_KPACK_NAMESPACE      ns of the cluster-scoped builder ServiceAccount
#                             (default bex-system; charts/kpack/platform.yaml)
#
# Secrets created (idempotent — re-run to rotate):
#   bex-registry/zot-htpasswd        key htpasswd   (bcrypt, bex-builder + preserved per-App users)
#   <build-ns>/bex-registry-push     key config.json  (docker-config, bex-builder)
#   <build-ns>/bex-registry-push-kpack type dockerconfigjson (same credential + alias)
#   <build-ns>/bex-registry-pull     type dockerconfigjson (bex-builder cred for publish Job)
#   <kpack-ns>/bex-registry-push-kpack type dockerconfigjson (ClusterBuilder push credential)
#
# Usage: scripts/registry-secrets.sh             # create/update the Secrets
#        DRY_RUN=1 scripts/registry-secrets.sh   # print what would be applied (names only)
# Requires: kubectl (respects $KUBECONFIG), htpasswd (apache2-utils; /usr/sbin/htpasswd on macOS).
# Generate passwords: openssl rand -hex 16
set -euo pipefail
script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
cd "$(dirname "$0")/.."
# shellcheck source=lib/secret-install.sh
. "$script_dir/lib/secret-install.sh"

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

REGISTRY="${BEX_REGISTRY:-zot.bex-registry.svc:5000}"
KPACK_REGISTRY="${BEX_KPACK_REGISTRY:-zot.local:5000}"
BUILD_NS="${BEX_BUILD_NAMESPACE:-bex-build}"
KPACK_NS="${BEX_KPACK_NAMESPACE:-bex-system}"

command -v kubectl >/dev/null || { echo "error: kubectl not found" >&2; exit 1; }
command -v jq >/dev/null || { echo "error: jq not found" >&2; exit 1; }
# htpasswd ships at /usr/sbin/htpasswd on macOS; apache2-utils on Linux.
HTPASSWD_BIN="$(command -v htpasswd || echo /usr/sbin/htpasswd)"
command -v "$HTPASSWD_BIN" >/dev/null || {
  echo "error: htpasswd not found — install apache2-utils (Linux) or it ships at /usr/sbin/htpasswd on macOS" >&2
  exit 1
}

# bcrypt is the only hash algorithm Zot's htpasswd accepts (-B). Read the
# password from stdin so it never appears in htpasswd's argv; -n prints the
# `user:$2y$…` record to stdout (no file written).
htpasswd_line() {
	local user="$1" pass="$2"
	printf '%s\n' "$pass" | "$HTPASSWD_BIN" -nB -i "$user" 2>/dev/null
}

# Docker config with both names of the same Zot endpoint. BuildKit uses the
# canonical Service name; kpack uses the *.local alias so its upstream registry
# client selects HTTP. jq handles arbitrary password characters safely.
registry_config() {
	local username="$1" password="$2"
	printf '%s' "$password" | jq -Rsn \
	  --arg registry "$REGISTRY" \
	  --arg kpackRegistry "$KPACK_REGISTRY" \
	  --arg username "$username" \
	  'input as $password | {auths: {($registry): {username: $username, password: $password,
                            auth: (($username + ":" + $password) | @base64)}}}
     | if $kpackRegistry == $registry then .
       else .auths[$kpackRegistry] = {username: $username, password: $password,
                                      auth: (($username + ":" + $password) | @base64)}
       end'
}

if [ "${DRY_RUN:-}" = "1" ]; then
  echo "would ensure namespace $REGISTRY_NS"
  echo "would apply secret $REGISTRY_NS/zot-htpasswd (key htpasswd, refresh bex-builder + preserve existing app-* users)"
  echo "would apply secret $BUILD_NS/bex-registry-push (key config.json, user bex-builder → $REGISTRY)"
  echo "would apply secret $BUILD_NS/bex-registry-push-kpack (dockerconfigjson, user bex-builder → $KPACK_REGISTRY)"
  echo "would apply secret $BUILD_NS/bex-registry-pull (dockerconfigjson, bex-builder for publish Job)"
  if [ "$KPACK_NS" != "$BUILD_NS" ]; then
    echo "would apply secret $KPACK_NS/bex-registry-push-kpack (dockerconfigjson, ClusterBuilder user bex-builder → $KPACK_REGISTRY)"
  fi
  echo "NOTE: bex-registry-pull in apps namespace is no longer created — operator mints reg-pull-<name> per App"
  exit 0
fi

# bex-registry + the build namespace must exist (the zot Application creates
# bex-registry with CreateNamespace=true, but the build namespace — where the
# push Secret lands — may not yet; this script may run first locally).
kubectl get namespace "$REGISTRY_NS" >/dev/null 2>&1 || kubectl create namespace "$REGISTRY_NS" >/dev/null
kubectl get namespace "$BUILD_NS" >/dev/null 2>&1 || kubectl create namespace "$BUILD_NS" >/dev/null
kubectl get namespace "$KPACK_NS" >/dev/null 2>&1 || kubectl create namespace "$KPACK_NS" >/dev/null

# 1. htpasswd (bcrypt) Zot mounts via externalSecrets (deploy/gitops/base/zot.yaml).
# Refresh the out-of-band builder hash but retain the operator's existing
# repository-scoped App identities. Re-running this rotation script must not
# invalidate every live App until its next reconcile. Unknown and deprecated
# users (including bex-puller) are deliberately not carried forward.
EXISTING_HTPASSWD="$(kubectl get secret zot-htpasswd -n "$REGISTRY_NS" -o json 2>/dev/null \
  | jq -r '.data.htpasswd | @base64d' 2>/dev/null || true)"
HTPASSWD="$({
  printf '%s\n' "$(htpasswd_line bex-builder "$BEX_REGISTRY_BUILDER_PASSWORD")"
  printf '%s\n' "$EXISTING_HTPASSWD" | awk -F: '$1 ~ /^app-/ && NF == 2 { print }'
} | awk -F: 'NF == 2 && !seen[$1]++ { print }')"
apply_secret "$REGISTRY_NS" zot-htpasswd Opaque htpasswd "$HTPASSWD"

# 2. Push credential (build namespace) — docker-config at key config.json.
apply_secret "$BUILD_NS" bex-registry-push Opaque config.json "$(registry_config bex-builder "$BEX_REGISTRY_BUILDER_PASSWORD")"

# kpack reads only the standard dockerconfigjson Secret type/key. The operator
# also refreshes this derived copy at dispatch time; creating it here unblocks
# the cluster-scoped builder before any App exists.
apply_secret "$BUILD_NS" bex-registry-push-kpack kubernetes.io/dockerconfigjson \
  .dockerconfigjson "$(registry_config bex-builder "$BEX_REGISTRY_BUILDER_PASSWORD")"

# The cluster-scoped `bex` ClusterBuilder uses a ServiceAccount in bex-system,
# independently of tenant Image/Build objects in the build namespace. Kubernetes
# Secret references are namespace-local, so it needs the same out-of-band
# credential in its own namespace. Without this copy the controller reaches Zot
# anonymously and the builder remains NoLatestImage with a 401.
if [ "$KPACK_NS" != "$BUILD_NS" ]; then
  apply_secret "$KPACK_NS" bex-registry-push-kpack kubernetes.io/dockerconfigjson \
    .dockerconfigjson "$(registry_config bex-builder "$BEX_REGISTRY_BUILDER_PASSWORD")"
fi

# 3. Pull credential for the build namespace only — used by the static-site publish
# Job's extract initContainer to pull the built image. The builder password is used
# here because bex-builder has read access in the ** wildcard ACL.
# NOTE: the apps-namespace bex-registry-pull is no longer created; the operator
# mints per-App "reg-pull-<name>" Secrets (w7/m36, docs/ADR022-tenant-isolation.md).
kubectl get namespace "$BUILD_NS" >/dev/null 2>&1 || kubectl create namespace "$BUILD_NS" >/dev/null
apply_secret "$BUILD_NS" bex-registry-pull kubernetes.io/dockerconfigjson \
  .dockerconfigjson "$(registry_config bex-builder "$BEX_REGISTRY_BUILDER_PASSWORD")"

echo "applied: $REGISTRY_NS/zot-htpasswd (bex-builder + preserved app-* users), $BUILD_NS/bex-registry-push{,-kpack,-pull}, $KPACK_NS/bex-registry-push-kpack"
echo "operator will mint per-App reg-pull-<name> / reg-pull-<tea-id>-<name> Secrets and add app-* htpasswd entries as Apps are reconciled"
echo "after first reconcile, restart Zot to load initial zot-config:  kubectl rollout restart statefulset zot -n $REGISTRY_NS"
