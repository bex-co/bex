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
	"maps"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// countingLookups is fakeIdentities counting its email reads.
type countingLookups struct {
	fakeIdentities
	batches int
}

func (c *countingLookups) LookupEmails(ctx context.Context, subjects []string) map[string]string {
	c.batches++
	return c.fakeIdentities.LookupEmails(ctx, subjects)
}

// TestOwnerListsReadEveryContactAtOnce (w5/126): listing W workspaces, or the
// owners of a resource page, makes one contact read and one identity lookup,
// not one of each per workspace. Each workspace keeps its own rule: a bound
// owner answers, an unbound workspace falls back to its oldest admin, and a
// bound owner who does not resolve leaves the email empty rather than naming
// another admin.
func TestOwnerListsReadEveryContactAtOnce(t *testing.T) {
	st := newFakeStore()
	identities := &countingLookups{fakeIdentities: fakeIdentities{"user-a": {Email: "a@example.com"}, "owner-b": {Email: "b@example.com"}}}
	svc := &Service{Base: &core.Base{Authz: &fakeChecker{allow: true}}, Store: st, Identities: identities}
	ids := map[string]string{}
	for _, name := range []string{"unbound", "bound", "vanished"} {
		w, err := svc.Create(ctxAs("user-a"), name, "pro")
		if err != nil {
			t.Fatal(err)
		}
		ids[name] = w.ID
	}
	st.ownerSubjects[ids["bound"]] = "owner-b"
	st.ownerSubjects[ids["vanished"]] = "owner-gone"
	want := map[string]string{"unbound": "a@example.com", "bound": "b@example.com", "vanished": ""}
	once := func(t *testing.T, list string) {
		t.Helper()
		if st.contactReads != 1 || identities.batches != 1 {
			t.Errorf("%s read contacts %d times and looked identities up %d times, want once each", list, st.contactReads, identities.batches)
		}
		st.contactReads, identities.batches = 0, 0
	}

	owners, err := svc.ListOwners(ctxAs("user-a"), OwnerFilter{})
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, owner := range owners {
		got[owner.Name] = owner.Email
	}
	if !maps.Equal(got, want) {
		t.Errorf("owner emails = %v, want %v", got, want)
	}
	once(t, "ListOwners")

	resolved := svc.ResolveResourceOwners(ctxAs("user-a"), []string{ids["unbound"], ids["bound"], ids["vanished"]})
	got = map[string]string{}
	for name, id := range ids {
		got[name] = resolved[id].Email
	}
	if len(resolved) != 3 || !maps.Equal(got, want) {
		t.Errorf("resource owner emails = %v, want %v", got, want)
	}
	once(t, "ResolveResourceOwners")
}
