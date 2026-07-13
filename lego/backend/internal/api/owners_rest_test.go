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
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
	"github.com/bex-co/bex/lego/backend/internal/workspaces"
)

// fakeWSStore satisfies workspaces.WorkspaceStore — checked at compile time.
var _ workspaces.WorkspaceStore = (*fakeWSStore)(nil)

// owners_rest_test.go drives w6/m2's owners read API + MCP workspace tools
// through the FULL stack (auth middleware, REST router, MCP registry) with an
// in-memory WorkspaceStore fake — the DoD's "two workspaces and an API key"
// scenario, without requiring live Postgres/OpenFGA (that's
// TestWorkspaceLifecycleE2E in workspaces_e2e_test.go).

// fakeWSStore is an in-memory workspaces.WorkspaceStore. Ids are assigned in
// creation order so ListTenantsForSubject's "oldest first" contract (the
// own- default-workspace resolution) is exercised meaningfully.
type fakeWSStore struct {
	mu       sync.Mutex
	tenants  []store.Tenant
	members  map[string][]store.TenantMember
	invites  map[string][]store.Invite // tenantID -> outstanding invites
	ownerIDs map[string]string         // subject -> own- id (lazy)
}

func newFakeWSStore() *fakeWSStore { return &fakeWSStore{members: map[string][]store.TenantMember{}} }

func (f *fakeWSStore) CreateWorkspace(_ context.Context, name, plan, owner string) (store.Tenant, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	t := store.Tenant{ID: fmt.Sprintf("tea-%d", len(f.tenants)+1), Name: name, Plan: plan,
		CreatedAt: time.Unix(int64(len(f.tenants)+1), 0).UTC()}
	f.tenants = append(f.tenants, t)
	f.members[t.ID] = []store.TenantMember{{TenantID: t.ID, Subject: owner, Role: "admin"}}
	return t, nil
}
func (f *fakeWSStore) GetTenant(_ context.Context, id string) (store.Tenant, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, t := range f.tenants {
		if t.ID == id {
			return t, nil
		}
	}
	return store.Tenant{}, fmt.Errorf("workspace %s: %w", id, store.ErrNotFound)
}
func (f *fakeWSStore) RenameTenant(context.Context, string, string) (store.Tenant, error) {
	return store.Tenant{}, fmt.Errorf("unused: %w", store.ErrNotFound)
}
func (f *fakeWSStore) UpdateTenantPlan(context.Context, string, string) (store.Tenant, error) {
	return store.Tenant{}, fmt.Errorf("unused: %w", store.ErrNotFound)
}
func (f *fakeWSStore) DeleteTenant(context.Context, string) error { return nil }
func (f *fakeWSStore) CountAppsForTenant(context.Context, string) (int, error) { return 0, nil }
func (f *fakeWSStore) ListTenantsForSubject(_ context.Context, subject string) ([]store.Tenant, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []store.Tenant
	for _, t := range f.tenants {
		for _, m := range f.members[t.ID] {
			if m.Subject == subject {
				out = append(out, t)
			}
		}
	}
	return out, nil
}
func (f *fakeWSStore) ListTenantMembers(_ context.Context, id string) ([]store.TenantMember, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]store.TenantMember(nil), f.members[id]...), nil
}
func (f *fakeWSStore) ListInvites(_ context.Context, id string) ([]store.Invite, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]store.Invite(nil), f.invites[id]...), nil
}
func (f *fakeWSStore) CountWorkspacesForSubjectPlan(context.Context, string, string) (int, error) {
	return 0, nil
}

// OwnerIDForSubject mints a stable, well-formed own- id per subject (w6/m7).
func (f *fakeWSStore) OwnerIDForSubject(_ context.Context, subject string) (string, error) {
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

// addMember is a test-only helper to add a second member with a given role
// (m1's create-only-admin behavior gives every workspace one admin, but the
// members endpoint should render whatever the store carries).
func (f *fakeWSStore) addMember(tenantID, subject, role string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.members[tenantID] = append(f.members[tenantID], store.TenantMember{TenantID: tenantID, Subject: subject, Role: role})
}

// addInvite records an outstanding invite — a seat ChangePlan's downgrade
// guards must count even though nobody occupies it yet (w6/m15/t003).
func (f *fakeWSStore) addInvite(tenantID, email, role string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.invites == nil {
		f.invites = map[string][]store.Invite{}
	}
	f.invites[tenantID] = append(f.invites[tenantID], store.Invite{TenantID: tenantID, Email: email, Role: role})
}

// fakeMembershipChecker allows can_view/can_manage on a workspace only for its
// creator (or when the workspace is unknown to it — the coarse
// Authorize(RelCanView) default-workspace gate every verb starts with).
type fakeMembershipChecker struct {
	memberOf map[string]string // tenantID -> allowed subject
}

func (c *fakeMembershipChecker) Check(_ context.Context, subject, _, object string) (bool, error) {
	if !strings.HasPrefix(object, "workspace:tea-") {
		return true, nil // the coarse workspace:default gate every verb starts with
	}
	id := strings.TrimPrefix(object, "workspace:")
	return c.memberOf[id] == subject, nil
}

func TestOwnersREST_ListGetMembers(t *testing.T) {
	st := newFakeWSStore()
	base := serverBase(t, st)
	h, _ := serverWith(t, base, Deps{WorkspaceStore: st})

	first := mustCreate(t, st, "first", "hobby", "client-1")
	second := mustCreate(t, st, "second", "hobby", "client-1")
	st.addMember(second.ID, "bob", "developer")

	// GET /v1/owners: both workspaces, Render-shaped ({owner,cursor}[]).
	rec := do(t, h, "GET", "/v1/owners", testToken, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /v1/owners: %d body=%s", rec.Code, rec.Body.String())
	}
	var list []struct {
		Owner struct {
			ID, Name, Email, Type string
		}
		Cursor string
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatalf("decode list: %v (%s)", err, rec.Body.String())
	}
	if len(list) != 2 {
		t.Fatalf("GET /v1/owners = %+v, want 2", list)
	}
	for _, o := range list {
		if o.Owner.Type != "team" || o.Cursor != o.Owner.ID {
			t.Errorf("owner entry malshaped: %+v", o)
		}
	}

	// GET /v1/owners?name=…: the array-of-string name filter, through the
	// actual query-param wiring (not just the core.OwnerFilter unit test).
	rec = do(t, h, "GET", "/v1/owners?name=second", testToken, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /v1/owners?name=second: %d body=%s", rec.Code, rec.Body.String())
	}
	var filtered []struct {
		Owner struct{ Name string }
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &filtered)
	if len(filtered) != 1 || filtered[0].Owner.Name != "second" {
		t.Fatalf("GET /v1/owners?name=second = %+v, want only \"second\"", filtered)
	}

	// GET /v1/owners/{tea-…}: one workspace.
	rec = do(t, h, "GET", "/v1/owners/"+second.ID, testToken, "")
	var got struct{ ID, Name, Type string }
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /v1/owners/%s: %d", second.ID, rec.Code)
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &got)
	if got.ID != second.ID || got.Name != "second" {
		t.Fatalf("retrieve = %+v", got)
	}

	// GET /v1/owners/{own-…}: the caller's DEFAULT (oldest) workspace, not an error.
	rec = do(t, h, "GET", "/v1/owners/own-whatever", testToken, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /v1/owners/own-…: %d body=%s", rec.Code, rec.Body.String())
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &got)
	if got.ID != first.ID {
		t.Fatalf("own- resolved to %+v, want the oldest workspace %q", got, first.ID)
	}

	// GET /v1/owners/{id}/members: bare array, uppercase role enum.
	rec = do(t, h, "GET", "/v1/owners/"+second.ID+"/members", testToken, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET members: %d body=%s", rec.Code, rec.Body.String())
	}
	var members []struct{ UserID, Role, Status string }
	_ = json.Unmarshal(rec.Body.Bytes(), &members)
	if len(members) != 2 {
		t.Fatalf("members = %+v, want 2", members)
	}
	roles := map[string]bool{}
	for _, m := range members {
		// userId is the opaque own- id (w6/m7), never the raw Kratos subject.
		if !strings.HasPrefix(m.UserID, "own-") {
			t.Errorf("member userId = %q, want an own- id (not a raw subject)", m.UserID)
		}
		if m.Status != "active" {
			t.Errorf("status = %q, want active", m.Status)
		}
		roles[m.Role] = true
	}
	if !roles["ADMIN"] || !roles["DEVELOPER"] {
		t.Fatalf("roles = %+v, want ADMIN + DEVELOPER", roles)
	}
}

// TestOwnersREST_Pagination drives Render's cursor pagination end to end:
// ?limit caps the page, and ?cursor resumes after a given item (w6/001).
func TestOwnersREST_Pagination(t *testing.T) {
	st := newFakeWSStore()
	h, _ := serverWith(t, serverBase(t, st), Deps{WorkspaceStore: st})
	for i := 0; i < 3; i++ {
		mustCreate(t, st, fmt.Sprintf("ws%d", i), "hobby", "client-1")
	}

	type entry struct {
		Owner  struct{ ID string }
		Cursor string
	}
	page := func(query string) []entry {
		rec := do(t, h, "GET", "/v1/owners"+query, testToken, "")
		if rec.Code != http.StatusOK {
			t.Fatalf("GET /v1/owners%s: %d body=%s", query, rec.Code, rec.Body.String())
		}
		var out []entry
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatalf("decode %s: %v (%s)", query, err, rec.Body.String())
		}
		return out
	}

	// First page of 2, then resume after its last cursor for the remainder.
	first := page("?limit=2")
	if len(first) != 2 {
		t.Fatalf("first page = %d entries, want 2", len(first))
	}
	next := page("?limit=2&cursor=" + first[1].Cursor)
	if len(next) != 1 {
		t.Fatalf("second page = %d entries, want 1 (3 total)", len(next))
	}
	if next[0].Owner.ID == first[0].Owner.ID || next[0].Owner.ID == first[1].Owner.ID {
		t.Fatalf("second page %q overlaps first page", next[0].Owner.ID)
	}
	// Paging past the end returns an empty array (the stop signal), not an error.
	if last := page("?cursor=" + next[0].Cursor); len(last) != 0 {
		t.Fatalf("page past end = %d entries, want 0", len(last))
	}
}

// TestOwnersREST_MemberUserIDStableOwnID asserts a member's userId is an opaque
// own- id (never the raw Kratos subject) and stable across calls (w6/m7).
func TestOwnersREST_MemberUserIDStableOwnID(t *testing.T) {
	st := newFakeWSStore()
	h, _ := serverWith(t, serverBase(t, st), Deps{WorkspaceStore: st})
	ws := mustCreate(t, st, "acme", "hobby", "client-1")

	userID := func() string {
		rec := do(t, h, "GET", "/v1/owners/"+ws.ID+"/members", testToken, "")
		if rec.Code != http.StatusOK {
			t.Fatalf("GET members: %d body=%s", rec.Code, rec.Body.String())
		}
		var members []struct{ UserID string }
		if err := json.Unmarshal(rec.Body.Bytes(), &members); err != nil || len(members) != 1 {
			t.Fatalf("decode members: %v (%s)", err, rec.Body.String())
		}
		return members[0].UserID
	}

	first := userID()
	if !strings.HasPrefix(first, "own-") || first == "client-1" {
		t.Fatalf("userId = %q, want an opaque own- id", first)
	}
	if second := userID(); second != first {
		t.Fatalf("userId not stable across calls: %q then %q", first, second)
	}
}

func TestOwnersREST_NoMutations(t *testing.T) {
	st := newFakeWSStore()
	h, _ := serverWith(t, serverBase(t, st), Deps{WorkspaceStore: st})
	for _, m := range []string{"POST", "PATCH", "DELETE"} {
		if code := do(t, h, m, "/v1/owners", testToken, "{}").Code; code != http.StatusNotFound && code != http.StatusMethodNotAllowed {
			t.Errorf("%s /v1/owners: got %d, want no route (404/405) — Render exposes no owners mutations", m, code)
		}
	}
}

func TestOwnersREST_CrossTenantForbidden(t *testing.T) {
	st := newFakeWSStore()
	foreign, err := st.CreateWorkspace(context.Background(), "someone-elses", "hobby", "eve")
	if err != nil {
		t.Fatal(err)
	}
	base := serverBase(t, st)
	base.Authz = &fakeMembershipChecker{memberOf: map[string]string{foreign.ID: "eve"}}
	h, _ := serverWith(t, base, Deps{WorkspaceStore: st})

	rec := do(t, h, "GET", "/v1/owners/"+foreign.ID, testToken, "")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("cross-tenant GET /v1/owners/%s: got %d, want 403", foreign.ID, rec.Code)
	}
	rec = do(t, h, "GET", "/v1/owners/"+foreign.ID+"/members", testToken, "")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("cross-tenant members: got %d, want 403", rec.Code)
	}
}

// mustCreate creates a workspace directly through the store (bypassing
// HTTP — these tests exercise the read side) and returns its Tenant.
func mustCreate(t *testing.T, st *fakeWSStore, name, plan, owner string) store.Tenant {
	t.Helper()
	tenant, err := st.CreateWorkspace(context.Background(), name, plan, owner)
	if err != nil {
		t.Fatalf("CreateWorkspace(%s): %v", name, err)
	}
	return tenant
}

// serverBase is a minimal *core.Base for the owners REST tests — no k8s
// client needed since these tests never touch Apps/Databases. Workspace wires
// the w1/m9 resolver over st, the same real code path GetWorkspace's own- id
// resolution (core.Base.Tenant) uses in production.
func serverBase(t *testing.T, st workspaces.WorkspaceStore) *core.Base {
	t.Helper()
	return &core.Base{Client: fakeClient(), Namespace: "default", Workspace: &apiFakeResolver{store: st}}
}

// apiFakeResolver satisfies core.WorkspaceResolver over a fakeWSStore's
// ListTenantsForSubject — fakeWSStore assigns ids in creation order, so index
// 0 (the caller's oldest workspace) is deterministic across test runs.
type apiFakeResolver struct{ store workspaces.WorkspaceStore }

func (r *apiFakeResolver) Tenant(ctx context.Context, id core.Identity) (string, bool) {
	ts, err := r.store.ListTenantsForSubject(ctx, id.Subject)
	if err != nil || len(ts) == 0 {
		return "", false
	}
	return ts[0].ID, true
}

// IsMember answers from the same membership list, so a caller with several
// workspaces is a member of all of them — not just the first (w6/m14).
func (r *apiFakeResolver) IsMember(ctx context.Context, id core.Identity, tenantID string) (bool, error) {
	ts, err := r.store.ListTenantsForSubject(ctx, id.Subject)
	if err != nil {
		return false, err
	}
	for _, t := range ts {
		if t.ID == tenantID {
			return true, nil
		}
	}
	return false, nil
}
