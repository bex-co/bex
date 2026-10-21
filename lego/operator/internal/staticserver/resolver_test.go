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
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

// A newer release does not own the serving prefix. Suspension must withhold even
// cached published content, and resumption must restore that same publication
// without waiting for the newer build to finish or succeed.
func TestPublishedSiteSuspendResumeWhileNewerBuildHeld(t *testing.T) {
	for _, tc := range []struct {
		name          string
		phase         appv1alpha1.AppPhase
		conditionType string
		reason        string
	}{
		{"failed", appv1alpha1.PhaseRunning, appv1alpha1.ConditionBuild, appv1alpha1.ReasonBuildFailedUserError},
		{"building", appv1alpha1.PhaseBuilding, appv1alpha1.ConditionReady, appv1alpha1.ReasonBuilding},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := t.Context()
			scheme := runtime.NewScheme()
			if err := appv1alpha1.AddToScheme(scheme); err != nil {
				t.Fatal(err)
			}
			const publishedPrefix = "tea-aaaaaaaaaaaaaaaaaaaa/mysite/rev-3/"
			const pendingPrefix = "tea-aaaaaaaaaaaaaaaaaaaa/mysite/rev-4/"
			app := &appv1alpha1.App{
				ObjectMeta: metav1.ObjectMeta{Name: appID, Namespace: "apps", Generation: 4},
				Spec: appv1alpha1.AppSpec{
					Type: appv1alpha1.TypeStaticSite,
					Host: testHost,
				},
				Status: appv1alpha1.AppStatus{
					Phase:             tc.phase,
					ActiveRevision:    "rev-3",
					StaticPrefix:      publishedPrefix,
					ReleaseGeneration: 4,
					Conditions: []metav1.Condition{{
						Type: tc.conditionType, Status: metav1.ConditionFalse,
						Reason: tc.reason, ObservedGeneration: 4,
					}},
				},
			}
			cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(app).
				WithStatusSubresource(&appv1alpha1.App{}).Build()
			if err := cl.Get(ctx, client.ObjectKeyFromObject(app), app); err != nil {
				t.Fatal(err)
			}
			resolver := NewCachedResolver(cl, app.Namespace, "onbex.co")
			origin := newFakeOrigin(map[string]Object{
				publishedPrefix + "index.html": {Body: []byte("published page"), ContentType: "text/html"},
				publishedPrefix + "app.js":     {Body: []byte("published asset")},
				pendingPrefix + "index.html":   {Body: []byte("unpublished page"), ContentType: "text/html"},
				pendingPrefix + "app.js":       {Body: []byte("unpublished asset")},
			})
			h := New(resolver, origin, 1<<20)
			if err := resolver.Refresh(ctx); err != nil {
				t.Fatal(err)
			}
			if rec := do(h, http.MethodGet, "/"); rec.Code != http.StatusOK || rec.Body.String() != "published page" {
				t.Fatalf("before suspend: GET / => %d %q, want published page", rec.Code, rec.Body)
			}

			app.Spec.Suspended = true
			app.Generation++
			if err := cl.Update(ctx, app); err != nil {
				t.Fatal(err)
			}
			if err := resolver.Refresh(ctx); err != nil {
				t.Fatal(err)
			}
			for _, path := range []string{"/", "/app.js"} {
				if rec := do(h, http.MethodGet, path); rec.Code != http.StatusServiceUnavailable || strings.Contains(rec.Body.String(), "published") {
					t.Fatalf("suspended: GET %s => %d %q, want a temporary 503 without content", path, rec.Code, rec.Body)
				}
			}
			if got := origin.gets[publishedPrefix+"app.js"]; got != 0 {
				t.Fatalf("suspended origin fetches = %d, want 0", got)
			}

			app.Spec.Suspended = false
			app.Generation++
			if err := cl.Update(ctx, app); err != nil {
				t.Fatal(err)
			}
			if err := resolver.Refresh(ctx); err != nil {
				t.Fatal(err)
			}
			for _, request := range []struct{ path, body string }{{"/", "published page"}, {"/app.js", "published asset"}} {
				if rec := do(h, http.MethodGet, request.path); rec.Code != http.StatusOK || rec.Body.String() != request.body {
					t.Fatalf("resumed: GET %s => %d %q, want %q", request.path, rec.Code, rec.Body, request.body)
				}
			}
			if got := origin.gets[publishedPrefix+"index.html"]; got != 1 {
				t.Errorf("published index fetches = %d, want one cached publication", got)
			}
			for _, path := range []string{"index.html", "app.js"} {
				if got := origin.gets[pendingPrefix+path]; got != 0 {
					t.Errorf("unpublished %s fetches = %d, want 0", path, got)
				}
			}
		})
	}
}

func TestStaticServePrefixRecordedThenLegacy(t *testing.T) {
	app := &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{Name: "web"}}
	app.Status.ActiveRevision = "rev-1"
	if got, want := staticServePrefix(app), "web/rev-1/"; got != want {
		t.Errorf("legacy fallback = %q, want %q", got, want)
	}
	app.Labels = map[string]string{"app.bex.co/workspace": "tea-aaaaaaaaaaaaaaaaaaaa"}
	if got, want := staticServePrefix(app), "web/rev-1/"; got != want {
		t.Errorf("labeled but empty status still dual-reads legacy = %q, want %q", got, want)
	}
	app.Status.StaticPrefix = "tea-aaaaaaaaaaaaaaaaaaaa/web/rev-1/"
	if got, want := staticServePrefix(app), "tea-aaaaaaaaaaaaaaaaaaaa/web/rev-1/"; got != want {
		t.Errorf("recorded prefix = %q, want %q", got, want)
	}
}

func TestIsLegacyStaticPrefix(t *testing.T) {
	cases := []struct {
		name string
		site Site
		want bool
	}{
		{"explicit legacy", Site{AppID: "web", Prefix: "web/rev-1/"}, true},
		{"empty prefix synthesizes legacy", Site{AppID: "web", Revision: "rev-1"}, true},
		{"scoped prefix", Site{AppID: "web", Prefix: "tea-aaaaaaaaaaaaaaaaaaaa/web/rev-1/"}, false},
		{"missing app", Site{Prefix: "web/rev-1/"}, false},
	}
	for _, tc := range cases {
		if got := isLegacyStaticPrefix(tc.site); got != tc.want {
			t.Errorf("%s: got %v, want %v", tc.name, got, tc.want)
		}
	}
}

// Header snapshots change independently of the immutable publication and its cache.
func TestHeaderSnapshotRefreshPreservesPublicationAcrossHosts(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := appv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	app := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: appID, Namespace: "apps"},
		Spec: appv1alpha1.AppSpec{Type: appv1alpha1.TypeStaticSite, Host: testHost,
			Hosts: []string{"custom.example.test"}, Headers: []appv1alpha1.StaticHeader{{Path: "/*.css", Name: "X-Policy", Value: "before"}}},
		Status: appv1alpha1.AppStatus{ActiveRevision: rev, StaticPrefix: appID + "/" + rev + "/"},
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(app).WithStatusSubresource(app).Build()
	resolver := NewCachedResolver(cl, app.Namespace, "onbex.co")
	origin := newFakeOrigin(map[string]Object{key("theme.css"): {Body: []byte("original"), ContentType: "text/css"}})
	h := New(resolver, origin, 1<<20)
	assertPolicy := func(want string) {
		t.Helper()
		for _, host := range []string{testHost, "custom.example.test"} {
			for _, method := range []string{http.MethodGet, http.MethodHead} {
				rec := httptest.NewRecorder()
				h.ServeHTTP(rec, httptest.NewRequest(method, "https://"+host+"/theme.css", nil))
				if rec.Code != 200 || rec.Header().Get("X-Policy") != want || rec.Header().Get("Content-Type") != "text/css" {
					t.Fatalf("%s %s: %d %v", method, host, rec.Code, rec.Header())
				}
				if method == http.MethodGet && rec.Body.String() != "original" {
					t.Fatal("publication changed")
				}
			}
		}
	}
	if err := resolver.Refresh(t.Context()); err != nil {
		t.Fatal(err)
	}
	assertPolicy("before")
	if err := cl.Get(t.Context(), client.ObjectKeyFromObject(app), app); err != nil {
		t.Fatal(err)
	}
	app.Spec.Headers[0].Value = "after"
	if err := cl.Update(t.Context(), app); err != nil {
		t.Fatal(err)
	}
	assertPolicy("before")
	if err := resolver.Refresh(t.Context()); err != nil {
		t.Fatal(err)
	}
	assertPolicy("after")
	if origin.gets[key("theme.css")] != 1 {
		t.Fatal("configuration change refetched published object")
	}
	if err := cl.Get(t.Context(), client.ObjectKeyFromObject(app), app); err != nil {
		t.Fatal(err)
	}
	if app.Status.ActiveRevision != rev || app.Status.StaticPrefix != appID+"/"+rev+"/" {
		t.Fatal("header save changed publication identity")
	}
}

// TestRunReloadsOnAnAppChange is w4/m235: a first publish's site must be
// servable as soon as its active revision is recorded, before the operator
// hands it the host, not one resync later.
func TestRunReloadsOnAnAppChange(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := appv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	app := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: appID, Namespace: "apps"},
		Spec:       appv1alpha1.AppSpec{Type: appv1alpha1.TypeStaticSite, Host: testHost},
	}
	var lists atomic.Int64
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(app).WithStatusSubresource(app).
		WithInterceptorFuncs(interceptor.Funcs{List: func(ctx context.Context, c client.WithWatch, list client.ObjectList, opts ...client.ListOption) error {
			defer lists.Add(1)
			return c.List(ctx, list, opts...)
		}}).Build()
	resolver := NewCachedResolver(cl, app.Namespace, "onbex.co")
	changed := make(chan struct{}, 1)
	go resolver.Run(t.Context(), time.Hour, changed, nil)
	for lists.Load() == 0 {
		time.Sleep(time.Millisecond)
	}
	app.Status.ActiveRevision = rev
	if err := cl.Status().Update(t.Context(), app); err != nil {
		t.Fatal(err)
	}
	changed <- struct{}{}
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, ok := resolver.Resolve(testHost); ok {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("the published site never resolved after the change signal")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// A replica is Ready as soon as its port listens (TCP probe), so Start must
// return only after the first snapshot exists: a live host resolves the moment
// Start returns, with no watch signal and no backstop tick ever firing (w3/m175).
func TestStartServesLiveHostsBeforeAnyBackgroundRefresh(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := appv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	app := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: appID, Namespace: "apps"},
		Spec:       appv1alpha1.AppSpec{Type: appv1alpha1.TypeStaticSite, Host: testHost},
		Status:     appv1alpha1.AppStatus{ActiveRevision: rev, StaticPrefix: appID + "/" + rev + "/"},
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(app).WithStatusSubresource(app).Build()
	resolver := NewCachedResolver(cl, app.Namespace, "onbex.co")
	never := make(chan struct{})
	if err := resolver.Start(t.Context(), time.Hour, never, func(err error) { t.Error(err) }); err != nil {
		t.Fatal(err)
	}
	if _, ok := resolver.Resolve(testHost); !ok {
		t.Fatalf("%s is unknown right after Start: a fresh replica would 404 live sites until its first background refresh", testHost)
	}
}

// A first snapshot that cannot be built must stop startup rather than open the
// listener with an empty host map.
func TestStartFailsWhenTheFirstSnapshotCannotBeBuilt(t *testing.T) {
	cl := fake.NewClientBuilder().WithScheme(runtime.NewScheme()).Build() // App kind unregistered: List fails
	resolver := NewCachedResolver(cl, "apps", "onbex.co")
	if err := resolver.Start(t.Context(), time.Hour, make(chan struct{}), nil); err == nil {
		t.Fatal("Start succeeded without a snapshot")
	}
}
