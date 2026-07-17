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
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// --- fakes -------------------------------------------------------------------

// fakeStore is an in-memory MembersStore mirroring PGStore's error taxonomy.
type fakeStore struct {
	tenant   store.Tenant
	members  map[string]store.TenantMember // subject -> row
	invites  map[string]store.Invite       // id -> row
	nextInv  int
	ownerIDs map[string]string // subject -> own- id, mint-on-first-sight
	ownIDErr error             // when set, OwnerIDForSubject fails every call
}

func newFakeStore(plan string) *fakeStore {
	return &fakeStore{
		tenant:  store.Tenant{ID: "tea-1", Name: "acme", Plan: plan},
		members: map[string]store.TenantMember{},
		invites: map[string]store.Invite{},
	}
}

// OwnerIDForSubject mints a stable, well-formed own- id per subject (matches
// the PGStore's get-or-create semantics for tests).
func (f *fakeStore) OwnerIDForSubject(_ context.Context, subject string) (string, error) {
	if f.ownIDErr != nil {
		return "", f.ownIDErr
	}
	if f.ownerIDs == nil {
		f.ownerIDs = map[string]string{}
	}
	if id, ok := f.ownerIDs[subject]; ok {
		return id, nil
	}
	id := fmt.Sprintf("own-%020d", len(f.ownerIDs)+1)
	f.ownerIDs[subject] = id
	return id, nil
}

// fakeIdentities is the one IdentityLookup fake — subject -> attrs (email +
// MFA state), miss on absence.
type fakeIdentities map[string]IdentityAttrs

func (f fakeIdentities) LookupIdentity(_ context.Context, subject string) (IdentityAttrs, bool) {
	attrs, ok := f[subject]
	return attrs, ok
}

func (f *fakeStore) seedMember(subject, role string) {
	f.members[subject] = store.TenantMember{TenantID: "tea-1", Subject: subject, Role: role, CreatedAt: time.Unix(1, 0)}
}

func (f *fakeStore) GetTenant(_ context.Context, id string) (store.Tenant, error) {
	if id != f.tenant.ID {
		return store.Tenant{}, fmt.Errorf("workspace: %w", store.ErrNotFound)
	}
	return f.tenant, nil
}

func (f *fakeStore) ListTenantMembers(_ context.Context, _ string) ([]store.TenantMember, error) {
	out := make([]store.TenantMember, 0, len(f.members))
	for _, m := range f.members {
		out = append(out, m)
	}
	return out, nil
}

func (f *fakeStore) GetTenantMember(_ context.Context, _, subject string) (store.TenantMember, error) {
	m, ok := f.members[subject]
	if !ok {
		return store.TenantMember{}, fmt.Errorf("tenant_member: %w", store.ErrNotFound)
	}
	return m, nil
}

func (f *fakeStore) CountTenantMembers(_ context.Context, _ string) (int, error) {
	return len(f.members), nil
}

func (f *fakeStore) CountInvites(_ context.Context, _ string) (int, error) {
	return len(f.invites), nil
}

func (f *fakeStore) CountTenantAdmins(_ context.Context, _ string) (int, error) {
	n := 0
	for _, m := range f.members {
		if m.Role == "admin" {
			n++
		}
	}
	return n, nil
}

func (f *fakeStore) UpdateMemberRole(_ context.Context, _, subject, role string) error {
	m, ok := f.members[subject]
	if !ok {
		return fmt.Errorf("tenant_member: %w", store.ErrNotFound)
	}
	m.Role = role
	f.members[subject] = m
	return nil
}

func (f *fakeStore) RemoveMember(_ context.Context, _, subject string) error {
	if _, ok := f.members[subject]; !ok {
		return fmt.Errorf("tenant_member: %w", store.ErrNotFound)
	}
	delete(f.members, subject)
	return nil
}

func (f *fakeStore) CreateInvite(_ context.Context, tenantID, email, role, token, invitedBy string, expiresAt time.Time) (store.Invite, error) {
	for _, inv := range f.invites {
		if inv.Email == email {
			return store.Invite{}, fmt.Errorf("invite: %w", store.ErrConflict)
		}
	}
	f.nextInv++
	inv := store.Invite{
		ID: fmt.Sprintf("inv-%d", f.nextInv), TenantID: tenantID, Email: email, Role: role,
		Token: token, InvitedBy: invitedBy, CreatedAt: time.Unix(1, 0), ExpiresAt: expiresAt,
	}
	f.invites[inv.ID] = inv
	return inv, nil
}

func (f *fakeStore) ListInvites(_ context.Context, _ string) ([]store.Invite, error) {
	out := make([]store.Invite, 0, len(f.invites))
	for _, inv := range f.invites {
		out = append(out, inv)
	}
	return out, nil
}

func (f *fakeStore) GetInvite(_ context.Context, _, id string) (store.Invite, error) {
	inv, ok := f.invites[id]
	if !ok {
		return store.Invite{}, fmt.Errorf("invite: %w", store.ErrNotFound)
	}
	return inv, nil
}

func (f *fakeStore) DeleteInvite(_ context.Context, _, id string) error {
	if _, ok := f.invites[id]; !ok {
		return fmt.Errorf("invite: %w", store.ErrNotFound)
	}
	delete(f.invites, id)
	return nil
}

func (f *fakeStore) RefreshInvite(_ context.Context, _, id string, expiresAt time.Time) (store.Invite, error) {
	inv, ok := f.invites[id]
	if !ok || inv.AcceptedAt != nil {
		return store.Invite{}, fmt.Errorf("invite: %w", store.ErrNotFound)
	}
	inv.ExpiresAt = expiresAt
	f.invites[id] = inv
	return inv, nil
}

func (f *fakeStore) AcceptInviteByToken(_ context.Context, token, subject string) (store.Invite, error) {
	for id, inv := range f.invites {
		if inv.Token != token {
			continue
		}
		switch {
		case inv.AcceptedAt != nil:
			return store.Invite{}, fmt.Errorf("invite already accepted: %w", store.ErrConflict)
		case time.Now().After(inv.ExpiresAt):
			return store.Invite{}, fmt.Errorf("invite expired: %w", store.ErrConflict)
		}
		if !store.RoleAllowedOnPlan(f.tenant.Plan, inv.Role) {
			return store.Invite{}, fmt.Errorf("plan cannot seat this invite: %w", store.ErrConflict)
		}
		if lim := store.LimitsFor(f.tenant.Plan).MaxMembers; lim > 0 {
			if _, already := f.members[subject]; !already && len(f.members) >= lim {
				return store.Invite{}, fmt.Errorf("plan cannot seat this invite: %w", store.ErrConflict)
			}
		}
		f.members[subject] = store.TenantMember{TenantID: inv.TenantID, Subject: subject, Role: inv.Role, CreatedAt: time.Unix(2, 0)}
		now := time.Now()
		inv.AcceptedAt = &now
		f.invites[id] = inv
		return inv, nil
	}
	return store.Invite{}, fmt.Errorf("invite: %w", store.ErrNotFound)
}

// fakeGranter records grants + revokes and answers Check over its tuple set.
type fakeGranter struct {
	granted []string
	revoked []string
	tuples  map[string]bool
}

func newFakeGranter() *fakeGranter { return &fakeGranter{tuples: map[string]bool{}} }

func key(relation, tenantID, subject string) string { return relation + ":" + tenantID + ":" + subject }

func (g *fakeGranter) GrantWorkspaceRole(_ context.Context, tenantID, subject, relation string) error {
	g.granted = append(g.granted, key(relation, tenantID, subject))
	g.tuples[key(relation, tenantID, subject)] = true
	return nil
}

func (g *fakeGranter) RevokeWorkspaceMember(_ context.Context, tenantID, subject, relation string) error {
	g.revoked = append(g.revoked, key(relation, tenantID, subject))
	delete(g.tuples, key(relation, tenantID, subject))
	return nil
}

func (g *fakeGranter) Check(_ context.Context, subject, relation, object string) (bool, error) {
	tenantID := strings.TrimPrefix(object, "workspace:")
	return g.tuples[key(relation, tenantID, subject)], nil
}

// fakeMailer records the last message.
type fakeMailer struct {
	to, subject, body string
	calls             int
	err               error
}

func (m *fakeMailer) Send(_ context.Context, to, subject, body string) error {
	m.calls++
	m.to, m.subject, m.body = to, subject, body
	return m.err
}

// denyChecker refuses every check — the non-admin caller.
type denyChecker struct{}

func (denyChecker) Check(context.Context, string, string, string) (bool, error) { return false, nil }

// roleChecker allows a caller to hold exactly one relation on the workspace.
type roleChecker struct{ relation string }

func (c roleChecker) Check(_ context.Context, _, relation, _ string) (bool, error) {
	// Model Render's derived permissions: a can_view holder passes can_view; a
	// can_manage holder passes both. Keep it minimal — only the relations the
	// tests probe.
	switch c.relation {
	case "admin":
		return true, nil
	case "viewer":
		return relation == core.RelCanView, nil
	}
	return false, nil
}

func ctxWith(subject string) context.Context {
	return core.WithIdentity(context.Background(), core.Identity{Subject: subject, Method: "session"})
}

func svc(st MembersStore, g *fakeGranter, mailer Mailer, authz core.Checker) *Service {
	return &Service{
		Base:    &core.Base{Authz: authz},
		Store:   st,
		Granter: g,
		Revoker: g,
		Mailer:  mailer,
	}
}

// --- tests -------------------------------------------------------------------

func TestInviteRecordsRowAndMails(t *testing.T) {
	st := newFakeStore(store.PlanPro)
	st.seedMember("admin-1", "admin")
	m := &fakeMailer{}
	s := svc(st, newFakeGranter(), m, nil) // authz nil => allow
	inv, err := s.Invite(ctxWith("admin-1"), "tea-1", "New.Person@Example.com", "developer")
	if err != nil {
		t.Fatalf("invite: %v", err)
	}
	if inv.Email != "new.person@example.com" {
		t.Errorf("email not normalized: %q", inv.Email)
	}
	if inv.Role != "DEVELOPER" {
		t.Errorf("role not UPPERCASE on the wire: %q", inv.Role)
	}
	if m.calls != 1 || m.to != "new.person@example.com" {
		t.Errorf("mail: calls=%d to=%q", m.calls, m.to)
	}
	if len(st.invites) != 1 {
		t.Errorf("invite row not recorded: %d", len(st.invites))
	}
}

func TestInviteRejectsBadRoleAndEmail(t *testing.T) {
	st := newFakeStore(store.PlanPro)
	s := svc(st, newFakeGranter(), nil, nil)
	if _, err := s.Invite(ctxWith("a"), "tea-1", "x@y.com", "superuser"); !errors.Is(err, core.ErrBadRequest) {
		t.Errorf("bad role: want ErrBadRequest, got %v", err)
	}
	if _, err := s.Invite(ctxWith("a"), "tea-1", "not-an-email", "developer"); !errors.Is(err, core.ErrBadRequest) {
		t.Errorf("bad email: want ErrBadRequest, got %v", err)
	}
}

func TestInviteEnforcesPlanRoleGate(t *testing.T) {
	// Pro only offers admin/developer (RESEARCH-workspaces.md finding 5) —
	// inviting a VIEWER is refused, DEVELOPER succeeds.
	st := newFakeStore(store.PlanPro)
	st.seedMember("admin-1", "admin")
	s := svc(st, newFakeGranter(), nil, nil)
	if _, err := s.Invite(ctxWith("admin-1"), "tea-1", "v@example.com", "viewer"); !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("viewer on pro: want ErrBadRequest, got %v", err)
	}
	if _, err := s.Invite(ctxWith("admin-1"), "tea-1", "d@example.com", "developer"); err != nil {
		t.Fatalf("developer on pro: %v", err)
	}
	// Scale offers the full role set.
	stScale := newFakeStore(store.PlanScale)
	stScale.seedMember("admin-1", "admin")
	sScale := svc(stScale, newFakeGranter(), nil, nil)
	if _, err := sScale.Invite(ctxWith("admin-1"), "tea-1", "v@example.com", "viewer"); err != nil {
		t.Fatalf("viewer on scale: %v", err)
	}
}

func TestChangeRoleEnforcesPlanRoleGate(t *testing.T) {
	st := newFakeStore(store.PlanPro)
	st.seedMember("admin-1", "admin")
	st.seedMember("bob", "developer")
	s := svc(st, newFakeGranter(), nil, nil)
	if _, err := s.ChangeRole(ctxWith("admin-1"), "tea-1", "bob", "viewer"); !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("viewer on pro: want ErrBadRequest, got %v", err)
	}
}

func TestInviteEnforcesMemberCap(t *testing.T) {
	st := newFakeStore(store.PlanHobby) // single-member
	st.seedMember("admin-1", "admin")
	s := svc(st, newFakeGranter(), nil, nil)
	_, err := s.Invite(ctxWith("admin-1"), "tea-1", "second@example.com", "developer")
	if !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("hobby cap: want ErrBadRequest, got %v", err)
	}
}

// TestInvitePlanLimitCode replaces TestInvitePlanRefusalsNameThePlan (w2/m28):
// the dashboard now keys on the PLAN_LIMIT error code rather than a substring
// of the English message, so this test asserts code+params — a copy change to
// the error text has zero effect on whether the CTA shows.
func TestInvitePlanLimitCode(t *testing.T) {
	var ce *core.CodedError

	// seat cap (Hobby is single-member; use role "admin" — the only role Hobby
	// allows — so the role gate passes and the seat cap is what fires)
	capped := newFakeStore(store.PlanHobby)
	capped.seedMember("admin-1", "admin")
	_, err := svc(capped, newFakeGranter(), nil, nil).
		Invite(ctxWith("admin-1"), "tea-1", "second@example.com", "admin")
	if !errors.As(err, &ce) {
		t.Fatalf("seat cap: want *core.CodedError, got %T: %v", err, err)
	}
	if ce.Code != "PLAN_LIMIT" {
		t.Errorf("seat cap code = %q, want PLAN_LIMIT", ce.Code)
	}
	if ce.Params["plan"] != store.PlanHobby {
		t.Errorf("seat cap plan = %v, want %q", ce.Params["plan"], store.PlanHobby)
	}
	if ce.Params["limit"] != 1 {
		t.Errorf("seat cap limit = %v, want 1", ce.Params["limit"])
	}
	if !errors.Is(err, core.ErrBadRequest) {
		t.Error("seat cap: want errors.Is(err, ErrBadRequest)")
	}

	// role gate (Pro doesn't offer viewer)
	pro := newFakeStore(store.PlanPro)
	pro.seedMember("admin-1", "admin")
	_, err = svc(pro, newFakeGranter(), nil, nil).
		Invite(ctxWith("admin-1"), "tea-1", "v@example.com", "viewer")
	if !errors.As(err, &ce) {
		t.Fatalf("role gate: want *core.CodedError, got %T: %v", err, err)
	}
	if ce.Code != "PLAN_LIMIT" {
		t.Errorf("role gate code = %q, want PLAN_LIMIT", ce.Code)
	}
	if ce.Params["plan"] != store.PlanPro {
		t.Errorf("role gate plan = %v, want %q", ce.Params["plan"], store.PlanPro)
	}
	if !errors.Is(err, core.ErrBadRequest) {
		t.Error("role gate: want errors.Is(err, ErrBadRequest)")
	}
}

func TestInviteRequiresAdmin(t *testing.T) {
	st := newFakeStore(store.PlanPro)
	s := svc(st, newFakeGranter(), nil, denyChecker{})
	if _, err := s.Invite(ctxWith("viewer-1"), "tea-1", "x@y.com", "developer"); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("non-admin invite: want ErrForbidden, got %v", err)
	}
}

func TestListAllowsViewer(t *testing.T) {
	st := newFakeStore(store.PlanPro)
	st.seedMember("admin-1", "admin")
	s := svc(st, newFakeGranter(), nil, roleChecker{relation: "viewer"})
	ms, err := s.List(ctxWith("viewer-1"), "tea-1")
	if err != nil {
		t.Fatalf("viewer list: %v", err)
	}
	if len(ms) != 1 || ms[0].Role != "ADMIN" {
		t.Errorf("members = %+v", ms)
	}
}

func TestListEnrichesUserIDAndEmail(t *testing.T) {
	st := newFakeStore(store.PlanPro)
	st.seedMember("admin-1", "admin")
	s := svc(st, newFakeGranter(), nil, roleChecker{relation: "viewer"})
	s.Identities = fakeIdentities{"admin-1": {Email: "admin@example.com"}}
	ms, err := s.List(ctxWith("viewer-1"), "tea-1")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(ms) != 1 {
		t.Fatalf("members = %+v", ms)
	}
	if ms[0].UserID == "" || !strings.HasPrefix(ms[0].UserID, "own-") {
		t.Errorf("userId not minted: %q", ms[0].UserID)
	}
	if ms[0].Email != "admin@example.com" {
		t.Errorf("email = %q, want admin@example.com", ms[0].Email)
	}
	// The own- id is stable across calls for the same subject.
	ms2, err := s.List(ctxWith("viewer-1"), "tea-1")
	if err != nil {
		t.Fatalf("list #2: %v", err)
	}
	if ms2[0].UserID != ms[0].UserID {
		t.Errorf("userId not stable: %q != %q", ms2[0].UserID, ms[0].UserID)
	}
}

func TestListNilIdentitiesOmitsEmail(t *testing.T) {
	st := newFakeStore(store.PlanPro)
	st.seedMember("admin-1", "admin")
	s := svc(st, newFakeGranter(), nil, roleChecker{relation: "viewer"})
	ms, err := s.List(ctxWith("viewer-1"), "tea-1")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(ms) != 1 || ms[0].Email != "" {
		t.Errorf("members = %+v, want empty email (no identity reader)", ms)
	}
	if ms[0].UserID == "" {
		t.Errorf("userId should still be minted with no identity reader: %+v", ms[0])
	}
}

func TestListEmailLookupMissDegradesOnlyThatMember(t *testing.T) {
	st := newFakeStore(store.PlanPro)
	st.seedMember("admin-1", "admin")
	st.seedMember("admin-2", "admin")
	s := svc(st, newFakeGranter(), nil, roleChecker{relation: "viewer"})
	s.Identities = fakeIdentities{"admin-1": {Email: "admin@example.com"}} // admin-2 is a miss
	ms, err := s.List(ctxWith("viewer-1"), "tea-1")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	byEmail := map[string]string{}
	for _, m := range ms {
		byEmail[m.Subject] = m.Email
	}
	if byEmail["admin-1"] != "admin@example.com" {
		t.Errorf("admin-1 email = %q", byEmail["admin-1"])
	}
	if byEmail["admin-2"] != "" {
		t.Errorf("admin-2 email = %q, want empty (lookup miss)", byEmail["admin-2"])
	}
}

func TestListOwnerIDStoreErrorSurfaces(t *testing.T) {
	st := newFakeStore(store.PlanPro)
	st.seedMember("admin-1", "admin")
	st.ownIDErr = errors.New("db unavailable")
	s := svc(st, newFakeGranter(), nil, roleChecker{relation: "viewer"})
	if _, err := s.List(ctxWith("viewer-1"), "tea-1"); err == nil {
		t.Fatal("list: want error from OwnerIDForSubject, got nil")
	}
}

// TestListOwnerIDStoreErrorIsHTTP5xx confirms the store error reaches the
// caller as a 5xx over REST, not a silently empty/200 member list — the
// "own- id lookup fails loudly" half of t008's target cases.
func TestListOwnerIDStoreErrorIsHTTP5xx(t *testing.T) {
	st := newFakeStore(store.PlanPro)
	st.seedMember("admin-1", "admin")
	st.ownIDErr = errors.New("db unavailable")
	s := svc(st, newFakeGranter(), nil, roleChecker{relation: "viewer"})

	mux := http.NewServeMux()
	s.RegisterREST(mux)
	req := httptest.NewRequest(http.MethodGet, "/v1/workspaces/tea-1/members", nil).WithContext(ctxWith("viewer-1"))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code < 500 || rec.Code >= 600 {
		t.Fatalf("REST status = %d, want 5xx; body %s", rec.Code, rec.Body)
	}
}

func TestListInvitesDeniedToViewer(t *testing.T) {
	st := newFakeStore(store.PlanPro)
	s := svc(st, newFakeGranter(), nil, roleChecker{relation: "viewer"})
	if _, err := s.ListInvites(ctxWith("viewer-1"), "tea-1"); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("viewer listing invites: want ErrForbidden, got %v", err)
	}
}

func TestChangeRoleGrantsNewRevokesOld(t *testing.T) {
	st := newFakeStore(store.PlanPro)
	st.seedMember("admin-1", "admin")
	st.seedMember("bob", "viewer")
	g := newFakeGranter()
	g.tuples[key("viewer", "tea-1", "user:bob")] = true
	s := svc(st, g, nil, nil)
	m, err := s.ChangeRole(ctxWith("admin-1"), "tea-1", "bob", "DEVELOPER")
	if err != nil {
		t.Fatalf("change role: %v", err)
	}
	if m.Role != "DEVELOPER" {
		t.Errorf("view role = %q", m.Role)
	}
	if st.members["bob"].Role != "developer" {
		t.Errorf("row role = %q", st.members["bob"].Role)
	}
	if len(g.granted) != 1 || g.granted[0] != key("developer", "tea-1", "user:bob") {
		t.Errorf("granted = %v", g.granted)
	}
	if len(g.revoked) != 1 || g.revoked[0] != key("viewer", "tea-1", "user:bob") {
		t.Errorf("revoked = %v", g.revoked)
	}
}

func TestChangeRoleSameRoleIsNoop(t *testing.T) {
	st := newFakeStore(store.PlanPro)
	st.seedMember("admin-1", "admin")
	st.seedMember("bob", "developer")
	g := newFakeGranter()
	s := svc(st, g, nil, nil)
	if _, err := s.ChangeRole(ctxWith("admin-1"), "tea-1", "bob", "developer"); err != nil {
		t.Fatalf("noop change: %v", err)
	}
	if len(g.granted) != 0 || len(g.revoked) != 0 {
		t.Errorf("same-role change wrote tuples: granted=%v revoked=%v", g.granted, g.revoked)
	}
}

func TestChangeRoleRefusesDemotingLastAdmin(t *testing.T) {
	st := newFakeStore(store.PlanPro)
	st.seedMember("admin-1", "admin") // the only admin
	s := svc(st, newFakeGranter(), nil, nil)
	if _, err := s.ChangeRole(ctxWith("admin-1"), "tea-1", "admin-1", "developer"); !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("demote last admin: want ErrBadRequest, got %v", err)
	}
	if st.members["admin-1"].Role != "admin" {
		t.Errorf("last admin demoted anyway: %q", st.members["admin-1"].Role)
	}
}

func TestChangeRoleAllowsDemotingNonLastAdmin(t *testing.T) {
	st := newFakeStore(store.PlanPro)
	st.seedMember("admin-1", "admin")
	st.seedMember("admin-2", "admin")
	s := svc(st, newFakeGranter(), nil, nil)
	if _, err := s.ChangeRole(ctxWith("admin-1"), "tea-1", "admin-2", "developer"); err != nil {
		t.Fatalf("demote one of two admins: %v", err)
	}
}

func TestRemoveDropsRowAndRevokes(t *testing.T) {
	st := newFakeStore(store.PlanPro)
	st.seedMember("admin-1", "admin")
	st.seedMember("bob", "developer")
	g := newFakeGranter()
	s := svc(st, g, nil, nil)
	if err := s.Remove(ctxWith("admin-1"), "tea-1", "bob"); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if _, ok := st.members["bob"]; ok {
		t.Error("member row not deleted")
	}
	if len(g.revoked) != 1 || g.revoked[0] != key("developer", "tea-1", "user:bob") {
		t.Errorf("revoked = %v", g.revoked)
	}
}

func TestRemoveRefusesLastAdmin(t *testing.T) {
	st := newFakeStore(store.PlanPro)
	st.seedMember("admin-1", "admin")
	s := svc(st, newFakeGranter(), nil, nil)
	if err := s.Remove(ctxWith("admin-1"), "tea-1", "admin-1"); !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("remove last admin: want ErrBadRequest, got %v", err)
	}
	if _, ok := st.members["admin-1"]; !ok {
		t.Error("last admin removed anyway")
	}
}

func TestRevokeInviteDeletes(t *testing.T) {
	st := newFakeStore(store.PlanPro)
	st.seedMember("admin-1", "admin")
	s := svc(st, newFakeGranter(), nil, nil)
	inv, err := s.Invite(ctxWith("admin-1"), "tea-1", "p@example.com", "developer")
	if err != nil {
		t.Fatalf("invite: %v", err)
	}
	if err := s.RevokeInvite(ctxWith("admin-1"), "tea-1", inv.ID); err != nil {
		t.Fatalf("revoke invite: %v", err)
	}
	if len(st.invites) != 0 {
		t.Errorf("invite not deleted: %d", len(st.invites))
	}
}

func TestVerbsUnavailableWithoutStore(t *testing.T) {
	s := &Service{Base: &core.Base{}} // Store nil, authz nil (allow)
	if _, err := s.List(ctxWith("a"), "tea-1"); !errors.Is(err, ErrMembersUnavailable) {
		t.Errorf("List: want ErrMembersUnavailable, got %v", err)
	}
	if _, err := s.Invite(ctxWith("a"), "tea-1", "x@y.com", "developer"); !errors.Is(err, ErrMembersUnavailable) {
		t.Errorf("Invite: want ErrMembersUnavailable, got %v", err)
	}
}
