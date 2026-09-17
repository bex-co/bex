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
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

// owner_attribution_test.go covers w5/m103 t004: the workspace's surfaced email
// must follow the OWNER BINDING, not "whichever admin happens to be oldest".
// Before this, removing a founding admin silently reassigned who a workspace
// appeared to belong to — on a field billing and support read.

// emailLookup is an IdentityReader over a subject→email map; a miss is the
// honest-omit path.
type emailLookup map[string]string

func (e emailLookup) Lookup(_ context.Context, subject string) (IdentityAttrs, bool) {
	email, ok := e[subject]
	if !ok {
		return IdentityAttrs{}, false
	}
	return IdentityAttrs{Email: email}, true
}

func TestOwnerEmailFollowsTheOwnerBindingNotTheOldestAdmin(t *testing.T) {
	st := newFakeStore()
	st.tenants["tea-1"] = store.Tenant{ID: "tea-1", Name: "acme", Plan: store.PlanPro}
	// founder is the OLDEST admin; owner is the workspace's actual owner and was
	// added later. Before the fix, ownerEmail answered founder@.
	st.members["tea-1"] = []store.TenantMember{
		{TenantID: "tea-1", Subject: "founder", Role: "admin", CreatedAt: time.Unix(1, 0)},
		{TenantID: "tea-1", Subject: "owner", Role: "admin", CreatedAt: time.Unix(2, 0)},
	}
	st.ownerSubjects["tea-1"] = "owner"

	s := &Service{Store: st, Identities: emailLookup{
		"founder": "founder@example.com",
		"owner":   "owner@example.com",
	}}

	if got := s.ownerEmail(context.Background(), "tea-1"); got != "owner@example.com" {
		t.Errorf("ownerEmail = %q, want the owner's address, not the oldest admin's", got)
	}
}

func TestOwnerEmailFallsBackToOldestAdminOnlyWithoutABinding(t *testing.T) {
	// A workspace created through CreateWorkspace has no owner binding at all,
	// so there is nobody to name; the oldest admin is a contact address, and it
	// is the only case where one is guessed.
	st := newFakeStore()
	st.tenants["tea-1"] = store.Tenant{ID: "tea-1", Name: "acme", Plan: store.PlanPro}
	st.members["tea-1"] = []store.TenantMember{
		{TenantID: "tea-1", Subject: "founder", Role: "admin", CreatedAt: time.Unix(1, 0)},
		{TenantID: "tea-1", Subject: "later", Role: "admin", CreatedAt: time.Unix(2, 0)},
	}

	s := &Service{Store: st, Identities: emailLookup{
		"founder": "founder@example.com",
		"later":   "later@example.com",
	}}

	if got := s.ownerEmail(context.Background(), "tea-1"); got != "founder@example.com" {
		t.Errorf("unbound workspace: ownerEmail = %q, want the oldest admin as a fallback", got)
	}
}

func TestOwnerEmailDoesNotSubstituteWhenTheOwnerIsUnresolvable(t *testing.T) {
	// The workspace HAS an owner whose Kratos identity does not resolve. Naming
	// the oldest admin here would assert that somebody else owns it — the exact
	// substitution this task removes. An honest empty is the right answer.
	st := newFakeStore()
	st.tenants["tea-1"] = store.Tenant{ID: "tea-1", Name: "acme", Plan: store.PlanPro}
	st.members["tea-1"] = []store.TenantMember{
		{TenantID: "tea-1", Subject: "founder", Role: "admin", CreatedAt: time.Unix(1, 0)},
	}
	st.ownerSubjects["tea-1"] = "vanished-owner"

	s := &Service{Store: st, Identities: emailLookup{"founder": "founder@example.com"}}

	if got := s.ownerEmail(context.Background(), "tea-1"); got != "" {
		t.Errorf("ownerEmail = %q, want \"\" — the owner is unresolvable, not somebody else", got)
	}
}
