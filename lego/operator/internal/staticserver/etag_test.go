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

	"golang.org/x/sync/semaphore"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w4/341: the m160 default makes browsers revalidate every view, but with no
// validator a revalidation could never be a 304, so each one re-downloaded
// (and billed) the whole file. Every successful response now carries a strong
// tag over the bytes it represents.

// TestEveryServePathCarriesTheGeneratedETag covers the three serveObject
// callers: a direct object, a rewrite target and the SPA fallback. Each tags
// the bytes it served and answers its own tag with a bodyless 304.
func TestEveryServePathCarriesTheGeneratedETag(t *testing.T) {
	h, _ := newTestHandler(t, Site{
		Routes: []appv1alpha1.StaticRoute{{Type: "rewrite", Source: "/app", Destination: "/app.html"}},
	}, map[string]Object{
		key("index.html"): {Body: []byte("<h1>spa</h1>"), ContentType: "text/html"},
		key("app.html"):   {Body: []byte("<h1>app</h1>"), ContentType: "text/html"},
		key("site.css"):   {Body: []byte("body{}"), ContentType: "text/css"},
	})
	for _, tc := range []struct{ path, served string }{
		{"/site.css", "body{}"},
		{"/app", "<h1>app</h1>"},
		{"/dashboard/settings", "<h1>spa</h1>"},
	} {
		rec := do(h, http.MethodGet, tc.path)
		want := contentETag([]byte(tc.served))
		if rec.Code != http.StatusOK || rec.Body.String() != tc.served || rec.Header().Get("ETag") != want {
			t.Fatalf("GET %s => %d %q ETag %q, want 200 %q ETag %q", tc.path, rec.Code, rec.Body, rec.Header().Get("ETag"), tc.served, want)
		}
		if again := doWith(h, http.MethodGet, tc.path, http.Header{"If-None-Match": {want}}); again.Code != http.StatusNotModified || again.Body.Len() != 0 {
			t.Errorf("GET %s If-None-Match => %d (%d bytes), want bodyless 304", tc.path, again.Code, again.Body.Len())
		}
	}
}

// TestBrotliAndIdentityValidateAgainstEachOther: the br representation carries
// the weak form of the identity tag, and If-None-Match uses weak comparison,
// so either tag revalidates either representation.
func TestBrotliAndIdentityValidateAgainstEachOther(t *testing.T) {
	body := repeated("console.log('etag');\n")
	h, _ := newTestHandler(t, Site{}, map[string]Object{key("app.js"): {Body: body, ContentType: "text/javascript"}})
	strong := contentETag(body)

	br := doWith(h, http.MethodGet, "/app.js", brHeader("br"))
	assertBrotli(t, "br", br, body)
	if got := br.Header().Get("ETag"); got != "W/"+strong {
		t.Fatalf("br ETag = %q, want W/%s", got, strong)
	}
	for label, header := range map[string]http.Header{
		"br with identity tag": {"Accept-Encoding": {"br"}, "If-None-Match": {strong}},
		"identity with br tag": {"If-None-Match": {"W/" + strong}},
	} {
		if rec := doWith(h, http.MethodGet, "/app.js", header); rec.Code != http.StatusNotModified {
			t.Errorf("%s => %d, want 304", label, rec.Code)
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

// TestRangeResumeUsesTheGeneratedETag: If-Range with the current tag resumes
// the identity bytes; a stale tag gets the whole file. The br representation's
// weak tag never satisfies If-Range's strong comparison, so encoded bytes are
// never spliced onto identity bytes.
func TestRangeResumeUsesTheGeneratedETag(t *testing.T) {
	body := repeated("console.log('resume');\n")
	h, _ := newTestHandler(t, Site{}, map[string]Object{key("app.js"): {Body: body, ContentType: "text/javascript"}})
	tag := contentETag(body)

	if rec := doWith(h, http.MethodGet, "/app.js", http.Header{"Range": {"bytes=0-9"}, "If-Range": {tag}}); rec.Code != http.StatusPartialContent || rec.Body.String() != string(body[:10]) {
		t.Errorf("If-Range current tag => %d %q, want 206 of the first 10 bytes", rec.Code, rec.Body)
	}
	if rec := doWith(h, http.MethodGet, "/app.js", http.Header{"Range": {"bytes=0-9"}, "If-Range": {`"stale"`}}); rec.Code != http.StatusOK || rec.Body.Len() != len(body) {
		t.Errorf("If-Range stale tag => %d (%d bytes), want 200 with the whole file", rec.Code, rec.Body.Len())
	}
	br := doWith(h, http.MethodGet, "/app.js", http.Header{"Accept-Encoding": {"br"}, "Range": {"bytes=0-9"}, "If-Range": {tag}})
	if br.Code != http.StatusOK {
		t.Fatalf("br If-Range identity tag => %d, want a full 200", br.Code)
	}
	assertBrotli(t, "br If-Range", br, body)
}

// TestShedResponsesCarryNoGeneratedETag: an unsatisfiable range and a request
// shed for live-body memory both go through serveObject and describe no stored
// representation, so neither may leak the generated tag.
func TestShedResponsesCarryNoGeneratedETag(t *testing.T) {
	h, _ := newTestHandler(t, Site{}, map[string]Object{key("version.txt"): {Body: []byte(versionBytes), ContentType: "text/plain"}})
	rec := doWith(h, http.MethodGet, "/version.txt", http.Header{"Range": {"bytes=999-"}})
	if rec.Code != http.StatusRequestedRangeNotSatisfiable || rec.Header().Get("ETag") != "" {
		t.Errorf("unsatisfiable range => %d ETag %q, want 416 with no ETag", rec.Code, rec.Header().Get("ETag"))
	}

	h.liveBodies = semaphore.NewWeighted(1)
	rec = do(h, http.MethodGet, "/version.txt")
	if rec.Code != http.StatusServiceUnavailable || rec.Header().Get("ETag") != "" {
		t.Errorf("live-body shed => %d ETag %q, want 503 with no ETag", rec.Code, rec.Header().Get("ETag"))
	}
}
