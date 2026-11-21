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
	"bytes"
	"context"
	"net/http"
	"strconv"
	"strings"
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
		// The page goes out with a published file's headers, as on Render
		// (w8/322), its own ETag included (w8/360), weak on identity (w8/361).
		hdr := rec.Header()
		if hdr.Get("X-Site") != "yes" || hdr.Get("Etag") != "W/"+contentETag([]byte("custom 404"), "text/html") || hdr.Get("Cache-Control") != renderCacheControl ||
			hdr.Get("Strict-Transport-Security") != defaultStrictTransportSecurity || hdr.Get("X-Content-Type-Options") != "nosniff" {
			t.Errorf("GET %s headers = %v, want the site's rules, HSTS, the cache policy, nosniff and the page's ETag", p, hdr)
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

// The 404.html page takes the site's rules as a file does: one replaces its
// cache policy, HSTS or type, and its length stays its own.
func TestThe404PageTakesTheSitesRulesAsAFileDoes(t *testing.T) {
	h, _ := newTestHandler(t, Site{NoSPAFallback: true, Headers: []appv1alpha1.StaticHeader{
		{Path: "/*", Name: "Cache-Control", Value: "no-store"},
		{Path: "/*", Name: "Strict-Transport-Security", Value: "max-age=60"},
		{Path: "/*", Name: "Content-Type", Value: "text/plain; charset=utf-8"},
		{Path: "/*", Name: "Content-Length", Value: "999"},
	}}, map[string]Object{key("404.html"): {Body: []byte("custom 404"), ContentType: "text/html"}})
	rec := do(h, http.MethodGet, "/missing")
	hdr := rec.Header()
	if rec.Code != http.StatusNotFound || rec.Body.String() != "custom 404" || hdr.Get("Cache-Control") != "no-store" ||
		hdr.Get("Strict-Transport-Security") != "max-age=60" || hdr.Get("Content-Type") != "text/plain; charset=utf-8" || hdr.Get("Content-Length") != "10" {
		t.Fatalf("miss = %d %q %v, want the page with the rules' cache policy, HSTS and type, and its own length", rec.Code, rec.Body, hdr)
	}
	// Its tag follows the rule's type, as the file's does (w5/182, w8/360),
	// in the weak form identity sends (w8/361).
	retyped := "W/" + retypedETag(contentETag([]byte("custom 404"), "text/html"), "text/plain; charset=utf-8")
	if file := do(h, http.MethodGet, "/404.html"); hdr.Get("Etag") != retyped || file.Header().Get("Etag") != retyped {
		t.Fatalf("miss ETag %q, file ETag %q, want both the retyped tag %q", hdr.Get("Etag"), file.Header().Get("Etag"), retyped)
	}
}

// Render sends a 404.html miss the page's own ETag, the tag a fetch of
// /404.html gets in the same coding, and answers the full 404 whatever the
// request's preconditions or Range say: never 304, 412 or 206 (captured
// 2026-11-20 on mentorship-guide, buzz-docs, breadcrumbs-wiki and wc3v,
// w8/360). The page goes identity to a gzip-only client (w8/359), so that
// client gets identity's tag.
func TestThe404PageSendsItsOwnETagAndStaysA404(t *testing.T) {
	page := repeated("<p>custom 404</p>\n")
	h, _ := newTestHandler(t, Site{NoSPAFallback: true}, map[string]Object{key("404.html"): {Body: page, ContentType: "text/html"}})
	for offer, fileOffer := range map[string]string{"": "", "gzip": "", "gzip, br": "gzip, br"} {
		file := doWith(h, http.MethodGet, "/404.html", brHeader(fileOffer))
		tag, coding := file.Header().Get("Etag"), file.Header().Get("Content-Encoding")
		if file.Code != http.StatusOK || tag == "" {
			t.Fatalf("GET /404.html offered %q => %d ETag %q, want 200 with a tag", fileOffer, file.Code, tag)
		}
		// Render's page tag is strong on br and weak otherwise (w8/360, w8/361).
		if weak := strings.HasPrefix(tag, "W/"); weak == (coding == codingBrotli) {
			t.Fatalf("GET /404.html offered %q ETag %q with coding %q, want strong on br and weak otherwise", fileOffer, tag, coding)
		}
		for name, cond := range map[string]http.Header{
			"plain":                  {},
			"matching If-None-Match": {"If-None-Match": {tag}},
			"If-None-Match *":        {"If-None-Match": {"*"}},
			"foreign If-Match":       {"If-Match": {`"qa-nope"`}},
			"Range":                  {"Range": {"bytes=0-9"}},
		} {
			for _, method := range []string{http.MethodGet, http.MethodHead} {
				req := cond.Clone()
				req.Set("Accept-Encoding", offer)
				rec := doWith(h, method, "/qa-nope-z", req)
				got := rec.Header()
				if rec.Code != http.StatusNotFound || got.Get("Etag") != tag || got.Get("Content-Encoding") != coding ||
					got.Get("Content-Range") != "" || got.Get("Accept-Ranges") != "" {
					t.Fatalf("%s %s offered %q => %d ETag %q CE %q Content-Range %q, want the 404 with the file's tag %q and coding %q",
						method, name, offer, rec.Code, got.Get("Etag"), got.Get("Content-Encoding"), got.Get("Content-Range"), tag, coding)
				}
				want := file.Body.Bytes()
				if method == http.MethodHead {
					want = nil
				}
				if !bytes.Equal(rec.Body.Bytes(), want) || got.Get("Content-Length") != strconv.Itoa(file.Body.Len()) {
					t.Fatalf("%s %s offered %q => %d body bytes, Content-Length %q, want the file's %d", method, name, offer, rec.Body.Len(), got.Get("Content-Length"), file.Body.Len())
				}
			}
		}
	}
	// A bare 404 sends no tag, whatever its preconditions (Render, w8/360).
	bare, _ := newTestHandler(t, Site{NoSPAFallback: true}, nil)
	for _, offer := range []string{"", "gzip, br"} {
		if rec := doWith(bare, http.MethodGet, "/qa-nope-y", http.Header{"Accept-Encoding": {offer}, "If-None-Match": {"*"}}); rec.Code != http.StatusNotFound || rec.Header().Get("Etag") != "" {
			t.Fatalf("bare miss offered %q => %d ETag %q, want 404 with no tag", offer, rec.Code, rec.Header().Get("Etag"))
		}
	}
}

// Render answers a miss on a site without a 404.html with a bare plain-text
// 404: nosniff only, none of the site's rules (w8/322).
func TestMissWithoutA404PageIsAPlain404(t *testing.T) {
	h, _ := newTestHandler(t, Site{NoSPAFallback: true, Headers: markerHeaders()}, map[string]Object{
		key("index.html"): {Body: []byte("HOME"), ContentType: "text/html"},
	})
	for _, p := range []string{"/no-such-page", "/missing.css"} {
		assertBareNotFound(t, do(h, http.MethodGet, p))
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
