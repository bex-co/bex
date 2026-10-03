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
	"strings"
	"testing"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func TestRouteDestinationExpansionBudget(t *testing.T) {
	for _, tc := range []struct {
		name, destination, capture string
		size                       int
		invalid                    bool
	}{
		{name: "configured boundary", destination: "/" + strings.Repeat("a", 2047), size: 2048},
		{name: "oversized direct CR", destination: "/" + strings.Repeat("a", 2048), invalid: true},
		{name: "single capture boundary", destination: "/:splat", capture: strings.Repeat("a", 8191), size: 8192},
		{name: "single capture oversized", destination: "/:splat", capture: strings.Repeat("a", 8192), invalid: true},
		{name: "trailing alias boundary", destination: "/docs/*", capture: strings.Repeat("a", 8186), size: 8192},
		{name: "trailing alias oversized", destination: "/docs/*", capture: strings.Repeat("a", 8187), invalid: true},
		{name: "repeated captures boundary", destination: "/:splat/:splat", capture: strings.Repeat("a", 4095), size: 8192},
		{name: "repeated captures amplification", destination: "/" + strings.Repeat(":splat/", 250), capture: strings.Repeat("a", 100), invalid: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			target, err := expandDest(tc.destination, tc.capture)
			if tc.invalid {
				if err == nil {
					t.Fatal("oversized target accepted")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if len(target.Path) != tc.size {
				t.Fatalf("decoded path length=%d want%d", len(target.Path), tc.size)
			}
		})
	}
}

func TestRouteDestinationBudgetRejectsBeforeFallback(t *testing.T) {
	for _, action := range []string{"redirect", "rewrite"} {
		t.Run(action, func(t *testing.T) {
			h, origin := newTestHandler(t, Site{Routes: []appv1alpha1.StaticRoute{{
				Type: action, Source: "/jump/*", Destination: "/" + strings.Repeat(":splat/", 250),
			}}}, map[string]Object{key("index.html"): {Body: []byte("SPA fallback")}})
			requestPath := "/jump/" + strings.Repeat("a", 100)
			response := do(h, http.MethodGet, requestPath)
			if response.Code != http.StatusBadRequest || response.Header().Get("Location") != "" {
				t.Fatalf("oversized expansion status=%d headers=%v", response.Code, response.Header())
			}
			if len(origin.gets) != 1 || origin.gets[key(strings.TrimPrefix(requestPath, "/"))] != 1 {
				t.Fatalf("oversized expansion fetched a destination or fallback: %v", origin.gets)
			}
		})
	}
}
