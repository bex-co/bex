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
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/graphql-go/graphql"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/savedsearch"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// viewStore is the project fake with the saved-search capability: Render's
// three defaults per project, as the migration's trigger creates them, then
// whatever the write verbs change.
type viewStore struct {
	*fakeProjectStore
	services map[string][]store.ProjectService
	views    []store.SavedSearch
	next     int
}

func newViewStore(projects ...store.Project) *viewStore {
	v := &viewStore{fakeProjectStore: newFakeProjectStore(projects...), services: map[string][]store.ProjectService{}}
	for _, p := range projects {
		for _, d := range []struct{ name, kind string }{
			{"Application Logs", savedsearch.TypeApplicationLogs},
			{"Request Logs", savedsearch.TypeRequestLogs},
			{"Service Metrics", savedsearch.TypeMetrics},
		} {
			v.views = append(v.views, store.SavedSearch{ID: "svs-" + p.ID + "-" + d.kind, ProjectID: p.ID, TenantID: p.TenantID,
				Name: d.name, Type: d.kind, Filters: json.RawMessage(`{"filters": [], "version": 1}`)})
		}
	}
	return v
}

func (v *viewStore) ListSavedSearches(_ context.Context, projectID, searchType string) ([]store.SavedSearch, error) {
	var out []store.SavedSearch
	for _, s := range v.views {
		if s.ProjectID == projectID && (searchType == "" || s.Type == searchType) {
			out = append(out, s)
		}
	}
	return out, nil
}

func (v *viewStore) GetSavedSearch(_ context.Context, id string) (store.SavedSearch, error) {
	for _, s := range v.views {
		if s.ID == id {
			return s, nil
		}
	}
	return store.SavedSearch{}, fmt.Errorf("saved search: %w", store.ErrNotFound)
}

// nameTaken is the store's unique (project_id, name), except for the view
// being written.
func (v *viewStore) nameTaken(projectID, name, except string) bool {
	for _, s := range v.views {
		if s.ProjectID == projectID && s.Name == name && s.ID != except {
			return true
		}
	}
	return false
}

func (v *viewStore) CreateSavedSearch(_ context.Context, row store.SavedSearch) (store.SavedSearch, error) {
	p, ok := v.projects[row.ProjectID]
	if !ok {
		return store.SavedSearch{}, fmt.Errorf("saved search reference: %w", store.ErrNotFound)
	}
	if v.nameTaken(row.ProjectID, row.Name, "") {
		return store.SavedSearch{}, fmt.Errorf("saved search: %w", store.ErrConflict)
	}
	if views, _ := v.ListSavedSearches(context.Background(), row.ProjectID, ""); len(views) >= store.MaxSavedSearchesPerProject {
		return store.SavedSearch{}, store.ErrSavedSearchLimit
	}
	v.next++
	row.ID, row.TenantID = fmt.Sprintf("svs-new-%d", v.next), p.TenantID
	v.views = append(v.views, row)
	return row, nil
}

func (v *viewStore) UpdateSavedSearch(_ context.Context, edit store.SavedSearch) (store.SavedSearch, error) {
	for i, s := range v.views {
		if s.ID != edit.ID {
			continue
		}
		if v.nameTaken(s.ProjectID, edit.Name, edit.ID) {
			return store.SavedSearch{}, fmt.Errorf("saved search: %w", store.ErrConflict)
		}
		v.views[i].Name, v.views[i].Filters = edit.Name, edit.Filters
		return v.views[i], nil
	}
	return store.SavedSearch{}, fmt.Errorf("saved search: %w", store.ErrNotFound)
}

func (v *viewStore) DeleteSavedSearch(_ context.Context, id string) error {
	for i, s := range v.views {
		if s.ID == id {
			v.views = append(v.views[:i], v.views[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("saved search: %w", store.ErrNotFound)
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

// viewerChecker is a workspace viewer: it may view the project and nothing
// that needs the project's edit permission.
type viewerChecker struct{}

func (viewerChecker) Check(_ context.Context, _, relation, _ string) (bool, error) {
	return relation == core.RelCanView, nil
}

// viewAuditSink records the audit rows a view write leaves.
type viewAuditSink struct{ events []core.AuditEvent }

func (s *viewAuditSink) Record(_ context.Context, ev core.AuditEvent) error {
	s.events = append(s.events, ev)
	return nil
}

func (s *viewAuditSink) take() []core.AuditEvent {
	out := s.events
	s.events = nil
	return out
}

func viewNames(t *testing.T, svc *Service, projectID string) []string {
	t.Helper()
	views, err := svc.SavedSearches(ctxAs("someone-else"), projectID, "")
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, v := range views {
		names = append(names, v.Name)
	}
	return names
}

// A project editor creates, edits, duplicates and deletes views, a default one
// too, and each change is what any other member then reads.
func TestProjectEditorManagesViews(t *testing.T) {
	st := newViewStore(ownProject)
	svc := &Service{Base: &core.Base{Authz: allowChecker{}}, Store: st}
	ctx := ctxAs("editor")

	created, err := svc.CreateSavedSearch(ctx, ownProject.ID, savedsearch.TypeApplicationLogs, "  Storefront errors ", map[string]any{
		"version": 1, "filters": []any{map[string]any{"field": "LEVEL", "operator": "IS", "values": []any{"error"}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.Name != "Storefront errors" || created.Type != savedsearch.TypeApplicationLogs ||
		string(created.Filters) != `{"filters":[{"field":"LEVEL","operator":"IS","values":["error"]}],"version":1}` {
		t.Fatalf("created %+v (%s)", created, created.Filters)
	}
	got, err := svc.SavedSearch(ctxAs("someone-else"), created.ID)
	if err != nil || got.Name != "Storefront errors" {
		t.Fatalf("another member reads %+v, %v", got, err)
	}

	// No document is Render's empty one.
	plain, err := svc.CreateSavedSearch(ctx, ownProject.ID, savedsearch.TypeRequestLogs, "API requests", nil)
	if err != nil || string(plain.Filters) != `{"filters":[],"version":1}` {
		t.Fatalf("created without filters: %+v (%s), %v", plain, plain.Filters, err)
	}

	renamed := "Checkout errors"
	edited, err := svc.UpdateSavedSearch(ctx, created.ID, SavedSearchEdit{Name: &renamed, Filters: map[string]any{
		"version": 1, "filters": []any{map[string]any{"field": "RESOURCE_TYPE", "operator": "IS_NOT", "values": []any{"POSTGRES"}}},
	}})
	if err != nil || edited.Name != renamed || string(edited.Filters) != `{"filters":[{"field":"RESOURCE_TYPE","operator":"IS_NOT","values":["POSTGRES"]}],"version":1}` {
		t.Fatalf("edited %+v (%s), %v", edited, edited.Filters, err)
	}
	// An edit naming only the name keeps the filters, and the same type is no change.
	again := "Checkout"
	kept, err := svc.UpdateSavedSearch(ctx, created.ID, SavedSearchEdit{Name: &again, Type: savedsearch.TypeApplicationLogs})
	if err != nil || kept.Name != again || string(kept.Filters) != string(edited.Filters) {
		t.Fatalf("name-only edit %+v (%s), %v", kept, kept.Filters, err)
	}

	copied, err := svc.DuplicateSavedSearch(ctx, created.ID)
	if err != nil || copied.Name != "Checkout (copy)" || copied.Type != savedsearch.TypeApplicationLogs ||
		string(copied.Filters) != string(edited.Filters) || copied.ID == created.ID {
		t.Fatalf("duplicate %+v (%s), %v", copied, copied.Filters, err)
	}
	second, err := svc.DuplicateSavedSearch(ctx, created.ID)
	if err != nil || second.Name != "Checkout (copy 2)" {
		t.Fatalf("second duplicate %+v, %v", second, err)
	}
	defaultCopy, err := svc.DuplicateSavedSearch(ctx, "svs-prj-own-"+savedsearch.TypeRequestLogs)
	if err != nil || defaultCopy.Name != "Request Logs (copy)" || defaultCopy.Type != savedsearch.TypeRequestLogs {
		t.Fatalf("a default view's duplicate %+v, %v", defaultCopy, err)
	}

	want := []string{"Application Logs", "Request Logs", "Service Metrics", "Checkout", "API requests", "Checkout (copy)", "Checkout (copy 2)", "Request Logs (copy)"}
	if got := viewNames(t, svc, ownProject.ID); fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("views = %q, want %q", got, want)
	}

	// Every view goes, the defaults too, and the project still answers.
	views, _ := svc.SavedSearches(ctx, ownProject.ID, "")
	for _, v := range views {
		if err := svc.DeleteSavedSearch(ctx, v.ID); err != nil {
			t.Fatalf("delete %s: %v", v.Name, err)
		}
	}
	if got := viewNames(t, svc, ownProject.ID); len(got) != 0 {
		t.Fatalf("views after deleting every one = %q", got)
	}
	if err := svc.DeleteSavedSearch(ctx, created.ID); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("deleting a deleted view: %v, want ErrNotFound", err)
	}
	if _, err := svc.Get(ctx, ownProject.ID); err != nil {
		t.Fatalf("the project with no views: %v", err)
	}
}

// A viewer reads the views but every write is refused before it lands, and a
// project in another workspace is refused as its own read is.
func TestViewWritesNeedTheProjectsEditPermission(t *testing.T) {
	st := newViewStore(ownProject, otherProject)
	viewer := &Service{Base: &core.Base{Authz: viewerChecker{}}, Store: st}
	ctx := ctxAs("viewer")
	view := "svs-prj-own-" + savedsearch.TypeApplicationLogs
	name := "Mine"

	if _, err := viewer.SavedSearches(ctx, ownProject.ID, ""); err != nil {
		t.Fatalf("a viewer reads the views: %v", err)
	}
	for verb, err := range map[string]error{
		"create":    second(viewer.CreateSavedSearch(ctx, ownProject.ID, savedsearch.TypeApplicationLogs, name, nil)),
		"update":    second(viewer.UpdateSavedSearch(ctx, view, SavedSearchEdit{Name: &name})),
		"duplicate": second(viewer.DuplicateSavedSearch(ctx, view)),
		"delete":    viewer.DeleteSavedSearch(ctx, view),
	} {
		if !errors.Is(err, core.ErrForbidden) {
			t.Errorf("a viewer's %s: %v, want ErrForbidden", verb, err)
		}
	}
	if got := viewNames(t, viewer, ownProject.ID); fmt.Sprint(got) != "[Application Logs Request Logs Service Metrics]" {
		t.Fatalf("a viewer's refused writes changed the views: %q", got)
	}

	editor, _ := viewService()
	editor.Store = st
	other := "svs-prj-other-" + savedsearch.TypeApplicationLogs
	for verb, err := range map[string]error{
		"create":    second(editor.CreateSavedSearch(ctxAs("member"), otherProject.ID, savedsearch.TypeApplicationLogs, name, nil)),
		"update":    second(editor.UpdateSavedSearch(ctxAs("member"), other, SavedSearchEdit{Name: &name})),
		"duplicate": second(editor.DuplicateSavedSearch(ctxAs("member"), other)),
		"delete":    editor.DeleteSavedSearch(ctxAs("member"), other),
	} {
		if !errors.Is(err, core.ErrForbidden) {
			t.Errorf("%s of another workspace's view: %v, want ErrForbidden", verb, err)
		}
	}
	if _, err := editor.UpdateSavedSearch(ctxAs("member"), "svs-missing", SavedSearchEdit{Name: &name}); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("editing an unknown view: %v, want ErrNotFound", err)
	}
}

func second[T any](_ T, err error) error { return err }

// The write path refuses what Render refuses: a name the project already
// uses with duplicate_saved_search_name, a WORKFLOW_RUNS view, a type change,
// and a document the view's type does not take.
func TestViewWritesRefuseByName(t *testing.T) {
	st := newViewStore(ownProject)
	svc := &Service{Base: &core.Base{Authz: allowChecker{}}, Store: st}
	ctx := ctxAs("editor")
	logs := "svs-prj-own-" + savedsearch.TypeApplicationLogs

	_, err := svc.CreateSavedSearch(ctx, ownProject.ID, savedsearch.TypeRequestLogs, "Application Logs", nil)
	var coded *core.CodedError
	if !errors.As(err, &coded) || coded.Code != DuplicateSavedSearchNameCode || !errors.Is(err, core.ErrConflict) {
		t.Fatalf("a taken name on create: %v, want %s", err, DuplicateSavedSearchNameCode)
	}
	taken := "Request Logs"
	if _, err := svc.UpdateSavedSearch(ctx, logs, SavedSearchEdit{Name: &taken}); !errors.As(err, &coded) || coded.Code != DuplicateSavedSearchNameCode {
		t.Fatalf("a taken name on edit: %v, want %s", err, DuplicateSavedSearchNameCode)
	}

	blank := "   "
	for label, err := range map[string]error{
		"WORKFLOW_RUNS":   second(svc.CreateSavedSearch(ctx, ownProject.ID, "WORKFLOW_RUNS", "Runs", nil)),
		"no type":         second(svc.CreateSavedSearch(ctx, ownProject.ID, "", "Untyped", nil)),
		"blank name":      second(svc.CreateSavedSearch(ctx, ownProject.ID, savedsearch.TypeApplicationLogs, " ", nil)),
		"blank rename":    second(svc.UpdateSavedSearch(ctx, logs, SavedSearchEdit{Name: &blank})),
		"type change":     second(svc.UpdateSavedSearch(ctx, logs, SavedSearchEdit{Type: savedsearch.TypeRequestLogs})),
		"another version": second(svc.CreateSavedSearch(ctx, ownProject.ID, savedsearch.TypeApplicationLogs, "v2", map[string]any{"version": 2, "filters": []any{}})),
		"a request field on application logs": second(svc.UpdateSavedSearch(ctx, logs, SavedSearchEdit{Filters: map[string]any{
			"version": 1, "filters": []any{map[string]any{"field": "METHOD", "operator": "IS", "values": []any{"GET"}}},
		}})),
		"IS_NOT on Text Search": second(svc.UpdateSavedSearch(ctx, logs, SavedSearchEdit{Filters: map[string]any{
			"version": 1, "filters": []any{map[string]any{"field": "TEXT", "operator": "IS_NOT", "values": []any{"timeout"}}},
		}})),
	} {
		if !errors.Is(err, core.ErrBadRequest) {
			t.Errorf("%s: %v, want a bad request", label, err)
		}
	}
	if got, _ := svc.SavedSearch(ctx, logs); got.Name != "Application Logs" || string(got.Filters) != `{"filters": [], "version": 1}` {
		t.Fatalf("a refused edit changed the view: %+v (%s)", got, got.Filters)
	}
	// Render's editor negates a line field (w3/m523), and so a view stores it.
	notInfo := map[string]any{"version": 1, "filters": []any{map[string]any{"field": "LEVEL", "operator": "IS_NOT", "values": []any{"info"}}}}
	if got, err := svc.UpdateSavedSearch(ctx, logs, SavedSearchEdit{Filters: notInfo}); err != nil || !strings.Contains(string(got.Filters), `"IS_NOT"`) {
		t.Fatalf("LEVEL IS_NOT info: %+v, %v; want it stored", got, err)
	}

	// A project's views are bounded, and the bound is refused by its own code.
	for i := len(st.views); i < store.MaxSavedSearchesPerProject; i++ {
		if _, err := svc.CreateSavedSearch(ctx, ownProject.ID, savedsearch.TypeApplicationLogs, fmt.Sprintf("view %d", i), nil); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := svc.DuplicateSavedSearch(ctx, logs); !errors.As(err, &coded) || coded.Code != SavedSearchLimitCode {
		t.Fatalf("a view past the bound: %v, want %s", err, SavedSearchLimitCode)
	}
}

// A copy's name stays a valid label: a long name is cut to make room for the
// suffix, and the next free number is taken.
func TestCopyNameFitsALabel(t *testing.T) {
	long := strings.Repeat("é", core.MaxLabelLength)
	got := copyName(long, map[string]bool{})
	if utf8.RuneCountInString(got) != core.MaxLabelLength || !strings.HasSuffix(got, " (copy)") {
		t.Fatalf("copy of a %d-rune name = %q (%d runes)", core.MaxLabelLength, got, utf8.RuneCountInString(got))
	}
	taken := map[string]bool{"Logs (copy)": true, "Logs (copy 2)": true}
	if got := copyName("Logs", taken); got != "Logs (copy 3)" {
		t.Fatalf("copyName = %q, want Logs (copy 3)", got)
	}
}

// Render's audit log records a view's create (and a duplicate's) and its
// delete; an edit, and a refused write, record no allowed row.
func TestViewWritesRecordRendersEvents(t *testing.T) {
	sink := &viewAuditSink{}
	svc := &Service{Base: &core.Base{Authz: allowChecker{}, Audit: sink}, Store: newViewStore(ownProject)}
	ctx := ctxAs("editor")

	created, err := svc.CreateSavedSearch(ctx, ownProject.ID, savedsearch.TypeApplicationLogs, "Errors", nil)
	if err != nil {
		t.Fatal(err)
	}
	wantRow := func(label, verb, id, name string) {
		t.Helper()
		events := sink.take()
		if len(events) != 1 {
			t.Fatalf("%s recorded %d rows, want 1: %+v", label, len(events), events)
		}
		ev := events[0]
		if ev.Verb != verb || ev.Outcome != core.AuditAllowed || ev.Target != core.SavedSearchTarget(id) ||
			ev.TargetName != name || ev.Resource != core.WorkspaceObject(ownProject.TenantID) || ev.Caller != "editor" {
			t.Fatalf("%s recorded %+v", label, ev)
		}
	}
	wantRow("create", "projects.CreateSavedSearch", created.ID, "Errors")

	renamed := "Errors only"
	if _, err := svc.UpdateSavedSearch(ctx, created.ID, SavedSearchEdit{Name: &renamed}); err != nil {
		t.Fatal(err)
	}
	if events := sink.take(); len(events) != 0 {
		t.Fatalf("an edit recorded %+v; Render's enum has no update event", events)
	}

	copied, err := svc.DuplicateSavedSearch(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	wantRow("duplicate", "projects.DuplicateSavedSearch", copied.ID, "Errors only (copy)")

	if _, err := svc.CreateSavedSearch(ctx, ownProject.ID, savedsearch.TypeApplicationLogs, "Errors only", nil); err == nil {
		t.Fatal("a taken name was created")
	}
	if events := sink.take(); len(events) != 0 {
		t.Fatalf("a refused create recorded %+v", events)
	}

	if err := svc.DeleteSavedSearch(ctx, created.ID); err != nil {
		t.Fatal(err)
	}
	wantRow("delete", "projects.DeleteSavedSearch", created.ID, "Errors only")

	// A viewer's refusal is recorded as the denial it is, under the verb.
	viewer := &Service{Base: &core.Base{Authz: viewerChecker{}, Audit: sink}, Store: svc.Store}
	if err := viewer.DeleteSavedSearch(ctxAs("viewer"), copied.ID); !errors.Is(err, core.ErrForbidden) {
		t.Fatal(err)
	}
	if events := sink.take(); len(events) != 1 || events[0].Verb != "projects.DeleteSavedSearch" || events[0].Outcome != core.AuditDenied {
		t.Fatalf("a viewer's delete recorded %+v, want one denied projects.DeleteSavedSearch", events)
	}
}

// The GraphQL mutations take Render's arguments, return the view with its
// document as an object, and carry Render's error code in the extensions.
func TestSavedSearchMutationsOverGraphQL(t *testing.T) {
	svc := &Service{Base: &core.Base{Authz: allowChecker{}}, Store: newViewStore(ownProject)}
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
		Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: svc.GraphQLMutation()}),
	})
	if err != nil {
		t.Fatal(err)
	}
	do := func(query string, vars map[string]any) *graphql.Result {
		t.Helper()
		return graphql.Do(graphql.Params{Schema: schema, Context: ctxAs("editor"), RequestString: query, VariableValues: vars})
	}
	res := do(`mutation($f: JSONObject) { createSavedSearch(projectId: "prj-own", type: REQUEST_LOGS, name: "5xx", filters: $f) { id name type filters } }`,
		map[string]any{"f": map[string]any{"version": 1, "filters": []any{map[string]any{"field": "STATUS_CODE", "operator": "IS", "values": []any{"500", "503"}}}}})
	if len(res.Errors) > 0 {
		t.Fatal(res.Errors)
	}
	created := res.Data.(map[string]any)["createSavedSearch"].(map[string]any)
	got, _ := json.Marshal(created)
	if want := `{"filters":{"filters":[{"field":"STATUS_CODE","operator":"IS","values":["500","503"]}],"version":1},"id":"svs-new-1","name":"5xx","type":"REQUEST_LOGS"}`; string(got) != want {
		t.Fatalf("createSavedSearch = %s, want %s", got, want)
	}

	res = do(`mutation { updateSavedSearch(id: "svs-new-1", name: "Server errors", filters: {version: 1, filters: []}) { name filters } }`, nil)
	if got, _ := json.Marshal(res.Data); len(res.Errors) > 0 || string(got) != `{"updateSavedSearch":{"filters":{"filters":[],"version":1},"name":"Server errors"}}` {
		t.Fatalf("updateSavedSearch = %s, %v", got, res.Errors)
	}
	res = do(`mutation { duplicateSavedSearch(id: "svs-new-1") { id name type } }`, nil)
	if got, _ := json.Marshal(res.Data); len(res.Errors) > 0 || string(got) != `{"duplicateSavedSearch":{"id":"svs-new-2","name":"Server errors (copy)","type":"REQUEST_LOGS"}}` {
		t.Fatalf("duplicateSavedSearch = %s, %v", got, res.Errors)
	}
	res = do(`mutation { deleteSavedSearch(id: "svs-new-2") }`, nil)
	if got, _ := json.Marshal(res.Data); len(res.Errors) > 0 || string(got) != `{"deleteSavedSearch":"svs-new-2"}` {
		t.Fatalf("deleteSavedSearch = %s, %v", got, res.Errors)
	}

	res = do(`mutation { createSavedSearch(projectId: "prj-own", type: APPLICATION_LOGS, name: "Request Logs") { id } }`, nil)
	if len(res.Errors) != 1 || res.Errors[0].Extensions["code"] != DuplicateSavedSearchNameCode {
		t.Fatalf("a taken name answered %+v, want extensions.code %s", res.Errors, DuplicateSavedSearchNameCode)
	}
	for _, refused := range []string{
		`mutation { createSavedSearch(projectId: "prj-own", type: WORKFLOW_RUNS, name: "Runs") { id } }`,
		`mutation { updateSavedSearch(id: "svs-new-1", type: APPLICATION_LOGS) { id } }`,
	} {
		if res := do(refused, nil); len(res.Errors) == 0 {
			t.Errorf("%s was accepted", refused)
		}
	}
}
