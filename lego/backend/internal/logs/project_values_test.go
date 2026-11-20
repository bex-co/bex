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
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/graphql-go/graphql"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/savedsearch"
)

// The project-scoped label-values read (w3/079): Render's logValues(input:
// {label, logQuery: {project, …}}), which feeds the Edit view's pickers.

// storeLabels is a fake store's label values, by the stream a read selects
// (an App, a Postgres or a Key Value name) and the label; it records the
// streams asked.
type storeLabels struct {
	mu     sync.Mutex
	values map[string]map[string][]string
	asked  []string
}

func (f *storeLabels) source(_ context.Context, _ string, label string, q LogQuery) ([]string, error) {
	stream := q.App + q.Database + q.KeyValue
	f.mu.Lock()
	defer f.mu.Unlock()
	f.asked = append(f.asked, stream+"/"+label)
	return f.values[stream][lokiLabelFor(label)], nil
}

// projectValuesFixture is projectFixture with a store that knows two pods of
// the web service, one of the cron job, the Postgres primary, and a pod of a
// service outside the project.
func projectValuesFixture() (*Service, *fakeProject, *storeLabels) {
	svc, project := projectFixture()
	store := &storeLabels{values: map[string]map[string][]string{
		projectWeb:  {"pod": {"web-1", "web-0"}, "method": {"GET", "POST"}, "level": {"info"}},
		projectCron: {"pod": {"nightly-1"}, "method": {}, "level": {"error"}},
		postgresID:  {"pod": {postgresID + "-1"}, "level": {"LOG"}},
		"elsewhere": {"pod": {"elsewhere-1"}, "method": {"DELETE"}},
	}}
	svc.LabelValues = store.source
	return svc, project, store
}

// runProjectValues reads logLabelValues over the project from the fixture
// day.
func runProjectValues(t *testing.T, svc *Service, ctx context.Context, args string) ([]string, []error) {
	t.Helper()
	schema, err := gqlSchema(svc)
	if err != nil {
		t.Fatal(err)
	}
	res := graphql.Do(graphql.Params{Schema: schema, Context: ctx, RequestString: `{ logLabelValues(` + args + `, startTime:"` + fixtureDay + `") }`})
	var errs []error
	for _, e := range res.Errors {
		errs = append(errs, e)
	}
	data, _ := res.Data.(map[string]any)
	list, _ := data["logLabelValues"].([]any)
	var out []string
	for _, v := range list {
		out = append(out, v.(string))
	}
	return out, errs
}

// instancesOf is each pod as the instance id its resource's logs name it by.
func instancesOf(resource string, pods ...string) []string {
	out := make([]string, 0, len(pods))
	for _, pod := range pods {
		out = append(out, projectLogInstance(resource, pod))
	}
	return out
}

// The DoD's core: one read lists the instances every resource of the view
// produced (the web service's two, the cron job's and the Postgres's), each
// carrying its resource's id, and none of a service outside the project.
func TestProjectInstanceValuesListEveryResourcesInstances(t *testing.T) {
	svc, _, store := projectValuesFixture()
	got, errs := runProjectValues(t, svc, context.Background(), `project:"prj-observed", label:"instance", type:"app"`)
	if errs != nil {
		t.Fatal(errs)
	}
	want := slices.Concat(instancesOf(projectWeb, "web-0", "web-1"), instancesOf(projectCron, "nightly-1"), instancesOf(postgresID, postgresID+"-1"))
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Fatalf("instances = %v, want %v", got, want)
	}
	for _, v := range got {
		if !strings.HasPrefix(v, projectWeb+"-") && !strings.HasPrefix(v, projectCron+"-") && !strings.HasPrefix(v, postgresID+"-") {
			t.Errorf("instance %q names no resource of the view", v)
		}
	}
	for _, asked := range store.asked {
		if strings.HasPrefix(asked, "elsewhere/") {
			t.Fatalf("a service outside the project was read: %v", store.asked)
		}
	}
}

// The view's own document narrows the read as it narrows the view's lines,
// and Request Logs reads only the web services.
func TestProjectValuesFollowTheViewsScope(t *testing.T) {
	svc, _, _ := projectValuesFixture()
	for _, tc := range []struct {
		args string
		want []string
	}{
		{`project:"prj-observed", label:"instance", filterDocument:{filters: [{field: "ENVIRONMENT", operator: "IS", values: ["evm-c185th5c2rvvnhbfiltg"]}], version: 1}`,
			slices.Concat(instancesOf(projectWeb, "web-0", "web-1"), instancesOf(postgresID, postgresID+"-1"))},
		{`project:"prj-observed", label:"instance", filterDocument:{filters: [{field: "RESOURCE_TYPE", operator: "IS_NOT", values: ["SERVICE", "POSTGRES"]}], version: 1}`,
			instancesOf(projectCron, "nightly-1")},
		{`project:"prj-observed", label:"instance", type:"request"`, instancesOf(projectWeb, "web-0", "web-1")},
	} {
		got, errs := runProjectValues(t, svc, context.Background(), tc.args)
		if errs != nil {
			t.Fatalf("%s: %v", tc.args, errs)
		}
		slices.Sort(tc.want)
		if !slices.Equal(got, tc.want) {
			t.Errorf("%s: values = %v, want %v", tc.args, got, tc.want)
		}
	}
}

// Every label the per-resource read discovers reads the same way across the
// project (w3/084 reuses the read): a Postgres, whose lines carry only an
// instance and a level, adds nothing to a service label rather than refusing
// the whole read.
func TestProjectValuesReadEveryDiscoverableLabel(t *testing.T) {
	svc, _, _ := projectValuesFixture()
	for label, want := range map[string][]string{
		LabelMethod: {"GET", "POST"},
		LabelLevel:  {"LOG", "error", "info"},
	} {
		got, errs := runProjectValues(t, svc, context.Background(), `project:"prj-observed", label:"`+label+`"`)
		if errs != nil {
			t.Fatalf("%s: %v", label, errs)
		}
		if !slices.Equal(got, want) {
			t.Errorf("%s = %v, want %v", label, got, want)
		}
	}
}

// Authorization: a caller without log access reads no value and learns of no
// resource, and a resource that is gone by the read is dropped quietly.
func TestProjectValuesNameNothingTheCallerCannotRead(t *testing.T) {
	svc, project, store := projectValuesFixture()
	project.set(append(project.resources, savedsearch.Resource{ID: "srv-gone", Name: "deleted", Type: "web_service"})...)
	got, errs := runProjectValues(t, svc, context.Background(), `project:"prj-observed", label:"instance"`)
	if errs != nil {
		t.Fatalf("a resource that is gone failed the read: %v", errs)
	}
	if len(got) != 4 {
		t.Fatalf("instances = %v, want the four of the resources still there", got)
	}

	svc.Base.Workspace = homeWorkspaceOnly{}
	svc.Base.Authz = relationChecker{deny: core.RelCanViewLogs}
	store.asked = nil
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "viewer", Method: "session"})
	got, errs = runProjectValues(t, svc, ctx, `project:"prj-observed", label:"instance"`)
	if errs != nil {
		t.Fatal(errs)
	}
	if len(got) != 0 || len(store.asked) != 0 {
		t.Fatalf("a caller without log access read %v (store asked %v)", got, store.asked)
	}

	project.err = core.ErrForbidden
	if _, errs := runProjectValues(t, svc, ctx, `project:"prj-observed", label:"instance"`); len(errs) != 1 || !strings.Contains(errs[0].Error(), "forbidden") {
		t.Fatalf("a project the caller may not view: %v", errs)
	}
}

// What the read refuses, each by name, as the project log read does: a label
// that is not discoverable, a scope past the cap, both a resource and a
// project, and a document not valid for its view. A Hobby workspace's
// Request Logs is refused with the plan error.
func TestProjectValuesRefusals(t *testing.T) {
	svc, project, _ := projectValuesFixture()
	for _, tc := range []struct {
		args string
		want string
	}{
		{`project:"prj-observed", label:"pod"`, `unknown log label "pod"`},
		{`project:"prj-observed", resource:"web", label:"instance"`, "logLabelValues takes exactly one of resource and project"},
		{`label:"instance"`, "logLabelValues takes exactly one of resource and project"},
		{`resource:"web", label:"instance", filterDocument:{filters: [], version: 1}`, "filterDocument applies to a project read"},
		{`project:"prj-observed", label:"instance", filterDocument:{filters: [], version: 2}`, "version 2 is not supported"},
		{`project:"prj-observed", label:"instance", type:"build"`, "a project log view reads type=app or type=request"},
		{`project:"prj-missing", label:"instance"`, "not found"},
	} {
		_, errs := runProjectValues(t, svc, context.Background(), tc.args)
		if len(errs) != 1 || !strings.Contains(errs[0].Error(), tc.want) {
			t.Errorf("%s: errors %v, want one naming %q", tc.args, errs, tc.want)
		}
	}

	svc.Plans = core.WorkspacePlanFunc(func(context.Context, string) (string, error) { return "hobby", nil })
	if _, errs := runProjectValues(t, svc, context.Background(), `project:"prj-observed", label:"statusCode", type:"request"`); len(errs) != 1 || !strings.Contains(errs[0].Error(), "HTTP request logs require the Pro plan") {
		t.Fatalf("hobby Request Logs = %v, want the plan refusal", errs)
	}
	svc.Plans = nil

	var many []savedsearch.Resource
	for i := range maxProjectLogResources + 1 {
		many = append(many, savedsearch.Resource{ID: fmt.Sprintf("srv-%02d", i), Name: "s", Type: "web_service"})
	}
	project.set(many...)
	if _, errs := runProjectValues(t, svc, context.Background(), `project:"prj-observed", label:"instance"`); len(errs) != 1 || !strings.Contains(errs[0].Error(), "this view reads 21 resources, and one project log query reads at most 20") {
		t.Fatalf("over the cap: %v", errs)
	}
}

// Without the store an App's instances cannot be discovered: the read says so
// rather than answering a partial list.
func TestProjectValuesNeedTheStore(t *testing.T) {
	svc, _, _ := projectValuesFixture()
	svc.LabelValues = nil
	if _, errs := runProjectValues(t, svc, context.Background(), `project:"prj-observed", label:"instance"`); len(errs) != 1 || !strings.Contains(errs[0].Error(), core.ErrLogStoreUnavailable.Error()) {
		t.Fatalf("no store: %v", errs)
	}
}
