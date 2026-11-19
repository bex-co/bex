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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/graphql-go/graphql"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/savedsearch"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// viewStore is the project fake with the saved-search capability: Render's
// three defaults per project, as the migration's trigger creates them.
type viewStore struct {
	*fakeProjectStore
	services map[string][]store.ProjectService
}

func newViewStore(projects ...store.Project) *viewStore {
	return &viewStore{fakeProjectStore: newFakeProjectStore(projects...), services: map[string][]store.ProjectService{}}
}

func (v *viewStore) defaults(projectID string) []store.SavedSearch {
	p := v.projects[projectID]
	var out []store.SavedSearch
	for _, d := range []struct{ name, kind string }{
		{"Application Logs", savedsearch.TypeApplicationLogs},
		{"Request Logs", savedsearch.TypeRequestLogs},
		{"Service Metrics", savedsearch.TypeMetrics},
	} {
		out = append(out, store.SavedSearch{ID: "svs-" + projectID + "-" + d.kind, ProjectID: projectID, TenantID: p.TenantID,
			Name: d.name, Type: d.kind, Filters: json.RawMessage(`{"filters": [], "version": 1}`)})
	}
	return out
}

func (v *viewStore) ListSavedSearches(_ context.Context, projectID, searchType string) ([]store.SavedSearch, error) {
	var out []store.SavedSearch
	for _, s := range v.defaults(projectID) {
		if searchType == "" || s.Type == searchType {
			out = append(out, s)
		}
	}
	return out, nil
}

func (v *viewStore) GetSavedSearch(_ context.Context, id string) (store.SavedSearch, error) {
	for projectID := range v.projects {
		for _, s := range v.defaults(projectID) {
			if s.ID == id {
				return s, nil
			}
		}
	}
	return store.SavedSearch{}, fmt.Errorf("saved search: %w", store.ErrNotFound)
}

func (v *viewStore) ListProjectServices(_ context.Context, projectID string) ([]store.ProjectService, error) {
	return v.services[projectID], nil
}

var (
	ownProject   = store.Project{ID: "prj-own", TenantID: "tea-own", Name: "own"}
	otherProject = store.Project{ID: "prj-other", TenantID: "tea-other", Name: "other"}
)

func viewService() (*Service, *viewStore) {
	st := newViewStore(ownProject, otherProject)
	return &Service{Base: &core.Base{Authz: denyObjectChecker(core.WorkspaceObject("tea-other"))}, Store: st}, st
}

// A member who may view the project reads its views, by type and by id; a
// view of a project in a workspace the caller is not in is refused as that
// project's own read is, and an unknown id is not found.
func TestSavedSearchesFollowTheProjectsViewPermission(t *testing.T) {
	svc, _ := viewService()
	ctx := ctxAs("member")

	views, err := svc.SavedSearches(ctx, ownProject.ID, "")
	if err != nil || len(views) != 3 || views[0].Name != "Application Logs" || views[2].Type != savedsearch.TypeMetrics {
		t.Fatalf("SavedSearches = %+v, %v", views, err)
	}
	requests, err := svc.SavedSearches(ctx, ownProject.ID, savedsearch.TypeRequestLogs)
	if err != nil || len(requests) != 1 || requests[0].Name != "Request Logs" {
		t.Fatalf("SavedSearches(REQUEST_LOGS) = %+v, %v", requests, err)
	}
	if _, err := svc.SavedSearches(ctx, ownProject.ID, "WORKFLOW_RUNS"); !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("WORKFLOW_RUNS views: %v, want a bad request", err)
	}
	one, err := svc.SavedSearch(ctx, views[0].ID)
	if err != nil || one.ID != views[0].ID || string(one.Filters) != `{"filters": [], "version": 1}` {
		t.Fatalf("SavedSearch = %+v, %v", one, err)
	}

	if _, err := svc.SavedSearches(ctx, otherProject.ID, ""); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("another workspace's views: %v, want the project read's ErrForbidden", err)
	}
	if _, err := svc.SavedSearch(ctx, "svs-prj-other-"+savedsearch.TypeApplicationLogs); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("another workspace's view: %v, want ErrForbidden", err)
	}
	if _, err := svc.SavedSearch(ctx, "svs-missing"); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("unknown view: %v, want ErrNotFound", err)
	}
	// A view named in another workspace than the one the caller acts in is
	// not found, as every by-id read in a named workspace is.
	if _, err := svc.SavedSearch(core.WithWorkspace(ctx, "tea-own"), "svs-prj-other-"+savedsearch.TypeApplicationLogs); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("a view outside the named workspace: %v, want ErrNotFound", err)
	}

	// No store with views: every verb is unavailable, never an empty answer.
	bare := &Service{Base: &core.Base{Authz: allowChecker{}}, Store: newFakeProjectStore(ownProject)}
	if _, err := bare.SavedSearches(ctx, ownProject.ID, ""); !errors.Is(err, core.ErrUnavailable) {
		t.Fatalf("store without views: %v, want unavailable", err)
	}
}

// The GraphQL reads return Render's fields, the filter document as an object.
func TestSavedSearchGraphQLReturnsRendersShape(t *testing.T) {
	svc, _ := viewService()
	schema, err := graphql.NewSchema(graphql.SchemaConfig{Query: graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()})})
	if err != nil {
		t.Fatal(err)
	}
	res := graphql.Do(graphql.Params{Schema: schema, Context: ctxAs("member"), RequestString: `{
		all: savedSearches(projectId: "prj-own") { id name type filters }
		logs: savedSearches(projectId: "prj-own", type: APPLICATION_LOGS) { name }
		one: savedSearch(id: "svs-prj-own-REQUEST_LOGS") { name type filters } }`})
	if len(res.Errors) > 0 {
		t.Fatal(res.Errors)
	}
	got, _ := json.Marshal(res.Data)
	want := `{"all":[` +
		`{"filters":{"filters":[],"version":1},"id":"svs-prj-own-APPLICATION_LOGS","name":"Application Logs","type":"APPLICATION_LOGS"},` +
		`{"filters":{"filters":[],"version":1},"id":"svs-prj-own-REQUEST_LOGS","name":"Request Logs","type":"REQUEST_LOGS"},` +
		`{"filters":{"filters":[],"version":1},"id":"svs-prj-own-METRICS","name":"Service Metrics","type":"METRICS"}],` +
		`"logs":[{"name":"Application Logs"}],` +
		`"one":{"filters":{"filters":[],"version":1},"name":"Request Logs","type":"REQUEST_LOGS"}}`
	if string(got) != want {
		t.Fatalf("GraphQL saved searches =\n%s\nwant\n%s", got, want)
	}
	res = graphql.Do(graphql.Params{Schema: schema, Context: ctxAs("member"), RequestString: `{ savedSearches(projectId: "prj-own", type: WORKFLOW_RUNS) { id } }`})
	if len(res.Errors) == 0 {
		t.Fatal("WORKFLOW_RUNS was accepted by the schema")
	}
}

// ProjectLogResources lists the project's services and datastores with their
// names, types and public environment ids, only to a caller who may view the
// project.
func TestProjectLogResourcesListsTheProject(t *testing.T) {
	svc, st := viewService()
	st.services[ownProject.ID] = []store.ProjectService{
		{ID: "srv-web", Name: "storefront", Type: "web_service", EnvironmentID: "evm-c185th5c2rvvnhbfiltg"},
		{ID: "srv-cron", Name: "nightly", Type: "cron_job"},
	}
	svc.Databases = newFakeResourceIndex("tea-own",
		projectResource{id: "dpg-db", projectID: ownProject.ID, environmentID: "env-c185th5c2rvvnhbfiltg"},
		projectResource{id: "dpg-elsewhere", projectID: "prj-third"})
	svc.KeyValues = newFakeResourceIndex("tea-own", projectResource{id: "red-kv", projectID: ownProject.ID})

	workspace, resources, err := svc.ProjectLogResources(ctxAs("member"), ownProject.ID)
	if err != nil || workspace != "tea-own" {
		t.Fatalf("ProjectLogResources = %q, %v", workspace, err)
	}
	want := []savedsearch.Resource{
		{ID: "srv-web", Name: "storefront", Type: "web_service", EnvironmentID: "evm-c185th5c2rvvnhbfiltg"},
		{ID: "srv-cron", Name: "nightly", Type: "cron_job"},
		{ID: "dpg-db", Type: savedsearch.KindPostgres, EnvironmentID: "evm-c185th5c2rvvnhbfiltg"},
		{ID: "red-kv", Type: savedsearch.KindKeyValue},
	}
	if fmt.Sprint(resources) != fmt.Sprint(want) {
		t.Fatalf("resources = %+v, want %+v", resources, want)
	}
	if _, _, err := svc.ProjectLogResources(ctxAs("member"), otherProject.ID); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("another workspace's project: %v, want ErrForbidden", err)
	}
}
