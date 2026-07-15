#!/usr/bin/env bash
# w9/m4/t005 — mutation-prove the cli-compat verify legs fail loudly.
#
# The verify script (.pm/w9/done/m2/verify.sh) IS this milestone's test surface,
# so its own failure modes are what need proving: a leg that cannot fail is
# worthless (the RC14 lesson). This harness stands a response-mutating proxy
# (mutation-proxy.py, beside this script) in front of a live bex-api, reintroduces one fixed wire-
# shape regression per family, runs that family's real verify leg against it,
# and asserts the leg FAILS — either the official CLI's own decode errors
# nonzero, or verify's whole-shape `checkFields` finds the dropped field.
#
# Run it exactly like the verifier, from repo root with dev-9 up:
#   scripts/cli-compat.sh mutation-check      # (wrapper wires host/key), or
#   BEX_API_URL=… HYDRA_PUBLIC_URL=… bash .pm/w9/done/m4/mutation-check.sh
set -uo pipefail
HERE="$(cd "$(dirname "$0")" && pwd)"       # this script's dir (proxy lives beside it)
cd "$(git -C "$HERE" rev-parse --show-toplevel)" # repo root, independent of this file's depth

BEX_API_URL="${BEX_API_URL:-http://localhost:54090}"
HYDRA_PUBLIC_URL="${HYDRA_PUBLIC_URL:-http://localhost:59090}"
CLI_KEY_ENV="${CLI_KEY_ENV:-.pm/w9/dev-9/.cli-key.env}"
RENDER_BIN="${RENDER_BIN:-.pm/w9/dev-9/bin/render}"
# shellcheck disable=SC1090
source "$CLI_KEY_ENV"

# When invoked through scripts/cli-compat.sh, RENDER_API_KEY is already a fresh
# bearer (the wrapper did the Hydra exchange); reuse it. Only exchange when run
# standalone.
TOKEN="${RENDER_API_KEY:-}"
if [ -z "$TOKEN" ]; then
  TOKEN=$(curl -sf -X POST "$HYDRA_PUBLIC_URL/oauth2/token" \
    -d "grant_type=client_credentials&client_id=$CLI_COMPAT_KEY_ID&client_secret=$CLI_COMPAT_KEY_SECRET" \
    | python3 -c 'import json,sys; print(json.load(sys.stdin)["access_token"])')
  [ -n "$TOKEN" ] || { echo "error: token exchange failed" >&2; exit 1; }
fi

export RENDER_API_KEY="$TOKEN"
export RENDER_WORKSPACE="${CLI_COMPAT_TENANT_ID:-}"
export RENDER_CLI_CONFIG_PATH="$(mktemp -d)/cli.yaml"
REAL="$BEX_API_URL/v1"
fail=0
SVC="mut-svc-$$"; PG="mut-pg-$$"; KV="mut-kv-$$"; PROJ=""; ENVID=""; PROXY_PID=""

api() { curl -sS -X "$1" -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' ${3:+-d "$3"} "$REAL$2"; }
jf() { python3 -c "import json,sys
d=json.load(sys.stdin); d=d.get('data',d) if isinstance(d,dict) else d
print(d$1)"; }

cleanup() {
  [ -n "$PROXY_PID" ] && kill "$PROXY_PID" 2>/dev/null
  RENDER_HOST="$REAL/" "$RENDER_BIN" services delete "$SVC" --confirm -o json >/dev/null 2>&1
  RENDER_HOST="$REAL/" "$RENDER_BIN" postgres delete "$PG" --confirm -o json >/dev/null 2>&1
  RENDER_HOST="$REAL/" "$RENDER_BIN" keyvalues delete "$KV" --confirm -o json >/dev/null 2>&1
  [ -n "$ENVID" ] && api DELETE "/environments/$ENVID" >/dev/null 2>&1
  [ -n "$PROJ" ] && api DELETE "/projects/$PROJ" >/dev/null 2>&1
  return 0
}
trap cleanup EXIT

echo "==> seeding real resources"
svc_create=$(RENDER_HOST="$REAL/" "$RENDER_BIN" services create --name "$SVC" --type web_service \
  --image traefik/whoami:v1.10.3 --plan free --num-instances 1 --confirm -o json 2>/dev/null)
SVC_ID=$(jf '["id"]' <<<"$svc_create" 2>/dev/null || true)
DEPLOY_ID=$(RENDER_HOST="$REAL/" "$RENDER_BIN" deploys create "$SVC_ID" --confirm -o json 2>/dev/null | jf '["id"]' 2>/dev/null || true)
RENDER_HOST="$REAL/" "$RENDER_BIN" postgres create --name "$PG" --plan free --version 16 --confirm -o json >/dev/null 2>&1
RENDER_HOST="$REAL/" "$RENDER_BIN" keyvalues create --name "$KV" \
  --ip-allow-list "cidr=10.0.0.0/8,description=x" --confirm -o json >/dev/null 2>&1
PROJ=$(api POST /projects "{\"name\":\"mut-proj-$$\",\"ownerId\":\"$RENDER_WORKSPACE\"}" | jf '["id"]' 2>/dev/null || true)
ENVID=$(api POST /environments "{\"name\":\"staging\",\"projectId\":\"$PROJ\"}" | jf '["id"]' 2>/dev/null || true)

# proxy MUTATION — (re)start the mutating proxy in the named mode, export
# RENDER_HOST at it, echo nothing. Kills any previous proxy first.
start_proxy() {
  [ -n "$PROXY_PID" ] && kill "$PROXY_PID" 2>/dev/null
  local port
  exec 3< <(MUTATION="$1" UPSTREAM="$BEX_API_URL" PROXY_PORT=0 python3 "$HERE/mutation-proxy.py")
  PROXY_PID=$!
  read -r port <&3
  export RENDER_HOST="http://127.0.0.1:$port/v1/"
}

# expect_break DESC KIND CMD... — run the family's leg CMD through the proxy and
# assert it FAILS. KIND=decode ⇒ the official CLI must exit nonzero (it could not
# decode the regressed shape). KIND=fields ⇒ the CLI may exit 0, but verify's
# WANT_FIELDS whole-shape assertion (set by the caller) must find a field
# missing — the RC14-class bug the single-field spot check would have missed.
expect_break() {
  local desc="$1" kind="$2"; shift 2
  local got status missing="" f
  got="$("$@" 2>&1)"; status=$?
  if [ "$kind" = decode ]; then
    if [ "$status" != 0 ]; then
      echo "PROVEN: $desc — official CLI rejected the regressed shape (exit $status)"
    else
      echo "VACUOUS: $desc — leg passed against the broken shape!"; echo "  got: $got"; fail=1
    fi
  else
    for f in $WANT_FIELDS; do grep -qE "$f" <<<"$got" || missing="$missing $f"; done
    if [ -n "$missing" ]; then
      echo "PROVEN: $desc — whole-shape check caught dropped field(s):$missing"
    else
      echo "VACUOUS: $desc — every asserted field survived the break!"; echo "  got: $got"; fail=1
    fi
  fi
}

echo "==> mutation checks (each reintroduces one fixed RC; the leg must fail)"

start_proxy svc_autodeploy
expect_break "services (RC2 autoDeploy bool)" decode \
  "$RENDER_BIN" services -o json

start_proxy pg_flatten
expect_break "postgres get (RC3 cursor envelope unwrapped)" decode \
  "$RENDER_BIN" postgres get "$PG" -o json

if [ -n "$DEPLOY_ID" ]; then
  start_proxy deploy_image
  expect_break "deploys list (RC2 image object -> bare string)" decode \
    "$RENDER_BIN" deploys list "$SVC_ID" -o json
else
  echo "SKIP: deploys mutation — no seed deploy id"; fail=1
fi

start_proxy logs_blank
expect_break "logs (RC8 blank nextStartTime crash)" decode \
  "$RENDER_BIN" logs --resources "$SVC" --limit 5 -o json

start_proxy kv_flatten
WANT_FIELDS='"ownerId":[[:space:]]*"tea-[a-z0-9]+" "maxmemoryPolicy":[[:space:]]*"allkeys_lru" "persistenceMode":[[:space:]]*"journal_snapshot"'
expect_break "keyvalues get (RC14 nested owner/options flattened)" fields \
  "$RENDER_BIN" keyvalues get "$KV" -o json

start_proxy env_flatten
WANT_FIELDS="\"id\":[[:space:]]*\"$ENVID\" \"name\":[[:space:]]*\"staging\" \"projectId\":[[:space:]]*\"$PROJ\""
expect_break "environments (RC15 cursor envelope unwrapped)" fields \
  "$RENDER_BIN" environments "$PROJ" -o json

if [ "$fail" != 0 ]; then
  echo "mutation-check: a leg passed against a broken shape (vacuous guard)" >&2
  exit 1
fi
echo "mutation-check: every family's leg failed loudly against its regressed shape"
