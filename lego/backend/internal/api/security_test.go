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
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSecurityHeadersPresent(t *testing.T) {
	h := withSecurityHeaders(ok200)
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
	h := withSecurityHeaders(ok200)
	r := httptest.NewRequest(http.MethodGet, "/v1/services", nil)
	// No X-Forwarded-Proto header → plain HTTP, HSTS must not be set.
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if got := w.Header().Get("Strict-Transport-Security"); got != "" {
		t.Errorf("HSTS must not be set over plain HTTP, got %q", got)
	}
}

func TestHSTSPresentWithTLS(t *testing.T) {
	h := withSecurityHeaders(ok200)
	r := httptest.NewRequest(http.MethodGet, "/v1/services", nil)
	r.Header.Set("X-Forwarded-Proto", "https")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if got := w.Header().Get("Strict-Transport-Security"); got == "" {
		t.Error("Strict-Transport-Security must be set when X-Forwarded-Proto: https")
	}
}

func TestSecurityHeadersOnAllMethods(t *testing.T) {
	h := withSecurityHeaders(ok200)
	for _, method := range []string{http.MethodGet, http.MethodPost, http.MethodOptions} {
		r := httptest.NewRequest(method, "/v1/services", nil)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if got := w.Header().Get("X-Content-Type-Options"); got != "nosniff" {
			t.Errorf("%s: X-Content-Type-Options = %q, want nosniff", method, got)
		}
	}
}
