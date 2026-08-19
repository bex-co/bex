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
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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
		RequestString: `mutation { createSandbox { networkPolicy { default } } }`,
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
	if !ok || policy["default"] != "deny-all" {
		t.Fatalf("GraphQL networkPolicy = %#v", create["networkPolicy"])
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

	res, err := client.CallTool(ctx, &mcp.CallToolParams{
		Name: "spawn_sandbox",
		Arguments: map[string]any{
			"networkPolicy": map[string]any{"default": "allow-all"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.IsError || !strings.Contains(res.Content[0].(*mcp.TextContent).Text, "SANDBOX_NETWORK_POLICY_UNSUPPORTED") {
		t.Fatalf("MCP policy refusal = %#v", res)
	}
}
