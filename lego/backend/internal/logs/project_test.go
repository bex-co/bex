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

package logs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/graphql-go/graphql"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/savedsearch"
)

// fakeProject is one project's resource listing, as projects.Service answers
// it; the listing can change while a tail runs.
type fakeProject struct {
	mu        sync.Mutex
	workspace string
	resources []savedsearch.Resource
	err       error
	calls     int
}

func (f *fakeProject) ProjectLogResources(_ context.Context, projectID string) (string, []savedsearch.Resource, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	if f.err != nil {
		return "", nil, f.err
	}
	if projectID != "prj-observed" {
		return "", nil, core.NotFound("project")
	}
	return f.workspace, append([]savedsearch.Resource(nil), f.resources...), nil
}

func (f *fakeProject) set(resources ...savedsearch.Resource) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.resources = resources
}

const (
	projectWeb  = "web"
	projectCron = "nightly"
	// projectPG is the fixture Postgres line as a log line shows it.
	projectPG = postgresID + ":2026-07-05 00:00:03 UTC [7] LOG:  checkpoint complete"
)

// projectFixture is a project with a web service, a cron job and a Postgres,
// the web service and the database in one environment, each with lines that
// interleave in time.
func projectFixture(objs ...client.Object) (*Service, *fakeProject) {
	cron := sampleApp(projectCron)
	cron.Spec.Type = "cron_job"
	lines := map[string][]string{
		"web-1":     {"2026-07-05T00:00:01Z web first", "2026-07-05T00:00:04Z web second"},
		"nightly-1": {"2026-07-05T00:00:02Z cron ran"},
		postgresID + "-1": {
			`2026-07-05T00:00:03Z {"logger":"postgres","msg":"record","record":{"log_time":"2026-07-05 00:00:03 UTC","process_id":7,"error_severity":"LOG","message":"checkpoint complete"}}`,
		},
		"elsewhere-1": {"2026-07-05T00:00:05Z not in the project"},
	}
	objs = append(objs, sampleApp(projectWeb), podFor(projectWeb, "web-1"), cron, podFor(projectCron, "nightly-1"),
		sampleDatabase(postgresID), databasePod(postgresID, postgresID+"-1"),
		sampleApp("elsewhere"), podFor("elsewhere", "elsewhere-1"))
	svc := newService(lines, objs...)
	project := &fakeProject{workspace: "tea-home", resources: []savedsearch.Resource{
		{ID: projectWeb, Name: "storefront", Type: "web_service", EnvironmentID: "evm-c185th5c2rvvnhbfiltg"},
		{ID: projectCron, Name: "nightly-report", Type: "cron_job"},
		{ID: postgresID, Name: "orders-db", Type: savedsearch.KindPostgres, EnvironmentID: "evm-c185th5c2rvvnhbfiltg"},
		{ID: "srv-static", Name: "docs", Type: "static_site"},
	}}
	svc.Projects = project
	return svc, project
}

// runProjectQuery reads the project view from the fixture day: without a
// startTime the read covers only the last hour (w5/197).
func runProjectQuery(t *testing.T, svc *Service, ctx context.Context, args string) (map[string]any, []error) {
	t.Helper()
	args += `, startTime:"` + fixtureDay + `"`
	schema, err := gqlSchema(svc)
	if err != nil {
		t.Fatal(err)
	}
	res := graphql.Do(graphql.Params{Schema: schema, Context: ctx, RequestString: `{ logs(` + args + `) {
		hasMore nextStartTime nextEndTime
		logs { message resource instance type }
		resources { id name type environmentId } } }`})
	var errs []error
	for _, e := range res.Errors {
		errs = append(errs, e)
	}
	data, _ := res.Data.(map[string]any)
	logs, _ := data["logs"].(map[string]any)
	return logs, errs
}

func lineResources(t *testing.T, list map[string]any) []string {
	t.Helper()
	var out []string
	for _, row := range list["logs"].([]any) {
		r := row.(map[string]any)
		out = append(out, fmt.Sprintf("%s:%s", r["resource"], r["message"]))
	}
	return out
}

// The DoD's core: one query reads the project's web service, cron job and
// Postgres, interleaved in time, each line naming its resource, and the page
// lists the resources it read. Neither the static site (no application log)
// nor a service outside the project contributes.
func TestProjectApplicationLogsInterleaveEveryResource(t *testing.T) {
	svc, _ := projectFixture()
	list, errs := runProjectQuery(t, svc, context.Background(), `project:"prj-observed", type:"app", direction:"forward"`)
	if errs != nil {
		t.Fatal(errs)
	}
	got := lineResources(t, list)
	want := []string{"web:web first", "nightly:cron ran", projectPG, "web:web second"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("project lines = %v, want %v", got, want)
	}
	var names []string
	for _, r := range list["resources"].([]any) {
		m := r.(map[string]any)
		names = append(names, fmt.Sprintf("%s=%s/%s", m["id"], m["name"], m["type"]))
	}
	if strings.Join(names, ",") != "web=storefront/web_service,nightly=nightly-report/cron_job,"+postgresID+"=orders-db/postgres" {
		t.Fatalf("resources = %v", names)
	}
}

// An ENVIRONMENT filter narrows the view to that environment's resources,
// and RESOURCE_TYPE to the kinds it names.
func TestProjectViewFiltersNarrowTheScope(t *testing.T) {
	svc, _ := projectFixture()
	for _, tc := range []struct {
		doc  string
		want string
	}{
		{`{filters: [{field: "ENVIRONMENT", operator: "IS", values: ["evm-c185th5c2rvvnhbfiltg"]}], version: 1}`, "web:web first|" + projectPG + "|web:web second"},
		{`{filters: [{field: "RESOURCE_TYPE", operator: "IS_NOT", values: ["SERVICE", "POSTGRES"]}], version: 1}`, "nightly:cron ran"},
		{`{filters: [{field: "RESOURCE", operator: "IS", values: ["` + postgresID + `"]}], version: 1}`, projectPG},
		{`{filters: [{field: "TEXT", operator: "IS", values: ["second"]}], version: 1}`, "web:web second"},
	} {
		list, errs := runProjectQuery(t, svc, context.Background(), `project:"prj-observed", direction:"forward", filterDocument:`+tc.doc)
		if errs != nil {
			t.Fatalf("%s: %v", tc.doc, errs)
		}
		if got := strings.Join(lineResources(t, list), "|"); got != tc.want {
			t.Errorf("%s: lines = %s, want %s", tc.doc, got, tc.want)
		}
	}
}

// A caller who may view the project but not read its logs reads an empty
// page: no line, and no resource named.
func TestProjectViewNamesNothingTheCallerCannotRead(t *testing.T) {
	svc, _ := projectFixture()
	svc.Base.Workspace = homeWorkspaceOnly{}
	svc.Base.Authz = relationChecker{deny: core.RelCanViewLogs}
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "viewer", Method: "session"})
	list, errs := runProjectQuery(t, svc, ctx, `project:"prj-observed"`)
	if errs != nil {
		t.Fatal(errs)
	}
	if rows := list["logs"].([]any); len(rows) != 0 {
		t.Fatalf("a caller without log access read %v", rows)
	}
	if rs := list["resources"].([]any); len(rs) != 0 {
		t.Fatalf("a caller without log access was told about %v", rs)
	}
}

// A listed resource that is gone by the read (or refuses) is dropped: it
// contributes no line and is not named, and the rest still read.
func TestProjectViewDropsAResourceThatIsGone(t *testing.T) {
	svc, project := projectFixture()
	project.set(append(project.resources, savedsearch.Resource{ID: "srv-gone", Name: "deleted", Type: "web_service"})...)
	list, errs := runProjectQuery(t, svc, context.Background(), `project:"prj-observed", direction:"forward"`)
	if errs != nil {
		t.Fatal(errs)
	}
	for _, r := range list["resources"].([]any) {
		if r.(map[string]any)["id"] == "srv-gone" {
			t.Fatal("a resource that is gone was named")
		}
	}
	if len(lineResources(t, list)) != 4 {
		t.Fatalf("lines = %v", lineResources(t, list))
	}
}

// Request Logs reads web services only, and a Hobby workspace is refused with
// the plan error, as a service's request logs are.
func TestProjectRequestLogsFollowThePlan(t *testing.T) {
	svc, _ := projectFixture()
	svc.Plans = core.WorkspacePlanFunc(func(context.Context, string) (string, error) { return "hobby", nil })
	_, errs := runProjectQuery(t, svc, context.Background(), `project:"prj-observed", type:"request"`)
	if len(errs) != 1 || !strings.Contains(errs[0].Error(), "HTTP request logs require the Pro plan") {
		t.Fatalf("hobby Request Logs = %v, want the plan refusal", errs)
	}

	svc.Plans = core.WorkspacePlanFunc(func(context.Context, string) (string, error) { return "pro", nil })
	v, err := newProjectView("prj-observed", nil, &LogQuery{Types: []string{LogTypeRequest}})
	if err != nil {
		t.Fatal(err)
	}
	_, resources, err := svc.resolveProjectView(context.Background(), v, nil)
	if err != nil || len(resources) != 1 || resources[0].ID != projectWeb {
		t.Fatalf("Request Logs scope = %+v, %v; want the web service alone", resources, err)
	}
}

// What a project read refuses, each by name: a scope past the cap, a filter
// document that is not valid for its view, a filter set twice, and a read
// that names both a resource and a project.
func TestProjectReadRefusals(t *testing.T) {
	svc, project := projectFixture()
	for _, tc := range []struct {
		args string
		want string
	}{
		{`project:"prj-observed", resource:"web"`, "exactly one of resource and project"},
		{`resource:"web", filterDocument:{filters: [], version: 1}`, "filterDocument applies to a project read"},
		{`project:"prj-observed", type:"build"`, "a project log view reads type=app or type=request"},
		{`project:"prj-observed", filterDocument:{filters: [], version: 2}`, "version 2 is not supported"},
		{`project:"prj-observed", filterDocument:{filters: [{field: "METHOD", operator: "IS", values: ["GET"]}], version: 1}`, `filter field "METHOD" is not a field of a APPLICATION_LOGS view`},
		{`project:"prj-observed", filterDocument:{filters: [{field: "LEVEL", operator: "IS_NOT", values: ["error"]}], version: 1}`, `"LEVEL" does not support operator IS_NOT`},
		{`project:"prj-observed", level:["error"], filterDocument:{filters: [{field: "LEVEL", operator: "IS", values: ["warning"]}], version: 1}`, "the view already filters on LEVEL"},
		{`project:"prj-missing"`, "not found"},
	} {
		_, errs := runProjectQuery(t, svc, context.Background(), tc.args)
		if len(errs) != 1 || !strings.Contains(errs[0].Error(), tc.want) {
			t.Errorf("%s: errors %v, want one naming %q", tc.args, errs, tc.want)
		}
	}

	var many []savedsearch.Resource
	for i := range maxProjectLogResources + 1 {
		many = append(many, savedsearch.Resource{ID: fmt.Sprintf("srv-%02d", i), Name: "s", Type: "web_service"})
	}
	project.set(many...)
	_, errs := runProjectQuery(t, svc, context.Background(), `project:"prj-observed"`)
	if len(errs) != 1 || !strings.Contains(errs[0].Error(), "this view reads 21 resources, and one project log query reads at most 20") {
		t.Fatalf("over the cap: %v", errs)
	}

	svc.Projects = nil
	if _, errs := runProjectQuery(t, svc, context.Background(), `project:"prj-observed"`); len(errs) != 1 || !strings.Contains(errs[0].Error(), "project log views need the projects store") {
		t.Fatalf("unwired: %v", errs)
	}
}

// relationChecker allows every relation but one.
type relationChecker struct{ deny string }

func (c relationChecker) Check(_ context.Context, _, relation, _ string) (bool, error) {
	return relation != c.deny, nil
}

// subscribeProject opens a project tail and returns the SSE frames once the
// caller's wait ends.
func subscribeProject(t *testing.T, svc *Service, query url.Values, wait time.Duration) *httptest.ResponseRecorder {
	t.Helper()
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/logs/subscribe?"+query.Encode(), nil)
	req.Header.Set("Accept", "text/event-stream")
	ctx, cancel := context.WithTimeout(req.Context(), wait)
	defer cancel()
	mux.ServeHTTP(rec, req.WithContext(ctx))
	return rec
}

func tailResources(t *testing.T, body string) map[string][]string {
	t.Helper()
	out := map[string][]string{}
	for _, line := range strings.Split(body, "\n") {
		payload, ok := strings.CutPrefix(line, "data: ")
		if !ok {
			continue
		}
		var l renderLog
		if err := json.Unmarshal([]byte(payload), &l); err != nil {
			t.Fatalf("frame %q: %v", payload, err)
		}
		for _, label := range l.Labels {
			if label.Name == "resource" {
				out[label.Value] = append(out[label.Value], l.Message)
			}
		}
	}
	return out
}

// The live tail follows every resource in the view with its resource label,
// and a resource that joins the project joins the tail at the next
// re-resolution. The whole tail holds one subscription slot.
func TestProjectTailFollowsTheScopeAsItChanges(t *testing.T) {
	svc, project := projectFixture()
	svc.MaxSSEConnsPerSubject = 1
	svc.ProjectRescopeInterval = 50 * time.Millisecond
	web, cron, pg := project.resources[0], project.resources[1], project.resources[2]
	project.set(web, pg) // the cron job joins later
	go func() {
		time.Sleep(120 * time.Millisecond)
		project.set(web, pg, cron)
	}()
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "alice", Method: "session"})
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/logs/subscribe?project=prj-observed&type=app", nil).WithContext(ctx)
	req.Header.Set("Accept", "text/event-stream")
	reqCtx, cancel := context.WithTimeout(req.Context(), 600*time.Millisecond)
	defer cancel()
	mux.ServeHTTP(rec, req.WithContext(reqCtx))
	if rec.Code != http.StatusOK {
		t.Fatalf("project tail => %d %s", rec.Code, rec.Body.String())
	}
	got := tailResources(t, rec.Body.String())
	if len(got[projectWeb]) != 2 || len(got[postgresID]) != 1 {
		t.Fatalf("tail = %v, want both web lines and the Postgres line", got)
	}
	if len(got[projectCron]) != 1 {
		t.Fatalf("tail = %v: the cron job that joined the project never streamed", got)
	}
	if _, leaked := got["elsewhere"]; leaked {
		t.Fatal("a service outside the project streamed")
	}
}

// A tail refuses before any streaming header what a read refuses: a scope
// over the cap (named), an invalid document, both a resource and a project.
func TestProjectTailRefusesBeforeStreaming(t *testing.T) {
	svc, project := projectFixture()
	var many []savedsearch.Resource
	for i := range maxProjectLogResources + 1 {
		many = append(many, savedsearch.Resource{ID: fmt.Sprintf("srv-%02d", i), Name: "s", Type: "web_service"})
	}
	project.set(many...)
	rec := subscribeProject(t, svc, url.Values{"project": {"prj-observed"}}, time.Second)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "one project log query reads at most 20") {
		t.Fatalf("over the cap => %d %s", rec.Code, rec.Body.String())
	}
	rec = subscribeProject(t, svc, url.Values{"project": {"prj-observed"}, "filterDocument": {`{"filters":[],"version":3}`}}, time.Second)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "version 3") {
		t.Fatalf("bad document => %d %s", rec.Code, rec.Body.String())
	}
	rec = subscribeProject(t, svc, url.Values{"project": {"prj-observed"}, "resource": {"web"}}, time.Second)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("resource and project => %d %s", rec.Code, rec.Body.String())
	}
	project.err = core.ErrForbidden
	rec = subscribeProject(t, svc, url.Values{"project": {"prj-observed"}}, time.Second)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("a project the caller may not view => %d %s", rec.Code, rec.Body.String())
	}
	if rec := serveREST(svc, http.MethodGet, "/v1/logs?project=prj-observed"); rec.Code != http.StatusBadRequest {
		t.Fatalf("REST /v1/logs?project => %d; the history read takes resources", rec.Code)
	}
}

// A follower whose resource is gone ends quietly; the tail keeps the others.
func TestProjectTailOutlivesAResourceThatIsGone(t *testing.T) {
	svc, project := projectFixture()
	svc.ProjectRescopeInterval = time.Hour
	v, err := newProjectView("prj-observed", nil, &LogQuery{})
	if err != nil {
		t.Fatal(err)
	}
	q := LogQuery{Types: []string{LogTypeApp}, projectScope: true}
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	var mu sync.Mutex
	var got []string
	err = svc.followProject(ctx, v, q, append(project.resources[:1], savedsearch.Resource{ID: "srv-gone"}), func(e LogEntry) error {
		mu.Lock()
		defer mu.Unlock()
		got = append(got, e.Message)
		return nil
	}, nil)
	if err != nil && !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("followProject = %v", err)
	}
	if strings.Join(got, "|") != "web first|web second" {
		t.Fatalf("tail = %v", got)
	}
}

// A page reads its resources at once rather than one after another, so a
// project view waits for its slowest resource, not the sum of them: each
// store read here returns only once both are in flight.
func TestPageReadsItsResourcesConcurrently(t *testing.T) {
	svc, _ := projectFixture()
	var mu sync.Mutex
	inFlight := 0
	both := make(chan struct{})
	svc.History = func(ctx context.Context, _ string, q LogQuery) ([]LogEntry, error) {
		mu.Lock()
		inFlight++
		if inFlight == 2 {
			close(both)
		}
		mu.Unlock()
		select {
		case <-both:
			return []LogEntry{{Timestamp: "2026-07-05T00:00:01Z", Message: q.App}}, nil
		case <-time.After(2 * time.Second):
			return nil, errors.New("the reads ran one at a time")
		}
	}
	page, err := svc.queryLogPage(context.Background(), []string{projectWeb, projectCron}, LogQuery{})
	if err != nil || len(page.Entries) != 2 {
		t.Fatalf("page = %+v, %v", page.Entries, err)
	}
}
