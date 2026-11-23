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

package sandbox

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// renderRateTable is render.com/docs/sandboxes-limits § API rate limits as
// read on 2026-11-17, written out a second time so a changed constant fails
// here instead of moving the expectation with it.
var renderRateTable = map[RateClass]int{
	RateClassCreate:     100,
	RateClassTerminate:  1000,
	RateClassExec:       1000,
	RateClassFiles:      1000,
	RateClassRead:       400,
	RateClassGroupWrite: 100,
	RateClassCapture:    10,
}

// frozenLimits are limits on a clock the test moves.
func frozenLimits(keys core.KeyCreatorResolver) (*RateLimits, *time.Time) {
	now := time.Date(2026, 11, 17, 12, 0, 0, 0, time.UTC)
	limits := NewRateLimits(keys)
	limits.Now = func() time.Time { return now }
	return limits, &now
}

func asUser(subject string) context.Context {
	return core.WithIdentity(context.Background(), core.Identity{Subject: subject, Method: core.MethodSession})
}

func asKey(clientID string) context.Context {
	return core.WithIdentity(context.Background(), core.Identity{Subject: clientID, ClientID: clientID, Method: core.MethodOAuth2})
}

// spend charges class n times and returns how many were admitted.
func spend(limits *RateLimits, ctx context.Context, class RateClass, n int) int {
	admitted := 0
	for range n {
		if limits.charge(ctx, class, mcpRateLimitedCode, false) == nil {
			admitted++
		}
	}
	return admitted
}

func TestEveryClassAdmitsRendersRateThenRefusesAndRefills(t *testing.T) {
	if len(rateClassPerMinute) != len(renderRateTable) {
		t.Fatalf("classes = %v, want Render's seven", rateClassPerMinute)
	}
	for class, perMinute := range renderRateTable {
		t.Run(string(class), func(t *testing.T) {
			limits, now := frozenLimits(nil)
			ctx := asUser("identity-1")
			if got := spend(limits, ctx, class, perMinute+1); got != perMinute {
				t.Fatalf("admitted %d of %d in one instant, want exactly %d", got, perMinute+1, perMinute)
			}
			err := limits.charge(ctx, class, gqlRateLimitedCode, false)
			var coded *core.CodedError
			if !errors.As(err, &coded) || coded.Code != gqlRateLimitedCode || !errors.Is(err, core.ErrTooManyRequests) {
				t.Fatalf("refusal = %v, want a 429 coded %s", err, gqlRateLimitedCode)
			}
			if seconds, _ := coded.Params["retryAfterSeconds"].(int); seconds < 1 {
				t.Fatalf("params = %v, want the seconds to wait", coded.Params)
			}
			// A minute later the bucket holds the whole rate again, and no more.
			*now = now.Add(time.Minute)
			if got := spend(limits, ctx, class, perMinute+1); got != perMinute {
				t.Fatalf("admitted %d a minute later, want %d", got, perMinute)
			}
		})
	}
}

func TestAClassSpendsOnlyItsOwnBucket(t *testing.T) {
	limits, _ := frozenLimits(nil)
	ctx := asUser("identity-1")
	if got := spend(limits, ctx, RateClassCreate, 101); got != 100 {
		t.Fatalf("creates admitted = %d, want 100", got)
	}
	for class, perMinute := range renderRateTable {
		if class == RateClassCreate {
			continue
		}
		if got := spend(limits, ctx, class, perMinute); got != perMinute {
			t.Errorf("%s admitted %d after the create class was spent, want its full %d", class, got, perMinute)
		}
	}
}

type fakeKeyCreators struct {
	creators map[string]string
	fail     bool
}

func (f *fakeKeyCreators) APIKeyCreator(_ context.Context, clientID string) (string, bool, error) {
	if f.fail {
		return "", false, errors.New("hydra is down")
	}
	creator, ok := f.creators[clientID]
	return creator, ok, nil
}

func TestAUsersKeysAndSessionShareOneBucketAndAnotherUserDoesNot(t *testing.T) {
	keys := &fakeKeyCreators{creators: map[string]string{"key-a": "identity-1", "key-b": "identity-1", "key-c": "identity-2"}}
	limits, _ := frozenLimits(keys)
	if got := spend(limits, asKey("key-a"), RateClassCreate, 60); got != 60 {
		t.Fatalf("first key admitted %d, want 60", got)
	}
	if got := spend(limits, asKey("key-b"), RateClassCreate, 30); got != 30 {
		t.Fatalf("second key admitted %d, want 30", got)
	}
	// The user's own session spends the same bucket: 10 are left.
	if got := spend(limits, asUser("identity-1"), RateClassCreate, 20); got != 10 {
		t.Fatalf("the user's session admitted %d, want the 10 the two keys left", got)
	}
	if got := spend(limits, asKey("key-b"), RateClassCreate, 1); got != 0 {
		t.Fatal("the 101st create of one user was admitted through a second key")
	}
	if got := spend(limits, asKey("key-c"), RateClassCreate, 100); got != 100 {
		t.Fatalf("another user's key admitted %d, want its own 100", got)
	}
}

func TestAKeyWithNoKnownCreatorIsItsOwnCaller(t *testing.T) {
	for name, keys := range map[string]core.KeyCreatorResolver{
		"no lookup wired":   nil,
		"lookup failing":    &fakeKeyCreators{fail: true},
		"no creator stamp":  &fakeKeyCreators{creators: map[string]string{"key-a": "", "key-b": ""}},
		"not a bex API key": &fakeKeyCreators{},
	} {
		t.Run(name, func(t *testing.T) {
			limits, _ := frozenLimits(keys)
			if got := spend(limits, asKey("key-a"), RateClassCapture, 11); got != 10 {
				t.Fatalf("admitted %d, want 10", got)
			}
			if got := spend(limits, asKey("key-b"), RateClassCapture, 11); got != 10 {
				t.Fatalf("a second key admitted %d, want its own 10", got)
			}
			// A key never lands in a user's bucket by its client id.
			if got := spend(limits, asUser("key-a"), RateClassCapture, 1); got != 1 {
				t.Fatal("a session shared a bucket with an API key of the same id")
			}
		})
	}
}

func TestTheWorkspaceBoundSitsAboveOneUsersWholeTable(t *testing.T) {
	sum := 0
	for _, perMinute := range renderRateTable {
		sum += perMinute
	}
	if sandboxWorkspaceRatePerMinute <= sum {
		t.Fatalf("workspace bound %d must exceed one user's table, %d a minute", sandboxWorkspaceRatePerMinute, sum)
	}
	limits, _ := frozenLimits(nil)
	scoped := func(subject string) context.Context {
		return core.WithRateLimitScope(asUser(subject), core.NewRateLimitScope("workspace:tea-a", http.Header{}, nil, false))
	}
	// One user spends every class in full and the workspace bound never
	// answers first.
	for class, perMinute := range renderRateTable {
		if got := spend(limits, scoped("identity-1"), class, perMinute); got != perMinute {
			t.Fatalf("%s admitted %d of %d under the workspace bound", class, got, perMinute)
		}
	}
	// The bound is real: members of one workspace cannot pass it together.
	admitted := sum
	for member := range 10 {
		admitted += spend(limits, scoped("member-"+string(rune('a'+member))), RateClassExec, 1000)
	}
	if admitted != sandboxWorkspaceRatePerMinute {
		t.Fatalf("a workspace's members were admitted %d sandbox operations in one instant, want the bound %d", admitted, sandboxWorkspaceRatePerMinute)
	}
}

func TestNilRateLimitsLimitNothing(t *testing.T) {
	var limits *RateLimits
	for range 2000 {
		if err := limits.charge(asUser("identity-1"), RateClassCapture, mcpRateLimitedCode, true); err != nil {
			t.Fatalf("disabled limits refused: %v", err)
		}
	}
	if limits.RequestBuckets(httptest.NewRequest(http.MethodPost, "/v1/sandboxes", nil), "") != nil {
		t.Fatal("disabled limits classified a request")
	}
}

func TestRequestBucketsClassifiesOnlySandboxRoutes(t *testing.T) {
	limits, _ := frozenLimits(nil)
	group := "sbg-" + strings.Repeat("a", 20)
	for _, tc := range []struct {
		method, path string
		class        RateClass
	}{
		{http.MethodPost, "/v1/sandboxes", RateClassCreate},
		{http.MethodPost, "/v1/sandboxes/sbx-1/resume", RateClassCreate},
		{http.MethodPost, "/v1/sandboxes/sbx-1/terminate", RateClassTerminate},
		{http.MethodPost, "/v1/sandboxes/sbx-1/pause", RateClassTerminate},
		{http.MethodPost, "/v1/sandboxes/sbx-1/exec", RateClassExec},
		{http.MethodPost, "/v1/sandboxes/sbx-1/runs/exec/token", RateClassExec},
		{http.MethodPost, "/v1/sandboxes/sbx-1/files/upload/token", RateClassFiles},
		{http.MethodPost, "/v1/sandboxes/sbx-1/snapshots", RateClassCapture},
		{http.MethodGet, "/v1/sandboxes?ownerId=tea-a", RateClassRead},
		{http.MethodGet, "/v1/sandboxes/sbx-1/execs/exe-1", RateClassRead},
		{http.MethodGet, "/v1/sandbox-groups/" + group + "/snapshots", RateClassRead},
		{http.MethodPatch, "/v1/sandbox-groups/" + group, RateClassGroupWrite},
		{http.MethodDelete, "/v1/sandbox-groups/" + group + "/builds/sbb-1", RateClassGroupWrite},
		{http.MethodDelete, "/v1/sandbox-groups/" + group + "/snapshots/snp-1", RateClassGroupWrite},
		{http.MethodGet, "/v1/sandbox-groups/" + group + "/credentials/sgc-1/versions", RateClassRead},
		{http.MethodPost, "/v1/sandbox-groups/" + group + "/credentials", RateClassGroupWrite},
		{http.MethodPost, "/v1/sandbox-groups/" + group + "/credentials/sgc-1/restore", RateClassGroupWrite},
		// Not sandbox operations: the general budget keeps them.
		{http.MethodGet, "/v1/services", ""},
		{http.MethodPost, "/graphql", ""},
		{http.MethodPost, "/mcp", ""},
		{http.MethodPut, "/v1/sandboxes/sbx-1", ""},
	} {
		r := httptest.NewRequest(tc.method, tc.path, nil).WithContext(asUser("identity-1"))
		buckets := limits.RequestBuckets(r, "workspace:tea-a")
		if tc.class == "" {
			if buckets != nil {
				t.Errorf("%s %s was classified", tc.method, tc.path)
			}
			continue
		}
		if len(buckets) != 2 {
			t.Errorf("%s %s: buckets = %d, want the class and the workspace", tc.method, tc.path, len(buckets))
			continue
		}
		if got, want := buckets[0].Burst(), renderRateTable[tc.class]; got != want {
			t.Errorf("%s %s spends a bucket of %d, want %s's %d", tc.method, tc.path, got, tc.class, want)
		}
		if got := buckets[1].Burst(); got != sandboxWorkspaceRatePerMinute {
			t.Errorf("%s %s: workspace bucket = %d", tc.method, tc.path, got)
		}
	}
	// No workspace key, no workspace bucket.
	r := httptest.NewRequest(http.MethodPost, "/v1/sandboxes", nil).WithContext(asUser("identity-1"))
	if got := limits.RequestBuckets(r, ""); len(got) != 1 {
		t.Fatalf("buckets without a workspace = %d, want 1", len(got))
	}
}

// Every operation the three surfaces mount has a class with a rate, the table
// names nothing that is not mounted, and an operation without a class cannot
// be mounted at all.
func TestEverySandboxOperationIsClassifiedAtRegistration(t *testing.T) {
	s := &Service{}
	mounted := map[string]bool{}

	mux := http.NewServeMux()
	s.RegisterREST(mux)
	for operation := range rateClasses {
		pattern, ok := strings.CutPrefix(operation, "REST ")
		if !ok {
			continue
		}
		method, path, _ := strings.Cut(pattern, " ")
		path = strings.NewReplacer("{", "", "}", "").Replace(path)
		if _, got := mux.Handler(httptest.NewRequest(method, path, nil)); got != pattern {
			t.Errorf("%s is classified but RegisterREST mounts %q there", operation, got)
		}
		mounted[operation] = true
	}
	for root, fields := range map[string]graphql.Fields{"Query": s.GraphQLQuery(), "Mutation": s.GraphQLMutation()} {
		for name := range fields {
			mounted["GQL "+root+"."+name] = true
		}
	}
	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	s.RegisterMCP(server)
	serverT, clientT := mcp.NewInMemoryTransports()
	if _, err := server.Connect(t.Context(), serverT, nil); err != nil {
		t.Fatal(err)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil).Connect(t.Context(), clientT, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	tools, err := cs.ListTools(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range tools.Tools {
		mounted["MCP "+tool.Name] = true
	}

	for operation, class := range rateClasses {
		if !mounted[operation] {
			t.Errorf("%s is classified but no surface mounts it", operation)
		}
		if _, ok := rateClassPerMinute[class]; !ok {
			t.Errorf("%s has class %q, which has no rate", operation, class)
		}
	}
	for operation := range mounted {
		if _, ok := rateClasses[operation]; !ok {
			t.Errorf("%s is mounted without a class", operation)
		}
	}

	mustPanic := func(name string, mount func()) {
		t.Helper()
		defer func() {
			if recover() == nil {
				t.Errorf("%s: an unclassified operation was mounted", name)
			}
		}()
		mount()
	}
	mustPanic("REST", func() {
		s.handle(http.NewServeMux(), "POST /v1/sandboxes/{id}/fork", func(http.ResponseWriter, *http.Request) {})
	})
	mustPanic("GraphQL", func() {
		s.limitedFields("Mutation", graphql.Fields{"forkSandbox": &graphql.Field{Type: graphql.Boolean}})
	})
	mustPanic("MCP", func() {
		addTool(s, server, &mcp.Tool{Name: "fork_sandbox"},
			func(context.Context, *mcp.CallToolRequest, sandboxIDArgs) (*mcp.CallToolResult, deleteResult, error) {
				return nil, deleteResult{}, nil
			})
	})
}

// A GraphQL document made only of sandbox fields gives the request's general
// tokens back; one that mixes in another field keeps paying for it.
func TestGraphQLRefundsTheGeneralBudgetOnlyForSandboxOnlyDocuments(t *testing.T) {
	limits, now := frozenLimits(nil)
	s := &Service{RateLimits: limits}
	resolved := 0
	fields := s.limitedFields("Query", graphql.Fields{
		"sandbox": &graphql.Field{Type: graphql.String, Resolve: func(graphql.ResolveParams) (any, error) {
			resolved++
			return "ok", nil
		}},
	})
	fields["services"] = &graphql.Field{Type: graphql.String, Resolve: func(graphql.ResolveParams) (any, error) { return "ok", nil }}
	schema, err := graphql.NewSchema(graphql.SchemaConfig{Query: graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: fields})})
	if err != nil {
		t.Fatal(err)
	}
	general := core.NewKeyedRateLimiter[string](60, 5, time.Minute, time.Minute).Bucket("id:identity-1")
	run := func(query string) *graphql.Result {
		t.Helper()
		if _, wait := core.ReserveEach(*now, 1, general); wait > 0 {
			t.Fatal("the general bucket is empty")
		}
		ctx := core.WithRateLimitScope(asUser("identity-1"), core.NewRateLimitScope("", http.Header{}, []core.RateLimitBucket{core.LocalRateLimitBucket(general)}, false))
		return graphql.Do(graphql.Params{Schema: schema, RequestString: query, Context: ctx})
	}
	tokens := func() int { return int(general.TokensAt(*now)) }

	for range 20 {
		if res := run(`{ a: sandbox b: sandbox }`); res.HasErrors() {
			t.Fatalf("sandbox-only document: %v", res.Errors)
		}
	}
	if got := tokens(); got != 5 {
		t.Fatalf("general tokens after 20 sandbox-only documents = %d, want all 5", got)
	}
	if resolved != 40 {
		t.Fatalf("resolved %d sandbox fields, want 40 (each alias is one read)", resolved)
	}
	if res := run(`{ sandbox services }`); res.HasErrors() {
		t.Fatalf("mixed document: %v", res.Errors)
	}
	if res := run(`{ ...F } fragment F on Query { sandbox }`); res.HasErrors() {
		t.Fatalf("fragment document: %v", res.Errors)
	}
	if got := tokens(); got != 3 {
		t.Fatalf("general tokens after a mixed and a fragment document = %d, want 3", got)
	}

	// The 400 reads are spent by fields, not documents: 42 are gone, so the
	// 359th further alias is refused with the general limiter's GraphQL code.
	if got := spend(limits, asUser("identity-1"), RateClassRead, 400); got != 358 {
		t.Fatalf("reads left = %d, want 358", got)
	}
	res := run(`{ sandbox }`)
	if len(res.Errors) != 1 || res.Errors[0].Extensions["code"] != gqlRateLimitedCode {
		t.Fatalf("refused read = %+v, want one error coded %s", res.Errors, gqlRateLimitedCode)
	}
	if got := tokens(); got != 3 {
		t.Fatalf("a refused sandbox document spent the general budget: %d tokens", got)
	}
}

// An MCP tool spends its class, answers the 429 body's id as its code, gives
// the request's general tokens back and writes Retry-After where the request
// has a header.
func TestMCPToolSpendsItsClassAndRefusesByName(t *testing.T) {
	limits, now := frozenLimits(nil)
	s := &Service{RateLimits: limits}
	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	calls := 0
	addTool(s, server, &mcp.Tool{Name: "create_sandbox_snapshot"},
		func(context.Context, *mcp.CallToolRequest, sandboxIDArgs) (*mcp.CallToolResult, deleteResult, error) {
			calls++
			return nil, deleteResult{}, nil
		})
	general := core.NewKeyedRateLimiter[string](60, 5, time.Minute, time.Minute).Bucket("id:identity-1")
	core.ReserveEach(*now, 1, general)
	header := http.Header{}
	ctx := core.WithRateLimitScope(asUser("identity-1"), core.NewRateLimitScope("", header, []core.RateLimitBucket{core.LocalRateLimitBucket(general)}, false))
	serverT, clientT := mcp.NewInMemoryTransports()
	if _, err := server.Connect(ctx, serverT, nil); err != nil {
		t.Fatal(err)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil).Connect(ctx, clientT, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	call := func() *mcp.CallToolResult {
		t.Helper()
		res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "create_sandbox_snapshot", Arguments: map[string]any{"id": "sbx-1"}})
		if err != nil {
			t.Fatal(err)
		}
		return res
	}
	for i := range 10 {
		if res := call(); res.IsError {
			t.Fatalf("capture %d refused: %+v", i+1, res.Content)
		}
	}
	res := call()
	text, _ := res.Content[0].(*mcp.TextContent)
	if !res.IsError || text == nil || !strings.HasPrefix(text.Text, mcpRateLimitedCode+": rate limit exceeded") {
		t.Fatalf("11th capture = %+v, want a tool error naming %s", res.Content, mcpRateLimitedCode)
	}
	if calls != 10 {
		t.Fatalf("the tool ran %d times, want 10", calls)
	}
	if header.Get("Retry-After") == "" || header.Get("RateLimit-Remaining") != "0" || header.Get("RateLimit-Limit") != "10" {
		t.Fatalf("refusal headers = %v", header)
	}
	if got := int(general.TokensAt(*now)); got != 5 {
		t.Fatalf("general tokens = %d, want all 5 given back", got)
	}
}
