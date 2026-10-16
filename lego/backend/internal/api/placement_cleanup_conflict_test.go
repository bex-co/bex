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

package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// TestProjectDeleteSurvivesDatastoreLockConflicts is w4/m214's a698 journey
// composed through every adapter: an unrelated write to the project's
// PostgreSQL between cleanup's read and patch (bumping its resourceVersion)
// used to fail DeleteProject with an internal error. One such write is now
// recovered inside the request and the project deletes with both datastores
// detached and the unrelated write kept. Writes that never stop answer the
// public PLACEMENT_CLEANUP_CONFLICT explaining that cleanup may be partial and
// to reload; the project survives, and once the contention ends the same
// delete finishes.
func TestProjectDeleteSurvivesDatastoreLockConflicts(t *testing.T) {
	for _, surface := range []string{"REST", "GraphQL", "MCP"} {
		for _, contention := range []string{"transient", "persistent"} {
			t.Run(surface+"/"+contention, func(t *testing.T) {
				ws := newFakeWSStore()
				owner := mustCreate(t, ws, "contention", store.PlanHobby, "client-1")
				projectID, envID := id.New(id.Project), id.New(id.Environment)
				groupings := &datastorePlacementStore{newConformProjectStore(
					store.Project{ID: projectID, TenantID: owner.ID, Name: "doomed", CreatedAt: conformEpoch, UpdatedAt: conformEpoch},
				)}
				groupings.envs[projectID] = []store.Environment{{
					ID: id.EnvironmentStorageID(envID), ProjectID: projectID, TenantID: owner.ID, Name: "prod",
					ProtectedStatus: core.ProtectedStatusUnprotected,
				}}
				_, db, kv := metadataResources(owner.ID)
				db.Name, kv.Name = id.New(id.Postgres), id.New(id.KeyValue)
				for _, labels := range []*map[string]string{&db.Labels, &kv.Labels} {
					*labels = core.TenantLabels(owner.ID)
					(*labels)[core.LabelProject] = projectID
					(*labels)[core.LabelEnvironment] = id.EnvironmentStorageID(envID)
				}

				remaining := 1
				if contention == "persistent" {
					remaining = 1 << 20
				}
				cl := fake.NewClientBuilder().WithScheme(testScheme()).WithObjects(db, kv).
					WithInterceptorFuncs(interceptor.Funcs{
						Patch: func(ctx context.Context, c client.WithWatch, obj client.Object, p client.Patch, opts ...client.PatchOption) error {
							if _, isDB := obj.(*appv1alpha1.Database); isDB && remaining > 0 {
								remaining--
								var current appv1alpha1.Database
								if err := c.Get(ctx, client.ObjectKeyFromObject(obj), &current); err != nil {
									return err
								}
								if current.Annotations == nil {
									current.Annotations = map[string]string{}
								}
								current.Annotations["unrelated"] = "kept"
								if err := c.Update(ctx, &current); err != nil {
									return err
								}
							}
							return c.Patch(ctx, obj, p, opts...)
						},
					}).Build()
				base := serverBase(t, ws)
				base.Client = cl
				h, srv := serverWith(t, base, Deps{WorkspaceStore: ws, ProjectsStore: groupings, EnvironmentsStore: groupings})

				// deleteProject returns "" on success, or the public error text with
				// its code when the adapter refuses.
				deleteProject := func() string {
					t.Helper()
					switch surface {
					case "REST":
						rec := do(t, h, http.MethodDelete, "/v1/projects/"+projectID, testToken, "")
						if rec.Code == http.StatusNoContent {
							return ""
						}
						if rec.Code != http.StatusConflict {
							t.Fatalf("REST delete = %d %s, want 204 or 409", rec.Code, rec.Body.String())
						}
						return rec.Body.String()
					case "GraphQL":
						body, _ := json.Marshal(map[string]string{"query": fmt.Sprintf(`mutation { deleteProject(id: %q) }`, projectID)})
						rec := do(t, h, http.MethodPost, "/graphql", testToken, string(body))
						var out struct {
							Errors []struct {
								Message    string         `json:"message"`
								Extensions map[string]any `json:"extensions"`
							} `json:"errors"`
						}
						if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
							t.Fatal(err)
						}
						if len(out.Errors) == 0 {
							return ""
						}
						return fmt.Sprintf("%v %s", out.Errors[0].Extensions["code"], out.Errors[0].Message)
					default:
						res, err := mcpSessionAs(t, srv, "client-1").CallTool(context.Background(),
							&mcp.CallToolParams{Name: "delete_project", Arguments: map[string]any{"id": projectID}})
						if err != nil {
							t.Fatal(err)
						}
						if !res.IsError {
							return ""
						}
						raw, _ := json.Marshal(res.Content)
						return string(raw)
					}
				}
				detached := func() {
					t.Helper()
					for _, obj := range []client.Object{&appv1alpha1.Database{}, &appv1alpha1.KeyValue{}} {
						name := db.Name
						if _, isKV := obj.(*appv1alpha1.KeyValue); isKV {
							name = kv.Name
						}
						if err := cl.Get(context.Background(), client.ObjectKey{Namespace: db.Namespace, Name: name}, obj); err != nil {
							t.Fatal(err)
						}
						if labels := obj.GetLabels(); labels[core.LabelProject] != "" || labels[core.LabelEnvironment] != "" {
							t.Fatalf("%s placement after delete = %v, want detached", name, labels)
						}
					}
					if rec := do(t, h, http.MethodGet, "/v1/projects/"+projectID, testToken, ""); rec.Code != http.StatusNotFound {
						t.Fatalf("project after delete = %d, want 404", rec.Code)
					}
				}

				refusal := deleteProject()
				if contention == "transient" {
					if refusal != "" {
						t.Fatalf("delete after one unrelated write = %q, want success", refusal)
					}
					detached()
					var current appv1alpha1.Database
					if err := cl.Get(context.Background(), client.ObjectKeyFromObject(db), &current); err != nil || current.Annotations["unrelated"] != "kept" {
						t.Fatalf("unrelated write lost: %v %v", current.Annotations, err)
					}
					return
				}
				if !strings.Contains(refusal, core.CodePlacementCleanupConflict) || !strings.Contains(refusal, "reload the current placement") ||
					strings.Contains(refusal, "internal error") {
					t.Fatalf("persistent contention answered %q, want the public placement-cleanup conflict", refusal)
				}
				if rec := do(t, h, http.MethodGet, "/v1/projects/"+projectID, testToken, ""); rec.Code != http.StatusOK {
					t.Fatalf("project after refused delete = %d, want it kept for the retry", rec.Code)
				}
				remaining = 0
				if retried := deleteProject(); retried != "" {
					t.Fatalf("delete once contention ended = %q, want success", retried)
				}
				detached()
			})
		}
	}
}
