#!/usr/bin/env bash
# Tests for scripts/oidc-aws-verify.sh with kubectl, docker and curl stubbed:
# a healthy service passes, and each failure the script exists to catch (a
# writable token file, a claim off Render's template, an STS for another
# audience accepting the token, a token never refreshed) fails it.
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
script="$repo_root/scripts/oidc-aws-verify.sh"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

ws="tea-d98210cbbpdc73dcrkvg"
srv="srv-c0000000000000000000"
export TEST_DIR=$tmp OIDC_POLL_SECONDS=0 OIDC_REFRESH_TIMEOUT=5

# jwt <iat> [aud] prints an unsigned token with Render's claims.
jwt() {
  local payload
  payload=$(jq -cn --arg iss "$OIDC_ISSUER_BASE/$ws" --arg aud "${2:-sts.amazonaws.com}" --argjson iat "$1" \
    --arg sub "workspace:$ws:environment:default:service:$srv" '{iss: $iss, sub: $sub, aud: $aud, iat: $iat, exp: ($iat + 3600)}' |
    base64 | tr -d '\n=' | tr '/+' '_-')
  echo "eyJhbGciOiJFUzI1NiJ9.$payload.c2ln"
}
export -f jwt
export ws srv

mkdir -p "$tmp/bin"
cat >"$tmp/bin/kubectl" <<'EOF'
#!/usr/bin/env bash
case "$*" in
  "-n $ws get pods -l bex.co/app-id=$srv -o json")
    # The rollout's old instance still reads Running while it terminates.
    echo '{"items":[{"metadata":{"name":"web-old","deletionTimestamp":"2026-11-18T00:00:00Z"},"status":{"phase":"Running"}},{"metadata":{"name":"web-abc"},"status":{"phase":"Running"}}]}' ;;
  "-n $ws exec web-old "*) echo "exec into the terminating instance" >&2; exit 1 ;;
  *'printf %s "${AWS_ROLE_ARN:-}"') printf arn:aws:iam::123456789012:role/qa ;;
  *'printf %s "${AWS_WEB_IDENTITY_TOKEN_FILE:-}"') printf /var/lib/render/oidc/aws-token ;;
  *': >>"$AWS_WEB_IDENTITY_TOKEN_FILE"') [ "$TEST_MODE" = writable ] || { echo "Read-only file system" >&2; exit 1; } ;;
  *'cat "$AWS_WEB_IDENTITY_TOKEN_FILE"')
    n=$(($(cat "$TEST_DIR/reads" 2>/dev/null || echo 0) + 1))
    echo "$n" >"$TEST_DIR/reads"
    aud=sts.amazonaws.com
    [ "$TEST_MODE" != aud ] || aud=api.openai.com
    if [ "$n" -ge 3 ] && [ "$TEST_MODE" != stale ]; then jwt 2200 "$aud"; else jwt 1000 "$aud"; fi ;;
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
  port)
    n=$(wc -l <"$TEST_DIR/docker-runs" | tr -d ' ')
    echo "127.0.0.1:900$n" ;;
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
audience=$(cat "$TEST_DIR/audience-$n")
claimed=$(cut -d. -f2 <<<"$token" | tr '_-' '/+')
while [ $((${#claimed} % 4)) -ne 0 ]; do claimed="$claimed="; done
claimed=$(base64 -d <<<"$claimed" | jq -r .aud)
if [ "$audience" = "$claimed" ] || [ "$TEST_MODE" = cross ]; then
  echo "<Credentials><AccessKeyId>AKIA1</AccessKeyId><SecretAccessKey>s3cret</SecretAccessKey></Credentials><SubjectFromWebIdentityToken>workspace:x</SubjectFromWebIdentityToken>"
else
  echo "<Error><Code>InvalidParameterValue</Code><Message>Token audience mismatch</Message></Error>"
fi
EOF
chmod +x "$tmp/bin/"*

run() { # run <mode> [base]
  rm -f "$tmp/reads" "$tmp/docker-runs" "$tmp"/audience-*
  OIDC_ISSUER_BASE=${2:-https://oidc.bex.test} TEST_MODE=$1 PATH="$tmp/bin:$PATH" \
    bash "$script" "$ws" "$srv" >"$tmp/out" 2>&1
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

export OIDC_ISSUER_BASE=https://oidc.bex.test
check "a healthy service passes, across a refresh" run healthy
check "the refresh is reported with both iats" grep -q 'iat 1000 -> 2200' "$tmp/out"
check "credentials never reach the output" bash -c "! grep -q s3cret '$tmp/out'"
check "a production issuer needs no host mapping" bash -c "! grep -q add-host '$tmp/docker-runs'"
check "a writable token file fails" fails_with writable "can write its token file"
check "a token for another audience fails" fails_with aud "claims are not Render's"
check "an STS for another audience accepting the token fails" fails_with cross "another audience accepted"
check "a token that is never replaced fails" fails_with stale "still holds iat 1000"
devn() {
  OIDC_ISSUER_BASE=http://oidc.localhost:54020 run healthy http://oidc.localhost:54020 &&
    grep -q -- '--add-host oidc.localhost:host-gateway' "$tmp/docker-runs"
}
check "a dev-N issuer reaches bex-api through the docker host" devn
check "a malformed service id is a usage error" bash -c "! bash '$script' '$ws' web 2>/dev/null"

if [ "$failures" -gt 0 ]; then
  echo "oidc-aws-verify.test: $failures failure(s)" >&2
  exit 1
fi
echo "oidc-aws-verify.test: all checks passed"
