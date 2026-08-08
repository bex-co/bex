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

package keyvalue

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/resourcemeta"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// seedKVWithLabels seeds a minimal KeyValue CR with the given spec and labels.
func seedKVWithLabels(t *testing.T, svc *Service, name string, spec appv1alpha1.KeyValueSpec, labels map[string]string) {
	t.Helper()
	if spec.Name == "" {
		spec.Name = name
	}
	kv := &appv1alpha1.KeyValue{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
			Labels:    labels,
		},
		Spec: spec,
	}
	if err := svc.Client.Create(t.Context(), kv); err != nil {
		t.Fatalf("seed kv %s: %v", name, err)
	}
}

func listKVNames(t *testing.T, mux *http.ServeMux, query string) []string {
	t.Helper()
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/v1/key-value"+query, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /v1/key-value%s = %d: %s", query, rec.Code, rec.Body.String())
	}
	var page []keyValueWithCursor
	if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil {
		t.Fatalf("decode: %v", err)
	}
	names := make([]string, 0, len(page))
	for _, kv := range page {
		names = append(names, kv.KeyValue.Name)
	}
	return names
}

func TestRESTListKeyValueSuspendedFilter(t *testing.T) {
	// GET /v1/key-value?suspended=suspended|not_suspended (w2/m53).
	svc, _ := newService()
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	seedKVWithLabels(t, svc, "active-kv", appv1alpha1.KeyValueSpec{Plan: "free"}, nil)
	seedKVWithLabels(t, svc, "susp-kv", appv1alpha1.KeyValueSpec{Plan: "free", Suspended: true}, nil)

	got := listKVNames(t, mux, "?suspended=suspended")
	if len(got) != 1 || got[0] != "susp-kv" {
		t.Errorf("suspended=suspended = %v, want [susp-kv]", got)
	}

	got = listKVNames(t, mux, "?suspended=not_suspended")
	if len(got) != 1 || got[0] != "active-kv" {
		t.Errorf("suspended=not_suspended = %v, want [active-kv]", got)
	}

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/v1/key-value?suspended=true", nil))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("suspended=true (invalid enum) => 400, got %d", rec.Code)
	}
}

func TestRESTListKeyValueEnvironmentIDFilter(t *testing.T) {
	// GET /v1/key-value?environmentId= (w2/m53).
	svc, _ := newService()
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	seedKVWithLabels(t, svc, "kv-env-a", appv1alpha1.KeyValueSpec{Plan: "free"},
		map[string]string{core.LabelEnvironment: "env-staging"})
	seedKVWithLabels(t, svc, "kv-env-b", appv1alpha1.KeyValueSpec{Plan: "free"},
		map[string]string{core.LabelEnvironment: "env-prod"})
	seedKVWithLabels(t, svc, "kv-no-env", appv1alpha1.KeyValueSpec{Plan: "free"}, nil)

	got := listKVNames(t, mux, "?environmentId=env-staging")
	if len(got) != 1 || got[0] != "kv-env-a" {
		t.Errorf("environmentId=env-staging = %v, want [kv-env-a]", got)
	}

	got = listKVNames(t, mux, "?environmentId=env-staging&environmentId=env-prod")
	if len(got) != 2 {
		t.Errorf("multiple environmentId = %v, want 2 results", got)
	}
}

func TestRESTListKeyValueNameByIDFilter(t *testing.T) {
	// GET /v1/key-value?name=<id> resolves by id as well as name (w2/m53).
	svc, _ := newService()
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	seedKVWithLabels(t, svc, "my-cache", appv1alpha1.KeyValueSpec{Plan: "free"}, nil)

	// Resolve by name (existing behaviour).
	got := listKVNames(t, mux, "?name=my-cache")
	if len(got) != 1 || got[0] != "my-cache" {
		t.Errorf("?name=my-cache (by name) = %v, want [my-cache]", got)
	}

	// Resolve by id (the fix added in m53).
	views, err := svc.ListKeyValues(t.Context(), "")
	if err != nil || len(views) != 1 {
		t.Fatalf("ListKeyValues: %v %v", views, err)
	}
	kvID := views[0].ID
	got = listKVNames(t, mux, "?name="+kvID)
	if len(got) != 1 || got[0] != "my-cache" {
		t.Errorf("?name=%s (by id) = %v, want [my-cache]", kvID, got)
	}
}

func TestRESTListKeyValueTimeFilters(t *testing.T) {
	// Malformed timestamp → 400; empty timestamps on CRs (fake client has no
	// CreationTimestamp) pass any time window (w2/m53 — same rule as services).
	svc, _ := newService()
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	seedKVWithLabels(t, svc, "kv-a", appv1alpha1.KeyValueSpec{Plan: "free"}, nil)
	seedKVWithLabels(t, svc, "kv-b", appv1alpha1.KeyValueSpec{Plan: "free"}, nil)

	for _, param := range []string{"createdBefore", "createdAfter", "updatedBefore", "updatedAfter"} {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest("GET", "/v1/key-value?"+param+"=yesterday", nil))
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s=yesterday => 400, got %d", param, rec.Code)
		}
	}

	got := listKVNames(t, mux, "?createdBefore=2020-01-01T00:00:00Z")
	if len(got) != 2 {
		t.Errorf("empty createdAt passes createdBefore filter: got %d, want 2", len(got))
	}
}

// seedKVUpdatedAt seeds a KeyValue whose UpdatedAt resolves to the given
// timestamp, so a time-window filter has something real to place.
func seedKVUpdatedAt(t *testing.T, svc *Service, name, updatedAt string) {
	t.Helper()
	kv := &appv1alpha1.KeyValue{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   "default",
			Annotations: map[string]string{resourcemeta.UpdatedAtAnnotation: updatedAt},
		},
		Spec: appv1alpha1.KeyValueSpec{Name: name, Plan: "free"},
	}
	if err := svc.Client.Create(t.Context(), kv); err != nil {
		t.Fatalf("seed kv %s: %v", name, err)
	}
}

func TestRESTListKeyValueTimeWindowExcludes(t *testing.T) {
	// The 400 and legacy-passthrough cases above never prove the window
	// narrows anything. Stamped instances must actually fall in or out of it.
	svc, _ := newService()
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	seedKVUpdatedAt(t, svc, "kv-old", "2026-01-01T00:00:00Z")
	seedKVUpdatedAt(t, svc, "kv-new", "2026-12-01T00:00:00Z")
	seedKVWithLabels(t, svc, "kv-unstamped", appv1alpha1.KeyValueSpec{Plan: "free"}, nil)

	cases := []struct {
		query string
		want  []string
	}{
		// kv-unstamped has no timestamp, so it rides along with every window.
		{"?updatedBefore=2026-06-01T00:00:00Z", []string{"kv-old", "kv-unstamped"}},
		{"?updatedAfter=2026-06-01T00:00:00Z", []string{"kv-new", "kv-unstamped"}},
		{"?updatedAfter=2026-06-01T00:00:00Z&updatedBefore=2027-01-01T00:00:00Z", []string{"kv-new", "kv-unstamped"}},
		{"?updatedAfter=2027-01-01T00:00:00Z", []string{"kv-unstamped"}},
	}
	for _, c := range cases {
		got := listKVNames(t, mux, c.query)
		slices.Sort(got)
		want := slices.Clone(c.want)
		slices.Sort(want)
		if !slices.Equal(got, want) {
			t.Errorf("GET /v1/key-value%s = %v, want %v", c.query, got, want)
		}
	}
}

func TestRESTListKeyValueNameFilterAcceptsCommaAndRepeatedForms(t *testing.T) {
	// Render's form-style array encoding: the official CLI sends a multi-value
	// flag as one comma-separated value, hand-written clients repeat the key.
	svc, _ := newService()
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	for _, name := range []string{"kv-alpha", "kv-bravo", "kv-charlie"} {
		seedKVWithLabels(t, svc, name, appv1alpha1.KeyValueSpec{Plan: "free"}, nil)
	}

	for _, query := range []string{
		"?name=kv-alpha,kv-bravo",
		"?name=kv-alpha&name=kv-bravo",
		"?name=%20kv-alpha%20,kv-bravo",
	} {
		got := listKVNames(t, mux, query)
		slices.Sort(got)
		if !slices.Equal(got, []string{"kv-alpha", "kv-bravo"}) {
			t.Errorf("GET /v1/key-value%s = %v, want [kv-alpha kv-bravo]", query, got)
		}
	}
}
