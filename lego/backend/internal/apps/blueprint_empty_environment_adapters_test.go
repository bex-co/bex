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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/graphql-go/graphql"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

const emptyEnvironmentAdapterManifest = `services:
  - name: empty-site
    type: web
    runtime: static
    repo: https://github.com/bex-co/bex
    branch: main
    staticPublishPath: .
    envVars: []
`

func assertEmptyEnvironmentAction(t *testing.T, result map[string]any, operation BlueprintPlanOperation, resourceID string, paths []string) {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	var validation BlueprintValidation
	if err := json.Unmarshal(encoded, &validation); err != nil {
		t.Fatal(err)
	}
	if !validation.Valid || validation.Plan == nil || validation.Plan.TotalActions != 1 || len(validation.Plan.Actions) != 1 {
		t.Fatalf("invalid single-service plan: %s", encoded)
	}
	action := validation.Plan.Actions[0]
	if action.Operation != operation || action.ResourceID != resourceID || !slices.Equal(fieldPaths(action.ChangedFields), paths) {
		t.Fatalf("action = %+v, want %s resource=%q paths=%v", action, operation, resourceID, paths)
	}
}

func TestBlueprintEmptyEnvironmentDirectApplyAdapters(t *testing.T) {
	for _, surface := range []string{"REST", "MCP"} {
		t.Run(surface, func(t *testing.T) {
			recorder := &recordingStore{}
			svc, _ := connectionService(t)
			svc.Store = recorder
			svc.Client = blueprintSerializedClient{Client: svc.Client}
			var mux *http.ServeMux
			var call func(string, map[string]any) (map[string]any, bool)
			if surface == "REST" {
				mux = http.NewServeMux()
				svc.RegisterREST(mux)
			} else {
				call = detachSurfaceMCP(t, svc)
			}
			apply := func(manifest string) string {
				t.Helper()
				var result map[string]any
				if surface == "MCP" {
					var refused bool
					result, refused = call("deploy", map[string]any{"bexYaml": manifest})
					if refused {
						t.Fatalf("apply: %v", result)
					}
				} else {
					rec := httptest.NewRecorder()
					mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/blueprints/deploy", strings.NewReader(fmt.Sprintf(`{"bexYaml":%q,"ownerId":%q}`, manifest, connOwner))).WithContext(ownershipCtx()))
					if rec.Code != http.StatusOK {
						t.Fatalf("apply: %d %s", rec.Code, rec.Body.String())
					}
					if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
						t.Fatal(err)
					}
				}
				return result["services"].([]any)[0].(map[string]any)["id"].(string)
			}
			firstID := apply(emptyEnvironmentAdapterManifest)
			before := getTenantApp(t, svc.Client, connOwner, "empty-site")
			for range 2 {
				if got := apply(emptyEnvironmentAdapterManifest); got != firstID {
					t.Fatalf("repeat apply changed id: %q -> %q", firstID, got)
				}
				after := getTenantApp(t, svc.Client, connOwner, "empty-site")
				assertBlueprintAppUnchanged(t, before, after)
				if len(recorder.deployCalls) != 0 {
					t.Fatalf("repeat apply created %d deploys", len(recorder.deployCalls))
				}
			}
			changed := strings.Replace(emptyEnvironmentAdapterManifest, "envVars: []", "envVars: [{key: MESSAGE, value: changed}]", 1)
			if got := apply(changed); got != firstID {
				t.Fatalf("real change replaced service: %q -> %q", firstID, got)
			}
			after := getTenantApp(t, svc.Client, connOwner, "empty-site")
			if len(recorder.deployCalls) != 1 || len(after.Spec.Env) != 1 || after.Spec.Env[0].Value != "changed" || after.Spec.RestartedAt == before.Spec.RestartedAt {
				t.Fatalf("real environment change did not deploy: calls=%v spec=%+v", recorder.deployCalls, after.Spec)
			}
		})
	}
}

func TestBlueprintEmptyEnvironmentGitAdapters(t *testing.T) {
	for _, surface := range []string{"REST", "GraphQL", "MCP"} {
		t.Run(surface, func(t *testing.T) {
			svc, fs := connectionService(t)
			svc.Client = blueprintSerializedClient{Client: svc.Client}
			recorder := &recordingStore{}
			svc.Store = recorder
			svc.GitFetcher = fakeBlueprintFetcher{contents: emptyEnvironmentAdapterManifest, sha: testCommitSHA}
			var mux *http.ServeMux
			var schema graphql.Schema
			var mcpCall func(string, map[string]any) (map[string]any, bool)
			switch surface {
			case "REST":
				mux = http.NewServeMux()
				svc.RegisterREST(mux)
			case "GraphQL":
				schema = blueprintSchema(t, svc)
			case "MCP":
				mcpCall = detachSurfaceMCP(t, svc)
			}
			call := func(verb, blueprintID string) map[string]any {
				t.Helper()
				args := map[string]any{"repo": repoBex, "branch": "main", "path": CanonicalBlueprintFilename}
				var result map[string]any
				switch surface {
				case "REST":
					args["ownerId"] = connOwner
					path := "/v1/blueprints"
					if verb == "preview" {
						path += "/preview"
						args["blueprintId"] = blueprintID
					} else if verb == "sync" {
						path += "/" + blueprintID + "/sync"
						args = map[string]any{"ownerId": connOwner}
					}
					body, err := json.Marshal(args)
					if err != nil {
						t.Fatal(err)
					}
					rec := httptest.NewRecorder()
					mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body)).WithContext(ownershipCtx()))
					if rec.Code != http.StatusOK && rec.Code != http.StatusCreated {
						t.Fatalf("%s: %d %s", verb, rec.Code, rec.Body.String())
					}
					if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
						t.Fatal(err)
					}
				case "GraphQL":
					var query, field string
					switch verb {
					case "create":
						field = "createBlueprint"
						query = fmt.Sprintf(`mutation { createBlueprint(repo:%q,branch:"main",path:%q,ownerId:%q) { id } }`, repoBex, CanonicalBlueprintFilename, connOwner)
					case "preview":
						field = "blueprintPreview"
						query = fmt.Sprintf(`{ blueprintPreview(repo:%q,branch:"main",path:%q,ownerId:%q,blueprintId:%q) { validation { valid plan { totalActions actions { operation resourceId changedFields { path } } } } } }`, repoBex, CanonicalBlueprintFilename, connOwner, blueprintID)
					case "sync":
						field = "syncBlueprint"
						query = fmt.Sprintf(`mutation { syncBlueprint(id:%q,ownerId:%q) { blueprint { id } services { id } } }`, blueprintID, connOwner)
					}
					response := graphql.Do(graphql.Params{Schema: schema, Context: ownershipCtx(), RequestString: query})
					if len(response.Errors) != 0 {
						t.Fatal(response.Errors)
					}
					result = response.Data.(map[string]any)[field].(map[string]any)
				case "MCP":
					if verb == "preview" {
						args["blueprintId"] = blueprintID
					} else if verb == "sync" {
						args = map[string]any{"id": blueprintID}
					}
					var refused bool
					result, refused = mcpCall(verb+"_blueprint", args)
					if refused {
						t.Fatalf("%s: %v", verb, result)
					}
				}
				return result
			}
			preview := call("preview", "")
			assertEmptyEnvironmentAction(t, preview["validation"].(map[string]any), BlueprintPlanCreate, "", nil)
			blueprintID := call("create", "")["id"].(string)
			before := getTenantApp(t, svc.Client, connOwner, "empty-site")
			preview = call("preview", blueprintID)
			assertEmptyEnvironmentAction(t, preview["validation"].(map[string]any), BlueprintPlanNoop, before.Labels[store.LabelAppID], nil)
			for range 2 {
				call("sync", blueprintID)
			}
			if surface == "REST" {
				if _, err := fs.EnqueueBlueprintAutoSyncIntent(context.Background(), store.BlueprintAutoSyncIntent{
					TenantID: connOwner, BlueprintID: blueprintID, DeliveryDigest: "empty-env-auto-sync",
					CommitSHA: testCommitSHA, Path: CanonicalBlueprintFilename,
				}); err != nil {
					t.Fatal(err)
				}
				worker := &BlueprintAutoSyncWorker{Svc: svc}
				if err := worker.tick(context.Background()); err != nil {
					t.Fatal(err)
				}
				for _, intent := range fs.autoSyncIntents {
					if intent.State != store.BlueprintAutoSyncIntentCompleted {
						t.Fatalf("auto-sync did not complete: %+v", intent)
					}
				}
			}
			after := getTenantApp(t, svc.Client, connOwner, "empty-site")
			assertBlueprintAppUnchanged(t, before, after)
			if len(recorder.deployCalls) != 0 {
				t.Fatalf("Git sync created %d deploys", len(recorder.deployCalls))
			}
			if len(recorder.appCreates) != 1 {
				t.Fatalf("Git sync created another service: %d creates", len(recorder.appCreates))
			}
			wantRuns := 3 // creation and two explicit syncs still record successful history.
			if surface == "REST" {
				wantRuns++ // The auto-sync worker must have applied, rather than skipped, its intent.
			}
			if len(fs.insertedSyncs) != wantRuns {
				t.Fatalf("sync history = %d runs, want %d", len(fs.insertedSyncs), wantRuns)
			}
		})
	}
}
