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
	"strings"
	"testing"
)

// captureFuzzSeeds are the destination shapes the package's route tests
// exercise — a literal path, the splat rewrite, a redirect that splats, and a
// named capture — plus the malformed edges substituteCaptures must survive: an
// undeclared ":name" that stays literal, a dangling ":", adjacent tokens, a
// trailing slash-star with no path, a "*" mid-path and beside a token, and a
// repeated splat that would amplify a capture. Seeds live as source so a reviewer sees exactly what the untrusted
// surface is probed with; the on-disk testdata/fuzz cache is never committed.
func captureFuzzSeeds(f *testing.F) {
	f.Helper()
	seeds := []struct {
		dest, splat, name, value string
	}{
		{"/index.html", "", "", ""},
		{"/archive/:splat", "a/b", "", ""},
		{"https://cdn.example.com/maps/*", "x", "", ""},
		{"https://cdn.example.com/:splat", "x", "", ""},
		{"/blog/:slug", "y", "slug", "post"},
		{"/undeclared/:name", "y", "", ""},
		{"/:", "y", "", ""},
		{"/:splat:splat", "y", "", ""},
		{"/:a:b", "y", "a", "1"},
		{"/x/*", "", "", ""},
		{"/x/*", strings.Repeat("s", maxExpandedRoutePathBytes), "", ""},
		{"/:splat", strings.Repeat("s", maxExpandedRoutePathBytes), "", ""},
		{"/:splat", "", "splat", "z"},
		{"/a/*/*", "b", "", ""},
		{"/%2e%2e/*", "../x", "", ""},
		{"/new-*", "page", "", ""},
		{"/*/index.html", "docs", "", ""},
		{"/*:splat*", "a", "", ""},
		{"/" + strings.Repeat("*", 300), "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "", ""},
	}
	for _, seed := range seeds {
		f.Add(seed.dest, seed.splat, seed.name, seed.value)
	}
}

// FuzzSubstituteCaptures drives the capture substitution every static-site
// rewrite and redirect destination goes through. The destination is
// tenant-configured (a CR field, so a direct write reaches it) and the splat
// and captures come from the request path, so arbitrary inputs must never
// panic, never slice past the destination, and never expand past the bound the
// function promises. A destination with no token must come back unchanged.
func FuzzSubstituteCaptures(f *testing.F) {
	captureFuzzSeeds(f)
	f.Fuzz(func(t *testing.T, destPath, splat, name, value string) {
		captures := map[string]string{}
		if name != "" {
			captures[name] = value
		}
		out, err := substituteCaptures(destPath, []string{splat}, captures)
		if err != nil {
			return
		}
		if len(out) > maxExpandedRoutePathBytes {
			t.Fatalf("substituteCaptures(%q, %q, %v) expanded to %d bytes, past the %d bound", destPath, splat, captures, len(out), maxExpandedRoutePathBytes)
		}
		if !strings.ContainsAny(destPath, ":*") && out != destPath {
			t.Fatalf("substituteCaptures(%q, %q, %v) = %q, a token-free destination must stay literal", destPath, splat, captures, out)
		}
	})
}
