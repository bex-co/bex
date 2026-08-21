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
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/graphql-go/graphql"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// cron_settings_test.go covers the SetCronJob write path (w5/m18): schedule +
// command editable after create, consistent across REST PATCH, GraphQL
// updateCronJob, and MCP update_cron_job. Mirrors the rootdir_test.go depth.

func sp(s string) *string { return &s }

// --- SetCronJob service verb ---

func TestSetCronJobUpdatesScheduleAndCommand(t *testing.T) {
	svc, cl := newService(nil, cronApp("nightly"))

	v, err := svc.SetCronJob(context.Background(), "nightly", sp("0 6 * * *"), sp("python weekly.py"))
	if err != nil {
		t.Fatalf("SetCronJob: %v", err)
	}
	if v.Schedule != "0 6 * * *" || v.Command != "python weekly.py" {
		t.Errorf("view = %+v, want schedule=0 6 * * * command=python weekly.py", v)
	}
	a := getApp(t, cl, "nightly")
	if a.Spec.Schedule != "0 6 * * *" || a.Spec.Command != "python weekly.py" {
		t.Errorf("spec = %+v", a.Spec)
	}
}

func TestSetCronJobClearsCommand(t *testing.T) {
	svc, cl := newService(nil, cronApp("nightly"))

	if _, err := svc.SetCronJob(context.Background(), "nightly", sp("0 6 * * *"), sp("")); err != nil {
		t.Fatalf("SetCronJob with empty command: %v", err)
	}
	if got := getApp(t, cl, "nightly").Spec.Command; got != "" {
		t.Errorf("spec.command = %q, want empty (image default)", got)
	}
}

func TestSetCronJobRejectsEmptySchedule(t *testing.T) {
	svc, _ := newService(nil, cronApp("nightly"))

	if _, err := svc.SetCronJob(context.Background(), "nightly", sp(""), sp("cmd")); !errors.Is(err, core.ErrBadRequest) {
		t.Errorf("empty schedule => ErrBadRequest, got %v", err)
	}
}

func TestSetCronJobRejectsInvalidSchedule(t *testing.T) {
	svc, _ := newService(nil, cronApp("nightly"))

	for _, bad := range []string{"* * *", "not a cron", "* * * * * *", "99 99 * * *", "0 24 * * *", "60 * * * *"} {
		if _, err := svc.SetCronJob(context.Background(), "nightly", sp(bad), sp("")); !errors.Is(err, core.ErrBadRequest) {
			t.Errorf("schedule %q => ErrBadRequest, got %v", bad, err)
		}
	}
}

func TestSetCronJobRejectsNonCronApp(t *testing.T) {
	svc, cl := newService(nil, sampleApp("web"))

	if _, err := svc.SetCronJob(context.Background(), "web", sp("0 * * * *"), sp("")); !errors.Is(err, core.ErrBadRequest) {
		t.Errorf("non-cron => ErrBadRequest, got %v", err)
	}
	// Must not modify the App.
	a := getApp(t, cl, "web")
	if a.Spec.Schedule != "" || a.Spec.Type == appv1alpha1.TypeCronJob {
		t.Error("rejected SetCronJob must not mutate a non-cron App")
	}
}

// --- REST PATCH ---

func TestRESTPatchCronJobScheduleAndCommand(t *testing.T) {
	svc, cl := newService(nil, cronApp("nightly"))
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	body := `{"schedule":"0 8 * * 1","command":"node weekly.js"}`
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("PATCH", "/v1/services/nightly", strings.NewReader(body)))
	if rec.Code != http.StatusOK {
		t.Fatalf("PATCH cron => 200, got %d: %s", rec.Code, rec.Body)
	}
	var out struct {
		Schedule string `json:"schedule"`
		Command  string `json:"command"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil || out.Schedule != "0 8 * * 1" || out.Command != "node weekly.js" {
		t.Errorf("response = %s, want schedule=0 8 * * 1 command=node weekly.js", rec.Body)
	}
	a := getApp(t, cl, "nightly")
	if a.Spec.Schedule != "0 8 * * 1" || a.Spec.Command != "node weekly.js" {
		t.Errorf("spec = %+v", a.Spec)
	}
}

func TestRESTPatchCronJobScheduleOnlyPreservesCommand(t *testing.T) {
	svc, cl := newService(nil, cronApp("nightly")) // starts with command "npm run report"
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("PATCH", "/v1/services/nightly", strings.NewReader(`{"schedule":"0 8 * * 1"}`)))
	if rec.Code != http.StatusOK {
		t.Fatalf("PATCH schedule-only => 200, got %d: %s", rec.Code, rec.Body)
	}
	a := getApp(t, cl, "nightly")
	if a.Spec.Schedule != "0 8 * * 1" {
		t.Errorf("spec.schedule = %q, want 0 8 * * 1", a.Spec.Schedule)
	}
	if a.Spec.Command != "npm run report" {
		t.Errorf("spec.command = %q, want npm run report (preserved)", a.Spec.Command)
	}
}

func TestRESTPatchCronJobRejectsNonCron(t *testing.T) {
	svc, _ := newService(nil, sampleApp("web"))
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("PATCH", "/v1/services/web", strings.NewReader(`{"schedule":"0 * * * *"}`)))
	if rec.Code == http.StatusOK {
		t.Errorf("PATCH schedule on non-cron App must not return 200: %s", rec.Body)
	}
}

func TestRESTPatchCronJobRejectsInvalidSchedule(t *testing.T) {
	svc, _ := newService(nil, cronApp("nightly"))
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("PATCH", "/v1/services/nightly", strings.NewReader(`{"schedule":"not valid"}`)))
	if rec.Code == http.StatusOK {
		t.Errorf("PATCH with invalid schedule must not return 200: %s", rec.Body)
	}
}

// --- GraphQL updateCronJob ---

func TestGraphQLUpdateCronJob(t *testing.T) {
	svc, cl := newService(nil, cronApp("nightly"))
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
		Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: svc.GraphQLMutation()}),
	})
	if err != nil {
		t.Fatalf("schema: %v", err)
	}

	res := graphql.Do(graphql.Params{Schema: schema, Context: context.Background(),
		RequestString: `mutation { updateCronJob(id: "nightly", schedule: "0 6 * * *", command: "node daily.js") { schedule command } }`})
	if len(res.Errors) > 0 {
		t.Fatalf("updateCronJob: %v", res.Errors)
	}
	got := res.Data.(map[string]any)["updateCronJob"].(map[string]any)
	if got["schedule"] != "0 6 * * *" || got["command"] != "node daily.js" {
		t.Errorf("updateCronJob result = %+v", got)
	}
	a := getApp(t, cl, "nightly")
	if a.Spec.Schedule != "0 6 * * *" || a.Spec.Command != "node daily.js" {
		t.Errorf("spec = %+v", a.Spec)
	}
}

func TestGraphQLUpdateCronJobRejectsNonCron(t *testing.T) {
	svc, _ := newService(nil, sampleApp("web"))
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
		Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: svc.GraphQLMutation()}),
	})
	if err != nil {
		t.Fatalf("schema: %v", err)
	}

	res := graphql.Do(graphql.Params{Schema: schema, Context: context.Background(),
		RequestString: `mutation { updateCronJob(id: "web", schedule: "0 * * * *") { id } }`})
	if len(res.Errors) == 0 {
		t.Error("updateCronJob on a non-cron App must return an error")
	}
}

// --- MCP update_service (cron fields; update_cron_job folded in at w1/m74) ---

// w1/m74 folded update_cron_job into update_service, so the cron fields are
// arguments on the patch tool now — and, like every other folded field, an
// omitted one is not a write.
func TestMCPUpdateCronJobArgs(t *testing.T) {
	a := updateServiceArgs{ServiceID: "nightly", Schedule: sp("0 6 * * *"), Command: sp("node daily.js")}
	if a.ServiceID != "nightly" || a.Schedule == nil || *a.Schedule != "0 6 * * *" || a.Command == nil || *a.Command != "node daily.js" {
		t.Errorf("updateServiceArgs cron fields not set: %+v", a)
	}
	if a.Plan != nil || a.PublishPath != nil {
		t.Errorf("unrelated folded fields must default to absent: %+v", a)
	}
}

func TestMCPUpdateCronJobDelegatesToSetCronJob(t *testing.T) {
	svc, cl := newService(nil, cronApp("nightly"))

	v, err := svc.SetCronJob(context.Background(), "nightly", sp("0 6 * * *"), sp("python daily.py"))
	if err != nil {
		t.Fatalf("SetCronJob (update_cron_job delegate): %v", err)
	}
	out := toRenderService(v)
	if out.Schedule != "0 6 * * *" || out.Command != "python daily.py" {
		t.Errorf("update_cron_job (via renderService) schedule=%q command=%q", out.Schedule, out.Command)
	}
	a := getApp(t, cl, "nightly")
	if a.Spec.Schedule != "0 6 * * *" || a.Spec.Command != "python daily.py" {
		t.Errorf("spec = %+v", a.Spec)
	}
}
