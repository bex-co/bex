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

package workspaces

import (
	"context"
	"slices"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

// organization_downgrade_test.go covers how a blocked org downgrade names
// people (w7/087): by email, and on a plan with a member cap, everyone the
// plan cannot seat. TestOrganizationDowngradeNamesPeopleE2E walks the same
// rules through GraphQL on a real Postgres.

func TestUnseatedKeepsTheCallerThenAdmins(t *testing.T) {
	members := []store.TenantMember{
		{Subject: "viewer", Role: "viewer"},
		{Subject: "other-admin", Role: "admin"},
		{Subject: "caller", Role: "admin"},
		{Subject: "developer", Role: "developer"},
	}
	invites := []store.Invite{{Email: "a@example.com"}, {Email: "b@example.com"}}
	subjects := func(ms []store.TenantMember) []string {
		var out []string
		for _, m := range ms {
			out = append(out, m.Subject)
		}
		return out
	}
	emails := func(invs []store.Invite) []string {
		var out []string
		for _, inv := range invs {
			out = append(out, inv.Email)
		}
		return out
	}
	for _, tc := range []struct {
		name         string
		limit        int
		caller       string
		wantMembers  []string
		wantInvitees []string
	}{
		// Hobby: the caller keeps the one seat; everyone else is over it, in
		// the order they joined, and so is every pending invite.
		{"hobby", 1, "caller", []string{"viewer", "other-admin", "developer"}, []string{"a@example.com", "b@example.com"}},
		// A caller who is no member (a platform operator) leaves the seat to
		// the first admin who joined.
		{"caller not a member", 1, "operator", []string{"viewer", "caller", "developer"}, []string{"a@example.com", "b@example.com"}},
		{"two seats", 2, "caller", []string{"viewer", "developer"}, []string{"a@example.com", "b@example.com"}},
		// Seats the members leave over go to the invites, oldest first.
		{"seats left for one invite", 5, "caller", nil, []string{"b@example.com"}},
		{"room for everyone", 6, "caller", nil, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			over, overInvites := unseated(members, invites, tc.limit, tc.caller)
			if got := subjects(over); !slices.Equal(got, tc.wantMembers) {
				t.Errorf("members over the limit = %v, want %v", got, tc.wantMembers)
			}
			if got := emails(overInvites); !slices.Equal(got, tc.wantInvitees) {
				t.Errorf("invites over the limit = %v, want %v", got, tc.wantInvitees)
			}
		})
	}
}

func TestMemberRolesNamesByEmailElseSubject(t *testing.T) {
	s := &Service{Identities: fakeIdentities{"sub-1": {Email: "vic@example.com"}}}
	members := []store.TenantMember{{Subject: "sub-1", Role: "viewer"}, {Subject: "sub-unknown", Role: "billing"}}
	want := []string{"vic@example.com:viewer", "sub-unknown:billing"}
	if got := s.memberRoles(context.Background(), members); !slices.Equal(got, want) {
		t.Fatalf("memberRoles = %v, want %v", got, want)
	}
	// No IdP wired: every member by subject.
	if got := (&Service{}).memberRoles(context.Background(), members); !slices.Equal(got, []string{"sub-1:viewer", "sub-unknown:billing"}) {
		t.Fatalf("memberRoles with no IdP = %v", got)
	}
}
