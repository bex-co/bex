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
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// owners_test.go covers w6/m2's read verbs: GetWorkspace (own-/tea- resolution),
// ListOwners (filters + email), ListMembers (roles + identity lookup), and the
// Render wire-shape mapping in render.go.

// fakeIdentities is an in-memory IdentityReader keyed on subject.
type fakeIdentities map[string]IdentityAttrs

func (f fakeIdentities) Lookup(_ context.Context, subject string) (IdentityAttrs, bool) {
	a, ok := f[subject]
	return a, ok
}

// --- GetWorkspace ------------------------------------------------------------

func TestGetWorkspace_OwnPrefixResolvesToCallersOldestWorkspace(t *testing.T) {
	st := newFakeStore()
	svc := allowSvc(st, &fakeGranter{}, nil, nil)
	first, err := svc.Create(ctxAs("user-a"), "first", "hobby")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Create(ctxAs("user-a"), "second", "hobby"); err != nil {
		t.Fatal(err)
	}

	got, err := svc.GetWorkspace(ctxAs("user-a"), "own-whatever-suffix")
	if err != nil {
		t.Fatalf("GetWorkspace(own-): %v", err)
	}
	if got.ID != first.ID {
		t.Fatalf("own- resolved to %+v, want the oldest workspace %+v", got, first)
	}
}

func TestGetWorkspace_OwnPrefixNoWorkspacesIsNotFound(t *testing.T) {
	svc := allowSvc(newFakeStore(), &fakeGranter{}, nil, nil)
	if _, err := svc.GetWorkspace(ctxAs("user-nobody"), "own-x"); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestGetWorkspace_TeaID(t *testing.T) {
	st := newFakeStore()
	svc := allowSvc(st, &fakeGranter{}, nil, nil)
	w, err := svc.Create(ctxAs("user-a"), "acme", "hobby")
	if err != nil {
		t.Fatal(err)
	}

	// Authorized (allow-all checker): retrieves the exact workspace.
	got, err := svc.GetWorkspace(ctxAs("user-a"), w.ID)
	if err != nil || got.ID != w.ID || got.Name != "acme" {
		t.Fatalf("GetWorkspace(tea-): %+v %v", got, err)
	}

	// Cross-tenant: a deny-all checker (the caller isn't a member) is forbidden,
	// not a leaked lookup.
	denied := &Service{Base: &core.Base{Authz: &fakeChecker{allow: false}}, Store: st}
	if _, err := denied.GetWorkspace(ctxAs("user-b"), w.ID); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("cross-tenant get: want ErrForbidden, got %v", err)
	}

	// Unknown id (authorized, since nil identity check happens after the gate):
	// not found, not a zero-value workspace.
	if _, err := svc.GetWorkspace(ctxAs("user-a"), "tea-missing"); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("unknown id: want ErrNotFound, got %v", err)
	}
}

// --- ListOwners ----------------------------------------------------------------

func TestListOwners_NameAndEmailFilters(t *testing.T) {
	st := newFakeStore()
	ids := fakeIdentities{"user-a": {Email: "a@example.com"}}
	svc := &Service{Base: &core.Base{Authz: &fakeChecker{allow: true}}, Store: st, Identities: ids}
	if _, err := svc.Create(ctxAs("user-a"), "acme", "hobby"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Create(ctxAs("user-a"), "beta", "hobby"); err != nil {
		t.Fatal(err)
	}

	all, err := svc.ListOwners(ctxAs("user-a"), OwnerFilter{})
	if err != nil || len(all) != 2 {
		t.Fatalf("unfiltered: %+v %v", all, err)
	}
	if all[0].Email != "a@example.com" {
		t.Fatalf("email not resolved via Identities: %+v", all[0])
	}

	byName, err := svc.ListOwners(ctxAs("user-a"), OwnerFilter{Names: []string{"beta"}})
	if err != nil || len(byName) != 1 || byName[0].Name != "beta" {
		t.Fatalf("name filter: %+v %v", byName, err)
	}

	byEmail, err := svc.ListOwners(ctxAs("user-a"), OwnerFilter{Emails: []string{"a@example.com"}})
	if err != nil || len(byEmail) != 2 {
		t.Fatalf("email filter (matches both, same admin): %+v %v", byEmail, err)
	}

	none, err := svc.ListOwners(ctxAs("user-a"), OwnerFilter{Emails: []string{"nobody@example.com"}})
	if err != nil || len(none) != 0 {
		t.Fatalf("non-matching email filter: %+v %v", none, err)
	}
}

func TestListOwners_NoIdentitiesOmitsEmail(t *testing.T) {
	st := newFakeStore()
	svc := allowSvc(st, &fakeGranter{}, nil, nil) // Identities left nil
	if _, err := svc.Create(ctxAs("user-a"), "acme", "hobby"); err != nil {
		t.Fatal(err)
	}
	out, err := svc.ListOwners(ctxAs("user-a"), OwnerFilter{})
	if err != nil || len(out) != 1 || out[0].Email != "" {
		t.Fatalf("want empty email with no Identities wired: %+v %v", out, err)
	}
}

// --- ListMembers -----------------------------------------------------------

func TestListMembers_RolesAndIdentities(t *testing.T) {
	st := newFakeStore()
	ids := fakeIdentities{"user-a": {Email: "a@example.com", MFAEnabled: true}}
	svc := &Service{Base: &core.Base{Authz: &fakeChecker{allow: true}}, Store: st, Identities: ids}
	w, err := svc.Create(ctxAs("user-a"), "acme", "hobby")
	if err != nil {
		t.Fatal(err)
	}

	members, err := svc.ListMembers(ctxAs("user-a"), w.ID)
	if err != nil || len(members) != 1 {
		t.Fatalf("ListMembers: %+v %v", members, err)
	}
	m := members[0]
	if m.Subject != "user-a" || m.Role != "admin" || m.Email != "a@example.com" || !m.MFAEnabled {
		t.Fatalf("member = %+v", m)
	}
}

func TestListMembers_UnknownWorkspaceNotFound(t *testing.T) {
	svc := allowSvc(newFakeStore(), &fakeGranter{}, nil, nil)
	if _, err := svc.ListMembers(ctxAs("user-a"), "tea-missing"); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestListMembers_NonMemberForbidden(t *testing.T) {
	st := newFakeStore()
	svc := allowSvc(st, &fakeGranter{}, nil, nil)
	w, _ := svc.Create(ctxAs("user-a"), "acme", "hobby")

	denied := &Service{Base: &core.Base{Authz: &fakeChecker{allow: false}}, Store: st}
	if _, err := denied.ListMembers(ctxAs("user-b"), w.ID); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("want ErrForbidden, got %v", err)
	}
}

// --- Render wire shapes ------------------------------------------------------

func TestRenderOwner_ShapeAndType(t *testing.T) {
	o := toRenderOwner(OwnerView{WorkspaceView: WorkspaceView{ID: "tea-1", Name: "acme"}, Email: "a@example.com"})
	if o.ID != "tea-1" || o.Name != "acme" || o.Email != "a@example.com" || o.Type != "team" {
		t.Fatalf("renderOwner = %+v", o)
	}
}

func TestToOwnerList_CursorIsSiblingNotWrapper(t *testing.T) {
	list := toOwnerList([]OwnerView{{WorkspaceView: WorkspaceView{ID: "tea-1", Name: "acme"}}})
	if len(list) != 1 || list[0].Cursor != "tea-1" || list[0].Owner.ID != "tea-1" {
		t.Fatalf("ownerWithCursor = %+v", list)
	}
}

func TestRenderTeamMemberRole_UppercaseMapping(t *testing.T) {
	cases := map[string]string{
		"admin":       "ADMIN",
		"developer":   "DEVELOPER",
		"contributor": "WORKSPACE_CONTRIBUTOR",
		"billing":     "WORKSPACE_BILLING",
		"viewer":      "WORKSPACE_VIEWER",
		"custom":      "CUSTOM",
	}
	for in, want := range cases {
		if got := renderTeamMemberRole(in); got != want {
			t.Errorf("renderTeamMemberRole(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestToRenderTeamMember_NoNameHonestOmission(t *testing.T) {
	// userId is the opaque own- id (w6/m7), NOT the raw subject.
	m := toRenderTeamMember(MemberView{
		Subject: "user-a", OwnerID: "own-00000000000000000001",
		Role: "admin", Email: "a@example.com", MFAEnabled: true,
	})
	if m.UserID != "own-00000000000000000001" || m.Name != "" || m.Email != "a@example.com" ||
		m.Status != "active" || m.Role != "ADMIN" || !m.MFAEnabled {
		t.Fatalf("renderTeamMember = %+v", m)
	}
}

// ownIDPrefix sanity: the constant must match the pinned spec's literal prefix.
func TestOwnIDPrefix(t *testing.T) {
	if ownIDPrefix != "own-" || !strings.HasPrefix("own-abc", ownIDPrefix) {
		t.Fatalf("ownIDPrefix = %q", ownIDPrefix)
	}
}
