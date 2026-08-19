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

package v1alpha1

import (
	"strings"
	"testing"
)

// TestBuildJobNameGoldenVectors pins the exact strings both modules must
// derive. The operator creates the Job and bex-api addresses it; a change here
// that is not a deliberate, coordinated migration orphans in-flight builds.
func TestBuildJobNameGoldenVectors(t *testing.T) {
	long := strings.Repeat("a", 70)
	cases := []struct{ name, app, revision, want string }{
		{"short", "web", "gen-3", "bld-web-gen-3"},
		{"uppercase is lowered", "Web", "gen-3", "bld-web-gen-3"},
		{"empty revision defaults", "web", "", "bld-web-latest"},
		{"at the limit", strings.Repeat("a", 53), "gen-1", "bld-" + strings.Repeat("a", 53) + "-gen-1"},
		{"over the limit is hash-suffixed", long, "gen-1", "bld-" + strings.Repeat("a", 46) + "-a56dea413824"},
		{"same app, next generation differs", long, "gen-2", "bld-" + strings.Repeat("a", 46) + "-fce778e3d7d4"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := BuildJobName(c.app, c.revision)
			if got != c.want {
				t.Fatalf("BuildJobName(%q, %q) = %q, want %q", c.app, c.revision, got, c.want)
			}
			if len(got) > 63 {
				t.Fatalf("name %q is %d chars, exceeds the 63-char DNS label limit", got, len(got))
			}
		})
	}
}

// TestBuildJobNameTruncationKeepsRevisionsDistinct is the property the plain
// slice broke: past 63 characters a truncating derivation must still separate
// two revisions of the same App, or one build's Job would answer for another.
func TestBuildJobNameTruncationKeepsRevisionsDistinct(t *testing.T) {
	app := strings.Repeat("b", 80)
	first := BuildJobName(app, "gen-1")
	second := BuildJobName(app, "gen-2")
	if first == second {
		t.Fatalf("two revisions collapsed to one Job name: %q", first)
	}
	if len(first) > 63 || len(second) > 63 {
		t.Fatalf("names exceed the DNS label limit: %q (%d), %q (%d)", first, len(first), second, len(second))
	}
	if BuildJobName(app, "gen-1") != first {
		t.Fatal("BuildJobName is not deterministic")
	}
}
