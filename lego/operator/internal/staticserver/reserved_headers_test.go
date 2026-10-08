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

// TestReservedHeaderRulesCannotBreakResponses is w4/204: a saved
// Content-Length: 1 rule made Go's server abort every matching response
// (HTTP/2 INTERNAL_ERROR, HTTP/1.1 empty reply), taking the site down. A
// real server is used because a recorder does not enforce body framing.
func TestReservedHeaderRulesCannotBreakResponses(t *testing.T) {
	body := "<!doctype html><h1>home</h1>"
	h, _ := newTestHandler(t, Site{Headers: []appv1alpha1.StaticHeader{
		{Path: "/*", Name: "Content-Length", Value: "1"},
		{Path: "/*", Name: "connection", Value: "close"},
		{Path: "/*", Name: "TE", Value: "trailers"},
		{Path: "/*", Name: "X-Kept", Value: "yes"},
	}}, map[string]Object{key("index.html"): {Body: []byte(body), ContentType: "text/html"}})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Host = testHost
		h.ServeHTTP(w, r)
	}))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/")
	if err != nil {
		t.Fatalf("GET / failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	got, err := io.ReadAll(resp.Body)
	if err != nil || resp.StatusCode != http.StatusOK || string(got) != body {
		t.Fatalf("GET / = %d %q, %v; want 200 with the whole page", resp.StatusCode, got, err)
	}
	if resp.Header.Get("X-Kept") != "yes" {
		t.Fatal("an ordinary rule next to the reserved ones was dropped")
	}
}
