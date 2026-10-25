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

func TestRouteNamedPlaceholders(t *testing.T) {
	routes := []appv1alpha1.StaticRoute{
		{Type: "redirect", Source: "/blog/posts/:postid", Destination: "/blog/:postid"},
		{Type: "redirect", Source: "/updates/:month/:year", Destination: "/changelog/:year/:month"},
		{Type: "rewrite", Source: "/item/:id", Destination: "/other.html"},
		{Type: "redirect", Source: "/combo/:id/*", Destination: "/c/:id/*"},
		{Type: "redirect", Source: "/a:b", Destination: "/literal"},
	}
	for _, tc := range []struct {
		path string
		want string
		act  routeAction
	}{
		{"/blog/posts/my-post", "/blog/my-post", actRedirect},
		{"/updates/03/2024", "/changelog/2024/03", actRedirect},
		{"/item/7", "/other.html", actRewrite},
		{"/combo/9/x/y", "/c/9/x/y", actRedirect},
		{"/a:b", "/literal", actRedirect},
	} {
		act, target, err := matchRoutes(routes, tc.path)
		if err != nil || act != tc.act || target == nil || target.Path != tc.want {
			t.Errorf("%s: act=%v target=%v err=%v, want %v %q", tc.path, act, target, err, tc.act, tc.want)
		}
	}
	for _, path := range []string{"/blog/posts/", "/blog/posts/a/b", "/blog/posts"} {
		if act, _, _ := matchRoutes(routes, path); act != actNone {
			t.Errorf("%s: matched, want no match", path)
		}
	}
}

func TestRouteNamedPlaceholderEncodedSegmentStaysPathData(t *testing.T) {
	routes := []appv1alpha1.StaticRoute{{Type: "redirect", Source: "/blog/:id", Destination: "/blog/:id"}}
	_, target, err := matchRoutes(routes, "/blog/a?b")
	if err != nil || target == nil {
		t.Fatalf("target=%v err=%v", target, err)
	}
	if got := target.String(); got != "/blog/a%3Fb" {
		t.Fatalf("location path = %q, want /blog/a%%3Fb", got)
	}
}

func TestRouteNamedPlaceholderUndeclaredStaysLiteral(t *testing.T) {
	routes := []appv1alpha1.StaticRoute{{Type: "redirect", Source: "/a/:x", Destination: "/b/:y"}}
	_, target, err := matchRoutes(routes, "/a/1")
	if err != nil || target == nil || target.Path != "/b/:y" {
		t.Fatalf("target=%v err=%v, want literal /b/:y", target, err)
	}
}

func TestRouteNamedPlaceholderExpansionBudget(t *testing.T) {
	routes := []appv1alpha1.StaticRoute{{Type: "redirect", Source: "/a/:x", Destination: "/" + strings.Repeat(":x/", 250)}}
	if _, _, err := matchRoutes(routes, "/a/"+strings.Repeat("a", 100)); err == nil {
		t.Fatal("oversized named-capture expansion accepted")
	}
}

func TestHeaderPatternTreatsNamedPlaceholderLiterally(t *testing.T) {
	if matchHeaderPattern("/blog/:id", "/blog/7") {
		t.Fatal("header pattern matched :id as a placeholder")
	}
	if !matchHeaderPattern("/blog/:id", "/blog/:id") {
		t.Fatal("header pattern did not match :id literally")
	}
}

func TestRouteNamedPlaceholderLosesToExistingObject(t *testing.T) {
	h, _ := newTestHandler(t, Site{Routes: []appv1alpha1.StaticRoute{
		{Type: "rewrite", Source: "/item/:id", Destination: "/other.html"},
	}}, map[string]Object{
		key("item/7"):     {Body: []byte("real")},
		key("other.html"): {Body: []byte("page2")},
	})
	if resp := do(h, http.MethodGet, "/item/7"); resp.Body.String() != "real" {
		t.Fatalf("existing object not served: %q", resp.Body.String())
	}
	if resp := do(h, http.MethodGet, "/item/8"); resp.Body.String() != "page2" {
		t.Fatalf("placeholder rewrite not served: %q", resp.Body.String())
	}
}
