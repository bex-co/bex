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
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// --- fakes -----------------------------------------------------------------

// fakeStore is an in-memory WorkspaceStore mirroring PGStore's classification
// (ErrConflict on duplicate name, ErrNotFound on missing id) so the service's
// error mapping can be asserted without Postgres.
type fakeStore struct {
	mu       sync.Mutex
	tenants  map[string]store.Tenant
	members  map[string][]store.TenantMember // tenantID -> members
	ownerIDs map[string]string               // subject -> own- id (lazy)
	apps     map[string]int                  // tenantID -> service count (ChangePlan's service-cap guard)
}

func newFakeStore() *fakeStore {
	return &fakeStore{tenants: map[string]store.Tenant{}, members: map[string][]store.TenantMember{}}
}

func (f *fakeStore) CreateWorkspace(_ context.Context, name, plan, owner string) (store.Tenant, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, t := range f.tenants {
		if t.Name == name {
			return store.Tenant{}, fmt.Errorf("workspace: %w", store.ErrConflict)
		}
	}
	id := fmt.Sprintf("tea-%d", len(f.tenants)+1)
	t := store.Tenant{ID: id, Name: name, Plan: plan, CreatedAt: time.Unix(int64(len(f.tenants)+1), 0).UTC()}
	f.tenants[id] = t
	f.members[id] = []store.TenantMember{{TenantID: id, Subject: owner, Role: "admin"}}
	return t, nil
}

func (f *fakeStore) GetTenant(_ context.Context, id string) (store.Tenant, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	t, ok := f.tenants[id]
	if !ok {
		return store.Tenant{}, fmt.Errorf("workspace: %w", store.ErrNotFound)
	}
	return t, nil
}

func (f *fakeStore) RenameTenant(_ context.Context, id, name string) (store.Tenant, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	t, ok := f.tenants[id]
	if !ok {
		return store.Tenant{}, fmt.Errorf("workspace: %w", store.ErrNotFound)
	}
	for oid, other := range f.tenants {
		if oid != id && other.Name == name {
			return store.Tenant{}, fmt.Errorf("workspace: %w", store.ErrConflict)
		}
	}
	t.Name = name
	f.tenants[id] = t
	return t, nil
}

func (f *fakeStore) UpdateTenantPlan(_ context.Context, id, plan string) (store.Tenant, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	t, ok := f.tenants[id]
	if !ok {
		return store.Tenant{}, fmt.Errorf("workspace: %w", store.ErrNotFound)
	}
	t.Plan = plan
	f.tenants[id] = t
	return t, nil
}

// CountAppsForTenant returns the fake service count seeded via f.apps (tests
// set it directly) — 0 (unlimited-safe) for any tenant not seeded.
func (f *fakeStore) CountAppsForTenant(_ context.Context, id string) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.apps[id], nil
}

func (f *fakeStore) DeleteTenant(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.tenants[id]; !ok {
		return fmt.Errorf("workspace %s: %w", id, store.ErrNotFound)
	}
	delete(f.tenants, id)
	delete(f.members, id)
	return nil
}

func (f *fakeStore) ListTenantsForSubject(_ context.Context, subject string) ([]store.Tenant, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []store.Tenant
	for id, ms := range f.members {
		for _, m := range ms {
			if m.Subject == subject {
				out = append(out, f.tenants[id])
			}
		}
	}
	// Mirror PGStore.ListTenantsForSubject's "ORDER BY t.created_at": the map
	// iteration above is unordered, but callers (List, GetWorkspace's own-
	// resolution) rely on oldest-first for the "default workspace" contract.
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out, nil
}

func (f *fakeStore) ListTenantMembers(_ context.Context, id string) ([]store.TenantMember, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]store.TenantMember(nil), f.members[id]...), nil
}

// OwnerIDForSubject mints a stable, well-formed own- id per subject (matches the
// PGStore's get-or-create semantics for tests).
func (f *fakeStore) OwnerIDForSubject(_ context.Context, subject string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
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

func (f *fakeStore) CountWorkspacesForSubjectPlan(_ context.Context, subject, plan string) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for id, ms := range f.members {
		if f.tenants[id].Plan != plan {
			continue
		}
		for _, m := range ms {
			if m.Subject == subject {
				n++
			}
		}
	}
	return n, nil
}

// fakeChecker allows or denies uniformly and records the (relation, object) of
// the last check, so tests can assert the verb authorized against the right
// workspace object.
type fakeChecker struct {
	allow      bool
	lastRel    string
	lastObject string
}

func (c *fakeChecker) Check(_ context.Context, _, relation, object string) (bool, error) {
	c.lastRel, c.lastObject = relation, object
	return c.allow, nil
}

// fakeResolver satisfies core.WorkspaceResolver (w1/m9) over a WorkspaceStore's
// ListTenantsForSubject — the same underlying tenant_members data
// TenantForIdentity reads in production, so defaultWorkspace's own- resolution
// exercises the real code path (Base.Tenant) rather than a parallel test-only
// heuristic. fakeStore's ListTenantsForSubject is deterministically sorted
// (oldest first), so index 0 here is stable across test runs.
type fakeResolver struct{ store WorkspaceStore }

func (r *fakeResolver) Tenant(ctx context.Context, id core.Identity) (string, bool) {
	ts, err := r.store.ListTenantsForSubject(ctx, id.Subject)
	if err != nil || len(ts) == 0 {
		return "", false
	}
	return ts[0].ID, true
}

// fakeGranter records grants and can be told to fail (to exercise the
// compensating rollback).
type fakeGranter struct {
	granted []string
	failErr error
}

func (g *fakeGranter) GrantWorkspaceAdmin(_ context.Context, tenantID, subject string) error {
	if g.failErr != nil {
		return g.failErr
	}
	g.granted = append(g.granted, tenantID+"/"+subject)
	return nil
}

// fakeRevoker records revokes.
type fakeRevoker struct{ revoked []string }

func (r *fakeRevoker) RevokeWorkspaceMember(_ context.Context, tenantID, subject, relation string) error {
	r.revoked = append(r.revoked, tenantID+"/"+subject+"/"+relation)
	return nil
}

// fakePurger records purges and can fail.
type fakePurger struct {
	purged  []string
	failErr error
}

func (p *fakePurger) PurgeWorkspace(_ context.Context, tenantID string) error {
	if p.failErr != nil {
		return p.failErr
	}
	p.purged = append(p.purged, tenantID)
	return nil
}

// ctxAs returns a context carrying the given subject identity.
func ctxAs(subject string) context.Context {
	return core.WithIdentity(context.Background(), core.Identity{Subject: subject, Method: "session"})
}

// allowSvc builds a Service with an allow-all checker and the given collaborators.
func allowSvc(st WorkspaceStore, g WorkspaceGranter, r WorkspaceRevoker, kick func(), purgers ...WorkspacePurger) *Service {
	return &Service{
		Base:    &core.Base{Authz: &fakeChecker{allow: true}, Workspace: &fakeResolver{store: st}},
		Store:   st,
		Granter: g,
		Revoker: r,
		Kick:    kick,
		Purgers: purgers,
	}
}

// --- Create ----------------------------------------------------------------

func TestCreate_WritesRowMembershipAndGrant(t *testing.T) {
	st := newFakeStore()
	g := &fakeGranter{}
	svc := allowSvc(st, g, nil, nil)

	w, err := svc.Create(ctxAs("user-a"), "acme", "pro")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if w.Plan != "pro" || w.Role != "admin" || !strings.HasPrefix(w.ID, "tea-") {
		t.Fatalf("view = %+v", w)
	}
	if got, err := st.GetTenant(context.Background(), w.ID); err != nil || got.Name != "acme" {
		t.Fatalf("row not persisted: %+v %v", got, err)
	}
	if len(g.granted) != 1 || g.granted[0] != w.ID+"/user:user-a" {
		t.Fatalf("grant = %v", g.granted)
	}
	members, _ := st.ListTenantMembers(context.Background(), w.ID)
	if len(members) != 1 || members[0].Subject != "user-a" || members[0].Role != "admin" {
		t.Fatalf("members = %+v", members)
	}
}

func TestCreate_DefaultsToHobby(t *testing.T) {
	st := newFakeStore()
	svc := allowSvc(st, &fakeGranter{}, nil, nil)
	w, err := svc.Create(ctxAs("user-a"), "acme", "")
	if err != nil || w.Plan != store.PlanHobby {
		t.Fatalf("want hobby default, got %+v err=%v", w, err)
	}
}

func TestCreate_SixthHobbyRefused(t *testing.T) {
	st := newFakeStore()
	svc := allowSvc(st, &fakeGranter{}, nil, nil)
	ctx := ctxAs("user-a")
	for i := 0; i < 5; i++ {
		if _, err := svc.Create(ctx, fmt.Sprintf("w%d", i), "hobby"); err != nil {
			t.Fatalf("hobby #%d: %v", i, err)
		}
	}
	_, err := svc.Create(ctx, "w5", "hobby")
	if !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("6th hobby: want ErrBadRequest, got %v", err)
	}
	// A paid workspace is unaffected by the Hobby cap.
	if _, err := svc.Create(ctx, "paid", "pro"); err != nil {
		t.Fatalf("pro after 5 hobby: %v", err)
	}
}

func TestCreate_GrantFailureRollsBackRow(t *testing.T) {
	st := newFakeStore()
	g := &fakeGranter{failErr: errors.New("fga down")}
	svc := allowSvc(st, g, nil, nil)
	_, err := svc.Create(ctxAs("user-a"), "acme", "hobby")
	if err == nil {
		t.Fatal("want error on grant failure")
	}
	// No partial state: the compensating delete removed the row.
	if ws, _ := st.ListTenantsForSubject(context.Background(), "user-a"); len(ws) != 0 {
		t.Fatalf("row not rolled back: %+v", ws)
	}
}

func TestCreate_Guards(t *testing.T) {
	st := newFakeStore()
	// Deny-all checker → forbidden before any write.
	denied := &Service{Base: &core.Base{Authz: &fakeChecker{allow: false}}, Store: st, Granter: &fakeGranter{}}
	if _, err := denied.Create(ctxAs("user-a"), "acme", "hobby"); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("deny-all: want forbidden, got %v", err)
	}
	// Bad name / bad plan → bad request (allow-all checker).
	svc := allowSvc(st, &fakeGranter{}, nil, nil)
	if _, err := svc.Create(ctxAs("user-a"), "Bad Name", "hobby"); !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("bad name: want bad request, got %v", err)
	}
	if _, err := svc.Create(ctxAs("user-a"), "acme", "platinum"); !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("bad plan: want bad request, got %v", err)
	}
	// No store wired → unavailable (still after the gate).
	nostore := &Service{Base: &core.Base{Authz: &fakeChecker{allow: true}}}
	if _, err := nostore.Create(ctxAs("user-a"), "acme", "hobby"); !errors.Is(err, core.ErrWorkspacesUnavailable) {
		t.Fatalf("no store: want unavailable, got %v", err)
	}
}

// --- Rename ----------------------------------------------------------------

func TestRename_AdminOnly(t *testing.T) {
	st := newFakeStore()
	svc := allowSvc(st, &fakeGranter{}, nil, nil)
	w, _ := svc.Create(ctxAs("user-a"), "acme", "hobby")

	chk := &fakeChecker{allow: true}
	svc.Base.Authz = chk
	renamed, err := svc.Rename(ctxAs("user-a"), w.ID, "acme2")
	if err != nil || renamed.Name != "acme2" {
		t.Fatalf("rename: %+v %v", renamed, err)
	}
	// Authorized against the exact workspace object with the manage relation.
	if chk.lastRel != core.RelCanManage || chk.lastObject != core.WorkspaceObject(w.ID) {
		t.Fatalf("authorized %s on %s, want %s on %s", chk.lastRel, chk.lastObject, core.RelCanManage, core.WorkspaceObject(w.ID))
	}

	// Non-admin (deny-all) is refused.
	denied := &Service{Base: &core.Base{Authz: &fakeChecker{allow: false}}, Store: st}
	if _, err := denied.Rename(ctxAs("user-b"), w.ID, "x"); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("non-admin rename: want forbidden, got %v", err)
	}
}

func TestRename_NotFound(t *testing.T) {
	svc := allowSvc(newFakeStore(), &fakeGranter{}, nil, nil)
	if _, err := svc.Rename(ctxAs("user-a"), "tea-missing", "x"); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("want not found, got %v", err)
	}
}

// --- Delete ----------------------------------------------------------------

func TestDelete_TearsDownEverything(t *testing.T) {
	st := newFakeStore()
	rev := &fakeRevoker{}
	purge := &fakePurger{}
	var kicks int
	svc := allowSvc(st, &fakeGranter{}, rev, func() { kicks++ }, purge)
	w, _ := svc.Create(ctxAs("user-a"), "acme", "hobby")

	if err := svc.Delete(ctxAs("user-a"), w.ID, "sudo delete workspace acme"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := st.GetTenant(context.Background(), w.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("row not deleted: %v", err)
	}
	if len(rev.revoked) != 1 || rev.revoked[0] != w.ID+"/user:user-a/admin" {
		t.Fatalf("revoked = %v", rev.revoked)
	}
	if kicks != 1 {
		t.Fatalf("kicks = %d, want 1", kicks)
	}
	if len(purge.purged) != 1 || purge.purged[0] != w.ID {
		t.Fatalf("purged = %v", purge.purged)
	}
}

func TestDelete_WrongConfirmationNoSideEffects(t *testing.T) {
	st := newFakeStore()
	rev := &fakeRevoker{}
	var kicks int
	svc := allowSvc(st, &fakeGranter{}, rev, func() { kicks++ })
	w, _ := svc.Create(ctxAs("user-a"), "acme", "hobby")

	// The bare workspace name is deliberately insufficient: Render's guard is
	// the full "sudo delete workspace <name>" phrase (w6/m5, workspace-lifecycle.md).
	err := svc.Delete(ctxAs("user-a"), w.ID, "acme")
	if !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("want bad request, got %v", err)
	}
	if _, err := st.GetTenant(context.Background(), w.ID); err != nil {
		t.Fatalf("workspace destroyed on bad confirmation: %v", err)
	}
	if len(rev.revoked) != 0 || kicks != 0 {
		t.Fatalf("side effects on bad confirmation: revoked=%v kicks=%d", rev.revoked, kicks)
	}
}

func TestDelete_AdminOnly(t *testing.T) {
	st := newFakeStore()
	svc := allowSvc(st, &fakeGranter{}, &fakeRevoker{}, nil)
	w, _ := svc.Create(ctxAs("user-a"), "acme", "hobby")
	denied := &Service{Base: &core.Base{Authz: &fakeChecker{allow: false}}, Store: st}
	if err := denied.Delete(ctxAs("user-b"), w.ID, "sudo delete workspace acme"); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("non-admin delete: want forbidden, got %v", err)
	}
	if _, err := st.GetTenant(context.Background(), w.ID); err != nil {
		t.Fatalf("forbidden delete still removed the row: %v", err)
	}
}

func TestDelete_PurgerFailureSurfaces(t *testing.T) {
	st := newFakeStore()
	svc := allowSvc(st, &fakeGranter{}, &fakeRevoker{}, nil, &fakePurger{failErr: errors.New("bao down")})
	w, _ := svc.Create(ctxAs("user-a"), "acme", "hobby")
	if err := svc.Delete(ctxAs("user-a"), w.ID, "sudo delete workspace acme"); err == nil {
		t.Fatal("want purger error surfaced")
	}
}

// --- List ------------------------------------------------------------------

func TestList_OnlyCallersWorkspaces(t *testing.T) {
	st := newFakeStore()
	svc := allowSvc(st, &fakeGranter{}, nil, nil)
	if _, err := svc.Create(ctxAs("user-a"), "a1", "hobby"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Create(ctxAs("user-b"), "b1", "hobby"); err != nil {
		t.Fatal(err)
	}
	ws, err := svc.List(ctxAs("user-a"))
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(ws) != 1 || ws[0].Name != "a1" {
		t.Fatalf("list = %+v, want only user-a's", ws)
	}
}

// --- ChangePlan --------------------------------------------------------------

func TestChangePlan_UpgradeSucceeds(t *testing.T) {
	st := newFakeStore()
	svc := allowSvc(st, &fakeGranter{}, nil, nil)
	w, _ := svc.Create(ctxAs("user-a"), "acme", "hobby")
	got, err := svc.ChangePlan(ctxAs("user-a"), w.ID, "pro")
	if err != nil {
		t.Fatalf("upgrade: %v", err)
	}
	if got.Plan != "pro" {
		t.Fatalf("plan = %q, want pro", got.Plan)
	}
	if row, _ := st.GetTenant(context.Background(), w.ID); row.Plan != "pro" {
		t.Fatalf("row not persisted: %+v", row)
	}
}

func TestChangePlan_NoopOnSamePlan(t *testing.T) {
	st := newFakeStore()
	svc := allowSvc(st, &fakeGranter{}, nil, nil)
	w, _ := svc.Create(ctxAs("user-a"), "acme", "pro")
	got, err := svc.ChangePlan(ctxAs("user-a"), w.ID, "pro")
	if err != nil || got.Plan != "pro" {
		t.Fatalf("no-op: %+v %v", got, err)
	}
}

func TestChangePlan_DowngradeRefusedOverMemberCap(t *testing.T) {
	st := newFakeStore()
	svc := allowSvc(st, &fakeGranter{}, nil, nil)
	w, _ := svc.Create(ctxAs("user-a"), "acme", "pro")
	st.members[w.ID] = append(st.members[w.ID], store.TenantMember{TenantID: w.ID, Subject: "user-b", Role: "developer"})
	if _, err := svc.ChangePlan(ctxAs("user-a"), w.ID, "hobby"); !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("2 members -> hobby: want ErrBadRequest, got %v", err)
	}
}

func TestChangePlan_DowngradeRefusedOverServiceCap(t *testing.T) {
	st := newFakeStore()
	svc := allowSvc(st, &fakeGranter{}, nil, nil)
	w, _ := svc.Create(ctxAs("user-a"), "acme", "pro")
	if st.apps == nil {
		st.apps = map[string]int{}
	}
	st.apps[w.ID] = 26
	if _, err := svc.ChangePlan(ctxAs("user-a"), w.ID, "hobby"); !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("26 services -> hobby: want ErrBadRequest, got %v", err)
	}
}

func TestChangePlan_DowngradeRefusedOverPerUserWorkspaceCap(t *testing.T) {
	st := newFakeStore()
	svc := allowSvc(st, &fakeGranter{}, nil, nil)
	ctx := ctxAs("user-a")
	for i := 0; i < 5; i++ {
		if _, err := svc.Create(ctx, fmt.Sprintf("h%d", i), "hobby"); err != nil {
			t.Fatalf("hobby #%d: %v", i, err)
		}
	}
	w, _ := svc.Create(ctx, "sixth", "pro")
	if _, err := svc.ChangePlan(ctx, w.ID, "hobby"); !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("6th hobby via downgrade: want ErrBadRequest, got %v", err)
	}
}

func TestChangePlan_DowngradeRefusedOverOutOfPlanRole(t *testing.T) {
	st := newFakeStore()
	svc := allowSvc(st, &fakeGranter{}, nil, nil)
	w, _ := svc.Create(ctxAs("user-a"), "acme", "scale")
	st.members[w.ID][0] = store.TenantMember{TenantID: w.ID, Subject: "user-a", Role: "viewer"}
	if _, err := svc.ChangePlan(ctxAs("user-a"), w.ID, "pro"); !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("viewer on scale->pro: want ErrBadRequest, got %v", err)
	}
}

func TestChangePlan_AdminOnly(t *testing.T) {
	st := newFakeStore()
	svc := allowSvc(st, &fakeGranter{}, nil, nil)
	w, _ := svc.Create(ctxAs("user-a"), "acme", "hobby")
	denied := &Service{Base: &core.Base{Authz: &fakeChecker{allow: false}}, Store: st}
	if _, err := denied.ChangePlan(ctxAs("user-b"), w.ID, "pro"); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("non-admin: want forbidden, got %v", err)
	}
}

// --- plan rules (store) ----------------------------------------------------

func TestPlanLimitsAndGuards(t *testing.T) {
	if l := store.LimitsFor(store.PlanHobby); l.MaxServices != 25 || l.MaxMembers != 1 || l.MaxWorkspacesPerUser != 5 {
		t.Fatalf("hobby limits = %+v", l)
	}
	if l := store.LimitsFor(store.PlanPro); l.MaxServices != 0 || l.MaxMembers != 0 {
		t.Fatalf("pro should be unlimited, got %+v", l)
	}
	if store.CanAddMember(store.PlanHobby, 1) {
		t.Fatal("hobby should refuse a 2nd member")
	}
	if !store.CanAddMember(store.PlanPro, 100) {
		t.Fatal("pro should allow more members")
	}
	if _, err := store.NormalizePlan("nonsense"); !errors.Is(err, store.ErrInvalid) {
		t.Fatalf("bad plan: want invalid, got %v", err)
	}
	if p, _ := store.NormalizePlan(""); p != store.PlanHobby {
		t.Fatalf("empty plan default = %q", p)
	}
	// RolesFor was folded into PlanLimits.AllowedRoles (w6/m12 /simplify pass) —
	// cover the per-plan role ladder directly, not just through ChangePlan/Invite.
	if store.RoleAllowedOnPlan(store.PlanPro, "viewer") {
		t.Fatal("pro should not allow viewer")
	}
	if !store.RoleAllowedOnPlan(store.PlanPro, "developer") {
		t.Fatal("pro should allow developer")
	}
	if !store.RoleAllowedOnPlan(store.PlanScale, "viewer") {
		t.Fatal("scale should allow viewer")
	}
	if !store.RoleAllowedOnPlan(store.PlanHobby, "admin") {
		t.Fatal("hobby should allow admin (the sole member)")
	}
}
