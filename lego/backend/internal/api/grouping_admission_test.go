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
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

type groupingAdmissionFixture struct {
	handler      http.Handler
	server       *Server
	client       client.Client
	groupings    *datastorePlacementStore
	project, env string
	objects      []client.Object
	members      map[string]string
	candidates   map[string]string
	foreign      map[string]string
	writes       twinWrites
}

func newGroupingAdmissionFixture(t *testing.T, denyLaterChild bool) *groupingAdmissionFixture {
	t.Helper()
	workspaces := newFakeWSStore()
	owner := mustCreate(t, workspaces, "grouping-admission", store.PlanHobby, "client-1")
	foreign := mustCreate(t, workspaces, "foreign", store.PlanHobby, "client-2")
	f := &groupingAdmissionFixture{
		project: id.New(id.Project), env: id.New(id.Environment),
		members: map[string]string{}, candidates: map[string]string{}, foreign: map[string]string{},
	}
	otherProject, otherEnv, laterEnv := id.New(id.Project), id.New(id.Environment), id.New(id.Environment)
	f.groupings = &datastorePlacementStore{newConformProjectStore(
		store.Project{ID: f.project, TenantID: owner.ID, Name: "target", CreatedAt: conformEpoch, UpdatedAt: conformEpoch},
		store.Project{ID: otherProject, TenantID: owner.ID, Name: "source", CreatedAt: conformEpoch, UpdatedAt: conformEpoch},
	)}
	f.groupings.envs[f.project] = []store.Environment{
		{ID: f.env, ProjectID: f.project, TenantID: owner.ID, Name: "first", ProtectedStatus: core.ProtectedStatusUnprotected},
		{ID: laterEnv, ProjectID: f.project, TenantID: owner.ID, Name: "later", ProtectedStatus: core.ProtectedStatusProtected},
	}
	f.groupings.envs[otherProject] = []store.Environment{{ID: otherEnv, ProjectID: otherProject, TenantID: owner.ID, Name: "source"}}
	for _, member := range []struct {
		tenant, project, environment string
		ids                          map[string]string
	}{
		{owner.ID, f.project, f.env, f.members},
		{owner.ID, otherProject, otherEnv, f.candidates},
		{foreign.ID, otherProject, otherEnv, f.foreign},
	} {
		_, pg, kv := metadataResources(member.tenant)
		pg.Name, kv.Name = id.New(id.Postgres), id.New(id.KeyValue)
		pg.Spec.IPAllowList = []appv1alpha1.IPAllowEntry{{CIDR: "192.0.2.1/32", Description: "owned"}}
		kv.Spec.IPAllowList = slices.Clone(pg.Spec.IPAllowList)
		pg.Spec.EnvironmentIPAllowList, kv.Spec.EnvironmentIPAllowList = []string{"198.51.100.1/32"}, []string{"198.51.100.1/32"}
		for _, object := range []client.Object{pg, kv} {
			labels := core.TenantLabels(member.tenant)
			labels[core.LabelProject], labels[core.LabelEnvironment] = member.project, member.environment
			object.SetLabels(labels)
			f.objects = append(f.objects, object)
		}
		member.ids["postgres"], member.ids["keyvalue"] = pg.Name, kv.Name
	}
	base := serverBase(t, workspaces)
	base.Client = recordingClient(&f.writes, f.objects...)
	if denyLaterChild {
		// The first child is unprotected, so its writes used to happen before
		// the later protected child's can_manage refusal was discovered.
		base.Authz = &recordingChecker{decide: func(relation string) bool { return relation != core.RelCanManage }}
	}
	f.handler, f.server = serverWith(t, base, Deps{WorkspaceStore: workspaces, ProjectsStore: f.groupings, EnvironmentsStore: f.groupings})
	f.client = base.Client
	return f
}

func (f *groupingAdmissionFixture) snapshot(t *testing.T) (map[string]client.Object, []byte) {
	t.Helper()
	objects := make(map[string]client.Object, len(f.objects))
	for _, seeded := range f.objects {
		current := seeded.DeepCopyObject().(client.Object)
		if err := f.client.Get(t.Context(), client.ObjectKeyFromObject(seeded), current); err != nil {
			t.Fatal(err)
		}
		objects[current.GetName()] = current
	}
	groupings, err := json.Marshal([]any{f.groupings.projects, f.groupings.envs})
	if err != nil {
		t.Fatal(err)
	}
	return objects, groupings
}

type groupingAdmissionMutation struct {
	surface, grouping, resource string
	ids                         []string
	delete                      bool
}

func (f *groupingAdmissionFixture) mutate(t *testing.T, mutation groupingAdmissionMutation, wantError string) {
	t.Helper()
	target, gqlKind := f.project, "Project"
	if mutation.grouping == "environment" {
		target, gqlKind = f.env, "Environment"
	}
	field, suffix, link := "databaseIds", "Databases", "database-links"
	if mutation.resource == "keyvalue" {
		field, suffix, link = "keyValueIds", "KeyValues", "keyvalue-links"
	}
	var message string
	switch mutation.surface {
	case "REST":
		body, err := json.Marshal(map[string]any{field: mutation.ids})
		if err != nil {
			t.Fatal(err)
		}
		method, path, status := http.MethodPut, "/v1/"+mutation.grouping+"s/"+target+"/"+link, http.StatusOK
		if mutation.delete {
			method, path, body, status = http.MethodDelete, "/v1/projects/"+target, nil, http.StatusNoContent
		}
		if wantError != "" {
			status = http.StatusForbidden
		}
		rec := do(t, f.handler, method, path, testToken, string(body))
		if rec.Code != status {
			t.Errorf("REST status = %d, want %d: %s", rec.Code, status, rec.Body.String())
		}
		if wantError != "" {
			message, _ = decodeObject(t, rec.Body.Bytes())["message"].(string)
		}
	case "GraphQL":
		ids, err := json.Marshal(mutation.ids)
		if err != nil {
			t.Fatal(err)
		}
		query := fmt.Sprintf(`mutation { set%s%s(id:%q, %s:%s) { id } }`, gqlKind, suffix, target, field, ids)
		if mutation.delete {
			query = fmt.Sprintf(`mutation { deleteProject(id:%q) }`, target)
		}
		body, err := json.Marshal(map[string]string{"query": query})
		if err != nil {
			t.Fatal(err)
		}
		rec := do(t, f.handler, http.MethodPost, "/graphql", testToken, string(body))
		var result struct {
			Errors []struct{ Message string } `json:"errors"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil || rec.Code != http.StatusOK {
			t.Fatalf("GraphQL response = %d %s, decode %v", rec.Code, rec.Body.String(), err)
		}
		if (len(result.Errors) != 0) != (wantError != "") {
			t.Errorf("GraphQL errors = %+v, want refusal %v", result.Errors, wantError != "")
		}
		for _, issue := range result.Errors {
			message += issue.Message
			if wantError != "" && issue.Message != wantError {
				t.Errorf("GraphQL error message = %q, want %q", issue.Message, wantError)
			}
		}
	case "MCP":
		name, args := "update_"+mutation.grouping, map[string]any{"id": target, field: mutation.ids}
		if mutation.delete {
			name, args = "delete_project", map[string]any{"id": target}
		}
		session := mcpSessionAs(t, f.server, "client-1")
		result, err := session.CallTool(t.Context(), &mcp.CallToolParams{Name: name, Arguments: args})
		if err != nil {
			t.Fatalf("MCP transport error: %v", err)
		}
		if result.IsError != (wantError != "") {
			t.Errorf("MCP isError = %v, want refusal %v: %v", result.IsError, wantError != "", result.Content)
		}
		for _, content := range result.Content {
			if text, ok := content.(*mcp.TextContent); ok {
				message += text.Text
			}
		}
	}
	if wantError != "" && !strings.Contains(message, wantError) {
		t.Errorf("%s error = %q, want %q", mutation.surface, message, wantError)
	}
}

func TestGroupingMembershipAdmissionAcrossSurfaces(t *testing.T) {
	for _, surface := range []string{"REST", "GraphQL", "MCP"} {
		for _, grouping := range []string{"environment", "project"} {
			for _, resource := range []string{"postgres", "keyvalue"} {
				for _, scenario := range []string{"malformed", "missing", "foreign", "mixed-foreign", "mixed-missing", "empty-clear"} {
					t.Run(strings.Join([]string{surface, grouping, resource, scenario}, "/"), func(t *testing.T) {
						f := newGroupingAdmissionFixture(t, false)
						before, groupingBefore := f.snapshot(t)
						missing := id.New(id.Postgres)
						if resource == "keyvalue" {
							missing = id.New(id.KeyValue)
						}
						ids, wantError := []string{missing}, "does not belong to workspace"
						switch scenario {
						case "malformed":
							ids = []string{"not-a-resource-id"}
						case "foreign":
							ids = []string{f.foreign[resource]}
						case "mixed-foreign":
							ids = []string{f.candidates[resource], f.foreign[resource]}
						case "mixed-missing":
							ids = []string{f.candidates[resource], missing}
						case "empty-clear":
							ids, wantError = []string{}, ""
						}
						if wantError != "" {
							wantError = fmt.Sprintf("%v: %q does not belong to workspace %q", core.ErrForbidden, ids[len(ids)-1], f.groupings.projects[f.project].TenantID)
						}
						f.mutate(t, groupingAdmissionMutation{surface: surface, grouping: grouping, resource: resource, ids: ids}, wantError)
						after, groupingAfter := f.snapshot(t)
						if wantError != "" {
							if f.writes.patches != 0 || f.writes.updates != 0 || !reflect.DeepEqual(before, after) || !bytes.Equal(groupingBefore, groupingAfter) {
								t.Errorf("refused membership changed state: patches=%d updates=%d objectsEqual=%v groupingsEqual=%v", f.writes.patches, f.writes.updates, reflect.DeepEqual(before, after), bytes.Equal(groupingBefore, groupingAfter))
							}
							return
						}
						member := after[f.members[resource]]
						wantProject := f.project
						if grouping == "project" {
							wantProject = ""
						}
						var inherited []string
						switch current := member.(type) {
						case *appv1alpha1.Database:
							inherited = current.Spec.EnvironmentIPAllowList
							if !slices.Equal(current.Spec.IPAllowList, before[current.Name].(*appv1alpha1.Database).Spec.IPAllowList) {
								t.Error("empty clear changed owned Postgres rules")
							}
						case *appv1alpha1.KeyValue:
							inherited = current.Spec.EnvironmentIPAllowList
							if !slices.Equal(current.Spec.IPAllowList, before[current.Name].(*appv1alpha1.KeyValue).Spec.IPAllowList) {
								t.Error("empty clear changed owned Key Value rules")
							}
						}
						if member.GetLabels()[core.LabelProject] != wantProject || member.GetLabels()[core.LabelEnvironment] != "" || len(inherited) != 0 {
							t.Errorf("authorized empty clear retained membership: labels=%v inherited=%v", member.GetLabels(), inherited)
						}
						for name, original := range before {
							if name != member.GetName() && !reflect.DeepEqual(original, after[name]) {
								t.Errorf("authorized empty clear changed unrelated datastore %s", name)
							}
						}
					})
				}
			}
		}
	}
}

func TestGroupingProjectDeleteAuthorizesEveryChildBeforeMutation(t *testing.T) {
	for _, surface := range []string{"REST", "GraphQL", "MCP"} {
		t.Run(surface, func(t *testing.T) {
			f := newGroupingAdmissionFixture(t, true)
			before, groupingBefore := f.snapshot(t)
			f.mutate(t, groupingAdmissionMutation{surface: surface, grouping: "project", delete: true}, "forbidden")
			after, groupingAfter := f.snapshot(t)
			if f.writes.patches != 0 || f.writes.updates != 0 || !reflect.DeepEqual(before, after) || !bytes.Equal(groupingBefore, groupingAfter) {
				t.Errorf("later child's denial followed partial cleanup: patches=%d updates=%d objectsEqual=%v groupingsEqual=%v", f.writes.patches, f.writes.updates, reflect.DeepEqual(before, after), bytes.Equal(groupingBefore, groupingAfter))
			}
		})
	}
}
