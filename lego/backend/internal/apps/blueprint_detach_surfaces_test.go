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

package apps

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const detachReplacementManifest = `services:
  - name: remaining
    type: web
    runtime: image
    image: {url: nginx:1}
`

func detachSurfaceMCP(t *testing.T, svc *Service) func(string, map[string]any) (map[string]any, bool) {
	t.Helper()
	server := mcp.NewServer(&mcp.Implementation{Name: "detach-test", Version: "0"}, nil)
	svc.RegisterMCP(server)
	a, b := mcp.NewInMemoryTransports()
	ctx := ownershipCtx()
	if _, err := server.Connect(ctx, a, nil); err != nil {
		t.Fatal(err)
	}
	session, err := mcp.NewClient(&mcp.Implementation{Name: "detach-test", Version: "0"}, nil).Connect(ctx, b, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	return func(tool string, args map[string]any) (map[string]any, bool) {
		result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: tool, Arguments: args})
		if err != nil {
			t.Fatal(err)
		}
		var out map[string]any
		encoded, _ := json.Marshal(result.StructuredContent)
		if err := json.Unmarshal(encoded, &out); err != nil {
			t.Fatal(err)
		}
		return out, result.IsError
	}
}

func detachSurfaceFixture(t *testing.T) (*Service, *fakeBlueprintStore, BlueprintView) {
	t.Helper()
	svc, fs := connectionService(t)
	bp := connectA(t, svc)
	svc.GitFetcher = multiPathFetcher{byPath: map[string]string{pathSite: detachReplacementManifest}}
	return svc, fs, bp
}

func TestBlueprintDetachPlanAcrossSurfaces(t *testing.T) {
	for _, surface := range []string{"REST", "GraphQL", "MCP"} {
		t.Run(surface, func(t *testing.T) {
			svc, fs, bp := detachSurfaceFixture(t)
			foreignID := id.New(id.Blueprint)
			fs.blueprints[foreignID] = store.Blueprint{ID: foreignID, TenantID: "tea-foreign", Repo: repoBex, Branch: "main", Manifest: m125SiteManifest}
			mux := http.NewServeMux()
			svc.RegisterREST(mux)
			schema := blueprintSchema(t, svc)
			call := detachSurfaceMCP(t, svc)
			for _, preview := range []bool{false, true} {
				for _, providedID := range []string{"", bp.ID, foreignID} {
					t.Run(fmt.Sprintf("preview=%t/id=%s", preview, providedID), func(t *testing.T) {
						args := map[string]any{"blueprintId": providedID}
						field, tool, path := "validateBlueprint", "validate_bex_yml", "validate"
						if preview {
							field, tool, path = "blueprintPreview", "preview_blueprint", "preview"
							args["repo"], args["branch"], args["path"] = repoBex, "main", pathSite
						} else {
							args["bexYaml"] = detachReplacementManifest
						}
						var result map[string]any
						refused := false
						switch surface {
						case "REST":
							args["ownerId"] = connOwner
							body, _ := json.Marshal(args)
							rec := httptest.NewRecorder()
							mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/blueprints/"+path, strings.NewReader(string(body))).WithContext(ownershipCtx()))
							refused = rec.Code != http.StatusOK
							if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
								t.Fatal(err)
							}
						case "GraphQL":
							fields := `valid plan { actions { operation name resourceId message } }`
							argument := fmt.Sprintf(`bexYaml:%q`, detachReplacementManifest)
							if preview {
								argument = fmt.Sprintf(`repo:%q,branch:"main",path:%q`, repoBex, pathSite)
								fields = "validation {" + fields + "}"
							}
							q := fmt.Sprintf(`{%s(%s,ownerId:%q,blueprintId:%q){%s}}`, field, argument, connOwner, providedID, fields)
							response := graphql.Do(graphql.Params{Schema: schema, Context: ownershipCtx(), RequestString: q})
							refused = len(response.Errors) > 0
							if !refused {
								result = response.Data.(map[string]any)[field].(map[string]any)
							}
						case "MCP":
							result, refused = call(tool, args)
						}
						if providedID == foreignID {
							if !refused {
								t.Fatal("foreign blueprint accepted")
							}
							return
						}
						if refused {
							t.Fatalf("surface refused own/no blueprint: %v", result)
						}
						if preview {
							result = result["validation"].(map[string]any)
						}
						encoded, _ := json.Marshal(result)
						var validation BlueprintValidation
						if err := json.Unmarshal(encoded, &validation); err != nil {
							t.Fatal(err)
						}
						if validation.Plan == nil {
							t.Fatal("missing validation plan")
						}
						detaches := 0
						for _, action := range validation.Plan.Actions {
							if action.Operation == "detach" {
								detaches++
								if action.Name != "static-site" || action.ResourceID == "" || action.Message == "" {
									t.Fatalf("incomplete detach explanation: %+v", action)
								}
							}
						}
						want := 0
						if providedID == bp.ID {
							want = 1
						}
						if detaches != want {
							t.Fatalf("detaches=%d want%d plan=%+v", detaches, want, validation.Plan)
						}
					})
				}
			}
			if owner := appOwner(t, svc, "static-site"); owner != bp.ID {
				t.Fatal("preview changed ownership")
			}
		})
	}
}

func TestBlueprintDetachSyncResultAcrossSurfaces(t *testing.T) {
	for _, surface := range []string{"REST", "GraphQL", "MCP"} {
		t.Run(surface, func(t *testing.T) {
			svc, fs, bp := detachSurfaceFixture(t)
			var result map[string]any
			switch surface {
			case "REST":
				mux := http.NewServeMux()
				svc.RegisterREST(mux)
				body, _ := json.Marshal(map[string]any{"ownerId": connOwner, "bexYaml": detachReplacementManifest})
				rec := httptest.NewRecorder()
				mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/blueprints/"+bp.ID+"/sync?ownerId="+connOwner, strings.NewReader(string(body))).WithContext(ownershipCtx()))
				if rec.Code != http.StatusOK {
					t.Fatalf("sync:%d %s", rec.Code, rec.Body.String())
				}
				if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
					t.Fatal(err)
				}
			case "GraphQL":
				query := fmt.Sprintf(`mutation {syncBlueprint(id:%q,ownerId:%q,bexYaml:%q){detachedResources{id name type}}}`, bp.ID, connOwner, detachReplacementManifest)
				response := graphql.Do(graphql.Params{Schema: blueprintSchema(t, svc), Context: ownershipCtx(), RequestString: query})
				if len(response.Errors) > 0 {
					t.Fatal(response.Errors)
				}
				result = response.Data.(map[string]any)["syncBlueprint"].(map[string]any)
			case "MCP":
				call := detachSurfaceMCP(t, svc)
				var refused bool
				result, refused = call("sync_blueprint", map[string]any{"id": bp.ID, "bexYaml": detachReplacementManifest})
				if refused {
					t.Fatal("MCP sync refused")
				}
			}
			rows, ok := result["detachedResources"].([]any)
			if !ok || len(rows) != 1 {
				t.Fatalf("sync detach result:%+v", result)
			}
			resource := rows[0].(map[string]any)
			if resource["name"] != "static-site" || resource["id"] == "" || resource["type"] == "" {
				t.Fatalf("incomplete detach result:%v", resource)
			}
			if owner := appOwner(t, svc, "static-site"); owner != "" {
				t.Fatalf("detached workload still claimed:%q", owner)
			}
			if !strings.Contains(fs.syncs[fs.lastSyncUpdate.ID].Note, "static-site") {
				t.Fatalf("sync history omitted detach:%q", fs.syncs[fs.lastSyncUpdate.ID].Note)
			}
		})
	}
}
