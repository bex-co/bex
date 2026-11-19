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
	"slices"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/core/coretest"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// key_member_verbs_test.go is w9/116's service half: a bex API key changes a
// member's role and removes a member as Render's key with "Update member roles
// and remove members" does, acting for the person who minted it and never
// above them. The same journey on REST, GraphQL and MCP against real Postgres
// and OpenFGA is TestAPIKeyManagesMembersAsItsCreatorE2E (internal/api).

// roleRelations restates deploy/gitops/authz/model.fga's workspace grants.
var roleRelations = map[string][]string{
	"admin":       {core.RelCanView, core.RelCanViewLogs, core.RelCanOperate, core.RelCanCreate, core.RelCanViewSensitive, core.RelCanManageKeys, core.RelCanManageSSHKeys, core.RelCanManage, core.RelCanManageBilling},
	"developer":   {core.RelCanView, core.RelCanViewLogs, core.RelCanOperate, core.RelCanCreate, core.RelCanViewSensitive, core.RelCanManageKeys, core.RelCanManageSSHKeys},
	"contributor": {core.RelCanView, core.RelCanViewLogs, core.RelCanOperate, core.RelCanManageSSHKeys},
	"viewer":      {core.RelCanView, core.RelCanManageSSHKeys},
	"billing":     {core.RelCanView, core.RelCanManageSSHKeys, core.RelCanManageBilling},
}

// rowChecker answers OpenFGA from the fake store's rows, as a converged model
// would. stale pins a subject's tuple at a role its row no longer holds (a
// revoke that failed).
type rowChecker struct {
	st    *fakeStore
	stale map[string]string
}

func (c rowChecker) Check(_ context.Context, subject, relation, _ string) (bool, error) {
	subject = strings.TrimPrefix(subject, "user:")
	role, ok := c.stale[subject]
	if !ok {
		m, member := c.st.members[subject]
		if !member {
			return false, nil
		}
		role = m.Role
	}
	return slices.Contains(roleRelations[role], relation), nil
}

// allowAll holds every relation for every subject: the model taken out of the
// way, so a test sees what the member verbs decide on their own.
type allowAll struct{}

func (allowAll) Check(context.Context, string, string, string) (bool, error) { return true, nil }

// keyFixture is a Scale workspace with two Admins, a Developer, a Contributor, a
// Viewer and a Billing member, and one key per creator in keys (key id ->
// creator), each bound at its creator's role.
func keyFixture(keys map[string]string) (*fakeStore, coretest.KeyCreators) {
	st := newFakeStore(store.PlanScale)
	for subject, role := range map[string]string{
		"admin-1": "admin", "admin-2": "admin", "dev-1": "developer",
		"contrib-1": "contributor", "viewer-1": "viewer", "bill-1": "billing",
	} {
		st.seedMember(subject, role)
	}
	creators := coretest.KeyCreators{}
	for key, creator := range keys {
		creators[key] = creator
		st.members[key] = store.TenantMember{
			TenantID: "tea-1", Subject: key, Role: st.members[creator].Role, Kind: store.MemberKindMachine,
		}
	}
	return st, creators
}

func keyCtx(clientID string) context.Context {
	return core.WithIdentity(context.Background(), core.Identity{Subject: clientID, ClientID: clientID, Method: core.MethodOAuth2})
}

func keyService(st MembersStore, authz core.Checker, creators coretest.KeyCreators) *Service {
	s := svc(st, newFakeGranter(), nil, authz)
	s.Base.KeyCreators = creators
	return s
}

func TestAnAdminsKeyChangesRolesAndRemovesMembers(t *testing.T) {
	st, creators := keyFixture(map[string]string{"key-a": "admin-1"})
	s := keyService(st, rowChecker{st: st}, creators)

	if _, err := s.ChangeRole(keyCtx("key-a"), "tea-1", "dev-1", "ADMIN"); err != nil {
		t.Fatalf("an Admin's key promotes a Developer to Admin: %v", err)
	}
	if _, err := s.ChangeRole(keyCtx("key-a"), "tea-1", "viewer-1", "CONTRIBUTOR"); err != nil {
		t.Fatalf("an Admin's key promotes a Viewer: %v", err)
	}
	if _, err := s.ChangeRole(keyCtx("key-a"), "tea-1", "admin-2", "DEVELOPER"); err != nil {
		t.Fatalf("an Admin's key demotes another Admin: %v", err)
	}
	if err := s.Remove(keyCtx("key-a"), "tea-1", "bill-1"); err != nil {
		t.Fatalf("an Admin's key removes a member: %v", err)
	}
	for subject, want := range map[string]string{"dev-1": "admin", "viewer-1": "contributor", "admin-2": "developer"} {
		if got := st.members[subject].Role; got != want {
			t.Errorf("%s = %q, want %q", subject, got, want)
		}
	}
	if _, ok := st.members["bill-1"]; ok {
		t.Error("the removed member is still a member")
	}
}

func TestBelowAdminKeysCannotManageMembers(t *testing.T) {
	st, creators := keyFixture(map[string]string{
		"key-d": "dev-1", "key-c": "contrib-1", "key-v": "viewer-1", "key-b": "bill-1",
	})
	s := keyService(st, rowChecker{st: st}, creators)

	for key := range creators {
		if _, err := s.ChangeRole(keyCtx(key), "tea-1", "viewer-1", "DEVELOPER"); !errors.Is(err, core.ErrForbidden) {
			t.Errorf("%s ChangeRole = %v, want ErrForbidden", key, err)
		}
		if err := s.Remove(keyCtx(key), "tea-1", "contrib-1"); !errors.Is(err, core.ErrForbidden) {
			t.Errorf("%s Remove = %v, want ErrForbidden", key, err)
		}
	}
	if st.members["viewer-1"].Role != "viewer" || st.members["contrib-1"].Role != "contributor" {
		t.Fatalf("a refused key still changed membership: %+v", st.members)
	}
}

func TestAKeyCannotActOnItsCreatorOrAnyKeyBinding(t *testing.T) {
	st, creators := keyFixture(map[string]string{"key-a": "admin-1", "key-a2": "admin-2"})
	g := newFakeGranter()
	s := svc(st, g, nil, rowChecker{st: st})
	s.Base.KeyCreators = creators

	// Its creator, in either direction and whatever role is named: the key
	// acts for them, so this is them changing their own membership.
	for _, role := range []string{"ADMIN", "DEVELOPER", "VIEWER", "bogus"} {
		_, err := s.ChangeRole(keyCtx("key-a"), "tea-1", "admin-1", role)
		if !errors.Is(err, core.ErrConflict) || codedErrorCode(err) != ErrorCannotChangeOwnRole {
			t.Errorf("the key re-roles its creator to %s: %v code=%q, want %s", role, err, codedErrorCode(err), ErrorCannotChangeOwnRole)
		}
	}
	if err := s.Remove(keyCtx("key-a"), "tea-1", "admin-1"); !errors.Is(err, core.ErrConflict) || codedErrorCode(err) != ErrorCannotRemoveSelf {
		t.Errorf("the key removes its creator: %v code=%q, want %s", err, codedErrorCode(err), ErrorCannotRemoveSelf)
	}
	// Its own binding (guardSelf on the key's subject) and another key's
	// (guardMachine, I5).
	if _, err := s.ChangeRole(keyCtx("key-a"), "tea-1", "key-a", "VIEWER"); codedErrorCode(err) != ErrorCannotChangeOwnRole {
		t.Errorf("the key re-roles its own binding: %v", err)
	}
	if err := s.Remove(keyCtx("key-a"), "tea-1", "key-a"); codedErrorCode(err) != ErrorCannotRemoveSelf {
		t.Errorf("the key removes its own binding: %v", err)
	}
	if _, err := s.ChangeRole(keyCtx("key-a"), "tea-1", "key-a2", "VIEWER"); codedErrorCode(err) != ErrorMemberIsMachine {
		t.Errorf("the key re-roles another key's binding: %v", err)
	}
	if err := s.Remove(keyCtx("key-a"), "tea-1", "key-a2"); codedErrorCode(err) != ErrorMemberIsMachine {
		t.Errorf("the key removes another key's binding: %v", err)
	}
	if st.members["admin-1"].Role != "admin" || st.members["key-a"].Role != "admin" || st.members["key-a2"].Role != "admin" {
		t.Fatalf("a refused change still moved a row: %+v", st.members)
	}
	if len(g.granted)+len(g.revoked) != 0 {
		t.Fatalf("tuples written for refused changes: granted=%v revoked=%v", g.granted, g.revoked)
	}
	// The other Admin's key may act on admin-1: the guard is on the key's own
	// creator, not on Admins.
	if _, err := s.ChangeRole(keyCtx("key-a2"), "tea-1", "admin-1", "DEVELOPER"); err != nil {
		t.Fatalf("another Admin's key demotes admin-1: %v", err)
	}
}

// TestAKeyNeverReachesAboveItsCreatorsRole takes the model out of the way
// (every subject holds every relation), so what refuses is the member verbs'
// own rule: a key sets no role its creator's current row does not cover, and
// acts on no member whose role its creator's does not cover.
func TestAKeyNeverReachesAboveItsCreatorsRole(t *testing.T) {
	st, creators := keyFixture(map[string]string{"key-d": "dev-1", "key-b": "bill-1"})
	s := keyService(st, allowAll{}, creators)
	above := func(err error) bool {
		return errors.Is(err, core.ErrForbidden) && codedErrorCode(err) == ErrorRoleAboveKeyCreator
	}

	for _, role := range []string{"ADMIN", "BILLING"} {
		if _, err := s.ChangeRole(keyCtx("key-d"), "tea-1", "viewer-1", role); !above(err) {
			t.Errorf("a Developer's key sets %s: %v code=%q", role, err, codedErrorCode(err))
		}
	}
	if _, err := s.ChangeRole(keyCtx("key-d"), "tea-1", "admin-2", "VIEWER"); !above(err) {
		t.Errorf("a Developer's key demotes an Admin: %v", err)
	}
	if err := s.Remove(keyCtx("key-d"), "tea-1", "admin-2"); !above(err) {
		t.Errorf("a Developer's key removes an Admin: %v", err)
	}
	if err := s.Remove(keyCtx("key-d"), "tea-1", "bill-1"); !above(err) {
		t.Errorf("a Developer's key removes a Billing member: %v", err)
	}
	if _, err := s.ChangeRole(keyCtx("key-b"), "tea-1", "viewer-1", "DEVELOPER"); !above(err) {
		t.Errorf("a Billing member's key sets Developer: %v", err)
	}
	if st.members["viewer-1"].Role != "viewer" || st.members["admin-2"].Role != "admin" {
		t.Fatalf("a refused change still moved a row: %+v", st.members)
	}
	// Within the creator's role the rule does not refuse.
	if _, err := s.ChangeRole(keyCtx("key-d"), "tea-1", "viewer-1", "CONTRIBUTOR"); err != nil {
		t.Fatalf("a change within the creator's role: %v", err)
	}
	if err := s.Remove(keyCtx("key-d"), "tea-1", "contrib-1"); err != nil {
		t.Fatalf("a removal within the creator's role: %v", err)
	}
}

func TestADemotedCreatorsKeyLosesMemberManagementAtOnce(t *testing.T) {
	st, creators := keyFixture(map[string]string{"key-a": "admin-1"})
	s := keyService(st, rowChecker{st: st}, creators)
	if _, err := s.ChangeRole(keyCtx("key-a"), "tea-1", "viewer-1", "CONTRIBUTOR"); err != nil {
		t.Fatalf("before the demotion: %v", err)
	}
	if _, err := s.ChangeRole(ctxWith("admin-2"), "tea-1", "admin-1", "DEVELOPER"); err != nil {
		t.Fatalf("demote the creator: %v", err)
	}
	// The key's own row has not moved here (the fake store does not follow
	// creators); the ceiling asks its creator, who no longer manages members.
	if _, err := s.ChangeRole(keyCtx("key-a"), "tea-1", "viewer-1", "DEVELOPER"); !errors.Is(err, core.ErrForbidden) {
		t.Errorf("the demoted Admin's key re-roles = %v, want ErrForbidden", err)
	}
	if err := s.Remove(keyCtx("key-a"), "tea-1", "viewer-1"); !errors.Is(err, core.ErrForbidden) {
		t.Errorf("the demoted Admin's key removes = %v, want ErrForbidden", err)
	}
	if st.members["viewer-1"].Role != "contributor" {
		t.Fatalf("the demoted Admin's key still changed a role: %q", st.members["viewer-1"].Role)
	}
}

// TestAKeyWaitsForItsCreatorsRoleChange is round-19 #3 for the person behind
// a key: while the creator's demotion is still queued, their stranded admin
// tuple (and the key's) would pass every relation check, so the key is
// refused until the change converges.
func TestAKeyWaitsForItsCreatorsRoleChange(t *testing.T) {
	q := newQueuedFakeStore(store.PlanScale)
	st, creators := keyFixture(map[string]string{"key-a": "admin-1"})
	q.fakeStore = st
	s := keyService(q, rowChecker{st: st, stale: map[string]string{"admin-1": "admin", "key-a": "admin"}}, creators)
	q.queued[roleQueueKey("tea-1", "admin-1")] = store.RoleReconciliation{TenantID: "tea-1", Subject: "admin-1", Role: "developer"}
	m := st.members["admin-1"]
	m.Role = "developer"
	st.members["admin-1"] = m

	_, err := s.ChangeRole(keyCtx("key-a"), "tea-1", "viewer-1", "DEVELOPER")
	if !errors.Is(err, core.ErrForbidden) || !strings.Contains(err.Error(), "still being applied") {
		t.Fatalf("a key whose creator's demotion is queued = %v, want the pending refusal", err)
	}
	if err := s.Remove(keyCtx("key-a"), "tea-1", "viewer-1"); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("a key whose creator's demotion is queued removes = %v", err)
	}
	delete(q.queued, roleQueueKey("tea-1", "admin-1"))
	// Converged in the queue but the tuple is still stale: the row decides
	// what the key may set, so it still cannot promote above a Developer.
	if _, err := s.ChangeRole(keyCtx("key-a"), "tea-1", "viewer-1", "ADMIN"); codedErrorCode(err) != ErrorRoleAboveKeyCreator {
		t.Fatalf("a stale admin tuple lets the key promote to Admin: %v", err)
	}
}

func TestAKeyWithNoPersonBehindItCannotManageMembers(t *testing.T) {
	st, creators := keyFixture(map[string]string{"key-a": "admin-1"})
	creators["key-legacy"] = ""     // minted before the created-by stamp
	creators["key-chain"] = "key-a" // minted by a key, before codex round-7 F3
	creators["key-gone"] = "left-1" // its creator is no member here
	for _, key := range []string{"key-legacy", "key-chain", "key-gone"} {
		st.members[key] = store.TenantMember{TenantID: "tea-1", Subject: key, Role: "admin", Kind: store.MemberKindMachine}
	}
	s := keyService(st, allowAll{}, creators)
	for _, key := range []string{"key-legacy", "key-chain", "key-gone"} {
		if _, err := s.ChangeRole(keyCtx(key), "tea-1", "viewer-1", "DEVELOPER"); !errors.Is(err, core.ErrForbidden) {
			t.Errorf("%s ChangeRole = %v, want ErrForbidden", key, err)
		}
		if err := s.Remove(keyCtx(key), "tea-1", "viewer-1"); !errors.Is(err, core.ErrForbidden) {
			t.Errorf("%s Remove = %v, want ErrForbidden", key, err)
		}
	}
	if st.members["viewer-1"].Role != "viewer" {
		t.Fatalf("a key with no person behind it changed a role: %+v", st.members["viewer-1"])
	}
	// A client_credentials client that is not a bex API key is not held to a
	// creator: its own tuples decide, as before (an operator's bootstrap).
	if err := s.Remove(keyCtx("operator-client"), "tea-1", "contrib-1"); err != nil {
		t.Fatalf("an operator-provisioned client removes = %v", err)
	}
	// ...but it still may not re-role anyone: that is the export class.
	if _, err := s.ChangeRole(keyCtx("operator-client"), "tea-1", "viewer-1", "DEVELOPER"); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("an operator-provisioned client re-roles = %v, want ErrForbidden", err)
	}
}

// TestRoleChangesStillRefuseADelegatedThirdPartyToken keeps round 24's H-1
// beside the key widening: a human OAuth token from a client bex did not
// provision is not a key and still makes no role change.
func TestRoleChangesStillRefuseADelegatedThirdPartyToken(t *testing.T) {
	st, creators := keyFixture(nil)
	s := keyService(st, rowChecker{st: st}, creators)
	s.Base.PlatformClients = coretest.PlatformClients{}
	thirdParty := core.WithIdentity(context.Background(), core.Identity{
		Subject: "admin-1", Method: core.MethodOAuth2, ClientID: "dcr-agent-7", Human: true, CanonicalScopes: core.ScopeWrite,
	})
	if _, err := s.ChangeRole(thirdParty, "tea-1", "dev-1", "ADMIN"); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("third-party ChangeRole = %v, want ErrForbidden", err)
	}
}
