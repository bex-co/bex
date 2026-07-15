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
	"testing"
	"time"

	"github.com/graphql-go/graphql"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func cronWithRuns(name string) *appv1alpha1.App {
	return &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
		Spec: appv1alpha1.AppSpec{
			Type: appv1alpha1.TypeCronJob, Image: "busybox", Schedule: "*/5 * * * *",
		},
		Status: appv1alpha1.AppStatus{Runs: []appv1alpha1.CronRun{
			{Name: name + "-run-live", StartedAt: "2026-07-14T12:03:00Z", Status: appv1alpha1.CronRunRunning},
			{Name: name + "-run-ok", StartedAt: "2026-07-14T12:02:00Z", FinishedAt: "2026-07-14T12:02:10Z", Status: appv1alpha1.CronRunSucceeded},
			{Name: name + "-run-bad", StartedAt: "2026-07-14T12:01:00Z", FinishedAt: "2026-07-14T12:01:10Z", Status: appv1alpha1.CronRunFailed},
			{Name: name + "-run-canceled", StartedAt: "2026-07-14T12:00:00Z", FinishedAt: "2026-07-14T12:00:10Z", Status: appv1alpha1.CronRunCanceled},
		}},
	}
}

func TestCronRunListGetPaginationAndStableIDs(t *testing.T) {
	svc, _ := newService(nil, cronWithRuns("nightly"))
	first, err := svc.ListCronRuns(context.Background(), "nightly", "", 2)
	if err != nil || len(first) != 2 {
		t.Fatalf("first page: len=%d err=%v", len(first), err)
	}
	if first[0].Status != "pending" || first[1].Status != "successful" {
		t.Fatalf("Render statuses = %q/%q", first[0].Status, first[1].Status)
	}
	if kind, ok := ids.KindOf(first[0].ID); !ok || kind != ids.CronRun {
		t.Fatalf("run id %q is not a registered crr- id", first[0].ID)
	}
	second, err := svc.ListCronRuns(context.Background(), "nightly", first[1].ID, 2)
	if err != nil || len(second) != 2 || second[0].Status != "unsuccessful" || second[1].Status != "canceled" {
		t.Fatalf("second page = %+v err=%v", second, err)
	}
	got, err := svc.GetCronRun(context.Background(), "nightly", first[0].ID)
	if err != nil || got != first[0] {
		t.Fatalf("get = %+v err=%v, want %+v", got, err, first[0])
	}
	again, _ := svc.ListCronRuns(context.Background(), "nightly", "", 1)
	if again[0].ID != first[0].ID {
		t.Fatalf("derived id changed across reads: %q != %q", again[0].ID, first[0].ID)
	}
	unknown, err := svc.ListCronRuns(context.Background(), "nightly", "crr-00000000000000000000", 2)
	if err != nil || len(unknown) != 0 {
		t.Fatalf("unknown cursor = %+v err=%v, want empty page", unknown, err)
	}
}

func TestCronRunReadFailuresAndLastSuccessfulRun(t *testing.T) {
	svc, _ := newService(nil, cronWithRuns("nightly"), sampleApp("web"))
	if _, err := svc.ListCronRuns(context.Background(), "web", "", 20); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("non-cron list = %v, want not found", err)
	}
	if _, err := svc.GetCronRun(context.Background(), "nightly", "crr-00000000000000000000"); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("unknown run = %v, want not found", err)
	}
	view, err := svc.Get(context.Background(), "nightly")
	if err != nil || view.LastSuccessfulRunAt != "2026-07-14T12:02:10Z" {
		t.Fatalf("lastSuccessfulRunAt = %q err=%v", view.LastSuccessfulRunAt, err)
	}
	rendered := toRenderService(view)
	if got := rendered.ServiceDetails["lastSuccessfulRunAt"]; got != "2026-07-14T12:02:10Z" {
		t.Fatalf("cronJobDetails.lastSuccessfulRunAt = %v", got)
	}
}

func TestCancelCronRunRecordsIntentAndRejectsTerminal(t *testing.T) {
	fixed := time.Date(2026, 7, 14, 12, 4, 5, 123, time.UTC)
	svc, cl := newService(nil, cronWithRuns("nightly"))
	svc.Clock = func() time.Time { return fixed }
	runs, _ := svc.ListCronRuns(context.Background(), "nightly", "", 20)
	canceled, err := svc.CancelCronRun(context.Background(), "nightly", runs[0].ID)
	if err != nil {
		t.Fatalf("cancel pending: %v", err)
	}
	if canceled.Status != "canceled" || canceled.FinishedAt != fixed.Format(time.RFC3339Nano) {
		t.Fatalf("cancel response = %+v", canceled)
	}
	intent := getApp(t, cl, "nightly").Spec.CancelRun
	if intent == nil || intent.Name != "nightly-run-live" || intent.RequestedAt != fixed.Format(time.RFC3339Nano) {
		t.Fatalf("cancel intent = %+v", intent)
	}
	if _, err := svc.CancelCronRun(context.Background(), "nightly", runs[1].ID); !errors.Is(err, core.ErrConflict) {
		t.Fatalf("cancel successful run = %v, want conflict", err)
	}
}

func TestTriggerCronRunReturnsPendingRunAndCancelsActive(t *testing.T) {
	fixed := time.Date(2026, 7, 14, 12, 5, 0, 0, time.UTC)
	svc, cl := newService(nil, cronWithRuns("nightly"))
	svc.Clock = func() time.Time { return fixed }
	run, err := svc.TriggerCronRun(context.Background(), "nightly")
	if err != nil {
		t.Fatalf("trigger: %v", err)
	}
	wantName := appv1alpha1.ManualCronRunJobName("nightly", fixed.Format(time.RFC3339Nano))
	if run.Name != wantName || run.ID != ids.Derive(ids.CronRun, wantName) || run.Status != "pending" {
		t.Fatalf("trigger run = %+v", run)
	}
	app := getApp(t, cl, "nightly")
	if app.Spec.RunAt != fixed.Format(time.RFC3339Nano) || app.Spec.CancelRun == nil || app.Spec.CancelRun.Name != "nightly-run-live" {
		t.Fatalf("trigger intent = runAt %q cancel %+v", app.Spec.RunAt, app.Spec.CancelRun)
	}
}

func TestCronRunRESTEnvelopeGetAndCancel(t *testing.T) {
	svc, _ := newService(nil, cronWithRuns("nightly"))
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	list := httptest.NewRecorder()
	mux.ServeHTTP(list, httptest.NewRequest(http.MethodGet, "/v1/cron-jobs/nightly/runs?limit=2", nil))
	if list.Code != http.StatusOK {
		t.Fatalf("list status=%d body=%s", list.Code, list.Body.String())
	}
	var page []cronJobRunWithCursor
	if err := json.Unmarshal(list.Body.Bytes(), &page); err != nil || len(page) != 2 {
		t.Fatalf("list decode=%v page=%+v", err, page)
	}
	if page[0].Cursor != page[0].CronJobRun.ID || page[0].CronJobRun.Status != "pending" {
		t.Fatalf("list envelope = %+v", page[0])
	}

	get := httptest.NewRecorder()
	mux.ServeHTTP(get, httptest.NewRequest(http.MethodGet, "/v1/cron-jobs/nightly/runs/"+page[0].Cursor, nil))
	if get.Code != http.StatusOK {
		t.Fatalf("get status=%d body=%s", get.Code, get.Body.String())
	}
	var one renderCronJobRun
	_ = json.Unmarshal(get.Body.Bytes(), &one)
	if one != page[0].CronJobRun {
		t.Fatalf("get=%+v list=%+v", one, page[0].CronJobRun)
	}

	cancel := httptest.NewRecorder()
	mux.ServeHTTP(cancel, httptest.NewRequest(http.MethodPost, "/v1/cron-jobs/nightly/runs/"+page[0].Cursor+"/cancel", nil))
	if cancel.Code != http.StatusOK {
		t.Fatalf("cancel status=%d body=%s", cancel.Code, cancel.Body.String())
	}
	_ = json.Unmarshal(cancel.Body.Bytes(), &one)
	if one.Status != "canceled" {
		t.Fatalf("cancel run = %+v", one)
	}

	terminal := httptest.NewRecorder()
	mux.ServeHTTP(terminal, httptest.NewRequest(http.MethodPost, "/v1/cron-jobs/nightly/runs/"+page[1].Cursor+"/cancel", nil))
	if terminal.Code != http.StatusConflict {
		t.Fatalf("terminal cancel status=%d body=%s", terminal.Code, terminal.Body.String())
	}

	// Render's current route cancels the active run implicitly and returns no body.
	svc, _ = newService(nil, cronWithRuns("nightly"))
	mux = http.NewServeMux()
	svc.RegisterREST(mux)
	cancelCurrent := httptest.NewRecorder()
	mux.ServeHTTP(cancelCurrent, httptest.NewRequest(http.MethodDelete, "/v1/cron-jobs/nightly/runs", nil))
	if cancelCurrent.Code != http.StatusNoContent || cancelCurrent.Body.Len() != 0 {
		t.Fatalf("current cancel status=%d body=%q", cancelCurrent.Code, cancelCurrent.Body.String())
	}
}

func TestCronRunGraphQLAndMCPAdapters(t *testing.T) {
	// GraphQL list/get/cancel all return the same CronRun fields.
	svc, _ := newService(nil, cronWithRuns("nightly"))
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
		Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: svc.GraphQLMutation()}),
	})
	if err != nil {
		t.Fatal(err)
	}
	res := graphql.Do(graphql.Params{Schema: schema, Context: context.Background(), RequestString: `{ cronJobRuns(serviceId:"nightly", limit:2) { id status startedAt finishedAt } }`})
	if len(res.Errors) > 0 {
		t.Fatalf("graphql list: %v", res.Errors)
	}
	gqlRuns := res.Data.(map[string]any)["cronJobRuns"].([]any)
	first := gqlRuns[0].(map[string]any)
	if first["status"] != "pending" {
		t.Fatalf("graphql run = %+v", first)
	}
	res = graphql.Do(graphql.Params{Schema: schema, Context: context.Background(), RequestString: `{ cronJobRun(serviceId:"nightly", runId:"` + first["id"].(string) + `") { id status } }`})
	if len(res.Errors) > 0 || res.Data.(map[string]any)["cronJobRun"].(map[string]any)["id"] != first["id"] {
		t.Fatalf("graphql get: data=%v errors=%v", res.Data, res.Errors)
	}
	res = graphql.Do(graphql.Params{Schema: schema, Context: context.Background(), RequestString: `mutation { cancelCronJobRun(serviceId:"nightly", runId:"` + first["id"].(string) + `") { id status } }`})
	if len(res.Errors) > 0 || res.Data.(map[string]any)["cancelCronJobRun"].(map[string]any)["status"] != "canceled" {
		t.Fatalf("graphql cancel: data=%v errors=%v", res.Data, res.Errors)
	}

	// MCP uses the same verbs and wire field names.
	svc, _ = newService(nil, cronWithRuns("nightly"))
	call, cleanup := appsMCPClient(t, svc)
	defer cleanup()
	mcpList := call("list_cron_job_runs", map[string]any{"serviceId": "nightly", "limit": 2})
	mcpRuns := mcpList["cronJobRuns"].([]any)
	mcpFirst := mcpRuns[0].(map[string]any)
	if mcpFirst["id"] != first["id"] || mcpFirst["status"] != first["status"] {
		t.Fatalf("MCP run=%v GraphQL run=%v", mcpFirst, first)
	}
	mcpGet := call("get_cron_job_run", map[string]any{"serviceId": "nightly", "runId": mcpFirst["id"]})
	if mcpGet["id"] != mcpFirst["id"] || mcpGet["status"] != "pending" {
		t.Fatalf("MCP get = %v", mcpGet)
	}
	mcpCancel := call("cancel_cron_job_run", map[string]any{"serviceId": "nightly", "runId": mcpFirst["id"]})
	if mcpCancel["status"] != "canceled" {
		t.Fatalf("MCP cancel = %v", mcpCancel)
	}
}
