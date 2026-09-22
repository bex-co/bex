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
	"net/http"
	"testing"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// A newer release does not own the serving prefix. Suspension must remove even
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
				if rec := do(h, http.MethodGet, path); rec.Code != http.StatusNotFound {
					t.Fatalf("suspended: GET %s => %d %q, want no serving route", path, rec.Code, rec.Body)
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
