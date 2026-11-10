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
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// fakeSandboxEgress records allow-list policy calls and serves Rules from them,
// the way the real manager reads the per-sandbox policy back.
type fakeSandboxEgress struct {
	mu       sync.Mutex
	ops      []string
	rules    map[string][]string
	applyErr error
}

func (f *fakeSandboxEgress) Apply(_ context.Context, namespace, id string, domains []string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.ops = append(f.ops, "apply "+namespace+" "+id)
	if f.applyErr != nil {
		return f.applyErr
	}
	if f.rules == nil {
		f.rules = map[string][]string{}
	}
	f.rules[id] = slices.Clone(domains)
	return nil
}

func (f *fakeSandboxEgress) Rules(_ context.Context, _, id string) ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.rules[id]), nil
}

func (f *fakeSandboxEgress) Delete(_ context.Context, namespace, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.ops = append(f.ops, "delete "+namespace+" "+id)
	delete(f.rules, id)
	return nil
}

func (f *fakeSandboxEgress) recorded() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.ops)
}

// policyUpstream is an OpenSandbox stub that records the create's metadata,
// then serves that sandbox back on list/get and accepts its terminate.
type policyUpstream struct {
	mu       sync.Mutex
	created  *createRequest
	failNext bool
	order    *[]string
}

func (u *policyUpstream) handler(t *testing.T) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u.mu.Lock()
		defer u.mu.Unlock()
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/sandboxes":
			if u.order != nil {
				*u.order = append(*u.order, "opensandbox create")
			}
			if u.failNext {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			var req createRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Errorf("decode create: %v", err)
			}
			u.created = &req
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":"os-1","status":{"state":"Running"}}`))
		case r.Method == http.MethodGet && u.created != nil:
			raw, _ := json.Marshal(map[string]any{"id": "os-1", "metadata": u.created.Metadata, "status": map[string]any{"state": "Running"}})
			if r.URL.Path == "/sandboxes" {
				raw = append(append([]byte("["), raw...), ']')
			}
			_, _ = w.Write(raw)
		case r.Method == http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, r)
		}
	}
}

func allowListRequest(domains ...string) CreateRequest {
	rules := make([]NetworkPolicyRule, 0, len(domains))
	for _, domain := range domains {
		rules = append(rules, NetworkPolicyRule{Domain: domain})
	}
	return CreateRequest{Template: "node", NetworkPolicy: &NetworkPolicy{Type: NetworkPolicyAllowList, Rules: rules}}
}

// Render: omitting networkPolicy defaults to allow-all. The Pod carries the
// egress selector label that the static allow-all platform policy selects.
func TestCreateDefaultsToAllowAllAndStampsTheEgressSelector(t *testing.T) {
	up := &policyUpstream{}
	svc := stubServer(t, up.handler(t))
	box, err := svc.Create(callerCtx(), CreateRequest{Template: "node"})
	if err != nil {
		t.Fatal(err)
	}
	if box.NetworkPolicy == nil || box.NetworkPolicy.Default != NetworkPolicyAllowAll || box.NetworkPolicy.Rules != nil {
		t.Fatalf("policy = %#v, want allow-all without rules", box.NetworkPolicy)
	}
	md := up.created.Metadata
	if md[metadataNetworkPolicy] != "allow-all" || md[metadataEgress] != "allow-all" {
		t.Fatalf("metadata = %#v, want network-policy and egress both allow-all", md)
	}
}

func TestCreateDenyAllStampsDenyAllWithoutAPerSandboxPolicy(t *testing.T) {
	up := &policyUpstream{}
	svc := stubServer(t, up.handler(t))
	eg := &fakeSandboxEgress{}
	svc.SandboxEgress = eg
	if _, err := svc.Create(callerCtx(), CreateRequest{Template: "node", NetworkPolicy: &NetworkPolicy{Type: NetworkPolicyDenyAll}}); err != nil {
		t.Fatal(err)
	}
	if up.created.Metadata[metadataEgress] != "deny-all" {
		t.Fatalf("egress label = %q, want deny-all", up.created.Metadata[metadataEgress])
	}
	if ops := eg.recorded(); len(ops) != 0 {
		t.Fatalf("deny-all wrote per-sandbox policy: %v", ops)
	}
}

// The allow-list policy must exist before the sandbox does, select the
// sandbox's own public id, and carry the canonical (sorted) domains; reads
// report them as Render's rules.
func TestCreateAllowListInstallsItsPolicyFirstAndReadsReportTheRules(t *testing.T) {
	var order []string
	up := &policyUpstream{order: &order}
	svc := stubServer(t, func(w http.ResponseWriter, r *http.Request) { up.handler(t)(w, r) })
	eg := &fakeSandboxEgress{}
	svc.SandboxEgress = &orderedEgress{fakeSandboxEgress: eg, order: &order}

	box, err := svc.Create(callerCtx(), allowListRequest("registry.npmjs.org", "*.githubusercontent.com"))
	if err != nil {
		t.Fatal(err)
	}
	if len(order) != 2 || order[0] != "apply" || order[1] != "opensandbox create" {
		t.Fatalf("order = %v, want the policy applied before the sandbox is created", order)
	}
	id := up.created.Metadata[metadataPublicID]
	if ops := eg.recorded(); len(ops) != 1 || ops[0] != "apply tea-a-sandbox "+id {
		t.Fatalf("egress ops = %v, want one apply in tea-a-sandbox for %s", ops, id)
	}
	if up.created.Metadata[metadataEgress] != "allow-list" || up.created.Metadata[metadataNetworkPolicy] != "allow-list" {
		t.Fatalf("metadata = %#v", up.created.Metadata)
	}
	want := []NetworkPolicyRule{{Domain: "*.githubusercontent.com", Protocol: "https"}, {Domain: "registry.npmjs.org", Protocol: "https"}}
	if box.NetworkPolicy == nil || !slices.Equal(box.NetworkPolicy.Rules, want) {
		t.Fatalf("create policy = %#v, want %v", box.NetworkPolicy, want)
	}
	got, err := svc.Get(callerCtx(), id)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got.NetworkPolicy.Rules, want) {
		t.Fatalf("get rules = %v, want %v", got.NetworkPolicy.Rules, want)
	}
	listed, err := svc.List(callerCtx())
	if err != nil || len(listed) != 1 || !slices.Equal(listed[0].NetworkPolicy.Rules, want) {
		t.Fatalf("list = %#v, %v; want the rules", listed, err)
	}
	out, _ := json.Marshal(got.NetworkPolicy)
	if string(out) != `{"type":"allow-list","default":"allow-list","rules":[{"domain":"*.githubusercontent.com","protocol":"https"},{"domain":"registry.npmjs.org","protocol":"https"}]}` {
		t.Fatalf("wire policy = %s", out)
	}

	if err := svc.Terminate(callerCtx(), id); err != nil {
		t.Fatal(err)
	}
	if ops := eg.recorded(); ops[len(ops)-1] != "delete tea-a-sandbox "+id {
		t.Fatalf("terminate egress ops = %v, want the policy deleted", ops)
	}
}

type orderedEgress struct {
	*fakeSandboxEgress
	order *[]string
}

func (o *orderedEgress) Apply(ctx context.Context, namespace, id string, domains []string) error {
	*o.order = append(*o.order, "apply")
	return o.fakeSandboxEgress.Apply(ctx, namespace, id, domains)
}

func TestCreateAllowListFailureRemovesItsPolicy(t *testing.T) {
	up := &policyUpstream{failNext: true}
	svc := stubServer(t, up.handler(t))
	eg := &fakeSandboxEgress{}
	svc.SandboxEgress = eg
	if _, err := svc.Create(callerCtx(), allowListRequest("example.com")); err == nil {
		t.Fatal("create succeeded against a failing upstream")
	}
	ops := eg.recorded()
	if len(ops) != 2 || !strings.HasPrefix(ops[0], "apply ") || !strings.HasPrefix(ops[1], "delete ") || ops[0][len("apply "):] != ops[1][len("delete "):] {
		t.Fatalf("egress ops = %v, want apply then delete of the same policy", ops)
	}
}

func TestCreateAllowListPolicyFailureNeverCreatesTheSandbox(t *testing.T) {
	up := &policyUpstream{}
	svc := stubServer(t, up.handler(t))
	svc.SandboxEgress = &fakeSandboxEgress{applyErr: errors.New("admission denied")}
	if _, err := svc.Create(callerCtx(), allowListRequest("example.com")); err == nil {
		t.Fatal("create succeeded although its policy could not be installed")
	}
	if up.created != nil {
		t.Fatal("the sandbox was created without its allow-list policy")
	}
}

func TestCreateAllowListWithoutTheEgressMechanismIsUnavailable(t *testing.T) {
	up := &policyUpstream{}
	svc := stubServer(t, up.handler(t))
	_, err := svc.Create(callerCtx(), allowListRequest("example.com"))
	if !errors.Is(err, core.ErrSandboxesUnavailable) {
		t.Fatalf("err = %v, want ErrSandboxesUnavailable", err)
	}
	if up.created != nil {
		t.Fatal("an allow-list sandbox was created with nothing to enforce its rules")
	}
}

// A sandbox is served only when its stamps name an enforced posture: the
// w32/m283 pair (reported policy equals the egress selector), or a legacy
// deny-all sandbox without a selector.
func TestEnforcedPolicyStamp(t *testing.T) {
	for _, tc := range []struct {
		policy, egress string
		want           bool
	}{
		{"deny-all", "", true},
		{"allow-all", "allow-all", true},
		{"allow-list", "allow-list", true},
		{"deny-all", "deny-all", true},
		{"allow-all", "", false},
		{"allow-list", "", false},
		{"deny-all", "allow-all", false},
		{"allow-some", "allow-some", false},
		{"", "", false},
	} {
		md := map[string]string{metadataNetworkPolicy: tc.policy}
		if tc.egress != "" {
			md[metadataEgress] = tc.egress
		}
		if got := enforcedPolicyStamp(md); got != tc.want {
			t.Errorf("policy %q egress %q = %v, want %v", tc.policy, tc.egress, got, tc.want)
		}
	}
}

func TestRESTAllowListRoundTripsRulesOnCreate(t *testing.T) {
	up := &policyUpstream{}
	svc := stubServer(t, up.handler(t))
	svc.SandboxEgress = &fakeSandboxEgress{}
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/sandboxes", strings.NewReader(
		`{"template":"node","networkPolicy":{"type":"allow-list","rules":[{"domain":"api.github.com","protocol":"https"}]}}`,
	)).WithContext(callerCtx())
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"networkPolicy":{"type":"allow-list","default":"allow-list","rules":[{"domain":"api.github.com","protocol":"https"}]}`) {
		t.Fatalf("body = %s", rec.Body.String())
	}
}

func TestGraphQLAndMCPAllowListCreateReportRules(t *testing.T) {
	up := &policyUpstream{}
	svc := stubServer(t, up.handler(t))
	svc.SandboxEgress = &fakeSandboxEgress{}
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
		Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: svc.GraphQLMutation()}),
	})
	if err != nil {
		t.Fatal(err)
	}
	res := graphql.Do(graphql.Params{
		Schema:        schema,
		RequestString: `mutation { createSandbox(template:"node", networkPolicy:{type:"allow-list", rules:[{domain:"api.github.com"}]}) { networkPolicy { type rules { domain protocol } } } }`,
		Context:       callerCtx(),
	})
	if len(res.Errors) != 0 {
		t.Fatalf("GraphQL errors = %#v", res.Errors)
	}
	got, _ := json.Marshal(res.Data)
	if string(got) != `{"createSandbox":{"networkPolicy":{"rules":[{"domain":"api.github.com","protocol":"https"}],"type":"allow-list"}}}` {
		t.Fatalf("GraphQL create = %s", got)
	}

	server := mcp.NewServer(&mcp.Implementation{Name: "sandbox-test", Version: "0"}, nil)
	svc.RegisterMCP(server)
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	ctx := context.Background()
	if _, err := server.Connect(ctx, serverTransport, nil); err != nil {
		t.Fatal(err)
	}
	client, err := mcp.NewClient(&mcp.Implementation{Name: "sandbox-test", Version: "0"}, nil).Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })
	out, err := client.CallTool(callerCtx(), &mcp.CallToolParams{Name: "spawn_sandbox", Arguments: map[string]any{
		"template":      "node",
		"networkPolicy": map[string]any{"type": "allow-list", "rules": []any{map[string]any{"domain": "api.github.com"}}},
	}})
	if err != nil || out.IsError {
		t.Fatalf("MCP create = %#v, %v", out, err)
	}
	var created struct {
		NetworkPolicy map[string]any `json:"networkPolicy"`
	}
	if err := json.Unmarshal([]byte(out.Content[0].(*mcp.TextContent).Text), &created); err != nil {
		t.Fatal(err)
	}
	policy, _ := json.Marshal(created.NetworkPolicy)
	if string(policy) != `{"default":"allow-list","rules":[{"domain":"api.github.com","protocol":"https"}],"type":"allow-list"}` {
		t.Fatalf("MCP create policy = %s", policy)
	}
}

// barrierEgress answers Rules only once `want` reads are in flight at the
// same time, so a list that reads policies one by one fails.
type barrierEgress struct {
	*fakeSandboxEgress
	want    int
	arrived chan struct{}
	failID  string
}

func (b *barrierEgress) Rules(ctx context.Context, namespace, id string) ([]string, error) {
	b.arrived <- struct{}{}
	deadline := time.After(2 * time.Second)
	for len(b.arrived) < b.want {
		select {
		case <-deadline:
			return nil, errors.New("policy reads were not concurrent")
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(time.Millisecond):
		}
	}
	if id == b.failID {
		return nil, errors.New("policy read failed")
	}
	return []string{"api.github.com"}, nil
}

// w32/006: a list's allow-list rules are read side by side, not one policy
// GET after another, and one failed read fails the list.
func TestListReadsAllowListRulesConcurrently(t *testing.T) {
	up := &policyUpstream{}
	seed := stubServer(t, func(w http.ResponseWriter, r *http.Request) { up.handler(t)(w, r) })
	seed.SandboxEgress = &fakeSandboxEgress{}
	if _, err := seed.Create(callerCtx(), allowListRequest("api.github.com")); err != nil {
		t.Fatal(err)
	}
	const n = 3
	var rows []map[string]any
	for i := range n {
		meta := maps.Clone(up.created.Metadata)
		meta[metadataPublicID] = fmt.Sprintf("sbx-%020d", i)
		rows = append(rows, map[string]any{"id": fmt.Sprintf("os-%d", i), "metadata": meta, "status": map[string]any{"state": "Running"}})
	}
	for _, failID := range []string{"", "sbx-00000000000000000001"} {
		svc := stubServer(t, func(w http.ResponseWriter, _ *http.Request) { _ = json.NewEncoder(w).Encode(rows) })
		svc.SandboxEgress = &barrierEgress{fakeSandboxEgress: &fakeSandboxEgress{}, want: n, arrived: make(chan struct{}, n), failID: failID}
		listed, err := svc.List(callerCtx())
		if failID != "" {
			if err == nil {
				t.Fatal("a failed policy read did not fail the list")
			}
			continue
		}
		if err != nil || len(listed) != n {
			t.Fatalf("list = %d sandboxes, %v", len(listed), err)
		}
		for _, box := range listed {
			if box.NetworkPolicy == nil || len(box.NetworkPolicy.Rules) != 1 || box.NetworkPolicy.Rules[0].Domain != "api.github.com" {
				t.Errorf("%s policy = %#v", box.ID, box.NetworkPolicy)
			}
		}
	}
}
