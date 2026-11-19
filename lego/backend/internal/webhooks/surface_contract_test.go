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
	"fmt"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/core/coretest"
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

func TestGraphQLWebhookAttemptEvidenceAndIdempotentResend(t *testing.T) {
	svc, st := newTestService()
	created, err := svc.Create(t.Context(), CreateRequest{
		Name: "graphql-attempts", URL: "https://hooks.example.com", EventTypes: []string{}, Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	sent := time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC)
	next := sent.Add(10 * time.Minute)
	st.deliveries[created.ID] = []store.WebhookAttempt{{
		ID: "whd-gql-source", NotificationID: "whd-gql-parent", EndpointID: created.ID,
		EventID: "evt-gql", EventType: TypeDeployEnded, ServiceID: "srv-gql",
		Status: store.WebhookAttemptFailed, AttemptNumber: 1, StatusCode: 503,
		TransportError: "", ResponseBody: "unavailable", Payload: `{"type":"deploy_ended"}`,
		SentAt: &sent, NextAttemptAt: &next, ParentStatus: store.WebhookAttemptPending, CreatedAt: sent,
	}}
	schema := webhookGraphQLSchema(t, svc)
	history := graphql.Do(graphql.Params{
		Schema: schema, Context: t.Context(),
		RequestString: fmt.Sprintf(`{ webhookDeliveries(endpointId:%q, status:"failed") { id eventId eventType serviceId status attemptNumber statusCode transportError responseBody requestBody sentAt nextAttemptAt parentStatus cursor } }`, created.ID),
	})
	if len(history.Errors) != 0 {
		t.Fatalf("history errors: %v", history.Errors)
	}
	raw, _ := json.Marshal(history.Data)
	var got struct {
		Deliveries []DeliveryView `json:"webhookDeliveries"`
	}
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Deliveries) != 1 || got.Deliveries[0].RequestBody != `{"type":"deploy_ended"}` ||
		got.Deliveries[0].ParentStatus != DeliveryPending || got.Deliveries[0].NextAttemptAt != next.Format(time.RFC3339) {
		t.Fatalf("GraphQL history = %+v", got.Deliveries)
	}

	mutation := fmt.Sprintf(`mutation { resendWebhookDelivery(endpointId:%q, attemptId:"whd-gql-source", idempotencyKey:"graphql-resend-0001") { id eventId status attemptNumber requestBody } }`, created.ID)
	first := graphql.Do(graphql.Params{Schema: schema, Context: t.Context(), RequestString: mutation})
	second := graphql.Do(graphql.Params{Schema: schema, Context: t.Context(), RequestString: mutation})
	if len(first.Errors) != 0 || len(second.Errors) != 0 {
		t.Fatalf("resend errors: first=%v second=%v", first.Errors, second.Errors)
	}
	var one, two struct {
		Delivery DeliveryView `json:"resendWebhookDelivery"`
	}
	raw, _ = json.Marshal(first.Data)
	_ = json.Unmarshal(raw, &one)
	raw, _ = json.Marshal(second.Data)
	_ = json.Unmarshal(raw, &two)
	if one.Delivery.ID == "" || one.Delivery.ID != two.Delivery.ID || one.Delivery.Status != DeliveryPending ||
		one.Delivery.EventID != "evt-gql" || one.Delivery.RequestBody != `{"type":"deploy_ended"}` {
		t.Fatalf("GraphQL idempotent resend = first %+v second %+v", one.Delivery, two.Delivery)
	}
}

// webhookMCPClient connects an in-memory MCP client to svc's tools.
func webhookMCPClient(t *testing.T, ctx context.Context, svc *Service) *mcp.ClientSession {
	t.Helper()
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
	return client
}

func TestMCPWebhookDeliveryDiagnosticsAndValidation(t *testing.T) {
	svc, st := newTestService()
	ctx := context.Background()
	client := webhookMCPClient(t, ctx, svc)

	createdResult, err := client.CallTool(ctx, &mcp.CallToolParams{
		Name: "create_webhook_endpoint",
		Arguments: map[string]any{
			"name": "agent-hook", "url": "https://hooks.example.com", "eventTypes": []string{}, "enabled": true,
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
	if created.ID == "" || !created.Enabled || len(created.EventFilter) != 0 || created.Secret == "" {
		t.Fatalf("MCP create = %+v", created)
	}

	sent := time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC)
	st.deliveries[created.ID] = []store.WebhookAttempt{{
		ID: "whd-1", NotificationID: "whd-parent", EndpointID: created.ID, EventID: "evt-1", EventType: TypeBuildEnded,
		ServiceID: "srv-1", AttemptNumber: 1, Status: store.WebhookAttemptFailed,
		StatusCode: 502, TransportError: "endpoint answered 502", ResponseBody: "upstream unavailable",
		Payload: `{"type":"build_ended"}`, ParentStatus: store.WebhookAttemptPending, SentAt: &sent, CreatedAt: sent,
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
	if len(history.Deliveries) != 1 || history.Deliveries[0].ResponseBody != "upstream unavailable" ||
		history.Deliveries[0].RequestBody != `{"type":"build_ended"}` || history.Deliveries[0].ParentStatus != DeliveryPending ||
		history.Deliveries[0].SentAt != sent.Format(time.RFC3339) {
		t.Fatalf("MCP delivery diagnostics = %+v", history.Deliveries)
	}

	firstResendID := ""
	for i := range 2 {
		resendResult, err := client.CallTool(ctx, &mcp.CallToolParams{
			Name: "resend_webhook_delivery", Arguments: map[string]any{
				"endpointId": created.ID, "attemptId": "whd-1", "idempotencyKey": "mcp-resend-0001",
			},
		})
		if err != nil || resendResult.IsError {
			t.Fatalf("MCP resend %d: err=%v result=%+v", i, err, resendResult)
		}
		raw, _ = json.Marshal(resendResult.StructuredContent)
		var resent DeliveryView
		if err := json.Unmarshal(raw, &resent); err != nil {
			t.Fatal(err)
		}
		if resent.Status != DeliveryPending || resent.EventID != "evt-1" || resent.RequestBody != `{"type":"build_ended"}` {
			t.Fatalf("MCP resend = %+v", resent)
		}
		if i == 0 {
			firstResendID = resent.ID
		} else if resent.ID != firstResendID {
			t.Fatalf("duplicate MCP resend id = %q, want %q", resent.ID, firstResendID)
		}
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

// w3/m282: GraphQL reads return the signing secret to a workspace manager and
// null to a member, the same rule REST and MCP render from the core view.
func TestGraphQLWebhookSecretFollowsTheManagerBoundary(t *testing.T) {
	st := newFakeEndpointStore()
	svc := &Service{Base: &core.Base{
		Namespace: "default", Workspace: coretest.Workspaces{"tea-a"}, Authz: manageChecker{admin: "id-admin"},
	}, Store: st}
	adminCtx := core.WithIdentity(context.Background(), core.Identity{Subject: "id-admin", Method: "session"})
	memberCtx := core.WithIdentity(context.Background(), core.Identity{Subject: "id-member", Method: "session"})
	created, err := svc.Create(adminCtx, CreateRequest{Name: "hook", URL: "https://hooks.example.com/x", EventTypes: []string{}, Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	schema := webhookGraphQLSchema(t, svc)
	secretOf := func(ctx context.Context) (one, listed any) {
		t.Helper()
		res := graphql.Do(graphql.Params{
			Schema: schema, Context: ctx,
			RequestString: fmt.Sprintf(`{ webhookEndpoint(id:%q) { secret } webhookEndpoints { secret } }`, created.ID),
		})
		if len(res.Errors) != 0 {
			t.Fatalf("errors: %v", res.Errors)
		}
		data := res.Data.(map[string]any)
		return data["webhookEndpoint"].(map[string]any)["secret"], data["webhookEndpoints"].([]any)[0].(map[string]any)["secret"]
	}
	if one, listed := secretOf(adminCtx); one != created.Secret || listed != created.Secret {
		t.Fatalf("manager secret = %v / %v, want %q", one, listed, created.Secret)
	}
	if one, listed := secretOf(memberCtx); one != nil || listed != nil {
		t.Fatalf("member secret = %v / %v, want null", one, listed)
	}
}

// w4/709: unemitted values from Render's pinned event enum (maintenance_started,
// maintenance_ended, postgres_connection_pool_changed) are accepted, stored,
// and read back in canonical order across REST, GraphQL, and MCP without
// entering the advertised event-type lists; truly unknown values still 400.
func TestUnemittedRenderEventTypesAcceptedAcrossRESTGraphQLAndMCP(t *testing.T) {
	svc, _ := newTestService()
	ctx := context.Background()

	// 1. REST: POST accepts mixed emitted + unemitted in canonical order, GET
	// and PATCH preserve them, unknown values still 400, and /event-types stays
	// emitted-only.
	createdRR := webhookREST(t, svc, http.MethodPost, "/v1/webhooks",
		`{"ownerId":"default","name":"render-cfg","url":"https://hooks.example.com/r","enabled":true,"eventFilter":["maintenance_started","deploy_ended"]}`)
	if createdRR.Code != http.StatusCreated {
		t.Fatalf("REST create = %d %s", createdRR.Code, createdRR.Body.String())
	}
	var restCreated endpointWire
	if err := json.Unmarshal(createdRR.Body.Bytes(), &restCreated); err != nil {
		t.Fatal(err)
	}
	wantInitial := []string{TypeDeployEnded, TypeMaintenanceStarted}
	if !slices.Equal(restCreated.EventFilter, wantInitial) {
		t.Fatalf("REST create eventFilter = %v, want %v", restCreated.EventFilter, wantInitial)
	}

	getRR := webhookREST(t, svc, http.MethodGet, "/v1/webhooks/"+restCreated.ID, "")
	if getRR.Code != http.StatusOK {
		t.Fatalf("REST get = %d %s", getRR.Code, getRR.Body.String())
	}
	var restGot endpointWire
	if err := json.Unmarshal(getRR.Body.Bytes(), &restGot); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(restGot.EventFilter, wantInitial) {
		t.Fatalf("REST get eventFilter = %v, want %v", restGot.EventFilter, wantInitial)
	}

	badRR := webhookREST(t, svc, http.MethodPost, "/v1/webhooks",
		`{"ownerId":"default","name":"bad-type","url":"https://hooks.example.com/bad","enabled":true,"eventFilter":["not_a_type"]}`)
	if badRR.Code != http.StatusBadRequest || !strings.Contains(badRR.Body.String(), WebhookEventFilterInvalidCode) {
		t.Fatalf("REST unknown event = %d %s", badRR.Code, badRR.Body.String())
	}

	vocabRR := webhookREST(t, svc, http.MethodGet, "/v1/webhooks/event-types", "")
	if vocabRR.Code != http.StatusOK {
		t.Fatalf("REST event-types = %d %s", vocabRR.Code, vocabRR.Body.String())
	}
	var restVocab []string
	if err := json.Unmarshal(vocabRR.Body.Bytes(), &restVocab); err != nil {
		t.Fatal(err)
	}
	for _, unemitted := range renderUnemittedEventTypes {
		if slices.Contains(restVocab, unemitted) {
			t.Errorf("GET /v1/webhooks/event-types advertises unemitted %q", unemitted)
		}
	}

	// 2. GraphQL: reads back REST's unemitted filter, updates it to include all
	// three unemitted Render types, and keeps webhookEventTypes emitted-only.
	schema := webhookGraphQLSchema(t, svc)
	gqlRead := graphql.Do(graphql.Params{
		Schema: schema, Context: ctx,
		RequestString: fmt.Sprintf(`{ webhookEndpoint(id:%q) { eventTypes } webhookEventTypes }`, restCreated.ID),
	})
	if len(gqlRead.Errors) != 0 {
		t.Fatalf("GraphQL read errors: %v", gqlRead.Errors)
	}
	raw, _ := json.Marshal(gqlRead.Data)
	var gqlData struct {
		Endpoint   struct{ EventTypes []string } `json:"webhookEndpoint"`
		EventTypes []string                      `json:"webhookEventTypes"`
	}
	if err := json.Unmarshal(raw, &gqlData); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(gqlData.Endpoint.EventTypes, wantInitial) {
		t.Fatalf("GraphQL read eventTypes = %v, want %v", gqlData.Endpoint.EventTypes, wantInitial)
	}
	for _, unemitted := range renderUnemittedEventTypes {
		if slices.Contains(gqlData.EventTypes, unemitted) {
			t.Errorf("GraphQL webhookEventTypes advertises unemitted %q", unemitted)
		}
	}

	gqlPatch := graphql.Do(graphql.Params{
		Schema: schema, Context: ctx,
		RequestString: fmt.Sprintf(`mutation { updateWebhookEndpoint(id:%q, eventTypes:["postgres_connection_pool_changed","maintenance_ended","deploy_ended","maintenance_started"]) { eventTypes } }`, restCreated.ID),
	})
	if len(gqlPatch.Errors) != 0 {
		t.Fatalf("GraphQL update errors: %v", gqlPatch.Errors)
	}
	wantAll := []string{TypeDeployEnded, TypeMaintenanceEnded, TypeMaintenanceStarted, TypePostgresConnectionPoolChanged}
	raw, _ = json.Marshal(gqlPatch.Data)
	var gqlPatched struct {
		Endpoint struct{ EventTypes []string } `json:"updateWebhookEndpoint"`
	}
	if err := json.Unmarshal(raw, &gqlPatched); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(gqlPatched.Endpoint.EventTypes, wantAll) {
		t.Fatalf("GraphQL update eventTypes = %v, want %v", gqlPatched.Endpoint.EventTypes, wantAll)
	}

	// 3. MCP: reads back the endpoint with unemitted types, creates/updates one,
	// and keeps list_webhook_endpoints.eventTypes emitted-only.
	client := webhookMCPClient(t, ctx, svc)
	listed, err := client.CallTool(ctx, &mcp.CallToolParams{Name: "list_webhook_endpoints"})
	if err != nil || listed.IsError {
		t.Fatalf("MCP list: err=%v result=%+v", err, listed)
	}
	raw, _ = json.Marshal(listed.StructuredContent)
	var mcpList listEndpointsResult
	if err := json.Unmarshal(raw, &mcpList); err != nil {
		t.Fatal(err)
	}
	if len(mcpList.Endpoints) != 1 || !slices.Equal(mcpList.Endpoints[0].EventFilter, wantAll) {
		t.Fatalf("MCP list endpoints = %+v, want %v", mcpList.Endpoints, wantAll)
	}
	for _, unemitted := range renderUnemittedEventTypes {
		if slices.Contains(mcpList.EventTypes, unemitted) {
			t.Errorf("MCP list_webhook_endpoints.eventTypes advertises unemitted %q", unemitted)
		}
	}

	mcpCreatedRes, err := client.CallTool(ctx, &mcp.CallToolParams{
		Name: "create_webhook_endpoint",
		Arguments: map[string]any{
			"name":       "mcp-render-cfg",
			"url":        "https://hooks.example.com/mcp",
			"eventTypes": []string{TypeMaintenanceStarted, TypeDeployEnded},
			"enabled":    true,
		},
	})
	if err != nil || mcpCreatedRes.IsError {
		t.Fatalf("MCP create: err=%v result=%+v", err, mcpCreatedRes)
	}
	raw, _ = json.Marshal(mcpCreatedRes.StructuredContent)
	var mcpCreated endpointWire
	if err := json.Unmarshal(raw, &mcpCreated); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(mcpCreated.EventFilter, wantInitial) {
		t.Fatalf("MCP create eventFilter = %v, want %v", mcpCreated.EventFilter, wantInitial)
	}

	// PATCH on REST preserves the unemitted filter when updating another field,
	// and accepts replacing the filter with unemitted values.
	patchRR := webhookREST(t, svc, http.MethodPatch, "/v1/webhooks/"+mcpCreated.ID,
		`{"eventFilter":["postgres_connection_pool_changed","maintenance_started"]}`)
	if patchRR.Code != http.StatusOK {
		t.Fatalf("REST patch = %d %s", patchRR.Code, patchRR.Body.String())
	}
	var restPatched endpointWire
	if err := json.Unmarshal(patchRR.Body.Bytes(), &restPatched); err != nil {
		t.Fatal(err)
	}
	wantPatched := []string{TypeMaintenanceStarted, TypePostgresConnectionPoolChanged}
	if !slices.Equal(restPatched.EventFilter, wantPatched) {
		t.Fatalf("REST patch eventFilter = %v, want %v", restPatched.EventFilter, wantPatched)
	}
}
