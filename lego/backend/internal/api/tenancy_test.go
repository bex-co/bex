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

package api

import (
	"context"
	"errors"
	"fmt"
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
	mu      sync.Mutex
	tenants map[string]store.Tenant
	members map[string]string // subject (identity or client id) -> tenantID
	ownerOf map[string]string // identityID -> tenantID (mint gate)
	invites []store.Invite    // outstanding invites (seeded via invite())
	n       int
}

func newFakeTenantStore() *fakeTenantStore {
	return &fakeTenantStore{
		tenants: map[string]store.Tenant{},
		members: map[string]string{},
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
	f.n++
	id := fmt.Sprintf("tea-%d", f.n)
	t := store.Tenant{ID: id, Name: id, Plan: plan}
	f.tenants[id] = t
	f.ownerOf[identityID] = id
	f.members[identityID] = id
	return t, nil
}

func (f *fakeTenantStore) BindClient(_ context.Context, clientID, tenantID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.members[clientID] = tenantID
	return nil
}

func (f *fakeTenantStore) UnbindClient(_ context.Context, clientID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.members, clientID)
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
	mu       sync.Mutex
	failNext int
	tuples   map[string]bool // "relation:tenantID:subject" -> live
	granted  []string        // successful grants, in call order
	revoked  []string
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
	tenantID := strings.TrimPrefix(object, "workspace:")
	return g.tuples[relation+":"+tenantID+":"+subject], nil
}

// --- Mint idempotency + race-safety (tenantService's own logic, above the store) ---

func TestEnsureTenantMintsOnceAndCaches(t *testing.T) {
	st := newFakeTenantStore()
	ts := NewTenantService(st, nil)
	ctx := context.Background()

	first, err := ts.EnsureTenant(ctx, "identity-a", "")
	if err != nil || first == "" {
		t.Fatalf("first mint: %v %q", err, first)
	}
	if len(st.tenants) != 1 {
		t.Fatalf("tenants after first mint = %d, want 1", len(st.tenants))
	}
	second, err := ts.EnsureTenant(ctx, "identity-a", "")
	if err != nil || second != first {
		t.Fatalf("second call: %v %q, want %q", err, second, first)
	}
	if len(st.tenants) != 1 {
		t.Errorf("tenants after second call = %d, want 1 (idempotent)", len(st.tenants))
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
			tid, err := ts.EnsureTenant(ctx, "identity-racer", "")
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

	tid, err := ts.EnsureTenant(ctx, "identity-a", "")
	if err != nil {
		t.Fatalf("mint: %v", err)
	}
	want := []string{"admin:" + tid + ":user:identity-a"}
	if len(granter.granted) != 1 || granter.granted[0] != want[0] {
		t.Fatalf("granted = %v, want %v", granter.granted, want)
	}
	if _, err := ts.EnsureTenant(ctx, "identity-a", ""); err != nil {
		t.Fatalf("second call: %v", err)
	}
	if len(granter.granted) != 1 {
		t.Errorf("granted after second call = %v, want unchanged (no re-grant on a returning caller)", granter.granted)
	}
}

func TestEnsureTenantGrantFailureFailsClosedNotHalfMinted(t *testing.T) {
	st := newFakeTenantStore()
	granter := fgWithFail(1)
	ts := NewTenantService(st, granter)
	ctx := context.Background()

	if _, err := ts.EnsureTenant(ctx, "identity-a", ""); err == nil {
		t.Fatal("grant failure must surface as an error, not silently succeed")
	}
	// The mint itself is idempotent (ON CONFLICT), so retrying must succeed and
	// grant exactly once — no leftover half-onboarded state blocks the retry.
	tid, err := ts.EnsureTenant(ctx, "identity-a", "")
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

	personal, err := ts.EnsureTenant(ctx, "identity-bob", "bob@example.com")
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

	if err := ts.BindKey(ctx, "client-1", ten.ID); err != nil {
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

	if err := ts.BindKey(ctx, "client-1", ten.ID); err == nil {
		t.Fatal("FGA grant failure must surface as an error")
	}
	// No orphaned binding: the mapping row must not survive a failed grant, or
	// the key would authorize against a tenant OpenFGA never heard about.
	if _, err := st.TenantForIdentity(ctx, "client-1"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("binding row after rollback: want ErrNotFound, got %v", err)
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

	if err := ts.BindKey(ctx, "client-1", ten.ID); err != nil {
		t.Fatalf("bind: %v", err)
	}
	if err := ts.UnbindKey(ctx, "client-1"); err != nil {
		t.Fatalf("UnbindKey: %v", err)
	}
	if _, err := st.TenantForIdentity(ctx, "client-1"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("binding row after unbind: want ErrNotFound, got %v", err)
	}
	want := "developer:" + ten.ID + ":user:client-1"
	if len(granter.revoked) != 1 || granter.revoked[0] != want {
		t.Fatalf("revoked = %v, want [%s]", granter.revoked, want)
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
