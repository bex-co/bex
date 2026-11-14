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
	"reflect"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// leave_workspace_test.go covers w5/m102's exit verb: the deliberate way out
// that w5/m101's CANNOT_REMOVE_SELF refusal points at. The real-infrastructure
// half (row+tuple both gone against live Postgres and OpenFGA, and the
// post-leave resolution) lives in internal/api/members_leave_e2e_test.go.

// leaveFixture seeds a workspace whose creator is owner-1, with admin-2 as a
// second admin and dev-1 a developer, so the last-admin rule cannot be what
// explains an unexpected refusal.
func leaveFixture(t *testing.T) *fakeStore {
	t.Helper()
	st := newFakeStore(store.PlanPro)
	st.ownerSubject = "owner-1"
	st.seedMember("owner-1", "admin")
	st.seedMember("admin-2", "admin")
	st.seedMember("dev-1", "developer")
	return st
}

func TestLeaveWorkspaceDropsTheCallersRowAndTuple(t *testing.T) {
	st := leaveFixture(t)
	g := newFakeGranter()
	g.tuples[key("developer", "tea-1", "user:dev-1")] = true
	s := svc(st, g, nil, nil)

	if err := s.LeaveWorkspace(ctxWith("dev-1"), "tea-1"); err != nil {
		t.Fatalf("leave: %v", err)
	}
	if _, ok := st.members["dev-1"]; ok {
		t.Error("membership row survived the leave")
	}
	if g.tuples[key("developer", "tea-1", "user:dev-1")] {
		t.Error("role tuple survived the leave")
	}
	// Everyone else is untouched — leaving is not a removal of the workspace.
	if len(st.members) != 2 {
		t.Errorf("other members affected: %v", st.members)
	}
}

// The workspace's creator leaves like any admin, as on Render (w5/m372): the
// membership ends and the onboarding binding with it.
func TestLeaveWorkspaceLetsTheCreatorLeave(t *testing.T) {
	st := leaveFixture(t)
	s := svc(st, newFakeGranter(), nil, nil)

	if err := s.LeaveWorkspace(ctxWith("owner-1"), "tea-1"); err != nil {
		t.Fatalf("creator leaving with another admin: %v", err)
	}
	if _, ok := st.members["owner-1"]; ok {
		t.Error("the creator's membership survived the leave")
	}
	if st.ownerSubject != "" {
		t.Errorf("owner binding = %q after the creator left, want none", st.ownerSubject)
	}
}

func TestLeaveWorkspaceRefusesTheLastAdmin(t *testing.T) {
	// One admin, one developer: the admin leaving would leave a workspace nobody
	// can administer, which is the pre-existing floor, reused unchanged.
	st := newFakeStore(store.PlanPro)
	st.seedMember("admin-1", "admin")
	st.seedMember("dev-1", "developer")
	s := svc(st, newFakeGranter(), nil, nil)

	if err := s.LeaveWorkspace(ctxWith("admin-1"), "tea-1"); !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("last admin leaving: %v, want ErrBadRequest", err)
	}
	if _, ok := st.members["admin-1"]; !ok {
		t.Error("last admin left anyway")
	}
	// The developer can still leave — the floor is about admins, not about
	// locking everyone in.
	if err := s.LeaveWorkspace(ctxWith("dev-1"), "tea-1"); err != nil {
		t.Fatalf("developer leaving a single-admin workspace: %v", err)
	}
}

func TestLeaveWorkspaceRefusesANonMember(t *testing.T) {
	st := leaveFixture(t)
	s := svc(st, newFakeGranter(), nil, nil)
	if err := s.LeaveWorkspace(ctxWith("stranger"), "tea-1"); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("non-member leaving: %v, want ErrNotFound", err)
	}
}

// The ordering Remove established is load-bearing here too: a failed key
// disposal must leave the membership intact and no tuple revoked, so a retry
// converges instead of stranding a live credential over a dead membership.
func TestLeaveWorkspaceKeyDisposalFailureStaysRetryable(t *testing.T) {
	st := leaveFixture(t)
	g := newFakeGranter()
	keys := newFakeKeyDisposer()
	keys.err = errors.New("hydra unavailable")
	s := svc(st, g, nil, nil)
	s.Keys = keys

	err := s.LeaveWorkspace(ctxWith("dev-1"), "tea-1")
	if err == nil {
		t.Fatal("leave reported success over a failed key revocation")
	}
	if !strings.Contains(err.Error(), "API keys") {
		t.Errorf("error does not name the cause: %v", err)
	}
	if _, ok := st.members["dev-1"]; !ok {
		t.Error("row deleted despite the failure — a retry can no longer reach it")
	}
	if len(g.revoked) != 0 {
		t.Errorf("tuple revoked despite the failure: %v", g.revoked)
	}
}

// A failed tuple revoke must likewise stop before the row is deleted: the row
// is the only handle a retry has.
func TestLeaveWorkspaceRevokeFailureKeepsTheRow(t *testing.T) {
	st := leaveFixture(t)
	g := newFakeGranter()
	g.revokeErr = errors.New("openfga unavailable")
	s := svc(st, g, nil, nil)

	if err := s.LeaveWorkspace(ctxWith("dev-1"), "tea-1"); err == nil {
		t.Fatal("leave reported success over a failed tuple revoke")
	}
	if _, ok := st.members["dev-1"]; !ok {
		t.Error("row deleted while the tuple revoke failed")
	}
}

func TestLeaveWorkspaceDisposesTheCallersKeysInThatWorkspaceOnly(t *testing.T) {
	st := leaveFixture(t)
	keys := newFakeKeyDisposer()
	keys.seed("tea-1", "dev-1", "key-1", "key-2")
	keys.seed("tea-1", "admin-2", "key-admin")
	keys.seed("tea-other", "dev-1", "key-elsewhere")
	s := svc(st, newFakeGranter(), nil, nil)
	s.Keys = keys

	if err := s.LeaveWorkspace(ctxWith("dev-1"), "tea-1"); err != nil {
		t.Fatalf("leave: %v", err)
	}
	if got := keys.live("tea-1", "dev-1"); len(got) != 0 {
		t.Errorf("the leaver's keys in this workspace survived: %v", got)
	}
	if got := keys.live("tea-other", "dev-1"); len(got) != 1 {
		t.Errorf("the leaver's keys in ANOTHER workspace were disposed: %v", got)
	}
	if got := keys.live("tea-1", "admin-2"); len(got) != 1 {
		t.Errorf("a remaining member's keys were disposed: %v", got)
	}
}

// The audit trail must distinguish leaving from being removed — same end state,
// different story, and the events feed is where that difference is read.
func TestLeaveWorkspaceRecordsItsOwnAuditVerb(t *testing.T) {
	st := leaveFixture(t)
	sink := &recordingAuditSink{}
	s := svc(st, newFakeGranter(), nil, nil)
	s.Base.Audit = sink

	if err := s.LeaveWorkspace(ctxWith("dev-1"), "tea-1"); err != nil {
		t.Fatalf("leave: %v", err)
	}
	var left *core.AuditEvent
	for i := range sink.events {
		if sink.events[i].Verb == core.AuditVerbMemberLeft {
			left = &sink.events[i]
		}
		if sink.events[i].Verb == core.AuditVerbMemberRemoved {
			t.Error("leaving recorded itself as members.Remove")
		}
	}
	if left == nil {
		t.Fatalf("no %s audit row; got %+v", core.AuditVerbMemberLeft, sink.events)
	}
	if left.Caller != "dev-1" || left.Target != core.MemberTarget("dev-1") {
		t.Errorf("audit row caller/target = %q/%q, want the leaving member on both", left.Caller, left.Target)
	}
}

// The cache eviction is the m13 lesson: after leaving, the caller's next
// request must not resolve into the workspace they just left.
func TestLeaveWorkspaceEvictsTheCallersResolution(t *testing.T) {
	st := leaveFixture(t)
	resolver := &recordingResolver{}
	s := svc(st, newFakeGranter(), nil, nil)
	s.Base.Workspace = resolver

	if err := s.LeaveWorkspace(ctxWith("dev-1"), "tea-1"); err != nil {
		t.Fatalf("leave: %v", err)
	}
	if !reflect.DeepEqual(resolver.invalidatedTenants, []string{"dev-1"}) {
		t.Errorf("tenant resolution not evicted: %v", resolver.invalidatedTenants)
	}
	if !reflect.DeepEqual(resolver.invalidatedMemberships, []string{"dev-1:tea-1"}) {
		t.Errorf("membership positive not evicted: %v", resolver.invalidatedMemberships)
	}
}

// A refused leave must evict nothing — eviction is a consequence of actually
// having left, and a spurious one would churn every caller's resolution.
func TestRefusedLeaveEvictsNothing(t *testing.T) {
	st := newFakeStore(store.PlanPro)
	st.seedMember("admin-1", "admin")
	st.seedMember("dev-1", "developer")
	resolver := &recordingResolver{}
	s := svc(st, newFakeGranter(), nil, nil)
	s.Base.Workspace = resolver

	if err := s.LeaveWorkspace(ctxWith("admin-1"), "tea-1"); err == nil {
		t.Fatal("the last admin's leave was not refused")
	}
	if len(resolver.invalidatedTenants) != 0 || len(resolver.invalidatedMemberships) != 0 {
		t.Errorf("refused leave evicted caches: %v %v",
			resolver.invalidatedTenants, resolver.invalidatedMemberships)
	}
}

// recordingResolver is a core.WorkspaceResolver that records the invalidation
// calls the leave path makes, matching tenantService's two seams.
type recordingResolver struct {
	invalidatedTenants     []string
	invalidatedMemberships []string
}

func (r *recordingResolver) Tenant(context.Context, core.Identity) (string, bool) {
	return "tea-1", true
}

func (r *recordingResolver) IsMember(context.Context, core.Identity, string) (bool, error) {
	return true, nil
}

func (r *recordingResolver) InvalidateTenant(subject string) {
	r.invalidatedTenants = append(r.invalidatedTenants, subject)
}

func (r *recordingResolver) InvalidateMembership(subject, tenantID string) {
	r.invalidatedMemberships = append(r.invalidatedMemberships, subject+":"+tenantID)
}

// recordingSessions is the agent-session seam: it records whose sessions were
// stopped, and whether the role tuple and the row were still there when.
type recordingSessions struct {
	g        *fakeGranter
	st       *fakeStore
	err      error
	creators [][]string
	hadRole  bool
	hadRow   bool
}

func (r *recordingSessions) CancelMemberSessions(_ context.Context, workspaceID string, creators []string) error {
	r.creators = append(r.creators, creators)
	r.hadRole = r.g.tuples[key("developer", workspaceID, "user:"+creators[0])]
	_, r.hadRow = r.st.members[creators[0]]
	return r.err
}

// w5/186: every membership exit stops the agent sessions the member started
// in that workspace, directly or through the keys the exit revoked. It runs
// after the keys and the role tuple are gone, so neither can start another,
// and before the row, so a failure leaves a member a retry can find.
func TestMembershipExitsStopTheMembersAgentSessions(t *testing.T) {
	exits := map[string]func(*Service) error{
		"remove": func(s *Service) error { return s.Remove(ctxWith("admin-2"), "tea-1", "dev-1") },
		"leave":  func(s *Service) error { return s.LeaveWorkspace(ctxWith("dev-1"), "tea-1") },
	}
	for name, exit := range exits {
		t.Run(name, func(t *testing.T) {
			st := leaveFixture(t)
			g := newFakeGranter()
			g.tuples[key("developer", "tea-1", "user:dev-1")] = true
			sessions := &recordingSessions{g: g, st: st}
			s := svc(st, g, nil, nil)
			keys := newFakeKeyDisposer()
			keys.seed("tea-1", "dev-1", "key-1")
			s.Keys = keys
			s.Sessions = sessions
			if err := exit(s); err != nil {
				t.Fatalf("%s: %v", name, err)
			}
			if !reflect.DeepEqual(sessions.creators, [][]string{{"dev-1", "key-1"}}) {
				t.Errorf("sessions stopped for %v, want the member and the key they created", sessions.creators)
			}
			if sessions.hadRole || !sessions.hadRow {
				t.Errorf("stopped with role tuple %v and row %v, want after the revoke and before the row delete", sessions.hadRole, sessions.hadRow)
			}
		})
		t.Run(name+" fails closed", func(t *testing.T) {
			st := leaveFixture(t)
			g := newFakeGranter()
			s := svc(st, g, nil, nil)
			s.Sessions = &recordingSessions{g: g, st: st, err: errors.New("database unavailable")}
			if err := exit(s); err == nil {
				t.Fatal("a failed session stop was swallowed")
			}
			if _, ok := st.members["dev-1"]; !ok {
				t.Error("a failed session stop still deleted the row; a retry could not find the member")
			}
		})
	}
}

func TestAccountOffboarderStopsTheMembersAgentSessions(t *testing.T) {
	st := leaveFixture(t)
	g := newFakeGranter()
	g.tuples[key("developer", "tea-1", "user:dev-1")] = true
	sessions := &recordingSessions{g: g, st: st}
	off := AccountOffboarder{Store: accountStoreOver{st}, Revoker: g, Sessions: sessions}
	if err := off.Remove(context.Background(), "tea-1", "dev-1"); err != nil {
		t.Fatalf("offboard: %v", err)
	}
	if !reflect.DeepEqual(sessions.creators, [][]string{{"dev-1"}}) || sessions.hadRole || !sessions.hadRow {
		t.Errorf("stopped %v (role %v, row %v), want dev-1 after the revoke and before the row delete", sessions.creators, sessions.hadRole, sessions.hadRow)
	}

	failing := AccountOffboarder{Store: accountStoreOver{st}, Revoker: g, Sessions: &recordingSessions{g: g, st: st, err: errors.New("database unavailable")}}
	if err := failing.Remove(context.Background(), "tea-1", "admin-2"); err == nil {
		t.Fatal("a failed session stop was swallowed")
	}
	if _, ok := st.members["admin-2"]; !ok {
		t.Error("a failed session stop still deleted the row")
	}
}

// refusingTwoFactorGate stands in for a workspace that requires 2FA and a
// member who has none: it refuses every workspace.
type refusingTwoFactorGate struct{}

func (refusingTwoFactorGate) Check(_ context.Context, workspaceID string, _ core.Identity) error {
	return core.NewWorkspaceTwoFactorRequiredError(workspaceID)
}

// w22/m402: leaving is one of the operations a blocked member can still do
// (docs/ADR012-auth.md §9a), so a workspace that requires two-factor
// authentication does not trap the members it blocks. Removing someone else is
// a workspace operation like any other and stays blocked.
func TestLeaveWorkspaceIsExemptFromTheTwoFactorRequirement(t *testing.T) {
	st := leaveFixture(t)
	s := svc(st, newFakeGranter(), nil, nil)
	s.Base.TwoFactor = refusingTwoFactorGate{}

	if err := s.LeaveWorkspace(ctxWith("dev-1"), "tea-1"); err != nil {
		t.Fatalf("a blocked member leaving = %v, want allowed", err)
	}
	if _, ok := st.members["dev-1"]; ok {
		t.Error("membership row survived the leave")
	}

	var coded *core.CodedError
	err := s.Remove(ctxWith("admin-2"), "tea-1", "owner-1")
	if !errors.As(err, &coded) || coded.Code != core.CodeWorkspaceTwoFactorRequired {
		t.Fatalf("a blocked admin removing a member = %v, want the coded 403", err)
	}
}

// The exemption belongs to the leave verb alone. It covers only the caller's
// own membership, never someone who is not a member, and it does not outlive the
// call: the context a blocked member reuses afterwards is still refused.
func TestTheLeaveExemptionDoesNotLeakToOtherOperations(t *testing.T) {
	st := leaveFixture(t)
	s := svc(st, newFakeGranter(), nil, nil)
	s.Base.TwoFactor = refusingTwoFactorGate{}
	wantCoded403 := func(what string, err error) {
		t.Helper()
		var coded *core.CodedError
		if !errors.As(err, &coded) || coded.Code != core.CodeWorkspaceTwoFactorRequired {
			t.Fatalf("%s = %v, want the coded WORKSPACE_2FA_REQUIRED 403", what, err)
		}
	}

	// Someone who is not a member gets nothing out of the exempt route.
	if err := s.LeaveWorkspace(ctxWith("stranger"), "tea-1"); err == nil {
		t.Fatal("a non-member leaving = nil, want a refusal")
	}
	if len(st.members) != 3 {
		t.Fatalf("members after a stranger's leave = %v, want all three untouched", st.members)
	}

	// After a blocked admin leaves, the same context is not an exempt one.
	ctx := ctxWith("admin-2")
	if err := s.LeaveWorkspace(ctx, "tea-1"); err != nil {
		t.Fatalf("a blocked admin leaving = %v", err)
	}
	wantCoded403("Remove on the context that just left", s.Remove(ctx, "tea-1", "dev-1"))
	if _, ok := st.members["dev-1"]; !ok {
		t.Fatal("a blocked caller removed another member through the context of an exempt call")
	}
	wantCoded403("List members on the same context", func() error { _, err := s.List(ctx, "tea-1"); return err }())
}
