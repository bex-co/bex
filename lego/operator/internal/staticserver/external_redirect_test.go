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
	"net/http"
	"testing"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// A redirect may name a fixed external host, as on Render; a rewrite may not,
// and a capture can never change the host (w8/162).
func TestRedirectToExternalHost(t *testing.T) {
	routes := []appv1alpha1.StaticRoute{
		{Type: "redirect", Source: "/maps/*", Destination: "https://cdn.example.com/maps/*"},
		{Type: "redirect", Source: "/old", Destination: "https://example.org/new?x=1"},
		{Type: "redirect", Source: "/at/*", Destination: "https://cdn.example.com/:splat"},
		{Type: "rewrite", Source: "/proxy", Destination: "https://example.org/x"},
	}
	for _, tc := range []struct {
		path, location string
		status         int
	}{
		{"/maps/a/b.png", "https://cdn.example.com/maps/a/b.png", http.StatusMovedPermanently},
		{"/old", "https://example.org/new?x=1", http.StatusMovedPermanently},
		{"/at/@evil.com/x", "https://cdn.example.com/@evil.com/x", http.StatusMovedPermanently},
		{"/at//evil.com", "https://cdn.example.com/evil.com", http.StatusMovedPermanently},
		{"/proxy", "", http.StatusBadRequest},
	} {
		h, _ := newTestHandler(t, Site{Routes: routes}, nil)
		response := do(h, http.MethodGet, tc.path)
		if response.Code != tc.status || response.Header().Get("Location") != tc.location {
			t.Errorf("%s: status=%d location=%q, want %d %q", tc.path, response.Code, response.Header().Get("Location"), tc.status, tc.location)
		}
	}
}

func TestRedirectDestinationRefusals(t *testing.T) {
	for _, dest := range []string{
		"https://*.example.com/x", "https://:splat/x", "https://user@host.example/x",
		"https:///x", "//host/x", `https://host.example\@evil/x`, "ftp://host.example/x", "https://host.example/\x01",
	} {
		if _, err := expandDest(dest, "a", true); err == nil {
			t.Errorf("redirect destination %q accepted", dest)
		}
	}
	if _, err := expandDest("https://host.example/x", "", false); err == nil {
		t.Error("absolute rewrite destination accepted")
	}
}
