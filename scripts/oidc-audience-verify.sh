#!/usr/bin/env bash
# oidc-audience-verify.sh — prove each Managed OIDC token a running service
# holds is accepted only where its own provider's audience is expected
# (docs/ADR116-managed-oidc.md D10, w2/m478 t003).
#
#   KUBECONFIG=<app cluster kubeconfig> bash scripts/oidc-audience-verify.sh <tea-…> <srv-…>
#
# The service must set the triggers of at least two providers (AWS_ROLE_ARN,
# ANTHROPIC_FEDERATION_RULE_ID, OPENAI_IDENTITY_PROVIDER_ID) in a Pro-or-higher
# workspace, be deployed since, and run an image with sh and cat (as an SSH
# session needs).
#
# 1. In a running instance, for every provider: when its trigger is set, its
#    file variable names /var/lib/render/oidc/<file>, the file cannot be
#    written, and the token's claims are Render's for that provider's
#    audience (iss <issuer base>/<tea>, aud, sub
#    workspace:<tea>:environment:…:service:<srv>, exp = iat + 3600); when it
#    is not, the file variable is unset. Every token carries the same sub.
# 2. For each mounted provider, a MinIO STS trusting <issuer base>/<tea> for
#    that provider's audience (scripts/lib/oidc-sts.sh) issues credentials for
#    that provider's token and refuses every other provider's, so a token
#    fails closed wherever another audience is expected, each way round.
#
# Exits 0 only when every check passes. Environment:
#   OIDC_ISSUER_BASE  the issuer origin (default https://oidc.bex.co; dev-N:
#                     http://oidc.localhost:<bex-api port>, reached from the
#                     container through the docker host)
#   APP_NAMESPACE     the service's namespace (default: the workspace id)
#   OIDC_STS_IMAGE    the MinIO image (scripts/lib/oidc-sts.sh)
set -euo pipefail
# shellcheck source=scripts/lib/oidc-sts.sh
. "$(dirname "${BASH_SOURCE[0]}")/lib/oidc-sts.sh"

usage() {
  echo "usage: KUBECONFIG=<kubeconfig> bash scripts/oidc-audience-verify.sh <tea-…> <srv-…>" >&2
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

# One line per provider: name, trigger, file variable, file, audience. The
# same table as ManagedOIDCProviders in lego/types/v1alpha1/managed_oidc.go;
# scripts/oidc-audience-verify.test.sh fails when the two drift.
providers="aws AWS_ROLE_ARN AWS_WEB_IDENTITY_TOKEN_FILE aws-token sts.amazonaws.com
anthropic ANTHROPIC_FEDERATION_RULE_ID ANTHROPIC_IDENTITY_TOKEN_FILE anthropic-token api.anthropic.com
openai OPENAI_IDENTITY_PROVIDER_ID OPENAI_IDENTITY_TOKEN_FILE openai-token api.openai.com"

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
in_pod() { kubectl -n "$namespace" exec "$pod" -c app -- sh -c "$1" </dev/null; }

want_sub_prefix="workspace:$workspace:environment:"
: >"$tmp/mounted"
while read -r name trigger file_env file audience <&3; do
  set_to=$(in_pod "printf %s \"\${$trigger:-}\"")
  path=$(in_pod "printf %s \"\${$file_env:-}\"")
  if [ -z "${set_to//[[:space:]]/}" ]; then
    if [ -z "$path" ]; then
      pass "$name: $trigger is not set, and neither is $file_env"
    else
      fail "$name: $trigger is not set, but $file_env=$path"
    fi
    continue
  fi
  want="/var/lib/render/oidc/$file"
  if [ "$path" = "$want" ]; then
    pass "$name: $trigger is set and $file_env=$path"
  else
    fail "$name: $trigger is set, but $file_env='$path' (want $want)"
    continue
  fi
  if in_pod ": >>\"\$$file_env\"" >/dev/null 2>&1; then
    fail "$name: $pod can write its token file"
  else
    pass "$name: the token file is read-only"
  fi
  token=$(in_pod "cat \"\$$file_env\"")
  claims=$(oidc_claims "$token")
  if jq -e --arg iss "$issuer" --arg aud "$audience" --arg pre "$want_sub_prefix" --arg srv ":service:$service" \
    '.iss == $iss and .aud == $aud and (.sub | startswith($pre) and endswith($srv)) and .exp - .iat == 3600' \
    <<<"$claims" >/dev/null 2>&1; then
    pass "$name: the token is Render's for aud $audience"
  else
    fail "$name: the token's claims are not Render's for aud $audience: $claims"
    continue
  fi
  printf %s "$token" >"$tmp/token-$name"
  jq -r .sub <<<"$claims" >>"$tmp/subs"
  echo "$name $audience" >>"$tmp/mounted"
done 3<<<"$providers"

mounted=$(wc -l <"$tmp/mounted" | tr -d ' ')
if [ "$mounted" -lt 2 ]; then
  fail "$mounted provider token(s) mounted; the check needs at least two (set two providers' triggers and redeploy)"
elif [ "$(sort -u "$tmp/subs" | wc -l | tr -d ' ')" -ne 1 ]; then
  fail "the tokens carry different subjects: $(sort -u "$tmp/subs" | tr '\n' ' ')"
else
  pass "$mounted tokens, one per provider, all for $(head -1 "$tmp/subs")"
fi

# 2. Each audience's relying party takes its own provider's token only.
if [ "$mounted" -ge 2 ]; then
  while read -r rp rp_audience <&3; do
    oidc_start_sts "$issuer" "$rp_audience"
    while read -r name _ <&4; do
      if oidc_exchange "$sts_port" "$sts_arn" "$(cat "$tmp/token-$name")" >"$tmp/out.xml"; then
        if [ "$name" = "$rp" ]; then
          pass "an STS for $rp_audience issued credentials for the $name token"
        else
          fail "an STS for $rp_audience accepted the $name token"
        fi
      elif [ "$name" = "$rp" ]; then
        fail "an STS for $rp_audience refused the $name token: $(head -c 400 "$tmp/out.xml")"
      else
        pass "an STS for $rp_audience refuses the $name token ($(grep -o '<Message>[^<]*' "$tmp/out.xml" | sed 's/.*>//' | head -1))"
      fi
    done 4<"$tmp/mounted"
  done 3<"$tmp/mounted"
fi

if [ "$failures" -gt 0 ]; then
  echo "oidc-audience-verify: $failures check(s) failed" >&2
  exit 1
fi
echo "oidc-audience-verify: every token of $service is accepted only for its own audience at $issuer"
