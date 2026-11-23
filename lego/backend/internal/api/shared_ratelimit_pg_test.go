/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/sandbox"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

func sharedRateReplicasPG(t *testing.T, st *store.PGStore) (srvA, srvB *Server, hA, hB http.Handler, advance func(time.Duration)) {
	t.Helper()
	if _, err := st.Pool.Exec(context.Background(), `DELETE FROM rate_limit_buckets`); err != nil {
		t.Fatalf("clean rate_limit_buckets: %v", err)
	}
	t.Cleanup(func() {
		_, _ = st.Pool.Exec(context.Background(), `DELETE FROM rate_limit_buckets`)
	})

	app := sampleApp("web")
	app.Labels = map[string]string{store.LabelAppID: "srv-web", core.LabelTenant: "tea-a"}
	cl := fakeClient(app)
	workspace := fakeWorkspace{"key-a": "tea-a", "key-b": "tea-a", "key-c": "tea-a", "identity-1": "tea-a"}
	hydraURL := rateKeysHydra(t)
	kratosURL := fakeKratos(t).URL

	now := time.Date(2026, 11, 23, 12, 0, 0, 0, time.UTC)
	clock := func() time.Time { return now }
	advance = func(d time.Duration) { now = now.Add(d) }

	sandboxUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[]}`))
	}))
	t.Cleanup(sandboxUpstream.Close)

	newReplica := func() (*Server, http.Handler) {
		srv := NewServer(&core.Base{
			Client: cl, Namespace: "default", Workspace: workspace, Authz: &fakeChecker{allow: true},
		}, Deps{
			DeployStore:            deployHookStore{},
			SandboxClient:          sandbox.NewClient(sandboxUpstream.URL),
			SandboxTemplates:       map[string]sandbox.Template{"base": {Image: "alpine:3", CPU: "500m", Memory: "512Mi"}},
			SandboxDefaultTemplate: "base",
			SandboxDefaultPlan:     sandbox.PlanStarter,
		})
		srv.HydraAdminURL = hydraURL
		srv.KratosURL = kratosURL
		srv.RateLimitStore = st
		srv.RateLimiter = NewRateLimiter(500, 0)
		srv.RateLimiter.Workspace = workspace
		srv.RateLimiter.SetSharedStore(st, "general")
		srv.RateLimiter.now = clock
		srv.LimitSandboxOperations()
		if srv.Sandbox != nil && srv.Sandbox.RateLimits != nil {
			srv.Sandbox.RateLimits.Keys = rateKeyCreators
			srv.Sandbox.RateLimits.Now = clock
		}
		srv.LimitRenderOperations()
		srv.RenderRateLimits.Keys = rateKeyCreators
		srv.Deploys.RateLimitNow = clock
		srv.Deploys.DeployHookLimiter.Now = clock
		srv.Deploys.DeployHookLimiter.SetSharedStore(st)
		return srv, buildHandler(t, srv)
	}

	srvA, hA = newReplica()
	srvB, hB = newReplica()
	return srvA, srvB, hA, hB, advance
}

// w8/m579 DoD 1: two bex-api replicas sharing Postgres enforce one 400/min
// Render GET budget, stepping RateLimit-Remaining monotonically 399..0 across
// both instances and refusing the 401st on either replica with 429 + Retry-After
// + RateLimit-Remaining: 0.
func TestSharedRenderGetBudgetAcrossTwoReplicasPG(t *testing.T) {
	_, _, st := ownerGuardPG(t)
	_, _, hA, hB, _ := sharedRateReplicasPG(t, st)
	handlers := []http.Handler{hA, hB}

	for i := range 400 {
		h := handlers[i%2]
		rec := do(t, h, http.MethodGet, "/v1/services?limit=1", "token-a", "")
		if rec.Code == http.StatusTooManyRequests {
			t.Fatalf("request %d (replica %d) refused early: %v", i+1, i%2, rec.Header())
		}
		wantRem := strconv.Itoa(399 - i)
		if gotLim, gotRem := rec.Header().Get("RateLimit-Limit"), rec.Header().Get("RateLimit-Remaining"); gotLim != "400" || gotRem != wantRem {
			t.Fatalf("request %d (replica %d): Limit=%q Remaining=%q, want 400 and %s", i+1, i%2, gotLim, gotRem, wantRem)
		}
	}

	// 401st on either replica must be refused with 429, Retry-After, Remaining: 0.
	for replicaIdx, h := range handlers {
		rec := do(t, h, http.MethodGet, "/v1/services?limit=1", "token-a", "")
		if rec.Code != http.StatusTooManyRequests {
			t.Fatalf("401st request on replica %d = %d, want 429", replicaIdx, rec.Code)
		}
		if rec.Header().Get("Retry-After") == "" || rec.Header().Get("RateLimit-Limit") != "400" || rec.Header().Get("RateLimit-Remaining") != "0" {
			t.Fatalf("401st request on replica %d headers = %v", replicaIdx, rec.Header())
		}
		var body map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || body["id"] != "rate_limited" {
			t.Fatalf("401st request on replica %d body = %q, want rate_limited JSON", replicaIdx, rec.Body.String())
		}
	}
}

// w8/m579 DoD 1: per-service deploy row (REST deploys + deploy hooks) and the
// hourly service-create row (20/h across a user's API keys) are shared across
// both replicas.
func TestSharedDeployRowAndServiceCreateAcrossTwoReplicasPG(t *testing.T) {
	_, _, st := ownerGuardPG(t)
	srvA, _, hA, hB, _ := sharedRateReplicasPG(t, st)
	handlers := []http.Handler{hA, hB}

	owner := core.WithIdentity(context.Background(), core.Identity{Subject: "identity-1", Method: core.MethodSession})
	hook, err := srvA.Deploys.GetDeployHook(owner, "web")
	if err != nil {
		t.Fatal(err)
	}

	// 5 REST deploys + 5 deploy hook calls spread across replicas A and B = 10 total.
	for i := range 5 {
		if rec := do(t, handlers[i%2], http.MethodPost, "/v1/services/srv-web/deploys", "token-a", `{}`); rec.Code == http.StatusTooManyRequests {
			t.Fatalf("REST deploy %d refused early", i+1)
		}
		if rec := do(t, handlers[(i+1)%2], http.MethodPost, hook.URL, "", ""); rec.Code == http.StatusTooManyRequests {
			t.Fatalf("deploy hook %d refused early", i+1)
		}
	}
	// 11th via REST on replica A and via hook on replica B are both 429.
	if rec := do(t, hA, http.MethodPost, "/v1/services/srv-web/deploys", "token-c", `{}`); rec.Code != http.StatusTooManyRequests || rec.Header().Get("RateLimit-Limit") != "10" || rec.Header().Get("RateLimit-Remaining") != "0" {
		t.Fatalf("11th REST deploy = %d %v, want 429 on 10/min deploy row", rec.Code, rec.Header())
	}
	if rec := do(t, hB, http.MethodPost, hook.URL, "", ""); rec.Code != http.StatusTooManyRequests || !strings.Contains(rec.Body.String(), "deploy hook rate limit exceeded") {
		t.Fatalf("11th deploy hook = %d %s, want hook 429", rec.Code, rec.Body.String())
	}

	// Hourly service-create row: 20 across both replicas and both of user's keys, 21st -> 429 Retry-After 180.
	tokens := []string{"token-a", "token-b"}
	for i := range 20 {
		if rec := do(t, handlers[i%2], http.MethodPost, "/v1/services", tokens[i%2], `{}`); rec.Code == http.StatusTooManyRequests {
			t.Fatalf("service create %d refused early", i+1)
		}
	}
	rec := do(t, hB, http.MethodPost, "/v1/services", "token-a", `{}`)
	if rec.Code != http.StatusTooManyRequests || rec.Header().Get("Retry-After") != "180" || rec.Header().Get("RateLimit-Limit") != "20" || rec.Header().Get("RateLimit-Remaining") != "0" {
		t.Fatalf("21st service create = %d %v, want 429 Retry-After 180 Limit 20 Remaining 0", rec.Code, rec.Header())
	}
}

// w8/m579 DoD 1: sandbox rate classes (e.g. create 100/min) share the Postgres
// store across both replicas.
func TestSharedSandboxRateClassesAcrossTwoReplicasPG(t *testing.T) {
	_, _, st := ownerGuardPG(t)
	if _, err := st.Pool.Exec(context.Background(), `DELETE FROM rate_limit_buckets`); err != nil {
		t.Fatalf("clean rate_limit_buckets: %v", err)
	}
	t.Cleanup(func() {
		_, _ = st.Pool.Exec(context.Background(), `DELETE FROM rate_limit_buckets`)
	})

	now := time.Date(2026, 11, 23, 12, 0, 0, 0, time.UTC)
	clock := func() time.Time { return now }

	newSandboxMiddleware := func() http.Handler {
		srv := NewServer(&core.Base{Client: fakeClient(), Namespace: "default"}, Deps{})
		srv.HydraAdminURL = rateKeysHydra(t)
		srv.RateLimitStore = st
		srv.RateLimiter = NewRateLimiter(500, 0)
		srv.RateLimiter.SetSharedStore(st, "general")
		srv.RateLimiter.now = clock
		limits := sandbox.NewRateLimits(rateKeyCreators)
		limits.SetSharedStore(st)
		limits.Now = clock
		srv.RateLimiter.Operations = limits
		srv.LimitRenderOperations()
		muxes, err := srv.composedMuxes()
		if err != nil {
			t.Fatal(err)
		}
		contract, err := renderContractOnce()
		if err != nil {
			t.Fatal(err)
		}
		return srv.restRateLimitMiddleware(muxes.rest, contract)(ok200)
	}

	hA := newSandboxMiddleware()
	hB := newSandboxMiddleware()
	handlers := []http.Handler{hA, hB}
	idA := core.Identity{Subject: "key-a", ClientID: "key-a", Method: core.MethodOAuth2}
	idB := core.Identity{Subject: "key-b", ClientID: "key-b", Method: core.MethodOAuth2}
	ids := []core.Identity{idA, idB}

	for i := range 100 {
		rec := httptest.NewRecorder()
		handlers[i%2].ServeHTTP(rec, reqWith(http.MethodPost, "/v1/sandboxes", ids[i%2]))
		if rec.Code != http.StatusOK {
			t.Fatalf("sandbox create %d = %d, want 200", i+1, rec.Code)
		}
		wantRem := strconv.Itoa(99 - i)
		if rec.Header().Get("RateLimit-Limit") != "100" || rec.Header().Get("RateLimit-Remaining") != wantRem {
			t.Fatalf("sandbox create %d headers = %v, want Limit 100 Remaining %s", i+1, rec.Header(), wantRem)
		}
	}
	rec := httptest.NewRecorder()
	hB.ServeHTTP(rec, reqWith(http.MethodPost, "/v1/sandboxes", idA))
	if rec.Code != http.StatusTooManyRequests || rec.Header().Get("RateLimit-Limit") != "100" || rec.Header().Get("RateLimit-Remaining") != "0" {
		t.Fatalf("101st sandbox create = %d %v, want 429 Limit 100 Remaining 0", rec.Code, rec.Header())
	}
}

// w8/m579 DoD 2: multi-bucket all-or-nothing refusal leaves the caller bucket
// untouched when the workspace bucket refuses; MCP tool refund and GraphQL
// batch admission both apply atomically in shared Postgres across replicas.
func TestSharedMultiBucketRefusalMCPAndGraphQLBatchPG(t *testing.T) {
	_, _, st := ownerGuardPG(t)
	_, _, hA, hB, _ := sharedRateReplicasPG(t, st)

	// MCP Render tool on replica A refunds its general token in Postgres and
	// spends Render's read row (400/min).
	mcpRec := mcpHTTPCall(t, hA, "token-a", `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"list_services","arguments":{"workspaceId":"tea-a"}}}`)
	if mcpRec.Code != http.StatusOK || mcpRec.Header().Get("RateLimit-Limit") != "400" || mcpRec.Header().Get("RateLimit-Remaining") != "399" {
		t.Fatalf("MCP list_services on replica A = %d %v, want 200 400/399", mcpRec.Code, mcpRec.Header())
	}
	// A subsequent REST GET on replica B sees 398 (shared read bucket).
	restRec := do(t, hB, http.MethodGet, "/v1/services?limit=1", "token-a", "")
	if restRec.Header().Get("RateLimit-Remaining") != "398" {
		t.Fatalf("REST GET on replica B after MCP list_services on A: Remaining=%q, want 398", restRec.Header().Get("RateLimit-Remaining"))
	}
	// A GraphQL query on replica B sees general Remaining = 499 (proving the
	// MCP tool's general token was refunded in Postgres).
	gqlRec := do(t, hB, http.MethodPost, "/graphql", "token-a", gqlBody(`{ __typename }`))
	if gqlRec.Header().Get("RateLimit-Limit") != "500" || gqlRec.Header().Get("RateLimit-Remaining") != "499" {
		t.Fatalf("GraphQL on replica B after MCP refund = %v, want 500/499", gqlRec.Header())
	}
	// A GraphQL batch of 3 on replica A spends 3 general tokens in Postgres, so
	// the next GraphQL call on replica B sees 499 - 3 - 1 = 495.
	batchBody := `[` + gqlBody(`{ __typename }`) + `,` + gqlBody(`{ __typename }`) + `,` + gqlBody(`{ __typename }`) + `]`
	batchRec := do(t, hA, http.MethodPost, "/graphql", "token-a", batchBody)
	if batchRec.Code != http.StatusOK || batchRec.Header().Get("RateLimit-Remaining") != "496" {
		t.Fatalf("GraphQL batch of 3 on replica A = %d %v, want 200 Remaining 496", batchRec.Code, batchRec.Header())
	}
	afterBatch := do(t, hB, http.MethodPost, "/graphql", "token-a", gqlBody(`{ __typename }`))
	if afterBatch.Header().Get("RateLimit-Remaining") != "495" {
		t.Fatalf("GraphQL on replica B after batch on A: Remaining=%q, want 495", afterBatch.Header().Get("RateLimit-Remaining"))
	}

	// A GraphQL sandbox-only query on replica A refunds its general token in
	// Postgres and spends the sandbox read class (400/min), so the general
	// budget observed on replica B stays at 495 while sandbox read drops by 1.
	sbxGQL := do(t, hA, http.MethodPost, "/graphql", "token-a", gqlBody(`{ sandboxes { id } }`))
	if sbxGQL.Code != http.StatusOK {
		t.Fatalf("GraphQL sandbox query on replica A = %d %s, want 200", sbxGQL.Code, sbxGQL.Body.String())
	}
	afterSbxGQL := do(t, hB, http.MethodPost, "/graphql", "token-a", gqlBody(`{ __typename }`))
	if afterSbxGQL.Header().Get("RateLimit-Remaining") != "494" {
		t.Fatalf("GraphQL general Remaining on replica B after sandbox-only query on A = %q, want 494 (sandbox query refunded its general token)", afterSbxGQL.Header().Get("RateLimit-Remaining"))
	}
	sbxREST := do(t, hB, http.MethodGet, "/v1/sandboxes?ownerId=tea-a", "token-a", "")
	if sbxREST.Header().Get("RateLimit-Limit") != "400" || sbxREST.Header().Get("RateLimit-Remaining") != "398" {
		t.Fatalf("REST sandbox read on replica B after GraphQL sandbox read on A = Limit %q Remaining %q, want 400 and 398", sbxREST.Header().Get("RateLimit-Limit"), sbxREST.Header().Get("RateLimit-Remaining"))
	}

	// All-or-nothing multi-bucket check across two RateLimiter instances:
	// caller has burst 5, workspace has burst 1 (spent by user-1 on replica A),
	// so user-2 in the same workspace is refused on replica B without spending
	// user-2's own caller bucket.
	now := time.Date(2026, 11, 23, 13, 0, 0, 0, time.UTC)
	ws := fakeRateWorkspace{"u1": "tea-shared", "u2": "tea-shared"}
	makeRL := func() *RateLimiter {
		rl := NewRateLimiter(1, 5)
		rl.workspace = core.NewKeyedRateLimiter[string](1, 1, time.Minute, time.Minute)
		rl.Workspace = ws
		rl.SetSharedStore(st, "test-aon")
		rl.now = func() time.Time { return now }
		return rl
	}
	rlA, rlB := makeRL(), makeRL()
	w1 := httptest.NewRecorder()
	rlA.Middleware(ok200).ServeHTTP(w1, reqWith(http.MethodGet, "/v1/test", core.Identity{Subject: "u1", Method: core.MethodOAuth2}))
	if w1.Code != http.StatusOK {
		t.Fatalf("u1 on replica A = %d, want 200", w1.Code)
	}
	w2 := httptest.NewRecorder()
	rlB.Middleware(ok200).ServeHTTP(w2, reqWith(http.MethodGet, "/v1/test", core.Identity{Subject: "u2", Method: core.MethodOAuth2}))
	if w2.Code != http.StatusTooManyRequests {
		t.Fatalf("u2 on replica B (workspace full) = %d, want 429", w2.Code)
	}
	// Peek u2's caller bucket in Postgres: must still be full (5 tokens).
	u2State, err := st.PeekRateLimitBucket(context.Background(), now, rlB.Handle("id:u2").Spec)
	if err != nil {
		t.Fatalf("peek u2: %v", err)
	}
	if u2State.Tokens != 5 {
		t.Fatalf("u2 caller bucket after workspace refusal = %v tokens, want 5 (untouched)", u2State.Tokens)
	}
}

type brokenRateLimitStore struct {
	err error
}

func (b brokenRateLimitStore) TakeRateLimitBuckets(context.Context, time.Time, int, []core.RateLimitBucketSpec) ([]core.RateLimitBucketState, time.Duration, error) {
	return nil, 0, b.err
}

func (b brokenRateLimitStore) RefundRateLimitBuckets(context.Context, time.Time, int, []core.RateLimitBucketSpec) error {
	return b.err
}

func (b brokenRateLimitStore) PeekRateLimitBucket(context.Context, time.Time, core.RateLimitBucketSpec) (core.RateLimitBucketState, error) {
	return core.RateLimitBucketState{}, b.err
}

// w8/m579 DoD 4: a Postgres outage falls back fail-open to the replica's local
// in-process limiter (no 500s) while still enforcing the local bound, and
// RefusedCredentialMiddleware keeps counting refused credentials locally too.
func TestRateLimiterFailsOpenToLocalOnStoreError(t *testing.T) {
	rl := NewRateLimiter(1, 1)
	rl.SetSharedStore(brokenRateLimitStore{err: errors.New("connection refused")}, "general")
	h := rl.Middleware(ok200)

	id := core.Identity{Subject: "alice", Method: core.MethodOAuth2}
	w1 := httptest.NewRecorder()
	h.ServeHTTP(w1, reqWith(http.MethodGet, "/v1/services", id))
	if w1.Code != http.StatusOK {
		t.Fatalf("first request during DB outage = %d, want 200 (fail-open to local)", w1.Code)
	}
	w2 := httptest.NewRecorder()
	h.ServeHTTP(w2, reqWith(http.MethodGet, "/v1/services", id))
	if w2.Code != http.StatusTooManyRequests {
		t.Fatalf("second request during DB outage = %d, want 429 (local bound still enforced)", w2.Code)
	}

	// RefusedCredentialMiddleware also falls back cleanly to local buckets.
	refuse := NewRateLimiter(1, 1)
	refuse.SetSharedStore(brokenRateLimitStore{err: errors.New("connection refused")}, "sandbox-connect")
	rc := refuse.RefusedCredentialMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	r := httptest.NewRequest(http.MethodPost, "/v1/sandboxes/connect", nil)
	r.RemoteAddr = "198.51.100.9:1234"
	rec1 := httptest.NewRecorder()
	rc.ServeHTTP(rec1, r)
	if rec1.Code != http.StatusUnauthorized {
		t.Fatalf("first bad credential during DB outage = %d, want 401", rec1.Code)
	}
	rec2 := httptest.NewRecorder()
	rc.ServeHTTP(rec2, r)
	if rec2.Code != http.StatusTooManyRequests {
		t.Fatalf("second bad credential during DB outage = %d, want 429", rec2.Code)
	}
}
