// Copyright 2026.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package bex_s3_metadata_case

import (
	"bufio"
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"strings"
	"testing"
)

// head sends one HTTP/1.1 request to addr over a raw connection and returns the
// response head as it went over the wire. net/http's client canonicalises every
// header name it reads, so only the bytes show the case a client such as
// botocore sees.
func head(t *testing.T, addr string) string {
	t.Helper()
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := io.WriteString(conn, "GET /b/k HTTP/1.1\r\nHost: x\r\nConnection: close\r\n\r\n"); err != nil {
		t.Fatal(err)
	}
	var sb strings.Builder
	r := bufio.NewReader(conn)
	for {
		line, err := r.ReadString('\n')
		sb.WriteString(line)
		if err != nil || line == "\r\n" {
			break
		}
	}
	return sb.String()
}

func edge(t *testing.T, app http.Handler) *httptest.Server {
	t.Helper()
	h, err := New(context.Background(), app, CreateConfig(), "s3-metadata-case")
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return srv
}

func lines(head string) map[string]bool {
	out := map[string]bool{}
	for _, l := range strings.Split(head, "\r\n") {
		out[l] = true
	}
	return out
}

// w21/113: a user-metadata name leaves in lowercase, as S3 names it, and every
// other header keeps its canonical name.
func TestMetadataNamesGoOutLowercase(t *testing.T) {
	srv := edge(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		h := w.Header()
		h.Set("ETag", `"abc"`)
		h.Set("Content-Type", "text/plain")
		h.Set("X-Amz-Meta-Mtime", "1700000000")
		h.Set("x-amz-meta-MixedCase", "v") // canonicalises to X-Amz-Meta-Mixedcase
		h.Add("X-Amz-Meta-Many", "one")
		h.Add("X-Amz-Meta-Many", "two")
		h.Set("X-Amz-Request-Id", "req")
		_, _ = io.WriteString(w, "body")
	}))
	got := lines(head(t, srv.Listener.Addr().String()))
	for _, want := range []string{
		"x-amz-meta-mtime: 1700000000",
		"x-amz-meta-mixedcase: v",
		"x-amz-meta-many: one",
		"x-amz-meta-many: two",
		"Etag: \"abc\"",
		"Content-Type: text/plain",
		"X-Amz-Request-Id: req",
	} {
		if !got[want] {
			t.Errorf("the head lacks %q:\n%v", want, got)
		}
	}
	for line := range got {
		if strings.HasPrefix(line, "X-Amz-Meta-") {
			t.Errorf("a canonical metadata header went out: %q", line)
		}
	}
}

// The proxy path: Traefik reads the backend's head with net/http, which makes
// every name canonical, and copies it out with Header.Add. The middleware still
// has the last word.
func TestMetadataNamesGoOutLowercaseBehindAReverseProxy(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header()["x-amz-meta-raw"] = []string{"1"} // not canonical: written as is
		w.Header().Set("X-Amz-Meta-Set", "2")
		w.Header().Set("Content-Length", "2")
		_, _ = io.WriteString(w, "ok")
	}))
	t.Cleanup(backend.Close)
	target, err := url.Parse(backend.URL)
	if err != nil {
		t.Fatal(err)
	}
	proxied := head(t, edge(t, httputil.NewSingleHostReverseProxy(target)).Listener.Addr().String())
	bare := httptest.NewServer(httputil.NewSingleHostReverseProxy(target))
	t.Cleanup(bare.Close)
	// Without the middleware the proxy canonicalises: the defect.
	if got := lines(head(t, bare.Listener.Addr().String())); !got["X-Amz-Meta-Raw: 1"] || !got["X-Amz-Meta-Set: 2"] {
		t.Fatalf("a bare reverse proxy no longer canonicalises, so this test shows nothing:\n%v", got)
	}
	got := lines(proxied)
	if !got["x-amz-meta-raw: 1"] || !got["x-amz-meta-set: 2"] {
		t.Errorf("the metadata names are not lowercase behind the proxy:\n%v", got)
	}
}

// A handler that never calls WriteHeader: the first Write sends the head.
func TestMetadataNamesGoOutLowercaseOnAnImplicitHead(t *testing.T) {
	srv := edge(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-Amz-Meta-K", "v")
		_, _ = io.WriteString(w, "body")
	}))
	if got := lines(head(t, srv.Listener.Addr().String())); !got["x-amz-meta-k: v"] {
		t.Errorf("the implicit head kept a canonical name:\n%v", got)
	}
}

// snapWriter keeps the header map as it stood at each WriteHeader, as the server
// writes a head from it at that moment (httptest.ResponseRecorder snapshots only
// the first).
type snapWriter struct {
	header http.Header
	heads  []http.Header
}

func (s *snapWriter) Header() http.Header         { return s.header }
func (s *snapWriter) Write(b []byte) (int, error) { return len(b), nil }
func (s *snapWriter) WriteHeader(code int) {
	s.heads = append(s.heads, s.header.Clone())
}

// An informational answer goes out with the map as it stands, and the final head
// carries headers set after it.
func TestMetadataNamesAreRenamedAfterAnInformationalAnswer(t *testing.T) {
	h, err := New(context.Background(), http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-Amz-Meta-First", "1")
		w.WriteHeader(http.StatusEarlyHints)
		w.Header().Set("X-Amz-Meta-Second", "2")
		w.WriteHeader(http.StatusOK)
	}), CreateConfig(), "s3-metadata-case")
	if err != nil {
		t.Fatal(err)
	}
	sw := &snapWriter{header: http.Header{}}
	h.ServeHTTP(sw, httptest.NewRequest(http.MethodGet, "/", nil))
	if len(sw.heads) != 2 {
		t.Fatalf("%d heads written, want 2", len(sw.heads))
	}
	for i, want := range [][]string{{"x-amz-meta-first"}, {"x-amz-meta-first", "x-amz-meta-second"}} {
		for _, name := range want {
			if sw.heads[i][name] == nil {
				t.Errorf("head %d lacks %q: %v", i, name, sw.heads[i])
			}
		}
		for name := range sw.heads[i] {
			if strings.HasPrefix(name, "X-Amz-Meta-") {
				t.Errorf("head %d holds the canonical %q: %v", i, name, sw.heads[i])
			}
		}
	}
}

// Nothing else is touched: no metadata header, a name that is only the prefix, and the
// body.
func TestOtherHeadersAndTheBodyPassThrough(t *testing.T) {
	rec := httptest.NewRecorder()
	h, err := New(context.Background(), http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-Amz-Meta-", "empty name")
		w.Header().Set("X-Amz-Metadata-Directive", "REPLACE")
		w.Header().Set("X-Amz-Request-Id", "r")
		w.WriteHeader(http.StatusTeapot)
		_, _ = io.WriteString(w, "body")
	}), CreateConfig(), "s3-metadata-case")
	if err != nil {
		t.Fatal(err)
	}
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	res := rec.Result()
	body, _ := io.ReadAll(res.Body)
	if res.StatusCode != http.StatusTeapot || string(body) != "body" {
		t.Fatalf("status %d body %q", res.StatusCode, body)
	}
	for _, name := range []string{"X-Amz-Meta-", "X-Amz-Metadata-Directive", "X-Amz-Request-Id"} {
		if res.Header[name] == nil {
			t.Errorf("%q was renamed or dropped: %v", name, res.Header)
		}
	}
}
