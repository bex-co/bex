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
	"reflect"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// owner_self_guards_test.go covers the membership-invariant matrix
// (docs/ADR024-members.md): nobody acts on their own membership through the
// manage-members verbs (w5/m101), and the workspace's creator is an ordinary
// Admin whose binding ends with their admin membership (w5/m372). The
// real-Postgres half — no resurrection through onboarding — lives in
// internal/api/members_owner_pg_test.go.

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

// The workspace's creator is an ordinary Admin, as on Render (w5/m372):
// another admin may remove them, and the onboarding binding ends with the row,
// so EnsureTenant neither restores their admin role nor strands the account.
func TestRemovingTheCreatorEndsTheBinding(t *testing.T) {
	st := ownerWorkspace(t, "owner-1", "admin-2")
	g := newFakeGranter()
	s := svc(st, g, nil, nil)

	if err := s.Remove(ctxWith("admin-2"), "tea-1", "owner-1"); err != nil {
		t.Fatalf("remove the creator: %v", err)
	}
	if _, ok := st.members["owner-1"]; ok {
		t.Error("the creator's row survived the removal")
	}
	if st.ownerSubject != "" {
		t.Errorf("owner binding = %q after removal, want none", st.ownerSubject)
	}
	if len(g.revoked) == 0 {
		t.Error("the creator's role tuple was not revoked")
	}
}

// regrantOnRemove is onboarding's admin re-grant landing between Remove's
// revoke and the commit that ends the creator's binding.
type regrantOnRemove struct {
	*fakeStore
	g *fakeGranter
}

func (r regrantOnRemove) RemoveMember(ctx context.Context, tenantID, subject string) error {
	if err := r.g.GrantWorkspaceRole(ctx, tenantID, "user:"+subject, "admin"); err != nil {
		return err
	}
	return r.fakeStore.RemoveMember(ctx, tenantID, subject)
}

// tupleChecker authorizes the caller and answers role checks from the
// granter's tuples, so Remove's check-first revoke sees the re-grant.
type tupleChecker struct{ g *fakeGranter }

func (c tupleChecker) Check(ctx context.Context, subject, relation, object string) (bool, error) {
	if strings.HasPrefix(relation, "can_") {
		return true, nil
	}
	return c.g.Check(ctx, subject, relation, object)
}

// Onboarding re-checks the binding after a grant, which covers a grant after
// the removal commits; Remove revokes again after it commits, which covers one
// before (w5/m372).
func TestRemoveTakesBackAnAdminTupleRegrantedBeforeItCommits(t *testing.T) {
	st := ownerWorkspace(t, "owner-1", "admin-2")
	g := newFakeGranter()
	g.tuples[key("admin", "tea-1", "user:owner-1")] = true
	s := svc(regrantOnRemove{st, g}, g, nil, tupleChecker{g})

	if err := s.Remove(ctxWith("admin-2"), "tea-1", "owner-1"); err != nil {
		t.Fatalf("remove the creator: %v", err)
	}
	if g.tuples[key("admin", "tea-1", "user:owner-1")] {
		t.Error("the removed creator kept an admin tuple onboarding wrote before the removal committed")
	}
}

// Removing or demoting a member evicts their cached resolution: a creator's
// named this workspace as theirs, and their next request must re-resolve.
func TestRemoveAndDemotionEvictTheSubjectsResolution(t *testing.T) {
	for name, act := range map[string]func(*Service) error{
		"remove": func(s *Service) error { return s.Remove(ctxWith("admin-2"), "tea-1", "owner-1") },
		"demote": func(s *Service) error {
			_, err := s.ChangeRole(ctxWith("admin-2"), "tea-1", "owner-1", "developer")
			return err
		},
	} {
		t.Run(name, func(t *testing.T) {
			resolver := &recordingResolver{}
			s := svc(ownerWorkspace(t, "owner-1", "admin-2"), newFakeGranter(), nil, nil)
			s.Base.Workspace = resolver
			if err := act(s); err != nil {
				t.Fatalf("%s: %v", name, err)
			}
			if !reflect.DeepEqual(resolver.invalidatedTenants, []string{"owner-1"}) {
				t.Errorf("tenant resolution evicted for %v, want [owner-1]", resolver.invalidatedTenants)
			}
		})
	}
}

func TestDemotingTheCreatorEndsTheBinding(t *testing.T) {
	st := ownerWorkspace(t, "owner-1", "admin-2")
	s := svc(st, newFakeGranter(), nil, nil)

	if _, err := s.ChangeRole(ctxWith("admin-2"), "tea-1", "owner-1", "developer"); err != nil {
		t.Fatalf("demote the creator: %v", err)
	}
	if st.members["owner-1"].Role != "developer" {
		t.Errorf("creator role = %q, want developer", st.members["owner-1"].Role)
	}
	if st.ownerSubject != "" {
		t.Errorf("owner binding = %q after demotion, want none: onboarding would restore admin", st.ownerSubject)
	}
}

func TestChangeRoleAllowsReassertingTheOwnerAsAdmin(t *testing.T) {
	// Re-asserting admin keeps the binding: the creator is still an admin, and
	// a partially failed reconciliation's repair path runs through here.
	st := ownerWorkspace(t, "owner-1", "admin-2")
	s := svc(st, newFakeGranter(), nil, nil)
	if _, err := s.ChangeRole(ctxWith("admin-2"), "tea-1", "owner-1", "admin"); err != nil {
		t.Fatalf("re-assert owner admin: %v", err)
	}
	if st.ownerSubject != "owner-1" {
		t.Errorf("owner binding = %q, want it kept for an admin", st.ownerSubject)
	}
}

func TestTheLastAdminStaysTheFloorForTheCreator(t *testing.T) {
	// The creator as the only admin cannot leave: the last-admin rule is the
	// floor for every admin, the creator included. (Nobody else could demote
	// or remove them, and guardSelf refuses doing either to yourself.)
	st := newFakeStore(store.PlanPro)
	st.ownerSubject = "owner-1"
	st.seedMember("owner-1", "admin")
	st.seedMember("dev-1", "developer")
	s := svc(st, newFakeGranter(), nil, nil)

	if err := s.LeaveWorkspace(ctxWith("owner-1"), "tea-1"); !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("the only admin leaving: %v, want the last-admin refusal", err)
	}
	if st.ownerSubject != "owner-1" || st.members["owner-1"].Role != "admin" {
		t.Fatalf("a refused leave changed the creator: binding %q role %q", st.ownerSubject, st.members["owner-1"].Role)
	}
}

func TestRemoveInAWorkspaceWithoutABinding(t *testing.T) {
	// A workspace minted through CreateWorkspace has a NULL binding.
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

// ADR086's account deletion removes the creator through the same store detach
// (store.RemoveAccountMember), so no dangling binding is left.
func TestAccountOffboarderRemovesTheCreator(t *testing.T) {
	st := ownerWorkspace(t, "owner-1", "admin-2")
	g := newFakeGranter()
	off := AccountOffboarder{Store: accountStoreOver{st}, Revoker: g}

	if err := off.Remove(context.Background(), "tea-1", "owner-1"); err != nil {
		t.Fatalf("offboard the creator: %v", err)
	}
	if _, ok := st.members["owner-1"]; ok {
		t.Error("offboarding left the creator's membership row")
	}
	if st.ownerSubject != "" {
		t.Errorf("owner binding after offboarding = %q, want none", st.ownerSubject)
	}
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
//     must run guardSelf (acting on yourself is not managing a teammate) and
//     guardLastAdmin.
//   - SELF-ONLY (no subject parameter, e.g. LeaveWorkspace): the subject comes
//     from the caller's identity, so guardSelf is meaningless; it must run
//     guardLastAdmin. A self-only verb that grew a subject parameter would fall
//     into the first branch and fail until it is guarded like one, which is the
//     escalation this pins shut.
//
// The workspace creator has no guard of its own since w5/m372: the store ends
// the onboarding binding with the membership (store.detachOwnerTx).
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
		var mutates, self, lastAdmin bool
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
				if f.Sel.Name == "guardLastAdmin" {
					lastAdmin = true
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
			if !self || !lastAdmin {
				t.Errorf("%s writes membership on a named subject without its guards: guardSelf=%v guardLastAdmin=%v",
					fn.Name.Name, self, lastAdmin)
			}
			continue
		}
		if !lastAdmin {
			t.Errorf("%s writes membership for the caller without the last-admin guard", fn.Name.Name)
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
