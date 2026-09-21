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
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	batchv1 "k8s.io/api/batch/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// recordingCreateStore is the filter harness's store with a working CreateJob,
// so a submit failure has a real record to flip.
type recordingCreateStore struct {
	recordingJobStore
	created  store.Job
	statuses []string
}

func (s *recordingCreateStore) CreateJob(_ context.Context, serviceName, tenantID, startCommand, planID string) (store.Job, error) {
	s.created = store.Job{
		ID: "job-submitfail", ServiceName: serviceName, TenantID: tenantID,
		StartCommand: startCommand, PlanID: planID, Status: store.JobPending,
	}
	return s.created, nil
}

func (s *recordingCreateStore) UpdateJobStatus(_ context.Context, _, status string) (store.Job, error) {
	s.statuses = append(s.statuses, status)
	j := s.created
	j.Status = status
	return j, nil
}

// submitRefusedHarness builds a Service whose Kubernetes client refuses to
// create batch Jobs exactly the way production does: bex-api's grant on
// batch/jobs is get,list only (deploy/gitops/base/bex-api-apps-rbac.yaml), so
// the submit comes back Forbidden from admission.
func submitRefusedHarness(t *testing.T, refusal error) (*recordingCreateStore, *http.ServeMux) {
	t.Helper()
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = appv1alpha1.AddToScheme(scheme)
	app := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default"},
		Spec:       appv1alpha1.AppSpec{Image: "web:v1"},
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(client.Object(app)).
		WithInterceptorFuncs(interceptor.Funcs{
			Create: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
				if _, isJob := obj.(*batchv1.Job); isJob {
					return refusal
				}
				return c.Create(ctx, obj, opts...)
			},
		}).Build()
	st := &recordingCreateStore{}
	svc := &Service{Base: &core.Base{Authz: allowAllChecker{}, Client: cl, Namespace: "default"}, Store: st}
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	return st, mux
}

func postJob(t *testing.T, mux *http.ServeMux) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/services/web/jobs",
		strings.NewReader(`{"startCommand":"echo qa-marker"}`))
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "user-a", Method: "session"})
	mux.ServeHTTP(rec, req.WithContext(ctx))
	return rec
}

// TestSubmitRefusalIsReportedNotSwallowed pins w4/m116/t003. `bex jobs create`
// returned a 201 carrying a job that was ALREADY `failed`, ~12-16ms after
// create, with no reason on REST, GraphQL or MCP — the Kubernetes refusal was
// logged server-side and dropped. A reason-less terminal state seconds after
// create must be impossible: the caller now gets the refusal itself.
func TestSubmitRefusalIsReportedNotSwallowed(t *testing.T) {
	refusal := apierrors.NewForbidden(
		schema.GroupResource{Group: "batch", Resource: "jobs"}, "job-submitfail",
		errors.New(`jobs.batch is forbidden: User "system:serviceaccount:bex-system:bex-api" cannot create resource "jobs"`))
	st, mux := submitRefusedHarness(t, refusal)

	rec := postJob(t, mux)
	if rec.Code == http.StatusCreated {
		t.Fatalf("a job that could not be submitted must not read as created: %d %s", rec.Code, rec.Body.String())
	}
	// A platform-side grant gap is not the caller's lack of permission — the
	// caller was already authorized — so it must not surface as 403.
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503 (body: %s)", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "job-submitfail") {
		t.Errorf("error must name the job so it can be found in `jobs list`: %s", body)
	}
	if !strings.Contains(body, "cannot create resource") {
		t.Errorf("error must carry the Kubernetes cause verbatim: %s", body)
	}

	// The record still lands as failed, so the attempt survives in history.
	if len(st.statuses) != 1 || st.statuses[0] != store.JobFailed {
		t.Errorf("job record statuses = %v, want one %q", st.statuses, store.JobFailed)
	}
}

// TestSubmitFailureClassIsUnavailableNotForbidden covers the non-RBAC refusals
// (admission policy, quota) through the same path: still reported, still 503,
// still naming the cause.
func TestSubmitFailureClassIsUnavailableNotForbidden(t *testing.T) {
	_, mux := submitRefusedHarness(t, errors.New("admission webhook denied the request: pod security"))

	rec := postJob(t, mux)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503 (body: %s)", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "admission webhook denied") {
		t.Errorf("error must carry the cause: %s", rec.Body.String())
	}
}
