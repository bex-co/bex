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

package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

type rejectingPaymentGate struct{ calls []string }

func (g *rejectingPaymentGate) RequirePaymentMethod(_ context.Context, workspaceID string) error {
	g.calls = append(g.calls, workspaceID)
	return core.NewPaymentRequiredError()
}

type switchablePaymentGate struct {
	bound bool
	calls int
}

func (g *switchablePaymentGate) RequirePaymentMethod(context.Context, string) error {
	g.calls++
	if g.bound {
		return nil
	}
	return core.NewPaymentRequiredError()
}

func TestPaidCreateRefusalHasOneRESTGraphQLMCPDialect(t *testing.T) {
	newGatedService := func() *Service {
		svc, _ := newService()
		svc.Payment = &rejectingPaymentGate{}
		return svc
	}

	t.Run("REST", func(t *testing.T) {
		svc, _ := newService()
		gate := &switchablePaymentGate{}
		svc.Payment = gate
		const requestBody = `{"name":"paid-rest","plan":"basic-256mb"}`
		rec := serveREST(svc, http.MethodPost, "/v1/postgres", requestBody)
		if rec.Code != http.StatusPaymentRequired {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
		var errorBody map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &errorBody); err != nil {
			t.Fatal(err)
		}
		if errorBody["code"] != "PAYMENT_REQUIRED" || errorBody["message"] != core.PaymentRequiredMessage {
			t.Fatalf("body=%#v", errorBody)
		}

		// This is the exact interrupted request, against the same running service.
		// The webhook/store tests establish that a verified Checkout completion is
		// what flips this local marker; no other request input needs to change.
		gate.bound = true
		retry := serveREST(svc, http.MethodPost, "/v1/postgres", requestBody)
		if retry.Code != http.StatusCreated {
			t.Fatalf("same request after binding = %d body=%s", retry.Code, retry.Body.String())
		}
		if gate.calls != 2 {
			t.Fatalf("payment gate calls=%d, want one per paid attempt", gate.calls)
		}
	})

	t.Run("GraphQL", func(t *testing.T) {
		schema, err := graphql.NewSchema(graphql.SchemaConfig{
			Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: newGatedService().GraphQLQuery()}),
			Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: newGatedService().GraphQLMutation()}),
		})
		if err != nil {
			t.Fatal(err)
		}
		result := graphql.Do(graphql.Params{Schema: schema, RequestString: `mutation { createDatabase(name:"paid-gql", plan:"basic-256mb") { id } }`, Context: context.Background()})
		if len(result.Errors) != 1 || result.Errors[0].Extensions["code"] != "PAYMENT_REQUIRED" || result.Errors[0].Message != core.PaymentRequiredMessage {
			t.Fatalf("errors=%#v", result.Errors)
		}
	})

	t.Run("MCP", func(t *testing.T) {
		srv := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
		newGatedService().RegisterMCP(srv)
		ctx := context.Background()
		serverTransport, clientTransport := mcp.NewInMemoryTransports()
		if _, err := srv.Connect(ctx, serverTransport, nil); err != nil {
			t.Fatal(err)
		}
		clientSession, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil).Connect(ctx, clientTransport, nil)
		if err != nil {
			t.Fatal(err)
		}
		defer clientSession.Close()
		result, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
			Name:      "create_postgres",
			Arguments: map[string]any{"name": "paid-mcp", "plan": "basic-256mb"},
		})
		if err != nil || result == nil || !result.IsError || len(result.Content) != 1 {
			t.Fatalf("result=%#v err=%v", result, err)
		}
		// MCP has no structured error envelope, so the shared registration seam
		// carries the code in the text; the actionable message survives intact.
		text, ok := result.Content[0].(*mcp.TextContent)
		want := core.PaymentRequiredCode + ": " + core.PaymentRequiredMessage
		if !ok || text.Text != want {
			t.Fatalf("MCP content=%#v, want %q (code + actionable payment message)", result.Content, want)
		}
	})
}

func TestPaidIntentGuardCoversPostgresCreateAndBothPlanUpdatePaths(t *testing.T) {
	t.Run("create", func(t *testing.T) {
		svc, _ := newService()
		svc.Workspace = fakeWorkspace{"user-a": "tea-a"}
		gate := &rejectingPaymentGate{}
		svc.Payment = gate
		_, err := svc.CreatePostgres(ctxAs("user-a"), CreatePostgresRequest{Name: "orders", Plan: "basic-256mb"})
		if !errors.Is(err, core.ErrPaymentRequired) || len(gate.calls) != 1 || gate.calls[0] != "tea-a" {
			t.Fatalf("paid create err=%v calls=%v", err, gate.calls)
		}
		gate.calls = nil
		if _, err := svc.CreatePostgres(ctxAs("user-a"), CreatePostgresRequest{Name: "free-db", Plan: "free"}); err != nil {
			t.Fatalf("free create: %v", err)
		}
		if len(gate.calls) != 0 {
			t.Fatalf("free create consulted paid gate: %v", gate.calls)
		}
	})

	database := func() *appv1alpha1.Database {
		return &appv1alpha1.Database{
			ObjectMeta: metav1.ObjectMeta{Name: "dpg-test", Namespace: "default", Labels: map[string]string{core.LabelTenant: "tea-a"}},
			Spec:       appv1alpha1.DatabaseSpec{Name: "orders", Plan: "free"},
		}
	}
	for _, tc := range []struct {
		name string
		run  func(*Service) error
	}{
		{name: "dedicated SetPlan", run: func(s *Service) error { _, err := s.SetPlan(ctxAs("user-a"), "dpg-test", "basic-256mb"); return err }},
		{name: "REST-compatible PATCH plan", run: func(s *Service) error {
			plan := "basic-256mb"
			_, err := s.UpdatePostgres(ctxAs("user-a"), "dpg-test", PostgresPatch{Plan: &plan})
			return err
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, _ := newService(database())
			svc.Workspace = fakeWorkspace{"user-a": "tea-a"}
			gate := &rejectingPaymentGate{}
			svc.Payment = gate
			if err := tc.run(svc); !errors.Is(err, core.ErrPaymentRequired) || len(gate.calls) != 1 {
				t.Fatalf("paid update err=%v calls=%v", err, gate.calls)
			}
		})
	}
}
