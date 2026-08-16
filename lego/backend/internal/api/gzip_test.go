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

package api

import (
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func jsonHandler(body string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Length", "999") // deliberately wrong; must be dropped
		_, _ = io.WriteString(w, body)
	})
}

func TestWithGzip_CompressesJSONWhenAccepted(t *testing.T) {
	body := strings.Repeat(`{"k":"v"},`, 500) // repetitive => highly compressible
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/graphql", nil)
	req.Header.Set("Accept-Encoding", "gzip, deflate, br")

	withGzip(jsonHandler(body)).ServeHTTP(rec, req)
	res := rec.Result()

	if got := res.Header.Get("Content-Encoding"); got != "gzip" {
		t.Fatalf("Content-Encoding = %q, want gzip", got)
	}
	if got := res.Header.Get("Vary"); !strings.Contains(got, "Accept-Encoding") {
		t.Fatalf("Vary = %q, want to contain Accept-Encoding", got)
	}
	if got := res.Header.Get("Content-Length"); got != "" {
		t.Fatalf("Content-Length = %q, want dropped for a compressed body", got)
	}
	gr, err := gzip.NewReader(res.Body)
	if err != nil {
		t.Fatalf("gzip.NewReader: %v", err)
	}
	decoded, err := io.ReadAll(gr)
	if err != nil {
		t.Fatalf("read gzip body: %v", err)
	}
	if string(decoded) != body {
		t.Fatalf("decoded body != original (len %d vs %d)", len(decoded), len(body))
	}
	if len(rec.Body.Bytes()) >= len(body) {
		t.Fatalf("compressed size %d not smaller than raw %d", len(rec.Body.Bytes()), len(body))
	}
}

func TestWithGzip_IdentityWhenNotAccepted(t *testing.T) {
	body := `{"ok":true}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/graphql", nil) // no Accept-Encoding

	withGzip(jsonHandler(body)).ServeHTTP(rec, req)
	res := rec.Result()

	if got := res.Header.Get("Content-Encoding"); got != "" {
		t.Fatalf("Content-Encoding = %q, want empty (identity)", got)
	}
	if rec.Body.String() != body {
		t.Fatalf("body = %q, want %q", rec.Body.String(), body)
	}
}

func TestWithGzip_SkipsEventStream(t *testing.T) {
	sse := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fl, ok := w.(http.Flusher)
		if !ok {
			t.Error("gzip writer must expose http.Flusher to SSE handlers")
			return
		}
		for i := 0; i < 3; i++ {
			_, _ = io.WriteString(w, "data: event\n\n")
			fl.Flush() // per-event flush must reach the client
		}
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/logs/subscribe", nil)
	req.Header.Set("Accept-Encoding", "gzip")

	withGzip(sse).ServeHTTP(rec, req)
	res := rec.Result()

	if got := res.Header.Get("Content-Encoding"); got != "" {
		t.Fatalf("SSE Content-Encoding = %q, want empty (never compress a stream)", got)
	}
	if !rec.Flushed {
		t.Fatal("SSE response was not flushed per event")
	}
	if got := rec.Body.String(); got != strings.Repeat("data: event\n\n", 3) {
		t.Fatalf("SSE body = %q, want the raw un-compressed events", got)
	}
}

func TestWithGzip_LeavesPreEncodedResponseAlone(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Encoding", "gzip") // handler already encoded
		_, _ = io.WriteString(w, "pre-encoded-bytes")
	})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("Accept-Encoding", "gzip")

	withGzip(h).ServeHTTP(rec, req)

	if rec.Body.String() != "pre-encoded-bytes" {
		t.Fatalf("double-compressed a pre-encoded body: %q", rec.Body.String())
	}
}

func TestClientAcceptsGzip(t *testing.T) {
	cases := map[string]bool{
		"gzip":              true,
		"gzip, deflate, br": true,
		"br, gzip":          true,
		"identity":          false,
		"":                  false,
		"deflate":           false,
		"x-gzip":            false,
		" gzip ;q=1.0":      true,
	}
	for header, want := range cases {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		if header != "" {
			r.Header.Set("Accept-Encoding", header)
		}
		if got := clientAcceptsGzip(r); got != want {
			t.Errorf("clientAcceptsGzip(%q) = %v, want %v", header, got, want)
		}
	}
}
