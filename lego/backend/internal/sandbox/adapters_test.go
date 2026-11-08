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
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

func policyAdapterService(t *testing.T) *Service {
	t.Helper()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("an unsupported policy must not reach OpenSandbox")
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(upstream.Close)
	return &Service{
		Base:            &core.Base{Namespace: "default"},
		Client:          NewClient(upstream.URL),
		Templates:       map[string]Template{"node": {Image: "node:20"}},
		DefaultTemplate: "node",
	}
}

func TestGraphQLDefaultPolicyRoundTrips(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":"sbx-1","status":{"state":"Running"}}`)
	}))
	t.Cleanup(upstream.Close)
	svc := &Service{
		Base:            &core.Base{Namespace: "default"},
		Client:          NewClient(upstream.URL),
		Templates:       map[string]Template{"node": {Image: "node:20"}},
		DefaultTemplate: "node",
	}
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
		Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: svc.GraphQLMutation()}),
	})
	if err != nil {
		t.Fatal(err)
	}
	res := graphql.Do(graphql.Params{
		Schema:        schema,
		RequestString: `mutation { createSandbox { networkPolicy { type default } } }`,
		Context:       callerCtx(),
	})
	if len(res.Errors) != 0 {
		t.Fatalf("GraphQL default policy errors = %#v", res.Errors)
	}
	create, ok := res.Data.(map[string]any)["createSandbox"].(map[string]any)
	if !ok {
		t.Fatalf("GraphQL create result = %#v", res.Data)
	}
	policy, ok := create["networkPolicy"].(map[string]any)
	if !ok || policy["default"] != "deny-all" || policy["type"] != "deny-all" {
		t.Fatalf("GraphQL networkPolicy = %#v", create["networkPolicy"])
	}
}

// Render renamed networkPolicy.default to type, keeping default as an alias
// that responses echo; Render CLI v2.29 sends type (w8/m67).
func TestNetworkPolicyTypeIsRendersFieldAndDefaultItsAlias(t *testing.T) {
	for _, body := range []string{`{"type":"deny-all"}`, `{"default":"deny-all"}`, `{"type":"deny-all","default":"deny-all"}`} {
		var in NetworkPolicy
		if err := json.Unmarshal([]byte(body), &in); err != nil {
			t.Fatal(err)
		}
		got, err := normalizeNetworkPolicy(&in)
		if err != nil {
			t.Fatalf("%s: %v", body, err)
		}
		out, _ := json.Marshal(got)
		if string(out) != `{"type":"deny-all","default":"deny-all"}` {
			t.Errorf("%s → %s, want type and its default alias", body, out)
		}
	}
	if _, err := normalizeNetworkPolicy(&NetworkPolicy{Type: "allow-all"}); err == nil || !strings.Contains(err.Error(), "only") {
		t.Errorf("type allow-all = %v, want the named refusal", err)
	}
}

func TestRESTNetworkPolicyNamedRefusalAndStrictMetadata(t *testing.T) {
	svc := policyAdapterService(t)
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/sandboxes", strings.NewReader(
		`{"networkPolicy":{"default":"allow-all"}}`,
	)).WithContext(callerCtx())
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "SANDBOX_NETWORK_POLICY_UNSUPPORTED") {
		t.Fatalf("REST policy refusal = %d %s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/v1/sandboxes", strings.NewReader(
		`{"metadata":{"bex.co/owner":"id-attacker"}}`,
	)).WithContext(callerCtx())
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "unknown field") {
		t.Fatalf("REST caller metadata override = %d %s, want strict 400", rec.Code, rec.Body.String())
	}
}

func TestGraphQLNetworkPolicyNamedRefusal(t *testing.T) {
	svc := policyAdapterService(t)
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
		Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: svc.GraphQLMutation()}),
	})
	if err != nil {
		t.Fatal(err)
	}
	res := graphql.Do(graphql.Params{
		Schema:        schema,
		RequestString: `mutation { createSandbox(networkPolicy:{default:"allow-all"}) { id } }`,
		Context:       callerCtx(),
	})
	if len(res.Errors) != 1 || res.Errors[0].Extensions["code"] != "SANDBOX_NETWORK_POLICY_UNSUPPORTED" {
		t.Fatalf("GraphQL policy refusal = %#v", res.Errors)
	}
}

// capacityAdapterService builds a Service whose upstream answers every create
// with the OpenSandbox pod-ready-timeout signature — the live quota-denied
// shape from .pm/w3/011.md.
func capacityAdapterService(t *testing.T) *Service {
	t.Helper()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusGatewayTimeout)
		_, _ = io.WriteString(w, `{"code":"KUBERNETES::POD_READY_TIMEOUT","message":"sandbox pod not ready within 300s"}`)
	}))
	t.Cleanup(upstream.Close)
	return &Service{
		Base:            &core.Base{Namespace: "default"},
		Client:          NewClient(upstream.URL),
		Templates:       map[string]Template{"node": {Image: "node:20"}},
		DefaultTemplate: "node",
	}
}

func TestRESTPodReadyTimeoutIsTypedCapacityRefusal(t *testing.T) {
	svc := capacityAdapterService(t)
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/sandboxes", strings.NewReader(`{}`)).WithContext(callerCtx())
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "SANDBOX_CAPACITY_LIMIT") {
		t.Fatalf("REST capacity refusal = %d %s, want 409 + SANDBOX_CAPACITY_LIMIT", rec.Code, rec.Body.String())
	}
}

func TestGraphQLPodReadyTimeoutIsTypedCapacityRefusal(t *testing.T) {
	svc := capacityAdapterService(t)
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
		Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: svc.GraphQLMutation()}),
	})
	if err != nil {
		t.Fatal(err)
	}
	res := graphql.Do(graphql.Params{
		Schema:        schema,
		RequestString: `mutation { createSandbox { id } }`,
		Context:       callerCtx(),
	})
	if len(res.Errors) != 1 || res.Errors[0].Extensions["code"] != "SANDBOX_CAPACITY_LIMIT" {
		t.Fatalf("GraphQL capacity refusal = %#v, want extensions.code SANDBOX_CAPACITY_LIMIT", res.Errors)
	}
}

func TestMCPPodReadyTimeoutIsTypedCapacityRefusal(t *testing.T) {
	svc := capacityAdapterService(t)
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

	res, err := client.CallTool(ctx, &mcp.CallToolParams{Name: "spawn_sandbox", Arguments: map[string]any{}})
	if err != nil {
		t.Fatal(err)
	}
	if !res.IsError || !strings.Contains(res.Content[0].(*mcp.TextContent).Text, "SANDBOX_CAPACITY_LIMIT") {
		t.Fatalf("MCP capacity refusal = %#v, want the code in the text error", res)
	}
}

func TestMCPNetworkPolicyNamedRefusal(t *testing.T) {
	svc := policyAdapterService(t)
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

	// Render's renamed `type` and allow-list `rules` must pass the tool's
	// input schema and reach the named refusal (w8/m67).
	for _, policy := range []map[string]any{
		{"default": "allow-all"},
		{"type": "allow-all"},
		{"type": "deny-all", "rules": []any{map[string]any{"domain": "example.com"}}},
	} {
		res, err := client.CallTool(ctx, &mcp.CallToolParams{
			Name:      "spawn_sandbox",
			Arguments: map[string]any{"networkPolicy": policy},
		})
		if err != nil {
			t.Fatal(err)
		}
		if !res.IsError || !strings.Contains(res.Content[0].(*mcp.TextContent).Text, "SANDBOX_NETWORK_POLICY_UNSUPPORTED") {
			t.Fatalf("MCP policy %v refusal = %#v", policy, res.Content)
		}
	}
}

// TestTimeoutSecondsAgreesAcrossRESTGraphQLMCP pins Render's timeoutSeconds
// contract (w2/m263 D3=A) on every surface: an explicit 0 is no timeout on a
// paid workspace (no OpenSandbox expiry, reported as 0), 86400 s on Hobby; an
// omitted timeout is 86400 s everywhere.
func TestTimeoutSecondsAgreesAcrossRESTGraphQLMCP(t *testing.T) {
	for _, tc := range []struct {
		name, plan string
		explicit   bool // send timeoutSeconds: 0, else omit it
		upstream   int  // OpenSandbox timeout (0 = omitted, no expiry)
		reported   int
		metadata   string
	}{
		{name: "paid zero", plan: "pro", explicit: true, upstream: 0, reported: 0, metadata: timeoutNone},
		{name: "hobby zero", plan: "hobby", explicit: true, upstream: 86400, reported: 86400, metadata: "86400"},
		{name: "paid omitted", plan: "pro", upstream: 86400, reported: 86400, metadata: "86400"},
		{name: "hobby omitted", plan: "hobby", upstream: 86400, reported: 86400, metadata: "86400"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var gotTimeout *int
			var gotMeta string
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost {
					return
				}
				var body struct {
					Timeout  *int              `json:"timeout"`
					Metadata map[string]string `json:"metadata"`
				}
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Errorf("decode: %v", err)
				}
				gotTimeout, gotMeta = body.Timeout, body.Metadata[metadataTimeout]
				_ = json.NewEncoder(w).Encode(map[string]any{
					"id":       "sbx-timeout",
					"metadata": body.Metadata,
					"status":   map[string]string{"state": "Running"},
					"created":  time.Now().UTC().Format(time.RFC3339Nano),
					"image":    map[string]string{"uri": "node:20"},
				})
			}))
			t.Cleanup(upstream.Close)
			svc := &Service{
				Base: &core.Base{Namespace: "default", Workspace: fakeWorkspace{"id-a": "tea-a"},
					Plans: core.WorkspacePlanFunc(func(context.Context, string) (string, error) { return tc.plan, nil })},
				Client:          NewClient(upstream.URL),
				Templates:       map[string]Template{"node": {Image: "node:20"}},
				DefaultTemplate: "node",
			}
			check := func(surface string, reported int) {
				t.Helper()
				upstreamTimeout := 0
				if gotTimeout != nil {
					upstreamTimeout = *gotTimeout
				}
				if upstreamTimeout != tc.upstream || reported != tc.reported || gotMeta != tc.metadata {
					t.Fatalf("%s: upstream timeout %d, reported %d, metadata %q; want %d, %d, %q",
						surface, upstreamTimeout, reported, gotMeta, tc.upstream, tc.reported, tc.metadata)
				}
				gotTimeout, gotMeta = nil, ""
			}

			mux := http.NewServeMux()
			svc.RegisterREST(mux)
			restBody, gqlArgs, mcpArgs := `{}`, ``, map[string]any{}
			if tc.explicit {
				restBody, gqlArgs, mcpArgs = `{"timeoutSeconds":0}`, `(timeoutSeconds: 0)`, map[string]any{"timeoutSeconds": 0}
			}
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/sandboxes", strings.NewReader(restBody)).WithContext(callerCtx()))
			if rec.Code != http.StatusCreated {
				t.Fatalf("REST create = %d %s", rec.Code, rec.Body.String())
			}
			var rest Sandbox
			if err := json.Unmarshal(rec.Body.Bytes(), &rest); err != nil {
				t.Fatalf("REST decode: %v", err)
			}
			check("REST", rest.TimeoutSeconds)

			schema, err := graphql.NewSchema(graphql.SchemaConfig{
				Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
				Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: svc.GraphQLMutation()}),
			})
			if err != nil {
				t.Fatal(err)
			}
			res := graphql.Do(graphql.Params{
				Schema:        schema,
				RequestString: `mutation { createSandbox` + gqlArgs + ` { timeoutSeconds } }`,
				Context:       callerCtx(),
			})
			if len(res.Errors) != 0 {
				t.Fatalf("GraphQL errors = %#v", res.Errors)
			}
			gqlTimeout, ok := res.Data.(map[string]any)["createSandbox"].(map[string]any)["timeoutSeconds"].(int)
			if !ok {
				t.Fatalf("GraphQL timeoutSeconds missing: %#v", res.Data)
			}
			check("GraphQL", gqlTimeout)

			server := mcp.NewServer(&mcp.Implementation{Name: "sandbox-test", Version: "0"}, nil)
			svc.RegisterMCP(server)
			serverTransport, clientTransport := mcp.NewInMemoryTransports()
			ctx := callerCtx()
			if _, err := server.Connect(ctx, serverTransport, nil); err != nil {
				t.Fatal(err)
			}
			client, err := mcp.NewClient(&mcp.Implementation{Name: "sandbox-test", Version: "0"}, nil).Connect(ctx, clientTransport, nil)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = client.Close() })
			toolRes, err := client.CallTool(ctx, &mcp.CallToolParams{Name: "spawn_sandbox", Arguments: mcpArgs})
			if err != nil {
				t.Fatal(err)
			}
			if toolRes.IsError {
				t.Fatalf("MCP spawn error = %#v", toolRes)
			}
			raw, err := json.Marshal(toolRes.StructuredContent)
			if err != nil {
				t.Fatal(err)
			}
			var mcpSB Sandbox
			if err := json.Unmarshal(raw, &mcpSB); err != nil {
				t.Fatalf("MCP decode: %v (%s)", err, raw)
			}
			check("MCP", mcpSB.TimeoutSeconds)
		})
	}
}
