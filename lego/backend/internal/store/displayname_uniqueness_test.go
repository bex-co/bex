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

package store

import "testing"

// TestDisplayNameNeedsUniqueness pins the rule the rename write and the patch
// preflight share (w4/359): a clear and the service's own creation name never
// collide; any other name is checked.
func TestDisplayNameNeedsUniqueness(t *testing.T) {
	for _, tc := range []struct {
		displayName, name string
		want              bool
	}{
		{"", "web", false},
		{"web", "web", false},
		{"Customer API", "web", true},
	} {
		if got := DisplayNameNeedsUniqueness(tc.displayName, tc.name); got != tc.want {
			t.Errorf("DisplayNameNeedsUniqueness(%q, %q) = %v, want %v", tc.displayName, tc.name, got, tc.want)
		}
	}
}
