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
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w4/272: a saved ETag rule was emitted, but the handler answered every
// revalidation with 200 and the full body, and a failed If-Match with 200.
const (
	savedETag    = `"qa-v1"`
	savedCache   = "public, max-age=60"
	versionBytes = "qa-a277-version-1\n"
)

// conditionalSite publishes version.txt directly, /v rewritten to it, and an
// SPA index served by the implicit fallback — the three success paths.
func conditionalSite(t *testing.T) *Handler {
	t.Helper()
	h, _ := newTestHandler(t, Site{
		Headers: []appv1alpha1.StaticHeader{
			{Path: "/*", Name: "ETag", Value: savedETag},
			{Path: "/*", Name: "Cache-Control", Value: savedCache},
		},
		Routes: []appv1alpha1.StaticRoute{{Type: "rewrite", Source: "/v", Destination: "/version.txt"}},
	}, map[string]Object{
		key("version.txt"): {Body: []byte(versionBytes), ContentType: "text/plain"},
		key("index.html"):  {Body: []byte(versionBytes), ContentType: "text/html"},
	})
	return h
}

func TestSavedETagPreconditions(t *testing.T) {
	h := conditionalSite(t)
	cases := []struct {
		name   string
		header string
		value  string
		want   int
	}{
		{"no precondition", "", "", http.StatusOK},
		{"matching If-None-Match", "If-None-Match", savedETag, http.StatusNotModified},
		{"weak matching If-None-Match", "If-None-Match", "W/" + savedETag, http.StatusNotModified},
		{"If-None-Match list member", "If-None-Match", `"other", ` + savedETag, http.StatusNotModified},
		{"If-None-Match wildcard", "If-None-Match", "*", http.StatusNotModified},
		{"mismatching If-None-Match", "If-None-Match", `"other"`, http.StatusOK},
		{"matching If-Match", "If-Match", savedETag, http.StatusOK},
		{"mismatching If-Match", "If-Match", `"other"`, http.StatusPreconditionFailed},
		{"weak If-Match against strong tag", "If-Match", "W/" + savedETag, http.StatusPreconditionFailed},
	}
	for _, path := range []string{"/version.txt", "/v", "/client/route"} {
		for _, method := range []string{http.MethodGet, http.MethodHead} {
			for _, tc := range cases {
				header := http.Header{}
				if tc.header != "" {
					header.Set(tc.header, tc.value)
				}
				rec := doWith(h, method, path, header)
				label := method + " " + path + " " + tc.name
				if rec.Code != tc.want {
					t.Errorf("%s => %d, want %d", label, rec.Code, tc.want)
					continue
				}
				wantBody := ""
				if tc.want == http.StatusOK && method == http.MethodGet {
					wantBody = versionBytes
				}
				if got := rec.Body.String(); got != wantBody {
					t.Errorf("%s body = %q, want %q", label, got, wantBody)
				}
				if tc.want == http.StatusPreconditionFailed {
					continue
				}
				// The validator and the owner's freshness policy describe a
				// 304 as much as a 200 (RFC 9110 §15.4.5).
				if got := rec.Header().Get("ETag"); got != savedETag {
					t.Errorf("%s ETag = %q, want %q", label, got, savedETag)
				}
				if got := rec.Header().Get("Cache-Control"); got != savedCache {
					t.Errorf("%s Cache-Control = %q, want %q", label, got, savedCache)
				}
				if got := rec.Header().Get("Last-Modified"); got != "" {
					t.Errorf("%s invented Last-Modified %q", label, got)
				}
			}
		}
	}
}

// Without a saved ETag there is no validator to match, except the wildcard,
// which any existing object satisfies; no ETag is generated.
func TestNoSavedETagOnlyWildcardMatches(t *testing.T) {
	h, _ := newTestHandler(t, Site{}, map[string]Object{
		key("version.txt"): {Body: []byte(versionBytes), ContentType: "text/plain"},
	})
	if rec := doWith(h, http.MethodGet, "/version.txt", http.Header{"If-None-Match": {`"qa-v1"`}}); rec.Code != http.StatusOK || rec.Body.String() != versionBytes {
		t.Errorf("tagless If-None-Match => %d %q, want 200 body", rec.Code, rec.Body)
	}
	rec := doWith(h, http.MethodGet, "/version.txt", http.Header{"If-None-Match": {"*"}})
	if rec.Code != http.StatusNotModified {
		t.Errorf("If-None-Match * on existing object => %d, want 304", rec.Code)
	}
	if got := rec.Header().Get("ETag"); got != "" {
		t.Errorf("generated ETag %q, want none", got)
	}
	if rec := doWith(h, http.MethodGet, "/missing.txt", http.Header{"If-None-Match": {"*"}}); rec.Code != http.StatusNotFound {
		t.Errorf("If-None-Match * on a miss => %d, want 404", rec.Code)
	}
}

// Over a real connection: a 304 carries no body bytes and a 200 is framed with
// the object's exact length; a byte range is served from the same object.
func TestConditionalFramingOverHTTP(t *testing.T) {
	server := httptest.NewServer(conditionalSite(t))
	defer server.Close()

	get := func(header http.Header) (*http.Response, string) {
		t.Helper()
		req, err := http.NewRequest(http.MethodGet, server.URL+"/version.txt", nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Host = testHost
		req.Header = header
		req.Header.Set("Accept-Encoding", "identity")
		res, err := http.DefaultTransport.RoundTrip(req)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = res.Body.Close() }()
		body, err := io.ReadAll(res.Body)
		if err != nil {
			t.Fatal(err)
		}
		return res, string(body)
	}

	res, body := get(http.Header{})
	if res.StatusCode != http.StatusOK || body != versionBytes || res.ContentLength != int64(len(versionBytes)) {
		t.Fatalf("plain GET => %d len %d %q, want 200 framed body", res.StatusCode, res.ContentLength, body)
	}
	res, body = get(http.Header{"If-None-Match": {savedETag}})
	if res.StatusCode != http.StatusNotModified || body != "" || res.Header.Get("ETag") != savedETag {
		t.Fatalf("revalidation => %d %q etag %q, want bodyless 304 with the saved ETag", res.StatusCode, body, res.Header.Get("ETag"))
	}
	res, body = get(http.Header{"If-Match": {`"other"`}})
	if res.StatusCode != http.StatusPreconditionFailed || body != "" {
		t.Fatalf("failed If-Match => %d %q, want bodyless 412", res.StatusCode, body)
	}
	res, body = get(http.Header{"Range": {"bytes=0-6"}})
	if res.StatusCode != http.StatusPartialContent || body != versionBytes[:7] || res.Header.Get("Content-Range") != "bytes 0-6/18" {
		t.Fatalf("range => %d %q %q, want 206 of the first 7 bytes", res.StatusCode, body, res.Header.Get("Content-Range"))
	}
}
