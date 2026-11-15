#!/usr/bin/env bash
# Provision ADR112 object storage on the platform's existing Wasabi account:
# one shared bucket per region, TWO bucket-scoped IAM users (read-write for
# bex-api, read-only for objectgw), and the out-of-band Secrets that arm them.
#
# Usage:
#   scripts/object-storage-s3-credentials.sh provision
#   scripts/object-storage-s3-credentials.sh verify
#   BEX_KUBE_CONTEXT=hetzner-prod scripts/object-storage-s3-credentials.sh install
#
# Modeled on scripts/agent-attachments-s3-credentials.sh and
# scripts/disk-snapshot-secret.sh. What differs:
#
#   1. TWO identities. bex-api reads, writes and deletes tenant objects; objectgw
#      serves public buckets to the internet and must never be able to write or
#      delete anything, so it holds a read-only credential of its own
#      (ADR029's split).
#
#   2. A SHARED account. Tenant objects sit beside bex-tfstate and the platform's
#      backups, so every check below proves the two identities are denied there
#      (ADR112 D1, D9). The root credential is the one the other Wasabi scripts
#      already use (TF_STATE_ACCESS_KEY and TF_STATE_SECRET_KEY, or AWS_ACCESS_KEY_ID
#      and AWS_SECRET_ACCESS_KEY when set, which is also how a later separate
#      account would be reached). There is no new root variable.
#
# The bucket names, regions and endpoints are constants in
# lego/backend/internal/objectstorage (regions.go) and in the committed policies
# under infra/wasabi; a Go test fails if the three drift. Environment variables
# carry the four Wasabi credentials, and the one bearer objectgw presents to
# bex-api's internal listener (BEX_OBJECT_STORAGE_GATEWAY_TOKEN, ADR112 D4, D9),
# and nothing else. The bearer is not a Wasabi key: provision mints it locally
# (64 hex characters, once, never rotated by a re-run) and install puts the same
# value in both Secrets, so bex-api can verify what objectgw sends.
#
# Wasabi shows a secret key once, so each key is written to .env the moment it
# is minted: an orphaned pair cannot be recovered (w2/m86).
#
# Secret material stays out of argv, stdout, Git, and generated manifests.
set -euo pipefail
cd "$(dirname "$0")/.."

COMMAND="${1:-}"
case "$COMMAND" in
  provision|verify|install) ;;
  *) echo "usage: $0 {provision|verify|install}" >&2; exit 2 ;;
esac

# BEX_ENV_FILE lets the offline guard tests (object-storage-s3-credentials.test.sh)
# point this at /dev/null: after a real provision .env carries every value those
# tests assert is absent. Operators never set it.
ENV_FILE="${BEX_ENV_FILE:-.env}"
command -v jq >/dev/null || { echo "error: jq is required" >&2; exit 1; }
if [ -f "$ENV_FILE" ]; then
  # shellcheck source=scripts/lib/env-file.sh
  . scripts/lib/env-file.sh && bex_load_env "$ENV_FILE" || exit 1
fi

KUBE=(kubectl)
if [ -n "${BEX_KUBE_CONTEXT:-}" ]; then
  KUBE=(kubectl --context "$BEX_KUBE_CONTEXT")
fi
namespace="${BEX_SYSTEM_NAMESPACE:-bex-system}"
rw_secret_name="${BEX_OBJECT_STORAGE_SECRET_NAME:-bex-object-storage}"
ro_secret_name="${BEX_OBJECT_STORAGE_READ_SECRET_NAME:-bex-object-storage-read}"

# Constants: keep in step with objectstorage/regions.go and infra/wasabi/*.json.
REGIONS=(oregon:us-west-1 frankfurt:eu-central-2)
RW_USER="bex-object-storage"
RO_USER="bex-object-storage-read"
RW_POLICY="BexObjectStorage"
RO_POLICY="BexObjectStorageRead"
TFSTATE_BUCKET="${TF_STATE_BUCKET:-bex-tfstate}"
# Other platform buckets a probe also tries. A bucket that does not exist on this
# account proves nothing, so those are reported and skipped, never failed.
OTHER_BUCKETS="${BEX_OBJECT_STORAGE_VERIFY_OTHER_BUCKETS:-bex-agent-attachments bex-disk-snapshots bex-static}"
# The platform's own buckets (bex-tfstate and the others) live where the other
# Wasabi scripts put them. A HEAD sent to another region's endpoint answers 400,
# not 403, so a denial can only be proven against the bucket's own endpoint.
PLATFORM_ENDPOINT="${TF_STATE_ENDPOINT:-https://s3.eu-central-2.wasabisys.com}"
PLATFORM_REGION="${TF_STATE_REGION:-eu-central-2}"
IAM_ENDPOINT="${WASABI_IAM_ENDPOINT:-https://iam.wasabisys.com}"
AWS_CLI_IMAGE="${AWS_CLI_IMAGE:-amazon/aws-cli:2.22.35@sha256:6977c83ae3dc99f28fcf8276b9ea5eec33833cd5be40574b34112e98113ec7a2}"

if [ -n "${AWS_ACCESS_KEY_ID:-}" ] || [ -n "${AWS_SECRET_ACCESS_KEY:-}" ]; then
  ROOT_ACCESS="${AWS_ACCESS_KEY_ID:-}"
  ROOT_SECRET="${AWS_SECRET_ACCESS_KEY:-}"
else
  ROOT_ACCESS="${TF_STATE_ACCESS_KEY:-}"
  ROOT_SECRET="${TF_STATE_SECRET_KEY:-}"
fi

require_nonempty() {
  local name="$1" value="$2"
  [ -n "$value" ] || { echo "error: $name is missing or empty" >&2; exit 1; }
}

[[ "$AWS_CLI_IMAGE" =~ ^[^[:space:]@]+@sha256:[0-9a-f]{64}$ ]] || {
  echo "error: AWS_CLI_IMAGE must be pinned by sha256 digest" >&2
  exit 1
}
# Tenant objects never share a bucket with the credentials that rebuild the
# platform; the committed policies name the two buckets literally.
case "$TFSTATE_BUCKET" in
  bex-objects-*) echo "error: the object storage buckets must not be $TFSTATE_BUCKET" >&2; exit 1 ;;
esac

region_of() { printf '%s' "${1#*:}"; }
name_of() { printf '%s' "${1%%:*}"; }
bucket_of() { printf 'bex-objects-%s' "$(name_of "$1")"; }
endpoint_of() { printf 'https://s3.%s.wasabisys.com' "$(region_of "$1")"; }

# aws_as <access> <secret> <region> <endpoint> <aws args...>
aws_as() {
  local access="$1" secret="$2" region="$3" endpoint="$4"
  shift 4
  (
    export AWS_ACCESS_KEY_ID="$access"
    export AWS_SECRET_ACCESS_KEY="$secret"
    export AWS_DEFAULT_REGION="$region"
    docker run --rm -i \
      -e AWS_ACCESS_KEY_ID -e AWS_SECRET_ACCESS_KEY -e AWS_DEFAULT_REGION \
      -v "$PWD/infra/wasabi:/policies:ro" \
      "$AWS_CLI_IMAGE" --endpoint-url "$endpoint" "$@"
  )
}
iam() { aws_as "$ROOT_ACCESS" "$ROOT_SECRET" us-east-1 "$IAM_ENDPOINT" iam "$@"; }
root_s3() { local r="$1"; shift; aws_as "$ROOT_ACCESS" "$ROOT_SECRET" "$(region_of "$r")" "$(endpoint_of "$r")" "$@"; }
rw_s3() { local r="$1"; shift; aws_as "${BEX_OBJECT_STORAGE_ACCESS_KEY:-}" "${BEX_OBJECT_STORAGE_SECRET_KEY:-}" "$(region_of "$r")" "$(endpoint_of "$r")" "$@"; }
ro_s3() { local r="$1"; shift; aws_as "${BEX_OBJECT_STORAGE_READ_ACCESS_KEY:-}" "${BEX_OBJECT_STORAGE_READ_SECRET_KEY:-}" "$(region_of "$r")" "$(endpoint_of "$r")" "$@"; }
# The same identities, pointed at the platform's own endpoint.
rw_platform_s3() { aws_as "${BEX_OBJECT_STORAGE_ACCESS_KEY:-}" "${BEX_OBJECT_STORAGE_SECRET_KEY:-}" "$PLATFORM_REGION" "$PLATFORM_ENDPOINT" "$@"; }
ro_platform_s3() { aws_as "${BEX_OBJECT_STORAGE_READ_ACCESS_KEY:-}" "${BEX_OBJECT_STORAGE_READ_SECRET_KEY:-}" "$PLATFORM_REGION" "$PLATFORM_ENDPOINT" "$@"; }

upsert_env() {
  local key="$1" value="$2"
  [ -f "$ENV_FILE" ] || touch "$ENV_FILE"
  chmod 600 "$ENV_FILE"
  ENV_PATH="$ENV_FILE" KEY="$key" VALUE="$value" python3 - <<'PY'
import os
path, key, value = os.environ["ENV_PATH"], os.environ["KEY"], os.environ["VALUE"]
lines = open(path).read().splitlines(True)
found = False
out = []
for line in lines:
    if line.startswith(key + "="):
        out.append(f"{key}={value}\n")
        found = True
    else:
        out.append(line)
if not found:
    out.append(f"{key}={value}\n")
open(path, "w").write("".join(out))
PY
}

ensure_bucket() {
  local region_spec="$1" bucket
  bucket="$(bucket_of "$region_spec")"
  if root_s3 "$region_spec" s3api head-bucket --bucket "$bucket" >/dev/null 2>&1; then
    echo "kept bucket: $bucket"
  else
    root_s3 "$region_spec" s3api create-bucket --bucket "$bucket" \
      --create-bucket-configuration LocationConstraint="$(region_of "$region_spec")" >/dev/null
    echo "created bucket: $bucket ($(region_of "$region_spec"))"
  fi
}

# ensure_user_and_keys <user> <policy-name> <policy-file> <access-var> <secret-var>
#
# Idempotent on the user and the policy. A key pair is minted only when the user
# has none, and it is written to .env at once because Wasabi shows the secret
# exactly once.
ensure_user_and_keys() {
  local user="$1" policy_name="$2" policy_file="$3" access_var="$4" secret_var="$5"
  local extra attached groups keys key_json
  if ! iam get-user --user-name "$user" >/dev/null 2>&1; then
    iam create-user --user-name "$user" >/dev/null
    echo "created IAM user: $user"
  fi
  iam put-user-policy --user-name "$user" --policy-name "$policy_name" \
    --policy-document "file:///policies/$policy_file" >/dev/null
  echo "applied least-privilege policy: $user/$policy_name"
  extra="$(iam list-user-policies --user-name "$user" | jq -r --arg p "$policy_name" '[.PolicyNames[] | select(. != $p)] | length')"
  attached="$(iam list-attached-user-policies --user-name "$user" | jq '.AttachedPolicies | length')"
  groups="$(iam list-groups-for-user --user-name "$user" | jq '.Groups | length')"
  if [ "$extra" != "0" ] || [ "$attached" != "0" ] || [ "$groups" != "0" ]; then
    echo "error: $user carries extra policies or group membership; refusing to widen its authority" >&2
    exit 1
  fi
  keys="$(iam list-access-keys --user-name "$user" | jq '.AccessKeyMetadata | length')"
  if [ "$keys" = "0" ]; then
    key_json="$(iam create-access-key --user-name "$user")"
    upsert_env "$access_var" "$(jq -r '.AccessKey.AccessKeyId' <<<"$key_json")"
    upsert_env "$secret_var" "$(jq -r '.AccessKey.SecretAccessKey' <<<"$key_json")"
    echo "minted access key for $user (stored in $ENV_FILE)"
  else
    [ -n "${!access_var:-}" ] && [ -n "${!secret_var:-}" ] || {
      echo "error: $user already has an access key but no secret is available locally." >&2
      echo "       Delete the key in Wasabi IAM and re-run provision, or set the pair in .env." >&2
      exit 1
    }
    echo "kept existing access key for $user"
  fi
}

provision() {
  require_nonempty "root Wasabi access key (TF_STATE_ACCESS_KEY)" "$ROOT_ACCESS"
  require_nonempty "root Wasabi secret key (TF_STATE_SECRET_KEY)" "$ROOT_SECRET"
  local r
  for r in "${REGIONS[@]}"; do ensure_bucket "$r"; done
  ensure_user_and_keys "$RW_USER" "$RW_POLICY" object-storage-s3-policy.json \
    BEX_OBJECT_STORAGE_ACCESS_KEY BEX_OBJECT_STORAGE_SECRET_KEY
  ensure_user_and_keys "$RO_USER" "$RO_POLICY" object-storage-read-s3-policy.json \
    BEX_OBJECT_STORAGE_READ_ACCESS_KEY BEX_OBJECT_STORAGE_READ_SECRET_KEY
  ensure_gateway_token
}

# ensure_gateway_token mints the bearer objectgw speaks to bex-api with, once. A
# value already in the environment is kept, so a re-run never invalidates the
# pair of Secrets an earlier install wrote.
ensure_gateway_token() {
  if [ -n "${BEX_OBJECT_STORAGE_GATEWAY_TOKEN:-}" ]; then
    echo "kept gateway token (BEX_OBJECT_STORAGE_GATEWAY_TOKEN)"
    return
  fi
  upsert_env BEX_OBJECT_STORAGE_GATEWAY_TOKEN "$(python3 -c 'import secrets; print(secrets.token_hex(32))')"
  echo "minted gateway token (stored in $ENV_FILE)"
}

# denied_check <absent-ok 0|1> <label> <command...>: an explicit AccessDenied
# proves a boundary; any other failure (network, endpoint) proves nothing, so
# it fails the check. With absent-ok, a bucket that does not exist on this
# account is skipped, not failed: it proves nothing either way.
denied_check() {
  local absent_ok="$1" label="$2" out
  shift 2
  if out="$("$@" 2>&1)"; then
    echo "error: $label was allowed" >&2
    exit 1
  fi
  if grep -Eq 'AccessDenied|Forbidden|\(403\)' <<<"$out"; then
    echo "verified denied: $label"
  elif [ "$absent_ok" = 1 ] && grep -Eq 'NoSuchBucket|Not Found|\(404\)' <<<"$out"; then
    echo "skipped (bucket absent): $label"
  else
    printf '%s\n' "$out" >&2
    echo "error: could not prove $label is denied" >&2
    exit 1
  fi
}
denied() { denied_check 0 "$@"; }
denied_or_absent() { denied_check 1 "$@"; }

ro_put_probe() { printf 'nope' | ro_s3 "$1" s3 cp - "s3://$2/verify/denied"; }

# require_credentials fails unless all four Wasabi credential variables are set.
require_credentials() {
  local v
  for v in BEX_OBJECT_STORAGE_ACCESS_KEY BEX_OBJECT_STORAGE_SECRET_KEY BEX_OBJECT_STORAGE_READ_ACCESS_KEY BEX_OBJECT_STORAGE_READ_SECRET_KEY; do
    require_nonempty "$v" "${!v:-}"
  done
}

verify() {
  require_credentials
  local r bucket key other
  key="verify/probe-$(date +%s)"
  for r in "${REGIONS[@]}"; do
    bucket="$(bucket_of "$r")"
    # read-write: put, get, list and delete one probe object
    printf 'bex object storage probe\n' | rw_s3 "$r" s3 cp - "s3://$bucket/$key" >/dev/null
    rw_s3 "$r" s3 cp "s3://$bucket/$key" - >/dev/null
    rw_s3 "$r" s3 ls "s3://$bucket/verify/" >/dev/null
    echo "verified read-write put/get/list on $bucket"
    # read-only: get and list work, put and delete do not
    ro_s3 "$r" s3 cp "s3://$bucket/$key" - >/dev/null
    ro_s3 "$r" s3 ls "s3://$bucket/verify/" >/dev/null
    echo "verified read-only get/list on $bucket"
    denied "the read-only user writing to $bucket" ro_put_probe "$r" "$bucket"
    denied "the read-only user deleting from $bucket" ro_s3 "$r" s3 rm "s3://$bucket/$key"
    rw_s3 "$r" s3 rm "s3://$bucket/$key" >/dev/null
  done
  # Neither identity may reach the platform's own buckets on this shared
  # account, whichever region the object buckets are in.
  denied "the read-write user reaching $TFSTATE_BUCKET" rw_platform_s3 s3api head-bucket --bucket "$TFSTATE_BUCKET"
  denied "the read-only user reaching $TFSTATE_BUCKET" ro_platform_s3 s3api head-bucket --bucket "$TFSTATE_BUCKET"
  for other in $OTHER_BUCKETS; do
    denied_or_absent "the read-write user reaching $other" rw_platform_s3 s3api head-bucket --bucket "$other"
    denied_or_absent "the read-only user reaching $other" ro_platform_s3 s3api head-bucket --bucket "$other"
  done
}

install_secret() {
  local secret_name="$1" label="$2"
  shift 2
  local env_file pair
  env_file="$(mktemp)"
  chmod 600 "$env_file"
  # shellcheck disable=SC2064
  trap "rm -f '$env_file'" EXIT
  for pair in "$@"; do
    require_nonempty "$pair" "${!pair:-}"
    printf '%s=%s\n' "$pair" "${!pair}" >>"$env_file"
  done
  "${KUBE[@]}" -n "$namespace" create secret generic "$secret_name" \
    --from-env-file="$env_file" --dry-run=client -o yaml | "${KUBE[@]}" apply -f - >/dev/null
  rm -f "$env_file"
  trap - EXIT
  echo "installed $namespace/$secret_name ($label)"
}

install() {
  # Check all four before touching the cluster, so a missing read-only pair
  # cannot leave only the read-write Secret installed.
  require_credentials
  require_nonempty "BEX_OBJECT_STORAGE_GATEWAY_TOKEN (run provision to mint it)" "${BEX_OBJECT_STORAGE_GATEWAY_TOKEN:-}"
  # The same bearer goes to both sides: bex-api verifies what objectgw sends.
  install_secret "$rw_secret_name" "bex-api: read-write, and the gateway bearer" \
    BEX_OBJECT_STORAGE_ACCESS_KEY BEX_OBJECT_STORAGE_SECRET_KEY BEX_OBJECT_STORAGE_GATEWAY_TOKEN
  install_secret "$ro_secret_name" "objectgw: read-only, and the gateway bearer" \
    BEX_OBJECT_STORAGE_READ_ACCESS_KEY BEX_OBJECT_STORAGE_READ_SECRET_KEY BEX_OBJECT_STORAGE_GATEWAY_TOKEN
}

case "$COMMAND" in
  provision) provision ;;
  verify) verify ;;
  install) install ;;
esac
