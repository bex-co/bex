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

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// fakeCronRunActors is cron_run_actors in memory, with the store's upsert
// semantics: a trigger and a cancel of one run share its row.
type fakeCronRunActors struct {
	rows map[string]store.CronRunActor // "<appID>/<runID>"
	err  error
}

func (f *fakeCronRunActors) RecordCronRunTrigger(_ context.Context, appID, runID, subject string) error {
	if f.err != nil {
		return f.err
	}
	row := f.rows[appID+"/"+runID]
	row.TriggeredBy = subject
	f.rows[appID+"/"+runID] = row
	return nil
}

func (f *fakeCronRunActors) RecordCronRunCancel(_ context.Context, appID, runID, subject string) error {
	if f.err != nil {
		return f.err
	}
	row := f.rows[appID+"/"+runID]
	row.CanceledBy = subject
	f.rows[appID+"/"+runID] = row
	return nil
}

func (f *fakeCronRunActors) CronRunActors(_ context.Context, appID string, runIDs []string) (map[string]store.CronRunActor, error) {
	if f.err != nil {
		return nil, f.err
	}
	out := map[string]store.CronRunActor{}
	for _, id := range runIDs {
		if row, ok := f.rows[appID+"/"+id]; ok {
			out[id] = row
		}
	}
	return out, nil
}

type fakeEmails map[string]string

func (f fakeEmails) LookupEmails(_ context.Context, subjects []string) (map[string]string, error) {
	out := map[string]string{}
	for _, s := range subjects {
		if e, ok := f[s]; ok {
			out[s] = e
		}
	}
	return out, nil
}

func asUser(ctx context.Context, subject string) context.Context {
	return core.WithIdentity(ctx, core.Identity{Subject: subject, Method: "session"})
}

// cronWithActors is cronWithRuns("nightly") (live, ok, bad, canceled — all
// scheduled) wired to an actor store and an email directory. The canceled
// run was triggered by alice and canceled by bob; the successful one carries
// a cancel by bob that lost the race to the run's own finish.
func cronWithActors(t *testing.T) (*Service, *fakeCronRunActors) {
	t.Helper()
	app := cronWithRuns("nightly")
	app.Labels = map[string]string{store.LabelAppID: "srv-nightly"}
	svc, _ := newService(nil, app)
	actors := &fakeCronRunActors{rows: map[string]store.CronRunActor{
		"srv-nightly/" + ids.Derive(ids.CronRun, "nightly-run-canceled"): {TriggeredBy: "id-alice", CanceledBy: "id-bob"},
		"srv-nightly/" + ids.Derive(ids.CronRun, "nightly-run-ok"):       {CanceledBy: "id-bob"},
	}}
	svc.CronRunActors = actors
	svc.ActorEmails = fakeEmails{"id-alice": "alice@example.com", "id-bob": "bob@example.com"}
	svc.Clock = func() time.Time { return time.Date(2026, 7, 14, 12, 5, 0, 0, time.UTC) }
	return svc, actors
}

func TestCronRunTriggerNamesItsCallerAndCancelsThePreemptedRunAsThem(t *testing.T) {
	svc, actors := cronWithActors(t)
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/cron-jobs/nightly/runs", nil)
	validated(t, mux).ServeHTTP(rec, req.WithContext(asUser(req.Context(), "id-alice")))
	if rec.Code != http.StatusOK {
		t.Fatalf("trigger status=%d body=%s", rec.Code, rec.Body.String())
	}
	var run renderCronJobRun
	if err := json.Unmarshal(rec.Body.Bytes(), &run); err != nil {
		t.Fatal(err)
	}
	if run.Status != "pending" || run.TriggeredBy != "alice@example.com" || run.CanceledBy != "" {
		t.Fatalf("trigger response = %+v, want pending and triggeredBy alice", run)
	}
	if got := actors.rows["srv-nightly/"+run.ID]; got.TriggeredBy != "id-alice" {
		t.Errorf("trigger recorded %+v, want id-alice", got)
	}
	// The trigger replaced the live scheduled run: its caller canceled it.
	live := ids.Derive(ids.CronRun, "nightly-run-live")
	if got := actors.rows["srv-nightly/"+live]; got != (store.CronRunActor{CanceledBy: "id-alice"}) {
		t.Errorf("preempted run actors = %+v, want canceled by id-alice only", got)
	}
	// Until the operator records it canceled, the run is still pending and
	// names no canceler, and a scheduled run never names a trigger.
	got, err := svc.GetCronRun(context.Background(), "nightly", live)
	if err != nil || got.Status != cronRunPending || got.TriggeredBy != "" || got.CanceledBy != "" {
		t.Errorf("preempted run before the operator = %+v (%v)", got, err)
	}
}

func TestCronRunReadsNameTheirActorsOnRESTGraphQLAndMCP(t *testing.T) {
	svc, _ := cronWithActors(t)
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	canceled := ids.Derive(ids.CronRun, "nightly-run-canceled")

	list := httptest.NewRecorder()
	validated(t, mux).ServeHTTP(list, httptest.NewRequest(http.MethodGet, "/v1/cron-jobs/nightly/runs", nil))
	var page []cronJobRunWithCursor
	if err := json.Unmarshal(list.Body.Bytes(), &page); list.Code != http.StatusOK || err != nil || len(page) != 4 {
		t.Fatalf("list status=%d err=%v body=%s", list.Code, err, list.Body.String())
	}
	want := map[string]renderCronJobRun{
		canceled: {TriggeredBy: "alice@example.com", CanceledBy: "bob@example.com"},
		// Scheduled, and the cancel recorded against it did not take.
		ids.Derive(ids.CronRun, "nightly-run-ok"): {},
	}
	for _, item := range page {
		w := want[item.CronJobRun.ID]
		if item.CronJobRun.TriggeredBy != w.TriggeredBy || item.CronJobRun.CanceledBy != w.CanceledBy {
			t.Errorf("list run %s = %+v, want triggeredBy %q canceledBy %q", item.CronJobRun.ID, item.CronJobRun, w.TriggeredBy, w.CanceledBy)
		}
	}

	get := httptest.NewRecorder()
	validated(t, mux).ServeHTTP(get, httptest.NewRequest(http.MethodGet, "/v1/cron-jobs/nightly/runs/"+canceled, nil))
	var one renderCronJobRun
	if err := json.Unmarshal(get.Body.Bytes(), &one); err != nil || one.TriggeredBy != "alice@example.com" || one.CanceledBy != "bob@example.com" {
		t.Fatalf("REST get = %+v (%v) body=%s", one, err, get.Body.String())
	}

	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
		Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: svc.GraphQLMutation()}),
	})
	if err != nil {
		t.Fatal(err)
	}
	res := graphql.Do(graphql.Params{Schema: schema, Context: context.Background(), RequestString: `{
		cronJobRun(serviceId:"nightly", runId:"` + canceled + `") { triggeredBy canceledBy }
		cronJobRuns(serviceId:"nightly") { id triggeredBy canceledBy }
		service(id:"nightly") { runs { name triggeredBy canceledBy } }
	}`})
	if len(res.Errors) > 0 {
		t.Fatalf("graphql: %v", res.Errors)
	}
	data := res.Data.(map[string]any)
	if g := data["cronJobRun"].(map[string]any); g["triggeredBy"] != "alice@example.com" || g["canceledBy"] != "bob@example.com" {
		t.Errorf("graphql cronJobRun = %v", g)
	}
	for _, raw := range data["cronJobRuns"].([]any) {
		r := raw.(map[string]any)
		if r["id"] != canceled && (r["triggeredBy"] != nil || r["canceledBy"] != nil) {
			t.Errorf("graphql scheduled run names an actor: %v", r)
		}
	}
	for _, raw := range data["service"].(map[string]any)["runs"].([]any) {
		r := raw.(map[string]any)
		if r["name"] == "nightly-run-canceled" && (r["triggeredBy"] != "alice@example.com" || r["canceledBy"] != "bob@example.com") {
			t.Errorf("graphql service.runs = %v", r)
		}
	}

	call, cleanup := appsMCPClient(t, svc)
	defer cleanup()
	mcpGet := call("get_cron_job_run", map[string]any{"serviceId": "nightly", "runId": canceled})
	if mcpGet["triggeredBy"] != "alice@example.com" || mcpGet["canceledBy"] != "bob@example.com" {
		t.Errorf("MCP get = %v", mcpGet)
	}
}

func TestCronRunCancelNamesTheCanceler(t *testing.T) {
	svc, actors := cronWithActors(t)
	live := ids.Derive(ids.CronRun, "nightly-run-live")
	run, err := svc.CancelCronRun(asUser(context.Background(), "id-bob"), "nightly", live)
	if err != nil || run.Status != cronRunCanceled || run.CanceledBy != "bob@example.com" {
		t.Fatalf("cancel = %+v (%v), want canceledBy bob", run, err)
	}

	// Render's DELETE …/runs records the same.
	svc, actors = cronWithActors(t)
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/v1/cron-jobs/nightly/runs", nil)
	validated(t, mux).ServeHTTP(rec, req.WithContext(asUser(req.Context(), "id-alice")))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("cancel current status=%d body=%s", rec.Code, rec.Body.String())
	}
	if got := actors.rows["srv-nightly/"+live]; got.CanceledBy != "id-alice" || got.TriggeredBy != "" {
		t.Errorf("cancel current recorded %+v, want canceled by id-alice", got)
	}
	if app := getApp(t, svc.Client, "nightly"); app.Spec.CancelRun == nil || app.Spec.CancelRun.Name != "nightly-run-live" {
		t.Errorf("cancel intent = %+v", app.Spec.CancelRun)
	}
}

func TestCronRunActorStoreFailures(t *testing.T) {
	// A failing write refuses the trigger before any intent lands, so no run
	// exists without its recorded caller.
	svc, actors := cronWithActors(t)
	actors.err = errors.New("db down")
	if _, err := svc.TriggerCronRun(asUser(context.Background(), "id-alice"), "nightly"); err == nil {
		t.Fatal("trigger with a failing actor store succeeded")
	}
	if app := getApp(t, svc.Client, "nightly"); app.Spec.RunAt != "" || app.Spec.CancelRun != nil {
		t.Fatalf("refused trigger wrote intent: runAt %q cancel %+v", app.Spec.RunAt, app.Spec.CancelRun)
	}
	// A failing read leaves the runs readable, naming no one.
	runs, err := svc.ListCronRuns(context.Background(), "nightly", "", 0)
	if err != nil || len(runs) != 4 {
		t.Fatalf("list with a failing actor store = %d runs (%v)", len(runs), err)
	}
	for _, r := range runs {
		if r.TriggeredBy != "" || r.CanceledBy != "" {
			t.Errorf("run %+v names an actor", r)
		}
	}
	// Store-less mode records and names nothing.
	svc.CronRunActors = nil
	run, err := svc.TriggerCronRun(asUser(context.Background(), "id-alice"), "nightly")
	if err != nil || run.TriggeredBy != "" {
		t.Fatalf("store-less trigger = %+v (%v)", run, err)
	}
}
