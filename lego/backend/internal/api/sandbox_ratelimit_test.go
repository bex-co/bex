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
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/sandbox"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway/gatewaytest"
)

// w22/m443: sandbox operations are rate limited per user and per operation at
// Render's documented rates, through the composed server: the auth gate, the
// general limiter, the Render request validator and the three surfaces.

// rateKeys maps each test bearer to the API key (OAuth client) it is, and
// rateKeyCreators each key to the user who created it: key-a and key-b are one
// user's, key-c another's.
var (
	rateKeys        = map[string]string{"token-a": "key-a", "token-b": "key-b", "token-c": "key-c"}
	rateKeyCreators = keyCreators{"key-a": "identity-1", "key-b": "identity-1", "key-c": "identity-2"}
)

type keyCreators map[string]string

func (k keyCreators) APIKeyCreator(_ context.Context, clientID string) (string, bool, error) {
	creator, ok := k[clientID]
	return creator, ok, nil
}

// rateKeysHydra introspects each bearer of rateKeys as its API key.
func rateKeysHydra(t *testing.T) string {
	t.Helper()
	hydra := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		w.Header().Set("Content-Type", "application/json")
		client, ok := rateKeys[r.PostFormValue("token")]
		if r.URL.Path != "/admin/oauth2/introspect" || !ok {
			_, _ = io.WriteString(w, `{"active":false}`)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"active": true, "sub": client, "client_id": client})
	}))
	t.Cleanup(hydra.Close)
	return hydra.URL
}

// rateLimitedSandboxServer is the composed server over a fake OpenSandbox that
// answers every create, read and terminate, with the general limiter at
// generalPerMinute on a clock that does not move, so a bucket never refills
// during a test. generalPerMinute 0 is BEX_RATE_LIMIT=0.
func rateLimitedSandboxServer(t *testing.T, generalPerMinute float64) (*Server, http.Handler, *atomic.Int32) {
	t.Helper()
	var created atomic.Int32
	// The fake keeps the first few sandboxes it creates, enough for a test to
	// read, exec in and terminate one of its own; a list stays one small page.
	var mu sync.Mutex
	var kept []map[string]any
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/sandboxes":
			var req struct {
				Metadata map[string]string `json:"metadata"`
			}
			_ = json.NewDecoder(r.Body).Decode(&req)
			out := map[string]any{"id": fmt.Sprintf("os-%d", created.Add(1)), "status": map[string]string{"state": "Running"}, "metadata": req.Metadata}
			if len(kept) < 4 {
				kept = append(kept, out)
			}
			_ = json.NewEncoder(w).Encode(out)
		case r.Method == http.MethodGet && r.URL.Path == "/sandboxes":
			_ = json.NewEncoder(w).Encode(kept)
		case r.Method == http.MethodGet:
			for _, box := range kept {
				if "/sandboxes/"+box["id"].(string) == r.URL.Path {
					_ = json.NewEncoder(w).Encode(box)
					return
				}
			}
			http.NotFound(w, r)
		default:
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	t.Cleanup(upstream.Close)

	workspace := fakeWorkspace{"key-a": "tea-a", "key-b": "tea-a", "key-c": "tea-a"}
	srv := NewServer(&core.Base{
		Client: fakeClient(sampleApp("web")), Namespace: "default", Workspace: workspace, Authz: &fakeChecker{allow: true},
	}, Deps{
		SandboxClient:          sandbox.NewClient(upstream.URL),
		SandboxTemplates:       map[string]sandbox.Template{"base": {Image: "alpine:3", CPU: "500m", Memory: "512Mi"}},
		SandboxDefaultTemplate: "base",
		SandboxDefaultPlan:     sandbox.PlanStarter,
	})
	srv.HydraAdminURL = rateKeysHydra(t)
	srv.RateLimiter = NewRateLimiter(generalPerMinute, 0)
	if srv.RateLimiter != nil {
		now := time.Date(2026, 11, 17, 12, 0, 0, 0, time.UTC)
		srv.RateLimiter.Workspace = workspace
		srv.RateLimiter.now = func() time.Time { return now }
		srv.LimitSandboxOperations()
		srv.Sandbox.RateLimits.Keys = rateKeyCreators
		srv.Sandbox.RateLimits.Now = srv.RateLimiter.now
	} else {
		srv.LimitSandboxOperations()
	}
	handler, err := srv.Handler()
	if err != nil {
		t.Fatal(err)
	}
	return srv, handler, &created
}

// send answers how many of n identical requests were refused by a rate limit,
// and fails on any other answer that is not wantStatus.
func send(t *testing.T, h http.Handler, n int, method, path, token, body string, wantStatus int) (limited int) {
	t.Helper()
	for i := range n {
		rec := do(t, h, method, path, token, body)
		switch rec.Code {
		case wantStatus:
		case http.StatusTooManyRequests:
			limited++
		default:
			t.Fatalf("%s %s #%d = %d %s, want %d", method, path, i+1, rec.Code, rec.Body.String(), wantStatus)
		}
	}
	return limited
}

const rateCreateBody = `{"ownerId":"tea-a"}`

// rateSandbox creates one sandbox as token's caller and returns its id. It
// spends one create of that caller's user.
func rateSandbox(t *testing.T, h http.Handler, token string) string {
	t.Helper()
	rec := do(t, h, http.MethodPost, "/v1/sandboxes", token, rateCreateBody)
	var box struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &box); err != nil || rec.Code != http.StatusCreated || box.ID == "" {
		t.Fatalf("create = %d %s", rec.Code, rec.Body.String())
	}
	return box.ID
}

// The definition of done's first line: one user, two API keys, the 101st
// create inside a minute is 429 and the first 100 succeed; another user has
// their own 100. The 429 is the general limiter's, but for the wait its
// message names.
func TestSandboxCreateIsRefusedAtThe101stAcrossOneUsersAPIKeys(t *testing.T) {
	_, h, created := rateLimitedSandboxServer(t, 500)
	if n := send(t, h, 60, http.MethodPost, "/v1/sandboxes", "token-a", rateCreateBody, http.StatusCreated); n != 0 {
		t.Fatalf("%d of the first key's 60 creates were rate limited", n)
	}
	if n := send(t, h, 40, http.MethodPost, "/v1/sandboxes", "token-b", rateCreateBody, http.StatusCreated); n != 0 {
		t.Fatalf("%d of the second key's 40 creates were rate limited", n)
	}
	refused := do(t, h, http.MethodPost, "/v1/sandboxes", "token-b", rateCreateBody)
	general := generalRefusal(t)
	retry, _ := strconv.Atoi(refused.Header().Get("Retry-After"))
	want := strings.Replace(general.Body.String(), core.RateLimitExceededMessage(60), core.RateLimitExceededMessage(retry), 1)
	if refused.Code != http.StatusTooManyRequests || refused.Body.String() != want {
		t.Fatalf("101st create = %d %s, want the general limiter's 429 %s", refused.Code, refused.Body.String(), want)
	}
	if got := refused.Header().Get("Content-Type"); got != general.Header().Get("Content-Type") {
		t.Fatalf("Content-Type = %q, want %q", got, general.Header().Get("Content-Type"))
	}
	for header, want := range map[string]string{"Retry-After": "1", "RateLimit-Limit": "100", "RateLimit-Remaining": "0", "RateLimit-Reset": "60"} {
		if got := refused.Header().Get(header); got != want {
			t.Errorf("%s = %q, want %q", header, got, want)
		}
		if general.Header().Get(header) == "" {
			t.Errorf("the general limiter's 429 has no %s to match", header)
		}
	}
	if got := created.Load(); got != 100 {
		t.Fatalf("OpenSandbox saw %d creates, want 100: a refused create must not reach it", got)
	}
	if n := send(t, h, 100, http.MethodPost, "/v1/sandboxes", "token-c", rateCreateBody, http.StatusCreated); n != 0 {
		t.Fatalf("%d of another user's 100 creates were rate limited", n)
	}
	// An accepted sandbox call reports its class's bucket, as Render's
	// RateLimit headers do.
	read := do(t, h, http.MethodGet, "/v1/sandboxes?ownerId=tea-a", "token-a", "")
	if read.Code != http.StatusOK || read.Header().Get("RateLimit-Limit") != "400" || read.Header().Get("RateLimit-Remaining") != "399" {
		t.Fatalf("read = %d, RateLimit-Limit %q, RateLimit-Remaining %q", read.Code, read.Header().Get("RateLimit-Limit"), read.Header().Get("RateLimit-Remaining"))
	}
}

// generalRefusal is the 429 the general limiter writes for a REST request.
func generalRefusal(t *testing.T) *httptest.ResponseRecorder {
	t.Helper()
	rl := NewRateLimiter(1, 1)
	h := rl.Middleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/v1/services", nil))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/services", nil))
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("general limiter = %d", rec.Code)
	}
	return rec
}

// Each class has its own bucket, a sandbox call spends none of the general
// budget, and a call that is not a sandbox operation spends no sandbox bucket.
func TestSandboxClassesAndTheGeneralBudgetAreSeparate(t *testing.T) {
	_, h, _ := rateLimitedSandboxServer(t, 500)
	group := "sbg-" + strings.Repeat("a", 20)
	box := "/v1/sandboxes/" + rateSandbox(t, h, "token-a")

	// 400 reads, then the 401st is refused. With the 100 creates below that is
	// 500 sandbox calls: the whole of what the general budget used to allow.
	if n := send(t, h, 401, http.MethodGet, box+"?ownerId=tea-a", "token-a", "", http.StatusOK); n != 1 {
		t.Fatalf("%d of 401 reads were rate limited, want exactly the 401st", n)
	}
	if n := send(t, h, 100, http.MethodPost, "/v1/sandboxes", "token-a", rateCreateBody, http.StatusCreated); n != 1 {
		t.Fatalf("%d of creates 2 to 101 were rate limited, want exactly the 101st", n)
	}
	// A terminate is still accepted after those 500, and 600 execs in a minute
	// are all accepted (the second line of the definition of done). This
	// server has no exec gateway, so each mint answers its named 503.
	if n := send(t, h, 1, http.MethodPost, box+"/terminate?ownerId=tea-a", "token-a", "", http.StatusNoContent); n != 0 {
		t.Fatal("a terminate was rate limited after 500 other sandbox calls")
	}
	if n := send(t, h, 600, http.MethodPost, box+"/runs/sync/token?ownerId=tea-a", "token-a", `{"command":"true"}`, http.StatusServiceUnavailable); n != 0 {
		t.Fatalf("%d of 600 execs in a minute were rate limited", n)
	}
	// 399 more make 1,000 of each; the next of each is refused.
	if n := send(t, h, 400, http.MethodPost, box+"/exec?ownerId=tea-a", "token-b", `{"command":"true"}`, http.StatusServiceUnavailable); n != 0 {
		t.Fatalf("%d execs were rate limited inside the 1,000", n)
	}
	if n := send(t, h, 1, http.MethodPost, box+"/exec?ownerId=tea-a", "token-b", `{"command":"true"}`, http.StatusServiceUnavailable); n != 1 {
		t.Fatal("the 1,001st exec was not rate limited: the mint and the single-step exec must share the exec class")
	}
	if n := send(t, h, 1001, http.MethodPost, box+"/files/upload/token?ownerId=tea-a&path=/tmp/x", "token-a", "", http.StatusServiceUnavailable); n != 1 {
		t.Fatalf("%d of 1,001 file transfers were rate limited, want exactly the last", n)
	}
	if n := send(t, h, 1000, http.MethodPost, box+"/terminate?ownerId=tea-a", "token-a", "", http.StatusNoContent); n != 1 {
		t.Fatalf("%d of terminates 2 to 1,001 were rate limited, want exactly the last", n)
	}
	// Group writes: 100, then refused. No store here, so each is a named 503.
	if n := send(t, h, 101, http.MethodPatch, "/v1/sandbox-groups/"+group, "token-a", `{"concurrencyLimit":5}`, http.StatusServiceUnavailable); n != 1 {
		t.Fatalf("%d of 101 group writes were rate limited, want exactly the 101st", n)
	}
	// Captures: 10, then refused.
	if n := send(t, h, 11, http.MethodPost, box+"/snapshots?ownerId=tea-a", "token-a", `{"kind":"filesystem"}`, http.StatusServiceUnavailable); n != 1 {
		t.Fatalf("%d of 11 captures were rate limited, want exactly the 11th", n)
	}

	// Thousands of sandbox calls later the general budget is whole: 500
	// other calls pass and the 501st is refused.
	if n := send(t, h, 501, http.MethodGet, "/v1/services", "token-a", "", http.StatusOK); n != 1 {
		t.Fatalf("%d of 501 general calls were rate limited, want exactly the 501st", n)
	}
	// The brokered-credential routes are named 404s on the general budget.
	if got := do(t, h, http.MethodGet, "/v1/sandbox-groups/"+group+"/credentials", "token-a", "").Code; got != http.StatusTooManyRequests {
		t.Fatalf("credentials route = %d, want the spent general budget's 429", got)
	}
	// And a spent general budget refuses no sandbox operation: the other
	// user's classes are untouched by all of the above, and key-c's general
	// budget is its own.
	if n := send(t, h, 400, http.MethodGet, "/v1/sandboxes?ownerId=tea-a", "token-c", "", http.StatusOK); n != 0 {
		t.Fatalf("%d of another user's reads were rate limited", n)
	}
}

// With the general budget spent, a user's sandbox calls still pass: the two
// budgets are separate in that direction too.
func TestASpentGeneralBudgetDoesNotRefuseSandboxCalls(t *testing.T) {
	_, h, _ := rateLimitedSandboxServer(t, 3)
	if n := send(t, h, 4, http.MethodGet, "/v1/services", "token-a", "", http.StatusOK); n != 1 {
		t.Fatalf("%d of 4 general calls were limited at 3 a minute", n)
	}
	if n := send(t, h, 50, http.MethodGet, "/v1/sandboxes?ownerId=tea-a", "token-a", "", http.StatusOK); n != 0 {
		t.Fatalf("%d sandbox reads were refused by a spent general budget", n)
	}
}

func gqlBody(query string) string {
	body, _ := json.Marshal(map[string]string{"query": query})
	return string(body)
}

// REST, GraphQL and MCP spend one bucket: 60 creates on REST and 40 on
// GraphQL leave none for MCP, and each surface refuses in its own shape with
// the general limiter's code.
func TestSandboxCreateSharesOneBucketAcrossRESTGraphQLAndMCP(t *testing.T) {
	srv, h, created := rateLimitedSandboxServer(t, 500)
	if n := send(t, h, 60, http.MethodPost, "/v1/sandboxes", "token-a", rateCreateBody, http.StatusCreated); n != 0 {
		t.Fatalf("%d REST creates were rate limited", n)
	}
	create := gqlBody(`mutation { createSandbox(ownerId:"tea-a") { id } }`)
	for i := range 40 {
		rec := do(t, h, http.MethodPost, "/graphql", "token-b", create)
		if rec.Code != http.StatusOK || strings.Contains(rec.Body.String(), `"errors"`) {
			t.Fatalf("GraphQL create %d = %d %s", i+1, rec.Code, rec.Body.String())
		}
	}
	// The user's own API key over MCP: the 101st create.
	cs := mcpSessionIdentity(t, srv, core.Identity{Subject: "key-a", ClientID: "key-a", Method: core.MethodOAuth2})
	res, err := cs.CallTool(t.Context(), &mcp.CallToolParams{Name: "spawn_sandbox", Arguments: map[string]any{"workspaceId": "tea-a"}})
	if err != nil {
		t.Fatal(err)
	}
	text, _ := res.Content[0].(*mcp.TextContent)
	if !res.IsError || text == nil || !strings.HasPrefix(text.Text, "rate_limited: rate limit exceeded") {
		t.Fatalf("MCP create after 100 = %+v, want a tool error coded rate_limited", res.Content)
	}
	// GraphQL refuses with the general limiter's code, and the HTTP response
	// carries Retry-After and Render's RateLimit headers.
	rec := do(t, h, http.MethodPost, "/graphql", "token-a", create)
	var body struct {
		Errors []struct {
			Message    string         `json:"message"`
			Extensions map[string]any `json:"extensions"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || len(body.Errors) != 1 || body.Errors[0].Extensions["code"] != "RATE_LIMITED" {
		t.Fatalf("GraphQL create after 100 = %d %s, want one error coded RATE_LIMITED", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("Retry-After") != "1" || rec.Header().Get("RateLimit-Limit") != "100" || rec.Header().Get("RateLimit-Remaining") != "0" {
		t.Fatalf("GraphQL refusal headers = %v", rec.Header())
	}
	// And REST refuses too.
	if got := do(t, h, http.MethodPost, "/v1/sandboxes", "token-b", rateCreateBody).Code; got != http.StatusTooManyRequests {
		t.Fatalf("REST create after 100 = %d", got)
	}
	if got := created.Load(); got != 100 {
		t.Fatalf("OpenSandbox saw %d creates, want 100", got)
	}
	// Another user's bucket is their own on GraphQL as on REST.
	if rec := do(t, h, http.MethodPost, "/graphql", "token-c", create); strings.Contains(rec.Body.String(), `"errors"`) {
		t.Fatalf("another user's GraphQL create: %s", rec.Body.String())
	}
}

// A GraphQL or MCP request made only of sandbox operations spends none of the
// general budget, as the same operation on REST; a document that mixes in
// another field pays the general budget for it.
func TestGraphQLAndMCPSandboxRequestsLeaveTheGeneralBudget(t *testing.T) {
	_, h, _ := rateLimitedSandboxServer(t, 3)
	box := rateSandbox(t, h, "token-a")
	list := gqlBody(`{ sandboxes(ownerId:"tea-a") { id } sandbox(id:"` + box + `", ownerId:"tea-a") { id } }`)
	for i := range 20 {
		rec := do(t, h, http.MethodPost, "/graphql", "token-a", list)
		if rec.Code != http.StatusOK || strings.Contains(rec.Body.String(), `"errors"`) {
			t.Fatalf("sandbox-only GraphQL request %d = %d %s", i+1, rec.Code, rec.Body.String())
		}
	}
	call := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"list_sandboxes","arguments":{"workspaceId":"tea-a"}}}`
	for i := range 20 {
		r := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(call))
		r.Header.Set("Authorization", "Bearer token-a")
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Accept", "application/json, text/event-stream")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, r)
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), box) {
			t.Fatalf("MCP sandbox call %d = %d %s", i+1, rec.Code, rec.Body.String())
		}
	}
	// 40 fields and 20 tool calls were 60 reads.
	read := do(t, h, http.MethodGet, "/v1/sandboxes?ownerId=tea-a", "token-a", "")
	if got := read.Header().Get("RateLimit-Remaining"); got != "339" {
		t.Fatalf("reads remaining = %q, want 339 of 400 after 61", got)
	}
	// The general budget is still whole: three mixed documents pass, and the
	// fourth is the general limiter's own GraphQL 429.
	mixed := gqlBody(`{ sandboxes(ownerId:"tea-a") { id } services { id } }`)
	if n := send(t, h, 4, http.MethodPost, "/graphql", "token-a", mixed, http.StatusOK); n != 1 {
		t.Fatalf("%d of 4 mixed documents were limited at a general 3 a minute, want exactly the 4th", n)
	}
}

// Each member of a GraphQL batch pays a general token of its own (ADR006) and
// is judged alone: a sandbox-only member gives its token back, and a mixed
// member beside it keeps paying.
func TestAGraphQLBatchRefundsEachSandboxOnlyMember(t *testing.T) {
	_, h, _ := rateLimitedSandboxServer(t, 3)
	sandboxOnly := gqlBody(`{ sandboxes(ownerId:"tea-a") { id } }`)
	mixed := gqlBody(`{ sandboxes(ownerId:"tea-a") { id } services { id } }`)
	batch := "[" + sandboxOnly + "," + sandboxOnly + "," + sandboxOnly + "]"
	for i := range 10 {
		rec := do(t, h, http.MethodPost, "/graphql", "token-a", batch)
		if rec.Code != http.StatusOK || strings.Contains(rec.Body.String(), `"errors"`) {
			t.Fatalf("sandbox-only batch %d = %d %s", i+1, rec.Code, rec.Body.String())
		}
	}
	// 30 reads were spent, and the general 3 are whole: a batch of one
	// sandbox-only and two mixed members takes all three and returns one.
	rec := do(t, h, http.MethodPost, "/graphql", "token-a", "["+mixed+","+sandboxOnly+","+mixed+"]")
	if rec.Code != http.StatusOK || strings.Contains(rec.Body.String(), `"errors"`) {
		t.Fatalf("mixed batch = %d %s", rec.Code, rec.Body.String())
	}
	if n := send(t, h, 2, http.MethodPost, "/graphql", "token-a", mixed, http.StatusOK); n != 1 {
		t.Fatalf("%d of 2 mixed documents were limited after a batch that kept 2 of the general 3, want exactly the 2nd", n)
	}
	read := do(t, h, http.MethodGet, "/v1/sandboxes?ownerId=tea-a", "token-a", "")
	if got := read.Header().Get("RateLimit-Remaining"); got != "365" {
		t.Fatalf("reads remaining = %q, want 365 of 400 after 35", got)
	}
}

// BEX_RATE_LIMIT=0 turns the sandbox limits off with the general limiter.
func TestSandboxRateLimitsAreOffWithTheGeneralLimiter(t *testing.T) {
	srv, h, _ := rateLimitedSandboxServer(t, 0)
	if srv.Sandbox.RateLimits != nil {
		t.Fatal("sandbox limits are wired without a general limiter")
	}
	if n := send(t, h, 150, http.MethodPost, "/v1/sandboxes", "token-a", rateCreateBody, http.StatusCreated); n != 0 {
		t.Fatalf("%d creates were rate limited with the limiter off", n)
	}
}

// Every sandbox operation the server mounts has a class, and the class table
// names nothing the server does not mount.
func TestEveryLiveSandboxOperationHasARateClass(t *testing.T) {
	classes := sandbox.RateClasses()
	live := liveScopeOperations(t)
	isSandbox := func(operation string) bool {
		surface, name, _ := strings.Cut(operation, " ")
		if surface == "REST" {
			_, path, _ := strings.Cut(name, " ")
			return path == "/v1/sandboxes" || strings.HasPrefix(path, "/v1/sandboxes/") ||
				path == "/v1/sandbox-groups" || strings.HasPrefix(path, "/v1/sandbox-groups/")
		}
		return strings.Contains(strings.ToLower(name), "sandbox")
	}
	sandboxOperations := 0
	for _, operation := range live {
		if !isSandbox(operation) {
			if _, ok := classes[operation]; ok {
				t.Errorf("%s has a sandbox rate class but is not a sandbox operation", operation)
			}
			continue
		}
		sandboxOperations++
		if _, classified := classes[operation]; !classified {
			t.Errorf("%s is a sandbox operation with no rate class (ADR104 D7)", operation)
		}
	}
	if sandboxOperations < len(classes) {
		t.Fatalf("found %d live sandbox operations, fewer than the %d classified: the enumerator looks broken", sandboxOperations, len(classes))
	}
	for operation := range classes {
		if !slices.Contains(live, operation) {
			t.Errorf("%s has a rate class but the server does not mount it", operation)
		}
	}
}

// The redeem routes of the exec and file connect tokens are public, and an
// IP-keyed budget used to count every redeem: 60 a minute with a burst of 10,
// far below the 1,000 execs a minute the mint allows. Only a token that does
// not redeem spends it now.
func TestARedeemedConnectTokenSpendsNoIPBudgetAndARefusedOneDoes(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "os-1", "status": map[string]string{"state": "Running"}, "metadata": map[string]string{
			"bex.co/owner": "identity-1", "bex.co/workspace": "tea-a", "app.bex.co/regime": "sandbox", "bex.co/network-policy": "deny-all",
		}})
	}))
	t.Cleanup(upstream.Close)
	srv := NewServer(&core.Base{Client: fakeClient(sampleApp("web")), Namespace: "default", Workspace: fakeWorkspace{"identity-1": "tea-a"}, Authz: &fakeChecker{allow: true}}, Deps{
		SandboxClient: sandbox.NewClient(upstream.URL),
		SandboxExec:   &sandbox.ExecConfig{Secret: []byte("exec-secret"), GatewayURL: "http://127.0.0.1:1", FileGatewayURL: "http://127.0.0.1:1/sandbox-files", TTL: time.Minute, Nonces: &sshgateway.NonceGuard{Store: &gatewaytest.FakeStore{}}},
	})
	srv.HydraAdminURL = fakeHydraURL(t)
	now := time.Date(2026, 11, 17, 12, 0, 0, 0, time.UTC)
	srv.SandboxConnectRateLimiter = NewRateLimiter(60, 2)
	srv.SandboxConnectRateLimiter.now = func() time.Time { return now }
	h, err := srv.Handler()
	if err != nil {
		t.Fatal(err)
	}
	ctx := core.WithIdentity(t.Context(), core.Identity{Subject: "identity-1", Method: core.MethodOAuth2})
	redeem := func(token string) int {
		t.Helper()
		minted, err := srv.Sandbox.ConnectRun(ctx, sandbox.ConnectRequest{OwnerID: "tea-a", SandboxID: "os-1", Operation: sandbox.ConnectOperationStream, Command: "true"}, "http://api.test")
		if err != nil {
			t.Fatalf("ConnectRun: %v", err)
		}
		if token == "" {
			token = minted.Token
		}
		return do(t, h, minted.Method, strings.TrimPrefix(minted.URI, "http://api.test"), token, `{"command":"true"}`).Code
	}
	// Twenty real tokens, ten times the budget's burst: each passes the token
	// check (this server's exec gateway is unreachable, so the exec itself
	// fails) and none is rate limited.
	for i := range 20 {
		switch code := redeem(""); code {
		case http.StatusTooManyRequests, http.StatusUnauthorized, http.StatusForbidden, http.StatusConflict, http.StatusGone:
			t.Fatalf("redeem %d of a freshly minted token = %d", i+1, code)
		}
	}
	// Two tokens that do not redeem spend the budget of two; the third request
	// from that address is shed before the token is looked at, a real one too.
	for i := range 2 {
		if code := redeem("not-a-connect-token"); code != http.StatusUnauthorized {
			t.Fatalf("bad token %d = %d, want 401", i+1, code)
		}
	}
	if code := redeem("not-a-connect-token"); code != http.StatusTooManyRequests {
		t.Fatalf("third bad token = %d, want 429", code)
	}
	if code := redeem(""); code != http.StatusTooManyRequests {
		t.Fatalf("a redeem from an address whose budget is spent = %d, want 429", code)
	}
	// A second later the budget has one token again.
	now = now.Add(time.Second)
	if code := redeem(""); code == http.StatusTooManyRequests {
		t.Fatal("still rate limited after the budget refilled")
	}
}

func TestRefusedCredentialMiddlewareChargesOnlyRefusals(t *testing.T) {
	now := time.Date(2026, 11, 17, 12, 0, 0, 0, time.UTC)
	rl := NewRateLimiter(60, 1)
	rl.now = func() time.Time { return now }
	status := http.StatusOK
	h := rl.RefusedCredentialMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(status) }))
	serve := func() *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/sandboxes/sbx-1/runs/exe-1/stream", nil))
		return rec
	}
	for _, free := range []int{http.StatusOK, http.StatusCreated, http.StatusBadRequest, http.StatusNotFound, http.StatusTooManyRequests, http.StatusBadGateway, http.StatusServiceUnavailable} {
		status = free
		for range 3 {
			if got := serve().Code; got != free {
				t.Fatalf("a %d answer spent the refusal budget: next request = %d", free, got)
			}
		}
	}
	for _, charged := range []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusConflict, http.StatusGone} {
		status = charged
		if got := serve().Code; got != charged {
			t.Fatalf("first %d = %d", charged, got)
		}
		rec := serve()
		if rec.Code != http.StatusTooManyRequests || rec.Header().Get("Retry-After") != "1" {
			t.Fatalf("after a %d the next request = %d (Retry-After %q), want 429 and 1", charged, rec.Code, rec.Header().Get("Retry-After"))
		}
		now = now.Add(time.Second)
	}
}
