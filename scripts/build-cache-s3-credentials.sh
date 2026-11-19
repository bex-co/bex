#!/usr/bin/env bash
# Provision and verify the BEX_BUILD_CACHE=s3 layer-cache store (w9/m492,
# docs/ADR060 D3 S3 amendment, docs/ADR019).
#
#   bex-build-cache bucket, 30-day expiry  (infra/wasabi/build-cache-lifecycle.json)
#   bex-build-cache IAM user               (infra/wasabi/build-cache-s3-policy.json)
#   -> bex-build/bex-build-cache-s3 Secret (read by build Jobs' cache phases only)
#
# Usage (KUBECONFIG selects the cluster the Secret is installed into):
#   scripts/build-cache-s3-credentials.sh provision
#   scripts/build-cache-s3-credentials.sh verify
#
# `provision` is idempotent: it creates the bucket and IAM user when missing,
# replaces the lifecycle rule and the inline policy from infra/wasabi, and
# installs the Secret only when it does not already hold an active key.
# `verify` proves the credential reads, writes, lists and deletes inside the
# cache bucket and is refused everywhere else on the account, and that the
# expiry rule is in force.
#
# The BuildKit container (tenant code) never sees this Secret: only the
# cache-restore / cache-purge / cache-save phases of a build Job and the App
# finalizer's cache-purge Job read it. Secret values stay out of argv, stdout,
# Git and generated manifests.
set -euo pipefail
cd "$(dirname "$0")/.."

COMMAND="${1:-}"
case "$COMMAND" in
  provision|verify) ;;
  *) echo "usage: $0 {provision|verify}" >&2; exit 2 ;;
esac

ENV_FILE="${BEX_ENV_FILE:-.env}"
if [ -f "$ENV_FILE" ]; then
  # shellcheck source=scripts/lib/env-file.sh
  . scripts/lib/env-file.sh && bex_load_env "$ENV_FILE" || exit 1
fi

AWS_CLI_IMAGE="${AWS_CLI_IMAGE:-amazon/aws-cli:2.22.35@sha256:6977c83ae3dc99f28fcf8276b9ea5eec33833cd5be40574b34112e98113ec7a2}"
[[ "$AWS_CLI_IMAGE" =~ ^[^[:space:]@]+@sha256:[0-9a-f]{64}$ ]] || {
  echo "error: AWS_CLI_IMAGE must be pinned by sha256 digest" >&2
  exit 1
}
IAM_ENDPOINT="${WASABI_IAM_ENDPOINT:-https://iam.wasabisys.com}"
S3_ENDPOINT="${BEX_BUILD_CACHE_S3_ENDPOINT:-${TF_STATE_ENDPOINT:-}}"
S3_REGION="${BEX_BUILD_CACHE_S3_REGION:-${TF_STATE_REGION:-}}"
BUCKET="${BEX_BUILD_CACHE_S3_BUCKET:-bex-build-cache}"
TFSTATE_BUCKET="${TF_STATE_BUCKET:-bex-tfstate}"
STATIC_BUCKET="${BEX_STATIC_S3_BUCKET:-bex-static}"
IAM_USER="${BEX_BUILD_CACHE_IAM_USER:-bex-build-cache}"
POLICY_NAME="BexBuildCache"
NAMESPACE="${BEX_BUILD_NAMESPACE:-bex-build}"
SECRET_NAME="${BEX_BUILD_CACHE_S3_SECRET:-bex-build-cache-s3}"
EXPIRY_DAYS=30

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

# The committed policy and lifecycle name this bucket literally, and a cache
# of tenant build layers never shares a bucket with platform state or the
# public static-site origin.
[ "$BUCKET" = "bex-build-cache" ] || {
  echo "error: the committed IAM policy is pinned to bex-build-cache; refusing bucket $BUCKET" >&2
  exit 1
}
for other in "$TFSTATE_BUCKET" "$STATIC_BUCKET"; do
  [ "$BUCKET" != "$other" ] || { echo "error: the build cache bucket must not be $other" >&2; exit 1; }
done
[ "$(jq -r '.Rules[0].Expiration.Days' infra/wasabi/build-cache-lifecycle.json)" = "$EXPIRY_DAYS" ] || {
  echo "error: infra/wasabi/build-cache-lifecycle.json no longer expires objects after $EXPIRY_DAYS days" >&2
  exit 1
}
require_nonempty "root Wasabi access key (TF_STATE_ACCESS_KEY)" "$ROOT_ACCESS"
require_nonempty "root Wasabi secret key (TF_STATE_SECRET_KEY)" "$ROOT_SECRET"
require_nonempty "S3 endpoint (BEX_BUILD_CACHE_S3_ENDPOINT or TF_STATE_ENDPOINT)" "$S3_ENDPOINT"
require_nonempty "S3 region (BEX_BUILD_CACHE_S3_REGION or TF_STATE_REGION)" "$S3_REGION"
for tool in docker jq kubectl; do
  command -v "$tool" >/dev/null || { echo "error: $tool is required" >&2; exit 1; }
done

with_credential() {
  local access="$1" secret="$2" region="$3"
  shift 3
  (
    export AWS_ACCESS_KEY_ID="$access"
    export AWS_SECRET_ACCESS_KEY="$secret"
    export AWS_DEFAULT_REGION="$region"
    docker run --rm -i \
      -e AWS_ACCESS_KEY_ID -e AWS_SECRET_ACCESS_KEY -e AWS_DEFAULT_REGION \
      -v "$PWD/infra/wasabi:/policies:ro" \
      "$AWS_CLI_IMAGE" "$@"
  )
}
iam() { with_credential "$ROOT_ACCESS" "$ROOT_SECRET" us-east-1 --endpoint-url "$IAM_ENDPOINT" iam "$@"; }
root_s3() { with_credential "$ROOT_ACCESS" "$ROOT_SECRET" "$S3_REGION" --endpoint-url "$S3_ENDPOINT" "$@"; }
cache_s3() { with_credential "$CACHE_ACCESS" "$CACHE_SECRET" "$S3_REGION" --endpoint-url "$S3_ENDPOINT" "$@"; }
cache_s3_at() {
  local endpoint="$1" region="$2"
  shift 2
  with_credential "$CACHE_ACCESS" "$CACHE_SECRET" "$region" --endpoint-url "$endpoint" "$@"
}

apply_secret() {
  local access="$1" secret="$2"
  # app.bex.co/protected-from-tenant-mount matches the operator's label-based
  # rejection (LabelProtectedFromTenantMount in lego/types), so a tenant App
  # that names this Secret in a mount field is refused.
  jq -cn \
    --arg namespace "$NAMESPACE" --arg name "$SECRET_NAME" \
    --arg access "$access" --arg secret "$secret" --arg region "$S3_REGION" \
    '{apiVersion:"v1", kind:"Secret", type:"Opaque",
      metadata:{namespace:$namespace,name:$name,
        labels:{"app.kubernetes.io/managed-by":"build-cache-s3-credentials",
          "app.bex.co/protected-from-tenant-mount":"true"}},
      stringData:{AWS_ACCESS_KEY_ID:$access,AWS_SECRET_ACCESS_KEY:$secret,AWS_DEFAULT_REGION:$region}}' \
    | kubectl apply -f - >/dev/null
}

secret_access_id() {
  kubectl -n "$NAMESPACE" get secret "$SECRET_NAME" -o json 2>/dev/null \
    | jq -r '.data.AWS_ACCESS_KEY_ID // "" | @base64d'
}

provision() {
  if root_s3 s3api head-bucket --bucket "$BUCKET" >/dev/null 2>&1; then
    echo "kept build cache bucket: $BUCKET"
  else
    root_s3 s3api create-bucket --bucket "$BUCKET" \
      --create-bucket-configuration LocationConstraint="$S3_REGION" >/dev/null
    echo "created build cache bucket: $BUCKET"
  fi
  root_s3 s3api put-bucket-lifecycle-configuration --bucket "$BUCKET" \
    --lifecycle-configuration "file:///policies/build-cache-lifecycle.json" >/dev/null
  echo "applied ${EXPIRY_DAYS}-day expiry: $BUCKET"

  if ! iam get-user --user-name "$IAM_USER" >/dev/null 2>&1; then
    iam create-user --user-name "$IAM_USER" >/dev/null
    echo "created IAM user: $IAM_USER"
  fi
  iam put-user-policy --user-name "$IAM_USER" --policy-name "$POLICY_NAME" \
    --policy-document "file:///policies/build-cache-s3-policy.json" >/dev/null
  echo "applied least-privilege policy: $IAM_USER/$POLICY_NAME"
  local extra attached groups
  extra="$(iam list-user-policies --user-name "$IAM_USER" | jq -r --arg p "$POLICY_NAME" '[.PolicyNames[] | select(. != $p)] | length')"
  attached="$(iam list-attached-user-policies --user-name "$IAM_USER" | jq '.AttachedPolicies | length')"
  groups="$(iam list-groups-for-user --user-name "$IAM_USER" | jq '.Groups | length')"
  if [ "$extra" != "0" ] || [ "$attached" != "0" ] || [ "$groups" != "0" ]; then
    echo "error: $IAM_USER carries extra policies or group membership; refusing to widen its authority" >&2
    exit 1
  fi

  local access_id key_count key_json secret_key
  access_id="$(secret_access_id || true)"
  if [ -n "$access_id" ] && iam list-access-keys --user-name "$IAM_USER" \
      | jq -e --arg id "$access_id" '.AccessKeyMetadata[] | select(.AccessKeyId == $id and .Status == "Active")' \
      >/dev/null; then
    echo "kept active credential Secret: $NAMESPACE/$SECRET_NAME"
    return
  fi
  key_count="$(iam list-access-keys --user-name "$IAM_USER" | jq '.AccessKeyMetadata | length')"
  [ "$key_count" -lt 2 ] || {
    echo "error: $IAM_USER already has two access keys and $NAMESPACE/$SECRET_NAME holds neither" >&2
    exit 1
  }
  key_json="$(iam create-access-key --user-name "$IAM_USER")"
  access_id="$(jq -r '.AccessKey.AccessKeyId' <<<"$key_json")"
  secret_key="$(jq -r '.AccessKey.SecretAccessKey' <<<"$key_json")"
  [ -n "$access_id" ] && [ -n "$secret_key" ] || {
    echo "error: Wasabi returned an incomplete access key for $IAM_USER" >&2
    exit 1
  }
  if ! apply_secret "$access_id" "$secret_key"; then
    iam delete-access-key --user-name "$IAM_USER" --access-key-id "$access_id" >/dev/null || true
    echo "error: failed to install $NAMESPACE/$SECRET_NAME; deleted the unused provider key" >&2
    exit 1
  fi
  unset key_json secret_key access_id
  echo "created and installed credential: $NAMESPACE/$SECRET_NAME ($IAM_USER)"
}

expect_allowed() {
  local label="$1"
  shift
  if "$@" >/dev/null 2>&1; then
    printf 'PASS  allow  %s\n' "$label"
  else
    printf 'FAIL  allow  %s\n' "$label" >&2
    return 1
  fi
}

# Only an explicit denial proves the boundary; a transport or configuration
# failure proves nothing, so it fails the check. Wasabi may conceal a bucket
# the caller cannot see as NoSuchBucket, which is a non-disclosing deny.
expect_denied() {
  local label="$1" output status
  shift
  set +e
  output="$("$@" 2>&1)"
  status=$?
  set -e
  if [ "$status" -eq 0 ]; then
    printf 'FAIL  deny   %s (request unexpectedly succeeded)\n' "$label" >&2
    return 1
  elif grep -Eqi 'AccessDenied|Forbidden|AllAccessDisabled|NoSuchBucket|status code:[[:space:]]*403' <<<"$output"; then
    printf 'PASS  deny   %s\n' "$label"
  else
    printf 'FAIL  deny   %s (request failed without an access-denied response)\n' "$label" >&2
    return 1
  fi
}

PROBE_URI=""
cleanup_probe() {
  [ -z "$PROBE_URI" ] || cache_s3 s3 rm "$PROBE_URI" --recursive >/dev/null 2>&1 || true
}

verify() {
  local json unrelated unrelated_region unrelated_endpoint expiry
  json="$(kubectl -n "$NAMESPACE" get secret "$SECRET_NAME" -o json)"
  CACHE_ACCESS="$(jq -r '.data.AWS_ACCESS_KEY_ID // "" | @base64d' <<<"$json")"
  CACHE_SECRET="$(jq -r '.data.AWS_SECRET_ACCESS_KEY // "" | @base64d' <<<"$json")"
  unset json
  [ -n "$CACHE_ACCESS" ] && [ -n "$CACHE_SECRET" ] || {
    echo "error: $NAMESPACE/$SECRET_NAME lacks the AWS credential keys" >&2
    exit 1
  }

  expiry="$(root_s3 s3api get-bucket-lifecycle-configuration --bucket "$BUCKET" \
    | jq -r '[.Rules[] | select(.Status == "Enabled" and (.Filter.Prefix // .Prefix // "") == "") | .Expiration.Days] | min // empty')"
  if [ "$expiry" = "$EXPIRY_DAYS" ]; then
    printf 'PASS  expiry %s-day rule on %s\n' "$EXPIRY_DAYS" "$BUCKET"
  else
    printf 'FAIL  expiry %s carries no %s-day rule (got %s)\n' "$BUCKET" "$EXPIRY_DAYS" "${expiry:-none}" >&2
    exit 1
  fi

  unrelated="$(root_s3 s3api list-buckets \
    | jq -r --arg cache "$BUCKET" --arg state "$TFSTATE_BUCKET" --arg static "$STATIC_BUCKET" \
      '[.Buckets[].Name | select(. != $cache and . != $state and . != $static)] | first // empty')"
  require_nonempty "an unrelated bucket for the isolation proof" "$unrelated"
  unrelated_region="$(with_credential "$ROOT_ACCESS" "$ROOT_SECRET" us-east-1 --endpoint-url https://s3.wasabisys.com \
    s3api get-bucket-location --bucket "$unrelated" | jq -r '.LocationConstraint // "us-east-1"')"
  if [ "$unrelated_region" = "us-east-1" ]; then
    unrelated_endpoint="https://s3.wasabisys.com"
  else
    unrelated_endpoint="https://s3.$unrelated_region.wasabisys.com"
  fi

  PROBE_URI="s3://$BUCKET/.bex-security-probe/$(date -u +%Y%m%dT%H%M%SZ)-$$/"
  trap cleanup_probe EXIT
  printf 'build cache S3 credential matrix (%s)\n' "$BUCKET"
  printf 'w9/m492 probe\n' | expect_allowed "cache put object" cache_s3 s3 cp - "${PROBE_URI}blobs/sha256/probe"
  expect_allowed "cache read object" cache_s3 s3 cp "${PROBE_URI}blobs/sha256/probe" -
  expect_allowed "cache list prefix" cache_s3 s3 ls "$PROBE_URI" --recursive
  expect_allowed "cache delete prefix" cache_s3 s3 rm "$PROBE_URI" --recursive
  expect_denied "cache list account buckets" cache_s3 s3api list-buckets
  expect_denied "cache list tfstate bucket" \
    cache_s3 s3api list-objects-v2 --bucket "$TFSTATE_BUCKET" --max-keys 1
  printf 'denied\n' | expect_denied "cache write tfstate bucket" \
    cache_s3 s3 cp - "s3://$TFSTATE_BUCKET/.bex-build-cache-probe-$$"
  expect_denied "cache list static bucket" \
    cache_s3 s3api list-objects-v2 --bucket "$STATIC_BUCKET" --max-keys 1
  printf 'denied\n' | expect_denied "cache write static bucket" \
    cache_s3 s3 cp - "s3://$STATIC_BUCKET/.bex-build-cache-probe-$$"
  expect_denied "cache list unrelated bucket" \
    cache_s3_at "$unrelated_endpoint" "$unrelated_region" s3api list-objects-v2 --bucket "$unrelated" --max-keys 1
  expect_denied "cache change the expiry rule" \
    cache_s3 s3api delete-bucket-lifecycle --bucket "$BUCKET"
  PROBE_URI=""
  trap - EXIT
  unset CACHE_ACCESS CACHE_SECRET
}

case "$COMMAND" in
  provision) provision ;;
  verify) verify ;;
esac
