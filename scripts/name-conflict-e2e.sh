#!/usr/bin/env bash
# E2E for w4/m19 — duplicate service names: workspace-unique names + globally-
# unique subdomains (Render-consistent). Proves, against the REAL CAPD mock
# cluster + a real bex-api + a real operator, that:
#
#   1. Workspace A creates "beancount-cms" -> 201; its App CR carries the bare
#      slug (spec.subdomain=beancount-cms), host beancount-cms.<base>.
#   2. Workspace A creates "beancount-cms" AGAIN -> 409 "name ... already in
#      use" on REST, GraphQL, AND MCP (create_web_service) — never an upsert,
#      never a 500.
#   3. serviceNameAvailable("beancount-cms") in workspace A -> {available:false,
#      suggestion:"beancount-cms-1"}; creating that suggestion -> 201.
#   4. Workspace B's serviceNameAvailable("beancount-cms") -> {available:true}
#      (no cross-tenant existence leak). Workspace B creates "beancount-cms" ->
#      201; the store mints a globally-unique slug with a random -xxxx suffix.
#   5. The two App CRs' spec.subdomain values are DISTINCT (bare vs suffixed),
#      and the operator reconciles two Ingresses with DISTINCT hosts — no
#      collision.
#
# Shape (environments-verify.sh's harness + auth-oauth21-e2e.sh's throwaway
# Hydra, adapted): bex-api + the operator run on the host (go build) against the
# real cluster via $KUBECONFIG. Hydra + the control-plane Postgres are throwaway
# Docker containers (in-memory / ephemeral). OpenFGA is UNSET -> allow-all
# (docs/ADR012-auth.md); authorization is exercised via tenant_members seeded
# directly, exactly like environments-verify.sh. Two OAuth2 clients (one per
# workspace) prove the cross-tenant legs.
#
# Usage: KUBECONFIG=infra/local/bex.kubeconfig scripts/name-conflict-e2e.sh
# Requires: kubectl, curl, jq, go, docker; the mock cluster up with the App CRD
# installed (cd lego/operator && make install).
set -euo pipefail
cd "$(dirname "$0")/.."

APP_NS=default
BASE_DOMAIN=e2e-m19.test

HYDRA_PUBLIC=127.0.0.1:24491
HYDRA_ADMIN=127.0.0.1:24492
DB_PORT=25491
API=127.0.0.1:18291
CP=127.0.0.1:18293

STAMP="$(date +%s)"
HYDRA_CONTAINER="bex-m19-hydra-$STAMP"
DB_CONTAINER="bex-m19-db-$STAMP"
SVC_NAME="beancount-cms"

API_PID=""
OP_PID=""
CREATED_CRS=()
bindir="$(mktemp -d)"

reap() {
  local pid="$1"
  [ -n "$pid" ] || return 0
  kill "$pid" 2>/dev/null || return 0
  for _ in $(seq 1 20); do kill -0 "$pid" 2>/dev/null || return 0; sleep 0.25; done
  kill -9 "$pid" 2>/dev/null || true
}

cleanup() {
  # Delete the App CRs FIRST, while the operator is still alive to clear its
  # finalizer (app.bex.co/finalizer) — reaping it first would make the delete
  # block forever on an unremovable finalizer.
  for cr in "${CREATED_CRS[@]:-}"; do
    [ -n "$cr" ] && kubectl -n "$APP_NS" delete app.app.bex.co "$cr" --ignore-not-found --wait=false >/dev/null 2>&1 || true
  done
  sleep 2
  # Belt-and-suspenders: strip any finalizer still lingering (operator already
  # gone / raced) so nothing blocks, then reap and remove containers.
  for cr in "${CREATED_CRS[@]:-}"; do
    [ -n "$cr" ] || continue
    kubectl -n "$APP_NS" get app.app.bex.co "$cr" >/dev/null 2>&1 \
      && kubectl -n "$APP_NS" patch app.app.bex.co "$cr" --type=merge -p '{"metadata":{"finalizers":[]}}' >/dev/null 2>&1 || true
  done
  reap "$API_PID"
  reap "$OP_PID"
  docker rm -f "$HYDRA_CONTAINER" "$DB_CONTAINER" >/dev/null 2>&1 || true
  rm -rf "$bindir"
}
trap cleanup EXIT

pass() { printf '\033[32mPASS\033[0m %s\n' "$1"; }
fail() { printf '\033[31mFAIL\033[0m %s\n' "$1"; exit 1; }
info() { printf '\033[36m•\033[0m %s\n' "$1"; }

wait_http() {
  for _ in $(seq 1 60); do
    [ "$(curl -s -o /dev/null -w '%{http_code}' "$1" || true)" != "000" ] && return 0
    sleep 1
  done
  fail "$1 did not become reachable"
}

# --- preflight ---------------------------------------------------------------
for pair in "$HYDRA_PUBLIC" "$HYDRA_ADMIN" "$API" "$CP"; do
  port="${pair##*:}"
  lsof -nP -iTCP:"$port" -sTCP:LISTEN >/dev/null 2>&1 && fail "port $port already in use"
done
lsof -nP -iTCP:"$DB_PORT" -sTCP:LISTEN >/dev/null 2>&1 && fail "port $DB_PORT already in use"
kubectl get crd apps.app.bex.co >/dev/null 2>&1 \
  || fail "App CRD not installed — run 'make -C lego/operator install' (absolute KUBECONFIG)"
docker info >/dev/null 2>&1 || fail "docker is not running"

echo "==> throwaway Hydra (in-memory) + control-plane Postgres (docker)"
docker rm -f "$HYDRA_CONTAINER" >/dev/null 2>&1 || true
docker run -d --name "$HYDRA_CONTAINER" \
  -p "${HYDRA_PUBLIC##*:}:4444" -p "${HYDRA_ADMIN##*:}:4445" \
  -e DSN=memory \
  -e URLS_SELF_ISSUER="http://$HYDRA_PUBLIC" \
  -e SECRETS_SYSTEM=e2e-only-system-secret-32-chars-x \
  oryd/hydra:v2.2.0 serve all --dev >/dev/null \
  || fail "could not start throwaway Hydra"
docker run --rm -d --name "$DB_CONTAINER" -e POSTGRES_PASSWORD=pw -p "$DB_PORT:5432" postgres:17 >/dev/null \
  || fail "could not start throwaway Postgres"
for _ in $(seq 1 30); do docker exec "$DB_CONTAINER" pg_isready -U postgres >/dev/null 2>&1 && break; sleep 1; done
docker exec "$DB_CONTAINER" pg_isready -U postgres >/dev/null 2>&1 || fail "throwaway Postgres never became ready"
wait_http "http://$HYDRA_ADMIN/health/ready"

echo "==> building bex-api + operator (manager)"
(cd lego/backend && go build -o "$bindir/bex-api" ./cmd/api) || fail "bex-api build failed"
(cd lego/operator && go build -o "$bindir/manager" ./cmd/manager) || fail "manager build failed"

echo "==> bootstrapping two OAuth2 clients (one per workspace)"
export BEX_BOOTSTRAP_CLIENT_SECRET="${BEX_BOOTSTRAP_CLIENT_SECRET:-m19-verify-secret-$STAMP}"
HYDRA_ADMIN_URL="http://$HYDRA_ADMIN" bash scripts/auth-bootstrap-client.sh >/dev/null
# Second client for workspace B (a distinct caller identity => distinct tenant).
CLIENT_B=bex-bootstrap-b
SECRET_B="m19-verify-b-secret-$STAMP"
body_b="$(printf '{"client_id":"%s","client_name":"bex bootstrap B (m19 e2e)","client_secret":"%s","grant_types":["client_credentials"],"token_endpoint_auth_method":"client_secret_post"}' "$CLIENT_B" "$SECRET_B")"
code="$(printf '%s' "$body_b" | curl -s -o /dev/null -w '%{http_code}' -X PUT -H 'Content-Type: application/json' -d @- "http://$HYDRA_ADMIN/admin/clients/$CLIENT_B")"
if [ "$code" = "404" ]; then
  code="$(printf '%s' "$body_b" | curl -s -o /dev/null -w '%{http_code}' -X POST -H 'Content-Type: application/json' -d @- "http://$HYDRA_ADMIN/admin/clients")"
fi
[ "$code" = "200" ] || [ "$code" = "201" ] || fail "could not upsert $CLIENT_B (HTTP $code)"

token_for() { # client_id secret -> access token
  curl -s -X POST "http://$HYDRA_PUBLIC/oauth2/token" \
    -d "grant_type=client_credentials&client_id=$1&client_secret=$2" | jq -r '.access_token // empty'
}
TOKEN_A="$(token_for bex-bootstrap "$BEX_BOOTSTRAP_CLIENT_SECRET")"
TOKEN_B="$(token_for "$CLIENT_B" "$SECRET_B")"
[ -n "$TOKEN_A" ] && [ -n "$TOKEN_B" ] || fail "could not mint both bootstrap tokens"

DB_URI="postgres://postgres:pw@127.0.0.1:${DB_PORT}/postgres?sslmode=disable"
CP_TOKEN="m19-cp-$STAMP"

echo "==> starting the operator (reconciles App CRs into Ingress objects)"
env KUBECONFIG="$KUBECONFIG" BEX_RUNTIME=kubernetes BEX_BASE_DOMAIN="$BASE_DOMAIN" \
  "$bindir/manager" --metrics-bind-address=0 --health-probe-bind-address=0 \
  >"$bindir/operator.log" 2>&1 &
OP_PID=$!

echo "==> starting bex-api (control-plane store wired; App CRs land on the real cluster)"
env "BEX_CP_DB_URI=$DB_URI" "BEX_CP_ADDR=:${CP##*:}" "BEX_CP_TOKEN=$CP_TOKEN" \
  "BEX_CP_APPS_NAMESPACE=$APP_NS" \
  "BEX_HYDRA_ADMIN_URL=http://$HYDRA_ADMIN" "BEX_API_ADDR=:${API##*:}" \
  "BEX_API_NAMESPACE=$APP_NS" "BEX_BASE_DOMAIN=$BASE_DOMAIN" \
  "$bindir/bex-api" >"$bindir/api.log" 2>&1 &
API_PID=$!
wait_http "http://$API/healthz"
wait_http "http://$CP/healthz"

echo "==> creating two workspaces (tenants) + seeding membership"
cp_post() { curl -s -X POST "http://$CP$1" -H "Authorization: Bearer $CP_TOKEN" -H 'Content-Type: application/json' -d "$2"; }
TENANT_A="$(cp_post /v1/tenants '{"name":"workspace-a","plan":"pro"}' | jq -r '.id // empty')"
TENANT_B="$(cp_post /v1/tenants '{"name":"workspace-b","plan":"pro"}' | jq -r '.id // empty')"
[ -n "$TENANT_A" ] && [ -n "$TENANT_B" ] || fail "could not create both tenants via the control-plane API"
docker exec "$DB_CONTAINER" psql -U postgres -c \
  "INSERT INTO tenant_members (tenant_id, subject, role) VALUES ('$TENANT_A','bex-bootstrap','admin'),('$TENANT_B','$CLIENT_B','admin')" >/dev/null \
  || fail "could not seed tenant_members"
info "workspace A=$TENANT_A (bex-bootstrap)  workspace B=$TENANT_B ($CLIENT_B)"

# --- request helpers ---------------------------------------------------------
request() { # TOKEN METHOD PATH BODY -> LAST_CODE + LAST_BODY
  local args=(-s -w '\n%{http_code}' -X "$2" "http://$API$3" -H "Authorization: Bearer $1")
  [ -n "${4:-}" ] && args+=(-H 'Content-Type: application/json' -d "$4")
  local out; out="$(curl "${args[@]}")"
  LAST_CODE="${out##*$'\n'}"; LAST_BODY="${out%$'\n'*}"
}
gql() { # TOKEN QUERY -> LAST_BODY
  local q; q="$(jq -n --arg q "$2" '{query:$q}')"
  LAST_BODY="$(curl -s -X POST "http://$API/graphql" -H "Authorization: Bearer $1" -H 'Content-Type: application/json' -d "$q")"
}
svc_body() { printf '{"name":"%s","image":{"imagePath":"traefik/whoami"},"port":80}' "$1"; }

# --- 1. Workspace A creates beancount-cms -> 201, bare slug ------------------
echo "==> [1] workspace A creates $SVC_NAME"
request "$TOKEN_A" POST /v1/services "$(svc_body "$SVC_NAME")"
[ "$LAST_CODE" = "201" ] || fail "[1] create $SVC_NAME: got $LAST_CODE want 201 (body: $LAST_BODY)"
CR_A="$TENANT_A-$SVC_NAME"; CREATED_CRS+=("$CR_A")
SUBDOMAIN_A="$(kubectl -n "$APP_NS" get app.app.bex.co "$CR_A" -o jsonpath='{.spec.subdomain}')"
[ "$SUBDOMAIN_A" = "$SVC_NAME" ] || fail "[1] workspace A slug = '$SUBDOMAIN_A', want bare '$SVC_NAME'"
pass "[1] workspace A: 201, CR $CR_A spec.subdomain=$SUBDOMAIN_A (bare, no suffix)"

# --- 2. Workspace A duplicate -> 409 on REST, GraphQL, MCP -------------------
echo "==> [2a] workspace A duplicate over REST -> 409"
request "$TOKEN_A" POST /v1/services "$(svc_body "$SVC_NAME")"
[ "$LAST_CODE" = "409" ] || fail "[2a] REST duplicate: got $LAST_CODE want 409 (body: $LAST_BODY)"
echo "$LAST_BODY" | grep -qi 'already in use' || fail "[2a] REST 409 body not 'already in use'-shaped: $LAST_BODY"
pass "[2a] REST duplicate -> 409: $(echo "$LAST_BODY" | jq -rc '.error // .')"

echo "==> [2b] workspace A duplicate over GraphQL -> error"
gql "$TOKEN_A" "mutation { createService(name: \"$SVC_NAME\", image: \"traefik/whoami\", port: 80) { id } }"
GQL_ERR="$(echo "$LAST_BODY" | jq -rc '.errors[0].message // empty')"
[ -n "$GQL_ERR" ] || fail "[2b] GraphQL duplicate: no error returned (body: $LAST_BODY)"
echo "$GQL_ERR" | grep -qi 'already in use' || fail "[2b] GraphQL error not 'already in use'-shaped: $GQL_ERR"
# It must NOT have created a real service. graphql-go serializes the resolver's
# zero-value AppView as a non-null object with EMPTY fields (not JSON null)
# alongside the error, so assert the returned object carries no real service
# (empty name) — i.e. nothing was created. The errors[] entry above is the
# Render-shaped conflict a GraphQL client actually checks.
GQL_NAME="$(echo "$LAST_BODY" | jq -r '.data.createService.name // ""')"
[ -z "$GQL_NAME" ] || fail "[2b] GraphQL duplicate returned a real service named '$GQL_NAME' (want an error, no create)"
pass "[2b] GraphQL duplicate -> error: $GQL_ERR"

echo "==> [2c] workspace A duplicate over MCP (create_web_service) -> error"
cat > "$bindir/mcpcheck.go" <<'EOF'
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type bearerRT struct {
	token string
	base  http.RoundTripper
}

func (b bearerRT) RoundTrip(r *http.Request) (*http.Response, error) {
	r.Header.Set("Authorization", "Bearer "+b.token)
	return b.base.RoundTrip(r)
}

func main() {
	url, token, name := os.Args[1], os.Args[2], os.Args[3]
	ctx := context.Background()
	hc := &http.Client{Transport: bearerRT{token: token, base: http.DefaultTransport}}
	transport := &mcp.StreamableClientTransport{Endpoint: url, HTTPClient: hc}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "m19-verify", Version: "0"}, nil).Connect(ctx, transport, nil)
	if err != nil {
		fmt.Println("connect error:", err)
		os.Exit(1)
	}
	defer cs.Close()

	res, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name:      "create_web_service",
		Arguments: map[string]any{"name": name, "image": "traefik/whoami", "port": 80},
	})
	// A duplicate must surface as a failure, never a silent success. The SDK
	// maps a tool handler error onto either a transport error or an
	// IsError result with the message in its content — accept either, and
	// require the "already in use" semantics.
	msg := ""
	if err != nil {
		msg = err.Error()
	} else if res != nil {
		if len(res.Content) > 0 {
			if tc, ok := res.Content[0].(*mcp.TextContent); ok {
				msg = tc.Text
			}
		}
		if !res.IsError {
			fmt.Printf("NOT-AN-ERROR: create_web_service returned a success result: %q\n", msg)
			os.Exit(1)
		}
	}
	if !strings.Contains(strings.ToLower(msg), "already in use") {
		fmt.Printf("WRONG-ERROR: %q\n", msg)
		os.Exit(1)
	}
	fmt.Printf("OK error=%q\n", msg)
}
EOF
(cd lego/backend && go run "$bindir/mcpcheck.go" "http://$API/mcp" "$TOKEN_A" "$SVC_NAME") | tee "$bindir/mcp.out"
grep -q '^OK ' "$bindir/mcp.out" || fail "[2c] MCP create_web_service duplicate did not report an 'already in use' error"
pass "[2c] MCP create_web_service duplicate -> error"

# --- 3. serviceNameAvailable in A -> not available + suggestion -------------
echo "==> [3] serviceNameAvailable($SVC_NAME) in workspace A"
gql "$TOKEN_A" "{ serviceNameAvailable(name: \"$SVC_NAME\") { available suggestion } }"
AV_A="$(echo "$LAST_BODY" | jq -rc '.data.serviceNameAvailable')"
[ "$(echo "$AV_A" | jq -r '.available')" = "false" ] || fail "[3] available should be false: $AV_A"
SUGG="$(echo "$AV_A" | jq -r '.suggestion')"
[ "$SUGG" = "$SVC_NAME-1" ] || fail "[3] suggestion = '$SUGG', want '$SVC_NAME-1'"
pass "[3] workspace A availability: $AV_A"

echo "==> [3b] creating the suggested name $SUGG -> 201"
request "$TOKEN_A" POST /v1/services "$(svc_body "$SUGG")"
[ "$LAST_CODE" = "201" ] || fail "[3b] create suggested $SUGG: got $LAST_CODE want 201 (body: $LAST_BODY)"
CREATED_CRS+=("$TENANT_A-$SUGG")
pass "[3b] the suggested name $SUGG creates successfully (201)"

# --- 4. Workspace B: no cross-tenant leak, then create ----------------------
echo "==> [4] serviceNameAvailable($SVC_NAME) in workspace B (must NOT leak A)"
gql "$TOKEN_B" "{ serviceNameAvailable(name: \"$SVC_NAME\") { available suggestion } }"
AV_B="$(echo "$LAST_BODY" | jq -rc '.data.serviceNameAvailable')"
[ "$(echo "$AV_B" | jq -r '.available')" = "true" ] \
  || fail "[4] workspace B was told $SVC_NAME is taken — cross-tenant existence LEAK: $AV_B"
pass "[4] workspace B availability: $AV_B (no leak of A's name)"

echo "==> [4b] workspace B creates $SVC_NAME -> 201, globally-unique suffixed slug"
request "$TOKEN_B" POST /v1/services "$(svc_body "$SVC_NAME")"
[ "$LAST_CODE" = "201" ] || fail "[4b] workspace B create $SVC_NAME: got $LAST_CODE want 201 (body: $LAST_BODY)"
CR_B="$TENANT_B-$SVC_NAME"; CREATED_CRS+=("$CR_B")
SUBDOMAIN_B="$(kubectl -n "$APP_NS" get app.app.bex.co "$CR_B" -o jsonpath='{.spec.subdomain}')"
[ -n "$SUBDOMAIN_B" ] || fail "[4b] workspace B CR has no spec.subdomain"
case "$SUBDOMAIN_B" in
  "$SVC_NAME"-????) : ;;
  *) fail "[4b] workspace B slug '$SUBDOMAIN_B' is not '$SVC_NAME-<4char>'" ;;
esac
pass "[4b] workspace B: 201, CR $CR_B spec.subdomain=$SUBDOMAIN_B (random suffix)"

# --- 5. Distinct subdomains + distinct reconciled Ingress hosts -------------
echo "==> [5] the two slugs are distinct + reconcile to distinct Ingress hosts"
[ "$SUBDOMAIN_A" != "$SUBDOMAIN_B" ] || fail "[5] both CRs carry the same subdomain '$SUBDOMAIN_A' — collision!"
info "A.subdomain=$SUBDOMAIN_A   B.subdomain=$SUBDOMAIN_B"

wait_ingress() { # cr-name -> host on stdout (waits for the operator to reconcile)
  for _ in $(seq 1 40); do
    local h; h="$(kubectl -n "$APP_NS" get ingress "$1" -o jsonpath='{.spec.rules[0].host}' 2>/dev/null || true)"
    [ -n "$h" ] && { echo "$h"; return 0; }
    sleep 1
  done
  return 1
}
HOST_A="$(wait_ingress "$CR_A")" || fail "[5] operator never reconciled an Ingress for $CR_A"
HOST_B="$(wait_ingress "$CR_B")" || fail "[5] operator never reconciled an Ingress for $CR_B"
[ "$HOST_A" = "$SUBDOMAIN_A.$BASE_DOMAIN" ] || fail "[5] Ingress A host '$HOST_A' != '$SUBDOMAIN_A.$BASE_DOMAIN'"
[ "$HOST_B" = "$SUBDOMAIN_B.$BASE_DOMAIN" ] || fail "[5] Ingress B host '$HOST_B' != '$SUBDOMAIN_B.$BASE_DOMAIN'"
[ "$HOST_A" != "$HOST_B" ] || fail "[5] the two Ingresses claim the SAME host '$HOST_A' — collision!"
pass "[5] distinct Ingress hosts: A=$HOST_A  B=$HOST_B (no collision)"

# Best-effort: report how far the App reached (phase/url). Pods may stay
# Deploying with no ingress controller / image pull in this sandbox — the
# NAMING/HOST correctness above is the milestone's subject, not traffic serving.
echo "==> reconcile snapshot (best-effort; serving infra is out of scope)"
kubectl -n "$APP_NS" get app.app.bex.co "$CR_A" "$CR_B" \
  -o custom-columns='NAME:.metadata.name,SUBDOMAIN:.spec.subdomain,PHASE:.status.phase,URL:.status.url' 2>/dev/null || true

echo
pass "w4/m19 verified: same-workspace duplicate rejected (409/error) across REST+GraphQL+MCP;"
pass "                 name-availability suggestion works; two workspaces both own '$SVC_NAME'"
pass "                 with distinct globally-unique slugs and no Ingress host collision"
