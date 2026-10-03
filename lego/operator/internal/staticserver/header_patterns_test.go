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
	"net/url"
	"strings"
	"testing"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func TestHeaderPatternsServedResponses(t *testing.T) {
	rules := []appv1alpha1.StaticHeader{
		{Path: "/*", Name: "X-All", Value: "yes"},
		{Path: "/render.yaml", Name: "X-Exact", Value: "yes"},
		{Path: "/nested/*", Name: "X-Subtree", Value: "yes"},
		{Path: "/*.yaml", Name: "X-Root-Yaml", Value: "yes"},
		{Path: "/**/*.yaml", Name: "X-Nested-Yaml", Value: "yes"},
		{Path: "/*.css", Name: "X-Root-Css", Value: "yes"},
		{Path: "/**/*.css", Name: "X-Nested-Css", Value: "yes"},
		{Path: "/**/*", Name: "X-Nested-All", Value: "yes"},
	}
	h, _ := newTestHandler(t, Site{Headers: rules}, map[string]Object{
		key("render.yaml"):        {Body: []byte("yaml")},
		key("style.css"):          {Body: []byte("css")},
		key("nested/config.yaml"): {Body: []byte("nested yaml")},
		key("nested/style.css"):   {Body: []byte("nested css")},
		key("nested/index.html"):  {Body: []byte("directory")},
	})
	for _, tc := range []struct {
		path, body, headers string
		status              int
	}{
		{"/render.yaml", "yaml", "X-All X-Exact X-Root-Yaml", 200},
		{"/missing.yaml", "", "X-All X-Root-Yaml", 404},
		{"/style.css", "css", "X-All X-Root-Css", 200},
		{"/missing.css", "", "X-All X-Root-Css", 404},
		{"/nested/config.yaml", "nested yaml", "X-All X-Subtree X-Nested-Yaml X-Nested-All", 200},
		{"/nested/style.css", "nested css", "X-All X-Subtree X-Nested-Css X-Nested-All", 200},
		{"/nested/missing.yaml", "", "X-All X-Subtree X-Nested-Yaml X-Nested-All", 404},
		{"/nested/deep/missing.css", "", "X-All X-Subtree X-Nested-Css X-Nested-All", 404},
		{"/nested/", "directory", "X-All X-Subtree X-Nested-All", 200},
		{"/other/deep/", "", "X-All X-Nested-All", 404},
		{"/nestedness/file.yaml", "", "X-All X-Nested-Yaml X-Nested-All", 404},
		{"/", "", "X-All", 404},
	} {
		for _, method := range []string{http.MethodGet, http.MethodHead} {
			t.Run(method+tc.path, func(t *testing.T) {
				response := do(h, method, tc.path)
				if response.Code != tc.status {
					t.Fatalf("status=%d, want %d", response.Code, tc.status)
				}
				for _, rule := range rules {
					want := ""
					if strings.Contains(" "+tc.headers+" ", " "+rule.Name+" ") {
						want = "yes"
					}
					if got := response.Header().Get(rule.Name); got != want {
						t.Errorf("%s=%q, want %q", rule.Name, got, want)
					}
				}
				if method == http.MethodHead && tc.status == http.StatusOK && response.Body.Len() != 0 {
					t.Error("HEAD emitted a body")
				}
				if method == http.MethodGet && tc.status == 200 && response.Body.String() != tc.body {
					t.Errorf("body=%q, want %q", response.Body.String(), tc.body)
				}
			})
		}
	}
}

func TestHeaderPatternsPreserveRouteContract(t *testing.T) {
	h, _ := newTestHandler(t, Site{
		Headers: []appv1alpha1.StaticHeader{
			{Path: "/*", Name: "X-Policy", Value: "global"},
			{Path: "/**/*.yaml", Name: "X-Policy", Value: "nested"},
			{Path: "/*.yaml", Name: "X-Root", Value: "yes"},
			{Path: "/index.html", Name: "X-Destination", Value: "yes"},
		},
		Routes: []appv1alpha1.StaticRoute{
			{Type: "redirect", Source: "/*.yaml", Destination: "/should-not-match"},
			{Type: "redirect", Source: "/old/*", Destination: "/archive/:splat"},
			{Type: "rewrite", Source: "/*", Destination: "/index.html"},
		},
	}, map[string]Object{
		key("index.html"):  {Body: []byte("rewritten")},
		key("render.yaml"): {Body: []byte("original")},
	})
	for _, tc := range []struct {
		path, body, policy, root, location string
		status                             int
	}{
		{"/render.yaml", "original", "global", "yes", "", 200},
		{"/missing.yaml", "rewritten", "global", "yes", "", 200},
		{"/nested/missing.yaml", "rewritten", "nested", "", "", 200},
		{"/old/deep/file.yaml", "", "nested", "", "/archive/deep/file.yaml", 301},
	} {
		for _, method := range []string{http.MethodGet, http.MethodHead} {
			t.Run(method+tc.path, func(t *testing.T) {
				response := do(h, method, tc.path)
				if response.Code != tc.status || response.Header().Get("Location") != tc.location {
					t.Fatalf("response=%d location=%q, want %d %q", response.Code, response.Header().Get("Location"), tc.status, tc.location)
				}
				for name, want := range map[string]string{"X-Policy": tc.policy, "X-Root": tc.root, "X-Destination": ""} {
					if got := response.Header().Get(name); got != want {
						t.Errorf("%s=%q, want %q", name, got, want)
					}
				}
				if method == http.MethodGet && tc.status == 200 && response.Body.String() != tc.body {
					t.Errorf("body=%q, want %q", response.Body.String(), tc.body)
				}
				if method == http.MethodHead && tc.status == http.StatusOK && response.Body.Len() != 0 {
					t.Error("HEAD emitted body")
				}
			})
		}
	}
}

// Unsupported multi-star syntax keeps the existing exact/trailing-wildcard
// contract; brackets are literal filename characters, not character classes.
func TestHeaderPatternsEncodedNamesAndLiteralMetacharacters(t *testing.T) {
	for _, tc := range []struct {
		pattern, path string
		matches       bool
	}{
		{"/*.css", "/caf%C3%A9.css", true},
		{"/café.css", "/caf%C3%A9.css", true},
		{"/**/*.css", "/assets/%E9%9B%AA.css", true},
		{"/*.css", "/assets/%E9%9B%AA.css", false},
		{"/[ab].css", "/%5Bab%5D.css", true},
		{"/[ab].css", "/a.css", false},
		{"/**.css", "/%2A%2A.css", true},
		{"/**.css", "/anything.css", false},
		{"/a**/*", "/a%2A%2A/file.css", true},
		{"/a**/*", "/abc/file.css", false},
	} {
		t.Run(tc.pattern+tc.path, func(t *testing.T) {
			decoded, err := url.PathUnescape(tc.path)
			if err != nil {
				t.Fatal(err)
			}
			h, _ := newTestHandler(t, Site{Headers: []appv1alpha1.StaticHeader{{Path: tc.pattern, Name: "X-Selected", Value: "yes"}}}, map[string]Object{
				key(strings.TrimPrefix(decoded, "/")): {Body: []byte("original file")},
			})
			response := do(h, http.MethodGet, tc.path)
			if response.Code != http.StatusOK || response.Body.String() != "original file" {
				t.Fatalf("encoded file response=%d %q", response.Code, response.Body.String())
			}
			if got := response.Header().Get("X-Selected"); (got == "yes") != tc.matches {
				t.Errorf("X-Selected=%q, want match=%v", got, tc.matches)
			}
		})
	}
}
