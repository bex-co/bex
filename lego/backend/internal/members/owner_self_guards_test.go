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
	"context"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// owner_self_guards_test.go covers w5/m101's membership-invariant matrix
// (docs/ADR024-members.md): the workspace owner cannot be removed or demoted,
// and nobody acts on their own membership through the manage-members verbs.
// The real-Postgres half — the resurrection path the owner rule exists to close
// — lives in internal/api/members_owner_pg_test.go.

// ownerWorkspace seeds a workspace whose owner binding names `owner`, with a
// second admin present so the last-admin rule never explains a refusal.
func ownerWorkspace(t *testing.T, owner, otherAdmin string) *fakeStore {
	t.Helper()
	st := newFakeStore(store.PlanPro)
	st.ownerSubject = owner
	st.seedMember(owner, "admin")
	st.seedMember(otherAdmin, "admin")
	return st
}

func TestRemoveRefusesTheWorkspaceOwner(t *testing.T) {
	st := ownerWorkspace(t, "owner-1", "admin-2")
	g := newFakeGranter()
	s := svc(st, g, nil, nil)

	err := s.Remove(ctxWith("admin-2"), "tea-1", "owner-1")
	if !errors.Is(err, core.ErrConflict) || codedErrorCode(err) != ErrorOwnerCannotBeRemoved {
		t.Fatalf("remove owner: %v code=%q, want ErrConflict/%s", err, codedErrorCode(err), ErrorOwnerCannotBeRemoved)
	}
	if _, ok := st.members["owner-1"]; !ok {
		t.Error("owner row deleted despite the refusal")
	}
	// The refusal must land before the revoke: a revoked tuple with a surviving
	// row is exactly the half-state the guard exists to prevent.
	if len(g.revoked) != 0 {
		t.Errorf("revoked tuples on a refused removal: %v", g.revoked)
	}
}

func TestChangeRoleRefusesDemotingTheWorkspaceOwner(t *testing.T) {
	st := ownerWorkspace(t, "owner-1", "admin-2")
	s := svc(st, newFakeGranter(), nil, nil)

	_, err := s.ChangeRole(ctxWith("admin-2"), "tea-1", "owner-1", "developer")
	if !errors.Is(err, core.ErrConflict) || codedErrorCode(err) != ErrorOwnerRoleCannotChange {
		t.Fatalf("demote owner: %v code=%q, want ErrConflict/%s", err, codedErrorCode(err), ErrorOwnerRoleCannotChange)
	}
	if st.members["owner-1"].Role != "admin" {
		t.Errorf("owner demoted anyway: %q", st.members["owner-1"].Role)
	}
}

func TestChangeRoleAllowsReassertingTheOwnerAsAdmin(t *testing.T) {
	// The guard protects the owner's admin standing, not the verb: another admin
	// re-asserting admin on the owner must still converge (the repair path a
	// partially-failed reconciliation depends on).
	st := ownerWorkspace(t, "owner-1", "admin-2")
	s := svc(st, newFakeGranter(), nil, nil)
	if _, err := s.ChangeRole(ctxWith("admin-2"), "tea-1", "owner-1", "admin"); err != nil {
		t.Fatalf("re-assert owner admin: %v", err)
	}
}

func TestOwnerGuardsAreInertWithoutAnOwnerBinding(t *testing.T) {
	// A workspace minted through CreateWorkspace has a NULL binding; the
	// last-admin rule is the only membership floor there, unchanged.
	st := newFakeStore(store.PlanPro)
	st.seedMember("admin-1", "admin")
	st.seedMember("admin-2", "admin")
	s := svc(st, newFakeGranter(), nil, nil)

	if err := s.Remove(ctxWith("admin-1"), "tea-1", "admin-2"); err != nil {
		t.Fatalf("remove in an unbound workspace: %v", err)
	}
	if _, ok := st.members["admin-2"]; ok {
		t.Error("row survived a permitted removal")
	}
}

func TestRemoveRefusesSelf(t *testing.T) {
	st := ownerWorkspace(t, "owner-1", "admin-2")
	s := svc(st, newFakeGranter(), nil, nil)

	err := s.Remove(ctxWith("admin-2"), "tea-1", "admin-2")
	if !errors.Is(err, core.ErrConflict) || codedErrorCode(err) != ErrorCannotRemoveSelf {
		t.Fatalf("self removal: %v code=%q, want ErrConflict/%s", err, codedErrorCode(err), ErrorCannotRemoveSelf)
	}
	if _, ok := st.members["admin-2"]; !ok {
		t.Error("caller removed themselves anyway")
	}
	if !strings.Contains(err.Error(), "leave the workspace") {
		t.Errorf("refusal does not name the recovery path: %q", err.Error())
	}
}

func TestChangeRoleRefusesOwnRole(t *testing.T) {
	st := ownerWorkspace(t, "owner-1", "admin-2")
	s := svc(st, newFakeGranter(), nil, nil)

	for _, role := range []string{"developer", "admin"} {
		// Both directions: a self-demotion and a self "promotion" to the role the
		// caller already holds. Neither is a teammate-management action.
		_, err := s.ChangeRole(ctxWith("admin-2"), "tea-1", "admin-2", role)
		if !errors.Is(err, core.ErrConflict) || codedErrorCode(err) != ErrorCannotChangeOwnRole {
			t.Fatalf("self role change to %s: %v code=%q, want ErrConflict/%s",
				role, err, codedErrorCode(err), ErrorCannotChangeOwnRole)
		}
	}
	if st.members["admin-2"].Role != "admin" {
		t.Errorf("caller changed their own role anyway: %q", st.members["admin-2"].Role)
	}
}

func TestSelfGuardCoversMachineCallers(t *testing.T) {
	// tenant_members holds Hydra client ids alongside Kratos identity ids, so an
	// API key removing its own binding row is the same self-removal.
	st := newFakeStore(store.PlanPro)
	st.seedMember("admin-1", "admin")
	st.seedMember("client-abc", "developer")
	s := svc(st, newFakeGranter(), nil, nil)

	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "client-abc", Method: "oauth2"})
	if err := s.Remove(ctx, "tea-1", "client-abc"); codedErrorCode(err) != ErrorCannotRemoveSelf {
		t.Fatalf("machine self removal: %v code=%q", err, codedErrorCode(err))
	}
}

func TestAccountOffboarderStillRemovesTheOwner(t *testing.T) {
	// ADR086's trusted path is exempt by construction — a different type, no
	// caller identity, and the store clears the owner binding in the same
	// transaction (store.RemoveAccountMember), so no dangling owner is left.
	st := ownerWorkspace(t, "owner-1", "admin-2")
	g := newFakeGranter()
	off := AccountOffboarder{Store: offboardStore{st}, Revoker: g}

	if err := off.Remove(context.Background(), "tea-1", "owner-1"); err != nil {
		t.Fatalf("offboard owner: %v", err)
	}
	if _, ok := st.members["owner-1"]; ok {
		t.Error("offboarding left the owner's membership row")
	}
}

// offboardStore adapts the members fake to AccountMemberStore, mirroring
// PGStore.RemoveAccountMember: the row goes AND the owner binding clears.
type offboardStore struct{ *fakeStore }

func (o offboardStore) RemoveAccountMember(_ context.Context, _, subject string) error {
	if _, ok := o.members[subject]; !ok {
		return store.ErrNotFound
	}
	delete(o.members, subject)
	if o.ownerSubject == subject {
		o.fakeStore.ownerSubject = ""
	}
	return nil
}

func TestListMarksTheOwnerAndTheCallersOwnRow(t *testing.T) {
	st := ownerWorkspace(t, "owner-1", "admin-2")
	st.seedMember("dev-1", "developer")
	s := svc(st, newFakeGranter(), nil, nil)

	ms, err := s.List(ctxWith("admin-2"), "tea-1")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	bySubject := map[string]MemberView{}
	for _, m := range ms {
		bySubject[m.Subject] = m
	}
	if len(bySubject) != 3 {
		t.Fatalf("members = %d, want 3", len(bySubject))
	}
	if !bySubject["owner-1"].IsOwner || bySubject["owner-1"].IsSelf {
		t.Errorf("owner row: isOwner=%v isSelf=%v", bySubject["owner-1"].IsOwner, bySubject["owner-1"].IsSelf)
	}
	if bySubject["admin-2"].IsOwner || !bySubject["admin-2"].IsSelf {
		t.Errorf("caller row: isOwner=%v isSelf=%v", bySubject["admin-2"].IsOwner, bySubject["admin-2"].IsSelf)
	}
	if bySubject["dev-1"].IsOwner || bySubject["dev-1"].IsSelf {
		t.Errorf("teammate row wrongly flagged: %+v", bySubject["dev-1"])
	}
}

// TestMemberMutatingVerbsRunTheGuards is the durable form of the rule (the
// ADR024 contributor-boundary precedent): a member-mutating verb added later
// must run the guards its shape calls for, or this sweep fails naming it. It
// reads service.go's AST rather than the behavior of known verbs, so the
// failure arrives with the NEW verb, not with a test nobody thought to extend.
//
// Two shapes are legal, distinguished by whether the verb names its target:
//
//   - TEAMMATE-TARGETED (takes a `subject` parameter, e.g. Remove/ChangeRole):
//     must run guardSelf — acting on yourself is not managing a teammate — and
//     guardOwner.
//   - SELF-ONLY (no subject parameter, e.g. LeaveWorkspace): the subject comes
//     from the caller's identity, so guardSelf is meaningless; it must instead
//     run the leaving owner guard. A self-only verb that grew a subject
//     parameter would fall into the first branch and fail until it is guarded
//     like one, which is the escalation this pins shut.
func TestMemberMutatingVerbsRunTheGuards(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "service.go", nil, 0)
	if err != nil {
		t.Fatalf("parse service.go: %v", err)
	}
	// The store writes that change who belongs to a workspace and with what
	// power, plus the shared teardown they were factored into — a verb must not
	// be able to shed its guards by calling the helper instead of the store
	// directly (this sweep caught exactly that during the w5/m102 refactor).
	// endMembership itself is unexported and is the teardown, not the gate: the
	// loop below only demands guards of EXPORTED verbs, which is where the
	// authorization decision belongs.
	mutators := map[string]bool{
		"RemoveMember": true, "UpdateMemberRole": true, "endMembership": true,
	}

	checked := 0
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		var mutates, self, owner, ownerLeaving bool
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			switch f := call.Fun.(type) {
			case *ast.SelectorExpr:
				if mutators[f.Sel.Name] {
					mutates = true
				}
				switch f.Sel.Name {
				case "guardOwner":
					owner = true
				case "guardOwnerLeaving":
					ownerLeaving = true
				}
			case *ast.Ident:
				if f.Name == "guardSelf" {
					self = true
				}
			}
			return true
		})
		if !mutates || !fn.Name.IsExported() {
			continue
		}
		checked++
		if namesASubject(fn) {
			if !self || !owner {
				t.Errorf("%s writes membership on a named subject without the w5/m101 guards: guardSelf=%v guardOwner=%v",
					fn.Name.Name, self, owner)
			}
			continue
		}
		if !ownerLeaving {
			t.Errorf("%s writes membership for the caller without the w5/m102 owner guard", fn.Name.Name)
		}
	}
	if checked != 3 {
		t.Fatalf("membership-mutating verbs found = %d, want 3 (Remove, ChangeRole, LeaveWorkspace) — "+
			"a verb was added or renamed; extend the matrix in docs/ADR024-members.md with it", checked)
	}
}

// namesASubject reports whether the verb takes a caller-supplied subject — the
// difference between "manage this teammate" and "act on myself".
func namesASubject(fn *ast.FuncDecl) bool {
	for _, param := range fn.Type.Params.List {
		for _, name := range param.Names {
			if name.Name == "subject" {
				return true
			}
		}
	}
	return false
}
