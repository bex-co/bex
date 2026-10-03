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

func TestHeaderPatternLiteralBoundaries(t *testing.T) {
	for _, tc := range []struct {
		pattern, path string
		match         bool
	}{
		{"/*.css", "/.css", true},
		{"/**/*.css", "/assets/.css", true},
		{"/**/*", "/blog/", true},
		{"/**/*", "/blog", false},
		{"/blog/*", "/blog", true}, // Existing subtree compatibility.
		{"/*.css", "/theme.CSS", false},
		{"/*.css", "/a%2Fb.css", true}, // Input is already URL-decoded once.
		{"/*.css", "/a/b.css", false},
		{"/*.css", "/café.css", true},
		{"/café.css", "/café.css", false}, // No Unicode normalization.
		{"/*[ab].css", "/x[ab].css", true},
		{"/*[ab].css", "/xa.css", false},
		{"/*?.css", "/x?.css", true},
		{"/*?.css", "/xy.css", false},
		{"/*{a,b}.css", "/x{a,b}.css", true},
		{"/*{a,b}.css", "/xa.css", false},
		{`/*\.css`, `/x\.css`, true},
		{`/*\.css`, `/x.css`, false},
		{"/**.css", "/theme.css", false},
		{"/**.css", "/**.css", true},
		{"/***/theme.css", "/a/theme.css", false},
		{"/*/theme.css", "/a/theme.css", false},
		{"/assets/*.css", "/assets/theme.css", true},
		{"/assets/*.css", "/assets/*.css", true},
		{"/assets/*.css", "/assets/nested/theme.css", false},
		{"/a**/*", "/a**/file", true}, // Existing literal-prefix behavior.
		{"/a**/*", "/abc/file", false},
		{"/**/*.css/", "/a/theme.css/", false},
		{"*.css", "/theme.css", false},
		{"/*.css", "", false},
		{"/**/*.css", "", false},
	} {
		t.Run(tc.pattern+"->"+tc.path, func(t *testing.T) {
			if got := matchHeaderPattern(tc.pattern, tc.path); got != tc.match {
				t.Fatalf("match=%v want %v", got, tc.match)
			}
		})
	}
}

func TestHeaderPatternBoundedWithoutAllocations(t *testing.T) {
	// The accepted 2048-character pattern bound must not induce recursive glob
	// expansion or allocate matcher state per visitor request.
	pattern := "/**/*" + strings.Repeat("a", 2043)
	request := "/nested/" + strings.Repeat("a", 2043)
	if !matchHeaderPattern(pattern, request) {
		t.Fatal("maximum length suffix did not match")
	}
	if allocations := testing.AllocsPerRun(100, func() {
		for range 100 {
			_ = matchHeaderPattern(pattern, request)
		}
	}); allocations != 0 {
		t.Fatalf("100 rules allocate %v objects", allocations)
	}
	adversarial := "/" + strings.Repeat("**/", 680) + "*.css"
	if matchHeaderPattern(adversarial, request) {
		t.Fatal("unsupported repeated stars expanded into a glob")
	}
}

func BenchmarkHeaderPatternMaximumRules(b *testing.B) {
	pattern := "/**/*" + strings.Repeat("a", 2043)
	request := "/nested/" + strings.Repeat("a", 2043)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		for range 100 {
			_ = matchHeaderPattern(pattern, request)
		}
	}
}
