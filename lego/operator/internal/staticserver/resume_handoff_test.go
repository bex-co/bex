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

package staticserver

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// failingReader fails List while fail is set, so a refresh error can be staged.
type failingReader struct {
	client.Reader
	fail bool
}

func (r *failingReader) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	if r.fail {
		return errors.New("apiserver unavailable")
	}
	return r.Reader.List(ctx, list, opts...)
}

type handoffFixture struct {
	cl       client.Client
	reader   *failingReader
	resolver *CachedResolver
	origin   *fakeOrigin
	h        *Handler
	app      *appv1alpha1.App
}

const handoffPrefix = "tea-aaaaaaaaaaaaaaaaaaaa/mysite/rev-3/"

func newHandoffFixture(t *testing.T) *handoffFixture {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := appv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	app := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: appID, Namespace: "apps"},
		Spec: appv1alpha1.AppSpec{
			Type: appv1alpha1.TypeStaticSite,
			Host: testHost,
			Routes: []appv1alpha1.StaticRoute{{
				Type: "rewrite", Source: "/friendly", Destination: "/version.txt",
			}},
			Headers: []appv1alpha1.StaticHeader{
				{Path: "/friendly", Name: "X-QA-A520", Value: "preserved"},
				// A customer rule must not make the transition cacheable or
				// disable backoff.
				{Path: "/*", Name: "Cache-Control", Value: "public, max-age=86400"},
				{Path: "/*", Name: "Retry-After", Value: "0"},
			},
		},
		Status: appv1alpha1.AppStatus{
			Phase:          appv1alpha1.PhaseRunning,
			ActiveRevision: "rev-3",
			StaticPrefix:   handoffPrefix,
		},
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(app).
		WithStatusSubresource(&appv1alpha1.App{}).Build()
	reader := &failingReader{Reader: cl}
	resolver := NewCachedResolver(reader, app.Namespace, "onbex.co")
	origin := newFakeOrigin(map[string]Object{
		handoffPrefix + "index.html":  {Body: []byte("QA_A520_ROOT\n"), ContentType: "text/html"},
		handoffPrefix + "version.txt": {Body: []byte("QA_A520_VERSION\n"), ContentType: "text/plain"},
	})
	f := &handoffFixture{cl: cl, reader: reader, resolver: resolver, origin: origin,
		h: New(resolver, origin, 1<<20), app: app}
	f.refresh(t)
	return f
}

func (f *handoffFixture) refresh(t *testing.T) {
	t.Helper()
	if err := f.resolver.Refresh(t.Context()); err != nil {
		t.Fatal(err)
	}
}

func (f *handoffFixture) setSuspended(t *testing.T, suspended bool) {
	t.Helper()
	if err := f.cl.Get(t.Context(), client.ObjectKeyFromObject(f.app), f.app); err != nil {
		t.Fatal(err)
	}
	f.app.Spec.Suspended = suspended
	if err := f.cl.Update(t.Context(), f.app); err != nil {
		t.Fatal(err)
	}
}

func (f *handoffFixture) fetches() int {
	n := 0
	for _, c := range f.origin.gets {
		n += c
	}
	return n
}

func assertTemporarilyUnavailable(t *testing.T, label string, rec *httptest.ResponseRecorder, wantCustom bool) {
	t.Helper()
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("%s: status %d %q, want 503", label, rec.Code, rec.Body)
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("%s: Cache-Control %q, want no-store", label, got)
	}
	if got := rec.Header().Get("Retry-After"); got != "5" {
		t.Errorf("%s: Retry-After %q, want 5", label, got)
	}
	if got := rec.Body.String(); got != "static site temporarily unavailable\n" {
		t.Errorf("%s: body %q, want the temporary-unavailable message", label, got)
	}
	if got := rec.Header().Get("X-QA-A520"); (got == "preserved") != wantCustom {
		t.Errorf("%s: X-QA-A520 %q, want custom header applied=%v", label, got, wantCustom)
	}
}

// Resume persists and the Ingress returns to the static-server before the
// resolver's next poll. The host must read as temporarily unavailable — never
// as absent, never with cached bytes — and serve its original publication once
// a Refresh observes the resume (w4/307).
func TestResumeBeforeRefreshAnswersRetryable503(t *testing.T) {
	f := newHandoffFixture(t)
	if rec := do(f.h, http.MethodGet, "/"); rec.Code != http.StatusOK || rec.Body.String() != "QA_A520_ROOT\n" {
		t.Fatalf("published: GET / => %d %q", rec.Code, rec.Body)
	}

	f.setSuspended(t, true)
	f.refresh(t)
	f.setSuspended(t, false) // resumed, not yet refreshed: ingress arrived first
	before := f.fetches()
	for _, path := range []string{"/", "/friendly", "/version.txt"} {
		assertTemporarilyUnavailable(t, "GET "+path, do(f.h, http.MethodGet, path), path == "/friendly")
	}
	if got := f.fetches(); got != before {
		t.Fatalf("origin fetches during handoff = %d, want none", got-before)
	}

	// A failed poll keeps the unavailable snapshot rather than dropping it.
	f.reader.fail = true
	if err := f.resolver.Refresh(t.Context()); err == nil {
		t.Fatal("refresh succeeded, want the staged error")
	}
	assertTemporarilyUnavailable(t, "after failed refresh", do(f.h, http.MethodGet, "/"), false)

	f.reader.fail = false
	f.refresh(t)
	rec := do(f.h, http.MethodGet, "/friendly")
	if rec.Code != http.StatusOK || rec.Body.String() != "QA_A520_VERSION\n" || rec.Header().Get("X-QA-A520") != "preserved" {
		t.Fatalf("resumed: GET /friendly => %d %q header %q", rec.Code, rec.Body, rec.Header().Get("X-QA-A520"))
	}
	if rec := do(f.h, http.MethodGet, "/"); rec.Code != http.StatusOK || rec.Body.String() != "QA_A520_ROOT\n" {
		t.Fatalf("resumed: GET / => %d %q", rec.Code, rec.Body)
	}
	if got := f.origin.gets[handoffPrefix+"index.html"]; got != 1 {
		t.Errorf("index fetches = %d, want the one cached publication", got)
	}
}

// Over a real server, HEAD carries the temporary status and headers and no
// body (a bare recorder would keep http.Error's bytes).
func TestUnavailableHEADOverHTTP(t *testing.T) {
	f := newHandoffFixture(t)
	f.setSuspended(t, true)
	f.refresh(t)
	srv := httptest.NewServer(f.h)
	defer srv.Close()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodHead, srv.URL+"/friendly", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Host = testHost
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusServiceUnavailable || len(body) != 0 {
		t.Fatalf("HEAD => %d with %d body bytes, want 503 without a body", resp.StatusCode, len(body))
	}
	if resp.Header.Get("Cache-Control") != "no-store" || resp.Header.Get("Retry-After") != "5" ||
		resp.Header.Get("X-QA-A520") != "preserved" {
		t.Fatalf("HEAD headers = %v", resp.Header)
	}
}

// A deleted site, or one never published, is genuinely absent: still 404.
func TestAbsentHostsStayNotFound(t *testing.T) {
	f := newHandoffFixture(t)
	f.setSuspended(t, true)
	f.refresh(t)
	if err := f.cl.Delete(t.Context(), f.app); err != nil {
		t.Fatal(err)
	}
	f.refresh(t)
	if rec := do(f.h, http.MethodGet, "/"); rec.Code != http.StatusNotFound {
		t.Fatalf("deleted: GET / => %d %q, want 404", rec.Code, rec.Body)
	}

	unpublished := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: "fresh", Namespace: "apps"},
		Spec:       appv1alpha1.AppSpec{Type: appv1alpha1.TypeStaticSite, Host: testHost, Suspended: true},
	}
	if err := f.cl.Create(t.Context(), unpublished); err != nil {
		t.Fatal(err)
	}
	f.refresh(t)
	if rec := do(f.h, http.MethodGet, "/"); rec.Code != http.StatusNotFound {
		t.Fatalf("suspended unpublished: GET / => %d %q, want 404", rec.Code, rec.Body)
	}
}
