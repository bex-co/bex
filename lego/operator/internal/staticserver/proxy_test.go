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
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w8/184: a rewrite whose destination is a public URL serves that URL's content
// at the original path, for any method.

func externalSite(destination string) Site {
	return Site{
		Routes: []appv1alpha1.StaticRoute{{Type: "rewrite", Source: "/api/*", Destination: destination}},
		Headers: []appv1alpha1.StaticHeader{
			{Path: "/*", Name: "X-Site", Value: "yes"},
		},
	}
}

func TestExternalRewriteProxiesAnyMethodAtTheOriginalPath(t *testing.T) {
	var hits atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		body, _ := io.ReadAll(r.Body)
		w.Header().Set("X-Upstream-Path", r.URL.RequestURI())
		w.Header().Set("X-Upstream-Method", r.Method)
		w.Header().Set("X-Upstream-Host", r.Host)
		w.Header().Set("X-Upstream-Custom", r.Header.Get("X-Custom"))
		w.Header().Set("X-Upstream-Hop", r.Header.Get(proxyHopHeader))
		w.Header().Set("Location", "https://elsewhere.example/next")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write(append([]byte("api:"), body...))
	}))
	defer upstream.Close()
	host := strings.TrimPrefix(upstream.URL, "http://")

	h, _ := newTestHandler(t, externalSite("http://"+host+"/v1/*"), nil)
	h.external = newExternalProxy(func(net.IP) bool { return false }) // the test dials loopback

	req := httptest.NewRequest(http.MethodPost, "https://site.onbex.co/api/users?x=1", strings.NewReader("payload"))
	req.Header.Set("X-Custom", "kept")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated || rr.Body.String() != "api:payload" {
		t.Fatalf("response = %d %q, want the destination's own", rr.Code, rr.Body.String())
	}
	for header, want := range map[string]string{
		"X-Upstream-Path": "/v1/users?x=1", "X-Upstream-Method": "POST", "X-Upstream-Host": host,
		"X-Upstream-Custom": "kept", "X-Upstream-Hop": "1",
		"Location": "https://elsewhere.example/next", // a redirect is passed through, not followed
		"X-Site":   "yes",                            // the site's custom headers apply
	} {
		if got := rr.Header().Get(header); got != want {
			t.Errorf("%s = %q, want %q", header, got, want)
		}
	}
	if hits.Load() != 1 {
		t.Errorf("upstream saw %d requests, want 1", hits.Load())
	}

	// GET works the same, and a method no rule claims stays a 405.
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "https://site.onbex.co/api/ping", nil))
	if rr.Code != http.StatusCreated || rr.Header().Get("X-Upstream-Path") != "/v1/ping" {
		t.Errorf("GET = %d, path %q", rr.Code, rr.Header().Get("X-Upstream-Path"))
	}
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "https://site.onbex.co/other", nil))
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST to an unclaimed path = %d, want 405", rr.Code)
	}
}

// A published file wins over the rule, and the root path is not rewritten.
func TestExternalRewriteNeverShadowsAFileOrTheRoot(t *testing.T) {
	var hits atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { hits.Add(1) }))
	defer upstream.Close()
	host := strings.TrimPrefix(upstream.URL, "http://")
	site := Site{Routes: []appv1alpha1.StaticRoute{{Type: "rewrite", Source: "/*", Destination: "http://" + host + "/*"}}}
	h, _ := newTestHandler(t, site, map[string]Object{key("app.js"): {Body: []byte("JS")}})
	h.external = newExternalProxy(func(net.IP) bool { return false })

	if rec := do(h, http.MethodGet, "/app.js"); rec.Body.String() != "JS" {
		t.Errorf("/app.js = %d %q, want the published file", rec.Code, rec.Body)
	}
	if rec := do(h, http.MethodGet, "/"); rec.Code != http.StatusNotFound {
		t.Errorf("/ = %d, want 404 (the root is not rewritten)", rec.Code)
	}
	if hits.Load() != 0 {
		t.Errorf("upstream saw %d requests", hits.Load())
	}
}

// The default proxy refuses every non-public destination: nothing is dialled.
func TestExternalRewriteRefusesNonPublicDestinations(t *testing.T) {
	var hits atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { hits.Add(1) }))
	defer upstream.Close()
	for _, dest := range []string{
		upstream.URL + "/*",                    // loopback
		"http://127.0.0.1:1/*",                 // loopback literal
		"http://10.0.0.1/*",                    // private
		"http://169.254.169.254/latest/*",      // cloud metadata
		"http://localhost/*",                   // resolves to loopback
		"http://kubernetes.default.svc/*",      // cluster-internal name
		"http://bex-api.bex-system.svc:8090/*", // control plane
	} {
		h, _ := newTestHandler(t, externalSite(dest), nil) // the production dialer
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "https://site.onbex.co/api/x", nil))
		if rr.Code != http.StatusBadGateway {
			t.Errorf("%s: response = %d, want 502", dest, rr.Code)
		}
	}
	if hits.Load() != 0 {
		t.Errorf("a refused destination was dialled %d times", hits.Load())
	}
}

// A destination on the platform's own static hosts, or a request this server
// already proxied, is a loop.
func TestExternalRewriteRefusesLoops(t *testing.T) {
	h, _ := newTestHandler(t, externalSite("https://"+testHost+"/*"), nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "https://site.onbex.co/api/x", nil)) // the test resolver knows site.onbex.co
	if rr.Code != http.StatusLoopDetected {
		t.Errorf("a static-host destination = %d, want 508", rr.Code)
	}
	req := httptest.NewRequest(http.MethodGet, "https://site.onbex.co/api/x", nil)
	req.Header.Set(proxyHopHeader, "1")
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusLoopDetected {
		t.Errorf("a proxied request = %d, want 508", rr.Code)
	}
}
