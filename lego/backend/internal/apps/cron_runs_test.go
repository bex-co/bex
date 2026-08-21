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
	"time"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

type cronBillingGate struct {
	calls []string
	err   error
}

func (g *cronBillingGate) CheckBillingMutationAllowed(_ context.Context, workspaceID string) error {
	g.calls = append(g.calls, workspaceID)
	return g.err
}

type cronAuthzCounter struct{ calls int }

func (c *cronAuthzCounter) Check(_ context.Context, _, _, _ string) (bool, error) {
	c.calls++
	return true, nil
}

type cronPatchCountingClient struct {
	client.Client
	patches int
}

func (c *cronPatchCountingClient) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	c.patches++
	return c.Client.Patch(ctx, obj, patch, opts...)
}

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

// TestCronNextRunAt covers w9/056: the computed next scheduled fire time (a bex
// extension), consistent across the view, REST cronJobDetails, and GraphQL, and
// omitted for a suspended cron, a non-cron service, and an invalid schedule.
func TestCronNextRunAt(t *testing.T) {
	// A cron on */5 * * * * at 12:02:10 next fires at 12:05:00 (UTC).
	svc, _ := newService(nil, cronWithRuns("nightly"), sampleApp("web"))
	svc.Clock = func() time.Time { return time.Date(2026, 7, 14, 12, 2, 10, 0, time.UTC) }

	view, err := svc.Get(context.Background(), "nightly")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	const wantNext = "2026-07-14T12:05:00Z"
	if view.NextRunAt != wantNext {
		t.Fatalf("view.NextRunAt = %q, want %q", view.NextRunAt, wantNext)
	}
	if got := toRenderService(view).ServiceDetails["nextRunAt"]; got != wantNext {
		t.Errorf("cronJobDetails.nextRunAt = %v, want %q", got, wantNext)
	}

	// A non-cron service carries no next-run.
	web, err := svc.Get(context.Background(), "web")
	if err != nil {
		t.Fatalf("Get web: %v", err)
	}
	if web.NextRunAt != "" {
		t.Errorf("non-cron NextRunAt = %q, want empty", web.NextRunAt)
	}
}

// TestCronNextRunAtSuspendedAndInvalid confirms the two omission paths: a
// suspended cron (its CronJob is paused, so there is no next run) and a cron
// whose stored schedule does not parse.
func TestCronNextRunAtSuspendedAndInvalid(t *testing.T) {
	suspended := cronApp("paused")
	suspended.Spec.Suspended = true
	invalid := cronApp("broken")
	invalid.Spec.Schedule = "99 99 * * *" // a legacy/hand-edited unparseable schedule
	svc, _ := newService(nil, suspended, invalid)
	svc.Clock = func() time.Time { return time.Date(2026, 7, 14, 12, 2, 10, 0, time.UTC) }

	for _, name := range []string{"paused", "broken"} {
		view, err := svc.Get(context.Background(), name)
		if err != nil {
			t.Fatalf("Get %s: %v", name, err)
		}
		if view.NextRunAt != "" {
			t.Errorf("%s NextRunAt = %q, want empty", name, view.NextRunAt)
		}
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
	svc, _ := newService(nil, cronWithRuns("nightly"))
	cl := &cronPatchCountingClient{Client: svc.Client}
	svc.Client = cl
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
	wantRequestedAt := fixed.Format(time.RFC3339Nano)
	if app.Spec.RunAt != wantRequestedAt || app.Spec.CancelRun == nil || app.Spec.CancelRun.Name != "nightly-run-live" || app.Spec.CancelRun.RequestedAt != wantRequestedAt {
		t.Fatalf("trigger intent = runAt %q cancel %+v", app.Spec.RunAt, app.Spec.CancelRun)
	}
	if cl.patches != 1 {
		t.Fatalf("active cancel-and-replace used %d Kubernetes patches, want 1", cl.patches)
	}
}

func TestTriggerCronRunRejectsSuspendedWithoutWritingIntent(t *testing.T) {
	app := cronWithRuns("nightly")
	app.Spec.Suspended = true
	app.Spec.RunAt = "2026-07-14T11:59:00Z"
	app.Spec.CancelRun = &appv1alpha1.CronRunCancellation{
		Name:        "nightly-prior-run",
		RequestedAt: "2026-07-14T11:59:01Z",
	}

	svc, _ := newService(nil, app)
	cl := &cronPatchCountingClient{Client: svc.Client}
	svc.Client = cl
	authz := &cronAuthzCounter{}
	billing := &cronBillingGate{err: core.ErrBillingEnforced}
	svc.Authz = authz
	svc.Billing = billing
	svc.Clock = func() time.Time { return time.Date(2026, 7, 14, 12, 5, 0, 0, time.UTC) }
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "cron-operator", Method: "session"})
	if _, err := svc.TriggerCronRun(ctx, "nightly"); !errors.Is(err, core.ErrConflict) {
		t.Fatalf("trigger suspended cron = %v, want conflict", err)
	}
	if authz.calls != 1 {
		t.Fatalf("suspended trigger authorization calls = %d, want 1", authz.calls)
	}
	if len(billing.calls) != 0 {
		t.Fatalf("suspended trigger consulted billing gate: %v", billing.calls)
	}
	if cl.patches != 0 {
		t.Fatalf("suspended trigger used %d Kubernetes patches, want 0", cl.patches)
	}

	got := getApp(t, cl, "nightly")
	if got.Spec.RunAt != "2026-07-14T11:59:00Z" {
		t.Fatalf("suspended trigger changed runAt to %q", got.Spec.RunAt)
	}
	if got.Spec.CancelRun == nil || got.Spec.CancelRun.Name != "nightly-prior-run" || got.Spec.CancelRun.RequestedAt != "2026-07-14T11:59:01Z" {
		t.Fatalf("suspended trigger changed cancel intent to %+v", got.Spec.CancelRun)
	}
}

func TestCronRunBillingEnforcementRejectsGraphQLTriggerWithoutWriteButAllowsCancel(t *testing.T) {
	app := cronWithRuns("nightly")
	app.Labels = map[string]string{core.LabelTenant: "tea-billing"}
	svc, _ := newService(nil, app)
	cl := &cronPatchCountingClient{Client: svc.Client}
	svc.Client = cl
	billing := &cronBillingGate{err: core.ErrBillingEnforced}
	svc.Billing = billing

	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
		Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: svc.GraphQLMutation()}),
	})
	if err != nil {
		t.Fatal(err)
	}
	res := graphql.Do(graphql.Params{
		Schema:        schema,
		Context:       context.Background(),
		RequestString: `mutation { runCronJob(id:"nightly") { id status } }`,
	})
	if len(res.Errors) != 1 || res.Errors[0].Message != core.ErrBillingEnforced.Error() {
		t.Fatalf("billing-enforced run errors = %+v, want exact %q", res.Errors, core.ErrBillingEnforced.Error())
	}
	if len(billing.calls) != 1 || billing.calls[0] != "tea-billing" {
		t.Fatalf("run billing calls = %v, want [tea-billing]", billing.calls)
	}
	if cl.patches != 0 {
		t.Fatalf("billing-enforced run used %d Kubernetes patches, want 0", cl.patches)
	}
	got := getApp(t, cl, "nightly")
	if got.Spec.RunAt != "" || got.Spec.CancelRun != nil {
		t.Fatalf("billing-enforced run wrote intent: runAt=%q cancel=%+v", got.Spec.RunAt, got.Spec.CancelRun)
	}

	runs, err := svc.ListCronRuns(context.Background(), "nightly", "", 1)
	if err != nil || len(runs) != 1 {
		t.Fatalf("list active run: runs=%+v err=%v", runs, err)
	}
	if _, err := svc.CancelCronRun(context.Background(), "nightly", runs[0].ID); err != nil {
		t.Fatalf("cancel under billing enforcement: %v", err)
	}
	if len(billing.calls) != 1 {
		t.Fatalf("cancel consulted billing gate: %v", billing.calls)
	}
	if cl.patches != 1 {
		t.Fatalf("cancel under billing enforcement used %d patches, want 1", cl.patches)
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

func callAppsMCPError(t *testing.T, svc *Service, name string, args map[string]any) string {
	t.Helper()
	server := mcp.NewServer(&mcp.Implementation{Name: "cron-error-test", Version: "0"}, nil)
	svc.RegisterMCP(server)
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	ctx := context.Background()
	if _, err := server.Connect(ctx, serverTransport, nil); err != nil {
		t.Fatal(err)
	}
	client, err := mcp.NewClient(&mcp.Implementation{Name: "cron-error-test", Version: "0"}, nil).Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })
	result, err := client.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatal(err)
	}
	if result == nil || !result.IsError || len(result.Content) != 1 {
		t.Fatalf("MCP %s result = %#v, want one error", name, result)
	}
	content, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("MCP %s error content = %T, want text", name, result.Content[0])
	}
	return content.Text
}

func TestCronActionErrorCodesMatchRESTGraphQLAndMCP(t *testing.T) {
	unknownRunID := "crr-00000000000000000000"
	terminalRunID := ids.Derive(ids.CronRun, "nightly-run-ok")
	tests := []struct {
		name       string
		service    func() *Service
		restMethod string
		restPath   string
		gql        string
		mcpTool    string
		mcpArgs    map[string]any
		wantStatus int
		wantCode   string
	}{
		{
			name: "suspended",
			service: func() *Service {
				app := cronWithRuns("nightly")
				app.Spec.Suspended = true
				svc, _ := newService(nil, app)
				return svc
			},
			restMethod: http.MethodPost,
			restPath:   "/v1/cron-jobs/nightly/runs",
			gql:        `mutation { runCronJob(id:"nightly") { id } }`,
			mcpTool:    "run_cron_job",
			mcpArgs:    map[string]any{"serviceId": "nightly"},
			wantStatus: http.StatusConflict,
			wantCode:   CronErrorSuspended,
		},
		{
			name: "run not found",
			service: func() *Service {
				svc, _ := newService(nil, cronWithRuns("nightly"))
				return svc
			},
			restMethod: http.MethodGet,
			restPath:   "/v1/cron-jobs/nightly/runs/" + unknownRunID,
			gql:        `{ cronJobRun(serviceId:"nightly", runId:"` + unknownRunID + `") { id } }`,
			mcpTool:    "get_cron_job_run",
			mcpArgs:    map[string]any{"serviceId": "nightly", "runId": unknownRunID},
			wantStatus: http.StatusNotFound,
			wantCode:   CronErrorRunNotFound,
		},
		{
			name: "terminal run",
			service: func() *Service {
				svc, _ := newService(nil, cronWithRuns("nightly"))
				return svc
			},
			restMethod: http.MethodPost,
			restPath:   "/v1/cron-jobs/nightly/runs/" + terminalRunID + "/cancel",
			gql:        `mutation { cancelCronJobRun(serviceId:"nightly", runId:"` + terminalRunID + `") { id } }`,
			mcpTool:    "cancel_cron_job_run",
			mcpArgs:    map[string]any{"serviceId": "nightly", "runId": terminalRunID},
			wantStatus: http.StatusConflict,
			wantCode:   CronErrorRunTerminal,
		},
		{
			name: "billing enforcement",
			service: func() *Service {
				app := cronWithRuns("nightly")
				app.Labels = map[string]string{core.LabelTenant: "tea-billing"}
				svc, _ := newService(nil, app)
				svc.Billing = &cronBillingGate{err: core.ErrBillingEnforced}
				return svc
			},
			restMethod: http.MethodPost,
			restPath:   "/v1/cron-jobs/nightly/runs",
			gql:        `mutation { runCronJob(id:"nightly") { id } }`,
			mcpTool:    "run_cron_job",
			mcpArgs:    map[string]any{"serviceId": "nightly"},
			wantStatus: http.StatusConflict,
			wantCode:   core.BillingErrorEnforced,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name+" REST", func(t *testing.T) {
			svc := tt.service()
			mux := http.NewServeMux()
			svc.RegisterREST(mux)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, httptest.NewRequest(tt.restMethod, tt.restPath, nil))
			if rec.Code != tt.wantStatus {
				t.Fatalf("status=%d body=%s, want %d", rec.Code, rec.Body.String(), tt.wantStatus)
			}
			var body map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if body["code"] != tt.wantCode {
				t.Fatalf("body=%#v, want code %s", body, tt.wantCode)
			}
		})

		t.Run(tt.name+" GraphQL", func(t *testing.T) {
			svc := tt.service()
			schema, err := graphql.NewSchema(graphql.SchemaConfig{
				Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
				Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: svc.GraphQLMutation()}),
			})
			if err != nil {
				t.Fatal(err)
			}
			res := graphql.Do(graphql.Params{Schema: schema, Context: context.Background(), RequestString: tt.gql})
			if len(res.Errors) != 1 || res.Errors[0].Extensions["code"] != tt.wantCode {
				t.Fatalf("errors=%#v, want code %s", res.Errors, tt.wantCode)
			}
		})

		t.Run(tt.name+" MCP", func(t *testing.T) {
			text := callAppsMCPError(t, tt.service(), tt.mcpTool, tt.mcpArgs)
			if !strings.Contains(text, tt.wantCode) {
				t.Fatalf("MCP error=%q, want code %s", text, tt.wantCode)
			}
		})
	}
}
