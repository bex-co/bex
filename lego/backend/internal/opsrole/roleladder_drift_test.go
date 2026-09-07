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

package opsrole

import (
	"slices"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/members"
)

// TestRoleLadderMatchesMembersRoles is the drift guard for the inherent
// duplication between the authority-ordered roleLadder and the canonical
// members.Roles vocabulary (the ladder cannot alias it: the order here is
// opsrole-specific, and importing members into prod code would drag in store).
// A role added to members without a ladder entry would make this verb silently
// answer member:false for its holders — fail-closed, but an unexplained
// operator lockout with no signal.
func TestRoleLadderMatchesMembersRoles(t *testing.T) {
	want := append([]string(nil), members.Roles...)
	got := append([]string(nil), roleLadder[:]...)
	slices.Sort(want)
	slices.Sort(got)
	if !slices.Equal(want, got) {
		t.Fatalf("roleLadder %v and members.Roles %v have drifted", roleLadder, members.Roles)
	}
}
