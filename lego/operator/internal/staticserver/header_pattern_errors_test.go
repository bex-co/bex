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
	"errors"
	"net/http"
	"strings"
	"testing"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
	"golang.org/x/sync/semaphore"
)

func TestWildcardHeaderSelectorsPreserveErrorResponses(t *testing.T) {
	for _, selector := range []struct{ pattern, path string }{
		{"/*.css", "/error.css"},
		{"/**/*.css", "/assets/nested/error.css"},
	} {
		for _, scenario := range []struct {
			name              string
			status            int
			body              string
			originErr         error
			rewrite, liveBody bool
		}{
			{name: "invalid redirect", status: http.StatusBadRequest, body: "invalid redirect target\n"},
			{name: "oversized object", status: http.StatusRequestEntityTooLarge, body: "object too large\n", originErr: ErrObjectTooLarge},
			{name: "origin denied", status: http.StatusBadGateway, body: "origin error\n", originErr: errors.New("origin denied")},
			{name: "origin overloaded", status: http.StatusServiceUnavailable, body: "server busy\n", originErr: ErrOverloaded},
			{name: "rewritten origin overloaded", status: http.StatusServiceUnavailable, body: "server busy\n", originErr: ErrOverloaded, rewrite: true},
			{name: "live body budget exhausted", status: http.StatusServiceUnavailable, body: "server busy\n", liveBody: true},
		} {
			for _, method := range []string{http.MethodGet, http.MethodHead} {
				// HEAD bypasses body leasing, so this case is a successful response already
				// covered by the success matrix rather than a capacity error.
				if scenario.liveBody && method == http.MethodHead {
					continue
				}
				t.Run(selector.pattern+"/"+scenario.name+"/"+method, func(t *testing.T) {
					site := Site{Headers: []appv1alpha1.StaticHeader{
						{Path: selector.pattern, Name: "X-Selected", Value: "request-file"},
						{Path: selector.pattern, Name: "Content-Type", Value: "text/wrong"},
						{Path: selector.pattern, Name: "Content-Length", Value: "999"},
						{Path: selector.pattern, Name: "Transfer-Encoding", Value: "chunked"},
						{Path: selector.pattern, Name: "Retry-After", Value: "999"},
						{Path: "/served.html", Name: "X-Destination", Value: "must-not-apply"},
					}}
					objectPath := strings.TrimPrefix(selector.path, "/")
					switch {
					case scenario.status == http.StatusBadRequest:
						site.Routes = []appv1alpha1.StaticRoute{{Type: "redirect", Source: selector.path, Destination: "//unsafe.example"}}
					case scenario.rewrite:
						site.Routes = []appv1alpha1.StaticRoute{{Type: "rewrite", Source: selector.path, Destination: "/served.html"}}
						objectPath = "served.html"
					}
					h, origin := newTestHandler(t, site, nil)
					if scenario.originErr != nil {
						origin.errs[key(objectPath)] = scenario.originErr
					}
					if scenario.liveBody {
						origin.objs = map[string]Object{key(objectPath): {Body: []byte("too large for the live body lease"), ContentType: "text/css"}}
						h.liveBodies = semaphore.NewWeighted(1)
					}
					response := do(h, method, selector.path)
					if response.Code != scenario.status {
						t.Fatalf("status=%d, want %d", response.Code, scenario.status)
					}
					if response.Header().Get("X-Selected") != "request-file" {
						t.Fatal("matching file selector missing from error response")
					}
					if got := response.Header().Get("X-Destination"); got != "" {
						t.Fatalf("error used rewritten destination headers: %q", got)
					}
					if got := response.Header().Get("Content-Type"); got != "text/plain; charset=utf-8" {
						t.Fatalf("error MIME changed: %q", got)
					}
					if got := response.Header().Get("Content-Length"); got != "" {
						t.Fatalf("error inherited false body length: %q", got)
					}
					if got := response.Header().Get("Transfer-Encoding"); got != "" {
						t.Fatalf("error inherited transfer metadata: %q", got)
					}
					wantRetry := "999"
					if scenario.status == http.StatusServiceUnavailable {
						wantRetry = "1"
					}
					if got := response.Header().Get("Retry-After"); got != wantRetry {
						t.Fatalf("Retry-After=%q, want %q", got, wantRetry)
					}
					if method == http.MethodGet && response.Body.String() != scenario.body {
						t.Fatalf("error body=%q, want %q", response.Body.String(), scenario.body)
					}
				})
			}
		}
	}
}
