#!/usr/bin/env bash
# E2E for the tenant env-vars feature (docs/ADR013-secrets.md, w4/m6) against the current
# kubeconfig cluster's OpenBao (m5) + Ory substrate. Proves the whole path:
#
#   1. POSITIVE: an authenticated caller PUTs env-vars on a service; the value
#      lands in OpenBao under tenants/default/services/<svc>/env (confirmed with a
#      scoped OpenBao read, not just the k8s Secret), the operator materializes it
#      into the <svc>-env Secret and rolls the pods, and the running app serves the
#      new value over HTTP (traefik/whoami echoes WHOAMI_NAME as its `Name:` line).
#   2. FORBIDDEN: with authorization on (BEX_OPENFGA_URL), a freshly minted key
#      with no tuples gets 403 on the env-vars verbs — secrets are gated by the
#      same OpenFGA checker as everything else.
#   3. UNAVAILABLE: with BEX_OPENBAO_URL unset, the env-vars verbs 503 while the
#      rest of the API (GET /v1/services) is byte-for-byte unchanged.
#
# Like auth-e2e.sh, bex-api AND the operator run on the host (go build) talking to
# the cluster apiserver via $KUBECONFIG; OpenBao/Hydra/OpenFGA are reached through
# kubectl port-forwards — so this runs on the CAPD mock cluster with no operator
# image. OpenBao login uses a token minted for the bex-api ServiceAccount
# (BEX_OPENBAO_JWT_PATH), the same identity the in-cluster pod would present.
#
# Usage: scripts/secrets-verify.sh    # respects $KUBECONFIG; exits 0 on pass
# Requires: kubectl, curl, yq v4, go; a cluster with m5 (OpenBao) + auth (Hydra,
# OpenFGA) running (scripts/mock-cluster.sh + the GitOps sync, or the prod deploy).
set -euo pipefail
cd "$(dirname "$0")/.."

AUTH_NS=auth
SECRETS_NS=secrets
SYS_NS=bex-system
APP_NS=default
SVC=whoami-secrets # a dedicated App so a rerun never collides with examples/whoami

HYDRA_PUBLIC=127.0.0.1:24444
HYDRA_ADMIN=127.0.0.1:24445
OPENFGA=127.0.0.1:24446
BAO=127.0.0.1:24200
API=127.0.0.1:18090
PIDS=()
API_PID=""
OP_PID=""

cleanup() {
  [ -n "$OP_PID" ] && { kill "$OP_PID" 2>/dev/null || true; }
  [ -n "$API_PID" ] && { kill "$API_PID" 2>/dev/null || true; }
  if [ "${#PIDS[@]}" -gt 0 ]; then
    kill "${PIDS[@]}" 2>/dev/null || true
    wait "${PIDS[@]}" 2>/dev/null || true
  fi
  kubectl -n "$APP_NS" delete app.app.bex.co "$SVC" --ignore-not-found >/dev/null 2>&1 || true
}
trap cleanup EXIT

fail() { echo "FAIL: $*" >&2; exit 1; }

wait_http() { # url — wait until it answers at all (any HTTP status)
  for _ in $(seq 1 30); do
    code="$(curl -s -o /dev/null -w '%{http_code}' "$1" || true)"
    [ "$code" != "000" ] && return 0
    sleep 1
  done
  fail "$1 did not become reachable"
}

# --- config from .env (never printed): OpenBao root token, OpenFGA key, bootstrap
if [ -f .env ]; then
  set -a
  # shellcheck disable=SC1091
  source ./.env
  set +a
fi
[ -n "${BAO_ROOT_TOKEN:-}" ] || fail "BAO_ROOT_TOKEN missing (.env) — run scripts/bao-init.sh first"

echo "==> ensuring the App CRD is installed"
make -C lego/operator install >/dev/null

echo "==> port-forwards (hydra, openfga, openbao) + building bex-api and the operator"
kubectl -n "$AUTH_NS" port-forward service/hydra-public 24444:4444 >/dev/null 2>&1 & PIDS+=($!)
kubectl -n "$AUTH_NS" port-forward service/hydra-admin 24445:4445 >/dev/null 2>&1 & PIDS+=($!)
kubectl -n "$AUTH_NS" port-forward service/openfga 24446:8080 >/dev/null 2>&1 & PIDS+=($!)
kubectl -n "$SECRETS_NS" port-forward service/openbao 24200:8200 >/dev/null 2>&1 & PIDS+=($!)
bindir="$(mktemp -d)"
(cd lego/backend && go build -o "$bindir/bex-api" ./cmd/api)
(cd lego/operator && go build -o "$bindir/manager" ./cmd/manager)
wait_http "http://$HYDRA_ADMIN/health/ready"
wait_http "http://$OPENFGA/healthz"
wait_http "http://$BAO/v1/sys/seal-status"

echo "==> scoping the bex-api ServiceAccount in OpenBao (idempotent)"
kubectl create namespace "$SYS_NS" >/dev/null 2>&1 || true
kubectl -n "$SYS_NS" create serviceaccount bex-api >/dev/null 2>&1 || true
BAO_ADDR="http://$BAO" bash scripts/bao-k8s-auth.sh >/dev/null
# Mint an SA token for OpenBao's Kubernetes auth (what the in-cluster pod projects).
jwt_file="$bindir/sa-token"
kubectl -n "$SYS_NS" create token bex-api --duration=1h > "$jwt_file"

echo "==> seeding the bootstrap OAuth2 client + OpenFGA model"
export BEX_BOOTSTRAP_CLIENT_SECRET="${BEX_BOOTSTRAP_CLIENT_SECRET:-verify-bootstrap-$(date +%s)}"
HYDRA_ADMIN_URL="http://$HYDRA_ADMIN" bash scripts/auth-bootstrap-client.sh >/dev/null
OPENFGA_URL="http://$OPENFGA" bash scripts/authz-model.sh >/dev/null

token_for() { # CLIENT_ID CLIENT_SECRET -> access token ("" on failure)
  { curl -s -X POST "http://$HYDRA_PUBLIC/oauth2/token" \
    -d "grant_type=client_credentials&client_id=$1&client_secret=$2" || true; } | yq '.access_token // ""' -
}
boot_token="$(token_for bex-bootstrap "$BEX_BOOTSTRAP_CLIENT_SECRET")"
[ -n "$boot_token" ] || fail "no access_token for the bootstrap client"

# --- run the operator (materializes Apps -> Deployments) and bex-api on the host.
start_operator() {
  BEX_RUNTIME=kubernetes "$bindir/manager" >/dev/null 2>&1 &
  OP_PID=$!
}
start_api() { # extra env...
  env "$@" "BEX_HYDRA_ADMIN_URL=http://$HYDRA_ADMIN" BEX_API_ADDR=":18090" BEX_API_NAMESPACE="$APP_NS" \
    "$bindir/bex-api" >/dev/null 2>&1 &
  API_PID=$!
  wait_http "http://$API/healthz"
}
stop_api() { kill "$API_PID" 2>/dev/null || true; wait "$API_PID" 2>/dev/null || true; API_PID=""; }

echo "==> starting the operator and bex-api (OpenBao wired)"
start_operator
start_api "BEX_OPENBAO_URL=http://$BAO" "BEX_OPENBAO_JWT_PATH=$jwt_file"

# request METHOD PATH TOKEN BODY — sets LAST_CODE + LAST_BODY.
request() {
  local args=(-s -w '\n%{http_code}' -X "$1" "http://$API$2")
  [ -n "$3" ] && args+=(-H "Authorization: Bearer $3")
  [ -n "$4" ] && args+=(-H 'Content-Type: application/json' -d "$4")
  local out; out="$(curl "${args[@]}")"
  LAST_CODE="${out##*$'\n'}"; LAST_BODY="${out%$'\n'*}"
}
assert_code() { [ "$LAST_CODE" = "$1" ] || fail "$2: got $LAST_CODE, want $1 (body: $LAST_BODY)"; echo "    ok: $2 ($LAST_CODE)"; }

echo "==> deploying the $SVC App (traefik/whoami echoes WHOAMI_NAME)"
kubectl apply -f - >/dev/null <<YAML
apiVersion: app.bex.co/v1alpha1
kind: App
metadata: { name: $SVC, namespace: $APP_NS }
spec: { image: traefik/whoami, port: 80, replicas: 1 }
YAML
kubectl -n "$APP_NS" wait --for=condition=Available --timeout=120s deployment/"$SVC" \
  || fail "$SVC Deployment never became Available (operator/CAPD scheduling)"

# --- FLOW 1: positive -----------------------------------------------------------
echo "==> [1/3] positive: PUT env-vars, confirm in OpenBao, confirm served"
WANT="verified-$(date +%s)"
request PUT "/v1/services/$SVC/env-vars" "$boot_token" "[{\"key\":\"WHOAMI_NAME\",\"value\":\"$WANT\"}]"
assert_code 200 "PUT env-vars"

# Present in OpenBao itself (not only the derived k8s Secret): read with a scoped
# bex-api SA login, exactly the path bex-api wrote.
scoped="$(curl -sf -X POST "http://$BAO/v1/auth/kubernetes/login" \
  -d "{\"role\":\"bex-api\",\"jwt\":\"$(cat "$jwt_file")\"}" | yq '.auth.client_token' -)"
[ -n "$scoped" ] && [ "$scoped" != "null" ] || fail "bex-api SA could not log into OpenBao"
bao_val="$(curl -sf -H "X-Vault-Token: $scoped" \
  "http://$BAO/v1/tenants/data/$APP_NS/services/$SVC/env" | yq '.data.data.WHOAMI_NAME' -)"
[ "$bao_val" = "$WANT" ] || fail "OpenBao does not hold the value: got '$bao_val', want '$WANT'"
echo "    ok: value present in OpenBao (tenants/$APP_NS/services/$SVC/env)"

# The materialized Secret exists and the App consumes it via envFrom.
kubectl -n "$APP_NS" get secret "$SVC-env" >/dev/null 2>&1 || fail "$SVC-env Secret was not materialized"
echo "    ok: $SVC-env Secret materialized, spec.envFromSecret wired"

# Pods roll; the running app serves the new WHOAMI_NAME. whoami is a scratch
# image (no shell/wget), so curl it over a port-forward to its Service from the
# host rather than exec-ing into the container.
kubectl -n "$APP_NS" rollout status deployment/"$SVC" --timeout=120s >/dev/null || fail "rollout after env change did not complete"
APP=127.0.0.1:18080
kubectl -n "$APP_NS" port-forward service/"$SVC" 18080:80 >/dev/null 2>&1 & app_pf=$!; PIDS+=("$app_pf")
wait_http "http://$APP/"
served=""
for _ in $(seq 1 20); do
  served="$(curl -s "http://$APP/" || true)"
  echo "$served" | grep -q "Name: $WANT" && break
  sleep 3
done
{ kill "$app_pf" && wait "$app_pf"; } 2>/dev/null || true
echo "$served" | grep -q "Name: $WANT" || fail "app did not serve the new value (got: $(echo "$served" | tr '\n' ' '))"
echo "    ok: running app serves 'Name: $WANT'"

# --- FLOW 2: forbidden (authorization on) --------------------------------------
echo "==> [2/3] forbidden: a tuple-less key gets 403 on the env-vars verbs"
stop_api
start_api "BEX_OPENBAO_URL=http://$BAO" "BEX_OPENBAO_JWT_PATH=$jwt_file" \
  "BEX_OPENFGA_URL=http://$OPENFGA" "BEX_OPENFGA_TOKEN=${OPENFGA_PRESHARED_KEY:-}"
# The seeded bootstrap identity is admin => may read; mint a fresh key with no tuples.
request POST /v1/api-keys "$boot_token" '{"name":"secrets-verify-unauthorized"}'
assert_code 201 "authz: bootstrap may mint"
k_id="$(printf '%s' "$LAST_BODY" | yq '.id' -)"; k_secret="$(printf '%s' "$LAST_BODY" | yq '.secret' -)"
k_token="$(token_for "$k_id" "$k_secret")"
[ -n "$k_token" ] || fail "no token for the unauthorized key"
request GET "/v1/services/$SVC/env-vars" "$k_token" "";                          assert_code 403 "authz: tuple-less key may not read env-vars"
request PUT "/v1/services/$SVC/env-vars" "$k_token" '[{"key":"X","value":"y"}]'; assert_code 403 "authz: tuple-less key may not write env-vars"
request GET "/v1/services/$SVC/env-vars" "$boot_token" "";                       assert_code 200 "authz: admin still may read env-vars"
request DELETE "/v1/api-keys/$k_id" "$boot_token" "" >/dev/null 2>&1 || true

# --- FLOW 3: unavailable (BEX_OPENBAO_URL unset) -------------------------------
echo "==> [3/3] unavailable: with no OpenBao wired, env-vars 503; the rest is unchanged"
stop_api
start_api # no BEX_OPENBAO_URL
request GET "/v1/services/$SVC/env-vars" "$boot_token" "";  assert_code 503 "no store: GET env-vars => 503"
request PUT "/v1/services/$SVC/env-vars" "$boot_token" '[]'; assert_code 503 "no store: PUT env-vars => 503"
request GET "/v1/services" "$boot_token" "";               assert_code 200 "no store: the rest of the API is unaffected"

echo "PASS: env-vars stored in OpenBao, materialized, served; 403 gated; 503 when unconfigured"
