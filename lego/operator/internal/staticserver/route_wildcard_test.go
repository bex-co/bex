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
	"maps"
	"net/http"
	"slices"
	"strings"
	"testing"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// Render: "In Source, * matches any string that appears starting at that
// position in the path" (render.com/docs/redirects-rewrites § Wildcards,
// w4/m698), anywhere in the source and across "/".
func TestRouteSourceWildcardMatchesFromItsPosition(t *testing.T) {
	for _, tc := range []struct {
		source, path string
		stars        []string // nil: no match
		names        map[string]string
	}{
		{"/blog*", "/blog", []string{""}, nil},
		{"/blog*", "/blog-post", []string{"-post"}, nil},
		{"/blog*", "/blog/2024/x", []string{"/2024/x"}, nil},
		{"/blog*", "/blo", nil, nil},
		{"/blog*", "/x/blog", nil, nil},
		{"/*/feed", "/a/feed", []string{"a"}, nil},
		{"/*/feed", "/a/b/feed", []string{"a/b"}, nil},
		{"/*/feed", "/feed", nil, nil},
		{"/*/feed", "/a/feed/x", nil, nil},
		{"/*.html", "/a/b.html", []string{"a/b"}, nil},
		{"/*.html", "/x.html", []string{"x"}, nil},
		{"/*.html", "/x.htm", nil, nil},
		{"/old-*", "/old-page", []string{"page"}, nil},
		{"/old-*", "/old", nil, nil},
		// "/x/*" needs the slash: the bare "/x" never matches (w8/325).
		{"/x/*", "/x", nil, nil},
		{"/x/*", "/x/", []string{""}, nil},
		{"/x/*", "/x/a/b", []string{"a/b"}, nil},
		{"/*", "/", []string{""}, nil},
		{"/*", "/a/b", []string{"a/b"}, nil},
		{"/home", "/home", []string{}, nil},
		{"/home", "/home/", nil, nil},
		// A ":name" segment next to a "*", on either side.
		{"/:lang/*", "/en/docs", []string{"docs"}, map[string]string{"lang": "en"}},
		{"/:lang/*", "/en", nil, nil},
		{"/*/:id", "/a/b/7", []string{"a/b"}, map[string]string{"id": "7"}},
		{"/*/:id", "/a/b/", nil, nil},
		{"/*/:id", "/7", nil, nil},
		{"/:a/*.html", "/en/x/y.html", []string{"x/y"}, map[string]string{"a": "en"}},
		// A segment touching a "*" is literal text, not a placeholder.
		{"/:id*", "/:id-x", []string{"-x"}, nil},
		{"/:id*", "/7", nil, nil},
		// Several "*"s: each earlier one takes the shortest string that lets
		// the rest match, and the last one what is left.
		{"/*/x/*", "/a/x/b/x/c", []string{"a", "b/x/c"}, nil},
		{"/*/x/*", "/a/x", nil, nil},
		{"/a**b", "/a/c/b", []string{"", "/c/"}, nil},
		{"/*-*-*", "/a-b-c-d", []string{"a", "b", "c-d"}, nil},
		{"/*/:id/*", "/a/7/b", []string{"a", "b"}, map[string]string{"id": "7"}},
		{"/*/:id/*", "/a//b", nil, nil},
		// The text before the first "*" and after the last may not overlap.
		{"/a*a", "/a", nil, nil},
		{"/a*a", "/aa", []string{""}, nil},
	} {
		stars, names, ok := matchRouteSource(tc.source, tc.path)
		if want := tc.stars != nil; ok != want {
			t.Errorf("%s on %s: matched=%v, want %v (stars %q)", tc.source, tc.path, ok, want, stars)
			continue
		}
		if !ok {
			continue
		}
		if !slices.Equal(stars, tc.stars) {
			t.Errorf("%s on %s: stars=%q, want %q", tc.source, tc.path, stars, tc.stars)
		}
		if !maps.Equal(names, tc.names) {
			t.Errorf("%s on %s: names=%v, want %v", tc.source, tc.path, names, tc.names)
		}
	}
}

// Render: "In Destination, * applies the entire string captured by the
// wildcard in Source", wherever the "*" is (w4/m698).
func TestRouteDestinationWildcardExpandsInPlace(t *testing.T) {
	for _, tc := range []struct {
		source, destination, path, want string
	}{
		{"/old-*", "/new-*", "/old-page", "/new-page"},
		{"/*", "/*/index.html", "/docs", "/docs/index.html"},
		{"/guides/*", "/*/index.html", "/guides/intro", "/intro/index.html"},
		{"/*", "/blog/*", "/path1/path2", "/blog/path1/path2"},
		{"/*.htm", "/*.html", "/a/b.htm", "/a/b.html"},
		{"/blog*", "https://blog.example.com/posts*", "/blog/2024", "/posts/2024"},
		// The nth "*" takes the nth capture, the last past the source's count;
		// ":splat" is the first.
		{"/*/old/*", "/*/new/*", "/a/old/b/c", "/a/new/b/c"},
		{"/*/old/*", "/:splat/*", "/a/old/b", "/a/a"},
		{"/x/*", "/*/*", "/x/y", "/y/y"},
		{"/x/*", "/a/:splat/*", "/x/y", "/a/y/y"},
		{"/exact", "/a/*", "/exact", "/a/"},
		{"/*/:id", "/items/:id/*", "/shop/7", "/items/7/shop"},
		// A source ":splat" segment captures like any ":name", so a destination
		// ":splat" with no "*" capture to take expands it instead of dropping
		// it: "/blog/:splat" → "/new/:splat" sends "/blog/hello" to
		// "/new/hello", not "/new/".
		{"/blog/:splat", "/new/:splat", "/blog/hello", "/new/hello"},
		{"/:splat", "/new/:splat", "/hello", "/new/hello"},
		// Past the source's star count a "*" still takes the last capture, and
		// a ":splat" still takes the first star capture even when the source
		// also declares a ":splat" segment.
		{"/*/:splat", "/:splat", "/a/b", "/a"},
	} {
		routes := []appv1alpha1.StaticRoute{{Type: "redirect", Source: tc.source, Destination: tc.destination}}
		act, target, err := matchRoutes(routes, tc.path)
		if err != nil || act != actRedirect || target == nil || target.Path != tc.want {
			t.Errorf("%s → %s on %s: act=%v target=%v err=%v, want %q", tc.source, tc.destination, tc.path, act, target, err, tc.want)
		}
	}
}

func TestRouteDestinationWildcardKeepsTheExpansionBudget(t *testing.T) {
	for _, tc := range []struct {
		name, destination string
		capture           int
		size              int
		invalid           bool
	}{
		{name: "mid-path boundary", destination: "/new-*.html", capture: 8192 - 10, size: 8192},
		{name: "mid-path oversized", destination: "/new-*.html", capture: 8192 - 9, invalid: true},
		{name: "repeated star boundary", destination: "/*/*", capture: 4095, size: 8192},
		{name: "repeated star amplification", destination: "/" + strings.Repeat("*/", 250), capture: 100, invalid: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			routes := []appv1alpha1.StaticRoute{{Type: "rewrite", Source: "/*", Destination: tc.destination}}
			_, target, err := matchRoutes(routes, "/"+strings.Repeat("a", tc.capture))
			if tc.invalid {
				if err == nil {
					t.Fatalf("oversized expansion accepted: %d bytes", len(target.Path))
				}
				return
			}
			if err != nil || len(target.Path) != tc.size {
				t.Fatalf("err=%v size=%d, want %d", err, len(target.Path), tc.size)
			}
		})
	}
}

// The rules a site moved from Render brings answer as Render's do: a redirect
// to the expanded destination, a rewrite serving it at the visitor's path, and
// a path the source does not match falling through to the 404.
func TestRouteWildcardRulesServeLikeRender(t *testing.T) {
	h, _ := newTestHandler(t, Site{NoSPAFallback: true, Routes: []appv1alpha1.StaticRoute{
		{Type: "redirect", Source: "/old-*", Destination: "/new-*"},
		{Type: "rewrite", Source: "/guides/*", Destination: "/*/index.html"},
		{Type: "rewrite", Source: "/*.htm", Destination: "/*.html"},
	}}, map[string]Object{
		key("intro/index.html"): {Body: []byte("intro")},
		key("a/b.html"):         {Body: []byte("b page")},
	})
	for _, tc := range []struct {
		path, location, body string
		status               int
	}{
		{"/old-page", "/new-page", "", http.StatusMovedPermanently},
		{"/old-a/b", "/new-a/b", "", http.StatusMovedPermanently},
		{"/guides/intro", "", "intro", http.StatusOK},
		{"/a/b.htm", "", "b page", http.StatusOK},
		{"/old", "", "", http.StatusNotFound},
		{"/guides", "", "", http.StatusNotFound},
		{"/guides/missing", "", "", http.StatusNotFound},
	} {
		response := do(h, http.MethodGet, tc.path)
		if response.Code != tc.status || response.Header().Get("Location") != tc.location {
			t.Errorf("%s: %d Location=%q, want %d %q", tc.path, response.Code, response.Header().Get("Location"), tc.status, tc.location)
			continue
		}
		if tc.status == http.StatusOK && response.Body.String() != tc.body {
			t.Errorf("%s: body=%q, want %q", tc.path, response.Body.String(), tc.body)
		}
	}
}

// A source and path built to make a ":name" between two "*"s try every
// candidate give up within the work budget instead of reading the path once
// per candidate.
func TestRouteSourceWildcardWorkIsBounded(t *testing.T) {
	source := "/*/:a/" + strings.Repeat("b/", 900) + "z/*"
	path := strings.Repeat("/b", 200_000)
	if _, _, ok := matchRouteSource(source, path); ok {
		t.Fatal("pathological source matched")
	}
	work := routeMatchWorkFloor + routeMatchWorkPerByte*(len(path)+len(source))
	var names map[string]string
	piece := strings.TrimSuffix(strings.TrimPrefix(source, "/*"), "*")
	if _, _, ok := findPiece(piece, path, &names, &work); ok || work > 0 {
		t.Fatalf("findPiece ok=%v with work %d left, want the budget spent", ok, work)
	}
}
