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
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type multiSandboxWorkspace struct{}

func (multiSandboxWorkspace) Tenant(context.Context, core.Identity) (string, bool) {
	return "tea-a", true
}
func (multiSandboxWorkspace) IsMember(_ context.Context, identity core.Identity, workspace string) (bool, error) {
	return identity.Subject == "id-a" && (workspace == "tea-a" || workspace == "tea-b"), nil
}

// The fake OpenSandbox intentionally returns every workspace's inventory, so
// these adapter tests exercise the real service's workspace and owner filtering.
func TestSandboxWorkspaceAdapters(t *testing.T) {
	for _, surface := range []string{"REST", "GraphQL", "MCP"} {
		for _, selected := range []struct{ name, workspace, own string }{
			{"default", "", "os-default"}, {"selected", "tea-b", "os-selected"},
		} {
			t.Run(surface+"/"+selected.name, func(t *testing.T) {
				records := map[string]string{
					"os-default":     osSandboxJSON("os-default", ""),
					"os-selected":    strings.ReplaceAll(osSandboxJSON("os-selected", ""), "tea-a", "tea-b"),
					"os-other-owner": strings.ReplaceAll(strings.ReplaceAll(osSandboxJSON("os-other-owner", ""), "tea-a", "tea-b"), "id-a", "id-b"),
					"os-foreign":     strings.ReplaceAll(osSandboxJSON("os-foreign", ""), "tea-a", "tea-foreign"),
				}
				var deleted []string
				svc := stubServer(t, func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					switch {
					case r.Method == http.MethodGet && r.URL.Path == "/sandboxes":
						_, _ = fmt.Fprintf(w, "[%s,%s,%s,%s]", records["os-default"], records["os-selected"], records["os-other-owner"], records["os-foreign"])
					case r.Method == http.MethodGet:
						if record, ok := records[strings.TrimPrefix(r.URL.Path, "/sandboxes/")]; ok {
							_, _ = w.Write([]byte(record))
						} else {
							http.NotFound(w, r)
						}
					case r.Method == http.MethodDelete:
						deleted = append(deleted, strings.TrimPrefix(r.URL.Path, "/sandboxes/"))
						w.WriteHeader(http.StatusNoContent)
					default:
						t.Errorf("unexpected upstream mutation: %s %s", r.Method, r.URL.Path)
						http.Error(w, "unexpected", 500)
					}
				})
				svc.Workspace = multiSandboxWorkspace{}
				svc.Authz = adminChecker{}
				call := sandboxWorkspaceAdapter(t, svc, surface)
				body, ok := call("list", selected.workspace, "")
				if !ok || !strings.Contains(body, selected.own) {
					t.Fatalf("list = %s, %v", body, ok)
				}
				for id := range records {
					if id != selected.own && strings.Contains(body, id) {
						t.Fatalf("list leaked %s: %s", id, body)
					}
				}
				if surface != "MCP" {
					body, ok = call("get", selected.workspace, selected.own)
					if !ok || !strings.Contains(body, selected.own) {
						t.Fatalf("get own = %s, %v", body, ok)
					}
				}
				otherWorkspaceID := "os-default"
				if selected.own == otherWorkspaceID {
					otherWorkspaceID = "os-selected"
				}
				for _, target := range []string{"os-other-owner", "os-foreign", otherWorkspaceID} {
					if surface != "MCP" {
						if body, ok := call("get", selected.workspace, target); ok {
							t.Fatalf("read %s allowed: %s", target, body)
						}
					}
					if body, ok := call("terminate", selected.workspace, target); ok {
						t.Fatalf("terminate %s allowed: %s", target, body)
					}
				}
				// Explicitly selecting a workspace the caller does not belong to is denied.
				for _, op := range []string{"list", "get", "terminate"} {
					if surface == "MCP" && op == "get" {
						continue
					}
					if body, ok := call(op, "tea-foreign", "os-foreign"); ok {
						t.Fatalf("foreign workspace %s allowed: %s", op, body)
					}
				}
				if len(deleted) != 0 {
					t.Fatalf("refused requests deleted upstream: %v", deleted)
				}
				body, ok = call("terminate", selected.workspace, selected.own)
				if !ok || len(deleted) != 1 || deleted[0] != selected.own {
					t.Fatalf("terminate own = %s, %v, deleted=%v", body, ok, deleted)
				}
			})
		}
	}
}

func sandboxWorkspaceAdapter(t *testing.T, svc *Service, surface string) func(string, string, string) (string, bool) {
	t.Helper()
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
		Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: svc.GraphQLMutation()}),
	})
	if err != nil {
		t.Fatal(err)
	}
	return func(op, workspace, id string) (string, bool) {
		switch surface {
		case "REST":
			method, path := http.MethodGet, "/v1/sandboxes"
			if op != "list" {
				path += "/" + id
			}
			if op == "terminate" {
				method, path = http.MethodPost, path+"/terminate"
			}
			if workspace != "" {
				path += "?ownerId=" + workspace
			}
			response := httptest.NewRecorder()
			mux.ServeHTTP(response, httptest.NewRequest(method, path, nil).WithContext(callerCtx()))
			return response.Body.String(), response.Code >= 200 && response.Code < 300
		case "GraphQL":
			args := ""
			if workspace != "" {
				args = fmt.Sprintf("ownerId:%q", workspace)
			}
			if op != "list" {
				if args != "" {
					args += ","
				}
				args += fmt.Sprintf("id:%q", id)
			}
			if args != "" {
				args = "(" + args + ")"
			}
			query := "{sandboxes" + args + "{id workspace owner}}"
			if op == "get" {
				query = "{sandbox" + args + "{id workspace owner}}"
			}
			if op == "terminate" {
				query = "mutation{terminateSandbox" + args + "}"
			}
			result := graphql.Do(graphql.Params{Schema: schema, RequestString: query, Context: callerCtx()})
			body, _ := json.Marshal(result)
			return string(body), len(result.Errors) == 0
		case "MCP":
			// The composition-root middleware binds workspaceId before feature handlers.
			// Bind that same context here; middleware itself has separate API tests.
			ctx := core.WithWorkspace(callerCtx(), workspace)
			server := mcp.NewServer(&mcp.Implementation{Name: "sandbox-scope", Version: "0"}, nil)
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
			name, args := "list_sandboxes", map[string]any{}
			if op == "terminate" {
				name, args = "stop_sandbox", map[string]any{"id": id}
			}
			result, err := client.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
			if err != nil {
				t.Fatal(err)
			}
			body, _ := json.Marshal(result)
			return string(body), !result.IsError
		}
		t.Fatalf("unknown surface %q", surface)
		return "", false
	}
}
