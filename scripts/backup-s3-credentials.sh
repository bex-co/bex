#!/usr/bin/env bash
# Provision, verify, and retire the per-store backup S3 credentials (w5/m63,
# docs/ADR050-encrypted-platform-backups.md §3).
#
# Today every off-cluster backup destination writes with the SAME shared
# Terraform-state root credential (TF_STATE_ACCESS_KEY/SECRET_KEY), which can
# read, write, and delete every backup in bex-tfstate. This script replaces that
# with two Wasabi IAM identities PER STORE, each scoped to that store's own
# bucket prefix:
#
#   <store>-backup-writer : PutObject/DeleteObject/ListBucket, NO GetObject
#                           -> installed in-cluster as the backup Job's Secret
#   <store>-backup-reader : GetObject/ListBucket only
#                           -> NOT installed in-cluster; captured to a mode-0600
#                              file for the operator to record in .env, where the
#                              restore scripts source it (ADR050 §Recovery flow)
#
# A leaked writer Secret can then overwrite/delete backups (an integrity concern
# tracked separately) but can no longer READ their contents. Combined with Tier A
# age encryption (etcd/OpenBao/KeyValue) and Tier B SSE (Barman Postgres), backup
# confidentiality no longer rides on the shared root credential staying secret.
#
# Store registry (prefix under bex-tfstate | writer Secret ns/name | secret shape):
#   etcd            etcd-snapshots     kube-system/etcd-backup-s3     endpoint+aws
#   openbao         openbao-snapshots  secrets/openbao-backup-s3      endpoint+aws
#   keyvalue        keyvalue           <apps-ns>/bex-kv-backup-s3     aws-only
#   bex-db          bex-db             bex-system/bex-db-backup-s3    aws-only
#   tenant-postgres postgres           <apps-ns>/bex-db-backup-s3     aws-only
#   auth-dbs        auth-dbs           auth/auth-dbs-backup-s3        aws-only
#
# Decision (per-store, not shared): bex-db and tenant-postgres get DISTINCT writer
# identities even though their in-cluster Secrets share the name bex-db-backup-s3
# (they live in different namespaces — bex-system vs the apps namespace — and back
# different prefixes: bex-db vs postgres). Sharing one credential across both would
# re-widen the very blast radius this script exists to shrink.
#
# Secret values stay out of argv, stdout, Git, and generated manifests. The AWS
# CLI runs in a pinned container (Wasabi IAM is AWS-compatible).
#
# Usage:
#   scripts/backup-s3-credentials.sh provision      [store...]   # default: all
#   scripts/backup-s3-credentials.sh verify         [store...]
#   scripts/backup-s3-credentials.sh revoke-legacy               # all stores
#
# `provision` is idempotent: it creates missing IAM users/keys, replaces the
# prefix-scoped inline policies, installs the writer Secrets (preserving each
# store's non-credential keys), and writes the reader credentials to
# ./.backup-reader-creds.env (mode 0600, gitignored — never committed).
# `verify` proves the positive/negative access matrix per store with a random
# probe. `revoke-legacy` refuses until every backup Secret carries a per-store
# writer credential rather than the shared TF_STATE_* root key.
set -euo pipefail
cd "$(dirname "$0")/.."

COMMAND="${1:-}"
case "$COMMAND" in
  provision|verify|revoke-legacy) ;;
  *) echo "usage: $0 {provision|verify|revoke-legacy} [store...]" >&2; exit 2 ;;
esac
shift || true

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
S3_ENDPOINT="${TF_STATE_ENDPOINT:-}"
S3_REGION="${TF_STATE_REGION:-}"
BUCKET="${TF_STATE_BUCKET:-bex-tfstate}"
APPS_NAMESPACE="${BEX_APPS_NAMESPACE:-default}"
READER_FILE="${BACKUP_READER_CREDS_FILE:-.backup-reader-creds.env}"

ROOT_ACCESS="${TF_STATE_ACCESS_KEY:-${AWS_ACCESS_KEY_ID:-}}"
ROOT_SECRET="${TF_STATE_SECRET_KEY:-${AWS_SECRET_ACCESS_KEY:-}}"

# Store registry. Each entry: prefix | writer-secret-namespace | writer-secret-name | shape
# shape=endpoint => the writer Secret also carries S3_ENDPOINT/S3_BUCKET/AWS_DEFAULT_REGION
#                    (the etcd/OpenBao CronJobs read those via envFrom); aws => AWS keys only.
declare -A STORE_PREFIX=(
  [etcd]=etcd-snapshots
  [openbao]=openbao-snapshots
  [keyvalue]=keyvalue
  [bex-db]=bex-db
  [tenant-postgres]=postgres
  [auth-dbs]=auth-dbs
)
declare -A STORE_SECRET_NS=(
  [etcd]=kube-system
  [openbao]=secrets
  [keyvalue]="$APPS_NAMESPACE"
  [bex-db]=bex-system
  [tenant-postgres]="$APPS_NAMESPACE"
  [auth-dbs]=auth
)
declare -A STORE_SECRET_NAME=(
  [etcd]=etcd-backup-s3
  [openbao]=openbao-backup-s3
  [keyvalue]=bex-kv-backup-s3
  [bex-db]=bex-db-backup-s3
  [tenant-postgres]=bex-db-backup-s3
  [auth-dbs]=auth-dbs-backup-s3
)
declare -A STORE_SHAPE=(
  [etcd]=endpoint
  [openbao]=endpoint
  [keyvalue]=aws
  [bex-db]=aws
  [tenant-postgres]=aws
  [auth-dbs]=aws
)
ALL_STORES=(etcd openbao keyvalue bex-db tenant-postgres auth-dbs)

require_nonempty() {
  local name="$1" value="$2"
  [ -n "$value" ] || { echo "error: $name is missing or empty" >&2; exit 1; }
}

selected_stores() {
  if [ "$#" -eq 0 ]; then
    printf '%s\n' "${ALL_STORES[@]}"
    return
  fi
  local store
  for store in "$@"; do
    [ -n "${STORE_PREFIX[$store]:-}" ] || { echo "error: unknown store: $store" >&2; exit 1; }
    printf '%s\n' "$store"
  done
}

# root_iam / root_s3 run as the out-of-band bootstrap credential, used ONLY to
# create/rotate the scoped identities — never mounted into any workload.
root_iam() {
  (
    export AWS_ACCESS_KEY_ID="$ROOT_ACCESS"
    export AWS_SECRET_ACCESS_KEY="$ROOT_SECRET"
    export AWS_DEFAULT_REGION=us-east-1
    docker run --rm -i \
      -e AWS_ACCESS_KEY_ID -e AWS_SECRET_ACCESS_KEY -e AWS_DEFAULT_REGION \
      -v "$1:/policy.json:ro" \
      "$AWS_CLI_IMAGE" --endpoint-url "$IAM_ENDPOINT" iam "${@:2}"
  )
}

root_iam_nofile() {
  (
    export AWS_ACCESS_KEY_ID="$ROOT_ACCESS"
    export AWS_SECRET_ACCESS_KEY="$ROOT_SECRET"
    export AWS_DEFAULT_REGION=us-east-1
    docker run --rm -i \
      -e AWS_ACCESS_KEY_ID -e AWS_SECRET_ACCESS_KEY -e AWS_DEFAULT_REGION \
      "$AWS_CLI_IMAGE" --endpoint-url "$IAM_ENDPOINT" iam "$@"
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

render_policy() {
  # $1 template file, $2 prefix -> stdout rendered JSON
  sed -e "s|__BUCKET__|$BUCKET|g" -e "s|__PREFIX__|$2|g" "$1"
}

put_scoped_policy() {
  local user="$1" policy_name="$2" template="$3" prefix="$4" tmp
  tmp="$(mktemp "${TMPDIR:-/tmp}/bex-backup-policy.XXXXXX.json")"
  render_policy "$template" "$prefix" >"$tmp"
  root_iam "$tmp" put-user-policy --user-name "$user" \
    --policy-name "$policy_name" --policy-document file:///policy.json >/dev/null
  rm -f "$tmp"
}

ensure_user() {
  local user="$1" policy_name="$2" template="$3" prefix="$4" extra_inline attached groups
  if ! root_iam_nofile get-user --user-name "$user" >/dev/null 2>&1; then
    root_iam_nofile create-user --user-name "$user" >/dev/null
    echo "created IAM user: $user"
  fi
  put_scoped_policy "$user" "$policy_name" "$template" "$prefix"
  echo "applied prefix-scoped policy: $user/$policy_name ($prefix)"
  # Refuse ambiguous authority: exactly one inline policy, nothing attached.
  extra_inline="$(root_iam_nofile list-user-policies --user-name "$user" \
    | jq -r --arg expected "$policy_name" '[.PolicyNames[] | select(. != $expected)] | length')"
  attached="$(root_iam_nofile list-attached-user-policies --user-name "$user" | jq '.AttachedPolicies | length')"
  groups="$(root_iam_nofile list-groups-for-user --user-name "$user" | jq '.Groups | length')"
  [ "$extra_inline" -eq 0 ] && [ "$attached" -eq 0 ] && [ "$groups" -eq 0 ] || {
    echo "error: $user has an unexpected inline/attached/group policy; refusing ambiguous authority" >&2
    exit 1
  }
}

# mint_access_key prints "ACCESS SECRET" on stdout for the caller to consume in a
# subshell capture; the secret never enters argv.
mint_access_key() {
  local user="$1" key_json access secret key_count
  key_count="$(root_iam_nofile list-access-keys --user-name "$user" | jq '.AccessKeyMetadata | length')"
  [ "$key_count" -lt 2 ] || {
    echo "error: $user already has two access keys; rotate/delete one before re-provisioning" >&2
    exit 1
  }
  key_json="$(root_iam_nofile create-access-key --user-name "$user")"
  access="$(jq -r '.AccessKey.AccessKeyId' <<<"$key_json")"
  secret="$(jq -r '.AccessKey.SecretAccessKey' <<<"$key_json")"
  [ -n "$access" ] && [ -n "$secret" ] || { echo "error: Wasabi returned an incomplete key for $user" >&2; exit 1; }
  printf '%s %s' "$access" "$secret"
}

install_writer_secret() {
  local namespace="$1" name="$2" shape="$3" access="$4" secret="$5" doc
  if [ "$shape" = endpoint ]; then
    doc="$(jq -cn \
      --arg ns "$namespace" --arg name "$name" \
      --arg endpoint "$S3_ENDPOINT" --arg bucket "$BUCKET" --arg region "$S3_REGION" \
      --arg access "$access" --arg secret "$secret" \
      '{apiVersion:"v1",kind:"Secret",type:"Opaque",
        metadata:{namespace:$ns,name:$name,labels:{"app.kubernetes.io/managed-by":"backup-s3-credentials"}},
        stringData:{S3_ENDPOINT:$endpoint,S3_BUCKET:$bucket,AWS_DEFAULT_REGION:$region,
                    AWS_ACCESS_KEY_ID:$access,AWS_SECRET_ACCESS_KEY:$secret}}')"
  else
    doc="$(jq -cn \
      --arg ns "$namespace" --arg name "$name" \
      --arg access "$access" --arg secret "$secret" \
      '{apiVersion:"v1",kind:"Secret",type:"Opaque",
        metadata:{namespace:$ns,name:$name,labels:{"app.kubernetes.io/managed-by":"backup-s3-credentials"}},
        stringData:{AWS_ACCESS_KEY_ID:$access,AWS_SECRET_ACCESS_KEY:$secret}}')"
  fi
  printf '%s' "$doc" | kubectl apply -f - >/dev/null
}

writer_secret_uses_root() {
  # True when the in-cluster writer Secret still carries the shared root key.
  local namespace="$1" name="$2" access
  access="$(kubectl -n "$namespace" get secret "$name" -o json 2>/dev/null \
    | jq -r '.data.AWS_ACCESS_KEY_ID // "" | @base64d')"
  [ -n "$access" ] && [ "$access" = "$ROOT_ACCESS" ]
}

record_reader() {
  local store="$1" access="$2" secret="$3" up
  up="$(printf '%s' "$store" | tr '[:lower:]-' '[:upper:]_')"
  {
    printf 'BEX_BACKUP_READER_%s_ACCESS_KEY_ID=%s\n' "$up" "$access"
    printf 'BEX_BACKUP_READER_%s_SECRET_ACCESS_KEY=%s\n' "$up" "$secret"
  } >>"$READER_FILE"
}

provision_store() {
  local store="$1"
  local prefix="${STORE_PREFIX[$store]}"
  local ns="${STORE_SECRET_NS[$store]}" name="${STORE_SECRET_NAME[$store]}" shape="${STORE_SHAPE[$store]}"
  local writer="bex-backup-writer-$store" reader="bex-backup-reader-$store"
  local wcred rcred waccess wsecret raccess rsecret

  ensure_user "$writer" BexBackupWrite infra/wasabi/backup-writer-policy.json "$prefix"
  ensure_user "$reader" BexBackupRead infra/wasabi/backup-reader-policy.json "$prefix"

  wcred="$(mint_access_key "$writer")"
  waccess="${wcred%% *}"; wsecret="${wcred#* }"
  install_writer_secret "$ns" "$name" "$shape" "$waccess" "$wsecret"
  echo "installed writer Secret: $ns/$name ($store)"
  unset wcred waccess wsecret

  rcred="$(mint_access_key "$reader")"
  raccess="${rcred%% *}"; rsecret="${rcred#* }"
  record_reader "$store" "$raccess" "$rsecret"
  echo "recorded reader credential for $store -> $READER_FILE (move into .env, then delete)"
  unset rcred raccess rsecret
}

expect_allowed() {
  local label="$1"; shift
  if "$@" >/dev/null 2>&1; then printf 'PASS  allow  %s\n' "$label"
  else printf 'FAIL  allow  %s\n' "$label" >&2; return 1; fi
}

expect_denied() {
  local label="$1" output status; shift
  set +e; output="$("$@" 2>&1)"; status=$?; set -e
  if [ "$status" -eq 0 ]; then
    printf 'FAIL  deny   %s (request unexpectedly succeeded)\n' "$label" >&2; return 1
  elif grep -Eqi 'AccessDenied|Forbidden|AllAccessDisabled|NoSuchBucket|status code:[[:space:]]*403' <<<"$output"; then
    printf 'PASS  deny   %s\n' "$label"
  else
    printf 'FAIL  deny   %s (no access-denied response)\n' "$label" >&2; return 1
  fi
}

verify_store() {
  local store="$1"
  local prefix="${STORE_PREFIX[$store]}"
  local ns="${STORE_SECRET_NS[$store]}" name="${STORE_SECRET_NAME[$store]}"
  local waccess wsecret raccess rsecret probe up
  waccess="$(kubectl -n "$ns" get secret "$name" -o json | jq -r '.data.AWS_ACCESS_KEY_ID // "" | @base64d')"
  wsecret="$(kubectl -n "$ns" get secret "$name" -o json | jq -r '.data.AWS_SECRET_ACCESS_KEY // "" | @base64d')"
  [ -n "$waccess" ] && [ -n "$wsecret" ] || { echo "error: $ns/$name lacks writer credentials" >&2; exit 1; }
  up="$(printf '%s' "$store" | tr '[:lower:]-' '[:upper:]_')"
  raccess="$(eval "printf '%s' \"\${BEX_BACKUP_READER_${up}_ACCESS_KEY_ID:-}\"")"
  rsecret="$(eval "printf '%s' \"\${BEX_BACKUP_READER_${up}_SECRET_ACCESS_KEY:-}\"")"
  [ -n "$raccess" ] && [ -n "$rsecret" ] || {
    echo "error: reader credential for $store not in env; source $READER_FILE (or .env) first" >&2; exit 1; }

  probe="s3://$BUCKET/$prefix/.bex-security-probe/$(date -u +%Y%m%dT%H%M%SZ)-$$"
  printf 'backup credential matrix (%s, prefix %s)\n' "$store" "$prefix"
  # writer: put / list / delete allowed; get DENIED.
  printf 'w5/m63 probe\n' | expect_allowed "writer put"    credential_s3 "$waccess" "$wsecret" s3 cp - "$probe"
  expect_allowed "writer list prefix" credential_s3 "$waccess" "$wsecret" s3 ls "s3://$BUCKET/$prefix/"
  expect_denied  "writer get (must be denied)" credential_s3 "$waccess" "$wsecret" s3 cp "$probe" -
  # reader: get / list allowed; put / delete DENIED.
  expect_allowed "reader get"  credential_s3 "$raccess" "$rsecret" s3 cp "$probe" -
  expect_allowed "reader list prefix" credential_s3 "$raccess" "$rsecret" s3 ls "s3://$BUCKET/$prefix/"
  expect_denied  "reader put (must be denied)"    credential_s3 "$raccess" "$rsecret" s3 cp - "${probe}-reader"
  expect_denied  "reader delete (must be denied)" credential_s3 "$raccess" "$rsecret" s3 rm "$probe"
  # cross-prefix isolation: neither identity may list the whole bucket root.
  expect_denied  "writer list bucket root" credential_s3 "$waccess" "$wsecret" s3 ls "s3://$BUCKET/"
  expect_denied  "reader list bucket root" credential_s3 "$raccess" "$rsecret" s3 ls "s3://$BUCKET/"
  # cleanup with the writer (its delete is the last allowed op above's counterpart).
  credential_s3 "$waccess" "$wsecret" s3 rm "$probe" >/dev/null 2>&1 || true
  unset waccess wsecret raccess rsecret
}

case "$COMMAND" in
  provision)
    require_nonempty TF_STATE_ACCESS_KEY "$ROOT_ACCESS"
    require_nonempty TF_STATE_SECRET_KEY "$ROOT_SECRET"
    require_nonempty TF_STATE_ENDPOINT "$S3_ENDPOINT"
    require_nonempty TF_STATE_REGION "$S3_REGION"
    umask 077
    : >"$READER_FILE"
    while IFS= read -r store; do provision_store "$store"; done < <(selected_stores "$@")
    echo "reader credentials written to $READER_FILE (mode 0600); merge into .env and delete the file"
    ;;
  verify)
    require_nonempty TF_STATE_ENDPOINT "$S3_ENDPOINT"
    require_nonempty TF_STATE_REGION "$S3_REGION"
    while IFS= read -r store; do verify_store "$store"; done < <(selected_stores "$@")
    ;;
  revoke-legacy)
    require_nonempty TF_STATE_ACCESS_KEY "$ROOT_ACCESS"
    remaining=()
    for store in "${ALL_STORES[@]}"; do
      if writer_secret_uses_root "${STORE_SECRET_NS[$store]}" "${STORE_SECRET_NAME[$store]}"; then
        remaining+=("$store (${STORE_SECRET_NS[$store]}/${STORE_SECRET_NAME[$store]})")
      fi
    done
    if [ "${#remaining[@]}" -gt 0 ]; then
      echo "error: refusing to declare migration complete; these Secrets still carry the shared root key:" >&2
      printf '  %s\n' "${remaining[@]}" >&2
      exit 1
    fi
    echo "all backup Secrets carry per-store writer credentials; the shared TF_STATE_* root key no longer writes backups"
    echo "(the root key remains in out-of-band custody for Terraform/IAM administration)"
    ;;
esac
