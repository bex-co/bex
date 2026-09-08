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

package projects

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

func TestNameValidation(t *testing.T) {
	for _, verb := range []string{"create", "rename", "graphqlCreate", "graphqlRename", "restCreate", "restRename", "mcpCreate", "mcpRename"} {
		for _, name := range []string{"", " \t\n\u2003", "  production  "} {
			t.Run(fmt.Sprintf("%s/%q", verb, name), func(t *testing.T) {
				st := newFakeProjectStore()
				st.projects["prj-1"] = store.Project{ID: "prj-1", TenantID: "tea-a", Name: "original"}
				svc := &Service{Base: &core.Base{Authz: allowChecker{}}, Store: st}
				ctx := ctxAs("user-a")
				var view ProjectView
				var err error
				switch verb {
				case "create":
					view, err = svc.Create(ctx, "tea-a", name)
				case "rename":
					view, err = svc.Rename(ctx, "prj-1", name)
				case "restCreate", "restRename":
					method, path := http.MethodPost, "/v1/projects"
					args := map[string]any{"name": name, "ownerId": "tea-a"}
					if verb == "restRename" {
						method, path = http.MethodPatch, "/v1/projects/prj-1"
						delete(args, "ownerId")
					}
					raw, _ := json.Marshal(args)
					mux := http.NewServeMux()
					svc.RegisterREST(mux)
					rec := httptest.NewRecorder()
					mux.ServeHTTP(rec, httptest.NewRequest(method, path, strings.NewReader(string(raw))).WithContext(ctx))
					if rec.Code == http.StatusBadRequest {
						err = core.ErrBadRequest
					} else if rec.Code >= 300 {
						t.Fatalf("REST: %d %s", rec.Code, rec.Body.String())
					}
					for _, row := range st.projects {
						if row.Name == "production" {
							view = ProjectView{ID: row.ID, Name: row.Name}
						}
					}
				case "mcpCreate", "mcpRename":
					tool := "create_project"
					args := map[string]any{"name": name}
					if verb == "mcpRename" {
						tool = "update_project"
						args = map[string]any{"id": "prj-1", "name": name}
					}
					result, callErr := newMCPClient(t, ctx, svc).CallTool(ctx, &mcp.CallToolParams{Name: tool, Arguments: args})
					if callErr != nil {
						t.Fatal(callErr)
					}
					if result.IsError {
						raw, _ := json.Marshal(result.Content)
						if !strings.Contains(string(raw), core.ErrBadRequest.Error()) {
							t.Fatalf("MCP: %s", raw)
						}
						err = core.ErrBadRequest
					}
					for _, row := range st.projects {
						if row.Name == "production" {
							view = ProjectView{ID: row.ID, Name: row.Name}
						}
					}
				default:
					field := "createProject"
					if verb == "graphqlRename" {
						field = "renameProject"
					}
					var out any
					out, err = svc.GraphQLMutation()[field].Resolve(graphql.ResolveParams{Context: ctx, Args: map[string]any{"id": "prj-1", "ownerId": "tea-a", "projectId": "prj-1", "name": name}})
					if err == nil {
						view = out.(ProjectView)
					}
				}
				if strings.TrimSpace(name) == "" {
					if !errors.Is(err, core.ErrBadRequest) {
						t.Fatalf("got %v, want bad request", err)
					}
					if len(st.projects) != 1 || st.projects["prj-1"].Name != "original" {
						t.Fatalf("rejected request changed state: %+v", st.projects)
					}
				} else {
					if err != nil || view.Name != "production" {
						t.Fatalf("result: %+v, %v", view, err)
					}
					if st.projects[view.ID].Name != "production" {
						t.Fatal("trimmed name was not persisted")
					}
				}
			})
		}
	}
}
