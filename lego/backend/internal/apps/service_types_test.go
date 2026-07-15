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
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/graphql-go/graphql"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// --- Create accepts `type` and `schedule` ---

func TestCreateBackgroundWorker(t *testing.T) {
	svc, cl := newService(nil)
	v, err := svc.Create(context.Background(), CreateRequest{
		Name: "worker", Type: appv1alpha1.TypeBackgroundWorker, Image: "job:v1",
	})
	if err != nil {
		t.Fatalf("Create worker: %v", err)
	}
	if v.Type != appv1alpha1.TypeBackgroundWorker {
		t.Errorf("view type = %q, want background_worker", v.Type)
	}
	a := getApp(t, cl, "worker")
	if a.Spec.Type != appv1alpha1.TypeBackgroundWorker {
		t.Errorf("spec.type = %q, want background_worker", a.Spec.Type)
	}
	if a.Spec.Expose {
		t.Error("a background_worker must not be exposed")
	}
}

func TestCreateCronJob(t *testing.T) {
	svc, cl := newService(nil)
	v, err := svc.Create(context.Background(), CreateRequest{
		Name: "nightly", Type: appv1alpha1.TypeCronJob, Image: "job:v1",
		Schedule: "0 2 * * *", Command: "npm run report",
	})
	if err != nil {
		t.Fatalf("Create cron: %v", err)
	}
	if v.Type != appv1alpha1.TypeCronJob || v.Schedule != "0 2 * * *" || v.Command != "npm run report" {
		t.Errorf("view = %+v, want type=cron_job schedule=0 2 * * * command=npm run report", v)
	}
	a := getApp(t, cl, "nightly")
	if a.Spec.Type != appv1alpha1.TypeCronJob || a.Spec.Schedule != "0 2 * * *" || a.Spec.Command != "npm run report" {
		t.Errorf("spec = %+v", a.Spec)
	}
	if a.Spec.Expose {
		t.Error("a cron_job must not be exposed")
	}
}

func TestCreateCronJobRequiresSchedule(t *testing.T) {
	svc, _ := newService(nil)
	_, err := svc.Create(context.Background(), CreateRequest{
		Name: "nightly", Type: appv1alpha1.TypeCronJob, Image: "job:v1",
	})
	if !errors.Is(err, core.ErrBadRequest) {
		t.Errorf("cron without schedule => ErrBadRequest, got %v", err)
	}
}

func TestCreateRejectsUnknownType(t *testing.T) {
	svc, _ := newService(nil)
	_, err := svc.Create(context.Background(), CreateRequest{
		Name: "svc", Type: "static_site", Image: "x:v1",
	})
	if !errors.Is(err, core.ErrBadRequest) {
		t.Errorf("unknown type => ErrBadRequest, got %v", err)
	}
}

func TestCreateWorkerRejectsDomains(t *testing.T) {
	svc, _ := newService(nil)
	_, err := svc.Create(context.Background(), CreateRequest{
		Name: "worker", Type: appv1alpha1.TypeBackgroundWorker, Image: "x:v1",
		Hosts: []string{"a.example.com"},
	})
	if !errors.Is(err, core.ErrBadRequest) {
		t.Errorf("worker+domains => ErrBadRequest, got %v", err)
	}
}

// --- Cron run trigger ---

func cronApp(name string) *appv1alpha1.App {
	return &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
		Spec: appv1alpha1.AppSpec{
			Type: appv1alpha1.TypeCronJob, Image: name + ":v1", Schedule: "*/5 * * * *", Command: "npm run report",
		},
		Status: appv1alpha1.AppStatus{
			Phase: appv1alpha1.PhaseRunning,
			Runs: []appv1alpha1.CronRun{
				{Name: name + "-run-abc", StartedAt: "2026-07-09T10:00:00Z", FinishedAt: "2026-07-09T10:00:05Z", Status: "Succeeded"},
			},
		},
	}
}

func TestTriggerCronRunBumpsRunAt(t *testing.T) {
	svc, cl := newService(nil, cronApp("nightly"))
	if _, err := svc.TriggerCronRun(context.Background(), "nightly"); err != nil {
		t.Fatalf("TriggerCronRun: %v", err)
	}
	if ra := getApp(t, cl, "nightly").Spec.RunAt; ra == "" {
		t.Error("TriggerCronRun must set spec.runAt")
	}
}

func TestTriggerCronRunRejectsNonCron(t *testing.T) {
	svc, _ := newService(nil, sampleApp("web"))
	_, err := svc.TriggerCronRun(context.Background(), "web")
	if !errors.Is(err, core.ErrBadRequest) {
		t.Errorf("run on non-cron => ErrBadRequest, got %v", err)
	}
}

// --- Render projection carries type / schedule / runs ---

func TestRenderServiceCarriesTypeScheduleRuns(t *testing.T) {
	svc, _ := newService(nil, cronApp("nightly"))
	v, err := svc.Get(context.Background(), "nightly")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	rs := toRenderService(v)
	if rs.Type != appv1alpha1.TypeCronJob {
		t.Errorf("render type = %q, want cron_job", rs.Type)
	}
	if rs.Schedule != "*/5 * * * *" {
		t.Errorf("render schedule = %q", rs.Schedule)
	}
	if rs.Command != "npm run report" {
		t.Errorf("render command = %q", rs.Command)
	}
	if len(rs.Runs) != 1 || rs.Runs[0].Status != "successful" {
		t.Errorf("render runs = %+v", rs.Runs)
	}
	if got, _ := rs.ServiceDetails["schedule"].(string); got != "*/5 * * * *" {
		t.Errorf("serviceDetails.schedule = %q (cronJobDetails.schedule)", got)
	}
	if got, _ := rs.ServiceDetails["command"].(string); got != "npm run report" {
		t.Errorf("serviceDetails.command = %q (cronJobDetails.command)", got)
	}
}

func TestRenderServiceDefaultsWebType(t *testing.T) {
	svc, _ := newService(nil, sampleApp("web")) // no spec.type set
	v, _ := svc.Get(context.Background(), "web")
	if rs := toRenderService(v); rs.Type != renderWebService {
		t.Errorf("untyped App => %q, want web_service", rs.Type)
	}
}

// --- Adapter mapping: REST body, MCP tools ---

func TestRESTCreateCronJob(t *testing.T) {
	svc, cl := newService(nil)
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	body := `{"name":"nightly","type":"cron_job","schedule":"0 * * * *","command":"npm run report","image":{"imagePath":"job:v1"}}`
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("POST", "/v1/services", strings.NewReader(body)))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create cron => 201, got %d: %s", rec.Code, rec.Body)
	}
	a := getApp(t, cl, "nightly")
	if a.Spec.Type != appv1alpha1.TypeCronJob || a.Spec.Schedule != "0 * * * *" || a.Spec.Command != "npm run report" {
		t.Errorf("spec = %+v", a.Spec)
	}
}

func TestRESTCronRunTrigger(t *testing.T) {
	svc, cl := newService(nil, cronApp("nightly"))
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("POST", "/v1/cron-jobs/nightly/runs", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("cron run => 200, got %d: %s", rec.Code, rec.Body)
	}
	if getApp(t, cl, "nightly").Spec.RunAt == "" {
		t.Error("run trigger must set spec.runAt")
	}
	// The same verb is reachable under the /v1/services noun too.
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("POST", "/v1/services/nightly/runs", nil))
	if rec.Code != http.StatusOK {
		t.Errorf("cron run (services noun) => 200, got %d", rec.Code)
	}
}

func TestMCPCreateCronJobArgsMap(t *testing.T) {
	req := createCronJobArgs{Name: "nightly", Schedule: "0 0 * * *", Command: "npm run report", Image: "job:v1"}.toCreateRequest()
	if req.Type != appv1alpha1.TypeCronJob || req.Schedule != "0 0 * * *" || req.Command != "npm run report" {
		t.Errorf("create_cron_job maps to %+v", req)
	}
}

func TestMCPCreateWebServiceThreadsType(t *testing.T) {
	req := createWebServiceArgs{Name: "w", Type: appv1alpha1.TypeBackgroundWorker, Image: "x:v1"}.toCreateRequest()
	if req.Type != appv1alpha1.TypeBackgroundWorker {
		t.Errorf("create_web_service type not threaded: %+v", req)
	}
}

// --- Deploy manifest maps short types ---

func TestDeployManifestCronType(t *testing.T) {
	svc, cl := newService(nil)
	manifest := "apps:\n  - name: nightly\n    image: job:v1\n    type: cron\n    schedule: \"0 3 * * *\"\n"
	if _, err := svc.Deploy(context.Background(), DeployRequest{Manifest: manifest}); err != nil {
		t.Fatalf("Deploy cron: %v", err)
	}
	a := getApp(t, cl, "nightly")
	if a.Spec.Type != appv1alpha1.TypeCronJob || a.Spec.Schedule != "0 3 * * *" {
		t.Errorf("spec = %+v", a.Spec)
	}
}

func TestGraphQLCreateServiceTypeAndCronFields(t *testing.T) {
	svc, cl := newService(nil, cronApp("nightly"))
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
		Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: svc.GraphQLMutation()}),
	})
	if err != nil {
		t.Fatalf("schema: %v", err)
	}

	// createService threads type onto the spec.
	res := graphql.Do(graphql.Params{Schema: schema, Context: context.Background(),
		RequestString: `mutation { createService(name: "worker", type: "background_worker", image: "x:v1") { type } }`})
	if len(res.Errors) > 0 {
		t.Fatalf("createService: %v", res.Errors)
	}
	if getApp(t, cl, "worker").Spec.Type != appv1alpha1.TypeBackgroundWorker {
		t.Error("createService did not thread type onto the spec")
	}

	// The Service type exposes schedule + command + runs for a cron.
	res = graphql.Do(graphql.Params{Schema: schema, Context: context.Background(),
		RequestString: `{ server(id: "nightly") { type schedule command runs { name status } } }`})
	if len(res.Errors) > 0 {
		t.Fatalf("server query: %v", res.Errors)
	}
	srv := res.Data.(map[string]any)["server"].(map[string]any)
	if srv["type"] != appv1alpha1.TypeCronJob || srv["schedule"] != "*/5 * * * *" || srv["command"] != "npm run report" {
		t.Errorf("server = %+v", srv)
	}
	runs := srv["runs"].([]any)
	if len(runs) != 1 || runs[0].(map[string]any)["status"] != "successful" {
		t.Errorf("runs = %+v", runs)
	}

	// runCronJob triggers a run (bumps spec.runAt).
	res = graphql.Do(graphql.Params{Schema: schema, Context: context.Background(),
		RequestString: `mutation { runCronJob(id: "nightly") { id } }`})
	if len(res.Errors) > 0 {
		t.Fatalf("runCronJob: %v", res.Errors)
	}
	if getApp(t, cl, "nightly").Spec.RunAt == "" {
		t.Error("runCronJob must set spec.runAt")
	}
}
