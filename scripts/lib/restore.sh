#!/usr/bin/env bash
# Shared, side-effect-conscious helpers for scripts/restore-*.sh.
#
# This file is sourced. Callers keep store-specific recovery logic in their own
# script, but use the same validation, snapshot selection, mutation gate, and
# throwaway namespace lifecycle.

restore_die() {
  echo "error: $*" >&2
  exit 1
}

restore_require_command() {
  command -v "$1" >/dev/null 2>&1 || restore_die "required command not found: $1"
}

# Recovery helpers pass production object-store or decryption credentials into
# fallback containers. A mutable tag is therefore never an acceptable identity
# for one of those credential receivers.
restore_require_digest_image() {
  local image="$1" label="$2"
  [[ "$image" =~ ^[^[:space:]@]+@sha256:[0-9a-f]{64}$ ]] || \
    restore_die "$label must be pinned by an @sha256 digest"
}

restore_load_dotenv() {
  local repo_root="$1"
  if [ "${RESTORE_SKIP_DOTENV:-0}" != "1" ] && [ -f "$repo_root/.env" ]; then
    set -a
    # Repository-local, gitignored operator config.
    # shellcheck disable=SC1091
    source "$repo_root/.env"
    set +a
  fi
  # Repository config names the shared backup-plane identity TF_STATE_*;
  # AWS CLI consumes the conventional names. Export aliases without printing.
  if [ -z "${AWS_ACCESS_KEY_ID:-}" ] && [ -n "${TF_STATE_ACCESS_KEY:-}" ]; then
    export AWS_ACCESS_KEY_ID="$TF_STATE_ACCESS_KEY"
  fi
  if [ -z "${AWS_SECRET_ACCESS_KEY:-}" ] && [ -n "${TF_STATE_SECRET_KEY:-}" ]; then
    export AWS_SECRET_ACCESS_KEY="$TF_STATE_SECRET_KEY"
  fi
  if [ -z "${AWS_DEFAULT_REGION:-}" ] && [ -n "${TF_STATE_REGION:-}" ]; then
    export AWS_DEFAULT_REGION="$TF_STATE_REGION"
  fi
}

restore_validate_dns_label() {
  local value="$1" label="$2"
  [[ "$value" =~ ^[a-z0-9]([-a-z0-9]*[a-z0-9])?$ ]] || \
    restore_die "$label must be a lowercase DNS label"
  [ "${#value}" -le 63 ] || restore_die "$label must be at most 63 characters"
}

restore_require_throwaway_namespace() {
  local namespace="$1"
  restore_validate_dns_label "$namespace" "target namespace"
  [[ "$namespace" == restore-* ]] || \
    restore_die "target namespace must start with restore- (live namespaces are forbidden)"
}

restore_validate_rfc3339() {
  local value="$1"
  [[ "$value" =~ ^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}(\.[0-9]+)?(Z|[+-][0-9]{2}:[0-9]{2})$ ]] || \
    restore_die "target time must be RFC3339"
}

restore_require_confirmation() {
  local target="$1" confirmation="$2"
  [ "$confirmation" = "$target" ] || \
    restore_die "refusing mutation: rerun with --confirm $target after reviewing DRY_RUN=1 output"
}

restore_s3_args() {
  RESTORE_S3_ARGS=()
  if [ -n "${RESTORE_S3_ENDPOINT:-${TF_STATE_ENDPOINT:-}}" ]; then
    RESTORE_S3_ARGS=(--endpoint-url "${RESTORE_S3_ENDPOINT:-${TF_STATE_ENDPOINT}}")
  fi
}

restore_aws() {
  local image
  if command -v aws >/dev/null 2>&1; then
    command aws "$@"
    return
  fi
  restore_require_command docker
  image="${RESTORE_AWS_IMAGE:-amazon/aws-cli:2.22.35@sha256:6977c83ae3dc99f28fcf8276b9ea5eec33833cd5be40574b34112e98113ec7a2}"
  restore_require_digest_image "$image" "RESTORE_AWS_IMAGE"
  # -e NAME forwards an already-exported value without placing the credential
  # itself in argv. The digest-pinned CLI matches the production backup Jobs.
  docker run --rm \
    -e AWS_ACCESS_KEY_ID -e AWS_SECRET_ACCESS_KEY -e AWS_SESSION_TOKEN \
    -e AWS_DEFAULT_REGION -e AWS_REGION -e AWS_EC2_METADATA_DISABLED=true \
    "$image" "$@"
}

restore_aws_download() {
  local uri="$1" destination="$2" parent name image
  if command -v aws >/dev/null 2>&1; then
    command aws "${RESTORE_S3_ARGS[@]}" s3 cp --only-show-errors "$uri" "$destination"
    return
  fi
  restore_require_command docker
  image="${RESTORE_AWS_IMAGE:-amazon/aws-cli:2.22.35@sha256:6977c83ae3dc99f28fcf8276b9ea5eec33833cd5be40574b34112e98113ec7a2}"
  restore_require_digest_image "$image" "RESTORE_AWS_IMAGE"
  parent="$(cd "$(dirname "$destination")" && pwd)"
  name="$(basename "$destination")"
  docker run --rm \
    -e AWS_ACCESS_KEY_ID -e AWS_SECRET_ACCESS_KEY -e AWS_SESSION_TOKEN \
    -e AWS_DEFAULT_REGION -e AWS_REGION -e AWS_EC2_METADATA_DISABLED=true \
    -v "$parent:/restore" \
    "$image" \
    "${RESTORE_S3_ARGS[@]}" s3 cp --only-show-errors "$uri" "/restore/$name"
}

restore_parse_s3_uri() {
  local uri="$1" remainder
  [[ "$uri" == s3://*/* ]] || restore_die "invalid S3 URI: $uri"
  remainder="${uri#s3://}"
  RESTORE_S3_BUCKET="${remainder%%/*}"
  RESTORE_S3_KEY="${remainder#*/}"
  if [ -z "$RESTORE_S3_BUCKET" ] || [ -z "$RESTORE_S3_KEY" ]; then
    restore_die "invalid S3 URI: $uri"
  fi
}

restore_latest_s3_uri() {
  local prefix_uri="$1" suffix="$2" listing latest
  restore_parse_s3_uri "$prefix_uri"
  restore_s3_args
  listing="$(restore_aws "${RESTORE_S3_ARGS[@]}" s3api list-objects-v2 \
    --bucket "$RESTORE_S3_BUCKET" --prefix "$RESTORE_S3_KEY" \
    --query 'Contents[].Key' --output text)"
  # Accept both the plain gz object and its ADR050 age-encrypted variant so a
  # single restore path spans the pre/post-encryption transition.
  latest="$(printf '%s\n' "$listing" | tr '\t' '\n' | \
    awk -v prefix="$RESTORE_S3_KEY" -v suffix="$suffix" -v age="$suffix.age" '
      NF && $0 != "None" && index($0, prefix) == 1 &&
      (substr($0, length($0) - length(suffix) + 1) == suffix ||
       substr($0, length($0) - length(age) + 1) == age)' | \
    LC_ALL=C sort | tail -1 || true)"
  [ -n "$latest" ] || restore_die "no $suffix snapshots found below $prefix_uri"
  printf 's3://%s/%s\n' "$RESTORE_S3_BUCKET" "$latest"
}

restore_resolve_snapshot() {
  local requested="$1" prefix_uri="$2" suffix="$3" resolved
  if [ -z "$requested" ] || [ "$requested" = "latest" ]; then
    resolved="$(restore_latest_s3_uri "$prefix_uri" "$suffix")"
  else
    restore_parse_s3_uri "$requested"
    [[ "$RESTORE_S3_KEY" == *"$suffix" || "$RESTORE_S3_KEY" == *"$suffix.age" ]] || \
      restore_die "snapshot must end in $suffix (optionally .age)"
    resolved="$requested"
  fi
  export RESTORE_SNAPSHOT_URI="$resolved"
}

restore_fetch_snapshot() {
  local uri="$1" destination="$2"
  restore_s3_args
  echo "fetching selected snapshot (credential values are never printed)"
  restore_aws_download "$uri" "$destination"
}

restore_gunzip_checked() {
  local archive="$1" destination="$2"
  gzip -t "$archive" || restore_die "snapshot gzip integrity check failed"
  gzip -dc "$archive" >"$destination"
  [ -s "$destination" ] || restore_die "snapshot decompressed to an empty file"
}

# restore_prefer_reader_credential <store> — when a per-store reader credential
# is present (BEX_BACKUP_READER_<STORE>_ACCESS_KEY_ID/_SECRET_ACCESS_KEY, recorded
# in .env by scripts/backup-s3-credentials.sh, ADR050 §3), use it for object-store
# reads instead of the shared TF_STATE_* root key. Unset ⇒ no-op (TF_STATE
# fallback, byte-identical to pre-ADR050). The writer credential is never used
# for restore.
restore_prefer_reader_credential() {
  local store="$1" up akv skv ak sk
  up="$(printf '%s' "$store" | tr 'a-z-' 'A-Z_')"
  akv="BEX_BACKUP_READER_${up}_ACCESS_KEY_ID"
  skv="BEX_BACKUP_READER_${up}_SECRET_ACCESS_KEY"
  ak="${!akv:-}"
  sk="${!skv:-}"
  if [ -n "$ak" ] && [ -n "$sk" ]; then
    export AWS_ACCESS_KEY_ID="$ak"
    export AWS_SECRET_ACCESS_KEY="$sk"
  fi
}

# restore_decrypt_if_age <resolved-uri> <fetched-file> <gz-destination> — reverse
# the ADR050 Tier A client-side age layer. When the resolved object ends in .age,
# decrypt with AGE_BACKUP_PRIVATE_KEY (env/.env) into the gz destination the
# gunzip step expects; otherwise the fetched bytes ARE the gz and are moved into
# place unchanged (byte-identical to pre-ADR050). A local `age` binary is used
# when present; otherwise RESTORE_AGE_IMAGE must name a pinned image whose
# entrypoint is age. The private key never enters argv (a mode-0600 keyfile).
restore_decrypt_if_age() {
  local uri="$1" fetched="$2" destination="$3" dir keyfile
  if [[ "$uri" != *.age ]]; then
    [ "$fetched" = "$destination" ] || mv -f "$fetched" "$destination"
    return
  fi
  [ -n "${AGE_BACKUP_PRIVATE_KEY:-}" ] || \
    restore_die "AGE_BACKUP_PRIVATE_KEY is required to decrypt an .age snapshot (set it in .env)"
  dir="$(cd "$(dirname "$destination")" && pwd)"
  keyfile="$dir/.age-restore-key"
  ( umask 077; printf '%s\n' "$AGE_BACKUP_PRIVATE_KEY" >"$keyfile" )
  if command -v age >/dev/null 2>&1; then
    command age -d -i "$keyfile" -o "$destination" "$fetched" \
      || { rm -f "$keyfile"; restore_die "age decryption failed"; }
  elif [ -n "${RESTORE_AGE_IMAGE:-}" ]; then
    restore_require_command docker
    restore_require_digest_image "$RESTORE_AGE_IMAGE" "RESTORE_AGE_IMAGE"
    docker run --rm -v "$dir:/work" "$RESTORE_AGE_IMAGE" \
      -d -i "/work/$(basename "$keyfile")" \
      -o "/work/$(basename "$destination")" "/work/$(basename "$fetched")" \
      || { rm -f "$keyfile"; restore_die "age decryption failed"; }
  else
    rm -f "$keyfile"
    restore_die "no local 'age' binary and RESTORE_AGE_IMAGE unset; install age or set RESTORE_AGE_IMAGE to decrypt .age snapshots"
  fi
  rm -f "$keyfile"
  [ -s "$destination" ] || restore_die "age decryption produced an empty file"
}

restore_create_namespace() {
  local namespace="$1" store="$2"
  cat <<EOF | kubectl apply -f - >/dev/null
apiVersion: v1
kind: Namespace
metadata:
  name: $namespace
  labels:
    bex.co/restore-target: "true"
    bex.co/restore-store: "$store"
EOF
}

restore_assert_owned_namespace() {
  local namespace="$1" label
  restore_require_throwaway_namespace "$namespace"
  label="$(kubectl get namespace "$namespace" -o jsonpath='{.metadata.labels.bex\.co/restore-target}' 2>/dev/null || true)"
  [ "$label" = "true" ] || \
    restore_die "namespace $namespace is not labeled bex.co/restore-target=true; refusing teardown"
}

restore_delete_namespace() {
  local namespace="$1"
  restore_assert_owned_namespace "$namespace"
  kubectl delete namespace "$namespace" --wait=false >/dev/null
  if ! kubectl wait --for=delete "namespace/$namespace" --timeout="${RESTORE_TEARDOWN_TIMEOUT:-10m}" >/dev/null; then
    restore_die "namespace $namespace did not finish deleting"
  fi
  echo "teardown verified: namespace $namespace and all namespaced restore resources are absent"
}

restore_print_dry_run() {
  if [ "${DRY_RUN:-0}" = "1" ]; then
    echo "DRY_RUN=1: no Kubernetes mutation will be attempted"
  fi
  return 0
}
