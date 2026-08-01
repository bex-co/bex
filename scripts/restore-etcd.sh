#!/usr/bin/env bash
# ADR011 Path A: restore a snapshot only into a local throwaway etcd, extract
# selected bex CRs, and optionally apply the sanitized manifests to an explicitly
# named restore-* kube context. This script cannot perform Path B/in-place work.
set -euo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$HERE/.." && pwd)"
# shellcheck source=scripts/lib/restore.sh
source "$HERE/lib/restore.sh"

usage() {
  cat <<'EOF'
Usage:
  DRY_RUN=1 scripts/restore-etcd.sh [--snapshot latest|s3://bucket/key.db.gz] \
    [--image registry.k8s.io/etcd:TAG] [--output-dir DIR]

  scripts/restore-etcd.sh [same options] --output-dir DIR

  scripts/restore-etcd.sh [same options] --apply --target-context restore-NAME \
    --confirm APPLY-restore-NAME

Default extracted prefixes are Apps, Databases, and KeyValues. --prefix may be
repeated for another /registry/... prefix. With no --apply, Kubernetes is never
contacted. Apply is permitted only to a kube context whose name starts restore-.
EOF
}

SNAPSHOT="latest"
IMAGE="registry.k8s.io/etcd:3.6.5-0"
OUTPUT_DIR=""
APPLY=0
TARGET_CONTEXT=""
CONFIRM=""
PREFIXES=(
  /registry/app.bex.co/apps/
  /registry/app.bex.co/databases/
  /registry/app.bex.co/keyvalues/
)
CUSTOM_PREFIX=0

while [ "$#" -gt 0 ]; do
  case "$1" in
    --snapshot) SNAPSHOT="${2:-}"; shift 2 ;;
    --image) IMAGE="${2:-}"; shift 2 ;;
    --output-dir) OUTPUT_DIR="${2:-}"; shift 2 ;;
    --prefix)
      [ "$CUSTOM_PREFIX" -eq 1 ] || { PREFIXES=(); CUSTOM_PREFIX=1; }
      PREFIXES+=("${2:-}"); shift 2 ;;
    --apply) APPLY=1; shift ;;
    --target-context) TARGET_CONTEXT="${2:-}"; shift 2 ;;
    --confirm) CONFIRM="${2:-}"; shift 2 ;;
    --in-place|--path-b) restore_die "in-place etcd restore is forbidden; ADR011 Path B remains prose-only" ;;
    -h|--help) usage; exit 0 ;;
    *) restore_die "unknown argument: $1" ;;
  esac
done

restore_require_command docker
restore_require_command jq
for prefix in "${PREFIXES[@]}"; do
  [[ "$prefix" == /registry/*/ ]] || restore_die "--prefix must start /registry/ and end /"
  [[ "$prefix" != *$'\n'* ]] || restore_die "invalid registry prefix"
done
[[ "$IMAGE" =~ ^[A-Za-z0-9./:@_-]+$ ]] || restore_die "invalid etcd image"

if [ "$APPLY" -eq 1 ]; then
  restore_require_command kubectl
  [[ "$TARGET_CONTEXT" == restore-* ]] || \
    restore_die "--target-context must start with restore-"
  [ "$CONFIRM" = "APPLY-$TARGET_CONTEXT" ] || \
    restore_die "refusing apply: rerun with --confirm APPLY-$TARGET_CONTEXT"
fi

restore_load_dotenv "$REPO_ROOT"
bucket="${RESTORE_S3_BUCKET_NAME:-${TF_STATE_BUCKET:-}}"
[ -n "$bucket" ] || restore_die "TF_STATE_BUCKET (or RESTORE_S3_BUCKET_NAME) is required"
restore_resolve_snapshot "$SNAPSHOT" "s3://$bucket/etcd-snapshots/" ".db.gz"

scratch="$(mktemp -d "${TMPDIR:-/tmp}/bex-etcd-restore.XXXXXX")"
container="bex-etcd-restore-$$"
cleanup() {
  docker rm -f "$container" >/dev/null 2>&1 || true
  # Docker can create root-owned restore files on Linux. Remove them through
  # the same image before deleting the local scratch directory.
  docker run --rm -v "$scratch:/work" --entrypoint /bin/sh "$IMAGE" \
    -ec 'rm -rf /work/restored' >/dev/null 2>&1 || true
  rm -rf "$scratch"
}
trap cleanup EXIT

archive="$scratch/snapshot.db.gz"
snapshot_file="$scratch/snapshot.db"
restore_fetch_snapshot "$RESTORE_SNAPSHOT_URI" "$archive"
restore_gunzip_checked "$archive" "$snapshot_file"
docker run --rm -v "$scratch:/work:ro" --entrypoint /usr/local/bin/etcdutl "$IMAGE" \
  snapshot status /work/snapshot.db -w json >/dev/null || \
  restore_die "etcd snapshot integrity/status check failed"
docker run --rm -v "$scratch:/work" --entrypoint /usr/local/bin/etcdutl "$IMAGE" \
  snapshot restore /work/snapshot.db --data-dir /work/restored >/dev/null
docker run -d --name "$container" -v "$scratch:/work" \
  --entrypoint /usr/local/bin/etcd "$IMAGE" \
  --name default --data-dir /work/restored \
  --listen-client-urls http://0.0.0.0:2379 \
  --advertise-client-urls http://127.0.0.1:2379 \
  --listen-peer-urls http://0.0.0.0:2380 >/dev/null
for _ in $(seq 1 60); do
  if docker exec "$container" /usr/local/bin/etcdctl endpoint health >/dev/null 2>&1; then
    break
  fi
  sleep 1
done
docker exec "$container" /usr/local/bin/etcdctl endpoint health >/dev/null 2>&1 || \
  restore_die "throwaway etcd did not become healthy"

if [ -n "$OUTPUT_DIR" ]; then
  mkdir -p "$OUTPUT_DIR"
  manifest_dir="$(cd "$OUTPUT_DIR" && pwd)"
else
  manifest_dir="$scratch/manifests"
  mkdir -p "$manifest_dir"
fi

count=0
for prefix in "${PREFIXES[@]}"; do
  keys_file="$scratch/keys.$count"
  docker exec "$container" /usr/local/bin/etcdctl \
    get "$prefix" --prefix --keys-only >"$keys_file"
  while IFS= read -r key; do
    [[ "$key" == "$prefix"* ]] || continue
    raw="$scratch/value.raw"
    value="$scratch/value.json"
    docker exec "$container" /usr/local/bin/etcdctl \
      get "$key" --print-value-only >"$raw"
    if jq -e . "$raw" >/dev/null 2>&1; then
      cp "$raw" "$value"
    else
      # Kubernetes prefixes stored JSON with the four bytes "k8s\0".
      tail -c +5 "$raw" >"$value"
      jq -e . "$value" >/dev/null 2>&1 || \
        restore_die "stored value at $key was not Kubernetes JSON"
    fi
    kind="$(jq -r '.kind // empty' "$value")"
    namespace="$(jq -r '.metadata.namespace // "default"' "$value")"
    name="$(jq -r '.metadata.name // empty' "$value")"
    if [ -z "$kind" ] || [ -z "$name" ]; then
      restore_die "stored value at $key lacked kind/name"
    fi
    safe="$(printf '%s-%s-%s' "$kind" "$namespace" "$name" | tr -c 'A-Za-z0-9._-' '-')"
    jq '
      del(.metadata.uid, .metadata.resourceVersion, .metadata.creationTimestamp,
          .metadata.generation, .metadata.managedFields, .metadata.finalizers,
          .metadata.ownerReferences, .metadata.deletionTimestamp, .status)
    ' "$value" >"$manifest_dir/$safe.json"
    echo "extractable: $kind $namespace/$name"
    count=$((count + 1))
  done <"$keys_file"
done
[ "$count" -gt 0 ] || restore_die "snapshot contained no resources under the selected prefixes"

echo "etcd snapshot status passed; extracted $count sanitized manifest(s)"
if [ -n "$OUTPUT_DIR" ]; then
  echo "manifests written to $manifest_dir"
else
  echo "DRY_RUN preview used temporary manifests; pass --output-dir to retain them"
fi
restore_print_dry_run

if [ "$APPLY" -eq 1 ]; then
  [ "${DRY_RUN:-0}" != "1" ] || restore_die "--apply is unavailable in DRY_RUN mode"
  kubectl --context "$TARGET_CONTEXT" apply -f "$manifest_dir"
  echo "applied sanitized manifests only to context $TARGET_CONTEXT"
fi
