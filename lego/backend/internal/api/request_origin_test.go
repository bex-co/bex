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
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// TestRequestOriginMiddlewareClassifiesEveryProductSurface drives the real
// middleware over real request paths and asserts the surface the recorder would
// derive. Without it the derivation is only tested against synthetic contexts,
// and a change to the path classification could silently turn every dashboard
// creation into `unknown` while the build and every other test stayed green.
func TestRequestOriginMiddlewareClassifiesEveryProductSurface(t *testing.T) {
	for _, tc := range []struct {
		name, method, path, userAgent string
		identity                      *core.Identity
		want                          string
	}{
		{
			name: "dashboard", method: http.MethodPost, path: "/graphql", userAgent: "Mozilla/5.0",
			identity: &core.Identity{Subject: "u-1", Method: "session"}, want: core.SurfaceDashboard,
		},
		{
			name: "cli", method: http.MethodPost, path: "/v1/services",
			userAgent: "render-cli/2.27.0 (macOS - 26.5.1)", want: core.SurfaceCLI,
		},
		{
			name: "mcp", method: http.MethodPost, path: "/mcp", userAgent: "", want: core.SurfaceMCP,
		},
		{
			name: "direct api", method: http.MethodPost, path: "/v1/services",
			userAgent: "curl/8.4.0", want: core.SurfaceAPI,
		},
		{
			name: "device-flow auth is not product traffic", method: http.MethodPost,
			path: "/v1/device-token", userAgent: "render-cli/2.27.0", want: core.SurfaceUnknown,
		},
		{
			name: "health checks are not product traffic", method: http.MethodGet,
			path: "/readyz", userAgent: "kube-probe/1.33", want: core.SurfaceUnknown,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var got string
			handler := RequestOriginMiddleware(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				ctx := r.Context()
				if tc.identity != nil {
					ctx = core.WithIdentity(ctx, *tc.identity)
				}
				got = core.ProductSurface(ctx)
			}))
			req := httptest.NewRequest(tc.method, tc.path, nil)
			if tc.userAgent != "" {
				req.Header.Set("User-Agent", tc.userAgent)
			}
			handler.ServeHTTP(httptest.NewRecorder(), req)
			if got != tc.want {
				t.Errorf("surface for %s %s = %q, want %q", tc.method, tc.path, got, tc.want)
			}
		})
	}
}

// TestRequestOriginMiddlewareRunsWithoutMetrics is the reason this middleware is
// separate from the metrics one: OriginMetrics.Middleware returns the handler
// untouched when metrics are disabled, which must not change what analytics
// records.
func TestRequestOriginMiddlewareRunsWithoutMetrics(t *testing.T) {
	var metrics *OriginMetrics // disabled
	var got string
	inner := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		got = core.ProductSurface(r.Context())
	})
	handler := metrics.Middleware(RequestOriginMiddleware(inner))
	req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	handler.ServeHTTP(httptest.NewRecorder(), req)
	if got != core.SurfaceMCP {
		t.Errorf("surface with metrics disabled = %q, want mcp", got)
	}
}

// TestRequestOriginSurvivesContextDerivation covers the handoff the recorder
// actually makes: it re-roots the context with context.WithoutCancel before the
// bounded write, which preserves values but is worth pinning.
func TestRequestOriginSurvivesContextDerivation(t *testing.T) {
	var got string
	handler := RequestOriginMiddleware(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		got = core.ProductSurface(context.WithoutCancel(r.Context()))
	}))
	req := httptest.NewRequest(http.MethodPost, "/v1/services", nil)
	req.Header.Set("User-Agent", "render-cli/2.27.0")
	handler.ServeHTTP(httptest.NewRecorder(), req)
	if got != core.SurfaceCLI {
		t.Errorf("surface after WithoutCancel = %q, want cli", got)
	}
}
