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
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/graphql-go/graphql"
)

func TestBlueprintEmptyEnvironmentValidationSurfaces(t *testing.T) {
	manifest := emptyEnvironmentManifest("static") + "    envVars: []\n"
	svc, cl := newService(nil)
	if _, err := svc.DeployStack(context.Background(), DeployRequest{Manifest: manifest}); err != nil {
		t.Fatal(err)
	}
	persisted := serializedBlueprintApp(t, getApp(t, cl, "empty-env"))
	if persisted.Spec.Env != nil {
		t.Fatalf("fixture env must deserialize nil: %#v", persisted.Spec.Env)
	}
	svc.Client = fakeClient(persisted)
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	schema := blueprintSchema(t, svc)
	call, cleanup := appsMCPClient(t, svc)
	defer cleanup()

	for _, surface := range []struct {
		name     string
		validate func(*testing.T, string) map[string]any
	}{
		{"REST multipart", func(t *testing.T, yaml string) map[string]any {
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, multipartBlueprintRequest(t, "tea-workspace", "render.yaml", yaml))
			if rec.Code != http.StatusOK {
				t.Fatalf("validate status=%d body=%s", rec.Code, rec.Body)
			}
			var out map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
				t.Fatal(err)
			}
			return out
		}},
		{"GraphQL", func(t *testing.T, yaml string) map[string]any {
			result := graphql.Do(graphql.Params{Schema: schema, Context: context.Background(), RequestString: fmt.Sprintf(`{ validateBlueprint(bexYaml: %q) { valid plan { actions { operation name changedFields { path } } } } }`, yaml)})
			if len(result.Errors) != 0 {
				t.Fatal(result.Errors)
			}
			return result.Data.(map[string]any)["validateBlueprint"].(map[string]any)
		}},
		{"MCP", func(t *testing.T, yaml string) map[string]any {
			return call("validate_bex_yml", map[string]any{"bexYaml": yaml})
		}},
	} {
		t.Run(surface.name, func(t *testing.T) {
			for _, tc := range []struct {
				name, yaml, operation, resourceName string
				paths                               []string
			}{
				{"empty", manifest, "noop", "empty-env", nil},
				{"omitted", emptyEnvironmentManifest("static"), "noop", "empty-env", nil},
				{"real change", strings.Replace(manifest, "staticPublishPath: .", "staticPublishPath: dist", 1), "update", "empty-env", []string{"staticPublishPath"}},
				{"new resource", strings.Replace(manifest, "name: empty-env", "name: new-site", 1), "create", "new-site", nil},
			} {
				t.Run(tc.name, func(t *testing.T) {
					out := surface.validate(t, tc.yaml)
					if out["valid"] != true {
						t.Fatalf("invalid: %+v", out)
					}
					plan, ok := out["plan"].(map[string]any)
					if !ok {
						t.Fatalf("missing plan: %+v", out)
					}
					actions, ok := plan["actions"].([]any)
					if !ok || len(actions) != 1 {
						t.Fatalf("actions=%+v", plan["actions"])
					}
					action := actions[0].(map[string]any)
					if action["operation"] != tc.operation || action["name"] != tc.resourceName {
						t.Fatalf("action=%+v", action)
					}
					// GraphQL returns an empty list; REST/MCP omit the optional field.
					var paths []string
					if changes, ok := action["changedFields"].([]any); ok {
						for _, change := range changes {
							paths = append(paths, change.(map[string]any)["path"].(string))
						}
					} else if surface.name == "GraphQL" || action["changedFields"] != nil {
						t.Fatalf("unexpected changedFields representation: %+v", action)
					}
					if !slices.Equal(paths, tc.paths) {
						t.Fatalf("changed paths=%v want=%v", paths, tc.paths)
					}
				})
			}
		})
	}
}
