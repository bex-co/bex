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

package members

import (
	"errors"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// machine_binding_test.go covers w5/m103's service half: the member verbs must
// refuse an API key's workspace binding and say where the key is managed. The
// store half (the row never reaching a member list or a seat count) is asserted
// against real Postgres in internal/store.
//
// The defect this closes was observed live on dev-5, 2026-09-17: PATCH on a
// bound key's subject returned 200 and promoted the machine to admin, writing a
// real workspace:admin tuple for it.

func machineFixture(t *testing.T) *fakeStore {
	t.Helper()
	st := newFakeStore(store.PlanPro)
	st.seedMember("admin-1", "admin")
	st.seedMember("admin-2", "admin")
	st.members["client-abc"] = store.TenantMember{
		TenantID: "tea-1", Subject: "client-abc", Role: "developer",
		Kind: store.MemberKindMachine,
	}
	return st
}

func TestChangeRoleRefusesAMachineBinding(t *testing.T) {
	st := machineFixture(t)
	g := newFakeGranter()
	s := svc(st, g, nil, nil)

	_, err := s.ChangeRole(ctxWith("admin-1"), "tea-1", "client-abc", "ADMIN")
	if !errors.Is(err, core.ErrConflict) || codedErrorCode(err) != ErrorMemberIsMachine {
		t.Fatalf("promote a key: %v code=%q, want ErrConflict/%s", err, codedErrorCode(err), ErrorMemberIsMachine)
	}
	if st.members["client-abc"].Role != "developer" {
		t.Errorf("key's role changed anyway: %q", st.members["client-abc"].Role)
	}
	// The live defect wrote a real admin tuple for the machine subject. Nothing
	// may reach OpenFGA on a refusal.
	if len(g.granted) != 0 || len(g.revoked) != 0 {
		t.Errorf("tuples written for a refused promotion: granted=%v revoked=%v", g.granted, g.revoked)
	}
	if !strings.Contains(err.Error(), "API keys") {
		t.Errorf("refusal does not point at the API-keys surface: %q", err.Error())
	}
}

func TestRemoveRefusesAMachineBinding(t *testing.T) {
	st := machineFixture(t)
	s := svc(st, newFakeGranter(), nil, nil)

	err := s.Remove(ctxWith("admin-1"), "tea-1", "client-abc")
	if !errors.Is(err, core.ErrConflict) || codedErrorCode(err) != ErrorMemberIsMachine {
		t.Fatalf("remove a key's binding: %v code=%q, want ErrConflict/%s", err, codedErrorCode(err), ErrorMemberIsMachine)
	}
	// The binding must survive: deleting it through the members surface left the
	// Hydra client alive but unbound — a key still listed on the API-keys page
	// that silently authorizes nothing.
	if _, ok := st.members["client-abc"]; !ok {
		t.Error("key's binding deleted through the members surface")
	}
}

func TestLeaveWorkspaceRefusesAMachineCaller(t *testing.T) {
	// An API key calling leave with its own credential would unbind itself
	// outside the surface that owns key lifecycle.
	st := machineFixture(t)
	s := svc(st, newFakeGranter(), nil, nil)

	err := s.LeaveWorkspace(core.WithIdentity(ctxWith("client-abc"), core.Identity{
		Subject: "client-abc", Method: "oauth2",
	}), "tea-1")
	if codedErrorCode(err) != ErrorMemberIsMachine {
		t.Fatalf("machine leaving: %v code=%q, want %s", err, codedErrorCode(err), ErrorMemberIsMachine)
	}
	if _, ok := st.members["client-abc"]; !ok {
		t.Error("key unbound itself through leave")
	}
}

// The refusal is about the row's KIND, not about the subject looking unusual:
// an ordinary member whose Kratos identity no longer resolves (w4/070) stays
// listed, mutable and removable. Conflating the two is the exact regression
// this milestone's ADR024 note warns about.
func TestUnresolvedHumanMembersStayMutable(t *testing.T) {
	st := newFakeStore(store.PlanPro)
	st.seedMember("admin-1", "admin")
	st.seedMember("ghost", "developer") // kind defaults to user
	s := svc(st, newFakeGranter(), nil, nil)
	s.Identities = fakeIdentities{} // every lookup misses

	ms, err := s.List(ctxWith("admin-1"), "tea-1")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	var ghost *MemberView
	for i := range ms {
		if ms[i].Subject == "ghost" {
			ghost = &ms[i]
		}
	}
	if ghost == nil {
		t.Fatal("an unresolved human was dropped from the member list")
	}
	if ghost.IdentityResolved {
		t.Error("identityResolved should stay false for an unresolved human")
	}
	if err := s.Remove(ctxWith("admin-1"), "tea-1", "ghost"); err != nil {
		t.Fatalf("removing an unresolved human: %v", err)
	}
}
