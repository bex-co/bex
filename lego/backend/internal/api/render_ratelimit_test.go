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
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/sandbox"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// w8/410: the Render-contract REST operations and MCP tools spend Render's
// per-endpoint rows (api-docs.render.com/reference/rate-limiting, read
// 2026-11-22) instead of the general BEX_RATE_LIMIT budget, through the
// composed server.

// renderRateServer is the composed server with the general limiter at 500 a
// minute and Render's rows on, over one service srv-web. With a fixed clock
// the buckets never refill unless advance moves it; without, they run on the
// wall clock (a deploy hook reads time.Now).
func renderRateServer(t *testing.T, fixedClock bool) (*Server, http.Handler, func(time.Duration)) {
	t.Helper()
	app := sampleApp("web")
	app.Labels = map[string]string{store.LabelAppID: "srv-web", core.LabelTenant: "tea-a"}
	workspace := fakeWorkspace{"key-a": "tea-a", "key-b": "tea-a", "key-c": "tea-a", "identity-1": "tea-a"}
	srv := NewServer(&core.Base{
		Client: fakeClient(app), Namespace: "default", Workspace: workspace, Authz: &fakeChecker{allow: true},
	}, Deps{DeployStore: deployHookStore{}})
	srv.HydraAdminURL = rateKeysHydra(t)
	srv.KratosURL = fakeKratos(t).URL
	srv.RateLimiter = NewRateLimiter(500, 0)
	srv.RateLimiter.Workspace = workspace
	advance := func(time.Duration) { t.Fatal("advance needs the fixed clock") }
	if fixedClock {
		now := time.Date(2026, 11, 22, 12, 0, 0, 0, time.UTC)
		srv.RateLimiter.now = func() time.Time { return now }
		advance = func(d time.Duration) { now = now.Add(d) }
	}
	srv.LimitRenderOperations()
	srv.RenderRateLimits.Keys = rateKeyCreators
	return srv, buildHandler(t, srv), advance
}

// limited sends n identical requests and answers how many were refused 429.
func limited(t *testing.T, h http.Handler, n int, method, path, token, body string) int {
	t.Helper()
	refused := 0
	for range n {
		if do(t, h, method, path, token, body).Code == http.StatusTooManyRequests {
			refused++
		}
	}
	return refused
}

// Each row reports its own budget in Render's headers: 400 on a GET, 30 on a
// write and on a log read, 20 on a service create. GraphQL and a bex-native
// route keep the general 500.
func TestRenderRowsReportTheirOwnBudgets(t *testing.T) {
	_, h, _ := renderRateServer(t, true)
	for _, c := range []struct {
		method, path, body, limit, remaining string
	}{
		{http.MethodGet, "/v1/services?limit=1", "", "400", "399"},
		{http.MethodHead, "/v1/services?limit=1", "", "400", "398"},
		{http.MethodPost, "/v1/env-groups", `{}`, "30", "29"},
		{http.MethodGet, "/v1/logs?ownerId=tea-a&resource=srv-web", "", "30", "29"},
		{http.MethodGet, "/v1/logs/values?ownerId=tea-a&resource=srv-web&label=level", "", "30", "28"},
		{http.MethodPost, "/v1/services", `{}`, "20", "19"},
		{http.MethodPost, "/v1/services/srv-web/custom-domains", `{}`, "50", "49"},
		{http.MethodPost, "/v1/services/srv-web/jobs", `{}`, "100", "99"},
		{http.MethodPost, "/v1/services/srv-web/deploys", `{}`, "10", "9"},
		{http.MethodPatch, "/v1/services/srv-web", `{}`, "10", "8"},
		{http.MethodPut, "/v1/services/srv-web/env-vars", `[]`, "30", "28"},
		{http.MethodPost, "/graphql", gqlBody(`{ __typename }`), "500", "499"},
		{http.MethodGet, "/v1/webhooks/event-types", "", "500", "498"},
	} {
		rec := do(t, h, c.method, c.path, "token-a", c.body)
		if rec.Code == http.StatusTooManyRequests || rec.Header().Get("RateLimit-Limit") != c.limit || rec.Header().Get("RateLimit-Remaining") != c.remaining {
			t.Errorf("%s %s = %d, RateLimit-Limit %q Remaining %q, want %s and %s",
				c.method, c.path, rec.Code, rec.Header().Get("RateLimit-Limit"), rec.Header().Get("RateLimit-Remaining"), c.limit, c.remaining)
		}
	}
}

// Twenty service creates an hour per user, across the user's API keys; a
// refused create counts, as the validator runs after the limiter. The bucket
// refills one create every three minutes.
func TestRenderServiceCreatesAreTwentyAnHourPerUser(t *testing.T) {
	_, h, advance := renderRateServer(t, true)
	if n := limited(t, h, 20, http.MethodPost, "/v1/services", "token-a", `{}`); n != 0 {
		t.Fatalf("%d of the first 20 creates were rate limited", n)
	}
	rec := do(t, h, http.MethodPost, "/v1/services", "token-b", `{}`)
	if rec.Code != http.StatusTooManyRequests || rec.Header().Get("Retry-After") != "180" ||
		rec.Header().Get("RateLimit-Limit") != "20" || rec.Header().Get("RateLimit-Remaining") != "0" {
		t.Fatalf("21st create from the same user's other key = %d %v, want 429 with Retry-After 180", rec.Code, rec.Header())
	}
	if got := do(t, h, http.MethodPost, "/v1/services", "token-c", `{}`).Code; got == http.StatusTooManyRequests {
		t.Fatal("another user's first create was rate limited")
	}
	// Other rows are untouched.
	if got := do(t, h, http.MethodGet, "/v1/services", "token-a", "").Code; got == http.StatusTooManyRequests {
		t.Fatal("a GET was refused by the spent create row")
	}
	advance(3 * time.Minute)
	if n := limited(t, h, 2, http.MethodPost, "/v1/services", "token-a", `{}`); n != 1 {
		t.Fatalf("%d of 2 creates three minutes later were rate limited, want exactly the second", n)
	}
}

// The deploy row is ten a minute per service, whoever sends them, across
// update, deploy, suspend and resume; another service has its own ten.
func TestRenderDeploysAreTenAMinutePerService(t *testing.T) {
	_, h, _ := renderRateServer(t, true)
	sent := limited(t, h, 4, http.MethodPost, "/v1/services/srv-a/deploys", "token-a", `{}`) +
		limited(t, h, 3, http.MethodPatch, "/v1/services/srv-a", "token-c", `{}`) +
		limited(t, h, 2, http.MethodPost, "/v1/services/srv-a/suspend", "token-a", "") +
		limited(t, h, 1, http.MethodPost, "/v1/services/srv-a/resume", "token-c", "")
	if sent != 0 {
		t.Fatalf("%d of srv-a's first 10 deploy-row calls were rate limited", sent)
	}
	if got := do(t, h, http.MethodPost, "/v1/services/srv-a/deploys", "token-c", `{}`).Code; got != http.StatusTooManyRequests {
		t.Fatalf("srv-a's 11th deploy-row call = %d, want 429", got)
	}
	if n := limited(t, h, 10, http.MethodPost, "/v1/services/srv-b/deploys", "token-a", `{}`); n != 0 {
		t.Fatalf("%d of srv-b's deploys were refused by srv-a's bucket", n)
	}
	// A restart is another write, not the deploy row.
	if got := do(t, h, http.MethodPost, "/v1/services/srv-a/restart", "token-a", "").Code; got == http.StatusTooManyRequests {
		t.Fatal("a restart was refused by the deploy row")
	}
}

// A deploy hook spends its service's deploy row: ten REST deploys leave the
// hook none, and its refusal is the hook's own 429.
func TestADeployHookSharesItsServicesDeployRow(t *testing.T) {
	srv, h, _ := renderRateServer(t, false)
	owner := core.WithIdentity(context.Background(), core.Identity{Subject: "identity-1", Method: core.MethodSession})
	hook, err := srv.Deploys.GetDeployHook(owner, "web")
	if err != nil {
		t.Fatal(err)
	}
	if n := limited(t, h, 10, http.MethodPost, "/v1/services/srv-web/deploys", "token-a", `{}`); n != 0 {
		t.Fatalf("%d of 10 REST deploys were rate limited", n)
	}
	rec := do(t, h, http.MethodPost, hook.URL, "", "")
	if rec.Code != http.StatusTooManyRequests || !strings.Contains(rec.Body.String(), "deploy hook rate limit exceeded") ||
		rec.Header().Get("Retry-After") == "" || rec.Header().Get("RateLimit-Limit") != "10" || rec.Header().Get("RateLimit-Remaining") != "0" {
		t.Fatalf("hook after 10 REST deploys = %d %s %v, want the hook's 429 on the service's bucket", rec.Code, rec.Body.String(), rec.Header())
	}
}

// A dashboard session keeps the general budget on REST: Render's dashboard
// does not use the public API, whose limits are for API keys.
func TestADashboardSessionKeepsTheGeneralBudgetOnREST(t *testing.T) {
	_, h, _ := renderRateServer(t, true)
	for _, path := range []string{"/v1/services?limit=1", "/v1/logs?ownerId=tea-a&resource=srv-web"} {
		r := httptest.NewRequest(http.MethodGet, path, nil)
		r.Header.Set("Cookie", "ory_kratos_session=live")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, r)
		if rec.Header().Get("RateLimit-Limit") != "500" {
			t.Errorf("session GET %s = %d RateLimit-Limit %q, want the general 500", path, rec.Code, rec.Header().Get("RateLimit-Limit"))
		}
	}
}

func mcpHTTPCall(t *testing.T, h http.Handler, token, call string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(call))
	r.Header.Set("Authorization", "Bearer "+token)
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Accept", "application/json, text/event-stream")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	return rec
}

// A Render MCP tool spends the row of its REST request and none of the
// general budget; trigger_deploy spends its service's deploy row; a bex-only
// tool keeps the general budget.
func TestRenderMCPToolsSpendTheirRESTRow(t *testing.T) {
	_, h, _ := renderRateServer(t, true)
	rec := mcpHTTPCall(t, h, "token-a", `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"list_services","arguments":{"workspaceId":"tea-a"}}}`)
	if rec.Code != http.StatusOK || rec.Header().Get("RateLimit-Limit") != "400" || rec.Header().Get("RateLimit-Remaining") != "399" {
		t.Fatalf("list_services = %d RateLimit-Limit %q Remaining %q, want the read row's 400 and 399", rec.Code, rec.Header().Get("RateLimit-Limit"), rec.Header().Get("RateLimit-Remaining"))
	}
	if got := do(t, h, http.MethodPost, "/graphql", "token-a", gqlBody(`{ __typename }`)).Header().Get("RateLimit-Remaining"); got != "499" {
		t.Fatalf("general remaining after an MCP read = %q, want 499: the tool gave its general token back", got)
	}
	deploy := `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"trigger_deploy","arguments":{"serviceId":"srv-m","workspaceId":"tea-a"}}}`
	for i := range 10 {
		if body := mcpHTTPCall(t, h, "token-a", deploy).Body.String(); strings.Contains(body, "rate_limited") {
			t.Fatalf("trigger_deploy %d was rate limited: %s", i+1, body)
		}
	}
	if body := mcpHTTPCall(t, h, "token-c", deploy).Body.String(); !strings.Contains(body, "rate_limited: rate limit exceeded") {
		t.Fatalf("11th trigger_deploy of srv-m = %s, want a tool error coded rate_limited", body)
	}
	if got := do(t, h, http.MethodPost, "/v1/services/srv-m/deploys", "token-a", `{}`).Code; got != http.StatusTooManyRequests {
		t.Fatalf("REST deploy of srv-m after 10 MCP deploys = %d, want 429", got)
	}
	rec = mcpHTTPCall(t, h, "token-a", `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"list_projects","arguments":{"workspaceId":"tea-a"}}}`)
	if rec.Header().Get("RateLimit-Limit") != "500" {
		t.Fatalf("bex-only list_projects RateLimit-Limit = %q, want the general 500", rec.Header().Get("RateLimit-Limit"))
	}
}

// BEX_RATE_LIMIT=0 turns the rows off with the general limiter.
func TestRenderRateLimitsAreOffWithTheGeneralLimiter(t *testing.T) {
	srv := NewServer(&core.Base{Client: fakeClient(), Namespace: "default"}, Deps{DeployStore: deployHookStore{}})
	srv.LimitRenderOperations()
	if srv.RenderRateLimits != nil || srv.Deploys.ServiceDeployRate != nil {
		t.Fatal("Render rows are wired without a general limiter")
	}
}

// The REST mount asks the sandbox table first: a sandbox create spends the
// sandbox create class (100), not Render's generic write row (30).
func TestTheSandboxTableComesBeforeRendersRows(t *testing.T) {
	srv := NewServer(&core.Base{Client: fakeClient(), Namespace: "default"}, Deps{})
	srv.HydraAdminURL = fakeHydraURL(t)
	srv.RateLimiter = NewRateLimiter(500, 0)
	srv.RateLimiter.Operations = sandbox.NewRateLimits(nil)
	srv.LimitRenderOperations()
	muxes, err := srv.composedMuxes()
	if err != nil {
		t.Fatal(err)
	}
	contract, err := renderContractOnce()
	if err != nil {
		t.Fatal(err)
	}
	h := srv.restRateLimitMiddleware(muxes.rest, contract)(ok200)
	id := core.Identity{Subject: "key-a", ClientID: "key-a", Method: core.MethodOAuth2}
	for path, want := range map[string]string{"/v1/sandboxes": "100", "/v1/env-groups": "30"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, reqWith(http.MethodPost, path, id))
		if got := rec.Header().Get("RateLimit-Limit"); got != want {
			t.Errorf("POST %s RateLimit-Limit = %q, want %s", path, got, want)
		}
	}
}

// Every Render MCP tool bex serves has a row, the table names only Render's
// tools, and a tool Render marks read-only takes a read row.
func TestEveryRenderMCPToolHasARow(t *testing.T) {
	pin, err := loadRenderMCPContract()
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range pin.Tools {
		if _, upstreamOnly := mcpKnownUpstreamOnly[tool.Name]; upstreamOnly {
			if _, ok := renderMCPToolRows[tool.Name]; ok {
				t.Errorf("%s is not served but has a row", tool.Name)
			}
			continue
		}
		row, ok := renderMCPToolRows[tool.Name]
		if !ok {
			t.Errorf("Render MCP tool %s has no row in renderMCPToolRows", tool.Name)
			continue
		}
		if tool.Annotations["readOnlyHint"] && row != renderRowRead && row != renderRowLogs {
			t.Errorf("read-only %s takes the %s row", tool.Name, row)
		}
	}
	for name := range renderMCPToolRows {
		if _, ok := pin.Tool(name); !ok {
			t.Errorf("renderMCPToolRows names %s, which Render's MCP server does not register", name)
		}
	}
}

// Every operation with a row of its own is an operation of the pinned spec.
func TestRenderOperationRowsNameRealOperations(t *testing.T) {
	contract, err := renderContractOnce()
	if err != nil {
		t.Fatal(err)
	}
	ids := map[string]bool{}
	for _, item := range contract.document.Paths.Map() {
		for _, op := range item.Operations() {
			ids[op.OperationID] = true
		}
	}
	for id := range renderOperationRows {
		if !ids[id] {
			t.Errorf("renderOperationRows names %q, which the pinned spec does not have", id)
		}
	}
}
