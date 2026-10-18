#!/usr/bin/env bash
# Temporary verification of the EXISTING pre-cutover backup. Never writes production.
set -euo pipefail
umask 077
repo_root="$(cd "$(dirname "$0")/../.." && pwd)"
source "$repo_root/scripts/lib/restore.sh"
restore_load_dotenv "$repo_root"
restore_prefer_reader_credential openbao
export AWS_MAX_ATTEMPTS=2
: "${M114_SNAPSHOT_URI:?exact pre-cutover snapshot URI required}"
: "${M114_REPORT:?unique sanitized report destination required}"
: "${RUNNER_TEMP:?runner scratch required}"
: "${M114_CONTAINER:?unique local container name required}"
[[ "$M114_CONTAINER" =~ ^bex-m114-proof-[0-9]+-[0-9]+$ ]] || exit 1
scratch="$(mktemp -d "$RUNNER_TEMP/m114-preservation-${GITHUB_RUN_ID}-${GITHUB_RUN_ATTEMPT}.XXXXXX")"
audit="$repo_root/scripts/audits/m114-preservation-audit.py"
exec 3>&1
cleanup() {
  original_status=$?
  trap - EXIT TERM INT
  set +e
  # Python's TERM handler first revokes its live token and removes Docker.
  # This second bounded removal also covers a failed/interrupted Python start.
  python3 "$audit" --cleanup-container "$M114_CONTAINER"
  docker_status=$?
  rm -rf -- "$scratch"
  plaintext_removed=false
  [ -e "$scratch" ] || plaintext_removed=true
  [ -s "$M114_REPORT" ] || printf '%s\n' '{"metadataEqual":false,"valuesEqual":false,"completed":false}' >"$M114_REPORT"
  python3 "$audit" --finalize-report "$original_status" "$plaintext_removed"
  final_status=$?
  cat "$M114_REPORT" >&3
  if [ "$original_status" -ne 0 ] || [ "$docker_status" -ne 0 ] || [ "$final_status" -ne 0 ]; then
    exit 1
  fi
  exit 0
}
trap cleanup EXIT
trap 'exit 143' TERM
trap 'exit 130' INT
# All tool diagnostics stay private and are deleted. The report has only
# equality/cleanup booleans, group IDs/owners and counts; never names or digests.
exec >"$scratch/private.log" 2>&1
printf '%s\n' '{"metadataEqual":false,"valuesEqual":false,"completed":false}' >"$M114_REPORT"
for variable in BAO_UNSEAL_KEY_1 BAO_UNSEAL_KEY_2 BAO_UNSEAL_KEY_3 BAO_ROOT_TOKEN AGE_BACKUP_PRIVATE_KEY; do
  [ -n "${!variable:-}" ] || exit 1
done
bucket="${RESTORE_S3_BUCKET_NAME:-${TF_STATE_BUCKET:-}}"
[ -n "$bucket" ]
restore_resolve_snapshot "$M114_SNAPSHOT_URI" "s3://$bucket/openbao-snapshots/" .snap.gz
# Exact reviewed key, not latest; the ciphertext digest also pins object contents.
python3 "$audit" --validate-uri "$M114_SNAPSHOT_URI" "$bucket"
restore_parse_s3_uri "$M114_SNAPSHOT_URI"
restore_s3_args
restore_aws "${RESTORE_S3_ARGS[@]}" s3api head-object --bucket "$RESTORE_S3_BUCKET" --key "$RESTORE_S3_KEY" >"$scratch/object.json"
# Keep the shared restore credential/decryption helpers, but stream this download:
# the optional AWS Docker fallback must not assume a shared runner filesystem.
restore_aws_download() {
  restore_aws "${RESTORE_S3_ARGS[@]}" s3 cp --only-show-errors "$1" - >"$2"
}
restore_fetch_snapshot "$M114_SNAPSHOT_URI" "$scratch/download.age"
python3 "$audit" --validate-ciphertext "$scratch/download.age"
restore_decrypt_if_age "$M114_SNAPSHOT_URI" "$scratch/download.age" "$scratch/snapshot.gz"
restore_gunzip_checked "$scratch/snapshot.gz" "$scratch/snapshot.snap"
python3 "$audit" "$scratch" "$M114_REPORT"
