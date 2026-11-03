#!/usr/bin/env bash
# Provision the ADR095 agent-attachments bucket, its one bucket-scoped Wasabi
# IAM user, and the out-of-band Secret that arms bex-api.
#
# Usage:
#   scripts/agent-attachments-s3-credentials.sh provision
#   scripts/agent-attachments-s3-credentials.sh verify
#   BEX_KUBE_CONTEXT=hetzner-prod scripts/agent-attachments-s3-credentials.sh install
#
# Modeled on scripts/disk-snapshot-secret.sh with one identity instead of two:
# bex-api is the only reader AND writer of this bucket (it streams uploads in,
# copies inputs into sandboxes, purges retired objects, and signs downloads).
# A presigned URL it hands a client grants GET on one object for 15 minutes,
# never the credential itself.
#
# Secret material stays out of argv, stdout, Git, and generated manifests.
set -euo pipefail
cd "$(dirname "$0")/.."

COMMAND="${1:-}"
case "$COMMAND" in
  provision|verify|install) ;;
  *) echo "usage: $0 {provision|verify|install}" >&2; exit 2 ;;
esac

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
secret_name="${BEX_AGENT_ATTACHMENTS_SECRET_NAME:-bex-agent-attachments}"

IAM_USER="${BEX_AGENT_ATTACHMENTS_IAM_USER:-bex-agent-attachments}"
POLICY_NAME="${BEX_AGENT_ATTACHMENTS_IAM_POLICY:-BexAgentAttachments}"
BUCKET="${BEX_AGENT_ATTACHMENTS_S3_BUCKET:-bex-agent-attachments}"
TFSTATE_BUCKET="${TF_STATE_BUCKET:-bex-tfstate}"
IAM_ENDPOINT="${WASABI_IAM_ENDPOINT:-https://iam.wasabisys.com}"
S3_ENDPOINT="${BEX_AGENT_ATTACHMENTS_S3_ENDPOINT:-${TF_STATE_ENDPOINT:-}}"
S3_REGION="${BEX_AGENT_ATTACHMENTS_S3_REGION:-${TF_STATE_REGION:-}}"
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
# Tenant files never share a bucket with the credentials that rebuild the
# platform, and the committed policy names this bucket literally.
[ "$BUCKET" != "$TFSTATE_BUCKET" ] || { echo "error: attachments bucket must not be $TFSTATE_BUCKET" >&2; exit 1; }
[ "$BUCKET" = "bex-agent-attachments" ] || {
  echo "error: the committed IAM policy is pinned to bex-agent-attachments; refusing bucket $BUCKET" >&2
  exit 1
}

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
user_s3() {
  with_credential "${BEX_AGENT_ATTACHMENTS_S3_ACCESS_KEY:-}" "${BEX_AGENT_ATTACHMENTS_S3_SECRET_KEY:-}" "$S3_REGION" \
    --endpoint-url "$S3_ENDPOINT" "$@"
}

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

provision() {
  require_nonempty "root Wasabi access key (TF_STATE_ACCESS_KEY)" "$ROOT_ACCESS"
  require_nonempty "root Wasabi secret key (TF_STATE_SECRET_KEY)" "$ROOT_SECRET"
  require_nonempty "S3 endpoint (BEX_AGENT_ATTACHMENTS_S3_ENDPOINT or TF_STATE_ENDPOINT)" "$S3_ENDPOINT"
  require_nonempty "S3 region (BEX_AGENT_ATTACHMENTS_S3_REGION or TF_STATE_REGION)" "$S3_REGION"

  if root_s3 s3api head-bucket --bucket "$BUCKET" >/dev/null 2>&1; then
    echo "kept attachments bucket: $BUCKET"
  else
    root_s3 s3api create-bucket --bucket "$BUCKET" \
      --create-bucket-configuration LocationConstraint="$S3_REGION" >/dev/null
    echo "created attachments bucket: $BUCKET"
  fi

  if ! iam get-user --user-name "$IAM_USER" >/dev/null 2>&1; then
    iam create-user --user-name "$IAM_USER" >/dev/null
    echo "created IAM user: $IAM_USER"
  fi
  iam put-user-policy --user-name "$IAM_USER" --policy-name "$POLICY_NAME" \
    --policy-document "file:///policies/agent-attachments-s3-policy.json" >/dev/null
  echo "applied least-privilege policy: $IAM_USER/$POLICY_NAME"
  local extra attached groups
  extra="$(iam list-user-policies --user-name "$IAM_USER" | jq -r --arg p "$POLICY_NAME" '[.PolicyNames[] | select(. != $p)] | length')"
  attached="$(iam list-attached-user-policies --user-name "$IAM_USER" | jq '.AttachedPolicies | length')"
  groups="$(iam list-groups-for-user --user-name "$IAM_USER" | jq '.Groups | length')"
  if [ "$extra" != "0" ] || [ "$attached" != "0" ] || [ "$groups" != "0" ]; then
    echo "error: $IAM_USER carries extra policies or group membership; refusing to widen its authority" >&2
    exit 1
  fi

  local keys key_json
  keys="$(iam list-access-keys --user-name "$IAM_USER" | jq '.AccessKeyMetadata | length')"
  if [ "$keys" = "0" ]; then
    key_json="$(iam create-access-key --user-name "$IAM_USER")"
    upsert_env BEX_AGENT_ATTACHMENTS_S3_ACCESS_KEY "$(jq -r '.AccessKey.AccessKeyId' <<<"$key_json")"
    upsert_env BEX_AGENT_ATTACHMENTS_S3_SECRET_KEY "$(jq -r '.AccessKey.SecretAccessKey' <<<"$key_json")"
    echo "minted access key for $IAM_USER (stored in $ENV_FILE)"
  else
    [ -n "${BEX_AGENT_ATTACHMENTS_S3_ACCESS_KEY:-}" ] && [ -n "${BEX_AGENT_ATTACHMENTS_S3_SECRET_KEY:-}" ] || {
      echo "error: $IAM_USER already has an access key but no secret is available locally." >&2
      echo "       Delete the key in Wasabi IAM and re-run provision, or set the pair in .env." >&2
      exit 1
    }
    echo "kept existing access key for $IAM_USER"
  fi
  upsert_env BEX_AGENT_ATTACHMENTS_S3_ENDPOINT "$S3_ENDPOINT"
  upsert_env BEX_AGENT_ATTACHMENTS_S3_REGION "$S3_REGION"
  upsert_env BEX_AGENT_ATTACHMENTS_S3_BUCKET "$BUCKET"
}

# verify proves the credential can write, read and delete one probe object and
# cannot touch the Terraform-state bucket.
verify() {
  require_nonempty BEX_AGENT_ATTACHMENTS_S3_ACCESS_KEY "${BEX_AGENT_ATTACHMENTS_S3_ACCESS_KEY:-}"
  require_nonempty BEX_AGENT_ATTACHMENTS_S3_SECRET_KEY "${BEX_AGENT_ATTACHMENTS_S3_SECRET_KEY:-}"
  require_nonempty S3_ENDPOINT "$S3_ENDPOINT"
  local key
  key="verify/probe-$(date +%s)"
  printf 'bex attachments probe\n' | user_s3 s3 cp - "s3://$BUCKET/$key" >/dev/null
  user_s3 s3 cp "s3://$BUCKET/$key" - >/dev/null
  user_s3 s3 rm "s3://$BUCKET/$key" >/dev/null
  echo "verified put/get/delete on $BUCKET"
  # Only an explicit AccessDenied proves the boundary; any other failure
  # (network, endpoint) proves nothing, so it fails the check.
  local denial
  if denial="$(user_s3 s3api head-bucket --bucket "$TFSTATE_BUCKET" 2>&1)"; then
    echo "error: the attachments credential can reach $TFSTATE_BUCKET" >&2
    exit 1
  fi
  if ! grep -Eq 'AccessDenied|Forbidden|\(403\)' <<<"$denial"; then
    printf '%s\n' "$denial" >&2
    echo "error: could not prove the attachments credential is denied $TFSTATE_BUCKET" >&2
    exit 1
  fi
  echo "verified $TFSTATE_BUCKET is denied"
}

install() {
  require_nonempty BEX_AGENT_ATTACHMENTS_S3_ACCESS_KEY "${BEX_AGENT_ATTACHMENTS_S3_ACCESS_KEY:-}"
  require_nonempty BEX_AGENT_ATTACHMENTS_S3_SECRET_KEY "${BEX_AGENT_ATTACHMENTS_S3_SECRET_KEY:-}"
  require_nonempty S3_ENDPOINT "$S3_ENDPOINT"
  local env_file
  env_file="$(mktemp)"
  chmod 600 "$env_file"
  # shellcheck disable=SC2064
  trap "rm -f '$env_file'" EXIT
  {
    printf 'BEX_AGENT_ATTACHMENTS_S3_ENDPOINT=%s\n' "$S3_ENDPOINT"
    printf 'BEX_AGENT_ATTACHMENTS_S3_BUCKET=%s\n' "$BUCKET"
    printf 'BEX_AGENT_ATTACHMENTS_S3_REGION=%s\n' "$S3_REGION"
    printf 'BEX_AGENT_ATTACHMENTS_S3_ACCESS_KEY=%s\n' "$BEX_AGENT_ATTACHMENTS_S3_ACCESS_KEY"
    printf 'BEX_AGENT_ATTACHMENTS_S3_SECRET_KEY=%s\n' "$BEX_AGENT_ATTACHMENTS_S3_SECRET_KEY"
  } >"$env_file"
  "${KUBE[@]}" -n "$namespace" create secret generic "$secret_name" \
    --from-env-file="$env_file" --dry-run=client -o yaml | "${KUBE[@]}" apply -f - >/dev/null
  rm -f "$env_file"
  trap - EXIT
  echo "installed $namespace/$secret_name (bex-api: attachments bucket)"
}

case "$COMMAND" in
  provision) provision ;;
  verify) verify ;;
  install) install ;;
esac
