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
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// recordingJobStore captures the filter the REST layer translated a query
// string into, so the translation can be asserted end to end rather than by
// calling the private parser directly.
type recordingJobStore struct{ filter store.JobListFilter }

func (s *recordingJobStore) CreateJob(context.Context, string, string, string, string) (store.Job, error) {
	return store.Job{}, nil
}

func (s *recordingJobStore) ListJobs(_ context.Context, _, _ string, filter store.JobListFilter) ([]store.Job, error) {
	s.filter = filter
	return nil, nil
}

func (s *recordingJobStore) GetJob(context.Context, string, string, string) (store.Job, error) {
	return store.Job{}, nil
}

func (s *recordingJobStore) UpdateJobStatus(context.Context, string, string) (store.Job, error) {
	return store.Job{}, nil
}

func filterHarness(t *testing.T) (*recordingJobStore, *http.ServeMux) {
	t.Helper()
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = appv1alpha1.AddToScheme(scheme)
	app := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default"},
		Spec:       appv1alpha1.AppSpec{Image: "web:v1"},
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(client.Object(app)).Build()
	st := &recordingJobStore{}
	svc := &Service{Base: &core.Base{Authz: allowAllChecker{}, Client: cl, Namespace: "default"}, Store: st}
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	return st, mux
}

func getJobs(t *testing.T, mux *http.ServeMux, target string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "user-a", Method: "session"})
	mux.ServeHTTP(rec, req.WithContext(ctx))
	return rec
}

// TestListJobsTranslatesEveryQueryParam covers the whole ListJobParams surface
// in one request. The list filter had no test at all, and its parser reached
// the query values through a redundant map[string][]string conversion plus a
// discarded http.Header parameter — shapes that are easy to "simplify" wrongly,
// since dropping the repeated-key handling on `status` or the first-value
// semantics elsewhere still compiles and still passes every other test.
func TestListJobsTranslatesEveryQueryParam(t *testing.T) {
	st, mux := filterHarness(t)

	rec := getJobs(t, mux, "/v1/services/web/jobs?"+
		"status=running&status=succeeded"+
		"&createdBefore=2026-01-02T03:04:05Z&createdAfter=2026-01-01T00:00:00Z"+
		"&startedBefore=2026-02-02T00:00:00Z&startedAfter=2026-02-01T00:00:00Z"+
		"&finishedBefore=2026-03-02T00:00:00Z&finishedAfter=2026-03-01T00:00:00Z"+
		"&cursor=job-7&limit=42")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d (body: %s)", rec.Code, rec.Body.String())
	}

	got := st.filter
	// Repeated keys must all survive — Render's status filter is a set.
	if !slices.Equal(got.Statuses, []string{"running", "succeeded"}) {
		t.Errorf("Statuses = %v, want both repeated values", got.Statuses)
	}
	if got.Cursor != "job-7" {
		t.Errorf("Cursor = %q, want job-7", got.Cursor)
	}
	if got.Limit != 42 {
		t.Errorf("Limit = %d, want 42", got.Limit)
	}
	for _, tc := range []struct {
		name string
		got  time.Time
		want string
	}{
		{"CreatedBefore", got.CreatedBefore, "2026-01-02T03:04:05Z"},
		{"CreatedAfter", got.CreatedAfter, "2026-01-01T00:00:00Z"},
		{"StartedBefore", got.StartedBefore, "2026-02-02T00:00:00Z"},
		{"StartedAfter", got.StartedAfter, "2026-02-01T00:00:00Z"},
		{"FinishedBefore", got.FinishedBefore, "2026-03-02T00:00:00Z"},
		{"FinishedAfter", got.FinishedAfter, "2026-03-01T00:00:00Z"},
	} {
		want, err := time.Parse(time.RFC3339, tc.want)
		if err != nil {
			t.Fatal(err)
		}
		if !tc.got.Equal(want) {
			t.Errorf("%s = %v, want %v", tc.name, tc.got, want)
		}
	}
}

func TestListJobsFilterEdgeCases(t *testing.T) {
	t.Run("no params leaves the filter zero", func(t *testing.T) {
		st, mux := filterHarness(t)
		if rec := getJobs(t, mux, "/v1/services/web/jobs"); rec.Code != http.StatusOK {
			t.Fatalf("status = %d (body: %s)", rec.Code, rec.Body.String())
		}
		// Absent limit stays 0 so the store applies its own newest-first cap
		// (codex-security round-6 #7), rather than silently defaulting to 20.
		if st.filter.Limit != 0 || st.filter.Cursor != "" || len(st.filter.Statuses) != 0 {
			t.Fatalf("filter = %+v, want zero", st.filter)
		}
		if !st.filter.CreatedBefore.IsZero() || !st.filter.FinishedAfter.IsZero() {
			t.Fatalf("time bounds = %+v, want zero", st.filter)
		}
	})

	for _, bad := range []string{"limit=0", "limit=-1", "limit=abc", "createdBefore=not-a-time"} {
		t.Run("rejects "+bad, func(t *testing.T) {
			_, mux := filterHarness(t)
			if rec := getJobs(t, mux, "/v1/services/web/jobs?"+bad); rec.Code != http.StatusBadRequest {
				t.Fatalf("%s = %d, want 400 (body: %s)", bad, rec.Code, rec.Body.String())
			}
		})
	}
}
