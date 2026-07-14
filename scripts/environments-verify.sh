#!/usr/bin/env bash
# E2E for the environments feature — a named subset of a project's services
# (e.g. staging/production), layered on top of w1/m31's projects — against the
# CAPD mock cluster. Proves: create a project, create two environments under
# it, assign real services to them (which also joins the services to the
# project), and read/filter that grouping back consistently over REST,
# GraphQL, and MCP (streamable-HTTP, bearer-gated).
#
# Shape (events-verify.sh's harness, adapted): bex-api runs on the host (go
# build) against the REAL cluster via $KUBECONFIG for App CRs — reconciled by
# whatever operator is already running there. Hydra is reached through a
# kubectl port-forward. The control-plane store is a THROWAWAY local Postgres
# (docker), not the cluster's own bex-db — isolated from any other
# in-progress workspace state on the shared database.
#
# Usage: scripts/environments-verify.sh     # respects $KUBECONFIG; exits 0 on pass
# Requires: kubectl, curl, jq, go, docker; the mock cluster with auth (Hydra)
# and the App CRD installed.
set -euo pipefail
cd "$(dirname "$0")/.."

AUTH_NS=auth
APP_NS=default

HYDRA_PUBLIC=127.0.0.1:24448
HYDRA_ADMIN=127.0.0.1:24449
DB_PORT=25438
API=127.0.0.1:18290
CP=127.0.0.1:18292

STAMP="$(date +%s)"
TENANT="envt$STAMP"
APPS=(alpha beta gamma)
DB_CONTAINER="bex-environments-verify-db-$STAMP"

API_PID=""
PIDS=()

reap() {
  local pid="$1"
  [ -n "$pid" ] || return 0
  kill "$pid" 2>/dev/null || return 0
  for _ in $(seq 1 20); do
    kill -0 "$pid" 2>/dev/null || return 0
    sleep 0.25
  done
  kill -9 "$pid" 2>/dev/null || true
}

cleanup() {
  reap "$API_PID"
  if [ "${#PIDS[@]}" -gt 0 ]; then
    kill "${PIDS[@]}" 2>/dev/null || true
    wait "${PIDS[@]}" 2>/dev/null || true
  fi
  for a in "${APPS[@]}"; do
    kubectl -n "$APP_NS" delete app.app.bex.co "$TENANT-$a" --ignore-not-found >/dev/null 2>&1 || true
  done
  docker rm -f "$DB_CONTAINER" >/dev/null 2>&1 || true
}
trap cleanup EXIT

pass() { printf '\033[32mPASS\033[0m %s\n' "$1"; }
fail() { printf '\033[31mFAIL\033[0m %s\n' "$1"; exit 1; }
info() { printf '\033[36m•\033[0m %s\n' "$1"; }

wait_http() {
  for _ in $(seq 1 40); do
    [ "$(curl -s -o /dev/null -w '%{http_code}' "$1" || true)" != "000" ] && return 0
    sleep 1
  done
  fail "$1 did not become reachable"
}

# --- preflight ---------------------------------------------------------------
for port in 18290 18292; do
  lsof -nP -iTCP:"$port" -sTCP:LISTEN >/dev/null 2>&1 \
    && fail "port $port is already in use — a previous bex-api is still running (pkill -f bex-api)"
done
kubectl get crd apps.app.bex.co >/dev/null 2>&1 \
  || fail "App CRD not installed — run 'make -C lego/operator install' (absolute KUBECONFIG)"

echo "==> throwaway control-plane Postgres (docker) + port-forward (hydra) + building bex-api"
docker run --rm -d --name "$DB_CONTAINER" -e POSTGRES_PASSWORD=pw -p "$DB_PORT:5432" postgres:17 >/dev/null \
  || fail "could not start throwaway Postgres (is docker running?)"
for _ in $(seq 1 30); do docker exec "$DB_CONTAINER" pg_isready -U postgres >/dev/null 2>&1 && break; sleep 1; done
docker exec "$DB_CONTAINER" pg_isready -U postgres >/dev/null 2>&1 || fail "throwaway Postgres never became ready"

kubectl -n "$AUTH_NS" port-forward service/hydra-public "${HYDRA_PUBLIC##*:}:4444" >/dev/null 2>&1 & PIDS+=($!)
kubectl -n "$AUTH_NS" port-forward service/hydra-admin "${HYDRA_ADMIN##*:}:4445" >/dev/null 2>&1 & PIDS+=($!)

bindir="$(mktemp -d)"
(cd lego/backend && go build -o "$bindir/bex-api" ./cmd/api) || fail "bex-api build failed"
wait_http "http://$HYDRA_ADMIN/health/ready"

echo "==> bootstrap OAuth2 client"
export BEX_BOOTSTRAP_CLIENT_SECRET="${BEX_BOOTSTRAP_CLIENT_SECRET:-environments-verify-$STAMP}"
HYDRA_ADMIN_URL="http://$HYDRA_ADMIN" bash scripts/auth-bootstrap-client.sh >/dev/null
TOKEN="$({ curl -s -X POST "http://$HYDRA_PUBLIC/oauth2/token" \
  -d "grant_type=client_credentials&client_id=bex-bootstrap&client_secret=$BEX_BOOTSTRAP_CLIENT_SECRET" || true; } \
  | jq -r '.access_token // empty')"
[ -n "$TOKEN" ] || fail "no access_token for the bootstrap client"

DB_URI="postgres://postgres:pw@127.0.0.1:${DB_PORT}/postgres?sslmode=disable"
CP_TOKEN="environments-verify-$STAMP"

echo "==> starting bex-api (control-plane store wired, App CRs land on the real cluster)"
env "BEX_CP_DB_URI=$DB_URI" "BEX_CP_ADDR=:18292" "BEX_CP_TOKEN=$CP_TOKEN" \
  "BEX_HYDRA_ADMIN_URL=http://$HYDRA_ADMIN" BEX_API_ADDR=":18290" BEX_API_NAMESPACE="$APP_NS" \
  "$bindir/bex-api" >"$bindir/api.log" 2>&1 &
API_PID=$!
wait_http "http://$API/healthz"

request() { # METHOD PATH BODY -> LAST_CODE + LAST_BODY (public API, bearer-gated)
  local args=(-s -w '\n%{http_code}' -X "$1" "http://$API$2" -H "Authorization: Bearer $TOKEN")
  [ -n "${3:-}" ] && args+=(-H 'Content-Type: application/json' -d "$3")
  local out; out="$(curl "${args[@]}")"
  LAST_CODE="${out##*$'\n'}"; LAST_BODY="${out%$'\n'*}"
}
expect() { [ "$LAST_CODE" = "$1" ] || fail "$2: got $LAST_CODE want $1 (body: $LAST_BODY)"; }
gql() { # QUERY -> LAST_BODY (parsed .data)
  local q; q="$(jq -n --arg q "$1" '{query:$q}')"
  LAST_BODY="$(curl -s -X POST "http://$API/graphql" -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' -d "$q")"
}

# --- create three store-managed services in one workspace --------------------
echo "==> creating three services through the control-plane API"
cp_post() { curl -s -X POST "http://$CP$1" -H "Authorization: Bearer $CP_TOKEN" -H 'Content-Type: application/json' -d "$2"; }
TENANT_ID="$(cp_post /v1/tenants "{\"name\":\"$TENANT\",\"plan\":\"pro\"}" | jq -r '.id // empty')"
[ -n "$TENANT_ID" ] || fail "could not create tenant via the control-plane API (is BEX_CP_ADDR up?)"
# The internal tenant API only inserts the tenants row (no membership);
# Projects/Environments verbs resolve the caller's workspace via
# tenant_members (core.Base's AuthorizeOn), so seed the bootstrap client's
# membership — otherwise every write below 403s, correctly.
docker exec "$DB_CONTAINER" psql -U postgres -c \
  "INSERT INTO tenant_members (tenant_id, subject, role) VALUES ('$TENANT_ID', 'bex-bootstrap', 'admin')" >/dev/null \
  || fail "could not seed tenant_members for the bootstrap client"

SVC_IDS=()
for a in "${APPS[@]}"; do
  aid="$(cp_post /v1/apps "{\"tenantId\":\"$TENANT_ID\",\"name\":\"$a\",\"image\":\"traefik/whoami\",\"port\":80,\"replicas\":1,\"tier\":\"starter\"}" | jq -r '.id // empty')"
  [ -n "$aid" ] || fail "could not create app row for $a"
  SVC_IDS+=("$TENANT-$a") # the projector names the App CR <tenant>-<app>
done
info "workspace $TENANT_ID with services: ${SVC_IDS[*]}"

for svc in "${SVC_IDS[@]}"; do
  for _ in $(seq 1 30); do kubectl -n "$APP_NS" get app.app.bex.co "$svc" >/dev/null 2>&1 && break; sleep 2; done
  kubectl -n "$APP_NS" get app.app.bex.co "$svc" >/dev/null 2>&1 || fail "the projector never created App/$svc"
done
pass "three services created and projected as App CRs"

# --- 1. create a project with two environments over REST ---------------------
echo "==> creating a project with two environments over REST"
request POST "/v1/projects" "{\"name\":\"web-platform\",\"ownerId\":\"$TENANT_ID\"}"
expect 201 "create project"
PROJECT_ID="$(echo "$LAST_BODY" | jq -r '.id')"
[ -n "$PROJECT_ID" ] && [ "$PROJECT_ID" != "null" ] || fail "no project id in response: $LAST_BODY"

request POST "/v1/environments" "{\"projectId\":\"$PROJECT_ID\",\"name\":\"staging\"}"
expect 201 "create staging environment"
STAGING_ID="$(echo "$LAST_BODY" | jq -r '.id')"
request POST "/v1/environments" "{\"projectId\":\"$PROJECT_ID\",\"name\":\"production\"}"
expect 201 "create production environment"
PRODUCTION_ID="$(echo "$LAST_BODY" | jq -r '.id')"
[ -n "$STAGING_ID" ] && [ -n "$PRODUCTION_ID" ] || fail "environment ids missing"
pass "project $PROJECT_ID created with environments staging=$STAGING_ID production=$PRODUCTION_ID"

# alpha,beta -> staging; gamma -> production. NOTE: the service-links verbs
# match on the control-plane apps.name column (see store.SetEnvironmentServices),
# which for services minted through the internal tenant bootstrap API (as
# these are, for speed) is the UNQUALIFIED app name ("alpha") — NOT the
# reconciler-projected App CR name ("$TENANT-alpha", stored in SVC_IDS). The
# public POST /v1/services create path has no such split (a caller's chosen
# name IS both the row name and the CR name), so this divergence is a quirk of
# the internal bootstrap API this script uses to seed services quickly, not of
# the environments/projects feature itself.
request PUT "/v1/environments/$STAGING_ID/service-links" "{\"serviceIds\":[\"${APPS[0]}\",\"${APPS[1]}\"]}"
expect 200 "assign alpha+beta to staging"
request PUT "/v1/environments/$PRODUCTION_ID/service-links" "{\"serviceIds\":[\"${APPS[2]}\"]}"
expect 200 "assign gamma to production"
pass "assigned alpha+beta to staging, gamma to production"

# --- 2. read back over REST: environment lists its services + auto-joined project
echo "==> verifying REST reads"
request GET "/v1/environments/$STAGING_ID" ''
expect 200 "get staging environment"
staging_services="$(echo "$LAST_BODY" | jq -r '.serviceIds | sort | join(",")')"
want="$(printf '%s\n%s' "${APPS[0]}" "${APPS[1]}" | sort | paste -sd, -)"
[ "$staging_services" = "$want" ] || fail "staging serviceIds = $staging_services, want $want"

request GET "/v1/environments?projectId=$PROJECT_ID" ''
expect 200 "list environments under project"
n="$(echo "$LAST_BODY" | jq 'length')"
[ "$n" = "2" ] || fail "environments under project = $n, want 2"

# Assigning to an environment also joins the service to the project — prove it
# via the project's own service-links (Projects' own read surface).
request GET "/v1/projects/$PROJECT_ID" ''
expect 200 "get project"
proj_services="$(echo "$LAST_BODY" | jq -r '.serviceIds | sort | join(",")')"
want_all="$(printf '%s\n%s\n%s' "${APPS[0]}" "${APPS[1]}" "${APPS[2]}" | sort | paste -sd, -)"
[ "$proj_services" = "$want_all" ] || fail "project serviceIds = $proj_services, want $want_all (assigning to an environment should auto-join the project)"
pass "REST: an environment lists its assigned services, projectId scopes the environments list, and assignment auto-joins the project"

# --- 3. read back over GraphQL -------------------------------------------------
echo "==> verifying GraphQL reads"
gql "{ environments(projectId: \"$PROJECT_ID\") { id name serviceIds } }"
gql_envs="$(echo "$LAST_BODY" | jq '.data.environments | length')"
[ "$gql_envs" = "2" ] || fail "GraphQL environments(projectId:) = $gql_envs, want 2 (body: $LAST_BODY)"

gql "{ environment(id: \"$PRODUCTION_ID\") { id projectId serviceIds } }"
gql_prod_services="$(echo "$LAST_BODY" | jq -r '.data.environment.serviceIds | join(",")')"
[ "$gql_prod_services" = "${APPS[2]}" ] || fail "GraphQL environment(production).serviceIds = $gql_prod_services, want ${APPS[2]} (body: $LAST_BODY)"
pass "GraphQL: environments(projectId:) and environment(id:) agree with REST"

# --- 4. read back over MCP (streamable-http, bearer-gated) --------------------
echo "==> verifying MCP reads"
cat > "$bindir/mcpcheck.go" <<'EOF'
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type bearerRT struct{ token string; base http.RoundTripper }

func (b bearerRT) RoundTrip(r *http.Request) (*http.Response, error) {
	r.Header.Set("Authorization", "Bearer "+b.token)
	return b.base.RoundTrip(r)
}

func main() {
	url, token, projectID, stagingID := os.Args[1], os.Args[2], os.Args[3], os.Args[4]
	ctx := context.Background()
	hc := &http.Client{Transport: bearerRT{token: token, base: http.DefaultTransport}}
	transport := &mcp.StreamableClientTransport{Endpoint: url, HTTPClient: hc}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "verify", Version: "0"}, nil).Connect(ctx, transport, nil)
	if err != nil {
		fmt.Println("connect error:", err)
		os.Exit(1)
	}
	defer cs.Close()

	call := func(name string, args map[string]any) map[string]any {
		res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
		if err != nil {
			fmt.Printf("%s error: %v\n", name, err)
			os.Exit(1)
		}
		if len(res.Content) == 0 {
			fmt.Printf("%s: empty content\n", name)
			os.Exit(1)
		}
		tc, ok := res.Content[0].(*mcp.TextContent)
		if !ok {
			fmt.Printf("%s: non-text content\n", name)
			os.Exit(1)
		}
		var out map[string]any
		if err := json.Unmarshal([]byte(tc.Text), &out); err != nil {
			fmt.Printf("%s: bad json: %v\n", name, err)
			os.Exit(1)
		}
		return out
	}

	list := call("list_environments", map[string]any{"projectId": projectID})
	els, _ := list["environments"].([]any)
	if len(els) != 2 {
		fmt.Printf("list_environments: %d, want 2 (%v)\n", len(els), list)
		os.Exit(1)
	}

	staging := call("get_environment", map[string]any{"id": stagingID})
	sids, _ := staging["serviceIds"].([]any)
	if len(sids) != 2 {
		fmt.Printf("get_environment(staging): serviceIds = %d, want 2 (%v)\n", len(sids), staging)
		os.Exit(1)
	}

	proj := call("get_project", map[string]any{"id": projectID})
	psids, _ := proj["serviceIds"].([]any)
	if len(psids) != 3 {
		fmt.Printf("get_project: serviceIds = %d, want 3 (%v)\n", len(psids), proj)
		os.Exit(1)
	}

	fmt.Println("OK")
}
EOF
(cd lego/backend && go run "$bindir/mcpcheck.go" "http://$API/mcp" "$TOKEN" "$PROJECT_ID" "$STAGING_ID") \
  | tee "$bindir/mcpcheck.out"
grep -q '^OK$' "$bindir/mcpcheck.out" || fail "MCP verification did not report OK"
pass "MCP: list_environments/get_environment/get_project agree with REST/GraphQL"

# --- 5. store-less 503 --------------------------------------------------------
echo "==> restarting bex-api with the control-plane store unwired"
reap "$API_PID"
API_PID=""
for _ in $(seq 1 20); do lsof -nP -iTCP:18290 -sTCP:LISTEN >/dev/null 2>&1 || break; sleep 0.5; done
env "BEX_HYDRA_ADMIN_URL=http://$HYDRA_ADMIN" BEX_API_ADDR=":18290" BEX_API_NAMESPACE="$APP_NS" \
  "$bindir/bex-api" >>"$bindir/api.log" 2>&1 &
API_PID=$!
wait_http "http://$API/healthz"
request GET "/v1/environments?projectId=$PROJECT_ID" ''
[ "$LAST_CODE" = "503" ] \
  && pass "store-less: environments 503 (omitted, not faked as an empty list)" \
  || fail "store-less GET /v1/environments = $LAST_CODE, want 503 (body: $LAST_BODY)"

echo
pass "environments verified: grouping + assignment (with project auto-join) + filtering agree across REST, GraphQL, and MCP"
