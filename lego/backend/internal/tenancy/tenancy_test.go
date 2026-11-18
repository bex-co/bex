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

package tenancy

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// fakeTenantStore is the in-memory TenantStore for tenancy.go tests — it
// mirrors PGStore's tenant_members design (one subject->tenant map serves both
// human identities and bound API-key client ids) and its race-safe mint
// semantics (the ownerOf gate, not a check-then-insert); the DB-level race
// itself is covered live in store_pg_test.go.
type fakeTenantStore struct {
	mu       sync.Mutex
	tenants  map[string]store.Tenant
	members  map[string]string // subject (identity or client id) -> tenantID
	clients  map[string]bool   // the subjects BindClient bound: API keys
	markers  []string          // BumpOAuthRevocation's subjects, in order
	ownerOf  map[string]string // identityID -> tenantID (mint gate)
	invites  []store.Invite    // outstanding invites (seeded via invite())
	n        int
	deleting bool
	// tombstoneErr fails every tombstone read; ownerFail, when set, is asked
	// before each TenantForOwner and fails that read when it returns an error.
	// createFail, when set, fails CreateTenantWithMember the same way.
	// How the w32/m446 tests model a store fault.
	tombstoneErr error
	ownerFail    func() error
	createFail   func() error
}

func (f *fakeTenantStore) AccountDeletionTombstoned(context.Context, string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.deleting, f.tombstoneErr
}

func newFakeTenantStore() *fakeTenantStore {
	return &fakeTenantStore{
		tenants: map[string]store.Tenant{},
		members: map[string]string{},
		clients: map[string]bool{},
		ownerOf: map[string]string{},
	}
}

func (f *fakeTenantStore) TenantForIdentity(_ context.Context, subject string) (store.Tenant, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	tid, ok := f.members[subject]
	if !ok {
		return store.Tenant{}, store.ErrNotFound
	}
	return f.tenants[tid], nil
}

func (f *fakeTenantStore) IsMember(_ context.Context, subject, tenantID string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.members[subject] == tenantID, nil
}

func (f *fakeTenantStore) TenantForOwner(_ context.Context, identityID string) (store.Tenant, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.ownerFail != nil {
		if err := f.ownerFail(); err != nil {
			return store.Tenant{}, err
		}
	}
	tid, ok := f.ownerOf[identityID]
	if !ok {
		return store.Tenant{}, store.ErrNotFound
	}
	return f.tenants[tid], nil
}

func (f *fakeTenantStore) CreateTenantWithMember(_ context.Context, identityID, plan string) (store.Tenant, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if tid, ok := f.ownerOf[identityID]; ok {
		f.members[identityID] = tid
		return f.tenants[tid], nil
	}
	// Only a genuine mint can fail: the store's unique owner_identity_id gate
	// answers a retry from the binding above, never from the insert below.
	if f.createFail != nil {
		if err := f.createFail(); err != nil {
			return store.Tenant{}, err
		}
	}
	f.n++
	id := fmt.Sprintf("tea-%d", f.n)
	t := store.Tenant{ID: id, Name: id, Plan: plan}
	f.tenants[id] = t
	f.ownerOf[identityID] = id
	f.members[identityID] = id
	return t, nil
}

func (f *fakeTenantStore) SetTenantDisplayNameIfUnset(_ context.Context, id, displayName string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	t, ok := f.tenants[id]
	if !ok {
		return store.ErrNotFound
	}
	if t.DisplayName == "" {
		t.DisplayName = displayName
		f.tenants[id] = t
	}
	return nil
}

func (f *fakeTenantStore) BindClient(_ context.Context, clientID, tenantID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.members[clientID] = tenantID
	f.clients[clientID] = true
	return nil
}

func (f *fakeTenantStore) UnbindClient(_ context.Context, clientID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.members, clientID)
	delete(f.clients, clientID)
	return nil
}

func (f *fakeTenantStore) ClientBindingsOfTenants(_ context.Context, tenantIDs []string) (map[string]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := map[string]string{}
	for clientID := range f.clients {
		if slices.Contains(tenantIDs, f.members[clientID]) {
			out[clientID] = f.members[clientID]
		}
	}
	return out, nil
}

func (f *fakeTenantStore) BumpOAuthRevocation(_ context.Context, subject, _ string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.markers = append(f.markers, subject)
	return nil
}

// invite seeds a pending invite the fake redeems on login (test helper).
func (f *fakeTenantStore) invite(email, tenantID, role string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.invites = append(f.invites, store.Invite{
		ID: fmt.Sprintf("inv-%d", len(f.invites)+1), TenantID: tenantID, Email: email, Role: role,
	})
}

func (f *fakeTenantStore) AcceptInvitesForEmail(_ context.Context, email, subject string) ([]store.Invite, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var accepted, remaining []store.Invite
	for _, inv := range f.invites {
		if inv.Email == email {
			f.members[subject] = inv.TenantID
			accepted = append(accepted, inv)
		} else {
			remaining = append(remaining, inv)
		}
	}
	f.invites = remaining
	return accepted, nil
}

// fakeGranter is a MembershipGranter that records every call and can be told
// to fail the next N grants — how tests exercise the rollback paths. It also
// implements core.Checker over its own tuple set, mirroring the production
// authz.openfgaChecker (which structurally satisfies both interfaces) — that
// parity is what makes ensureGranted's check-before-grant path testable at all.
type fakeGranter struct {
	mu         sync.Mutex
	failNext   int
	failRevoke bool            // when set, RevokeWorkspaceMember fails and leaves the tuple
	tuples     map[string]bool // "relation:tenantID:subject" -> live
	granted    []string        // successful grants, in call order
	revoked    []string
	checkErr   error // when set, every Check fails with it (an OpenFGA fault)
	checks     int
}

func newFakeGranter() *fakeGranter { return &fakeGranter{tuples: map[string]bool{}} }

// fgWithFail returns a fakeGranter whose next n grant calls fail — how tests
// simulate a transient OpenFGA write failure.
func fgWithFail(n int) *fakeGranter {
	g := newFakeGranter()
	g.failNext = n
	return g
}

func (g *fakeGranter) grant(tenantID, subject, relation string) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.failNext > 0 {
		g.failNext--
		return errors.New("fga write failed")
	}
	tuple := relation + ":" + tenantID + ":" + subject
	g.tuples[tuple] = true
	g.granted = append(g.granted, tuple)
	return nil
}

func (g *fakeGranter) GrantWorkspaceAdmin(_ context.Context, tenantID, subject string) error {
	return g.grant(tenantID, subject, "admin")
}
func (g *fakeGranter) GrantWorkspaceMember(_ context.Context, tenantID, subject string) error {
	return g.grant(tenantID, subject, "developer")
}
func (g *fakeGranter) GrantWorkspaceRole(_ context.Context, tenantID, subject, relation string) error {
	return g.grant(tenantID, subject, relation)
}
func (g *fakeGranter) RevokeWorkspaceMember(_ context.Context, tenantID, subject, relation string) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.failRevoke {
		// Model a real OpenFGA failure: the tuple stays live.
		return errors.New("fga revoke failed")
	}
	tuple := relation + ":" + tenantID + ":" + subject
	delete(g.tuples, tuple)
	g.revoked = append(g.revoked, tuple)
	return nil
}

// Check implements core.Checker: relation/object come in OpenFGA's wire shape
// ("admin", "workspace:tea-1"), translated to the same "relation:tenantID:.."
// key grant/revoke use.
func (g *fakeGranter) Check(_ context.Context, subject, relation, object string) (bool, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.checks++
	if g.checkErr != nil {
		return false, g.checkErr
	}
	tenantID := strings.TrimPrefix(object, "workspace:")
	return g.tuples[relation+":"+tenantID+":"+subject], nil
}

// --- Mint idempotency + race-safety (Service's own logic, above the store) ---

func TestEnsureTenantMintsOnceAndCaches(t *testing.T) {
	st := newFakeTenantStore()
	ts := NewTenantService(st, nil)
	ctx := context.Background()

	first, err := ts.EnsureTenant(ctx, "identity-a", "", false)
	if err != nil || first == "" {
		t.Fatalf("first mint: %v %q", err, first)
	}
	if len(st.tenants) != 1 {
		t.Fatalf("tenants after first mint = %d, want 1", len(st.tenants))
	}
	second, err := ts.EnsureTenant(ctx, "identity-a", "", false)
	if err != nil || second != first {
		t.Fatalf("second call: %v %q, want %q", err, second, first)
	}
	if len(st.tenants) != 1 {
		t.Errorf("tenants after second call = %d, want 1 (idempotent)", len(st.tenants))
	}
}

func TestEnsureTenantDeletionTombstoneWinsOverPositiveCache(t *testing.T) {
	st := newFakeTenantStore()
	ts := NewTenantService(st, nil)
	ctx := context.Background()
	if _, err := ts.EnsureTenant(ctx, "identity-a", "", false); err != nil {
		t.Fatalf("initial onboarding: %v", err)
	}
	st.mu.Lock()
	st.deleting = true
	st.mu.Unlock()
	_, err := ts.EnsureTenant(ctx, "identity-a", "", false)
	var coded *core.CodedError
	if !errors.As(err, &coded) || coded.Code != "ACCOUNT_DELETION_PENDING" {
		t.Fatalf("cached onboarding after tombstone = %v, want ACCOUNT_DELETION_PENDING", err)
	}
	if len(st.tenants) != 1 {
		t.Fatalf("tombstone reminted tenant: count=%d", len(st.tenants))
	}
}

func TestEnsureTenantConcurrentFirstLoginsConverge(t *testing.T) {
	st := newFakeTenantStore()
	ts := NewTenantService(st, nil)
	ctx := context.Background()

	const n = 20
	ids := make([]string, n)
	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			tid, err := ts.EnsureTenant(ctx, "identity-racer", "", false)
			if err != nil {
				t.Errorf("concurrent EnsureTenant %d: %v", i, err)
				return
			}
			ids[i] = tid
		}(i)
	}
	wg.Wait()
	for i, id := range ids {
		if id != ids[0] {
			t.Fatalf("goroutine %d minted %s, want %s (same as goroutine 0)", i, id, ids[0])
		}
	}
	if len(st.tenants) != 1 {
		t.Errorf("tenants after concurrent mint = %d, want 1", len(st.tenants))
	}
}

func TestEnsureTenantGrantsAdminOnFreshMintOnly(t *testing.T) {
	st := newFakeTenantStore()
	granter := newFakeGranter()
	ts := NewTenantService(st, granter)
	ctx := context.Background()

	tid, err := ts.EnsureTenant(ctx, "identity-a", "", false)
	if err != nil {
		t.Fatalf("mint: %v", err)
	}
	want := []string{"admin:" + tid + ":user:identity-a"}
	if len(granter.granted) != 1 || granter.granted[0] != want[0] {
		t.Fatalf("granted = %v, want %v", granter.granted, want)
	}
	if _, err := ts.EnsureTenant(ctx, "identity-a", "", false); err != nil {
		t.Fatalf("second call: %v", err)
	}
	if len(granter.granted) != 1 {
		t.Errorf("granted after second call = %v, want unchanged (no re-grant on a returning caller)", granter.granted)
	}
}

// endBindingOnGrant is another admin removing the creator, committing just
// after onboarding's admin grant lands.
type endBindingOnGrant struct {
	*fakeGranter
	st       *fakeTenantStore
	identity string
	tenantID string
}

func (g endBindingOnGrant) GrantWorkspaceAdmin(ctx context.Context, tenantID, subject string) error {
	if err := g.fakeGranter.GrantWorkspaceAdmin(ctx, tenantID, subject); err != nil {
		return err
	}
	if tenantID == g.tenantID {
		g.st.mu.Lock()
		delete(g.st.ownerOf, g.identity)
		delete(g.st.members, g.identity)
		g.st.mu.Unlock()
	}
	return nil
}

// w5/m372: a creator removed while their own request re-grants admin must not
// keep the grant; the request lands them in a new personal workspace.
func TestEnsureTenantTakesBackAGrantThatOutlivedTheBinding(t *testing.T) {
	st := newFakeTenantStore()
	granter := newFakeGranter()
	ctx := context.Background()
	old, err := NewTenantService(st, granter).EnsureTenant(ctx, "creator", "", false)
	if err != nil {
		t.Fatalf("mint: %v", err)
	}
	oldAdmin := "admin:" + old + ":user:creator"
	delete(granter.tuples, oldAdmin) // the remover's revoke, before its commit

	got, err := NewTenantService(st, endBindingOnGrant{granter, st, "creator", old}).EnsureTenant(ctx, "creator", "", false)
	if err != nil {
		t.Fatalf("EnsureTenant across the removal: %v", err)
	}
	if got == old {
		t.Fatalf("resolved the removed creator back to %q", old)
	}
	if granter.tuples[oldAdmin] {
		t.Error("the admin grant outlived the binding on the old workspace")
	}
	if !granter.tuples["admin:"+got+":user:creator"] {
		t.Errorf("no admin tuple on the new personal workspace %q", got)
	}
}

func TestEnsureTenantGrantFailureFailsClosedNotHalfMinted(t *testing.T) {
	st := newFakeTenantStore()
	granter := fgWithFail(1)
	ts := NewTenantService(st, granter)
	ctx := context.Background()

	if _, err := ts.EnsureTenant(ctx, "identity-a", "", false); err == nil {
		t.Fatal("grant failure must surface as an error, not silently succeed")
	}
	// The mint itself is idempotent (ON CONFLICT), so retrying must succeed and
	// grant exactly once — no leftover half-onboarded state blocks the retry.
	tid, err := ts.EnsureTenant(ctx, "identity-a", "", false)
	if err != nil || tid == "" {
		t.Fatalf("retry after grant failure: %v %q", err, tid)
	}
	if len(granter.granted) != 1 {
		t.Errorf("granted after retry = %v, want exactly one admin grant", granter.granted)
	}
}

// TestEnsureTenantRedeemsInvitesAndGrantsCorrectWorkspace verifies the w4/m12
// invite-acceptance path: a signup with a pending invite joins the invited
// workspace at the invited role, AND still gets admin on its OWN personal
// tenant — never admin on the invited workspace (the owner-keyed personal
// resolution keeps the admin grant off a workspace the caller was merely invited
// to as a viewer).
func TestEnsureTenantRedeemsInvitesAndGrantsCorrectWorkspace(t *testing.T) {
	st := newFakeTenantStore()
	granter := newFakeGranter()
	ts := NewTenantService(st, granter)
	ctx := context.Background()
	st.invite("bob@example.com", "tea-invited", "viewer")

	personal, err := ts.EnsureTenant(ctx, "identity-bob", "bob@example.com", true)
	if err != nil || personal == "" {
		t.Fatalf("ensure: %v %q", err, personal)
	}
	if personal == "tea-invited" {
		t.Fatal("personal tenant must not be the invited workspace")
	}
	// The invited role tuple lands on the invited workspace...
	if !granter.tuples["viewer:tea-invited:user:identity-bob"] {
		t.Errorf("invited viewer tuple missing; granted=%v", granter.granted)
	}
	// ...admin lands only on the personal tenant, never on the invited workspace.
	if !granter.tuples["admin:"+personal+":user:identity-bob"] {
		t.Errorf("personal admin tuple missing; granted=%v", granter.granted)
	}
	if granter.tuples["admin:tea-invited:user:identity-bob"] {
		t.Error("invited viewer was wrongly granted admin on the invited workspace")
	}
}

// TestEnsureTenantVerifiedEmailGate is the w1/m53 fix: when
// RequireVerifiedInviteEmail is set, a login whose email is NOT Kratos-verified
// must not redeem a pending invite (so an attacker who registers with a victim's
// not-yet-signed-up invited address can't claim it); a verified email still does.
func TestEnsureTenantVerifiedEmailGate(t *testing.T) {
	t.Run("unverified email is refused when the gate is on", func(t *testing.T) {
		st := newFakeTenantStore()
		granter := newFakeGranter()
		ts := NewTenantService(st, granter)
		ts.RequireVerifiedInviteEmail = true
		st.invite("bob@example.com", "tea-invited", "viewer")

		if _, err := ts.EnsureTenant(context.Background(), "identity-attacker", "bob@example.com", false); err != nil {
			t.Fatalf("ensure: %v", err)
		}
		if granter.tuples["viewer:tea-invited:user:identity-attacker"] {
			t.Fatal("unverified email redeemed the invite despite the gate being on")
		}
	})
	t.Run("verified email still redeems when the gate is on", func(t *testing.T) {
		st := newFakeTenantStore()
		granter := newFakeGranter()
		ts := NewTenantService(st, granter)
		ts.RequireVerifiedInviteEmail = true
		st.invite("bob@example.com", "tea-invited", "viewer")

		if _, err := ts.EnsureTenant(context.Background(), "identity-bob", "bob@example.com", true); err != nil {
			t.Fatalf("ensure: %v", err)
		}
		if !granter.tuples["viewer:tea-invited:user:identity-bob"] {
			t.Fatalf("verified email failed to redeem; granted=%v", granter.granted)
		}
	})
	t.Run("default off preserves redemption for any email", func(t *testing.T) {
		st := newFakeTenantStore()
		granter := newFakeGranter()
		ts := NewTenantService(st, granter) // RequireVerifiedInviteEmail defaults false
		st.invite("bob@example.com", "tea-invited", "viewer")

		if _, err := ts.EnsureTenant(context.Background(), "identity-bob", "bob@example.com", false); err != nil {
			t.Fatalf("ensure: %v", err)
		}
		if !granter.tuples["viewer:tea-invited:user:identity-bob"] {
			t.Fatal("default-off gate wrongly blocked redemption of an unverified email")
		}
	})
}

// --- Key binding: bind, unbind, and the FGA-failure rollback (t002/t006) ---

func TestBindKeyGrantsMembership(t *testing.T) {
	st := newFakeTenantStore()
	granter := newFakeGranter()
	ts := NewTenantService(st, granter)
	ctx := context.Background()
	ten, err := st.CreateTenantWithMember(ctx, "identity-owner", store.PlanHobby)
	if err != nil {
		t.Fatal(err)
	}

	if err := ts.BindKey(ctx, "client-1", ten.ID, ""); err != nil {
		t.Fatalf("BindKey: %v", err)
	}
	if tid, err := st.TenantForIdentity(ctx, "client-1"); err != nil || tid.ID != ten.ID {
		t.Fatalf("binding row: %v %+v", err, tid)
	}
	want := "developer:" + ten.ID + ":user:client-1"
	if len(granter.granted) != 1 || granter.granted[0] != want {
		t.Fatalf("granted = %v, want [%s]", granter.granted, want)
	}
}

func TestBindKeyFGAFailureRollsBackBinding(t *testing.T) {
	st := newFakeTenantStore()
	granter := fgWithFail(1)
	ts := NewTenantService(st, granter)
	ctx := context.Background()
	ten, err := st.CreateTenantWithMember(ctx, "identity-owner", store.PlanHobby)
	if err != nil {
		t.Fatal(err)
	}

	if err := ts.BindKey(ctx, "client-1", ten.ID, ""); err == nil {
		t.Fatal("FGA grant failure must surface as an error")
	}
	// No orphaned binding: the mapping row must not survive a failed grant, or
	// the key would authorize against a tenant OpenFGA never heard about.
	if _, err := st.TenantForIdentity(ctx, "client-1"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("binding row after rollback: want ErrNotFound, got %v", err)
	}
}

// creatorBindingStore is the fake store with the Postgres store's
// BindClientForCreator: the key is seated at role, or refused with err.
type creatorBindingStore struct {
	*fakeTenantStore
	role string
	err  error
}

func (s creatorBindingStore) BindClientForCreator(ctx context.Context, clientID, tenantID, _ string) (string, error) {
	if s.err != nil {
		return "", s.err
	}
	return s.role, s.BindClient(ctx, clientID, tenantID)
}

// w9/m481: a key is seated at its creator's role, in the row and the tuple;
// a creator whose role is unsettled, or who no longer mints keys here, binds
// nothing and is told why with a 403.
func TestBindKeySeatsTheKeyAtItsCreatorsRole(t *testing.T) {
	ctx := context.Background()
	st := newFakeTenantStore()
	ten, err := st.CreateTenantWithMember(ctx, "identity-owner", store.PlanHobby)
	if err != nil {
		t.Fatal(err)
	}
	granter := newFakeGranter()
	ts := NewTenantService(creatorBindingStore{fakeTenantStore: st, role: "admin"}, granter)
	if err := ts.BindKey(ctx, "client-admin", ten.ID, "identity-owner"); err != nil {
		t.Fatalf("BindKey: %v", err)
	}
	if want := "admin:" + ten.ID + ":user:client-admin"; len(granter.granted) != 1 || granter.granted[0] != want {
		t.Fatalf("granted = %v, want [%s]", granter.granted, want)
	}

	for _, refusal := range []error{store.ErrRoleUnsettled, fmt.Errorf("key creator: %w", store.ErrNotFound)} {
		granter := newFakeGranter()
		ts := NewTenantService(creatorBindingStore{fakeTenantStore: st, err: refusal}, granter)
		err := ts.BindKey(ctx, "client-refused", ten.ID, "identity-owner")
		if !errors.Is(err, core.ErrForbidden) {
			t.Errorf("BindKey refused by %v = %v, want ErrForbidden", refusal, err)
		}
		if len(granter.granted) != 0 {
			t.Errorf("a refused bind granted %v", granter.granted)
		}
	}

	// A failed grant still rolls the creator-seated row back.
	ts = NewTenantService(creatorBindingStore{fakeTenantStore: st, role: "developer"}, fgWithFail(1))
	if err := ts.BindKey(ctx, "client-rollback", ten.ID, "identity-owner"); err == nil {
		t.Fatal("a failed grant must surface")
	}
	if _, err := st.TenantForIdentity(ctx, "client-rollback"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("binding row after a failed grant: %v, want ErrNotFound", err)
	}
}

func TestUnbindKeyRemovesBindingAndRevokesMembership(t *testing.T) {
	st := newFakeTenantStore()
	granter := newFakeGranter()
	ts := NewTenantService(st, granter)
	ctx := context.Background()
	ten, err := st.CreateTenantWithMember(ctx, "identity-owner", store.PlanHobby)
	if err != nil {
		t.Fatal(err)
	}

	if err := ts.BindKey(ctx, "client-1", ten.ID, ""); err != nil {
		t.Fatalf("bind: %v", err)
	}
	// A key follows its creator's role (w9/m481), and a role change still
	// converging can leave it a tuple its row no longer names: revoke must
	// remove whatever role the key holds, not only developer.
	if err := granter.GrantWorkspaceRole(ctx, ten.ID, "user:client-1", "admin"); err != nil {
		t.Fatal(err)
	}
	if err := ts.UnbindKey(ctx, "client-1"); err != nil {
		t.Fatalf("UnbindKey: %v", err)
	}
	if _, err := st.TenantForIdentity(ctx, "client-1"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("binding row after unbind: want ErrNotFound, got %v", err)
	}
	for tuple, live := range granter.tuples {
		if live && strings.HasSuffix(tuple, ":user:client-1") {
			t.Errorf("tuple %s survived the unbind", tuple)
		}
	}
}

func TestUnbindKeyNeverBoundIsNoop(t *testing.T) {
	st := newFakeTenantStore()
	granter := newFakeGranter()
	ts := NewTenantService(st, granter)

	if err := ts.UnbindKey(context.Background(), "client-never-bound"); err != nil {
		t.Fatalf("unbind of a never-bound key must be a no-op, got %v", err)
	}
	if len(granter.revoked) != 0 {
		t.Errorf("revoked = %v, want none (nothing to revoke)", granter.revoked)
	}
}

// TestUnbindKeyFailClosedLeavesBindingWhenRevokeFails pins codex finding-4: a
// failed OpenFGA revoke must not have already deleted the local binding, or the
// surviving developer tuple would be stranded with no owner row for a retry to
// resolve. Both the row and the tuple stay put and the error surfaces (so
// RevokeAPIKey withholds the Hydra delete); a retry then converges.
func TestUnbindKeyFailClosedLeavesBindingWhenRevokeFails(t *testing.T) {
	st := newFakeTenantStore()
	granter := newFakeGranter()
	ts := NewTenantService(st, granter)
	ctx := context.Background()
	ten, err := st.CreateTenantWithMember(ctx, "identity-owner", store.PlanHobby)
	if err != nil {
		t.Fatal(err)
	}
	if err := ts.BindKey(ctx, "client-1", ten.ID, ""); err != nil {
		t.Fatalf("bind: %v", err)
	}
	tuple := "developer:" + ten.ID + ":user:client-1"

	granter.failRevoke = true
	if err := ts.UnbindKey(ctx, "client-1"); err == nil {
		t.Fatal("UnbindKey must fail closed when the FGA revoke fails")
	}
	// The local binding must survive so a retry can still resolve the tenant...
	if _, err := st.TenantForIdentity(ctx, "client-1"); err != nil {
		t.Errorf("binding row was deleted despite a failed revoke: %v", err)
	}
	// ...and the developer tuple must survive too (the revoke did not take effect).
	if !granter.tuples[tuple] {
		t.Error("developer tuple was removed despite a failed revoke")
	}

	// A retry after OpenFGA recovers converges: tuple revoked, row gone.
	granter.failRevoke = false
	if err := ts.UnbindKey(ctx, "client-1"); err != nil {
		t.Fatalf("retry UnbindKey: %v", err)
	}
	if granter.tuples[tuple] {
		t.Error("developer tuple survived the successful retry")
	}
	if _, err := st.TenantForIdentity(ctx, "client-1"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("binding row after successful retry: want ErrNotFound, got %v", err)
	}
}

// A deleted workspace's key teardown (w9/120) lists only the workspace's keys,
// takes every role tuple a key holds there, and refuses its tokens, but keeps
// its binding row, so a delete that fails after it still finds the key. A
// failed revoke writes no marker and surfaces.
func TestRevokeWorkspaceKeyKeepsTheBindingForTheRetry(t *testing.T) {
	st := newFakeTenantStore()
	granter := newFakeGranter()
	ts := NewTenantService(st, granter)
	ctx := context.Background()
	ten, err := st.CreateTenantWithMember(ctx, "identity-owner", store.PlanHobby)
	if err != nil {
		t.Fatal(err)
	}
	if err := ts.BindKey(ctx, "client-1", ten.ID, ""); err != nil {
		t.Fatal(err)
	}
	if err := ts.BindKey(ctx, "client-elsewhere", "tea-other", ""); err != nil {
		t.Fatal(err)
	}
	// A role change still converging can leave a key a second tuple.
	if err := granter.GrantWorkspaceRole(ctx, ten.ID, "user:client-1", "admin"); err != nil {
		t.Fatal(err)
	}

	keys, err := ts.WorkspaceKeys(ctx, ten.ID)
	if err != nil || !slices.Equal(keys, []string{"client-1"}) {
		t.Fatalf("WorkspaceKeys = %v, %v; want [client-1]: the owner is a person, the other key is bound elsewhere", keys, err)
	}

	granter.failRevoke = true
	if err := ts.RevokeWorkspaceKey(ctx, ten.ID, "client-1"); err == nil {
		t.Fatal("RevokeWorkspaceKey reported success over a failed revoke")
	}
	if len(st.markers) != 0 {
		t.Errorf("markers = %v, want none before the tuples are gone", st.markers)
	}

	granter.failRevoke = false
	for range 2 { // idempotent: the retry of a delete that failed later
		if err := ts.RevokeWorkspaceKey(ctx, ten.ID, "client-1"); err != nil {
			t.Fatalf("RevokeWorkspaceKey: %v", err)
		}
	}
	for _, role := range []string{"developer", "admin"} {
		if granter.tuples[role+":"+ten.ID+":user:client-1"] {
			t.Errorf("the key's %s tuple survived", role)
		}
	}
	if !slices.Equal(st.markers, []string{"client-1", "client-1"}) {
		t.Errorf("markers = %v, want client-1 refused on every replica", st.markers)
	}
	if keys, _ := ts.WorkspaceKeys(ctx, ten.ID); !slices.Equal(keys, []string{"client-1"}) {
		t.Errorf("WorkspaceKeys after the revoke = %v; the binding row must stay for the retry", keys)
	}
	if !granter.tuples["developer:tea-other:user:client-elsewhere"] {
		t.Error("the key bound to another workspace lost its tuple")
	}
}

// --- core.WorkspaceResolver.Tenant: the read path Authorize/List/Get use ---

func TestTenantResolvesSessionAndMachineCallersSeparately(t *testing.T) {
	st := newFakeTenantStore()
	ts := NewTenantService(st, nil)
	ctx := context.Background()

	if _, err := st.CreateTenantWithMember(ctx, "identity-a", store.PlanHobby); err != nil {
		t.Fatal(err)
	}
	if err := st.BindClient(ctx, "client-1", "tea-1"); err != nil {
		t.Fatal(err)
	}

	tid, ok := ts.Tenant(ctx, core.Identity{Subject: "identity-a", Method: "session"})
	if !ok || tid == "" {
		t.Fatalf("session resolve: %q %v", tid, ok)
	}
	tid, ok = ts.Tenant(ctx, core.Identity{Subject: "client-1", Method: "oauth2"})
	if !ok || tid != "tea-1" {
		t.Fatalf("machine resolve: %q %v, want tea-1", tid, ok)
	}
	// An unbound machine key (or the platform bootstrap) resolves ok=false —
	// the caller falls back to workspace:default (core.Base.Authorize), it does
	// not silently inherit any other tenant.
	if _, ok := ts.Tenant(ctx, core.Identity{Subject: "client-unbound", Method: "oauth2"}); ok {
		t.Error("unbound client must resolve ok=false")
	}
}

// TestInvalidateTenantEvictsStaleCachedResolution guards the m13 live-verify
// finding: Tenant's positive TTL cache had no eviction hook, so a subject
// resolved just before its tenant was deleted kept resolving to that
// now-gone tenant for up to core.PositiveTTL (30s) — core.Base.Authorize then
// 403'd every verb against it, including workspaces.Service.List for a caller
// who still owned another workspace. workspaces.Service.Delete now calls
// InvalidateTenant for every member it revokes; this asserts the cache
// primitive it relies on actually evicts, both before (stale hit reproduces
// the bug) and after (miss proves the fix) calling it.
func TestInvalidateTenantEvictsStaleCachedResolution(t *testing.T) {
	st := newFakeTenantStore()
	ts := NewTenantService(st, nil)
	ctx := context.Background()

	if _, err := st.CreateTenantWithMember(ctx, "identity-a", store.PlanHobby); err != nil {
		t.Fatal(err)
	}
	// Resolve under both possible Identity.Method values so the test can prove
	// InvalidateTenant clears both cache-key variants — InvalidateTenant's
	// caller (workspaces.Service.Delete) knows only the subject, never which
	// method minted the cached entry, so it must evict both unconditionally.
	tid, ok := ts.Tenant(ctx, core.Identity{Subject: "identity-a", Method: "session"})
	if !ok || tid != "tea-1" {
		t.Fatalf("initial session resolve: %q %v", tid, ok)
	}
	tid, ok = ts.Tenant(ctx, core.Identity{Subject: "identity-a", Method: "oauth2"})
	if !ok || tid != "tea-1" {
		t.Fatalf("initial oauth2 resolve: %q %v", tid, ok)
	}

	// Simulate the tenant being deleted (the workspaces feature drops the
	// tenant_members row) without invalidating yet: the cached positive answers
	// must still be served — this is the bug, reproduced.
	st.mu.Lock()
	delete(st.members, "identity-a")
	st.mu.Unlock()
	if tid, ok := ts.Tenant(ctx, core.Identity{Subject: "identity-a", Method: "session"}); !ok || tid != "tea-1" {
		t.Fatalf("expected stale session cache hit before invalidation, got %q %v", tid, ok)
	}
	if tid, ok := ts.Tenant(ctx, core.Identity{Subject: "identity-a", Method: "oauth2"}); !ok || tid != "tea-1" {
		t.Fatalf("expected stale oauth2 cache hit before invalidation, got %q %v", tid, ok)
	}

	ts.InvalidateTenant("identity-a")
	if _, ok := ts.Tenant(ctx, core.Identity{Subject: "identity-a", Method: "session"}); ok {
		t.Error("session cache entry survived InvalidateTenant")
	}
	if _, ok := ts.Tenant(ctx, core.Identity{Subject: "identity-a", Method: "oauth2"}); ok {
		t.Error("oauth2 cache entry survived InvalidateTenant")
	}
}

// --- Store-off fallback: NewTenantService(nil, ...) must be inert ---

func TestNewTenantServiceNilStoreReturnsNilResolver(t *testing.T) {
	if ts := NewTenantService(nil, newFakeGranter()); ts != nil {
		t.Errorf("NewTenantService(nil, ...) = %v, want nil (store off => no resolver, no mint)", ts)
	}
}

// w3/m246: a signup workspace is named after the person, as on Render, once.
func TestEnsureTenantNamesSignupWorkspaceAfterThePerson(t *testing.T) {
	ctx := context.Background()
	st := newFakeTenantStore()
	ts := NewTenantService(st, nil)
	who := core.WithIdentity(ctx, core.Identity{Subject: "identity-tian", Name: " Tian Pan ", Human: true})

	tid, err := ts.EnsureTenant(who, "identity-tian", "tian@example.com", true)
	if err != nil {
		t.Fatal(err)
	}
	if got := st.tenants[tid].DisplayName; got != "Tian Pan's Workspace" {
		t.Fatalf("display name = %q, want %q", got, "Tian Pan's Workspace")
	}
	if got := st.tenants[tid].Name; got != tid {
		t.Fatalf("slug changed to %q; it must stay the id", got)
	}

	// A rename the user made later is never overwritten by a later sign-in.
	renamed := st.tenants[tid]
	renamed.DisplayName = "Acme"
	st.tenants[tid] = renamed
	ts.cache.Delete(cacheKey(methodSession, "identity-tian"))
	if _, err := ts.EnsureTenant(who, "identity-tian", "tian@example.com", true); err != nil {
		t.Fatal(err)
	}
	if got := st.tenants[tid].DisplayName; got != "Acme" {
		t.Fatalf("display name = %q after re-login, want the user's %q", got, "Acme")
	}
}

func TestSignupWorkspaceName(t *testing.T) {
	for _, c := range []struct{ name, email, want string }{
		{"Tian Pan", "tian@example.com", "Tian Pan's Workspace"},
		{"", "first.run@dev-3.local", "first.run's Workspace"},
		{"  ", "", ""},
	} {
		if got := signupWorkspaceName(c.name, c.email); got != c.want {
			t.Errorf("signupWorkspaceName(%q, %q) = %q, want %q", c.name, c.email, got, c.want)
		}
	}
}
