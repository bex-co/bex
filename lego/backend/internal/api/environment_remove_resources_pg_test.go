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
	"fmt"
	"net/http"
	"testing"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// w4/714: Render's remove-from-environment ungroups the services it removes,
// over the composed transports against real SQL. A Render resource belongs to
// a project only through one of its environments, so after
// DELETE /v1/environments/{id}/resources each service reads with no project
// and no environment on REST, GraphQL and MCP, and the project no longer lists
// it, which is what puts it under the dashboard Overview's Ungrouped
// Resources. The removal used to keep the project, so the Overview listed the
// services nowhere.
func TestRemoveResourcesUngroupsServicesPG(t *testing.T) {
	ctx, _, st := ownerGuardPG(t)
	owner, err := st.CreateWorkspace(ctx, "ungroup-"+ids.New(ids.Workspace), store.PlanHobby, "client-1")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.DeleteTenant(context.Background(), owner.ID) })
	project, err := st.CreateProject(ctx, owner.ID, "stack")
	if err != nil {
		t.Fatal(err)
	}
	staging, err := st.CreateEnvironment(ctx, project.ID, owner.ID, "staging")
	if err != nil {
		t.Fatal(err)
	}
	var objects []client.Object
	var services []string
	for _, name := range []string{"web", "worker"} {
		row, err := st.CreateApp(ctx, store.App{TenantID: owner.ID, Name: name, Image: "nginx:1", Type: "web_service", Branch: "main", Port: 80, Replicas: 1, Tier: "starter"})
		if err != nil {
			t.Fatal(err)
		}
		app, _, _ := metadataResources(owner.ID)
		app.Name = row.ID
		app.Labels[store.LabelManagedBy], app.Labels[store.LabelAppID] = store.ManagedByValue, row.ID
		app.Labels[core.LabelServiceName] = name
		objects = append(objects, app)
		services = append(services, row.ID)
	}
	base := serverBase(t, st)
	base.Client = fakeClient(objects...)
	base.Authz = &fakeMembershipChecker{memberOf: map[string]string{owner.ID: "user:client-1"}}
	handler, srv := serverWith(t, base, Deps{WorkspaceStore: st, Store: st, ProjectsStore: st, EnvironmentsStore: st, AppPlacements: st})
	session := mcpSessionAs(t, srv, "client-1")
	request := func(method, path, body string, status int) []byte {
		t.Helper()
		rec := do(t, handler, method, path, testToken, body)
		if rec.Code != status {
			t.Fatalf("%s %s = %d, want %d: %s", method, path, rec.Code, status, rec.Body.String())
		}
		return rec.Body.Bytes()
	}
	environmentID := ids.EnvironmentPublicID(staging.ID)
	assertPlacement := func(wantProject, wantEnvironment string) {
		t.Helper()
		for _, serviceID := range services {
			for source, object := range map[string]map[string]any{
				"REST":    decodeObject(t, request(http.MethodGet, "/v1/services/"+serviceID, "", http.StatusOK)),
				"GraphQL": gql(t, handler, fmt.Sprintf(`{service(id:%q){projectId environmentId}}`, serviceID))["service"].(map[string]any),
				"MCP":     callTool[map[string]any](t, session, "get_service", map[string]any{"serviceId": serviceID}),
			} {
				gotProject, _ := object["projectId"].(string)
				gotEnvironment, _ := object["environmentId"].(string)
				if gotProject != wantProject || gotEnvironment != wantEnvironment {
					t.Errorf("%s %s: project %q, environment %q; want %q and %q", source, serviceID, gotProject, gotEnvironment, wantProject, wantEnvironment)
				}
			}
		}
		wantMembers := 0
		if wantProject != "" {
			wantMembers = len(services)
		}
		for source, object := range map[string]map[string]any{
			"GraphQL": gql(t, handler, fmt.Sprintf(`{project(id:%q){serviceIds}}`, project.ID))["project"].(map[string]any),
			"MCP":     callTool[map[string]any](t, session, "get_project", map[string]any{"id": project.ID}),
		} {
			if members, _ := object["serviceIds"].([]any); len(members) != wantMembers {
				t.Errorf("%s project serviceIds = %v, want %d services", source, members, wantMembers)
			}
		}
	}

	request(http.MethodPost, "/v1/environments/"+environmentID+"/resources",
		fmt.Sprintf(`{"resourceIds":[%q,%q]}`, services[0], services[1]), http.StatusCreated)
	assertPlacement(project.ID, environmentID)

	request(http.MethodDelete, "/v1/environments/"+environmentID+"/resources?resourceIds="+services[0]+"&resourceIds="+services[1], "", http.StatusNoContent)
	assertPlacement("", "")
}
