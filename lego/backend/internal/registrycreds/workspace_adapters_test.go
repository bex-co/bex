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

package registrycreds

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type credentialWorkspaces struct{}

func (credentialWorkspaces) Tenant(context.Context, core.Identity) (string, bool) {
	return "tea-a", true
}
func (credentialWorkspaces) IsMember(_ context.Context, _ core.Identity, workspace string) (bool, error) {
	return workspace == "tea-a" || workspace == "tea-b", nil
}

type credentialRoles struct{}

func (credentialRoles) Check(_ context.Context, subject, relation, _ string) (bool, error) {
	return relation == core.RelCanView || subject == "user:admin", nil
}

func TestCredentialWorkspaceAdapters(t *testing.T) {
	for _, surface := range []string{"REST", "GraphQL", "MCP"} {
		for _, selected := range []struct{ name, owner, workspace string }{{"default", "", "tea-a"}, {"selected", "tea-b", "tea-b"}} {
			t.Run(surface+"/"+selected.name, func(t *testing.T) {
				svc, st, kv := newTestService()
				svc.Workspace = credentialWorkspaces{}
				svc.Authz = credentialRoles{}
				ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "admin", Method: "session"})
				ids := map[string]string{}
				for _, workspace := range []string{"tea-a", "tea-b"} {
					row, err := svc.Create(ctx, CreateRequest{OwnerID: workspace, Name: "original", Host: "ghcr.io", Username: "registry-user", Secret: "fixture-original-token"})
					if err != nil {
						t.Fatal(err)
					}
					ids[workspace] = row.ID
				}
				call := credentialWorkspaceAdapter(t, svc, surface)
				body, ok := call(ctx, "get", selected.owner, ids[selected.workspace])
				if !ok || !strings.Contains(body, ids[selected.workspace]) || !strings.Contains(body, selected.workspace) {
					t.Fatalf("get selected: %s, %v", body, ok)
				}
				other := "tea-a"
				if selected.workspace == other {
					other = "tea-b"
				}
				beforeRows := maps.Clone(st.rows)
				beforeSecrets := map[string]map[string]string{}
				for path, data := range kv.m {
					beforeSecrets[path] = maps.Clone(data)
				}
				for _, operation := range []string{"get", "update", "delete"} {
					// Both workspaces are memberships, but a credential must still match the selected owner.
					if body, ok := call(ctx, operation, selected.owner, ids[other]); ok {
						t.Fatalf("wrong-owner %s allowed: %s", operation, body)
					}
					if body, ok := call(ctx, operation, "tea-foreign", ids[selected.workspace]); ok {
						t.Fatalf("nonmember %s allowed: %s", operation, body)
					}
				}
				reader := core.WithIdentity(context.Background(), core.Identity{Subject: "reader", Method: "session"})
				if body, ok := call(reader, "get", selected.owner, ids[selected.workspace]); !ok {
					t.Fatalf("member read denied: %s", body)
				}
				for _, operation := range []string{"update", "delete"} {
					if body, ok := call(reader, operation, selected.owner, ids[selected.workspace]); ok {
						t.Fatalf("nonadmin %s allowed: %s", operation, body)
					}
				}
				if !reflect.DeepEqual(beforeRows, st.rows) || !reflect.DeepEqual(beforeSecrets, kv.m) {
					t.Fatal("unauthorized calls changed credential rows or secrets")
				}
				body, ok = call(ctx, "update", selected.owner, ids[selected.workspace])
				if !ok || !strings.Contains(body, "renamed") {
					t.Fatalf("update selected: %s, %v", body, ok)
				}
				if st.rows[ids[selected.workspace]].Name != "renamed" || kv.m[secretPath(selected.workspace, ids[selected.workspace])]["password"] != "fixture-rotated-token" {
					t.Fatal("update did not change selected row and secret")
				}
				body, ok = call(ctx, "delete", selected.owner, ids[selected.workspace])
				if !ok {
					t.Fatalf("delete selected: %s", body)
				}
				if _, exists := st.rows[ids[selected.workspace]]; exists {
					t.Fatal("delete retained selected row")
				}
				if _, exists := kv.m[secretPath(selected.workspace, ids[selected.workspace])]; exists {
					t.Fatal("delete retained selected secret")
				}
				if !reflect.DeepEqual(beforeRows[ids[other]], st.rows[ids[other]]) || !reflect.DeepEqual(beforeSecrets[secretPath(other, ids[other])], kv.m[secretPath(other, ids[other])]) {
					t.Fatal("successful mutation touched other workspace")
				}
			})
		}
	}
}

func credentialWorkspaceAdapter(t *testing.T, svc *Service, surface string) func(context.Context, string, string, string) (string, bool) {
	t.Helper()
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	schema := testSchema(svc)
	return func(ctx context.Context, operation, owner, id string) (string, bool) {
		var body string
		var ok bool
		switch surface {
		case "REST":
			method := map[string]string{"get": http.MethodGet, "update": http.MethodPatch, "delete": http.MethodDelete}[operation]
			path := "/v1/registrycredentials/" + id
			if owner != "" {
				path += "?ownerId=" + owner
			}
			response := httptest.NewRecorder()
			request := httptest.NewRequest(method, path, strings.NewReader(`{"name":"renamed","authToken":"fixture-rotated-token"}`)).WithContext(ctx)
			mux.ServeHTTP(response, request)
			body, ok = response.Body.String(), response.Code >= 200 && response.Code < 300
		case "GraphQL":
			args := fmt.Sprintf("id:%q", id)
			if owner != "" {
				args += fmt.Sprintf(",ownerId:%q", owner)
			}
			query := "{registryCredential(" + args + "){id name ownerId}}"
			if operation == "update" {
				query = "mutation{updateRegistryCredential(" + args + `,name:"renamed",authToken:"fixture-rotated-token"){id name ownerId}}`
			}
			if operation == "delete" {
				query = "mutation{deleteRegistryCredential(" + args + ")}"
			}
			result := graphql.Do(graphql.Params{Schema: schema, Context: ctx, RequestString: query})
			raw, _ := json.Marshal(result)
			body, ok = string(raw), len(result.Errors) == 0
		case "MCP":
			// The shared API middleware supplies workspaceId through this context.
			ctx = core.WithWorkspace(ctx, owner)
			server := mcp.NewServer(&mcp.Implementation{Name: "registry-scope", Version: "0"}, nil)
			svc.RegisterMCP(server)
			serverTransport, clientTransport := mcp.NewInMemoryTransports()
			if _, err := server.Connect(ctx, serverTransport, nil); err != nil {
				t.Fatal(err)
			}
			client, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil).Connect(ctx, clientTransport, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer client.Close()
			args := map[string]any{"id": id}
			if operation == "update" {
				args["name"], args["authToken"] = "renamed", "fixture-rotated-token"
			}
			result, err := client.CallTool(ctx, &mcp.CallToolParams{Name: operation + "_registry_credential", Arguments: args})
			if err != nil {
				t.Fatal(err)
			}
			raw, _ := json.Marshal(result)
			body, ok = string(raw), !result.IsError
		default:
			t.Fatalf("unknown surface %s", surface)
		}
		if strings.Contains(body, "fixture-original-token") || strings.Contains(body, "fixture-rotated-token") {
			t.Fatal("adapter leaked secret")
		}
		return body, ok
	}
}
