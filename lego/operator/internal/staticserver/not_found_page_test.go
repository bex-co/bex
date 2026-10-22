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
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w8/183: a miss answers 404 with the site's own 404.html (plain 404 without
// one), and client-side routing is an explicit `/* → /index.html` rewrite.
func TestMissServes404HTMLWithStatus404(t *testing.T) {
	site := Site{
		NoSPAFallback: true,
		Headers:       []appv1alpha1.StaticHeader{{Path: "/*", Name: "X-Site", Value: "yes"}},
	}
	h, _ := newTestHandler(t, site, map[string]Object{
		key("index.html"): {Body: []byte("HOME"), ContentType: "text/html"},
		key("404.html"):   {Body: []byte("custom 404"), ContentType: "text/html"},
	})
	for _, p := range []string{"/no-such-page", "/blog/missing-post", "/missing.css", "/images/nope.png"} {
		rec := do(h, http.MethodGet, p)
		if rec.Code != http.StatusNotFound || rec.Body.String() != "custom 404" {
			t.Errorf("GET %s => %d %q, want 404 custom 404", p, rec.Code, rec.Body)
		}
		if rec.Header().Get("X-Site") != "yes" || rec.Header().Get("Etag") != "" {
			t.Errorf("GET %s headers = %v, want custom error headers and no validator", p, rec.Header())
		}
	}
	head := do(h, http.MethodHead, "/no-such-page")
	if head.Code != http.StatusNotFound || head.Body.Len() != 0 || head.Header().Get("Content-Length") != "10" {
		t.Errorf("HEAD => %d body %q Content-Length %q", head.Code, head.Body, head.Header().Get("Content-Length"))
	}
	// The published page itself is an ordinary file.
	if rec := do(h, http.MethodGet, "/404.html"); rec.Code != http.StatusOK || rec.Body.String() != "custom 404" {
		t.Errorf("GET /404.html => %d %q, want 200", rec.Code, rec.Body)
	}
	if rec := do(h, http.MethodGet, "/"); rec.Code != http.StatusOK || rec.Body.String() != "HOME" {
		t.Errorf("GET / => %d %q", rec.Code, rec.Body)
	}
}

func TestMissWithoutA404PageIsAPlain404(t *testing.T) {
	h, _ := newTestHandler(t, Site{NoSPAFallback: true}, map[string]Object{
		key("index.html"): {Body: []byte("HOME"), ContentType: "text/html"},
	})
	for _, p := range []string{"/no-such-page", "/missing.css"} {
		if rec := do(h, http.MethodGet, p); rec.Code != http.StatusNotFound || rec.Body.String() == "HOME" {
			t.Errorf("GET %s => %d %q, want a plain 404", p, rec.Code, rec.Body)
		}
	}
}

// A catch-all rewrite is how a Render site opts into client-side routing.
func TestSPARewriteStillServesTheIndex(t *testing.T) {
	h, _ := newTestHandler(t, Site{
		NoSPAFallback: true,
		Routes:        []appv1alpha1.StaticRoute{{Type: "rewrite", Source: "/*", Destination: "/index.html"}},
	}, map[string]Object{
		key("index.html"): {Body: []byte("HOME"), ContentType: "text/html"},
		key("404.html"):   {Body: []byte("custom 404"), ContentType: "text/html"},
	})
	if rec := do(h, http.MethodGet, "/deep/link"); rec.Code != http.StatusOK || rec.Body.String() != "HOME" {
		t.Errorf("GET /deep/link => %d %q, want the index with 200", rec.Code, rec.Body)
	}
}

// Sites created before the cutoff keep the implicit fallback; later ones get
// Render's miss handling.
func TestResolverGivesNewSitesRendersMissHandling(t *testing.T) {
	before := SPAFallbackCutoff.Add(-time.Hour)
	after := SPAFallbackCutoff.Add(time.Hour)
	for name, tc := range map[string]struct {
		created time.Time
		want    bool
	}{"existing site": {before, false}, "new site": {after, true}, "unknown creation": {time.Time{}, false}} {
		app := &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: "docs", Namespace: "bex-system", CreationTimestamp: metav1.NewTime(tc.created)},
			Spec:       appv1alpha1.AppSpec{Type: appv1alpha1.TypeStaticSite, Host: "docs.onbex.co"},
			Status:     appv1alpha1.AppStatus{ActiveRevision: "rev-1", URL: "https://docs.onbex.co"},
		}
		scheme := runtime.NewScheme()
		if err := appv1alpha1.AddToScheme(scheme); err != nil {
			t.Fatal(err)
		}
		cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(app).Build()
		resolver := NewCachedResolver(cl, "bex-system", "onbex.co")
		if err := resolver.Refresh(context.Background()); err != nil {
			t.Fatal(err)
		}
		site, ok := resolver.Resolve("docs.onbex.co")
		if !ok || site.NoSPAFallback != tc.want {
			t.Errorf("%s: NoSPAFallback = %v (found %v), want %v", name, site.NoSPAFallback, ok, tc.want)
		}
	}
}
