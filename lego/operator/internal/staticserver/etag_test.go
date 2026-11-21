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
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"golang.org/x/sync/semaphore"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w4/341: the m160 default makes browsers revalidate every view, but with no
// validator a revalidation could never be a 304, so each one re-downloaded
// (and billed) the whole file. Every successful response now carries a tag
// over the bytes it represents: strong on br and on a file with one
// representation, weak on a compressible file's identity and gzip bodies, as
// on Render (w8/361).

// TestEveryServePathCarriesTheGeneratedETag covers the three serveObject
// callers: a direct object, a rewrite target and the SPA fallback. Each tags
// the bytes it served and answers its own tag with a bodyless 304 that
// carries the strong form.
func TestEveryServePathCarriesTheGeneratedETag(t *testing.T) {
	h, _ := newTestHandler(t, Site{
		Routes: []appv1alpha1.StaticRoute{{Type: "rewrite", Source: "/app", Destination: "/app.html"}},
	}, map[string]Object{
		key("index.html"): {Body: []byte("<h1>spa</h1>"), ContentType: "text/html"},
		key("app.html"):   {Body: []byte("<h1>app</h1>"), ContentType: "text/html"},
		key("site.css"):   {Body: []byte("body{}"), ContentType: "text/css"},
	})
	for _, tc := range []struct{ path, served, servedType string }{
		{"/site.css", "body{}", "text/css"},
		{"/app", "<h1>app</h1>", "text/html"},
		{"/dashboard/settings", "<h1>spa</h1>", "text/html"},
	} {
		rec := do(h, http.MethodGet, tc.path)
		strong := contentETag([]byte(tc.served), tc.servedType)
		if rec.Code != http.StatusOK || rec.Body.String() != tc.served || rec.Header().Get("ETag") != "W/"+strong {
			t.Fatalf("GET %s => %d %q ETag %q, want 200 %q ETag W/%s", tc.path, rec.Code, rec.Body, rec.Header().Get("ETag"), tc.served, strong)
		}
		// Render's 304 repeats the default policy (w8/358), as RFC 9110 asks.
		if again := doWith(h, http.MethodGet, tc.path, http.Header{"If-None-Match": {rec.Header().Get("ETag")}}); again.Code != http.StatusNotModified || again.Body.Len() != 0 ||
			again.Header().Get("Cache-Control") != renderCacheControl || again.Header().Get("ETag") != strong {
			t.Errorf("GET %s If-None-Match => %d (%d bytes) Cache-Control %q ETag %q, want bodyless 304 with %q and %s",
				tc.path, again.Code, again.Body.Len(), again.Header().Get("Cache-Control"), again.Header().Get("ETag"), renderCacheControl, strong)
		}
	}
}

// TestETagStrengthFollowsRender (w8/361): Render's edge sends a compressible
// file's tag strong with br and weak (W/) with gzip, identity, no
// Accept-Encoding and HEAD, keeps an image's tag strong for every client, and
// answers each revalidation, in any coding and with either form, with a
// bodyless 304 carrying the strong tag (captured 2026-11-21 on
// sveltekit-static, mentorship-guide and buzz-docs).
func TestETagStrengthFollowsRender(t *testing.T) {
	js := repeated("console.log('etag');\n")
	png := []byte("\x89PNG\r\n\x1a\nnot a real picture")
	h, _ := newTestHandler(t, Site{}, map[string]Object{
		key("app.js"):   {Body: js, ContentType: "text/javascript"},
		key("logo.png"): {Body: png, ContentType: "image/png"},
	})
	strong := contentETag(js, contentType("/app.js", "text/javascript"))
	for _, tc := range []struct{ method, offer, want string }{
		{http.MethodGet, "br", strong},
		{http.MethodGet, "gzip, br", strong},
		{http.MethodHead, "br", strong},
		{http.MethodGet, "gzip", "W/" + strong},
		{http.MethodGet, "identity", "W/" + strong},
		{http.MethodGet, "", "W/" + strong},
		{http.MethodHead, "", "W/" + strong},
	} {
		label := tc.method + " offered " + strconv.Quote(tc.offer)
		if rec := doWith(h, tc.method, "/app.js", offerHeader(tc.offer)); rec.Code != http.StatusOK || rec.Header().Get("ETag") != tc.want {
			t.Errorf("%s => %d ETag %q, want 200 ETag %s", label, rec.Code, rec.Header().Get("ETag"), tc.want)
		}
		for _, inm := range []string{strong, "W/" + strong} {
			rec := doWith(h, tc.method, "/app.js", offerHeader(tc.offer, "If-None-Match", inm))
			if rec.Code != http.StatusNotModified || rec.Body.Len() != 0 || rec.Header().Get("ETag") != strong ||
				rec.Header().Get("Content-Encoding") != "" || !hasVaryAcceptEncoding(rec.Header()) {
				t.Errorf("%s If-None-Match %s => %d (%d bytes) ETag %q CE %q Vary %v, want a bodyless 304 with Vary and ETag %s",
					label, inm, rec.Code, rec.Body.Len(), rec.Header().Get("ETag"), rec.Header().Get("Content-Encoding"), rec.Header().Values("Vary"), strong)
			}
		}
	}
	image := contentETag(png, contentType("/logo.png", "image/png"))
	for _, offer := range []string{"br", "gzip", ""} {
		if got := doWith(h, http.MethodGet, "/logo.png", offerHeader(offer)).Header().Get("ETag"); got != image {
			t.Errorf("PNG offered %q ETag = %q, want the strong %s", offer, got, image)
		}
		if rec := doWith(h, http.MethodGet, "/logo.png", offerHeader(offer, "If-None-Match", image)); rec.Code != http.StatusNotModified || rec.Header().Get("ETag") != image {
			t.Errorf("PNG offered %q revalidation => %d ETag %q, want 304 with %s", offer, rec.Code, rec.Header().Get("ETag"), image)
		}
	}
}

// TestRepublishChangesTheTagOnlyWhenBytesChange keeps m160's guarantee: after
// a republish with new bytes the old tag no longer matches, so the next
// revalidation returns them; identical bytes under a new revision keep the tag.
func TestRepublishChangesTheTagOnlyWhenBytesChange(t *testing.T) {
	origin := newFakeOrigin(map[string]Object{
		appID + "/rev-1/version.txt": {Body: []byte("v1"), ContentType: "text/plain"},
		appID + "/rev-2/version.txt": {Body: []byte("v2"), ContentType: "text/plain"},
		appID + "/rev-3/version.txt": {Body: []byte("v2"), ContentType: "text/plain"},
	})
	site := Site{AppID: appID, Revision: "rev-1"}
	h := New(publishResolver{site: &site}, origin, 1<<20)

	old := do(h, http.MethodGet, "/version.txt").Header().Get("ETag")
	site.Revision = "rev-2"
	rec := doWith(h, http.MethodGet, "/version.txt", http.Header{"If-None-Match": {old}})
	if rec.Code != http.StatusOK || rec.Body.String() != "v2" || rec.Header().Get("ETag") == old {
		t.Fatalf("old tag after a changed republish => %d %q ETag %q, want 200 \"v2\" with a new tag", rec.Code, rec.Body, rec.Header().Get("ETag"))
	}
	current := rec.Header().Get("ETag")
	site.Revision = "rev-3"
	if rec := doWith(h, http.MethodGet, "/version.txt", http.Header{"If-None-Match": {current}}); rec.Code != http.StatusNotModified {
		t.Errorf("same bytes under a new revision => %d, want 304", rec.Code)
	}
}

// TestRangeResumeFollowsTheTagsStrength: If-Range needs a strong tag (RFC 9110
// §13.1.5), so a resume holds where the tag is strong, on a file with one
// representation and on a small br body (Render sends a 206 of the br bytes,
// captured 2026-11-21, w8/361), and a stale tag gets the whole file. A
// compressible file's identity body sends the weak form and takes no ranges
// (w8/362), so If-Range with either form gets the whole identity file, and br
// bytes are never spliced onto identity bytes.
func TestRangeResumeFollowsTheTagsStrength(t *testing.T) {
	js := repeated("console.log('resume');\n")
	bin := repeated("\x00\x01resume")
	h, _ := newTestHandler(t, Site{}, map[string]Object{
		key("app.js"):   {Body: js, ContentType: "text/javascript"},
		key("data.bin"): {Body: bin, ContentType: "application/octet-stream"},
	})
	resume := func(path, offer, ifRange string) *httptest.ResponseRecorder {
		return doWith(h, http.MethodGet, path, http.Header{"Accept-Encoding": {offer}, "Range": {"bytes=0-9"}, "If-Range": {ifRange}})
	}

	binTag := contentETag(bin, contentType("/data.bin", "application/octet-stream"))
	if rec := resume("/data.bin", "br", binTag); rec.Code != http.StatusPartialContent || rec.Body.String() != string(bin[:10]) {
		t.Errorf("If-Range current tag => %d %q, want 206 of the first 10 bytes", rec.Code, rec.Body)
	}
	if rec := resume("/data.bin", "br", `"stale"`); rec.Code != http.StatusOK || rec.Body.Len() != len(bin) {
		t.Errorf("If-Range stale tag => %d (%d bytes), want 200 with the whole file", rec.Code, rec.Body.Len())
	}

	jsTag := contentETag(js, contentType("/app.js", "text/javascript"))
	full := doWith(h, http.MethodGet, "/app.js", brHeader("br"))
	part := resume("/app.js", "br", jsTag)
	if part.Code != http.StatusPartialContent || part.Header().Get("Content-Encoding") != codingBrotli ||
		!bytes.Equal(part.Body.Bytes(), full.Body.Bytes()[:10]) || part.Header().Get("Content-Range") != fmt.Sprintf("bytes 0-9/%d", full.Body.Len()) {
		t.Errorf("br If-Range br tag => %d CE %q Content-Range %q, want 206 of the first 10 br bytes",
			part.Code, part.Header().Get("Content-Encoding"), part.Header().Get("Content-Range"))
	}
	for _, ifRange := range []string{jsTag, "W/" + jsTag} {
		assertIdentity(t, "identity If-Range "+ifRange, resume("/app.js", "identity", ifRange), js)
	}
}

// TestShedResponsesCarryNoGeneratedETag: an unsatisfiable range and a request
// shed for live-body memory both go through serveObject and describe no stored
// representation, so neither may leak the generated tag. The range goes to a
// binary file, since a text file's identity body takes none (w8/362).
func TestShedResponsesCarryNoGeneratedETag(t *testing.T) {
	h, _ := newTestHandler(t, Site{}, map[string]Object{
		key("version.txt"): {Body: []byte(versionBytes), ContentType: "text/plain"},
		key("data.bin"):    {Body: []byte(versionBytes), ContentType: "application/octet-stream"},
	})
	rec := doWith(h, http.MethodGet, "/data.bin", http.Header{"Range": {"bytes=999-"}})
	if rec.Code != http.StatusRequestedRangeNotSatisfiable || rec.Header().Get("ETag") != "" {
		t.Errorf("unsatisfiable range => %d ETag %q, want 416 with no ETag", rec.Code, rec.Header().Get("ETag"))
	}

	h.liveBodies = semaphore.NewWeighted(1)
	rec = do(h, http.MethodGet, "/version.txt")
	if rec.Code != http.StatusServiceUnavailable || rec.Header().Get("ETag") != "" {
		t.Errorf("live-body shed => %d ETag %q, want 503 with no ETag", rec.Code, rec.Header().Get("ETag"))
	}
}
