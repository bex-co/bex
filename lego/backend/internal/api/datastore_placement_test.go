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
	"errors"
	"fmt"
	"net/http"
	"slices"
	"testing"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// Datastore membership lives in real CRs. This fixture only holds grouping rows
// and models the project's environment cascade; it never edits those CRs.
type datastorePlacementStore struct{ *conformProjectStore }

func (s *datastorePlacementStore) GetEnvironment(_ context.Context, environmentID string) (store.Environment, error) {
	for _, envs := range s.envs {
		for _, env := range envs {
			if env.ID == environmentID {
				return env, nil
			}
		}
	}
	return store.Environment{}, core.ErrNotFound
}

func (s *datastorePlacementStore) ListWorkspaceEnvironments(_ context.Context, tenantID string) ([]store.Environment, error) {
	var out []store.Environment
	for _, envs := range s.envs {
		for _, env := range envs {
			if env.TenantID == tenantID {
				out = append(out, env)
			}
		}
	}
	return out, nil
}

func (s *datastorePlacementStore) DeleteEnvironment(_ context.Context, environmentID string) error {
	for projectID, envs := range s.envs {
		s.envs[projectID] = slices.DeleteFunc(envs, func(env store.Environment) bool { return env.ID == environmentID })
	}
	return nil
}

func (s *datastorePlacementStore) DeleteProject(ctx context.Context, projectID string) error {
	delete(s.envs, projectID)
	return s.conformProjectStore.DeleteProject(ctx, projectID)
}

func (*datastorePlacementStore) CreateEnvironment(context.Context, string, string, string) (store.Environment, error) {
	return store.Environment{}, errors.New("unexpected CreateEnvironment")
}

func (*datastorePlacementStore) RenameEnvironment(context.Context, string, string) error {
	return errors.New("unexpected RenameEnvironment")
}

func (*datastorePlacementStore) SetEnvironmentACL(context.Context, string, string, bool, []core.IPAllowListEntry) error {
	return errors.New("unexpected SetEnvironmentACL")
}

func (*datastorePlacementStore) SetEnvironmentServices(context.Context, string, string, string, []string) ([]core.ServicePlacementChange, error) {
	return nil, errors.New("unexpected SetEnvironmentServices")
}

func (*datastorePlacementStore) ListEnvironmentServices(context.Context, string, string) ([]string, error) {
	return nil, nil
}

func (*datastorePlacementStore) ListWorkspaceEnvironmentServices(context.Context, string) (map[string][]string, error) {
	return nil, nil
}

func TestDatastorePlacementAcrossComposedSurfaces(t *testing.T) {
	for _, surface := range []string{"REST", "GraphQL", "MCP"} {
		for _, operation := range []string{
			"delete-environment", "delete-project", "delete-project-without-environment",
			"move-project", "same-project", "move-environment", "move-environment-other-project",
			"unassign-environment", "unassign-project",
		} {
			t.Run(surface+"/"+operation, func(t *testing.T) {
				ws := newFakeWSStore()
				owner := mustCreate(t, ws, "placement", store.PlanHobby, "client-1")
				projectA, projectB := id.New(id.Project), id.New(id.Project)
				envA, envB, envC := id.New(id.Environment), id.New(id.Environment), id.New(id.Environment)
				groupings := &datastorePlacementStore{newConformProjectStore(
					store.Project{ID: projectA, TenantID: owner.ID, Name: "first", CreatedAt: conformEpoch, UpdatedAt: conformEpoch},
					store.Project{ID: projectB, TenantID: owner.ID, Name: "second", CreatedAt: conformEpoch, UpdatedAt: conformEpoch},
				)}
				for i, envID := range []string{envA, envB, envC} {
					projectID := projectA
					if envID == envC {
						projectID = projectB
					}
					groupings.envs[projectID] = append(groupings.envs[projectID], store.Environment{
						ID: envID, ProjectID: projectID, TenantID: owner.ID, Name: fmt.Sprintf("env-%d", i),
						ProtectedStatus: core.ProtectedStatusUnprotected,
						IPAllowList:     []core.IPAllowListEntry{{CIDRBlock: fmt.Sprintf("192.0.2.%d/32", i+1)}},
					})
				}
				_, db, kv := metadataResources(owner.ID)
				db.Name, kv.Name = id.New(id.Postgres), id.New(id.KeyValue)
				// metadataResources shares its label map; each datastore must have
				// independent API-object state before the fake client copies it.
				db.Labels = core.TenantLabels(owner.ID)
				kv.Labels = core.TenantLabels(owner.ID)
				initialEnv := envA
				if operation == "delete-project-without-environment" {
					initialEnv = ""
				}
				for _, resource := range []client.Object{db, kv} {
					resource.GetLabels()[core.LabelProject] = projectA
					if initialEnv != "" {
						resource.GetLabels()[core.LabelEnvironment] = initialEnv
					}
				}
				ownRules := []appv1alpha1.IPAllowEntry{{CIDR: "198.51.100.9/32", Description: "datastore-owned"}}
				db.Spec.IPAllowList, kv.Spec.IPAllowList = slices.Clone(ownRules), slices.Clone(ownRules)
				if initialEnv != "" {
					db.Spec.EnvironmentIPAllowList = []string{"192.0.2.1/32"}
					kv.Spec.EnvironmentIPAllowList = []string{"192.0.2.1/32"}
				}
				base := serverBase(t, ws)
				base.Client = fakeClient(db, kv)
				h, srv := serverWith(t, base, Deps{WorkspaceStore: ws, ProjectsStore: groupings, EnvironmentsStore: groupings})
				cs := mcpSessionAs(t, srv, "client-1")
				request := func(method, path, body string, status int) []byte {
					t.Helper()
					rec := do(t, h, method, path, testToken, body)
					if rec.Code != status {
						t.Fatalf("%s %s = %d, want %d: %s", method, path, rec.Code, status, rec.Body.String())
					}
					return rec.Body.Bytes()
				}
				assertPlacement := func(wantProject, wantEnvironment string) {
					t.Helper()
					assertResource := func(source string, object map[string]any, resourceID string) {
						t.Helper()
						projectID, _ := object["projectId"].(string)
						environmentID, _ := object["environmentId"].(string)
						if object["id"] != resourceID || projectID != wantProject || environmentID != wantEnvironment {
							t.Errorf("%s placement = id:%v project:%q environment:%q, want %s %q %q", source, object["id"], projectID, environmentID, resourceID, wantProject, wantEnvironment)
						}
					}
					for _, resource := range []struct{ path, envelope, gql, gqlList, tool, toolList, listKey, arg, resourceID string }{
						{"postgres", "postgres", "database", "databases", "get_postgres", "list_postgres_instances", "postgres", "postgresId", db.Name},
						{"key-value", "keyValue", "keyValue", "keyValues", "get_key_value", "list_key_value", "keyValues", "keyValueId", kv.Name},
					} {
						body := request(http.MethodGet, "/v1/"+resource.path+"/"+resource.resourceID, "", http.StatusOK)
						assertResource("REST get", decodeObject(t, body), resource.resourceID)
						var items []map[string]any
						body = request(http.MethodGet, "/v1/"+resource.path+"?ownerId="+owner.ID, "", http.StatusOK)
						if err := json.Unmarshal(body, &items); err != nil || len(items) != 1 {
							t.Fatalf("REST list = %s, err %v", body, err)
						}
						assertResource("REST list", items[0][resource.envelope].(map[string]any), resource.resourceID)
						data := gql(t, h, fmt.Sprintf(`{ one:%s(id:%q) { id projectId environmentId } all:%s(ownerId:%q) { id projectId environmentId } }`, resource.gql, resource.resourceID, resource.gqlList, owner.ID))
						assertResource("GraphQL get", data["one"].(map[string]any), resource.resourceID)
						gqlList, _ := data["all"].([]any)
						if len(gqlList) != 1 {
							t.Fatalf("GraphQL list = %v, want one datastore", data["all"])
						}
						assertResource("GraphQL list", gqlList[0].(map[string]any), resource.resourceID)
						object := callTool[map[string]any](t, cs, resource.tool, map[string]any{resource.arg: resource.resourceID})
						assertResource("MCP get", object, resource.resourceID)
						listed := callTool[map[string][]map[string]any](t, cs, resource.toolList, map[string]any{"workspaceId": owner.ID})
						if len(listed[resource.listKey]) != 1 {
							t.Fatalf("MCP list = %v, want one datastore", listed)
						}
						assertResource("MCP list", listed[resource.listKey][0], resource.resourceID)
					}
					assertMembers := func(source string, object map[string]any, member bool) {
						t.Helper()
						for field, resourceID := range map[string]string{"databaseIds": db.Name, "keyValueIds": kv.Name} {
							members, _ := object[field].([]any)
							if member && (len(members) != 1 || members[0] != resourceID) || !member && len(members) != 0 {
								t.Errorf("%s %s = %v, want member %v (%s)", source, field, members, member, resourceID)
							}
						}
					}
					for projectID := range groupings.projects {
						data := gql(t, h, fmt.Sprintf(`{ project(id:%q) { databaseIds keyValueIds } }`, projectID))
						assertMembers("GraphQL project", data["project"].(map[string]any), projectID == wantProject)
						object := callTool[map[string]any](t, cs, "get_project", map[string]any{"id": projectID})
						assertMembers("MCP project", object, projectID == wantProject)
					}
					for _, envs := range groupings.envs {
						for _, env := range envs {
							object := decodeObject(t, request(http.MethodGet, "/v1/environments/"+env.ID, "", http.StatusOK))
							assertMembers("REST environment", object, env.ID == wantEnvironment)
							data := gql(t, h, fmt.Sprintf(`{ environment(id:%q) { databaseIds keyValueIds } }`, env.ID))
							assertMembers("GraphQL environment", data["environment"].(map[string]any), env.ID == wantEnvironment)
							object = callTool[map[string]any](t, cs, "get_environment", map[string]any{"id": env.ID})
							assertMembers("MCP environment", object, env.ID == wantEnvironment)
						}
					}
					var gotDB appv1alpha1.Database
					var gotKV appv1alpha1.KeyValue
					if err := base.Client.Get(t.Context(), client.ObjectKeyFromObject(db), &gotDB); err != nil {
						t.Fatal(err)
					}
					if err := base.Client.Get(t.Context(), client.ObjectKeyFromObject(kv), &gotKV); err != nil {
						t.Fatal(err)
					}
					var inherited []string
					if wantEnvironment != "" {
						env, err := groupings.GetEnvironment(t.Context(), wantEnvironment)
						if err != nil {
							t.Fatal(err)
						}
						inherited = core.AllowListCIDRs(env.IPAllowList)
					}
					if !slices.Equal(gotDB.Spec.IPAllowList, ownRules) || !slices.Equal(gotKV.Spec.IPAllowList, ownRules) {
						t.Error("placement changed datastore-owned IP rules")
					}
					if !slices.Equal(gotDB.Spec.EnvironmentIPAllowList, inherited) || !slices.Equal(gotKV.Spec.EnvironmentIPAllowList, inherited) {
						t.Errorf("inherited IP rules postgres=%v keyValue=%v, want %v", gotDB.Spec.EnvironmentIPAllowList, gotKV.Spec.EnvironmentIPAllowList, inherited)
					}
				}
				assertPlacement(projectA, initialEnv)

				kind, target, wantProject, wantEnv, remove, empty := "environment", envA, projectA, "", false, false
				switch operation {
				case "delete-environment":
					remove = true
				case "delete-project", "delete-project-without-environment":
					kind, target, wantProject, remove = "project", projectA, "", true
				case "move-project":
					kind, target, wantProject = "project", projectB, projectB
				case "same-project":
					kind, target, wantEnv = "project", projectA, envA
				case "move-environment":
					target, wantEnv = envB, envB
				case "move-environment-other-project":
					target, wantProject, wantEnv = envC, projectB, envC
				case "unassign-environment":
					empty = true
				case "unassign-project":
					kind, target, wantProject, empty = "project", projectA, "", true
				}
				if remove {
					switch surface {
					case "REST":
						request(http.MethodDelete, "/v1/"+kind+"s/"+target, "", http.StatusNoContent)
					case "GraphQL":
						field := "deleteProject"
						if kind == "environment" {
							field = "deleteEnvironment"
						}
						gql(t, h, fmt.Sprintf(`mutation { %s(id:%q) }`, field, target))
					case "MCP":
						callTool[map[string]any](t, cs, "delete_"+kind, map[string]any{"id": target})
					}
					request(http.MethodGet, "/v1/"+kind+"s/"+target, "", http.StatusNotFound)
				} else {
					pgIDs, kvIDs := []string{db.Name}, []string{kv.Name}
					if empty {
						pgIDs, kvIDs = []string{}, []string{}
					}
					switch surface {
					case "REST":
						for _, membership := range []struct {
							path, field string
							ids         []string
						}{
							{"database-links", "databaseIds", pgIDs}, {"keyvalue-links", "keyValueIds", kvIDs},
						} {
							body, err := json.Marshal(map[string]any{membership.field: membership.ids})
							if err != nil {
								t.Fatal(err)
							}
							request(http.MethodPut, "/v1/"+kind+"s/"+target+"/"+membership.path, string(body), http.StatusOK)
						}
					case "GraphQL":
						prefix := "setProject"
						if kind == "environment" {
							prefix = "setEnvironment"
						}
						pgJSON, _ := json.Marshal(pgIDs)
						kvJSON, _ := json.Marshal(kvIDs)
						gql(t, h, fmt.Sprintf(`mutation { pg: %sDatabases(id:%q, databaseIds:%s) { id } kv: %sKeyValues(id:%q, keyValueIds:%s) { id } }`, prefix, target, pgJSON, prefix, target, kvJSON))
					case "MCP":
						callTool[map[string]any](t, cs, "update_"+kind, map[string]any{"id": target, "databaseIds": pgIDs, "keyValueIds": kvIDs})
					}
				}
				assertPlacement(wantProject, wantEnv)
			})
		}
	}
}
