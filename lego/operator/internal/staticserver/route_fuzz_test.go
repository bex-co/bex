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
	"testing"

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
