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

package jobs

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// gate_test.go pins w4/m116/t003: one-off jobs are off-roadmap
// (.pm/DO_NOT_DO.md), so create refuses with a named 410 on every surface —
// never the pre-gate behavior of persisting a record, having Kubernetes refuse
// the submit, and returning a job that was already `failed` — while the
// history verbs (list/get/cancel) keep serving jobs created before the gate.

// historyJobStore holds one pre-gate job and records every write, so a test can
// prove create persists nothing and cancel still transitions history.
type historyJobStore struct {
	job    store.Job
	writes []string
}

func (s *historyJobStore) ListJobs(context.Context, string, string, store.JobListFilter) ([]store.Job, error) {
	return []store.Job{s.job}, nil
}

func (s *historyJobStore) GetJob(_ context.Context, _, _, jobID string) (store.Job, error) {
	if jobID != s.job.ID {
		return store.Job{}, store.ErrNotFound
	}
	return s.job, nil
}

func (s *historyJobStore) UpdateJobStatus(_ context.Context, jobID, status string) (store.Job, error) {
	s.writes = append(s.writes, jobID+"="+status)
	s.job.Status = status
	return s.job, nil
}

type gateHarness struct {
	store      *historyJobStore
	svc        *Service
	mux        *http.ServeMux
	jobCreates int
}

// newGateHarness mounts the jobs surfaces over one App "web" carrying a
// running pre-gate job. Its Kubernetes client counts batch Job creates, so the
// gate is proven to refuse BEFORE any submit is attempted.
func newGateHarness(t *testing.T, authz core.Checker) *gateHarness {
	t.Helper()
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = appv1alpha1.AddToScheme(scheme)
	app := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default"},
		Spec:       appv1alpha1.AppSpec{Image: "web:v1"},
	}
	h := &gateHarness{store: &historyJobStore{job: store.Job{
		ID: "job-pregate", ServiceName: "web", TenantID: core.DefaultTenant,
		StartCommand: "echo old", Status: store.JobRunning,
	}}}
	k8sJob := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{Name: "job-pregate", Namespace: "default"},
		Status:     batchv1.JobStatus{Active: 1},
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(client.Object(app), k8sJob).
		WithInterceptorFuncs(interceptor.Funcs{
			Create: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
				if _, isJob := obj.(*batchv1.Job); isJob {
					h.jobCreates++
				}
				return c.Create(ctx, obj, opts...)
			},
		}).Build()
	h.svc = &Service{Base: &core.Base{Authz: authz, Client: cl, Namespace: "default"}, Store: h.store}
	h.mux = http.NewServeMux()
	h.svc.RegisterREST(h.mux)
	return h
}

func (h *gateHarness) do(t *testing.T, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "user-a", Method: "session"})
	h.mux.ServeHTTP(rec, req.WithContext(ctx))
	return rec
}

func (h *gateHarness) assertNothingSubmitted(t *testing.T) {
	t.Helper()
	if h.jobCreates != 0 {
		t.Errorf("a gated create submitted %d Kubernetes Job(s); want none", h.jobCreates)
	}
	if len(h.store.writes) != 0 {
		t.Errorf("a gated create wrote job records %v; want none", h.store.writes)
	}
}

// TestCreateIsRefusedWithNamedGone is the REST contract the pinned Render CLI
// reads: a 410 whose body carries Render's {id, message, code} error shape, so
// `bex jobs create` prints
// "received response code 410 (ONE_OFF_JOBS_UNSUPPORTED): one-off jobs are not supported …".
func TestCreateIsRefusedWithNamedGone(t *testing.T) {
	h := newGateHarness(t, allowAllChecker{})

	rec := h.do(t, http.MethodPost, "/v1/services/web/jobs", `{"startCommand":"echo qa-marker","planId":"free"}`)
	if rec.Code != http.StatusGone {
		t.Fatalf("status = %d, want 410 (body: %s)", rec.Code, rec.Body.String())
	}
	var body struct {
		ID      string `json:"id"`
		Message string `json:"message"`
		Code    string `json:"code"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v (%s)", err, rec.Body.String())
	}
	if body.ID != "gone" || body.Code != CodeOneOffJobsUnsupported {
		t.Errorf("body id/code = %q/%q, want gone/%s", body.ID, body.Code, CodeOneOffJobsUnsupported)
	}
	if !strings.Contains(body.Message, "one-off jobs are not supported") {
		t.Errorf("message must say why: %q", body.Message)
	}
	h.assertNothingSubmitted(t)
}

// TestCreateRefusalComesAfterAuthorization keeps the gate from becoming an
// existence oracle or bypassing the can_create relation: a caller who cannot
// act on the service, or a service that does not exist, still gets the
// ordinary authz/404 answer, never the 410.
func TestCreateRefusalComesAfterAuthorization(t *testing.T) {
	denied := newGateHarness(t, denyAllChecker{})
	if rec := denied.do(t, http.MethodPost, "/v1/services/web/jobs", `{"startCommand":"true"}`); rec.Code == http.StatusGone || rec.Code < 400 {
		t.Errorf("unauthorized create = %d, want an authz refusal, not 410 (body: %s)", rec.Code, rec.Body.String())
	}
	denied.assertNothingSubmitted(t)

	h := newGateHarness(t, allowAllChecker{})
	if rec := h.do(t, http.MethodPost, "/v1/services/nope/jobs", `{"startCommand":"true"}`); rec.Code != http.StatusNotFound {
		t.Errorf("create on a missing service = %d, want 404 (body: %s)", rec.Code, rec.Body.String())
	}
}

// TestCreateRefusalIsIdenticalOnGraphQLAndMCP drives the same Service verb
// through the GraphQL mutation and the MCP tool: both carry the stable code.
func TestCreateRefusalIsIdenticalOnGraphQLAndMCP(t *testing.T) {
	// Authz nil, as in list_limit_surface_test.go: the in-memory MCP transport
	// gives the handler its own identity-less context.
	h := newGateHarness(t, nil)

	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: h.svc.GraphQLQuery()}),
		Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: h.svc.GraphQLMutation()}),
	})
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	res := graphql.Do(graphql.Params{
		Schema:        schema,
		RequestString: `mutation { createJob(serviceId: "web", startCommand: "echo hi") { id status } }`,
		Context:       context.Background(),
	})
	if len(res.Errors) != 1 {
		t.Fatalf("GraphQL createJob errors = %v, want exactly one refusal", res.Errors)
	}
	if got := res.Errors[0].Extensions["code"]; got != CodeOneOffJobsUnsupported {
		t.Errorf("GraphQL extensions.code = %v, want %s", got, CodeOneOffJobsUnsupported)
	}

	out, err := mcpSession(t, h.svc).CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "create_job",
		Arguments: map[string]any{"serviceId": "web", "startCommand": "echo hi"},
	})
	if err != nil {
		t.Fatalf("MCP create_job transport error: %v", err)
	}
	if !out.IsError {
		t.Fatalf("MCP create_job succeeded; want the refusal")
	}
	var text string
	for _, c := range out.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			text += tc.Text
		}
	}
	if !strings.HasPrefix(text, CodeOneOffJobsUnsupported+": ") {
		t.Errorf("MCP error text = %q, want it prefixed with %s", text, CodeOneOffJobsUnsupported)
	}
	h.assertNothingSubmitted(t)
}

// TestHistoryVerbsStillServePreGateJobs: the gate retires creation only. A job
// created before it is still listed, fetched, and cancelable.
func TestHistoryVerbsStillServePreGateJobs(t *testing.T) {
	h := newGateHarness(t, allowAllChecker{})

	if rec := h.do(t, http.MethodGet, "/v1/services/web/jobs", ""); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "job-pregate") {
		t.Errorf("list = %d %s, want 200 naming the pre-gate job", rec.Code, rec.Body.String())
	}
	if rec := h.do(t, http.MethodGet, "/v1/services/web/jobs/job-pregate", ""); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"status":"running"`) {
		t.Errorf("get = %d %s, want 200 running", rec.Code, rec.Body.String())
	}
	rec := h.do(t, http.MethodPost, "/v1/services/web/jobs/job-pregate/cancel", "")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"status":"canceled"`) {
		t.Fatalf("cancel = %d %s, want 200 canceled", rec.Code, rec.Body.String())
	}
	if len(h.store.writes) != 1 || h.store.writes[0] != "job-pregate="+store.JobCanceled {
		t.Errorf("cancel writes = %v, want the one canceled transition", h.store.writes)
	}
}

type denyAllChecker struct{}

func (denyAllChecker) Check(context.Context, string, string, string) (bool, error) {
	return false, nil
}
