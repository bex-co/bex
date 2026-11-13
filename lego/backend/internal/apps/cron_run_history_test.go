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
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
	"time"

	"github.com/graphql-go/graphql"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// fakeCronRunHistory holds finished runs newest first, with the store's paging
// contract: start past `after` (an unknown one is an empty tail), skip exclude.
type fakeCronRunHistory struct {
	appID string
	runs  []store.CronRunRecord
	err   error
	calls int
}

func (f *fakeCronRunHistory) ListCronRunRecords(_ context.Context, appID, after string, exclude []string, limit int) ([]store.CronRunRecord, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	if appID != f.appID {
		return nil, nil
	}
	start := 0
	if after != "" {
		start = slices.IndexFunc(f.runs, func(r store.CronRunRecord) bool { return r.ID == after }) + 1
		if start == 0 {
			return nil, nil
		}
	}
	var out []store.CronRunRecord
	for _, r := range f.runs[start:] {
		if len(out) == limit {
			break
		}
		if !slices.Contains(exclude, r.ID) {
			out = append(out, r)
		}
	}
	return out, nil
}

func (f *fakeCronRunHistory) CronRunRecord(_ context.Context, appID, runID string) (store.CronRunRecord, bool, error) {
	if f.err != nil {
		return store.CronRunRecord{}, false, f.err
	}
	for _, r := range f.runs {
		if appID == f.appID && r.ID == runID {
			return r, true, nil
		}
	}
	return store.CronRunRecord{}, false, nil
}

// cronWithHistory is a cron job whose status holds its ten newest runs (the
// newest still running) and whose facts hold those plus five evicted ones: a
// canceled-before-start manual run among them, and the only success.
func cronWithHistory(t *testing.T) (*Service, *fakeCronRunHistory, []string) {
	t.Helper()
	base := time.Date(2026, 11, 12, 8, 0, 0, 0, time.UTC)
	at := func(i int) string { return base.Add(time.Duration(i) * time.Minute).Format(time.RFC3339) }
	app := cronWithRuns("nightly")
	app.Labels = map[string]string{store.LabelAppID: "srv-nightly"}
	app.Status.Runs = nil
	hist := &fakeCronRunHistory{appID: "srv-nightly"}
	var newestFirst []string
	for i := 14; i >= 0; i-- {
		name := fmt.Sprintf("nightly-%d", 29000000+i)
		run := appv1alpha1.CronRun{Name: name, StartedAt: at(i), FinishedAt: at(i), Status: appv1alpha1.CronRunFailed}
		switch {
		case i == 14:
			run.Status, run.FinishedAt = appv1alpha1.CronRunRunning, ""
		case i == 2:
			run.Status = appv1alpha1.CronRunSucceeded
		case i == 1:
			run.StartedAt, run.Status = "", appv1alpha1.CronRunCanceled
		}
		newestFirst = append(newestFirst, ids.Derive(ids.CronRun, name))
		if i >= 5 {
			app.Status.Runs = append(app.Status.Runs, run)
		}
		if run.Status == appv1alpha1.CronRunRunning {
			continue // no outcome yet: no record
		}
		rec := store.CronRunRecord{ID: ids.Derive(ids.CronRun, name), Name: name, Status: map[string]string{
			appv1alpha1.CronRunSucceeded: store.EventStatusSucceeded, appv1alpha1.CronRunFailed: store.EventStatusFailed, appv1alpha1.CronRunCanceled: store.EventStatusCanceled,
		}[run.Status]}
		if run.StartedAt != "" {
			rec.StartedAt, _ = time.Parse(time.RFC3339, run.StartedAt)
		}
		rec.FinishedAt, _ = time.Parse(time.RFC3339, run.FinishedAt)
		hist.runs = append(hist.runs, rec)
	}
	svc, _ := newService(nil, app)
	svc.CronRunHistory = hist
	return svc, hist, newestFirst
}

func TestCronRunListPagesPastTheStatusWindow(t *testing.T) {
	svc, _, want := cronWithHistory(t)
	ctx := context.Background()

	all, err := svc.ListCronRuns(ctx, "nightly", "", 100)
	if err != nil {
		t.Fatal(err)
	}
	got := make([]string, len(all))
	for i, r := range all {
		got[i] = r.ID
	}
	if !slices.Equal(got, want) {
		t.Fatalf("all runs =\n%v\nwant\n%v", got, want)
	}
	if all[0].Status != cronRunPending {
		t.Errorf("the running run reads %q, want pending", all[0].Status)
	}

	// Pages of four cross the window boundary (status holds ten) with no gap
	// or repeat, and a cursor from the facts part keeps going.
	var paged []string
	cursor := ""
	for range 10 {
		page, err := svc.ListCronRuns(ctx, "nightly", cursor, 4)
		if err != nil {
			t.Fatal(err)
		}
		if len(page) == 0 {
			break
		}
		for _, r := range page {
			paged = append(paged, r.ID)
		}
		cursor = page[len(page)-1].ID
	}
	if !slices.Equal(paged, want) {
		t.Fatalf("paged runs =\n%v\nwant\n%v", paged, want)
	}

	// The evicted runs keep their outcomes; the canceled-first one stays
	// canceled with no start (w5/m108), never pending.
	evicted := all[10:]
	if evicted[2].Status != cronRunSuccessful || evicted[3].Status != cronRunCanceled || evicted[3].StartedAt != "" || evicted[4].Status != cronRunUnsuccessful {
		t.Errorf("evicted runs = %+v", evicted)
	}
	if unknown, err := svc.ListCronRuns(ctx, "nightly", "crr-00000000000000000000", 4); err != nil || len(unknown) != 0 {
		t.Errorf("unknown cursor = %+v (%v), want an empty tail", unknown, err)
	}
}

func TestCronRunGetAndCancelReachEvictedRuns(t *testing.T) {
	svc, _, want := cronWithHistory(t)
	ctx := context.Background()
	evictedSuccess := want[12]

	run, err := svc.GetCronRun(ctx, "nightly", evictedSuccess)
	if err != nil || run.Status != cronRunSuccessful || run.StartedAt != "2026-11-12T08:02:00Z" || run.FinishedAt != "2026-11-12T08:02:00Z" {
		t.Fatalf("evicted run = %+v (%v)", run, err)
	}
	var coded *core.CodedError
	if _, err := svc.GetCronRun(ctx, "nightly", "crr-00000000000000000000"); !errors.As(err, &coded) || coded.Code != CronErrorRunNotFound {
		t.Errorf("unknown run = %v, want %s", err, CronErrorRunNotFound)
	}
	if _, err := svc.CancelCronRun(ctx, "nightly", evictedSuccess); !errors.As(err, &coded) || coded.Code != CronErrorRunTerminal {
		t.Errorf("cancel of an evicted run = %v, want the terminal 409 %s", err, CronErrorRunTerminal)
	}
}

func TestCronRunHistoryFailureAndStoreLessMode(t *testing.T) {
	svc, hist, want := cronWithHistory(t)
	ctx := context.Background()
	// A page the window fills never asks the store.
	if page, err := svc.ListCronRuns(ctx, "nightly", "", 3); err != nil || len(page) != 3 || hist.calls != 0 {
		t.Fatalf("window-only page = %d runs, %v, %d store calls", len(page), err, hist.calls)
	}
	hist.err = errors.New("store down")
	if _, err := svc.ListCronRuns(ctx, "nightly", "", 100); err == nil {
		t.Error("a failed history read was hidden")
	}
	svc.CronRunHistory = nil
	page, err := svc.ListCronRuns(ctx, "nightly", "", 100)
	if err != nil || len(page) != 10 || page[9].ID != want[9] {
		t.Fatalf("store-less list = %d runs (%v), want the status window", len(page), err)
	}
	if _, err := svc.GetCronRun(ctx, "nightly", want[12]); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("store-less get of an evicted run = %v, want not found", err)
	}
}

func TestLastSuccessfulRunAtPrefersTheDurableRecord(t *testing.T) {
	st := appv1alpha1.AppStatus{LastSuccessfulRunAt: "2026-11-12T08:02:00Z", Runs: []appv1alpha1.CronRun{
		{Name: "a", FinishedAt: "2026-11-12T09:00:00Z", Status: appv1alpha1.CronRunFailed},
	}}
	if got := lastSuccessfulCronRunAt(st); got != "2026-11-12T08:02:00Z" {
		t.Errorf("evicted success = %q, want the durable record", got)
	}
	st.Runs = append(st.Runs, appv1alpha1.CronRun{Name: "b", FinishedAt: "2026-11-12T09:05:00Z", Status: appv1alpha1.CronRunSucceeded})
	if got := lastSuccessfulCronRunAt(st); got != "2026-11-12T09:05:00Z" {
		t.Errorf("a newer success in status = %q, want it", got)
	}
	st.LastSuccessfulRunAt = ""
	if got := lastSuccessfulCronRunAt(st); got != "2026-11-12T09:05:00Z" {
		t.Errorf("an operator without the field = %q, want the status scan", got)
	}
	if got := lastSuccessfulCronRunAt(appv1alpha1.AppStatus{}); got != "" {
		t.Errorf("never succeeded = %q", got)
	}
}

// REST, GraphQL and MCP list and read an evicted run identically (w3/m366).
func TestCronRunSurfacesReachEvictedRuns(t *testing.T) {
	svc, _, want := cronWithHistory(t)
	evicted := want[12]

	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	list := httptest.NewRecorder()
	validated(t, mux).ServeHTTP(list, httptest.NewRequest(http.MethodGet, "/v1/cron-jobs/nightly/runs?limit=100", nil))
	var page []cronJobRunWithCursor
	if list.Code != http.StatusOK || json.Unmarshal(list.Body.Bytes(), &page) != nil || len(page) != len(want) {
		t.Fatalf("REST list status=%d runs=%d body=%s", list.Code, len(page), list.Body.String())
	}
	get := httptest.NewRecorder()
	validated(t, mux).ServeHTTP(get, httptest.NewRequest(http.MethodGet, "/v1/cron-jobs/nightly/runs/"+evicted, nil))
	var rest renderCronJobRun
	if get.Code != http.StatusOK || json.Unmarshal(get.Body.Bytes(), &rest) != nil || rest != page[12].CronJobRun || rest.Status != cronRunSuccessful {
		t.Fatalf("REST get status=%d run=%+v list=%+v", get.Code, rest, page[12].CronJobRun)
	}
	cancel := httptest.NewRecorder()
	validated(t, mux).ServeHTTP(cancel, httptest.NewRequest(http.MethodPost, "/v1/cron-jobs/nightly/runs/"+evicted+"/cancel", nil))
	if cancel.Code != http.StatusConflict {
		t.Errorf("REST cancel of an evicted run status=%d, want 409", cancel.Code)
	}

	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
		Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: svc.GraphQLMutation()}),
	})
	if err != nil {
		t.Fatal(err)
	}
	res := graphql.Do(graphql.Params{Schema: schema, Context: context.Background(), RequestString: `{ cronJobRun(serviceId:"nightly", runId:"` + evicted + `") { id status startedAt finishedAt } }`})
	if len(res.Errors) > 0 {
		t.Fatalf("GraphQL get: %v", res.Errors)
	}
	gql := res.Data.(map[string]any)["cronJobRun"].(map[string]any)
	if gql["id"] != evicted || gql["status"] != rest.Status || gql["finishedAt"] != "2026-11-12T08:02:00Z" {
		t.Errorf("GraphQL run = %v, REST run = %+v", gql, rest)
	}
	res = graphql.Do(graphql.Params{Schema: schema, Context: context.Background(), RequestString: `{ cronJobRuns(serviceId:"nightly", limit:100) { id } }`})
	if len(res.Errors) > 0 || len(res.Data.(map[string]any)["cronJobRuns"].([]any)) != len(want) {
		t.Errorf("GraphQL list: data=%v errors=%v", res.Data, res.Errors)
	}

	call, cleanup := appsMCPClient(t, svc)
	defer cleanup()
	mcpGet := call("get_cron_job_run", map[string]any{"serviceId": "nightly", "runId": evicted})
	if mcpGet["id"] != evicted || mcpGet["status"] != rest.Status {
		t.Errorf("MCP get = %v, REST run = %+v", mcpGet, rest)
	}
	mcpList := call("list_cron_job_runs", map[string]any{"serviceId": "nightly", "limit": 100})
	if runs := mcpList["cronJobRuns"].([]any); len(runs) != len(want) {
		t.Errorf("MCP list = %d runs, want %d", len(runs), len(want))
	}
}
