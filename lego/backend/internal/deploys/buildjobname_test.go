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

package deploys

import (
	"strings"
	"testing"
)

// TestBuildJobNameAddressesTheOperatorsJob pins what this package composes: the
// bld- prefix and the "gen-<generation>" revision spelling the operator names
// build Jobs by. Cancel and the built-image lookup address that Job by name, so
// a mismatch means operating on a Job that does not exist. The truncation rule
// itself is pinned in the contract module both sides call.
func TestBuildJobNameAddressesTheOperatorsJob(t *testing.T) {
	if got, want := buildJobName("web", 3), "bld-web-gen-3"; got != want {
		t.Fatalf("buildJobName = %q, want %q", got, want)
	}
	if got, want := buildJobName("Web", 3), "bld-web-gen-3"; got != want {
		t.Fatalf("uppercase name = %q, want %q", got, want)
	}

	// A name past the DNS label limit must stay within it and still separate
	// generations; a plain slice used to do neither.
	long := strings.Repeat("a", 70)
	first, second := buildJobName(long, 1), buildJobName(long, 2)
	if len(first) > 63 {
		t.Fatalf("name %q is %d chars, exceeds the 63-char DNS label limit", first, len(first))
	}
	if first == second {
		t.Fatalf("generations 1 and 2 collapsed to one Job name %q", first)
	}
}
