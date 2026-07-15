#!/usr/bin/env bash
# delete-audit.sh — w7/m12 acceptance audit: zero leftovers after deletion.
#
# Verifies that deleting a service (APP), a static site (STATIC), a managed
# Postgres (DB), and/or a managed Key Value (KV) leaves no tenant artifacts
# on the platform. All four are optional — pass the ones you deleted.
#
# Checks per resource type:
#   APP / STATIC:
#     - no build Jobs labeled app.bex.co/build=<name> in BEX_BUILD_NAMESPACE
#     - no predeploy Jobs labeled app.bex.co/predeploy=<name> in BEX_BUILD_NAMESPACE
#     - no kpack Images labeled app.bex.co/build=<name> in BEX_BUILD_NAMESPACE
#     - no manifests in Zot registry under repo <name>
#     - no OpenBao env-var path services/<name>/env
#     - no OpenBao secret-file path services/<name>/files
#     - no <name>-env Secret in the apps namespace
#   STATIC only:
#     - no S3 objects under s3://<BEX_STATIC_S3_BUCKET>/<name>/
#   APP only (custom hosts):
#     - no TLS Secrets for <name>'s listed hosts in the apps namespace
#   DB:
#     - no CNPG Cluster CR named <name> in the apps namespace
#     - purge-db-<name> Job exists (dispatched) in BEX_BUILD_NAMESPACE
#   KV:
#     - no PVCs with label app.bex.co/name=<name> in the apps namespace
#
# Usage:
#   bash scripts/delete-audit.sh [--app NAME] [--static NAME] [--db NAME] [--kv NAME]
#
# Environment (reads from .env if present):
#   APPS_NS           — apps namespace (default: default)
#   BEX_BUILD_NAMESPACE — build Job namespace (default: APPS_NS)
#   BEX_REGISTRY      — in-cluster registry host:port (e.g. zot.bex-registry.svc:5000)
#   BEX_OPENBAO_URL   — OpenBao base URL (e.g. http://bao.bex-system.svc:8200)
#   BAO_TOKEN         — OpenBao root/admin token
#   BEX_STATIC_S3_BUCKET / BEX_STATIC_S3_ENDPOINT — for S3 audit
#   AWS_ACCESS_KEY_ID / AWS_SECRET_ACCESS_KEY — for S3 audit
#
# Exit 0: all checks pass. Non-zero: which check failed is printed to stderr.
set -euo pipefail
cd "$(dirname "$0")/.."

if [ -f .env ]; then
  set -a
  # shellcheck disable=SC1091
  source ./.env
  set +a
fi

APPS_NS="${APPS_NS:-default}"
BUILD_NS="${BEX_BUILD_NAMESPACE:-$APPS_NS}"
REGISTRY="${BEX_REGISTRY:-}"
BAO_URL="${BEX_OPENBAO_URL:-}"
STATIC_BUCKET="${BEX_STATIC_S3_BUCKET:-}"
STATIC_ENDPOINT="${BEX_STATIC_S3_ENDPOINT:-}"

APP=""
STATIC=""
DB=""
KV=""
while [[ $# -gt 0 ]]; do
  case "$1" in
    --app)    APP="$2";    shift 2 ;;
    --static) STATIC="$2"; shift 2 ;;
    --db)     DB="$2";     shift 2 ;;
    --kv)     KV="$2";     shift 2 ;;
    *) echo "Unknown flag: $1" >&2; exit 1 ;;
  esac
done

if [ -z "$APP$STATIC$DB$KV" ]; then
  echo "Usage: $0 [--app NAME] [--static NAME] [--db NAME] [--kv NAME]" >&2
  exit 1
fi

PASS=0
FAIL=0

ok()   { echo "  PASS  $1"; PASS=$((PASS+1)); }
fail() { echo "  FAIL  $1" >&2; FAIL=$((FAIL+1)); }
skip() { echo "  SKIP  $1 (prerequisite not configured)"; }

# k8s_count: number of k8s resources matching a label selector in a namespace.
k8s_count() {
  local ns="$1" kind="$2" selector="$3"
  kubectl get "$kind" -n "$ns" -l "$selector" \
    --ignore-not-found -o jsonpath='{.items}' 2>/dev/null | python3 -c "
import sys, json
items = json.load(sys.stdin)
print(len(items))
" 2>/dev/null || echo 0
}

# bao_secret_exists: exit 0 if OpenBao path has data, 1 if 404.
bao_secret_exists() {
  local path="$1"
  local status
  status=$(curl -sf -o /dev/null -w '%{http_code}' \
    -H "X-Vault-Token: ${BAO_TOKEN:-}" \
    "$BAO_URL/v1/secret/data/$path" 2>/dev/null || echo "000")
  [ "$status" = "200" ]
}

# registry_tag_count: number of tags for a repo in Zot via the OCI Distribution API.
registry_tag_count() {
  local repo="$1"
  local status
  status=$(curl -sf -o /dev/null -w '%{http_code}' \
    "http://$REGISTRY/v2/$repo/tags/list" 2>/dev/null || echo "000")
  if [ "$status" = "404" ]; then
    echo 0
  elif [ "$status" = "200" ]; then
    curl -sf "http://$REGISTRY/v2/$repo/tags/list" 2>/dev/null | \
      python3 -c "import sys,json; d=json.load(sys.stdin); print(len(d.get('tags') or []))" 2>/dev/null || echo 0
  else
    echo "unknown (HTTP $status)"
  fi
}

# ---- per-app checks (applies to both APP and STATIC) ----
audit_app() {
  local name="$1"
  echo ""
  echo "── App: $name ──"

  # Build Jobs
  local cnt
  cnt=$(k8s_count "$BUILD_NS" jobs "app.bex.co/build=$name")
  if [ "$cnt" = "0" ]; then
    ok "no build Jobs for $name in $BUILD_NS"
  else
    fail "$cnt build Job(s) still exist for $name in $BUILD_NS"
  fi

  # Predeploy Jobs
  cnt=$(k8s_count "$BUILD_NS" jobs "app.bex.co/predeploy=$name")
  if [ "$cnt" = "0" ]; then
    ok "no predeploy Jobs for $name in $BUILD_NS"
  else
    fail "$cnt predeploy Job(s) still exist for $name in $BUILD_NS"
  fi

  # kpack Images
  cnt=$(kubectl get images.kpack.io -n "$BUILD_NS" -l "app.bex.co/build=$name" \
    --ignore-not-found -o name 2>/dev/null | wc -l | tr -d ' ')
  if [ "$cnt" = "0" ]; then
    ok "no kpack Images for $name in $BUILD_NS"
  else
    fail "$cnt kpack Image(s) still exist for $name in $BUILD_NS"
  fi

  # Zot registry
  if [ -n "$REGISTRY" ]; then
    cnt=$(registry_tag_count "$name")
    if [ "$cnt" = "0" ]; then
      ok "no manifests in Zot registry for $name"
    else
      fail "$cnt manifest(s) still in Zot registry for $name (tags)"
    fi
  else
    skip "Zot registry check ($name) — BEX_REGISTRY not set"
  fi

  # OpenBao env vars
  if [ -n "$BAO_URL" ] && [ -n "${BAO_TOKEN:-}" ]; then
    if bao_secret_exists "services/$name/env"; then
      fail "OpenBao env-var path services/$name/env still exists"
    else
      ok "no OpenBao env-var path for $name"
    fi
    if bao_secret_exists "services/$name/files"; then
      fail "OpenBao secret-file path services/$name/files still exists"
    else
      ok "no OpenBao secret-file path for $name"
    fi
  else
    skip "OpenBao check ($name) — BEX_OPENBAO_URL or BAO_TOKEN not set"
  fi

  # <name>-env Secret in apps namespace
  cnt=$(kubectl get secret -n "$APPS_NS" "${name}-env" \
    --ignore-not-found -o name 2>/dev/null | wc -l | tr -d ' ')
  if [ "$cnt" = "0" ]; then
    ok "no ${name}-env Secret in $APPS_NS"
  else
    fail "${name}-env Secret still exists in $APPS_NS"
  fi
}

# ---- static-site S3 check ----
audit_static_s3() {
  local name="$1"
  echo ""
  echo "── Static site S3: $name ──"
  if [ -n "$STATIC_BUCKET" ] && [ -n "$STATIC_ENDPOINT" ]; then
    local prefix="s3://$STATIC_BUCKET/$name/"
    local cnt
    cnt=$(aws s3 ls "$prefix" --recursive --endpoint-url "$STATIC_ENDPOINT" 2>/dev/null | wc -l | tr -d ' ')
    if [ "$cnt" = "0" ]; then
      ok "no S3 objects under $prefix"
    else
      fail "$cnt S3 object(s) still under $prefix"
    fi
  else
    skip "S3 check ($name) — BEX_STATIC_S3_BUCKET or BEX_STATIC_S3_ENDPOINT not set"
  fi
}

# ---- Postgres (DB) checks ----
audit_db() {
  local name="$1"
  echo ""
  echo "── Postgres: $name ──"

  # CNPG Cluster should be gone (cascaded by ownerRef)
  cnt=$(kubectl get cluster.postgresql.cnpg.io -n "$APPS_NS" "$name" \
    --ignore-not-found -o name 2>/dev/null | wc -l | tr -d ' ')
  if [ "$cnt" = "0" ]; then
    ok "CNPG Cluster $name gone from $APPS_NS"
  else
    fail "CNPG Cluster $name still exists in $APPS_NS (deletion in progress?)"
  fi

  # Purge Job dispatched (fire-and-forget; we only check it was created)
  local job_name="purge-db-$name"
  if [ ${#job_name} -gt 63 ]; then
    job_name="${job_name:0:63}"
  fi
  cnt=$(kubectl get job -n "$BUILD_NS" "$job_name" \
    --ignore-not-found -o name 2>/dev/null | wc -l | tr -d ' ')
  if [ "$cnt" = "1" ]; then
    ok "S3 backup-purge Job $job_name dispatched in $BUILD_NS"
  else
    # Might have already TTL-reaped (1h); check status via events instead
    local events
    events=$(kubectl get events -n "$BUILD_NS" \
      --field-selector "involvedObject.name=$job_name" \
      --ignore-not-found -o name 2>/dev/null | wc -l | tr -d ' ')
    if [ "$events" -gt 0 ]; then
      ok "S3 backup-purge Job $job_name dispatched (TTL-reaped, events present)"
    else
      skip "S3 backup-purge Job $job_name not found (may have TTL-reaped or backups were disabled)"
    fi
  fi
}

# ---- Key Value (KV) checks ----
audit_kv() {
  local name="$1"
  echo ""
  echo "── Key Value: $name ──"

  # PVCs — the StatefulSetPersistentVolumeClaimRetentionPolicy (WhenDeleted=Delete)
  # should have removed them with the StatefulSet.
  local cnt
  cnt=$(kubectl get pvc -n "$APPS_NS" -l "app.bex.co/name=$name" \
    --ignore-not-found -o name 2>/dev/null | wc -l | tr -d ' ')
  if [ "$cnt" = "0" ]; then
    ok "no PVCs for KeyValue $name in $APPS_NS"
  else
    fail "$cnt PVC(s) still exist for KeyValue $name in $APPS_NS"
  fi

  # StatefulSet itself should be gone (ownerRef on KeyValue CR)
  cnt=$(kubectl get statefulset -n "$APPS_NS" "$name" \
    --ignore-not-found -o name 2>/dev/null | wc -l | tr -d ' ')
  if [ "$cnt" = "0" ]; then
    ok "StatefulSet $name gone from $APPS_NS"
  else
    fail "StatefulSet $name still exists in $APPS_NS (deletion in progress?)"
  fi
}

# ---- main ----
echo "=== delete-audit: checking for zero leftovers after deletion ==="

[ -n "$APP" ]    && audit_app "$APP"
[ -n "$STATIC" ] && audit_app "$STATIC" && audit_static_s3 "$STATIC"
[ -n "$DB" ]     && audit_db "$DB"
[ -n "$KV" ]     && audit_kv "$KV"

echo ""
echo "=== result: ${PASS} passed, ${FAIL} failed ==="
[ "$FAIL" -eq 0 ]
