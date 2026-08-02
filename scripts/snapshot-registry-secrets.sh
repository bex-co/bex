#!/usr/bin/env bash
# Create the sandbox snapshot push credential OUT-OF-BAND of GitOps (w3/m42
# t002, docs/ADR042-sandbox-cluster-substrate.md § D5) — no secret material in
# git or Argo-managed manifests (repo rule; the registry-secrets.sh posture).
#
# The patched OpenSandbox controller (deploy/opensandbox/patches/) runs its
# privileged snapshot commit Jobs in the dedicated `opensandbox-snapshot`
# namespace. The commit Job's nerdctl push authenticates to Zot with the Secret
# named by the chart's controller.snapshot.snapshotPushSecret; Zot denies
# anonymous push (w7/m8). bex-builder already carries Zot's adminPolicy
# (read/create/update/delete on every repository — lego/operator/internal/
# registry/creds.go ensureZotBuilderAdminPolicy), so no new htpasswd user or
# ACL entry is needed for the snapshots/ repositories: this script materializes
# the existing out-of-band builder credential in the snapshot-job namespace.
#
# NOTE the resume-side credential is NOT created here: the controller injects
# controller.snapshot.resumePullSecret into the BatchSandbox pod template's
# imagePullSecrets, and Secret references are namespace-local — so it must
# exist in every tenant `<ws>-sandbox` namespace. A shared read credential
# there would let any tenant pull other tenants' snapshot images (rootfs may
# embed secrets), so that leg needs per-workspace scoping (NamespaceReconciler
# minting, the reg-pull-<name> pattern) — designed and reviewed in w3/m42 t002
# before production snapshot transport is enabled.
#
# Reads the repo-local .env (gitignored — never commit or print it). Required
# keys (names only; values are never echoed):
#   BEX_REGISTRY_BUILDER_PASSWORD  the bex-builder push credential (>= 12 chars)
# Optional:
#   BEX_REGISTRY              registry host in the docker-config auths key
#                             (default zot.bex-registry.svc:5000)
#   BEX_SNAPSHOT_JOB_NS       namespace the commit Jobs run in
#                             (default opensandbox-snapshot — chart value
#                             controller.snapshot.jobNamespace)
#
# Secrets created (idempotent — re-run to rotate):
#   <snapshot-ns>/bex-snapshot-push  type dockerconfigjson (user bex-builder)
#
# Usage: scripts/snapshot-registry-secrets.sh             # create/update
#        DRY_RUN=1 scripts/snapshot-registry-secrets.sh   # print names only
# Requires: kubectl (respects $KUBECONFIG), jq.
set -euo pipefail
cd "$(dirname "$0")/.."

# Load .env when present (local use); in CI the keys arrive as environment vars.
if [ -f .env ]; then
  set -a
  # shellcheck disable=SC1091
  source ./.env
  set +a
fi

require() {
  local name="$1" len="$2" val="${!1:-}"
  [ -n "$val" ] || { echo "error: $name is missing or empty (.env or environment)" >&2; exit 1; }
  [ "${#val}" -ge "$len" ] || { echo "error: $name must be at least $len characters (got ${#val})" >&2; exit 1; }
}

require BEX_REGISTRY_BUILDER_PASSWORD 12

REGISTRY="${BEX_REGISTRY:-zot.bex-registry.svc:5000}"
SNAPSHOT_NS="${BEX_SNAPSHOT_JOB_NS:-opensandbox-snapshot}"

command -v kubectl >/dev/null || { echo "error: kubectl not found" >&2; exit 1; }
command -v jq >/dev/null || { echo "error: jq not found" >&2; exit 1; }

# jq handles arbitrary password characters safely; never echo the value.
registry_config() {
  jq -cn \
    --arg registry "$REGISTRY" \
    --arg username bex-builder \
    --arg password "$BEX_REGISTRY_BUILDER_PASSWORD" \
    '{auths: {($registry): {username: $username, password: $password,
                            auth: (($username + ":" + $password) | @base64)}}}'
}

if [ "${DRY_RUN:-}" = "1" ]; then
  echo "would ensure namespace $SNAPSHOT_NS"
  echo "would apply secret $SNAPSHOT_NS/bex-snapshot-push (dockerconfigjson, user bex-builder → $REGISTRY)"
  exit 0
fi

# The chart creates opensandbox-snapshot (privileged PSS + Job Role), but this
# script may run before the first Argo sync on a fresh cluster.
kubectl get namespace "$SNAPSHOT_NS" >/dev/null 2>&1 || kubectl create namespace "$SNAPSHOT_NS" >/dev/null

# The commit Job mounts key .dockerconfigjson at /var/run/opensandbox/registry/
# config.json (buildCommitJob's KeyToPath), so the standard dockerconfigjson
# Secret type is required.
kubectl create secret generic bex-snapshot-push -n "$SNAPSHOT_NS" \
  --type=kubernetes.io/dockerconfigjson \
  --from-literal=.dockerconfigjson="$(registry_config)" \
  --dry-run=client -o yaml | kubectl apply -f - >/dev/null

echo "applied: $SNAPSHOT_NS/bex-snapshot-push (bex-builder → $REGISTRY)"
echo "enable transport via controller.snapshot values (registry + snapshotPushSecret=bex-snapshot-push + containerdSocketPath); resume-pull scoping is designed in w3/m42 t002 before prod enablement"
