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

func TestRouteCaptureRedirectURLComponents(t *testing.T) {
	for _, alias := range []string{":splat", "*"} {
		for _, tc := range []struct{ encoded, decoded string }{
			{"a%3Fb.yaml", "a?b.yaml"},
			{"a%23b.yaml", "a#b.yaml"},
			{"a%25b.yaml", "a%b.yaml"},
			{"a%20b.yaml", "a b.yaml"},
			{"caf%C3%A9.yaml", "café.yaml"},
			{"a%253Fb.yaml", "a%3Fb.yaml"},
			{"a%2Fb.yaml", "a/b.yaml"},
			{"a/../b.yaml", "b.yaml"},
		} {
			for _, method := range []string{http.MethodGet, http.MethodHead} {
				t.Run(alias+"/"+method+"/"+tc.encoded, func(t *testing.T) {
					destination := "/docs/" + alias + "?download=1#section"
					h, _ := newTestHandler(t, Site{Routes: []appv1alpha1.StaticRoute{{Type: "redirect", Source: "/jump/*", Destination: destination}}}, map[string]Object{
						key("docs/" + tc.decoded): {Body: []byte("intended file")},
					})
					response := do(h, method, "/jump/"+tc.encoded+"?visitor=ignored")
					if response.Code != http.StatusMovedPermanently {
						t.Fatalf("status=%d body=%q", response.Code, response.Body.String())
					}
					location, err := url.Parse(response.Header().Get("Location"))
					if err != nil {
						t.Fatal(err)
					}
					if location.Path != "/docs/"+tc.decoded || location.RawQuery != "download=1" || location.Fragment != "section" || location.Host != "" {
						t.Fatalf("Location components=%+v", location)
					}
					followed := do(h, method, location.RequestURI())
					if followed.Code != http.StatusOK {
						t.Fatalf("following Location %q = %d", location.String(), followed.Code)
					}
					if method == http.MethodGet && followed.Body.String() != "intended file" {
						t.Fatalf("followed body=%q", followed.Body.String())
					}
					if method == http.MethodHead && followed.Body.Len() != 0 {
						t.Fatal("HEAD emitted body")
					}
				})
			}
		}
	}
}

func TestRouteCaptureRewriteUsesDecodedPathOnce(t *testing.T) {
	for _, destination := range []string{"/docs/:splat?download=1#section", "/docs/*?download=1#section"} {
		for _, tc := range []struct{ encoded, decoded string }{
			{"a%3Fb.yaml", "a?b.yaml"}, {"a%23b.yaml", "a#b.yaml"}, {"a%25b.yaml", "a%b.yaml"},
			{"a%20b.yaml", "a b.yaml"}, {"%E9%9B%AA.yaml", "雪.yaml"}, {"a%253Fb.yaml", "a%3Fb.yaml"},
		} {
			for _, method := range []string{http.MethodGet, http.MethodHead} {
				t.Run(destination+"/"+method+tc.encoded, func(t *testing.T) {
					h, origin := newTestHandler(t, Site{Routes: []appv1alpha1.StaticRoute{{Type: "rewrite", Source: "/source/*", Destination: destination}}}, map[string]Object{
						key("docs/" + tc.decoded): {Body: []byte("intended file")},
					})
					response := do(h, method, "/source/"+tc.encoded)
					if response.Code != http.StatusOK {
						t.Fatalf("rewrite=%d, origin keys=%v", response.Code, origin.gets)
					}
					if origin.gets[key("docs/"+tc.decoded)] != 1 {
						t.Fatalf("wrong origin key: %v", origin.gets)
					}
					if response.Header().Get("Location") != "" {
						t.Fatal("rewrite emitted redirect")
					}
					if method == http.MethodGet && response.Body.String() != "intended file" {
						t.Fatal("wrong rewritten object")
					}
				})
			}
		}
	}
	t.Run("encoded configured path", func(t *testing.T) {
		h, _ := newTestHandler(t, Site{Routes: []appv1alpha1.StaticRoute{{Type: "rewrite", Source: "/source", Destination: "/docs/a%3Fb.yaml?download=1#section"}}}, map[string]Object{key("docs/a?b.yaml"): {Body: []byte("configured file")}})
		response := do(h, http.MethodGet, "/source")
		if response.Code != http.StatusOK || response.Body.String() != "configured file" {
			t.Fatalf("configured rewrite=%d %q", response.Code, response.Body.String())
		}
	})
}

func TestRouteCaptureRedirectRetainsLocalTargetGuard(t *testing.T) {
	for _, capture := range []string{"%5Cevil.example", "%5C%5Cevil.example"} {
		for _, alias := range []string{":splat", "*"} {
			h, _ := newTestHandler(t, Site{Routes: []appv1alpha1.StaticRoute{{Type: "redirect", Source: "/jump/*", Destination: "/" + alias}}}, nil)
			response := do(h, http.MethodGet, "/jump/"+capture)
			if response.Code != http.StatusBadRequest || response.Header().Get("Location") != "" {
				t.Fatalf("unsafe %s via%s: %d %q", capture, alias, response.Code, response.Header().Get("Location"))
			}
		}
	}
	// An encoded percent is literal filename text, not another decoding pass.
	h, _ := newTestHandler(t, Site{Routes: []appv1alpha1.StaticRoute{{Type: "redirect", Source: "/jump/*", Destination: "/docs/:splat"}}}, nil)
	response := do(h, http.MethodGet, "/jump/%255Cevil.yaml")
	if response.Code != http.StatusMovedPermanently || !strings.Contains(response.Header().Get("Location"), "%255Cevil.yaml") {
		t.Fatalf("literal percent redecoded: %d %q", response.Code, response.Header().Get("Location"))
	}
}

func TestRouteCaptureDoesNotExpandQueryOrFragmentTokens(t *testing.T) {
	h, _ := newTestHandler(t, Site{Routes: []appv1alpha1.StaticRoute{{Type: "redirect", Source: "/jump/*", Destination: "/docs/:splat?q=:splat/*#:splat/*"}}}, nil)
	response := do(h, http.MethodGet, "/jump/a%3Fb.yaml?incoming=discarded")
	location, err := url.Parse(response.Header().Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	if response.Code != http.StatusMovedPermanently || location.Path != "/docs/a?b.yaml" || location.RawQuery != "q=:splat/*" || location.Fragment != ":splat/*" {
		t.Fatalf("destination components expanded outside path: %d %+v", response.Code, location)
	}
}

func TestRouteCaptureRejectsMalformedDirectCRDestinations(t *testing.T) {
	for _, destination := range []string{
		"/bad%", "/bad%2", "/local.yaml?q=%zz", "/local.yaml#bad%zz",
		"https://external.example/file.yaml", "//external.example/file.yaml",
		"/%2fexternal.example/file.yaml", "/%5cexternal.example/file.yaml",
		"/nested/%5cfile.yaml", "/%00file.yaml", "/%1ffile.yaml", "/%7ffile.yaml",
	} {
		for _, kind := range []string{"redirect", "rewrite"} {
			for _, method := range []string{http.MethodGet, http.MethodHead} {
				t.Run(kind+"/"+method+destination, func(t *testing.T) {
					h, origin := newTestHandler(t, Site{
						Routes: []appv1alpha1.StaticRoute{{Type: kind, Source: "/source.yaml", Destination: destination}},
					}, nil)
					response := do(h, method, "/source.yaml")
					if response.Code != http.StatusBadRequest || response.Header().Get("Location") != "" {
						t.Fatalf("invalid direct CR response=%d Location=%q", response.Code, response.Header().Get("Location"))
					}
					// Only the initial same-site object lookup may occur. A rejected
					// destination cannot reach another origin key or the SPA fallback.
					if len(origin.gets) != 1 || origin.gets[key("source.yaml")] != 1 {
						t.Fatalf("invalid destination reached origin: %v", origin.gets)
					}
				})
			}
		}
	}
	t.Run("ordinary local rewrite miss remains not found", func(t *testing.T) {
		h, origin := newTestHandler(t, Site{
			Routes: []appv1alpha1.StaticRoute{{Type: "rewrite", Source: "/source.yaml", Destination: "/missing.yaml"}},
		}, nil)
		response := do(h, http.MethodGet, "/source.yaml")
		if response.Code != http.StatusNotFound || response.Header().Get("Location") != "" || origin.gets[key("missing.yaml")] != 1 {
			t.Fatalf("local miss response=%d Location=%q lookups=%v", response.Code, response.Header().Get("Location"), origin.gets)
		}
	})
}
