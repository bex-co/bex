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

package build

import (
	"strings"
	"testing"
)

// TestJobNameCrossModuleContract pins the exact Job names this operator
// creates, so a future re-implementation on either side of the CR boundary
// fails here rather than silently orphaning in-flight builds. Changing these
// literals is a deliberate migration, not a test update.
func TestJobNameCrossModuleContract(t *testing.T) {
	cases := []struct{ name, app, revision, want string }{
		{"short name", "web", "gen-3", "bld-web-gen-3"},
		{"uppercase is lowered", "Web", "gen-3", "bld-web-gen-3"},
		{"empty revision defaults", "web", "", "bld-web-latest"},
		{"just under the limit", strings.Repeat("a", 53), "gen-1", "bld-" + strings.Repeat("a", 53) + "-gen-1"},
		{"just over the limit", strings.Repeat("a", 54), "gen-1", "bld-" + strings.Repeat("a", 46) + "-cffb98e74742"},
		{"far over the limit", strings.Repeat("b", 70), "gen-42", "bld-" + strings.Repeat("b", 46) + "-de75591f8608"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := JobName(c.app, c.revision)
			if got != c.want {
				t.Fatalf("JobName(%q, %q) = %q, want %q — bex-api pins this exact literal", c.app, c.revision, got, c.want)
			}
			if len(got) > 63 {
				t.Fatalf("name %q is %d chars, exceeds the 63-char DNS label limit", got, len(got))
			}
		})
	}
}
