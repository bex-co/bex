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
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

type sandboxRejectingPaymentGate struct{ calls []string }

func (g *sandboxRejectingPaymentGate) RequirePaymentMethod(_ context.Context, workspaceID string) error {
	g.calls = append(g.calls, workspaceID)
	return core.NewPaymentRequiredError()
}

func gatedSandboxService(t *testing.T) (*Service, *sandboxRejectingPaymentGate) {
	t.Helper()
	upstream := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("payment-refused create reached OpenSandbox")
	}))
	t.Cleanup(upstream.Close)
	gate := &sandboxRejectingPaymentGate{}
	return &Service{
		Base: &core.Base{
			Namespace: "default",
			Workspace: fakeWorkspace{"id-a": "tea-a"},
			Payment:   gate,
		},
		Client:          NewClient(upstream.URL),
		Templates:       map[string]Template{"base": {Image: "alpine:3", CPU: "500m", Memory: "512Mi"}},
		DefaultTemplate: "base",
		DefaultPlan:     PlanStarter,
	}, gate
}

func TestSandboxCreatePaymentGateAcrossSurfaces(t *testing.T) {
	t.Run("REST", func(t *testing.T) {
		svc, gate := gatedSandboxService(t)
		mux := http.NewServeMux()
		svc.RegisterREST(mux)
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/v1/sandboxes", nil).WithContext(callerCtx())
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusPaymentRequired || !strings.Contains(rec.Body.String(), `"code":"PAYMENT_REQUIRED"`) {
			t.Fatalf("REST refusal = %d %s", rec.Code, rec.Body.String())
		}
		if len(gate.calls) != 1 || gate.calls[0] != "tea-a" {
			t.Fatalf("REST gate calls = %v", gate.calls)
		}
	})

	t.Run("GraphQL", func(t *testing.T) {
		svc, _ := gatedSandboxService(t)
		schema, err := graphql.NewSchema(graphql.SchemaConfig{
			Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
			Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: svc.GraphQLMutation()}),
		})
		if err != nil {
			t.Fatal(err)
		}
		res := graphql.Do(graphql.Params{Schema: schema, RequestString: `mutation { createSandbox { id } }`, Context: callerCtx()})
		if len(res.Errors) != 1 || res.Errors[0].Extensions["code"] != "PAYMENT_REQUIRED" {
			t.Fatalf("GraphQL refusal = %#v", res.Errors)
		}
	})

	t.Run("MCP", func(t *testing.T) {
		svc, _ := gatedSandboxService(t)
		server := mcp.NewServer(&mcp.Implementation{Name: "sandbox-gate-test", Version: "0"}, nil)
		svc.RegisterMCP(server)
		serverTransport, clientTransport := mcp.NewInMemoryTransports()
		ctx := callerCtx()
		if _, err := server.Connect(ctx, serverTransport, nil); err != nil {
			t.Fatal(err)
		}
		client, err := mcp.NewClient(&mcp.Implementation{Name: "sandbox-gate-test", Version: "0"}, nil).Connect(ctx, clientTransport, nil)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = client.Close() })
		res, err := client.CallTool(ctx, &mcp.CallToolParams{Name: "spawn_sandbox", Arguments: map[string]any{}})
		if err != nil {
			t.Fatal(err)
		}
		if !res.IsError || !strings.Contains(res.Content[0].(*mcp.TextContent).Text, "PAYMENT_REQUIRED") {
			t.Fatalf("MCP refusal = %#v", res)
		}
	})
}

func TestSandboxCreateWithoutPaymentGateIsUnaffected(t *testing.T) {
	svc := stubServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"os-1","status":{"state":"Creating"}}`))
	})
	if _, err := svc.Create(callerCtx(), CreateRequest{Template: "node", Plan: PlanStarter}); err != nil {
		t.Fatalf("gate-off create: %v", err)
	}
}
