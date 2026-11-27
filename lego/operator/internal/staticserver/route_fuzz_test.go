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
	"regexp"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/bex-co/bex/lego/types/routepattern"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// routeFuzzSeeds are the real route shapes the package's tests exercise, plus
// the edges the matcher documents: a whole-site splat, a prefix splat, exact
// sources, whole-segment placeholders, and a placeholder before a splat. Seeds
// live as source so a reviewer sees exactly what the untrusted surface is
// probed with; the on-disk testdata/fuzz cache is never committed.
func routeFuzzSeeds(f *testing.F) {
	f.Helper()
	type seed struct {
		source, dest, path string
		typ                string
	}
	for _, s := range []seed{
		{"/*", "/index.html", "/", "rewrite"},
		{"/*", "/index.html", "/a/b/c", "rewrite"},
		{"/v", "/version.txt", "/v", "rewrite"},
		{"/x", "/blog", "/x", "rewrite"},
		{"/alias.js", "/app.js", "/alias.js", "rewrite"},
		{"/api/*", "/api/*", "/api/v1/users", "rewrite"},
		{"/blog/:id", "/blog/:id", "/blog/42", "redirect"},
		{"/blog/:id", "/posts/:id", "/blog/42", "rewrite"},
		{"/blog/:id/*", "/posts/:id/*", "/blog/42/comments", "rewrite"},
		{"/files/:name", "https://cdn.example.com/:name", "/files/a.txt", "redirect"},
		{"/:a/:b", "/:a/:b", "/x/y", "redirect"},
		// malformed / edge shapes.
		{"", "", "", "rewrite"},
		{"/*", "", "/", "rewrite"},
		{"/*", "/*", "/", "rewrite"},
		{"/:x/*", "/:x/*", "/a", "redirect"},
		{"/a/:b", "/:c", "/a/b", "redirect"},
		{"/*", "/:splat:splat:splat", "/aaaaaaaaaaaaaaaaaaaaaaaa", "rewrite"},
		{"/*", "//evil.example", "/", "redirect"},
		{"/*", "http://evil.example/*", "/", "rewrite"},
		{"/x", "\\evil", "/x", "redirect"},
		// Render's wildcard anywhere in the path (w4/m698).
		{"/blog*", "/posts*", "/blog/2024/x", "redirect"},
		{"/old-*", "/new-*", "/old-page", "redirect"},
		{"/*/feed", "/feeds/*.xml", "/a/b/feed", "rewrite"},
		{"/*.html", "/*/", "/a/b.html", "redirect"},
		{"/guides/*", "/*/index.html", "/guides/intro", "rewrite"},
		{"/*/:id", "/items/:id/*", "/shop/7", "redirect"},
		{"/*/:id/*", "/:id/*/*", "/a/7/b", "redirect"},
		{"/*/x/*", "/*/y/*", "/a/x/b/x/c", "redirect"},
		{"/a**b", "/**", "/a/c/b", "rewrite"},
		{"/:id*", "/*", "/:id-x", "redirect"},
		{"/a*a", "/*", "/a", "redirect"},
		{"*/:id/*/:id", "/:id", "/0//1", "redirect"}, // a repeated name keeps its last segment
	} {
		f.Add(s.typ, s.source, s.dest, s.path)
	}
}

// FuzzMatchRoutes drives the route matcher over arbitrary request paths and
// route patterns. Both halves are untrusted at different times: the path comes
// from a public visitor, the pattern/destination from a customer's CR. A
// malformed pair must answer "no match" or an error, never panic, and a match
// must never expand past the matcher's own size bound.
func FuzzMatchRoutes(f *testing.F) {
	routeFuzzSeeds(f)
	f.Fuzz(func(t *testing.T, typ, source, dest, reqPath string) {
		routes := []appv1alpha1.StaticRoute{{Type: typ, Source: source, Destination: dest}}
		action, target, err := matchRoutes(routes, reqPath)
		if err != nil {
			return
		}
		if action == actNone {
			if target != nil {
				t.Fatalf("actNone with a non-nil target for source %q path %q", source, reqPath)
			}
			return
		}
		if target == nil {
			t.Fatalf("action %v with a nil target for source %q path %q", action, source, reqPath)
		}
		if len(target.Path) > maxExpandedRoutePathBytes {
			t.Fatalf("expanded path exceeds the size bound: %d bytes", len(target.Path))
		}
	})
}

// FuzzRouteSourceMatchesReference checks matchRouteSource against a second,
// independent reading of Render's rule: a regular expression in which each
// "*" is a lazy any-string group and each whole ":name" segment one
// non-empty segment, anchored at both ends. Both must agree on whether a path
// matches and on every capture. Inputs stay short enough that the work
// budget's floor covers them.
func FuzzRouteSourceMatchesReference(f *testing.F) {
	routeFuzzSeeds(f)
	f.Fuzz(func(t *testing.T, _, source, _, reqPath string) {
		if len(source) > 128 || len(reqPath) > 128 || !utf8.ValidString(source) || !utf8.ValidString(reqPath) {
			return
		}
		re, groups := routeSourceReference(source)
		stars, names, ok := matchRouteSource(source, reqPath)
		m := re.FindStringSubmatch(reqPath)
		if ok != (m != nil) {
			t.Fatalf("source %q on %q: matched=%v, reference %v", source, reqPath, ok, m != nil)
		}
		if !ok {
			return
		}
		wantStars, wantNames := []string{}, map[string]string{}
		for i, group := range groups {
			if group == "*" {
				wantStars = append(wantStars, m[i+1])
			} else {
				wantNames[group] = m[i+1]
			}
		}
		if !slices.Equal(stars, wantStars) || !maps.Equal(names, wantNames) {
			t.Fatalf("source %q on %q: stars %q names %v, reference %q %v", source, reqPath, stars, names, wantStars, wantNames)
		}
	})
}

// routeSourceReference translates a route source into the reference regular
// expression, returning the group each submatch fills: "*" or a ":name".
func routeSourceReference(source string) (*regexp.Regexp, []string) {
	var b strings.Builder
	var groups []string
	b.WriteString(`(?s)^`)
	for i, seg := range strings.Split(source, "/") {
		if i > 0 {
			b.WriteByte('/')
		}
		if name, ok := routepattern.PlaceholderName(seg); ok {
			b.WriteString(`([^/]+)`)
			groups = append(groups, name)
			continue
		}
		for j, part := range strings.Split(seg, "*") {
			if j > 0 {
				b.WriteString(`(.*?)`)
				groups = append(groups, "*")
			}
			b.WriteString(regexp.QuoteMeta(part))
		}
	}
	b.WriteByte('$')
	return regexp.MustCompile(b.String()), groups
}

// FuzzMatchHeaderPattern drives the custom-header selector matcher with the same
// untrusted request path and a customer's path pattern. The selector must never
// panic on a malformed glob or a path that ends at a directory slash.
func FuzzMatchHeaderPattern(f *testing.F) {
	for _, s := range [][2]string{
		{"/*", "/"},
		{"/**/*.css", "/a/b/app.css"},
		{"/**/*.css", "/app.css"},
		{"/**/*", "/dir/"},
		{"/assets/*.js", "/assets/app.js"},
		{"/assets/*.js", "/assets/sub/app.js"},
		{"*", ""},
		{"", ""},
		{"/a/*/b", "/a/x/b"},
		{"/**/*", ""},
		{"/*", "/"},
	} {
		f.Add(s[0], s[1])
	}
	f.Fuzz(func(t *testing.T, pattern, reqPath string) {
		_ = matchHeaderPattern(pattern, reqPath)
	})
}
