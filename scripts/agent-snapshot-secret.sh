#!/usr/bin/env bash
# Provision the ADR059 agent-session snapshot bucket, a bucket-scoped Wasabi
# IAM user, and the out-of-band bex-system/bex-agent-snapshot Secret that arms
# bex-api (w2/m77). Secret material stays out of argv, stdout, Git, and
# generated manifests.
#
# Usage:
#   scripts/agent-snapshot-secret.sh provision
#   scripts/agent-snapshot-secret.sh verify
#   BEX_KUBE_CONTEXT=hetzner-prod scripts/agent-snapshot-secret.sh install
#   DRY_RUN=1 scripts/agent-snapshot-secret.sh install
#
# `provision` is idempotent: it creates the dedicated SSE-enabled bucket
# (NEVER bex-tfstate), applies infra/wasabi/agent-snapshot-s3-policy.json,
# upserts the six BEX_AGENT_SNAPSHOT_S3_* names in .env, and installs the
# Secret. `verify` proves PUT/GET/DELETE under agent-snapshots/ and deny on
# bex-tfstate / other buckets. `install` (re)applies the Secret from .env.
set -euo pipefail
cd "$(dirname "$0")/.."

COMMAND="${1:-}"
case "$COMMAND" in
  provision|verify|install) ;;
  *) echo "usage: $0 {provision|verify|install}" >&2; exit 2 ;;
esac

if [ -f .env ]; then
  set -a
  # shellcheck disable=SC1091
  source ./.env
  set +a
fi

KUBE=(kubectl)
if [ -n "${BEX_KUBE_CONTEXT:-}" ]; then
  KUBE=(kubectl --context "$BEX_KUBE_CONTEXT")
fi
namespace="${BEX_SYSTEM_NAMESPACE:-bex-system}"
secret_name="${BEX_AGENT_SNAPSHOT_SECRET_NAME:-bex-agent-snapshot}"
IAM_USER="${BEX_AGENT_SNAPSHOT_IAM_USER:-bex-agent-snapshot}"
IAM_POLICY_NAME="${BEX_AGENT_SNAPSHOT_IAM_POLICY:-BexAgentSnapshot}"
SNAPSHOT_BUCKET="${BEX_AGENT_SNAPSHOT_S3_BUCKET:-bex-agent-snapshots}"
SNAPSHOT_PREFIX="${BEX_AGENT_SNAPSHOT_S3_PREFIX:-agent-snapshots}"
TFSTATE_BUCKET="${TF_STATE_BUCKET:-bex-tfstate}"
IAM_ENDPOINT="${WASABI_IAM_ENDPOINT:-https://iam.wasabisys.com}"
S3_ENDPOINT="${BEX_AGENT_SNAPSHOT_S3_ENDPOINT:-${TF_STATE_ENDPOINT:-${BEX_STATIC_S3_ENDPOINT:-}}}"
S3_REGION="${BEX_AGENT_SNAPSHOT_S3_REGION:-${TF_STATE_REGION:-${BEX_STATIC_S3_REGION:-}}}"
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

require_env() {
  local name="$1" value="${!1:-}"
  [ -n "$value" ] || { echo "error: $name is missing or empty (.env or environment)" >&2; exit 1; }
}

[[ "$AWS_CLI_IMAGE" =~ ^[^[:space:]@]+@sha256:[0-9a-f]{64}$ ]] || {
  echo "error: AWS_CLI_IMAGE must be pinned by sha256 digest" >&2
  exit 1
}

[ "$SNAPSHOT_BUCKET" != "$TFSTATE_BUCKET" ] || {
  echo "error: snapshot bucket must not be $TFSTATE_BUCKET" >&2
  exit 1
}
[ "$SNAPSHOT_BUCKET" = "bex-agent-snapshots" ] || {
  echo "error: committed IAM policy is pinned to bex-agent-snapshots; refusing bucket $SNAPSHOT_BUCKET" >&2
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
  (
    export AWS_ACCESS_KEY_ID="$access"
    export AWS_SECRET_ACCESS_KEY="$secret"
    export AWS_DEFAULT_REGION="$S3_REGION"
    docker run --rm -i \
      -e AWS_ACCESS_KEY_ID -e AWS_SECRET_ACCESS_KEY -e AWS_DEFAULT_REGION \
      "$AWS_CLI_IMAGE" --endpoint-url "$S3_ENDPOINT" "$@"
  )
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

upsert_env() {
  python3 -c '
import os, pathlib, re, sys
path = pathlib.Path(sys.argv[1])
keys = sys.argv[2:]
text = path.read_text() if path.exists() else ""
lines = text.splitlines(True)
seen = set()
out = []
for line in lines:
    match = re.match(r"^([A-Z0-9_]+)=", line)
    if match and match.group(1) in keys:
        key = match.group(1)
        out.append(f"{key}={os.environ[key]}\n")
        seen.add(key)
        continue
    out.append(line)
if out and not str(out[-1]).endswith("\n"):
    out.append("\n")
for key in keys:
    if key not in seen:
        out.append(f"{key}={os.environ[key]}\n")
path.write_text("".join(out))
' .env \
    BEX_AGENT_SNAPSHOT_S3_ENDPOINT \
    BEX_AGENT_SNAPSHOT_S3_BUCKET \
    BEX_AGENT_SNAPSHOT_S3_REGION \
    BEX_AGENT_SNAPSHOT_S3_PREFIX \
    BEX_AGENT_SNAPSHOT_S3_ACCESS_KEY \
    BEX_AGENT_SNAPSHOT_S3_SECRET_KEY
  chmod 600 .env 2>/dev/null || true
}

install_secret() {
  require_env BEX_AGENT_SNAPSHOT_S3_ENDPOINT
  require_env BEX_AGENT_SNAPSHOT_S3_BUCKET
  require_env BEX_AGENT_SNAPSHOT_S3_ACCESS_KEY
  require_env BEX_AGENT_SNAPSHOT_S3_SECRET_KEY
  [ "${BEX_AGENT_SNAPSHOT_S3_BUCKET}" != "$TFSTATE_BUCKET" ] || {
    echo "error: refusing to arm hibernation against $TFSTATE_BUCKET" >&2
    exit 1
  }
  BEX_AGENT_SNAPSHOT_S3_REGION="${BEX_AGENT_SNAPSHOT_S3_REGION:-$S3_REGION}"
  BEX_AGENT_SNAPSHOT_S3_PREFIX="${BEX_AGENT_SNAPSHOT_S3_PREFIX:-$SNAPSHOT_PREFIX}"

  if [ "${DRY_RUN:-0}" = "1" ]; then
    echo "would apply Secret $namespace/$secret_name (keys: BEX_AGENT_SNAPSHOT_S3_ENDPOINT BEX_AGENT_SNAPSHOT_S3_BUCKET BEX_AGENT_SNAPSHOT_S3_REGION BEX_AGENT_SNAPSHOT_S3_PREFIX BEX_AGENT_SNAPSHOT_S3_ACCESS_KEY BEX_AGENT_SNAPSHOT_S3_SECRET_KEY)"
    echo "bucket=$BEX_AGENT_SNAPSHOT_S3_BUCKET endpoint=$BEX_AGENT_SNAPSHOT_S3_ENDPOINT region=$BEX_AGENT_SNAPSHOT_S3_REGION prefix=$BEX_AGENT_SNAPSHOT_S3_PREFIX"
    return
  fi

  command -v kubectl >/dev/null || { echo "error: kubectl is required" >&2; exit 1; }
  umask 077
  secret_dir="$(mktemp -d)"
  secret_env="$secret_dir/snapshot.env"
  cleanup() {
    rm -f "$secret_env"
    rmdir "$secret_dir" 2>/dev/null || true
  }
  trap cleanup EXIT
  {
    printf 'BEX_AGENT_SNAPSHOT_S3_ENDPOINT=%s\n' "$BEX_AGENT_SNAPSHOT_S3_ENDPOINT"
    printf 'BEX_AGENT_SNAPSHOT_S3_BUCKET=%s\n' "$BEX_AGENT_SNAPSHOT_S3_BUCKET"
    printf 'BEX_AGENT_SNAPSHOT_S3_REGION=%s\n' "$BEX_AGENT_SNAPSHOT_S3_REGION"
    printf 'BEX_AGENT_SNAPSHOT_S3_PREFIX=%s\n' "$BEX_AGENT_SNAPSHOT_S3_PREFIX"
    printf 'BEX_AGENT_SNAPSHOT_S3_ACCESS_KEY=%s\n' "$BEX_AGENT_SNAPSHOT_S3_ACCESS_KEY"
    printf 'BEX_AGENT_SNAPSHOT_S3_SECRET_KEY=%s\n' "$BEX_AGENT_SNAPSHOT_S3_SECRET_KEY"
  } >"$secret_env"
  "${KUBE[@]}" get namespace "$namespace" >/dev/null 2>&1 || "${KUBE[@]}" create namespace "$namespace" >/dev/null
  "${KUBE[@]}" -n "$namespace" create secret generic "$secret_name" \
    --from-env-file="$secret_env" \
    --dry-run=client -o yaml | "${KUBE[@]}" apply -f - >/dev/null
  trap - EXIT
  cleanup
  if "${KUBE[@]}" -n "$namespace" get deployment/bex-api >/dev/null 2>&1 \
      && "${KUBE[@]}" -n "$namespace" get deployment/bex-api -o json \
      | jq -e '.spec.template.spec.containers[].env[]? | select(.name == "BEX_AGENT_SNAPSHOT_S3_BUCKET")' >/dev/null; then
    "${KUBE[@]}" -n "$namespace" rollout restart deployment/bex-api >/dev/null
    "${KUBE[@]}" -n "$namespace" rollout status deployment/bex-api --timeout=300s >/dev/null
    echo "installed $namespace/$secret_name for agent-session snapshots; bex-api rollout is ready"
  else
    echo "installed $namespace/$secret_name for agent-session snapshots (bex-api not yet consuming it; next deploy will arm)"
  fi
}

ensure_bucket() {
  if root_s3 s3api head-bucket --bucket "$SNAPSHOT_BUCKET" >/dev/null 2>&1; then
    echo "kept snapshot bucket: $SNAPSHOT_BUCKET"
  else
    root_s3 s3api create-bucket \
      --bucket "$SNAPSHOT_BUCKET" \
      --create-bucket-configuration LocationConstraint="$S3_REGION" >/dev/null
    echo "created snapshot bucket: $SNAPSHOT_BUCKET"
  fi
  # Wasabi encrypts every object at rest with AES-256 and does not implement
  # Put/GetBucketEncryption (docs.wasabi.com operations-on-buckets). Prove
  # the bucket exists; object-level SSE headers are checked in verify when the
  # provider surfaces them.
  if enc_err="$(root_s3 s3api put-bucket-encryption --bucket "$SNAPSHOT_BUCKET" --server-side-encryption-configuration \
      '{"Rules":[{"ApplyServerSideEncryptionByDefault":{"SSEAlgorithm":"AES256"},"BucketKeyEnabled":true}]}' 2>&1)"; then
    enc="$(root_s3 s3api get-bucket-encryption --bucket "$SNAPSHOT_BUCKET")"
    algo="$(jq -r '.ServerSideEncryptionConfiguration.Rules[0].ApplyServerSideEncryptionByDefault.SSEAlgorithm // empty' <<<"$enc")"
    [ "$algo" = "AES256" ] || {
      echo "error: $SNAPSHOT_BUCKET default SSE is '$algo', want AES256" >&2
      exit 1
    }
    echo "confirmed default SSE AES256 on $SNAPSHOT_BUCKET"
  elif grep -Eqi 'MethodNotAllowed|NotImplemented|InvalidRequest' <<<"$enc_err"; then
    echo "confirmed Wasabi automatic AES-256 at rest on $SNAPSHOT_BUCKET (bucket encryption API unsupported)"
  else
    printf '%s\n' "$enc_err" >&2
    echo "error: put-bucket-encryption failed for $SNAPSHOT_BUCKET" >&2
    exit 1
  fi
}

secret_access_id() {
  "${KUBE[@]}" -n "$namespace" get secret "$secret_name" -o json 2>/dev/null \
    | jq -r '.data.BEX_AGENT_SNAPSHOT_S3_ACCESS_KEY // "" | @base64d'
}

ensure_user_and_keys() {
  local access_id key_count key_json extra_inline attached groups

  if ! iam get-user --user-name "$IAM_USER" >/dev/null 2>&1; then
    iam create-user --user-name "$IAM_USER" >/dev/null
    echo "created IAM user: $IAM_USER"
  fi
  iam put-user-policy --user-name "$IAM_USER" --policy-name "$IAM_POLICY_NAME" \
    --policy-document "file:///policies/agent-snapshot-s3-policy.json" >/dev/null
  echo "applied least-privilege policy: $IAM_USER/$IAM_POLICY_NAME"

  extra_inline="$(iam list-user-policies --user-name "$IAM_USER" \
    | jq -r --arg expected "$IAM_POLICY_NAME" '[.PolicyNames[] | select(. != $expected)] | length')"
  attached="$(iam list-attached-user-policies --user-name "$IAM_USER" | jq '.AttachedPolicies | length')"
  groups="$(iam list-groups-for-user --user-name "$IAM_USER" | jq '.Groups | length')"
  [ "$extra_inline" -eq 0 ] && [ "$attached" -eq 0 ] && [ "$groups" -eq 0 ] || {
    echo "error: $IAM_USER has an unexpected inline, attached, or group policy; refusing ambiguous authority" >&2
    exit 1
  }

  if [ -n "${BEX_AGENT_SNAPSHOT_S3_ACCESS_KEY:-}" ] && [ -n "${BEX_AGENT_SNAPSHOT_S3_SECRET_KEY:-}" ] \
      && iam list-access-keys --user-name "$IAM_USER" \
      | jq -e --arg id "$BEX_AGENT_SNAPSHOT_S3_ACCESS_KEY" '.AccessKeyMetadata[] | select(.AccessKeyId == $id and .Status == "Active")' \
      >/dev/null; then
    echo "kept active snapshot credential from .env"
    return
  fi

  access_id="$(secret_access_id || true)"
  if [ -n "$access_id" ] && iam list-access-keys --user-name "$IAM_USER" \
      | jq -e --arg id "$access_id" '.AccessKeyMetadata[] | select(.AccessKeyId == $id and .Status == "Active")' \
      >/dev/null; then
    echo "error: $namespace/$secret_name holds an active key that .env does not; copy it into .env before re-running (this script will not print it)" >&2
    exit 1
  fi

  key_count="$(iam list-access-keys --user-name "$IAM_USER" | jq '.AccessKeyMetadata | length')"
  [ "$key_count" -lt 2 ] || {
    echo "error: $IAM_USER already has two access keys and no matching .env/Secret credential" >&2
    exit 1
  }

  key_json="$(iam create-access-key --user-name "$IAM_USER")"
  BEX_AGENT_SNAPSHOT_S3_ACCESS_KEY="$(jq -r '.AccessKey.AccessKeyId' <<<"$key_json")"
  BEX_AGENT_SNAPSHOT_S3_SECRET_KEY="$(jq -r '.AccessKey.SecretAccessKey' <<<"$key_json")"
  export BEX_AGENT_SNAPSHOT_S3_ACCESS_KEY BEX_AGENT_SNAPSHOT_S3_SECRET_KEY
  unset key_json
  echo "minted snapshot access key for $IAM_USER"
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
  elif grep -Eqi 'AccessDenied|Forbidden|AllAccessDisabled|NoSuchBucket|status code:[[:space:]]*403' <<<"$output"; then
    printf 'PASS  deny   %s\n' "$label"
  else
    printf 'FAIL  deny   %s (request failed without an access-denied response)\n' "$label" >&2
    return 1
  fi
}

verify_matrix() {
  local probe_key probe_uri sse unrelated unrelated_region unrelated_endpoint
  require_env BEX_AGENT_SNAPSHOT_S3_ACCESS_KEY
  require_env BEX_AGENT_SNAPSHOT_S3_SECRET_KEY

  unrelated="$(root_s3 s3api list-buckets \
    | jq -r --arg snap "$SNAPSHOT_BUCKET" --arg state "$TFSTATE_BUCKET" \
      '[.Buckets[].Name | select(. != $snap and . != $state)] | first // empty')"
  [ -n "$unrelated" ] || {
    echo "error: no unrelated bucket is available for the isolation proof" >&2
    exit 1
  }
  unrelated_region="$(root_aws --endpoint-url https://s3.wasabisys.com s3api get-bucket-location \
    --bucket "$unrelated" | jq -r '.LocationConstraint // "us-east-1"')"
  if [ "$unrelated_region" = "us-east-1" ]; then
    unrelated_endpoint="https://s3.wasabisys.com"
  else
    unrelated_endpoint="https://s3.$unrelated_region.wasabisys.com"
  fi

  probe_key="$SNAPSHOT_PREFIX/.bex-security-probe/$(date -u +%Y%m%dT%H%M%SZ)-$$"
  probe_uri="s3://$SNAPSHOT_BUCKET/$probe_key"
  printf 'agent snapshot S3 credential matrix (%s)\n' "$SNAPSHOT_BUCKET"
  printf 'w2/m77 probe\n' | expect_allowed "snapshot put object" \
    credential_s3 "$BEX_AGENT_SNAPSHOT_S3_ACCESS_KEY" "$BEX_AGENT_SNAPSHOT_S3_SECRET_KEY" \
      s3 cp - "$probe_uri"
  sse="$(credential_s3 "$BEX_AGENT_SNAPSHOT_S3_ACCESS_KEY" "$BEX_AGENT_SNAPSHOT_S3_SECRET_KEY" \
    s3api head-object --bucket "$SNAPSHOT_BUCKET" --key "$probe_key" \
    | jq -r '.ServerSideEncryption // empty')"
  if [ "$sse" = "AES256" ]; then
    echo "PASS  sse    probe object AES256"
  elif [ -z "$sse" ]; then
    echo "PASS  sse    probe object AES256 via Wasabi automatic AES-256 at rest (no S3 SSE header)"
  else
    echo "error: probe object SSE is '$sse', want AES256 or Wasabi default (empty header)" >&2
    exit 1
  fi
  expect_allowed "snapshot get object" \
    credential_s3 "$BEX_AGENT_SNAPSHOT_S3_ACCESS_KEY" "$BEX_AGENT_SNAPSHOT_S3_SECRET_KEY" \
      s3 cp "$probe_uri" -
  expect_allowed "snapshot list prefix" \
    credential_s3 "$BEX_AGENT_SNAPSHOT_S3_ACCESS_KEY" "$BEX_AGENT_SNAPSHOT_S3_SECRET_KEY" \
      s3api list-objects-v2 --bucket "$SNAPSHOT_BUCKET" --prefix "$SNAPSHOT_PREFIX/" --max-keys 1
  expect_denied "snapshot list tfstate bucket" \
    credential_s3 "$BEX_AGENT_SNAPSHOT_S3_ACCESS_KEY" "$BEX_AGENT_SNAPSHOT_S3_SECRET_KEY" \
      s3api list-objects-v2 --bucket "$TFSTATE_BUCKET" --max-keys 1
  expect_denied "snapshot list account buckets" \
    credential_s3 "$BEX_AGENT_SNAPSHOT_S3_ACCESS_KEY" "$BEX_AGENT_SNAPSHOT_S3_SECRET_KEY" \
      s3api list-buckets
  expect_denied "snapshot list unrelated bucket" \
    credential_s3_at "$BEX_AGENT_SNAPSHOT_S3_ACCESS_KEY" "$BEX_AGENT_SNAPSHOT_S3_SECRET_KEY" \
      "$unrelated_endpoint" "$unrelated_region" \
      s3api list-objects-v2 --bucket "$unrelated" --max-keys 1
  expect_allowed "snapshot delete object" \
    credential_s3 "$BEX_AGENT_SNAPSHOT_S3_ACCESS_KEY" "$BEX_AGENT_SNAPSHOT_S3_SECRET_KEY" \
      s3 rm "$probe_uri"
}

case "$COMMAND" in
  provision)
    command -v docker >/dev/null || { echo "error: docker not found" >&2; exit 1; }
    command -v jq >/dev/null || { echo "error: jq not found" >&2; exit 1; }
    require_nonempty S3_ENDPOINT "$S3_ENDPOINT"
    require_nonempty S3_REGION "$S3_REGION"
    require_nonempty TF_STATE_ACCESS_KEY/AWS_ACCESS_KEY_ID "$ROOT_ACCESS"
    require_nonempty TF_STATE_SECRET_KEY/AWS_SECRET_ACCESS_KEY "$ROOT_SECRET"
    export BEX_AGENT_SNAPSHOT_S3_ENDPOINT="$S3_ENDPOINT"
    export BEX_AGENT_SNAPSHOT_S3_BUCKET="$SNAPSHOT_BUCKET"
    export BEX_AGENT_SNAPSHOT_S3_REGION="$S3_REGION"
    export BEX_AGENT_SNAPSHOT_S3_PREFIX="$SNAPSHOT_PREFIX"
    ensure_bucket
    ensure_user_and_keys
    upsert_env
    echo "updated .env names for BEX_AGENT_SNAPSHOT_S3_* (values not printed)"
    echo "next: point kubectl at prod and run: $0 install"
    ;;
  verify)
    command -v docker >/dev/null || { echo "error: docker not found" >&2; exit 1; }
    command -v jq >/dev/null || { echo "error: jq not found" >&2; exit 1; }
    require_nonempty S3_ENDPOINT "$S3_ENDPOINT"
    require_nonempty S3_REGION "$S3_REGION"
    require_nonempty TF_STATE_ACCESS_KEY/AWS_ACCESS_KEY_ID "$ROOT_ACCESS"
    require_nonempty TF_STATE_SECRET_KEY/AWS_SECRET_ACCESS_KEY "$ROOT_SECRET"
    verify_matrix
    ;;
  install)
    install_secret
    ;;
esac
