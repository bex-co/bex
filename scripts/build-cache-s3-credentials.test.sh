#!/usr/bin/env bash
# Guards for scripts/build-cache-s3-credentials.sh (w9/m492, ADR060 D3 S3
# amendment, ADR019).
#
# These test the refusals and the committed policy, not the happy path:
# provisioning needs real Wasabi and a real cluster. Every guard below is a
# place where getting it wrong would let the bucket-wide credential that sits
# beside tenant builds reach another bucket, or let caches outlive their expiry.
set -euo pipefail
cd "$(dirname "$0")/.."

SCRIPT=scripts/build-cache-s3-credentials.sh
POLICY=infra/wasabi/build-cache-s3-policy.json
LIFECYCLE=infra/wasabi/build-cache-lifecycle.json
failures=0
pass() { echo "PASS  $1"; }
fail() { echo "FAIL  $1" >&2; failures=$((failures + 1)); }

# BEX_ENV_FILE=/dev/null: the script sources the repo's .env, which carries the
# root credential these refusal tests assert is absent.
run() { env -i PATH="$PATH" HOME="$HOME" BEX_ENV_FILE=/dev/null "$@" 2>&1 || true; }

out="$(run bash "$SCRIPT")"
grep -q 'usage:' <<<"$out" && pass "no subcommand prints usage" || fail "no subcommand should print usage"
out="$(run bash "$SCRIPT" install)"
grep -q 'usage:' <<<"$out" && pass "unknown subcommand refused" || fail "unknown subcommand should be refused"

out="$(run env AWS_CLI_IMAGE=amazon/aws-cli:latest bash "$SCRIPT" verify)"
grep -q 'pinned by sha256 digest' <<<"$out" \
  && pass "refuses a floating aws-cli image" || fail "must refuse an unpinned AWS_CLI_IMAGE"

out="$(run env BEX_BUILD_CACHE_S3_BUCKET=bex-tfstate bash "$SCRIPT" provision)"
grep -q 'pinned to bex-build-cache' <<<"$out" \
  && pass "refuses a bucket the committed policy does not name" || fail "must refuse a bucket other than bex-build-cache"
for other in TF_STATE_BUCKET BEX_STATIC_S3_BUCKET; do
  out="$(run env "$other=bex-build-cache" bash "$SCRIPT" provision)"
  grep -q 'must not be bex-build-cache' <<<"$out" \
    && pass "refuses sharing the cache bucket with $other" || fail "must refuse $other=bex-build-cache"
done

out="$(run env TF_STATE_ENDPOINT=https://s3.example TF_STATE_REGION=eu-central-2 bash "$SCRIPT" provision)"
grep -q 'TF_STATE_ACCESS_KEY) is missing or empty' <<<"$out" \
  && pass "provision names the existing root key variable" || fail "provision should ask for TF_STATE_ACCESS_KEY"
if grep -q 'BEX_BUILD_CACHE_ROOT' "$SCRIPT"; then
  fail "the script invents a root variable; it reuses TF_STATE_* and AWS_*"
else
  pass "no build-cache root variable"
fi

resources="$(jq -r '[.Statement[].Resource] | flatten | sort | join(",")' "$POLICY")"
[ "$resources" = "arn:aws:s3:::bex-build-cache,arn:aws:s3:::bex-build-cache/*" ] \
  && pass "policy is confined to the cache bucket" || fail "policy names more than the cache bucket: $resources"
actions="$(jq -r '[.Statement[] | .Action[]? // .Action] | flatten | sort | join(",")' "$POLICY")"
want='s3:AbortMultipartUpload,s3:DeleteObject,s3:GetBucketLocation,s3:GetObject,s3:ListBucket,s3:ListBucketMultipartUploads,s3:ListMultipartUploadParts,s3:PutObject'
[ "$actions" = "$want" ] && pass "policy grants object read/write/delete and listing only" \
  || fail "policy actions drifted: $actions"
if jq -e '[.Statement[] | select(.Effect != "Allow" or (.Condition != null))] | length == 0' "$POLICY" >/dev/null; then
  pass "policy has no conditional or deny statements to reason about"
else
  fail "policy carries a Deny or Condition statement; review the boundary before changing the test"
fi
# The credential must not be able to lift its own expiry.
if grep -Eq 'PutLifecycle|DeleteLifecycle|PutBucketPolicy|s3:\*' "$POLICY"; then
  fail "policy lets the credential change the bucket's rules"
else
  pass "policy cannot change the bucket's lifecycle or policy"
fi

[ "$(jq -r '.Rules | length' "$LIFECYCLE")" = 1 ] \
  && [ "$(jq -r '.Rules[0].Status' "$LIFECYCLE")" = Enabled ] \
  && [ "$(jq -r '.Rules[0].Filter.Prefix' "$LIFECYCLE")" = "" ] \
  && [ "$(jq -r '.Rules[0].Expiration.Days' "$LIFECYCLE")" = 30 ] \
  && pass "one enabled whole-bucket 30-day expiry rule" || fail "the lifecycle is not one whole-bucket 30-day expiry"
grep -q 'cache change the expiry rule' "$SCRIPT" \
  && pass "verify proves the credential cannot remove the expiry" || fail "verify lost its lifecycle denial probe"
for probe in 'cache list account buckets' 'cache list tfstate bucket' 'cache write tfstate bucket' \
  'cache list static bucket' 'cache write static bucket' 'cache list unrelated bucket'; do
  grep -qF "$probe" "$SCRIPT" && pass "verify keeps '$probe'" || fail "verify lost '$probe'"
done
grep -qF '"app.bex.co/protected-from-tenant-mount":"true"' "$SCRIPT" \
  && pass "the Secret carries the protected-from-tenant-mount label" || fail "the Secret lost its tenant-mount protection label"

if [ "$failures" -ne 0 ]; then
  echo "$failures check(s) failed" >&2
  exit 1
fi
echo "build-cache-s3-credentials guards: all passed"
