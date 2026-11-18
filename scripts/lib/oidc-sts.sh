#!/usr/bin/env bash
# oidc-sts.sh — the STS-compatible relying party the Managed OIDC checks
# exchange a service's tokens with (docs/ADR116-managed-oidc.md D9/D10):
# scripts/oidc-aws-verify.sh (w2/m477) and scripts/oidc-audience-verify.sh
# (w2/m478). A MinIO in docker trusts one workspace's issuer for one audience
# and answers AssumeRoleWithWebIdentity, the call the AWS SDKs make; MinIO
# refuses a token whose aud is not that audience. It does not check iss, so
# workspace isolation stays scripts/oidc-issuer-verify.sh's check.
#
#   . scripts/lib/oidc-sts.sh
#   oidc_sts_init <issuer base> <scratch dir>; trap oidc_sts_cleanup EXIT
#   pod=$(oidc_running_pod <namespace> <srv-…>)
#   oidc_start_sts <issuer> <audience>      # sets sts_port and sts_arn
#   oidc_exchange <port> <role arn> <token> # prints the redacted response
#
# OIDC_STS_IMAGE overrides the MinIO image (default: the one the local
# cluster pins, scripts/mock-cluster.sh).

oidc_sts_image=${OIDC_STS_IMAGE:-docker.io/bitnamilegacy/minio:2025.7.23-debian-12-r5@sha256:6dabb4a2088c9a79908de3bc05f4586c23ad2182c8908e7e3acbf61c1467fb20}
oidc_sts_containers=()
oidc_sts_args=()
oidc_sts_tmp=""

# oidc_sts_init <issuer base> <scratch dir> prepares the relying party for
# an issuer at base. A dev-N issuer is bex-api on this machine: the container
# reaches it through the docker host under the issuer's own name.
oidc_sts_init() {
  local host=${1#*://}
  host=${host%%[:/]*}
  oidc_sts_tmp=$2
  oidc_sts_containers=()
  oidc_sts_args=()
  case "$host" in localhost | *.localhost) oidc_sts_args+=(--add-host "$host:host-gateway") ;; esac
}

# oidc_sts_cleanup removes every relying party this shell started.
oidc_sts_cleanup() {
  local c
  for c in "${oidc_sts_containers[@]+"${oidc_sts_containers[@]}"}"; do
    docker rm -f "$c" >/dev/null 2>&1 || true
  done
}

# oidc_running_pod <namespace> <service> prints a running instance of the
# service that is not on its way out: a rollout's old pod still reads Running
# while it terminates, with the previous release's mounts. Empty for none.
oidc_running_pod() {
  kubectl -n "$1" get pods -l "bex.co/app-id=$2" -o json 2>/dev/null |
    jq -r '[.items[] | select(.status.phase == "Running" and .metadata.deletionTimestamp == null)][0].metadata.name // empty' || true
}

# oidc_claims <token> prints the token's payload as JSON (no signature check:
# the relying party is the verifier).
oidc_claims() {
  local payload
  payload=$(cut -d. -f2 <<<"$1" | tr '_-' '/+')
  while [ $((${#payload} % 4)) -ne 0 ]; do payload="$payload="; done
  base64 -d <<<"$payload" 2>/dev/null
}

# oidc_start_sts <issuer> <audience> starts a MinIO trusting issuer for
# audience and sets sts_port and sts_arn. It must run in the caller's shell,
# not a subshell, so oidc_sts_cleanup removes it.
oidc_start_sts() {
  local name="bex-oidc-sts-$$-${#oidc_sts_containers[@]}" port arn=""
  oidc_sts_containers+=("$name")
  docker run -d --name "$name" "${oidc_sts_args[@]+"${oidc_sts_args[@]}"}" -p 127.0.0.1::9000 --entrypoint minio \
    -e MINIO_ROOT_USER=oidcverify -e "MINIO_ROOT_PASSWORD=$(openssl rand -hex 16)" \
    -e "MINIO_IDENTITY_OPENID_CONFIG_URL=$1/.well-known/openid-configuration" \
    -e "MINIO_IDENTITY_OPENID_CLIENT_ID=$2" -e MINIO_IDENTITY_OPENID_ROLE_POLICY=readonly \
    "$oidc_sts_image" server /tmp/data >/dev/null
  for _ in $(seq 1 60); do
    arn=$(docker logs "$name" 2>&1 | grep -o 'arn:minio:iam:::role/[A-Za-z0-9_-]*' | head -1 || true)
    [ -z "$arn" ] || break
    sleep 1
  done
  port=$(docker port "$name" 9000 | head -1 | sed 's/.*://')
  [ -n "$arn" ] && [ -n "$port" ] || {
    echo "FAIL: the MinIO STS did not load $1: $(docker logs "$name" 2>&1 | tail -3)" >&2
    exit 1
  }
  # shellcheck disable=SC2034 # read by the caller
  sts_port=$port sts_arn=$arn
}

# oidc_exchange <port> <role arn> <token> prints the STS response body with
# its secrets redacted, and returns non-zero unless it carries credentials.
oidc_exchange() {
  printf %s "$3" >"$oidc_sts_tmp/token"
  curl -sS -X POST "http://127.0.0.1:$1/" -d Action=AssumeRoleWithWebIdentity -d Version=2011-06-15 \
    -d DurationSeconds=900 --data-urlencode "RoleArn=$2" --data-urlencode "WebIdentityToken@$oidc_sts_tmp/token" >"$oidc_sts_tmp/sts.xml" || true
  sed -E 's#<(SecretAccessKey|SessionToken)>[^<]*#<\1>REDACTED#g' "$oidc_sts_tmp/sts.xml"
  grep -q '<AccessKeyId>[A-Z0-9]' "$oidc_sts_tmp/sts.xml"
}
