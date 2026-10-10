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
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/envgroups"
	"github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// w4/m187: a group linked to a service in the same environment must not block
// that environment's (or its project's) deletion. The real env-group core and
// its durable metadata are composed here — the public move guard that used to
// refuse teardown with ENV_GROUP_MOVE_INCOMPATIBLE_SERVICES is live — and every
// deletion adapter must succeed with the group surviving workspace-scoped, its
// link and value intact. The standalone move still refuses first.
func TestLinkedEnvGroupDoesNotBlockGroupingDeletion(t *testing.T) {
	for _, surface := range []string{"REST", "GraphQL", "MCP"} {
		for _, grouping := range []string{"project", "environment"} {
			t.Run(surface+"/"+grouping, func(t *testing.T) {
				ws := newFakeWSStore()
				owner := mustCreate(t, ws, "teardown", store.PlanHobby, "client-1")
				projectID, envID := id.New(id.Project), id.New(id.Environment)
				groupings := &datastorePlacementStore{newConformProjectStore(
					store.Project{ID: projectID, TenantID: owner.ID, Name: "target", CreatedAt: conformEpoch, UpdatedAt: conformEpoch},
				)}
				groupings.envs[projectID] = []store.Environment{{
					ID: id.EnvironmentStorageID(envID), ProjectID: projectID, TenantID: owner.ID, Name: "target-env",
					ProtectedStatus: core.ProtectedStatusUnprotected,
				}}
				webID := id.New(id.Service)
				web := sampleApp("web")
				web.Labels = core.TenantLabels(owner.ID)
				web.Labels[core.LabelAppID] = webID
				web.Labels[core.LabelServiceName] = "web"
				web.Labels[core.LabelProject] = projectID
				web.Labels[core.LabelEnvironment] = id.EnvironmentStorageID(envID)
				base := serverBase(t, ws)
				base.Client = fakeClient(web)
				h, srv := serverWith(t, base, Deps{WorkspaceStore: ws, ProjectsStore: groupings, EnvironmentsStore: groupings, Secrets: newMemSecretStore()})
				request := func(method, path, body string, status int) []byte {
					t.Helper()
					rec := do(t, h, method, path, testToken, body)
					if rec.Code != status {
						t.Fatalf("%s %s = %d, want %d: %s", method, path, rec.Code, status, rec.Body.String())
					}
					return rec.Body.Bytes()
				}

				var group envgroups.EnvGroupView
				body := request(http.MethodPost, "/v1/env-groups", fmt.Sprintf(`{"name":"linked","environmentId":%q,"serviceIds":[%q],"envVars":[{"key":"QA_PLACEMENT","value":"saved"}]}`, envID, webID), http.StatusCreated)
				if err := json.Unmarshal(body, &group); err != nil {
					t.Fatal(err)
				}
				// The ordinary move guard stays: the linked service is still in
				// the environment, so moving the group to workspace scope refuses.
				request(http.MethodPatch, "/v1/env-groups/"+group.ID+"/environment", `{"environmentId":""}`, http.StatusConflict)

				target, path, gqlField, tool := projectID, "/v1/projects/", "deleteProject", "delete_project"
				if grouping == "environment" {
					target, path, gqlField, tool = envID, "/v1/environments/", "deleteEnvironment", "delete_environment"
				}
				switch surface {
				case "REST":
					request(http.MethodDelete, path+target, "", http.StatusNoContent)
				case "GraphQL":
					gql(t, h, fmt.Sprintf(`mutation { %s(id: %q) }`, gqlField, target))
				case "MCP":
					callTool[any](t, mcpSessionAs(t, srv, "client-1"), tool, map[string]any{"id": target})
				}

				request(http.MethodGet, "/v1/environments/"+envID, "", http.StatusNotFound)
				if grouping == "project" {
					request(http.MethodGet, "/v1/projects/"+projectID, "", http.StatusNotFound)
				} else {
					request(http.MethodGet, "/v1/projects/"+projectID, "", http.StatusOK)
				}
				data := gql(t, h, fmt.Sprintf(`{ envGroup(id: %q) { environmentId serviceLinks } envGroupVar(id: %q, key: "QA_PLACEMENT") { value } }`, group.ID, group.ID))
				got := data["envGroup"].(map[string]any)
				links, _ := got["serviceLinks"].([]any)
				if got["environmentId"] != nil || len(links) != 1 || links[0] != webID {
					t.Fatalf("group after deletion = %v, want workspace scope with link %s kept", got, webID)
				}
				if v := data["envGroupVar"].(map[string]any)["value"]; v != "saved" {
					t.Fatalf("group value after deletion = %v", v)
				}
			})
		}
	}
}
