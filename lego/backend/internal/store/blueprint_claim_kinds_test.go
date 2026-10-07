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

import (
	"regexp"
	"slices"
	"testing"
)

// TestBlueprintClaimKindsMatchTheMigrationsCheck (w5/115): the claim kinds are
// exactly the set blueprint_resource_claims_kind_chk admits, as the latest
// migration naming the constraint states it.
func TestBlueprintClaimKindsMatchTheMigrationsCheck(t *testing.T) {
	body := latestMatchingSQL(t, "blueprint_resource_claims_kind_chk")
	got := quotedList(t, body, regexp.MustCompile(`blueprint_resource_claims_kind_chk\s+CHECK\s*\(\s*kind\s+IN\s*\(([^)]*)\)`))
	var want []string
	for _, kind := range BlueprintClaimKinds() {
		want = append(want, string(kind))
	}
	if !slices.Equal(got, want) {
		t.Errorf("blueprint_resource_claims kind CHECK = %q, want the claim kinds %q", got, want)
	}
}
