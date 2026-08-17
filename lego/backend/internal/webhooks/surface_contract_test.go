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

package webhooks

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

func webhookGraphQLSchema(t *testing.T, svc *Service) graphql.Schema {
	t.Helper()
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
		Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: svc.GraphQLMutation()}),
	})
	if err != nil {
		t.Fatal(err)
	}
	return schema
}

func TestGraphQLWebhookSemanticsShareCoreValidation(t *testing.T) {
	svc, _ := newTestService()
	schema := webhookGraphQLSchema(t, svc)
	ctx := context.Background()

	created := graphql.Do(graphql.Params{
		Schema: schema, Context: ctx,
		RequestString: `mutation { createWebhookEndpoint(name:"disabled", url:"https://hooks.example.com", eventTypes:[], enabled:false) { id enabled eventTypes secret } }`,
	})
	if len(created.Errors) > 0 {
		t.Fatalf("create errors: %v", created.Errors)
	}
	raw, _ := json.Marshal(created.Data)
	var result struct {
		Endpoint struct {
			ID         string   `json:"id"`
			Enabled    bool     `json:"enabled"`
			EventTypes []string `json:"eventTypes"`
			Secret     string   `json:"secret"`
		} `json:"createWebhookEndpoint"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatal(err)
	}
	if result.Endpoint.ID == "" || result.Endpoint.Enabled || len(result.Endpoint.EventTypes) != 0 || result.Endpoint.Secret == "" {
		t.Fatalf("GraphQL create result = %+v", result.Endpoint)
	}

	for _, query := range []string{
		`mutation { createWebhookEndpoint(name:"bad-url", url:"http://hooks.example.com", eventTypes:[], enabled:true) { id } }`,
		`mutation { createWebhookEndpoint(name:"disabled", url:"https://other.example.com", eventTypes:[], enabled:true) { id } }`,
	} {
		failed := graphql.Do(graphql.Params{Schema: schema, Context: ctx, RequestString: query})
		if len(failed.Errors) != 1 {
			t.Fatalf("GraphQL refusal errors = %v", failed.Errors)
		}
		code, _ := failed.Errors[0].Extensions["code"].(string)
		if code != WebhookURLInvalidCode && code != WebhookNameConflictCode {
			t.Fatalf("GraphQL lost coded core refusal: message=%q extensions=%v", failed.Errors[0].Message, failed.Errors[0].Extensions)
		}
	}
}

func TestMCPWebhookDeliveryDiagnosticsAndValidation(t *testing.T) {
	svc, st := newTestService()
	ctx := context.Background()
	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	svc.RegisterMCP(server)
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	if _, err := server.Connect(ctx, serverTransport, nil); err != nil {
		t.Fatal(err)
	}
	client, err := mcp.NewClient(&mcp.Implementation{Name: "client", Version: "0"}, nil).Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })

	createdResult, err := client.CallTool(ctx, &mcp.CallToolParams{
		Name: "create_webhook_endpoint",
		Arguments: map[string]any{
			"name": "agent-hook", "url": "https://hooks.example.com", "eventTypes": []string{}, "enabled": false,
		},
	})
	if err != nil || createdResult.IsError {
		t.Fatalf("MCP create: err=%v result=%+v", err, createdResult)
	}
	raw, _ := json.Marshal(createdResult.StructuredContent)
	var created endpointWire
	if err := json.Unmarshal(raw, &created); err != nil {
		t.Fatal(err)
	}
	if created.ID == "" || created.Enabled || len(created.EventFilter) != 0 || created.Secret == "" {
		t.Fatalf("MCP create = %+v", created)
	}

	sent := time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC)
	st.deliveries[created.ID] = []store.WebhookDelivery{{
		ID: "whd-1", EndpointID: created.ID, EventID: "evt-1", EventType: TypeBuildEnded,
		ServiceID: "srv-1", AttemptCount: 8, LastStatus: 502, LastError: "endpoint answered 502",
		ResponseBody: "upstream unavailable", SentAt: &sent, CreatedAt: sent,
	}}
	historyResult, err := client.CallTool(ctx, &mcp.CallToolParams{
		Name: "list_webhook_deliveries", Arguments: map[string]any{"id": created.ID},
	})
	if err != nil || historyResult.IsError {
		t.Fatalf("MCP history: err=%v result=%+v", err, historyResult)
	}
	raw, _ = json.Marshal(historyResult.StructuredContent)
	var history listDeliveriesResult
	if err := json.Unmarshal(raw, &history); err != nil {
		t.Fatal(err)
	}
	if len(history.Deliveries) != 1 || history.Deliveries[0].ResponseBody != "upstream unavailable" || history.Deliveries[0].SentAt != sent.Format(time.RFC3339) {
		t.Fatalf("MCP delivery diagnostics = %+v", history.Deliveries)
	}

	bad, err := client.CallTool(ctx, &mcp.CallToolParams{
		Name: "create_webhook_endpoint",
		Arguments: map[string]any{
			"name": "bad", "url": "http://hooks.example.com", "eventTypes": []string{}, "enabled": true,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	content, _ := json.Marshal(bad.Content)
	if !bad.IsError || !strings.Contains(string(content), WebhookURLInvalidCode) {
		t.Fatalf("MCP lost coded core refusal: %+v content=%s", bad, content)
	}
}
