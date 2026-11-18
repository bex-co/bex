#!/usr/bin/env bash
# Tests for scripts/oidc-audience-verify.sh with kubectl, docker and curl
# stubbed: a service with two or three providers passes, and each failure the
# script exists to catch (an STS taking another audience's token, a token
# with another provider's audience, a file variable without its trigger, a
# writable file, fewer than two providers) fails it. Its provider table must
# be the operator's (ManagedOIDCProviders).
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
script="$repo_root/scripts/oidc-audience-verify.sh"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

ws="tea-d98210cbbpdc73dcrkvg"
srv="srv-c0000000000000000000"
export TEST_DIR=$tmp

# jwt <aud> [environment] prints an unsigned token with Render's claims.
jwt() {
  local payload
  payload=$(jq -cn --arg iss "$OIDC_ISSUER_BASE/$ws" --arg aud "$1" \
    --arg sub "workspace:$ws:environment:${2:-default}:service:$srv" '{iss: $iss, sub: $sub, aud: $aud, iat: 1000, exp: 4600}' |
    base64 | tr -d '\n=' | tr '/+' '_-')
  echo "eyJhbGciOiJFUzI1NiJ9.$payload.c2ln"
}
export -f jwt
export ws srv

mkdir -p "$tmp/bin"
cat >"$tmp/bin/kubectl" <<'EOF'
#!/usr/bin/env bash
# The triggers set: all three, or the mode's subset.
set_triggers="AWS_ROLE_ARN ANTHROPIC_FEDERATION_RULE_ID OPENAI_IDENTITY_PROVIDER_ID"
case "$TEST_MODE" in two | stray) set_triggers="AWS_ROLE_ARN ANTHROPIC_FEDERATION_RULE_ID" ;; one) set_triggers="AWS_ROLE_ARN" ;; esac
file_of() {
  case "$1" in
    AWS_WEB_IDENTITY_TOKEN_FILE) echo "AWS_ROLE_ARN aws-token sts.amazonaws.com" ;;
    ANTHROPIC_IDENTITY_TOKEN_FILE) echo "ANTHROPIC_FEDERATION_RULE_ID anthropic-token api.anthropic.com" ;;
    OPENAI_IDENTITY_TOKEN_FILE) echo "OPENAI_IDENTITY_PROVIDER_ID openai-token api.openai.com" ;;
  esac
}
case "$*" in
  "-n $ws get pods -l bex.co/app-id=$srv -o json")
    echo '{"items":[{"metadata":{"name":"web-old","deletionTimestamp":"2026-11-18T00:00:00Z"},"status":{"phase":"Running"}},{"metadata":{"name":"web-abc"},"status":{"phase":"Running"}}]}' ;;
  "-n $ws exec web-old "*) echo "exec into the terminating instance" >&2; exit 1 ;;
  "-n $ws exec web-abc -c app -- sh -c "*)
    all="$*"
    snippet=${all#*sh -c }
    var=$(grep -o '[A-Z_]*_\(ARN\|ID\|FILE\)' <<<"$snippet" | head -1)
    case "$snippet" in
      'printf %s "${'*'_FILE:-}"')
        read -r trigger file _ <<<"$(file_of "$var")"
        if [[ " $set_triggers " == *" $trigger "* ]] || { [ "$TEST_MODE" = stray ] && [ "$var" = OPENAI_IDENTITY_TOKEN_FILE ]; }; then
          printf %s "/var/lib/render/oidc/$file"
        fi ;;
      'printf %s "${'*)
        [[ " $set_triggers " != *" $var "* ]] || printf %s value ;;
      ': >>'*) [ "$TEST_MODE" = writable ] || { echo "Read-only file system" >&2; exit 1; } ;;
      'cat '*)
        read -r _ _ aud <<<"$(file_of "$var")"
        [ "$TEST_MODE" != sameaud ] || [ "$var" != ANTHROPIC_IDENTITY_TOKEN_FILE ] || aud=sts.amazonaws.com
        # A token minted before the service moved environment.
        if [ "$TEST_MODE" = othersub ] && [ "$var" = OPENAI_IDENTITY_TOKEN_FILE ]; then
          jwt "$aud" evm-c0000000000000000000
        else
          jwt "$aud"
        fi ;;
      *) echo "unexpected snippet $snippet" >&2; exit 1 ;;
    esac ;;
  *) echo "unexpected kubectl $*" >&2; exit 1 ;;
esac
EOF
cat >"$tmp/bin/docker" <<'EOF'
#!/usr/bin/env bash
case "$1" in
  run)
    echo "$*" >>"$TEST_DIR/docker-runs"
    for a in "$@"; do
      case "$a" in MINIO_IDENTITY_OPENID_CLIENT_ID=*) echo "${a#*=}" >"$TEST_DIR/audience-$(wc -l <"$TEST_DIR/docker-runs" | tr -d ' ')" ;; esac
    done
    echo container ;;
  logs) echo "INFO: IAM Roles: arn:minio:iam:::role/abc" ;;
  port) echo "127.0.0.1:900$(wc -l <"$TEST_DIR/docker-runs" | tr -d ' ')" ;;
  rm) ;;
  *) echo "unexpected docker $*" >&2; exit 1 ;;
esac
EOF
cat >"$tmp/bin/curl" <<'EOF'
#!/usr/bin/env bash
url="" token=""
while [ "$#" -gt 0 ]; do
  case "$1" in
    --data-urlencode) case "$2" in WebIdentityToken@*) token=$(cat "${2#WebIdentityToken@}") ;; esac; shift 2 ;;
    -d | -X) shift 2 ;;
    -sS) shift ;;
    *) url=$1; shift ;;
  esac
done
n=${url#http://127.0.0.1:900}; n=${n%/}
echo "$n" >>"$TEST_DIR/exchanges"
audience=$(cat "$TEST_DIR/audience-$n")
claimed=$(cut -d. -f2 <<<"$token" | tr '_-' '/+')
while [ $((${#claimed} % 4)) -ne 0 ]; do claimed="$claimed="; done
claimed=$(base64 -d <<<"$claimed" | jq -r .aud)
if [ "$audience" = "$claimed" ] || [ "$TEST_MODE" = cross ]; then
  echo "<Credentials><AccessKeyId>AKIA1</AccessKeyId><SecretAccessKey>s3cret</SecretAccessKey></Credentials>"
else
  echo "<Error><Code>InvalidParameterValue</Code><Message>Token audience mismatch</Message></Error>"
fi
EOF
chmod +x "$tmp/bin/"*

run() { # run <mode>
  rm -f "$tmp/docker-runs" "$tmp"/audience-* "$tmp/exchanges"
  TEST_MODE=$1 PATH="$tmp/bin:$PATH" bash "$script" "$ws" "$srv" >"$tmp/out" 2>&1
}

failures=0
check() { # check <name> <command…>
  local name=$1
  shift
  if "$@"; then
    echo "ok   $name"
  else
    echo "FAIL $name"
    sed 's/^/     /' "$tmp/out"
    failures=$((failures + 1))
  fi
}
fails_with() { ! run "$1" && grep -q "$2" "$tmp/out"; }
lines() { [ "$(wc -l <"$1" | tr -d ' ')" = "$2" ]; }
nine_exchanges() { lines "$tmp/docker-runs" 3 && lines "$tmp/exchanges" 9; }
six_refusals() { [ "$(grep -c '^PASS: an STS for .* refuses' "$tmp/out")" = 6 ]; }
two_pass() { run two && grep -q 'OPENAI_IDENTITY_PROVIDER_ID is not set, and neither is OPENAI_IDENTITY_TOKEN_FILE' "$tmp/out"; }
no_secret() { ! grep -q s3cret "$tmp/out"; }

export OIDC_ISSUER_BASE=https://oidc.bex.test
check "three providers pass" run healthy
check "three relying parties, nine exchanges" nine_exchanges
check "each token is refused for the two other audiences" six_refusals
check "the Anthropic token is refused where AWS's audience is expected" grep -q 'STS for sts.amazonaws.com refuses the anthropic token' "$tmp/out"
check "the AWS token is refused where Anthropic's audience is expected" grep -q 'STS for api.anthropic.com refuses the aws token' "$tmp/out"
check "credentials never reach the output" no_secret
check "two providers pass, and the third is reported unset" two_pass
check "an STS accepting another audience's token fails" fails_with cross "accepted the"
check "a token with another provider's audience fails" fails_with sameaud "anthropic: the token's claims are not Render's for aud api.anthropic.com"
check "tokens for two environments fail" fails_with othersub "different subjects"
check "a file variable without its trigger fails" fails_with stray "OPENAI_IDENTITY_PROVIDER_ID is not set, but OPENAI_IDENTITY_TOKEN_FILE"
check "a writable token file fails" fails_with writable "can write its token file"
check "one provider is not enough" fails_with one "1 provider token(s) mounted"
check "a malformed workspace id is a usage error" bash -c "! bash '$script' tea-abc '$srv' 2>/dev/null"

# The script's provider table is the operator's: name, trigger, file
# variable, file and audience, in ManagedOIDCProviders order.
go_table=$(grep -o '{Name: "[^}]*}' "$repo_root/lego/types/v1alpha1/managed_oidc.go" |
  sed -E 's/\{Name: "([^"]*)", Trigger: "([^"]*)", FileEnv: "([^"]*)", File: "([^"]*)", Audience: "([^"]*)"\}/\1 \2 \3 \4 \5/')
script_table=$(sed -n '/^providers="/,/"$/p' "$script" | sed -e 's/^providers="//' -e 's/"$//')
same_table() { [ -n "$go_table" ] && [ "$go_table" = "$script_table" ]; }
check "the provider table matches lego/types/v1alpha1/managed_oidc.go" same_table

if [ "$failures" -gt 0 ]; then
  echo "oidc-audience-verify.test: $failures failure(s)" >&2
  exit 1
fi
echo "oidc-audience-verify.test: all checks passed"
