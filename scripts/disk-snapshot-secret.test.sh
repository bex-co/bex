#!/usr/bin/env bash
# Guards for scripts/disk-snapshot-secret.sh (ADR082 D5, w1/m87).
#
# These test the refusals, not the happy path: provisioning needs real Wasabi
# and a real cluster, but every guard below is a place where getting it wrong
# would put a full copy of tenant filesystems somewhere it must never be, or
# hand bex-api authority it must never have. Those are checkable offline.
set -euo pipefail
cd "$(dirname "$0")/.."

SCRIPT=scripts/disk-snapshot-secret.sh
failures=0
pass() { echo "PASS  $1"; }
fail() { echo "FAIL  $1" >&2; failures=$((failures + 1)); }

run() { env -i PATH="$PATH" HOME="$HOME" "$@" 2>&1 || true; }

# --- usage ---
out="$(run bash "$SCRIPT" 2>&1 || true)"
grep -q 'usage:' <<<"$out" && pass "no subcommand prints usage" || fail "no subcommand should print usage"

out="$(run bash "$SCRIPT" bogus 2>&1 || true)"
grep -q 'usage:' <<<"$out" && pass "unknown subcommand refused" || fail "unknown subcommand should be refused"

# --- the bucket must be dedicated ---
# Sharing with Terraform state would put every tenant's filesystem next to the
# credentials that rebuild the platform.
out="$(run env BEX_DISK_SNAPSHOT_BUCKET=bex-tfstate TF_STATE_BUCKET=bex-tfstate \
  bash "$SCRIPT" install 2>&1 || true)"
grep -q 'must not be bex-tfstate' <<<"$out" \
  && pass "refuses the Terraform state bucket" \
  || fail "must refuse bex-tfstate as the snapshot bucket"

# The committed IAM policies name the bucket literally, so any other bucket
# would silently get no policy rather than the wrong one.
out="$(run env BEX_DISK_SNAPSHOT_BUCKET=some-other-bucket bash "$SCRIPT" install 2>&1 || true)"
grep -q 'pinned to bex-disk-snapshots' <<<"$out" \
  && pass "refuses a bucket the committed policies do not cover" \
  || fail "must refuse a bucket the committed policies do not name"

# --- the aws CLI image must be digest-pinned ---
out="$(run env AWS_CLI_IMAGE=amazon/aws-cli:latest bash "$SCRIPT" install 2>&1 || true)"
grep -q 'pinned by sha256 digest' <<<"$out" \
  && pass "refuses a floating aws-cli image" \
  || fail "must refuse an unpinned AWS_CLI_IMAGE"

# --- install requires every credential, including both identities ---
for missing in BEX_DISK_SNAPSHOT_ACCESS_KEY BEX_DISK_SNAPSHOT_SECRET_KEY \
               BEX_DISK_SNAPSHOT_READ_ACCESS_KEY BEX_DISK_SNAPSHOT_READ_SECRET_KEY \
               BEX_DISK_SNAPSHOT_AGE_PUBLIC_KEY; do
  args=(env DRY_RUN=1)
  for v in BEX_DISK_SNAPSHOT_ACCESS_KEY BEX_DISK_SNAPSHOT_SECRET_KEY \
           BEX_DISK_SNAPSHOT_READ_ACCESS_KEY BEX_DISK_SNAPSHOT_READ_SECRET_KEY \
           BEX_DISK_SNAPSHOT_AGE_PUBLIC_KEY; do
    [ "$v" = "$missing" ] || args+=("$v=set")
  done
  out="$(run "${args[@]}" bash "$SCRIPT" install 2>&1 || true)"
  grep -q "$missing is missing or empty" <<<"$out" \
    && pass "install refuses without $missing" \
    || fail "install should name $missing when it is absent"
done

# --- DRY_RUN installs nothing ---
out="$(run env DRY_RUN=1 \
  BEX_DISK_SNAPSHOT_ACCESS_KEY=a BEX_DISK_SNAPSHOT_SECRET_KEY=b \
  BEX_DISK_SNAPSHOT_READ_ACCESS_KEY=c BEX_DISK_SNAPSHOT_READ_SECRET_KEY=d \
  BEX_DISK_SNAPSHOT_AGE_PUBLIC_KEY=age1test bash "$SCRIPT" install 2>&1 || true)"
grep -q 'DRY_RUN: would install' <<<"$out" \
  && pass "DRY_RUN reports without touching the cluster" \
  || fail "DRY_RUN should report the Secrets it would install"

# --- the two IAM policies must stay least-privilege ---
# The operator writes and deletes; bex-api must be able to do NEITHER. A policy
# edit that hands bex-api PutObject or DeleteObject is exactly the mistake this
# separation exists to prevent, and it would not show up in any Go test.
read_policy=infra/wasabi/disk-snapshot-read-s3-policy.json
write_policy=infra/wasabi/disk-snapshot-s3-policy.json

for verb in s3:PutObject s3:DeleteObject s3:AbortMultipartUpload; do
  if jq -e --arg v "$verb" '[.Statement[].Action[]] | index($v)' "$read_policy" >/dev/null 2>&1; then
    fail "read-only policy grants $verb — bex-api must never write or delete"
  else
    pass "read-only policy withholds $verb"
  fi
done
for verb in s3:PutObject s3:DeleteObject s3:GetObject; do
  if jq -e --arg v "$verb" '[.Statement[].Action[]] | index($v)' "$write_policy" >/dev/null 2>&1; then
    pass "operator policy grants $verb"
  else
    fail "operator policy is missing $verb — backup, restore or purge would fail"
  fi
done

# Neither policy may name any bucket but the dedicated one.
for policy in "$read_policy" "$write_policy"; do
  others="$(jq -r '[.Statement[].Resource] | flatten | map(select(test("bex-disk-snapshots") | not)) | length' "$policy")"
  if [ "$others" = "0" ]; then
    pass "$(basename "$policy") is scoped to bex-disk-snapshots only"
  else
    fail "$(basename "$policy") names a bucket other than bex-disk-snapshots"
  fi
done

# --- the three contract details the w1/m87/t004 live drill caught ---
# Each of these failed silently or late: the credential name only surfaced when
# an upload was attempted, the namespace only when a Job tried to start, and the
# age key only after a snapshot had already been taken and could not be restored.

# 1. The backup Job hands the whole Secret to the AWS SDK via envFrom, so the
#    keys must be AWS_ACCESS_KEY_ID / AWS_SECRET_ACCESS_KEY. BEX_DISK_SNAPSHOT_*
#    names produce "no EC2 IMDS role found" at upload time.
if grep -q "printf 'AWS_ACCESS_KEY_ID=%s" "$SCRIPT" && grep -q "printf 'AWS_SECRET_ACCESS_KEY=%s" "$SCRIPT"; then
  pass "operator Secret uses the AWS_* key names the backup Job's envFrom needs"
else
  fail "operator Secret must carry AWS_ACCESS_KEY_ID/AWS_SECRET_ACCESS_KEY"
fi

# 2. The Job runs beside its App (ADR043 D8) and nothing projects the credential,
#    so it must be installed into every tenant namespace.
if grep -q "for ns in \$(tenant_namespaces)" "$SCRIPT"; then
  pass "operator Secret is installed into every tenant namespace"
else
  fail "operator Secret must reach tenant namespaces; bex-system alone leaves Jobs in CreateContainerConfigError"
fi

# 3. age-keygen writes comment lines above the key; storing the file makes the
#    restore fail with "malformed secret key: mixed case".
if grep -q "grep -o 'AGE-SECRET-KEY-\[A-Z0-9\]\*'" "$SCRIPT"; then
  pass "age Secret stores the bare key, not the keygen file"
else
  fail "age Secret must hold only the AGE-SECRET-KEY line"
fi

if [ "$failures" -ne 0 ]; then
  echo "error: $failures check(s) failed" >&2
  exit 1
fi
echo "all checks passed"
