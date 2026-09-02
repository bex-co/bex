#!/usr/bin/env bash
# Provision the ADR082 D5 persistent-disk snapshot bucket, TWO bucket-scoped
# Wasabi IAM users, a disk-dedicated age keypair, and the out-of-band Secrets
# that arm the operator and bex-api (w1/m87).
#
# Usage:
#   scripts/disk-snapshot-secret.sh provision
#   scripts/disk-snapshot-secret.sh verify
#   BEX_KUBE_CONTEXT=hetzner-prod scripts/disk-snapshot-secret.sh install
#   DRY_RUN=1 scripts/disk-snapshot-secret.sh install
#
# Modeled on scripts/agent-snapshot-secret.sh, with two differences that matter:
#
#   1. TWO identities, not one. The operator writes and deletes (backup +
#      retention + purge-on-detach); bex-api only LISTS, so it gets a separate
#      read-only credential. bex-api must never be able to write or delete a
#      tenant's backups, and it never holds the age key at all.
#
#   2. An age keypair DEDICATED to disks — deliberately not ADR050's platform
#      key (AGE_BACKUP_*). A disk restore has to decrypt inside the cluster, and
#      ADR050's whole point is that the platform key never goes there. The
#      public half is safe in clear; the private half lands only in a Secret the
#      operator's restore Job mounts.
#
# Secret material stays out of argv, stdout, Git, and generated manifests.
set -euo pipefail
cd "$(dirname "$0")/.."

COMMAND="${1:-}"
case "$COMMAND" in
  provision|verify|install) ;;
  *) echo "usage: $0 {provision|verify|install}" >&2; exit 2 ;;
esac

# BEX_ENV_FILE lets the offline guard tests (disk-snapshot-secret.test.sh)
# point this at /dev/null: after a real provision has upserted values into the
# operator's .env, sourcing it here would satisfy every refusal the tests
# assert. Operators never set it.
ENV_FILE="${BEX_ENV_FILE:-.env}"
if [ -f "$ENV_FILE" ]; then
  set -a
  # shellcheck disable=SC1090,SC1091
  source "$ENV_FILE"
  set +a
fi

KUBE=(kubectl)
if [ -n "${BEX_KUBE_CONTEXT:-}" ]; then
  KUBE=(kubectl --context "$BEX_KUBE_CONTEXT")
fi
namespace="${BEX_SYSTEM_NAMESPACE:-bex-system}"
write_secret_name="${BEX_DISK_SNAPSHOT_SECRET_NAME:-bex-disk-snapshot}"
read_secret_name="${BEX_DISK_SNAPSHOT_READ_SECRET_NAME:-bex-disk-snapshot-read}"
age_secret_name="${BEX_DISK_SNAPSHOT_AGE_SECRET:-bex-disk-snapshot-age}"

WRITE_IAM_USER="${BEX_DISK_SNAPSHOT_IAM_USER:-bex-disk-snapshot}"
READ_IAM_USER="${BEX_DISK_SNAPSHOT_READ_IAM_USER:-bex-disk-snapshot-read}"
WRITE_POLICY_NAME="${BEX_DISK_SNAPSHOT_IAM_POLICY:-BexDiskSnapshot}"
READ_POLICY_NAME="${BEX_DISK_SNAPSHOT_READ_IAM_POLICY:-BexDiskSnapshotRead}"

SNAPSHOT_BUCKET="${BEX_DISK_SNAPSHOT_BUCKET:-bex-disk-snapshots}"
SNAPSHOT_PREFIX="${BEX_DISK_SNAPSHOT_PREFIX:-disks}"
TFSTATE_BUCKET="${TF_STATE_BUCKET:-bex-tfstate}"
IAM_ENDPOINT="${WASABI_IAM_ENDPOINT:-https://iam.wasabisys.com}"
S3_ENDPOINT="${BEX_DISK_SNAPSHOT_ENDPOINT:-${TF_STATE_ENDPOINT:-${BEX_STATIC_S3_ENDPOINT:-}}}"
S3_REGION="${BEX_DISK_SNAPSHOT_REGION:-${TF_STATE_REGION:-${BEX_STATIC_S3_REGION:-}}}"
AWS_CLI_IMAGE="${AWS_CLI_IMAGE:-amazon/aws-cli:2.22.35@sha256:6977c83ae3dc99f28fcf8276b9ea5eec33833cd5be40574b34112e98113ec7a2}"
AGE_IMAGE="${AGE_IMAGE:-}"

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

# The bucket is dedicated. Sharing it with Terraform state would put a full copy
# of every tenant filesystem next to the credentials that rebuild the platform.
[ "$SNAPSHOT_BUCKET" != "$TFSTATE_BUCKET" ] || {
  echo "error: snapshot bucket must not be $TFSTATE_BUCKET" >&2
  exit 1
}
# The committed policies name the bucket literally, so a different bucket would
# silently get no policy rather than the wrong one.
[ "$SNAPSHOT_BUCKET" = "bex-disk-snapshots" ] || {
  echo "error: committed IAM policies are pinned to bex-disk-snapshots; refusing bucket $SNAPSHOT_BUCKET" >&2
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

iam() { root_aws --endpoint-url "$IAM_ENDPOINT" iam "$@"; }

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

# tenant_namespaces lists the workspace namespaces an App can live in. A disk's
# backup Job runs beside its App (ADR043 D8), so its credential has to be there.
tenant_namespaces() {
  "${KUBE[@]}" get ns -o jsonpath='{range .items[*]}{.metadata.name}{"\n"}{end}' 2>/dev/null \
    | grep -E '^tea-' || true
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
  # Put/GetBucketEncryption. Same handling as the agent-snapshot script: prove
  # the bucket exists and accept the documented unsupported-API answer.
  #
  # Note this is at-rest encryption BY THE PROVIDER. It is not what protects a
  # tenant's filesystem — the age layer does that, before the bytes ever leave
  # the cluster.
  local enc enc_err algo
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

# ensure_user_and_keys <user> <policy-name> <policy-file> <access-out-var> <secret-out-var>
#
# Idempotent on the user and the policy; mints a key pair only when the user has
# none, because Wasabi returns the secret exactly once at creation.
ensure_user_and_keys() {
  local user="$1" policy_name="$2" policy_file="$3" access_var="$4" secret_var="$5"
  local key_count key_json extra_inline attached groups

  if ! iam get-user --user-name "$user" >/dev/null 2>&1; then
    iam create-user --user-name "$user" >/dev/null
    echo "created IAM user: $user"
  fi
  iam put-user-policy --user-name "$user" --policy-name "$policy_name" \
    --policy-document "file:///policies/$policy_file" >/dev/null
  echo "applied least-privilege policy: $user/$policy_name"

  # A second policy, an attached managed policy, or a group membership would
  # widen this identity beyond the committed file — which is the whole point of
  # a dedicated user.
  extra_inline="$(iam list-user-policies --user-name "$user" \
    | jq -r --arg expected "$policy_name" '[.PolicyNames[] | select(. != $expected)] | length')"
  attached="$(iam list-attached-user-policies --user-name "$user" | jq '.AttachedPolicies | length')"
  groups="$(iam list-groups-for-user --user-name "$user" | jq '.Groups | length')"
  if [ "$extra_inline" != "0" ] || [ "$attached" != "0" ] || [ "$groups" != "0" ]; then
    echo "error: $user carries extra policies or group membership; refusing to widen its authority" >&2
    exit 1
  fi

  key_count="$(iam list-access-keys --user-name "$user" | jq '.AccessKeyMetadata | length')"
  if [ "$key_count" = "0" ]; then
    key_json="$(iam create-access-key --user-name "$user")"
    printf -v "$access_var" '%s' "$(jq -r '.AccessKey.AccessKeyId' <<<"$key_json")"
    printf -v "$secret_var" '%s' "$(jq -r '.AccessKey.SecretAccessKey' <<<"$key_json")"
    echo "minted access key for $user"
  else
    # Wasabi shows a secret once. If .env already carries a matching pair, keep
    # it; otherwise say so rather than silently installing a broken Secret.
    local existing_access="${!access_var:-}"
    [ -n "$existing_access" ] || {
      echo "error: $user already has an access key but no secret is available locally." >&2
      echo "       Delete the key in Wasabi IAM and re-run provision, or set the pair in .env." >&2
      exit 1
    }
    echo "kept existing access key for $user"
  fi
}

# ensure_age_keypair mints the disk-dedicated recipient pair when absent.
#
# `age-keygen` if present, else the operator's own compiled age (the repo
# already depends on filippo.io/age) via `go run`. The private half is written
# ONLY into the Kubernetes Secret; it never reaches stdout or .env.
ensure_age_keypair() {
  # `|| true`: on a cluster that has never held the Secret, kubectl exits 1 and
  # pipefail would abort the whole provision HERE — after the IAM keys were
  # minted but before they reach .env, orphaning a pair Wasabi shows only once
  # (exactly what happened on the first prod provision, w2/m86).
  local existing
  existing="$("${KUBE[@]}" -n "$namespace" get secret "$age_secret_name" -o json 2>/dev/null \
    | jq -r '.data.private // ""' || true)"
  if [ -n "$existing" ]; then
    echo "kept existing disk age keypair: $namespace/$age_secret_name"
    require_nonempty BEX_DISK_SNAPSHOT_AGE_PUBLIC_KEY "${BEX_DISK_SNAPSHOT_AGE_PUBLIC_KEY:-}"
    return
  fi

  # A 0700 directory with a path that does NOT yet exist: age-keygen refuses to
  # overwrite its -o target, so `mktemp` (which creates the file) would make it
  # fail. The directory carries the permissions instead.
  local keydir keyfile
  keydir="$(mktemp -d)"
  chmod 700 "$keydir"
  keyfile="$keydir/key.age"
  # shellcheck disable=SC2064
  trap "rm -rf '$keydir'" EXIT

  if command -v age-keygen >/dev/null 2>&1; then
    age-keygen -o "$keyfile" >/dev/null 2>&1
  else
    # The repo already depends on filippo.io/age, so no download is needed and
    # nothing is fetched at provisioning time.
    (cd lego/operator && go run filippo.io/age/cmd/age-keygen -o "$keyfile") >/dev/null 2>&1
  fi
  [ -s "$keyfile" ] || { echo "error: could not generate an age keypair" >&2; exit 1; }

  BEX_DISK_SNAPSHOT_AGE_PUBLIC_KEY="$(grep -o 'age1[0-9a-z]*' "$keyfile" | head -1)"
  require_nonempty BEX_DISK_SNAPSHOT_AGE_PUBLIC_KEY "$BEX_DISK_SNAPSHOT_AGE_PUBLIC_KEY"

  "${KUBE[@]}" get namespace "$namespace" >/dev/null 2>&1 || "${KUBE[@]}" create namespace "$namespace" >/dev/null
  # The BARE key, not the file: age-keygen writes two comment lines above it,
  # and handing the whole file to the restore Job fails with "parse identity:
  # malformed secret key: mixed case" — the comments are the mixed case. Found
  # by the w1/m87/t004 drill, after a snapshot had already been taken.
  local private
  private="$(grep -o 'AGE-SECRET-KEY-[A-Z0-9]*' "$keyfile" | head -1)"
  [ -n "$private" ] || { echo "error: could not read the generated private key" >&2; exit 1; }
  for ns in "$namespace" $(tenant_namespaces); do
    "${KUBE[@]}" -n "$ns" create secret generic "$age_secret_name" \
      --from-literal=private="$private" \
      --dry-run=client -o yaml | "${KUBE[@]}" apply -f - >/dev/null
  done
  unset private
  rm -rf "$keydir"
  trap - EXIT
  echo "installed $namespace/$age_secret_name (private half never printed)"
  echo "recipient (safe to record): $BEX_DISK_SNAPSHOT_AGE_PUBLIC_KEY"
}

install_secrets() {
  local env_file
  env_file="$(mktemp)"
  chmod 600 "$env_file"
  # shellcheck disable=SC2064
  trap "rm -f '$env_file'" EXIT

  "${KUBE[@]}" get namespace "$namespace" >/dev/null 2>&1 || "${KUBE[@]}" create namespace "$namespace" >/dev/null

  # Operator: write/delete credentials, consumed by the backup/purge Job via
  # `envFrom`. The key names are AWS_ACCESS_KEY_ID / AWS_SECRET_ACCESS_KEY
  # because the Job hands the whole Secret to the AWS SDK as environment — see
  # DiskSnapshotStore.S3Secret. Naming them BEX_DISK_SNAPSHOT_* instead makes
  # the Job fail at upload with "no EC2 IMDS role found", which is what the
  # w1/m87/t004 drill caught.
  # The age RECIPIENT (public) key rides in this Secret too: the manager
  # manifest arms BEX_DISK_SNAPSHOT_AGE_PUBLIC_KEY through a secretKeyRef on
  # bex-system/$write_secret_name (lego/operator/config/manager/manager.yaml),
  # so without this entry the operator's DiskSnapshots never reports
  # configured and NO backup CronJob is ever created — found by the w2/m86
  # prod arming, where this Secret existed nowhere in bex-system at all.
  {
    printf 'AWS_ACCESS_KEY_ID=%s\n' "${BEX_DISK_SNAPSHOT_ACCESS_KEY:-}"
    printf 'AWS_SECRET_ACCESS_KEY=%s\n' "${BEX_DISK_SNAPSHOT_SECRET_KEY:-}"
    printf 'BEX_DISK_SNAPSHOT_AGE_PUBLIC_KEY=%s\n' "${BEX_DISK_SNAPSHOT_AGE_PUBLIC_KEY:-}"
  } >"$env_file"
  # Installed into bex-system (the manager's secretKeyRef, above) AND every
  # tenant namespace: the Job runs beside the App (ADR043 D8 co-location) and
  # there is no projection for disk snapshots the way BackupSourceNamespace
  # projects the KeyValue credential. A Secret only in bex-system leaves every
  # backup Job in CreateContainerConfigError — found by the w1/m87 drill.
  for ns in "$namespace" $(tenant_namespaces); do
    "${KUBE[@]}" -n "$ns" create secret generic "$write_secret_name" \
      --from-env-file="$env_file" \
      --dry-run=client -o yaml | "${KUBE[@]}" apply -f - >/dev/null
  done
  echo "installed $write_secret_name (operator: write + purge + age recipient) into bex-system and every tenant namespace"

  # bex-api: LIST only. No age key, no write authority.
  {
    printf 'BEX_DISK_SNAPSHOT_ENDPOINT=%s\n' "$S3_ENDPOINT"
    printf 'BEX_DISK_SNAPSHOT_BUCKET=%s\n' "$SNAPSHOT_BUCKET"
    printf 'BEX_DISK_SNAPSHOT_REGION=%s\n' "$S3_REGION"
    printf 'BEX_DISK_SNAPSHOT_PREFIX=%s\n' "$SNAPSHOT_PREFIX"
    printf 'BEX_DISK_SNAPSHOT_ACCESS_KEY=%s\n' "${BEX_DISK_SNAPSHOT_READ_ACCESS_KEY:-}"
    printf 'BEX_DISK_SNAPSHOT_SECRET_KEY=%s\n' "${BEX_DISK_SNAPSHOT_READ_SECRET_KEY:-}"
  } >"$env_file"
  "${KUBE[@]}" -n "$namespace" create secret generic "$read_secret_name" \
    --from-env-file="$env_file" \
    --dry-run=client -o yaml | "${KUBE[@]}" apply -f - >/dev/null
  echo "installed $namespace/$read_secret_name (bex-api: list only)"

  rm -f "$env_file"
  trap - EXIT
}

upsert_env() {
  local key="$1" value="$2"
  [ -f .env ] || touch .env
  chmod 600 .env
  if grep -q "^${key}=" .env; then
    # In-place without a temp file that could outlive a failure.
    python3 - "$key" "$value" <<'PY'
import sys
key, value = sys.argv[1], sys.argv[2]
lines = open(".env").read().splitlines(True)
out = [f"{key}={value}\n" if l.startswith(f"{key}=") else l for l in lines]
open(".env", "w").write("".join(out))
PY
  else
    printf '%s=%s\n' "$key" "$value" >>.env
  fi
}

case "$COMMAND" in
  provision)
    require_nonempty "root S3 access key" "$ROOT_ACCESS"
    require_nonempty "root S3 secret key" "$ROOT_SECRET"
    require_nonempty BEX_DISK_SNAPSHOT_ENDPOINT "$S3_ENDPOINT"
    require_nonempty BEX_DISK_SNAPSHOT_REGION "$S3_REGION"

    ensure_bucket
    ensure_user_and_keys "$WRITE_IAM_USER" "$WRITE_POLICY_NAME" disk-snapshot-s3-policy.json \
      BEX_DISK_SNAPSHOT_ACCESS_KEY BEX_DISK_SNAPSHOT_SECRET_KEY
    ensure_user_and_keys "$READ_IAM_USER" "$READ_POLICY_NAME" disk-snapshot-read-s3-policy.json \
      BEX_DISK_SNAPSHOT_READ_ACCESS_KEY BEX_DISK_SNAPSHOT_READ_SECRET_KEY
    ensure_age_keypair

    upsert_env BEX_DISK_SNAPSHOT_ENDPOINT "$S3_ENDPOINT"
    upsert_env BEX_DISK_SNAPSHOT_BUCKET "$SNAPSHOT_BUCKET"
    upsert_env BEX_DISK_SNAPSHOT_REGION "$S3_REGION"
    upsert_env BEX_DISK_SNAPSHOT_PREFIX "$SNAPSHOT_PREFIX"
    upsert_env BEX_DISK_SNAPSHOT_ACCESS_KEY "${BEX_DISK_SNAPSHOT_ACCESS_KEY:-}"
    upsert_env BEX_DISK_SNAPSHOT_SECRET_KEY "${BEX_DISK_SNAPSHOT_SECRET_KEY:-}"
    upsert_env BEX_DISK_SNAPSHOT_READ_ACCESS_KEY "${BEX_DISK_SNAPSHOT_READ_ACCESS_KEY:-}"
    upsert_env BEX_DISK_SNAPSHOT_READ_SECRET_KEY "${BEX_DISK_SNAPSHOT_READ_SECRET_KEY:-}"
    upsert_env BEX_DISK_SNAPSHOT_AGE_PUBLIC_KEY "${BEX_DISK_SNAPSHOT_AGE_PUBLIC_KEY:-}"

    install_secrets
    echo
    echo "next: scripts/disk-snapshot-secret.sh verify"
    ;;

  install)
    require_nonempty BEX_DISK_SNAPSHOT_ACCESS_KEY "${BEX_DISK_SNAPSHOT_ACCESS_KEY:-}"
    require_nonempty BEX_DISK_SNAPSHOT_SECRET_KEY "${BEX_DISK_SNAPSHOT_SECRET_KEY:-}"
    require_nonempty BEX_DISK_SNAPSHOT_READ_ACCESS_KEY "${BEX_DISK_SNAPSHOT_READ_ACCESS_KEY:-}"
    require_nonempty BEX_DISK_SNAPSHOT_READ_SECRET_KEY "${BEX_DISK_SNAPSHOT_READ_SECRET_KEY:-}"
    require_nonempty BEX_DISK_SNAPSHOT_AGE_PUBLIC_KEY "${BEX_DISK_SNAPSHOT_AGE_PUBLIC_KEY:-}"
    if [ "${DRY_RUN:-}" = "1" ]; then
      echo "DRY_RUN: would install $namespace/{$write_secret_name,$read_secret_name}"
      exit 0
    fi
    install_secrets
    ;;

  verify)
    require_nonempty BEX_DISK_SNAPSHOT_ACCESS_KEY "${BEX_DISK_SNAPSHOT_ACCESS_KEY:-}"
    require_nonempty BEX_DISK_SNAPSHOT_SECRET_KEY "${BEX_DISK_SNAPSHOT_SECRET_KEY:-}"
    require_nonempty BEX_DISK_SNAPSHOT_READ_ACCESS_KEY "${BEX_DISK_SNAPSHOT_READ_ACCESS_KEY:-}"
    require_nonempty BEX_DISK_SNAPSHOT_READ_SECRET_KEY "${BEX_DISK_SNAPSHOT_READ_SECRET_KEY:-}"

    probe="s3://$SNAPSHOT_BUCKET/$SNAPSHOT_PREFIX/.verify-$$"
    failures=0
    pass() { echo "PASS  $1"; }
    fail() { echo "FAIL  $1" >&2; failures=$((failures + 1)); }

    # The operator's identity must be able to complete a full backup lifecycle.
    if printf 'probe' | credential_s3 "$BEX_DISK_SNAPSHOT_ACCESS_KEY" "$BEX_DISK_SNAPSHOT_SECRET_KEY" \
        s3 cp - "$probe" >/dev/null 2>&1; then
      pass "write  operator can PUT under $SNAPSHOT_PREFIX/"
    else
      fail "write  operator cannot PUT under $SNAPSHOT_PREFIX/"
    fi

    # bex-api must be able to LIST — and must NOT be able to write or delete.
    if credential_s3 "$BEX_DISK_SNAPSHOT_READ_ACCESS_KEY" "$BEX_DISK_SNAPSHOT_READ_SECRET_KEY" \
        s3 ls "s3://$SNAPSHOT_BUCKET/$SNAPSHOT_PREFIX/" >/dev/null 2>&1; then
      pass "read   bex-api can LIST"
    else
      fail "read   bex-api cannot LIST"
    fi
    if printf 'nope' | credential_s3 "$BEX_DISK_SNAPSHOT_READ_ACCESS_KEY" "$BEX_DISK_SNAPSHOT_READ_SECRET_KEY" \
        s3 cp - "s3://$SNAPSHOT_BUCKET/$SNAPSHOT_PREFIX/.should-not-exist" >/dev/null 2>&1; then
      fail "deny   bex-api was able to WRITE — its credential is too broad"
      credential_s3 "$BEX_DISK_SNAPSHOT_ACCESS_KEY" "$BEX_DISK_SNAPSHOT_SECRET_KEY" \
        s3 rm "s3://$SNAPSHOT_BUCKET/$SNAPSHOT_PREFIX/.should-not-exist" >/dev/null 2>&1 || true
    else
      pass "deny   bex-api cannot WRITE"
    fi
    if credential_s3 "$BEX_DISK_SNAPSHOT_READ_ACCESS_KEY" "$BEX_DISK_SNAPSHOT_READ_SECRET_KEY" \
        s3 rm "$probe" >/dev/null 2>&1; then
      fail "deny   bex-api was able to DELETE — its credential is too broad"
    else
      pass "deny   bex-api cannot DELETE"
    fi

    # Neither identity may reach Terraform state.
    for who in "$BEX_DISK_SNAPSHOT_ACCESS_KEY:$BEX_DISK_SNAPSHOT_SECRET_KEY:operator" \
               "$BEX_DISK_SNAPSHOT_READ_ACCESS_KEY:$BEX_DISK_SNAPSHOT_READ_SECRET_KEY:bex-api"; do
      IFS=: read -r a s label <<<"$who"
      if credential_s3 "$a" "$s" s3 ls "s3://$TFSTATE_BUCKET/" >/dev/null 2>&1; then
        fail "deny   $label can list $TFSTATE_BUCKET"
      else
        pass "deny   $label cannot reach $TFSTATE_BUCKET"
      fi
    done

    credential_s3 "$BEX_DISK_SNAPSHOT_ACCESS_KEY" "$BEX_DISK_SNAPSHOT_SECRET_KEY" \
      s3 rm "$probe" >/dev/null 2>&1 || true

    if [ "$failures" -ne 0 ]; then
      echo "error: $failures check(s) failed" >&2
      exit 1
    fi
    echo "all checks passed"
    ;;
esac
