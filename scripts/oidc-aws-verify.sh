#!/usr/bin/env bash
# oidc-aws-verify.sh — prove a running service's Managed OIDC AWS token works
# with an STS-compatible relying party, across one refresh
# (docs/ADR116-managed-oidc.md D9, w2/m477 t005).
#
#   KUBECONFIG=<app cluster kubeconfig> bash scripts/oidc-aws-verify.sh <tea-…> <srv-…>
#
# The service must set AWS_ROLE_ARN in a Pro-or-higher workspace, be deployed
# since, and run an image with sh and cat (as an SSH session needs).
#
# 1. In a running instance: AWS_ROLE_ARN is set, AWS_WEB_IDENTITY_TOKEN_FILE
#    names /var/lib/render/oidc/aws-token, the file cannot be written, and the
#    token's claims are Render's (iss <issuer base>/<tea>, aud
#    sts.amazonaws.com, sub workspace:<tea>:environment:…:service:<srv>,
#    exp = iat + 3600).
# 2. Starts a MinIO STS (docker) that trusts <issuer base>/<tea> for the
#    audience sts.amazonaws.com, and exchanges the token with
#    AssumeRoleWithWebIdentity, the call the AWS SDKs make. A second MinIO
#    trusting the same issuer for another audience must refuse it. MinIO does
#    not check iss, so workspace isolation is oidc-issuer-verify.sh's check.
# 3. Waits until the file holds a token with a later iat (the minter's
#    refresh, about 20 minutes plus the kubelet's Secret sync) and exchanges
#    that one too.
#
# Exits 0 only when every check passes. Environment:
#   OIDC_ISSUER_BASE      the issuer origin (default https://oidc.bex.co; dev-N:
#                         http://oidc.localhost:<bex-api port>, reached from the
#                         container through the docker host)
#   APP_NAMESPACE         the service's namespace (default: the workspace id)
#   OIDC_STS_IMAGE        the MinIO image (scripts/lib/oidc-sts.sh)
#   OIDC_WAIT_REFRESH     1 waits for a refresh (default), 0 skips step 3
#   OIDC_REFRESH_TIMEOUT  seconds to wait for it (default 1800)
#   OIDC_POLL_SECONDS     how often to look (default 30)
set -euo pipefail
# shellcheck source=scripts/lib/oidc-sts.sh
. "$(dirname "${BASH_SOURCE[0]}")/lib/oidc-sts.sh"

usage() {
  echo "usage: KUBECONFIG=<kubeconfig> bash scripts/oidc-aws-verify.sh <tea-…> <srv-…>" >&2
  exit 2
}
[ "$#" -eq 2 ] || usage
workspace=$1
service=$2
case "$workspace" in tea-????????????????????) ;; *) usage ;; esac
case "$service" in srv-????????????????????) ;; *) usage ;; esac

base=${OIDC_ISSUER_BASE:-https://oidc.bex.co}
base=${base%/}
issuer="$base/$workspace"
namespace=${APP_NAMESPACE:-$workspace}
wait_refresh=${OIDC_WAIT_REFRESH:-1}
refresh_timeout=${OIDC_REFRESH_TIMEOUT:-1800}
poll=${OIDC_POLL_SECONDS:-30}
token_path=/var/lib/render/oidc/aws-token
audience=sts.amazonaws.com

failures=0
pass() { echo "PASS: $*"; }
fail() {
  echo "FAIL: $*" >&2
  failures=$((failures + 1))
}

tmp=$(mktemp -d)
oidc_sts_init "$base" "$tmp"
cleanup() {
  oidc_sts_cleanup
  rm -rf "$tmp"
}
trap cleanup EXIT

# 1. The instance.
pod=$(oidc_running_pod "$namespace" "$service")
[ -n "$pod" ] || {
  echo "FAIL: no running instance of $service in $namespace" >&2
  exit 1
}
# in_pod runs a shell snippet in the instance; its variables expand there.
in_pod() { kubectl -n "$namespace" exec "$pod" -c app -- sh -c "$1"; }
read_token() { in_pod 'cat "$AWS_WEB_IDENTITY_TOKEN_FILE"'; }

role=$(in_pod 'printf %s "${AWS_ROLE_ARN:-}"')
file=$(in_pod 'printf %s "${AWS_WEB_IDENTITY_TOKEN_FILE:-}"')
if [ -n "$role" ] && [ "$file" = "$token_path" ]; then
  pass "$pod sets AWS_ROLE_ARN and AWS_WEB_IDENTITY_TOKEN_FILE=$file"
else
  fail "$pod: AWS_ROLE_ARN='$role', AWS_WEB_IDENTITY_TOKEN_FILE='$file' (want $token_path)"
fi
if in_pod ': >>"$AWS_WEB_IDENTITY_TOKEN_FILE"' >/dev/null 2>&1; then
  fail "$pod can write its token file"
else
  pass "the token file is read-only"
fi
token=$(read_token)
first=$(oidc_claims "$token")
want_sub_prefix="workspace:$workspace:environment:"
if jq -e --arg iss "$issuer" --arg aud "$audience" --arg pre "$want_sub_prefix" --arg srv ":service:$service" \
  '.iss == $iss and .aud == $aud and (.sub | startswith($pre) and endswith($srv)) and .exp - .iat == 3600' \
  <<<"$first" >/dev/null 2>&1; then
  pass "the token is Render's: sub $(jq -r .sub <<<"$first"), iat $(jq -r .iat <<<"$first"), 60-minute life"
else
  fail "the token's claims are not Render's: $first"
fi

# 2. The relying party (scripts/lib/oidc-sts.sh).
oidc_start_sts "$issuer" "$audience"
port=$sts_port arn=$sts_arn
if oidc_exchange "$port" "$arn" "$token" >"$tmp/first.xml"; then
  pass "an STS trusting $issuer for $audience issued credentials for the token (subject $(grep -o '<SubjectFromWebIdentityToken>[^<]*' "$tmp/first.xml" | sed 's/.*>//'))"
else
  fail "the STS refused the token: $(head -c 400 "$tmp/first.xml")"
fi
oidc_start_sts "$issuer" api.openai.com
if oidc_exchange "$sts_port" "$sts_arn" "$token" >"$tmp/other.xml"; then
  fail "an STS trusting $issuer for another audience accepted the token"
else
  pass "an STS for another audience refuses it ($(grep -o '<Message>[^<]*' "$tmp/other.xml" | sed 's/.*>//' | head -1))"
fi

# 3. A refresh.
if [ "$wait_refresh" = 1 ]; then
  iat=$(jq -r .iat <<<"$first")
  deadline=$(($(date +%s) + refresh_timeout))
  refreshed=""
  while [ "$(date +%s)" -lt "$deadline" ]; do
    sleep "$poll"
    next=$(read_token 2>/dev/null || true)
    [ -n "$next" ] || continue
    next_iat=$(oidc_claims "$next" | jq -r '.iat // empty' 2>/dev/null || true)
    if [ -n "$next_iat" ] && [ "$next_iat" -gt "$iat" ]; then
      refreshed=$next
      break
    fi
  done
  if [ -z "$refreshed" ]; then
    fail "the token file still holds iat $iat after ${refresh_timeout}s"
  else
    pass "the token file was replaced: iat $iat -> $next_iat ($((next_iat - iat))s later)"
    if oidc_exchange "$port" "$arn" "$refreshed" >"$tmp/second.xml"; then
      pass "the STS issued credentials for the refreshed token"
    else
      fail "the STS refused the refreshed token: $(head -c 400 "$tmp/second.xml")"
    fi
  fi
fi

if [ "$failures" -gt 0 ]; then
  echo "oidc-aws-verify: $failures check(s) failed" >&2
  exit 1
fi
echo "oidc-aws-verify: every check passed for $service at $issuer"
