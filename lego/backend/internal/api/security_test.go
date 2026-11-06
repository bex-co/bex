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
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// httptest.NewRequest stamps RemoteAddr as 192.0.2.1:1234, so a CIDR covering
// it stands in for "the request arrived from a trusted edge proxy".
func trustedForTestPeer(t *testing.T) core.TrustedProxies {
	t.Helper()
	p, err := core.ParseTrustedProxies("192.0.2.0/24")
	if err != nil {
		t.Fatalf("ParseTrustedProxies: %v", err)
	}
	return p
}

func TestSecurityHeadersPresent(t *testing.T) {
	h := withSecurityHeaders(nil, ok200)
	r := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	checks := map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":        "DENY",
	}
	for header, want := range checks {
		if got := w.Header().Get(header); got != want {
			t.Errorf("%s: got %q, want %q", header, got, want)
		}
	}
	if csp := w.Header().Get("Content-Security-Policy"); csp == "" {
		t.Error("Content-Security-Policy header missing")
	}
}

func TestHSTSAbsentWithoutTLS(t *testing.T) {
	h := withSecurityHeaders(trustedForTestPeer(t), ok200)
	r := httptest.NewRequest(http.MethodGet, "/v1/services", nil)
	// No X-Forwarded-Proto header → plain HTTP, HSTS must not be set.
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if got := w.Header().Get("Strict-Transport-Security"); got != "" {
		t.Errorf("HSTS must not be set over plain HTTP, got %q", got)
	}
}

func TestHSTSPresentWhenTrustedProxyForwardsHTTPS(t *testing.T) {
	h := withSecurityHeaders(trustedForTestPeer(t), ok200)
	r := httptest.NewRequest(http.MethodGet, "/v1/services", nil)
	r.Header.Set("X-Forwarded-Proto", "https")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if got := w.Header().Get("Strict-Transport-Security"); got == "" {
		t.Error("HSTS must be set when a trusted proxy forwards X-Forwarded-Proto: https")
	}
}

// codex-security target #10: a client that is NOT a trusted proxy must not be
// able to elicit HSTS by forging X-Forwarded-Proto: https.
func TestHSTSAbsentWhenForwardedProtoFromUntrustedPeer(t *testing.T) {
	for _, trusted := range []core.TrustedProxies{nil, trustedForTestPeer(t)} {
		h := withSecurityHeaders(trusted, ok200)
		r := httptest.NewRequest(http.MethodGet, "/v1/services", nil)
		r.RemoteAddr = "203.0.113.9:5555" // outside any trusted CIDR
		r.Header.Set("X-Forwarded-Proto", "https")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if got := w.Header().Get("Strict-Transport-Security"); got != "" {
			t.Errorf("HSTS must not be set from a spoofed header on an untrusted peer, got %q", got)
		}
	}
}

// A request that terminated TLS at this process emits HSTS regardless of any
// forwarding header, so a stripped X-Forwarded-Proto cannot suppress it.
func TestHSTSPresentWhenConnectionIsTLS(t *testing.T) {
	h := withSecurityHeaders(nil, ok200)
	r := httptest.NewRequest(http.MethodGet, "/v1/services", nil)
	r.TLS = &tls.ConnectionState{}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if got := w.Header().Get("Strict-Transport-Security"); got == "" {
		t.Error("HSTS must be set for a genuine TLS connection")
	}
}

func TestSecurityHeadersOnAllMethods(t *testing.T) {
	h := withSecurityHeaders(nil, ok200)
	for _, method := range []string{http.MethodGet, http.MethodPost, http.MethodOptions} {
		r := httptest.NewRequest(method, "/v1/services", nil)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if got := w.Header().Get("X-Content-Type-Options"); got != "nosniff" {
			t.Errorf("%s: X-Content-Type-Options = %q, want nosniff", method, got)
		}
	}
}

// TestAPIResponsesAreNoStore is w4/522: connection info, env values and keys
// carry credentials, and no response told a browser or intermediary not to
// keep them. Every bex-api answer defaults to no-store; a route that sets its
// own policy (an SSE stream's no-cache) keeps it.
func TestAPIResponsesAreNoStore(t *testing.T) {
	srv := matrixServer(t)
	h, err := srv.Handler()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ method, path, body string }{
		{http.MethodGet, "/v1/services?limit=1", ""},
		{http.MethodPost, "/graphql", `{"query":"{ services { id } }"}`},
		{http.MethodGet, "/v1/services?limit=1", "unauthenticated"},
	} {
		token := testToken
		if tc.body == "unauthenticated" {
			token, tc.body = "", ""
		}
		rec := do(t, h, tc.method, tc.path, token, tc.body)
		if got := rec.Header().Get("Cache-Control"); got != "no-store" {
			t.Errorf("%s %s (%d): Cache-Control %q, want no-store", tc.method, tc.path, rec.Code, got)
		}
	}
	own := withSecurityHeaders(nil, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Cache-Control", "no-cache")
	}))
	w := httptest.NewRecorder()
	own.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/v1/logs/subscribe", nil))
	if got := w.Header().Values("Cache-Control"); len(got) != 1 || got[0] != "no-cache" {
		t.Errorf("a route's own policy = %v, want [no-cache]", got)
	}
}
