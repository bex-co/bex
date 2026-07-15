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

package notifications

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// TestUpdateSettingsThreeSurfaceParity pins deployStarted as a real writable
// field on REST, GraphQL, and MCP. Each adapter drives the same service method
// and returns the identical three-field settings view.
func TestUpdateSettingsThreeSurfaceParity(t *testing.T) {
	want := SettingsView{DeployStarted: false, DeploySucceeded: true, DeployFailed: false}
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "alice"})

	newService := func() *Service {
		return newTestService(newFakeStore(), fakeWorkspace{"alice": "tea-a"}, nil, nil)
	}

	// REST.
	restSvc := newService()
	mux := http.NewServeMux()
	restSvc.RegisterREST(mux)
	req := httptest.NewRequest(http.MethodPatch, "/v1/notification-settings", strings.NewReader(`{"deployStarted":false,"deploySucceeded":true,"deployFailed":false}`)).WithContext(ctx)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("REST status = %d: %s", rec.Code, rec.Body)
	}
	var rest SettingsView
	if err := json.Unmarshal(rec.Body.Bytes(), &rest); err != nil {
		t.Fatalf("REST decode: %v", err)
	}

	// GraphQL.
	gqlSvc := newService()
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: gqlSvc.GraphQLQuery()}),
		Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: gqlSvc.GraphQLMutation()}),
	})
	if err != nil {
		t.Fatalf("GraphQL schema: %v", err)
	}
	result := graphql.Do(graphql.Params{
		Schema:        schema,
		RequestString: `mutation { updateNotificationSettings(deployStarted: false, deploySucceeded: true, deployFailed: false) { deployStarted deploySucceeded deployFailed } }`,
		Context:       ctx,
	})
	if len(result.Errors) > 0 {
		t.Fatalf("GraphQL errors: %v", result.Errors)
	}
	rawGQL, _ := json.Marshal(result.Data)
	var gql struct {
		Settings SettingsView `json:"updateNotificationSettings"`
	}
	if err := json.Unmarshal(rawGQL, &gql); err != nil {
		t.Fatalf("GraphQL decode: %v", err)
	}

	// MCP.
	mcpSvc := newService()
	srv := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	mcpSvc.RegisterMCP(srv)
	serverT, clientT := mcp.NewInMemoryTransports()
	if _, err := srv.Connect(ctx, serverT, nil); err != nil {
		t.Fatalf("MCP server connect: %v", err)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "0"}, nil).Connect(ctx, clientT, nil)
	if err != nil {
		t.Fatalf("MCP client connect: %v", err)
	}
	defer cs.Close()
	mcpResult, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name: "update_notification_settings",
		Arguments: map[string]any{
			"deployStarted": false, "deploySucceeded": true, "deployFailed": false,
		},
	})
	if err != nil || mcpResult.IsError {
		t.Fatalf("MCP update: err=%v result=%+v", err, mcpResult)
	}
	rawMCP, _ := json.Marshal(mcpResult.StructuredContent)
	var mcpSettings SettingsView
	if err := json.Unmarshal(rawMCP, &mcpSettings); err != nil {
		t.Fatalf("MCP decode: %v", err)
	}

	if rest != want || gql.Settings != want || mcpSettings != want {
		t.Fatalf("settings drift: REST=%+v GraphQL=%+v MCP=%+v want=%+v", rest, gql.Settings, mcpSettings, want)
	}
}
