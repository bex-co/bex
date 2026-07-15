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

package main

import (
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestIsBlockedMaintenanceFetchIP(t *testing.T) {
	cases := []struct {
		ip      string
		blocked bool
	}{
		{"8.8.8.8", false},
		{"93.184.216.34", false}, // a public IP (example.com at capture time)
		{"127.0.0.1", true},
		{"10.0.0.5", true},
		{"172.16.0.5", true},
		{"192.168.1.1", true},
		{"169.254.169.254", true}, // cloud metadata
		{"0.0.0.0", true},
		{"::1", true},
		{"fe80::1", true},
		{"fc00::1", true},
	}
	for _, tc := range cases {
		t.Run(tc.ip, func(t *testing.T) {
			ip := net.ParseIP(tc.ip)
			if ip == nil {
				t.Fatalf("net.ParseIP(%q) = nil", tc.ip)
			}
			if got := isBlockedMaintenanceFetchIP(ip); got != tc.blocked {
				t.Errorf("isBlockedMaintenanceFetchIP(%s) = %v, want %v", tc.ip, got, tc.blocked)
			}
		})
	}
}

func TestMaintenanceFetchClientBlocksLoopback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("should never be reached"))
	}))
	defer srv.Close()

	// httptest servers listen on 127.0.0.1 — exactly the loopback address the
	// dial Control hook must reject, proving the guard runs at connect time.
	_, err := maintenanceFetchClient.Get(srv.URL)
	if err == nil {
		t.Fatal("expected the loopback fetch to be blocked, got nil error")
	}
}

func TestServeMaintenancePageDefaultContentNegotiation(t *testing.T) {
	t.Run("browser gets the HTML default page", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Accept", "text/html")
		w := httptest.NewRecorder()
		serveMaintenancePage(w, req, "")

		if w.Code != http.StatusServiceUnavailable {
			t.Errorf("status = %d, want 503", w.Code)
		}
		if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
			t.Errorf("Content-Type = %q, want text/html", ct)
		}
		if !strings.Contains(w.Body.String(), "maintenance") {
			t.Errorf("body doesn't look like the maintenance page: %s", w.Body.String())
		}
	})

	t.Run("API client gets JSON", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Accept", "application/json")
		w := httptest.NewRecorder()
		serveMaintenancePage(w, req, "")

		if w.Code != http.StatusServiceUnavailable {
			t.Errorf("status = %d, want 503", w.Code)
		}
		if ct := w.Header().Get("Content-Type"); ct != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", ct)
		}
		if !strings.Contains(w.Body.String(), "maintenance") {
			t.Errorf("body doesn't look like a maintenance JSON error: %s", w.Body.String())
		}
	})
}

func TestServeMaintenancePageCustomURIFetchedAndServed(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("tenant's own maintenance page"))
	}))
	defer upstream.Close()

	// The real maintenanceFetchClient blocks loopback by design (that's
	// TestMaintenanceFetchClientBlocksLoopback above); swap in a plain client
	// so this test can exercise serveMaintenancePage's own passthrough logic
	// (status, content-type, body) against an httptest server, which only
	// ever binds to loopback addresses.
	prev := maintenanceFetchClient
	maintenanceFetchClient = &http.Client{Timeout: maintenanceFetchTimeout}
	t.Cleanup(func() { maintenanceFetchClient = prev })

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	serveMaintenancePage(w, req, upstream.URL)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503 (Render always returns 503 while enabled)", w.Code)
	}
	if got := w.Header().Get("Content-Type"); got != "text/plain; charset=utf-8" {
		t.Errorf("Content-Type = %q, want the upstream's", got)
	}
	if w.Body.String() != "tenant's own maintenance page" {
		t.Errorf("body = %q", w.Body.String())
	}
}

func TestServeMaintenancePageFetchFailureReturns502(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	// A loopback uri is both unreachable (SSRF-blocked) and a stand-in for
	// "the fetch failed" — either way serveMaintenancePage must surface 502,
	// never silently fall back to the default page.
	serveMaintenancePage(w, req, "http://127.0.0.1:1/unreachable")

	if w.Code != http.StatusBadGateway {
		t.Errorf("status = %d, want 502", w.Code)
	}
}
