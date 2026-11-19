#!/usr/bin/env bash
# Guards for scripts/object-storage-s3-credentials.sh (ADR112 D9).
#
# These test the refusals and the committed policies, not the happy path:
# provisioning needs real Wasabi and a real cluster, but every guard below is a
# place where getting it wrong would hand objectgw (which serves the internet)
# the power to write or delete, or let a tenant-object credential reach the
# platform's own buckets on the shared account. Those are checkable offline.
set -euo pipefail
cd "$(dirname "$0")/.."

SCRIPT=scripts/object-storage-s3-credentials.sh
failures=0
pass() { echo "PASS  $1"; }
fail() { echo "FAIL  $1" >&2; failures=$((failures + 1)); }

# BEX_ENV_FILE=/dev/null: the script sources the repo's .env, and after a real
# provision that file carries every value these refusal tests assert is absent.
run() { env -i PATH="$PATH" HOME="$HOME" BEX_ENV_FILE=/dev/null "$@" 2>&1 || true; }

KEYS=(BEX_OBJECT_STORAGE_ACCESS_KEY BEX_OBJECT_STORAGE_SECRET_KEY
  BEX_OBJECT_STORAGE_READ_ACCESS_KEY BEX_OBJECT_STORAGE_READ_SECRET_KEY)

# --- usage ---
out="$(run bash "$SCRIPT")"
grep -q 'usage:' <<<"$out" && pass "no subcommand prints usage" || fail "no subcommand should print usage"
out="$(run bash "$SCRIPT" bogus)"
grep -q 'usage:' <<<"$out" && pass "unknown subcommand refused" || fail "unknown subcommand should be refused"

# --- the aws CLI image must be digest-pinned ---
out="$(run env AWS_CLI_IMAGE=amazon/aws-cli:latest bash "$SCRIPT" install)"
grep -q 'pinned by sha256 digest' <<<"$out" \
  && pass "refuses a floating aws-cli image" \
  || fail "must refuse an unpinned AWS_CLI_IMAGE"

# --- tenant objects never share a bucket with the Terraform state ---
out="$(run env TF_STATE_BUCKET=bex-objects-oregon bash "$SCRIPT" install)"
grep -q 'must not be bex-objects-oregon' <<<"$out" \
  && pass "refuses an object bucket as the state bucket" \
  || fail "must refuse a state bucket in the object namespace"

# --- provision needs the root credential, and there is no new root variable ---
out="$(run bash "$SCRIPT" provision)"
grep -q 'TF_STATE_ACCESS_KEY) is missing or empty' <<<"$out" \
  && pass "provision names the existing root key variable" \
  || fail "provision should ask for TF_STATE_ACCESS_KEY, not a new variable"
if grep -q 'BEX_OBJECT_STORAGE_ROOT' "$SCRIPT"; then
  fail "the script invents a root variable; ADR112 D9 reuses TF_STATE_* and AWS_*"
else
  pass "no object-storage root variable"
fi

# --- verify and install need every one of the four credentials ---
for sub in verify install; do
  for missing in "${KEYS[@]}"; do
    args=(env)
    for v in "${KEYS[@]}"; do
      [ "$v" = "$missing" ] || args+=("$v=set")
    done
    out="$(run "${args[@]}" bash "$SCRIPT" "$sub")"
    grep -q "$missing is missing or empty" <<<"$out" \
      && pass "$sub refuses without $missing" \
      || fail "$sub should name $missing when it is absent"
  done
done

# --- install also needs the gateway bearer, and puts it in both Secrets ---
out="$(run env "${KEYS[0]}=set" "${KEYS[1]}=set" "${KEYS[2]}=set" "${KEYS[3]}=set" bash "$SCRIPT" install)"
grep -q 'BEX_OBJECT_STORAGE_GATEWAY_TOKEN (run provision to mint it) is missing or empty' <<<"$out" \
  && pass "install refuses without the gateway bearer" \
  || fail "install should name BEX_OBJECT_STORAGE_GATEWAY_TOKEN when it is absent"
# bex-api verifies what objectgw sends, so both Secrets carry the same value.
for secret_call in 'install_secret "$rw_secret_name"' 'install_secret "$ro_secret_name"'; do
  if grep -A2 -F "$secret_call" "$SCRIPT" | grep -q 'BEX_OBJECT_STORAGE_GATEWAY_TOKEN'; then
    pass "$secret_call carries the gateway bearer"
  else
    fail "$secret_call must carry BEX_OBJECT_STORAGE_GATEWAY_TOKEN: objectgw sends it and bex-api verifies it"
  fi
done
# The bearer is minted from the system's random source, once, and a value that
# exists is never replaced (a re-run must not break an installed pair).
if grep -q 'secrets.token_hex(32)' "$SCRIPT" && grep -q 'kept gateway token' "$SCRIPT"; then
  pass "provision mints a 64-hex-character bearer once"
else
  fail "provision must mint BEX_OBJECT_STORAGE_GATEWAY_TOKEN with secrets.token_hex(32) and keep an existing one"
fi

# --- the two IAM policies stay least-privilege ---
# objectgw serves the internet: a policy edit that lets it write or delete is
# exactly what the two identities exist to prevent, and no Go test would see it.
rw_policy=infra/wasabi/object-storage-s3-policy.json
ro_policy=infra/wasabi/object-storage-read-s3-policy.json
for verb in s3:PutObject s3:DeleteObject s3:AbortMultipartUpload s3:ListMultipartUploadParts s3:ListBucketMultipartUploads s3:PutBucketPolicy s3:DeleteBucket; do
  if jq -e --arg v "$verb" '[.Statement[].Action[]] | index($v)' "$ro_policy" >/dev/null 2>&1; then
    fail "read-only policy grants $verb; objectgw must never write or delete"
  else
    pass "read-only policy withholds $verb"
  fi
done
for verb in s3:GetObject s3:PutObject s3:DeleteObject s3:ListBucket s3:AbortMultipartUpload s3:ListMultipartUploadParts s3:ListBucketMultipartUploads; do
  if jq -e --arg v "$verb" '[.Statement[].Action[]] | index($v)' "$rw_policy" >/dev/null 2>&1; then
    pass "read-write policy grants $verb"
  else
    fail "read-write policy is missing $verb; bex-api could not serve objects"
  fi
done
for verb in s3:GetObject s3:ListBucket; do
  if jq -e --arg v "$verb" '[.Statement[].Action[]] | index($v)' "$ro_policy" >/dev/null 2>&1; then
    pass "read-only policy grants $verb"
  else
    fail "read-only policy is missing $verb; objectgw could not serve a public bucket"
  fi
done
if jq -e '[.Statement[].Action[]] | map(select(. == "s3:*" or . == "*")) | length == 0' "$rw_policy" "$ro_policy" >/dev/null; then
  pass "no policy grants a wildcard action"
else
  fail "a policy grants s3:* or *"
fi

# --- on a shared account, neither policy may name any bucket but the two ---
for policy in "$rw_policy" "$ro_policy"; do
  names="$(jq -r '[.Statement[].Resource] | flatten | map(sub("^arn:aws:s3:::"; "") | sub("/\\*$"; "")) | unique | sort | join(",")' "$policy")"
  if [ "$names" = "bex-objects-frankfurt,bex-objects-oregon" ]; then
    pass "$(basename "$policy") names exactly the two object buckets"
  else
    fail "$(basename "$policy") names '$names'; expected exactly bex-objects-frankfurt and bex-objects-oregon"
  fi
done

# --- the denial probes target the platform buckets' own endpoint ---
# Found on the first production verify: a HEAD of bex-tfstate sent to the
# us-west-1 endpoint answers 400 Bad Request, not 403, and proves nothing.
if grep -q 'rw_platform_s3 s3api head-bucket --bucket "\$TFSTATE_BUCKET"' "$SCRIPT" \
  && grep -q 'ro_platform_s3 s3api head-bucket --bucket "\$TFSTATE_BUCKET"' "$SCRIPT" \
  && ! grep -q 'rw_s3 "\$r" s3api head-bucket --bucket "\$TFSTATE_BUCKET"' "$SCRIPT"; then
  pass "the bex-tfstate denial probes use the platform endpoint, not the object bucket's region"
else
  fail "the bex-tfstate denial probes must use the platform endpoint (a cross-region HEAD answers 400, not 403)"
fi

# --- the script's constants match the policies ---
for spec in oregon:us-west-1 frankfurt:eu-central-2; do
  grep -q "REGIONS=.*$spec" "$SCRIPT" \
    && pass "script serves region $spec" \
    || fail "script does not list region $spec; objectstorage/regions.go and the policies do"
done

# --- the Secrets carry the environment variable names bex-api and objectgw read ---
for v in "${KEYS[@]}" BEX_OBJECT_STORAGE_GATEWAY_TOKEN; do
  grep -q "$v" "$SCRIPT" && pass "script handles $v" || fail "script never mentions $v"
done

if [ "$failures" -gt 0 ]; then
  echo "$failures object-storage credential guard check(s) failed" >&2
  exit 1
fi
echo "all object-storage credential guard checks passed"
