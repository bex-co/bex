#!/usr/bin/env bash
# Provision, install, verify, and retire the static-site S3 credentials (w7/m54).
#
# Two Wasabi IAM users replace the legacy Terraform-state root credential:
#   bex-static-reader     -> bex-system/bex-static-read-s3
#   bex-static-publisher  -> bex-build/bex-static-publish-s3
#
# Secret values remain out of argv, stdout, Git, and generated manifests. The
# AWS CLI runs in a pinned container because Wasabi IAM is AWS-compatible and a
# local aws binary is not otherwise required.
#
# Usage:
#   scripts/static-s3-credentials.sh provision
#   scripts/static-s3-credentials.sh verify
#   scripts/static-s3-credentials.sh revoke-legacy
#
# `provision` is idempotent: it creates missing users/keys, replaces the inline
# policies from infra/wasabi, and applies the two unused-by-legacy Secrets.
# `verify` proves the positive and negative access matrix with a random probe.
# `revoke-legacy` refuses to proceed until verification passes and no live or
# declarative workload references the old `static-s3` Secret.
set -euo pipefail
cd "$(dirname "$0")/.."

COMMAND="${1:-}"
case "$COMMAND" in
  provision|verify|revoke-legacy) ;;
  *) echo "usage: $0 {provision|verify|revoke-legacy}" >&2; exit 2 ;;
esac

if [ -f .env ]; then
  set -a
  # shellcheck disable=SC1091
  source ./.env
  set +a
fi

command -v docker >/dev/null || { echo "error: docker not found" >&2; exit 1; }
command -v jq >/dev/null || { echo "error: jq not found" >&2; exit 1; }
command -v kubectl >/dev/null || { echo "error: kubectl not found" >&2; exit 1; }

AWS_CLI_IMAGE="${AWS_CLI_IMAGE:-amazon/aws-cli:2.22.35}"
IAM_ENDPOINT="${WASABI_IAM_ENDPOINT:-https://iam.wasabisys.com}"
S3_ENDPOINT="${TF_STATE_ENDPOINT:-${BEX_STATIC_S3_ENDPOINT:-}}"
S3_REGION="${TF_STATE_REGION:-${BEX_STATIC_S3_REGION:-}}"
STATIC_BUCKET="${BEX_STATIC_S3_BUCKET:-bex-static}"
TFSTATE_BUCKET="${TF_STATE_BUCKET:-bex-tfstate}"
READ_USER="${BEX_STATIC_READ_IAM_USER:-bex-static-reader}"
PUBLISH_USER="${BEX_STATIC_PUBLISH_IAM_USER:-bex-static-publisher}"
READ_NAMESPACE="${BEX_STATIC_READ_NAMESPACE:-bex-system}"
PUBLISH_NAMESPACE="${BEX_BUILD_NAMESPACE:-bex-build}"
READ_SECRET="${BEX_STATIC_READ_S3_SECRET:-bex-static-read-s3}"
PUBLISH_SECRET="${BEX_STATIC_PUBLISH_S3_SECRET:-bex-static-publish-s3}"
LEGACY_SECRET="${BEX_STATIC_LEGACY_S3_SECRET:-static-s3}"

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

require_nonempty S3_ENDPOINT "$S3_ENDPOINT"
require_nonempty S3_REGION "$S3_REGION"
[ "$STATIC_BUCKET" = "bex-static" ] || {
  echo "error: committed IAM policies are pinned to bex-static; refusing bucket $STATIC_BUCKET" >&2
  exit 1
}
[ "$TFSTATE_BUCKET" != "$STATIC_BUCKET" ] || {
  echo "error: static and Terraform-state buckets must be different" >&2
  exit 1
}

root_aws() {
  (
    export AWS_ACCESS_KEY_ID="$ROOT_ACCESS"
    export AWS_SECRET_ACCESS_KEY="$ROOT_SECRET"
    export AWS_DEFAULT_REGION=us-east-1
    docker run --rm -i \
      -e AWS_ACCESS_KEY_ID -e AWS_SECRET_ACCESS_KEY -e AWS_DEFAULT_REGION \
      -v "$PWD/infra/wasabi:/policies:ro" \
      "$AWS_CLI_IMAGE" "$@"
  )
}

iam() {
  root_aws --endpoint-url "$IAM_ENDPOINT" iam "$@"
}

root_s3() {
  (
    export AWS_ACCESS_KEY_ID="$ROOT_ACCESS"
    export AWS_SECRET_ACCESS_KEY="$ROOT_SECRET"
    export AWS_DEFAULT_REGION="$S3_REGION"
    docker run --rm -i \
      -e AWS_ACCESS_KEY_ID -e AWS_SECRET_ACCESS_KEY -e AWS_DEFAULT_REGION \
      "$AWS_CLI_IMAGE" --endpoint-url "$S3_ENDPOINT" "$@"
  )
}

credential_s3() {
  local access="$1" secret="$2"
  shift 2
  credential_s3_at "$access" "$secret" "$S3_ENDPOINT" "$S3_REGION" "$@"
}

credential_s3_at() {
  local access="$1" secret="$2" endpoint="$3" region="$4"
  shift 4
  (
    export AWS_ACCESS_KEY_ID="$access"
    export AWS_SECRET_ACCESS_KEY="$secret"
    export AWS_DEFAULT_REGION="$region"
    docker run --rm -i \
      -e AWS_ACCESS_KEY_ID -e AWS_SECRET_ACCESS_KEY -e AWS_DEFAULT_REGION \
      "$AWS_CLI_IMAGE" --endpoint-url "$endpoint" "$@"
  )
}

declare -a CLEANUP_PROBE_URIS=()
CLEANUP_PROBE_ACCESS=""
CLEANUP_PROBE_SECRET=""
cleanup_probe() {
  local uri
  for uri in "${CLEANUP_PROBE_URIS[@]}"; do
    credential_s3 "$CLEANUP_PROBE_ACCESS" "$CLEANUP_PROBE_SECRET" \
      s3 rm "$uri" >/dev/null 2>&1 || true
  done
}

apply_secret() {
  local namespace="$1" name="$2" access="$3" secret="$4"
  jq -cn \
    --arg namespace "$namespace" --arg name "$name" \
    --arg access "$access" --arg secret "$secret" --arg region "$S3_REGION" \
    '{apiVersion:"v1", kind:"Secret", type:"Opaque",
      metadata:{namespace:$namespace,name:$name,
        labels:{"app.kubernetes.io/managed-by":"static-s3-credentials"}},
      stringData:{AWS_ACCESS_KEY_ID:$access,AWS_SECRET_ACCESS_KEY:$secret,AWS_DEFAULT_REGION:$region}}' \
    | kubectl apply -f - >/dev/null
}

secret_access_id() {
  local namespace="$1" name="$2"
  kubectl -n "$namespace" get secret "$name" -o json 2>/dev/null \
    | jq -r '.data.AWS_ACCESS_KEY_ID // "" | @base64d'
}

ensure_user_and_secret() {
  local user="$1" policy_name="$2" policy_file="$3" namespace="$4" secret_name="$5"
  local access_id key_count key_json secret_key extra_inline attached groups

  if ! iam get-user --user-name "$user" >/dev/null 2>&1; then
    iam create-user --user-name "$user" >/dev/null
    echo "created IAM user: $user"
  fi
  iam put-user-policy --user-name "$user" --policy-name "$policy_name" \
    --policy-document "file:///policies/$policy_file" >/dev/null
  echo "applied least-privilege policy: $user/$policy_name"

  extra_inline="$(iam list-user-policies --user-name "$user" \
    | jq -r --arg expected "$policy_name" '[.PolicyNames[] | select(. != $expected)] | length')"
  attached="$(iam list-attached-user-policies --user-name "$user" | jq '.AttachedPolicies | length')"
  groups="$(iam list-groups-for-user --user-name "$user" | jq '.Groups | length')"
  [ "$extra_inline" -eq 0 ] && [ "$attached" -eq 0 ] && [ "$groups" -eq 0 ] || {
    echo "error: $user has an unexpected inline, attached, or group policy; refusing ambiguous authority" >&2
    exit 1
  }

  access_id="$(secret_access_id "$namespace" "$secret_name" || true)"
  if [ -n "$access_id" ] && iam list-access-keys --user-name "$user" \
      | jq -e --arg id "$access_id" '.AccessKeyMetadata[] | select(.AccessKeyId == $id and .Status == "Active")' \
      >/dev/null; then
    echo "kept active credential Secret: $namespace/$secret_name"
    return
  fi

  key_count="$(iam list-access-keys --user-name "$user" | jq '.AccessKeyMetadata | length')"
  [ "$key_count" -lt 2 ] || {
    echo "error: $user already has two access keys and $namespace/$secret_name does not contain an active one" >&2
    exit 1
  }

  key_json="$(iam create-access-key --user-name "$user")"
  access_id="$(jq -r '.AccessKey.AccessKeyId' <<<"$key_json")"
  secret_key="$(jq -r '.AccessKey.SecretAccessKey' <<<"$key_json")"
  [ -n "$access_id" ] && [ -n "$secret_key" ] || {
    echo "error: Wasabi returned an incomplete access key for $user" >&2
    exit 1
  }
  if ! apply_secret "$namespace" "$secret_name" "$access_id" "$secret_key"; then
    iam delete-access-key --user-name "$user" --access-key-id "$access_id" >/dev/null || true
    echo "error: failed to install $namespace/$secret_name; deleted the unused provider key" >&2
    exit 1
  fi
  unset key_json secret_key access_id
  echo "created and installed credential: $namespace/$secret_name ($user)"
}

load_secret() {
  local namespace="$1" name="$2" json
  json="$(kubectl -n "$namespace" get secret "$name" -o json)"
  CRED_ACCESS="$(jq -r '.data.AWS_ACCESS_KEY_ID // "" | @base64d' <<<"$json")"
  CRED_SECRET="$(jq -r '.data.AWS_SECRET_ACCESS_KEY // "" | @base64d' <<<"$json")"
  CRED_REGION="$(jq -r '.data.AWS_DEFAULT_REGION // "" | @base64d' <<<"$json")"
  [ -n "$CRED_ACCESS" ] && [ -n "$CRED_SECRET" ] || {
    echo "error: $namespace/$name lacks the required AWS credential keys" >&2
    exit 1
  }
  [ -z "$CRED_REGION" ] || [ "$CRED_REGION" = "$S3_REGION" ] || {
    echo "error: $namespace/$name has a region different from configured S3_REGION" >&2
    exit 1
  }
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
  # Wasabi may conceal an existing but unauthorized bucket as NoSuchBucket.
  # The control-plane root listing above proves the selected bucket exists, so
  # this is a non-disclosing deny. Generic transport/config failures do not pass.
  elif grep -Eqi 'AccessDenied|Forbidden|AllAccessDisabled|NoSuchBucket|status code:[[:space:]]*403' <<<"$output"; then
    printf 'PASS  deny   %s\n' "$label"
  else
    printf 'FAIL  deny   %s (request failed without an access-denied response)\n' "$label" >&2
    return 1
  fi
}

verify_matrix() {
  local read_access read_secret publish_access publish_secret unrelated unrelated_region unrelated_endpoint probe_uri
  load_secret "$READ_NAMESPACE" "$READ_SECRET"
  read_access="$CRED_ACCESS"
  read_secret="$CRED_SECRET"
  load_secret "$PUBLISH_NAMESPACE" "$PUBLISH_SECRET"
  publish_access="$CRED_ACCESS"
  publish_secret="$CRED_SECRET"
  unset CRED_ACCESS CRED_SECRET CRED_REGION

  unrelated="$(root_s3 s3api list-buckets \
    | jq -r --arg static "$STATIC_BUCKET" --arg state "$TFSTATE_BUCKET" \
      '[.Buckets[].Name | select(. != $static and . != $state)] | first // empty')"
  [ -n "$unrelated" ] || {
    echo "error: no unrelated bucket is available for the isolation proof" >&2
    exit 1
  }
  unrelated_region="$(root_aws --endpoint-url https://s3.wasabisys.com s3api get-bucket-location \
    --bucket "$unrelated" | jq -r '.LocationConstraint // "us-east-1"')"
  [ -n "$unrelated_region" ] || {
    echo "error: could not discover the unrelated bucket region" >&2
    exit 1
  }
  if [ "$unrelated_region" = "us-east-1" ]; then
    unrelated_endpoint="https://s3.wasabisys.com"
  else
    unrelated_endpoint="https://s3.$unrelated_region.wasabisys.com"
  fi

  probe_uri="s3://$STATIC_BUCKET/.bex-security-probe/$(date -u +%Y%m%dT%H%M%SZ)-$$"
  # The publisher cleans both paths if any assertion aborts. This also removes
  # the reader-write object if that negative probe ever succeeds unexpectedly.
  CLEANUP_PROBE_URIS=("$probe_uri" "${probe_uri}-reader")
  CLEANUP_PROBE_ACCESS="$publish_access"
  CLEANUP_PROBE_SECRET="$publish_secret"
  trap cleanup_probe EXIT

  printf 'static S3 credential matrix (%s)\n' "$STATIC_BUCKET"
  printf 'w7/m54 probe\n' | expect_allowed "publisher put static object" \
    credential_s3 "$publish_access" "$publish_secret" s3 cp - "$probe_uri"
  expect_allowed "publisher read static object" \
    credential_s3 "$publish_access" "$publish_secret" s3 cp "$probe_uri" -
  expect_allowed "reader list static bucket" \
    credential_s3 "$read_access" "$read_secret" s3api list-objects-v2 --bucket "$STATIC_BUCKET" --max-keys 1
  expect_allowed "reader read static object" \
    credential_s3 "$read_access" "$read_secret" s3 cp "$probe_uri" -
  printf 'denied\n' | expect_denied "reader write static object" \
    credential_s3 "$read_access" "$read_secret" s3 cp - "${probe_uri}-reader"
  expect_denied "reader delete static object" \
    credential_s3 "$read_access" "$read_secret" s3 rm "$probe_uri"
  expect_denied "reader list tfstate bucket" \
    credential_s3 "$read_access" "$read_secret" s3api list-objects-v2 --bucket "$TFSTATE_BUCKET" --max-keys 1
  expect_denied "publisher list tfstate bucket" \
    credential_s3 "$publish_access" "$publish_secret" s3api list-objects-v2 --bucket "$TFSTATE_BUCKET" --max-keys 1
  expect_denied "reader list account buckets" \
    credential_s3 "$read_access" "$read_secret" s3api list-buckets
  expect_denied "publisher list account buckets" \
    credential_s3 "$publish_access" "$publish_secret" s3api list-buckets
  expect_denied "reader list unrelated bucket" \
    credential_s3_at "$read_access" "$read_secret" "$unrelated_endpoint" "$unrelated_region" \
      s3api list-objects-v2 --bucket "$unrelated" --max-keys 1
  expect_denied "publisher list unrelated bucket" \
    credential_s3_at "$publish_access" "$publish_secret" "$unrelated_endpoint" "$unrelated_region" \
      s3api list-objects-v2 --bucket "$unrelated" --max-keys 1
  expect_allowed "publisher delete static object" \
    credential_s3 "$publish_access" "$publish_secret" s3 rm "$probe_uri"

  CLEANUP_PROBE_URIS=()
  CLEANUP_PROBE_ACCESS=""
  CLEANUP_PROBE_SECRET=""
  trap - EXIT
  unset read_access read_secret publish_access publish_secret
}

legacy_references() {
  kubectl get pods,deployments,statefulsets,daemonsets,jobs,cronjobs -A -o json \
    | jq -r --arg secret "$LEGACY_SECRET" '
      .items[]
      | select((.spec.template.spec // .spec.jobTemplate.spec.template.spec
          // (if .kind == "Pod" then .spec else {} end)) as $pod
          | (($pod.volumes // []) | any(.secret.secretName == $secret))
            or (($pod.imagePullSecrets // []) | any(.name == $secret))
            or (($pod.containers // []) + ($pod.initContainers // [])
                | any(((.envFrom // []) | any(.secretRef.name == $secret))
                    or ((.env // []) | any(.valueFrom.secretKeyRef.name == $secret)))))
      | [.kind, .metadata.namespace, .metadata.name] | @tsv'
}

case "$COMMAND" in
  provision)
    require_nonempty TF_STATE_ACCESS_KEY/AWS_ACCESS_KEY_ID "$ROOT_ACCESS"
    require_nonempty TF_STATE_SECRET_KEY/AWS_SECRET_ACCESS_KEY "$ROOT_SECRET"
    ensure_user_and_secret "$READ_USER" BexStaticRead static-s3-read-policy.json "$READ_NAMESPACE" "$READ_SECRET"
    ensure_user_and_secret "$PUBLISH_USER" BexStaticPublish static-s3-publish-policy.json "$PUBLISH_NAMESPACE" "$PUBLISH_SECRET"
    ;;
  verify)
    require_nonempty TF_STATE_ACCESS_KEY/AWS_ACCESS_KEY_ID "$ROOT_ACCESS"
    require_nonempty TF_STATE_SECRET_KEY/AWS_SECRET_ACCESS_KEY "$ROOT_SECRET"
    verify_matrix
    ;;
  revoke-legacy)
    require_nonempty TF_STATE_ACCESS_KEY/AWS_ACCESS_KEY_ID "$ROOT_ACCESS"
    require_nonempty TF_STATE_SECRET_KEY/AWS_SECRET_ACCESS_KEY "$ROOT_SECRET"
    verify_matrix
    refs="$(legacy_references)"
    if [ -n "$refs" ]; then
      echo "error: refusing to delete $LEGACY_SECRET; workload references remain:" >&2
      printf '%s\n' "$refs" >&2
      exit 1
    fi
    legacy_namespaces="$(kubectl get secrets -A --field-selector "metadata.name=$LEGACY_SECRET" -o json \
      | jq -r '.items[].metadata.namespace' | sort -u)"
    while IFS= read -r namespace; do
      [ -n "$namespace" ] || continue
      kubectl -n "$namespace" delete secret "$LEGACY_SECRET" --ignore-not-found >/dev/null
      echo "removed legacy Secret: $namespace/$LEGACY_SECRET"
    done <<<"$legacy_namespaces"
    if kubectl get secrets -A --field-selector "metadata.name=$LEGACY_SECRET" -o json \
        | jq -e '.items | length > 0' >/dev/null; then
      echo "error: a legacy $LEGACY_SECRET Secret remains after revocation" >&2
      exit 1
    fi
    echo "legacy static-plane access is revoked; Terraform-state root credentials remain in out-of-band custody"
    ;;
esac
