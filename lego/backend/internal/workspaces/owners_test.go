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
	ids "github.com/bex-co/bex/lego/backend/internal/id"
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

func (f fakeIdentities) LookupMany(ctx context.Context, subjects []string) map[string]IdentityAttrs {
	return lookupEach(ctx, f, subjects)
}

func (f fakeIdentities) LookupEmails(ctx context.Context, subjects []string) (map[string]string, error) {
	return emailsEach(ctx, f, subjects), nil
}

// MFAEnrolled: a subject in the map is a Kratos identity, enrolled when its
// attributes say MFAEnabled; one absent from it is not human.
func (f fakeIdentities) MFAEnrolled(_ context.Context, subject string) (enrolled, human bool, err error) {
	a, ok := f[subject]
	return a.MFAEnabled, ok, nil
}

// emailsEach is LookupEmails for a fake reader: one Lookup per subject.
func emailsEach(ctx context.Context, r IdentityReader, subjects []string) map[string]string {
	return emailsOf(lookupEach(ctx, r, subjects))
}

// lookupEach is LookupMany for a fake reader: one Lookup per subject.
func lookupEach(ctx context.Context, r IdentityReader, subjects []string) map[string]IdentityAttrs {
	out := map[string]IdentityAttrs{}
	for _, subject := range subjects {
		if attrs, ok := r.Lookup(ctx, subject); ok {
			out[subject] = attrs
		}
	}
	return out
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
	ids := fakeIdentities{"user-a": {Email: "a@example.com", Name: "Ada Lovelace", MFAEnabled: true}}
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
	if m.Subject != "user-a" || m.Role != "admin" || m.Email != "a@example.com" ||
		m.Name != "Ada Lovelace" || !m.MFAEnabled {
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
	// userId is the opaque own- id (w6/m7), NOT the raw subject. An identity
	// that never set the optional name trait renders name: "" (required key,
	// honest empty value).
	m := toRenderTeamMember(MemberView{
		Subject: "user-a", OwnerID: "own-00000000000000000001",
		Role: "admin", Email: "a@example.com", MFAEnabled: true,
	})
	if m.UserID != "usr-00000000000000000001" || m.Name != "" || m.Email != "a@example.com" ||
		m.Status != "active" || m.Role != "ADMIN" || !m.MFAEnabled {
		t.Fatalf("renderTeamMember = %+v", m)
	}
}

func TestToRenderTeamMember_NameTraitThreadsThrough(t *testing.T) {
	// The name trait (w4/m25) lands in Render's required teamMember.name.
	m := toRenderTeamMember(MemberView{
		Subject: "user-a", OwnerID: "own-00000000000000000001",
		Role: "admin", Email: "a@example.com", Name: "Ada Lovelace",
	})
	if m.Name != "Ada Lovelace" {
		t.Fatalf("renderTeamMember = %+v", m)
	}
}

// ownIDPrefix sanity: the constant must match the pinned spec's literal prefix.
func TestOwnIDPrefix(t *testing.T) {
	if ownIDPrefix != "own-" || !strings.HasPrefix("own-abc", ownIDPrefix) {
		t.Fatalf("ownIDPrefix = %q", ownIDPrefix)
	}
}

// --- Render member mutations (w5/m267) ----------------------------------------

type fakeMembers struct {
	roles   map[string]string
	removed []string
	err     error
}

func (f *fakeMembers) ChangeRole(_ context.Context, _, subject, role string) error {
	if f.err != nil {
		return f.err
	}
	f.roles[subject] = role
	return nil
}

func (f *fakeMembers) Remove(_ context.Context, _, subject string) error {
	if f.err != nil {
		return f.err
	}
	f.removed = append(f.removed, subject)
	return nil
}

// The userId the members list answers is what the mutations accept; Render's
// role enum maps onto bex's roles; the members service's refusals pass
// through unchanged.
func TestRenderMemberMutationsResolveTheListedUserID(t *testing.T) {
	st := newFakeStore()
	members := &fakeMembers{roles: map[string]string{}}
	svc := &Service{Base: &core.Base{Authz: &fakeChecker{allow: true}}, Store: st, Members: members}
	w, err := svc.Create(ctxAs("user-a"), "acme", "hobby")
	if err != nil {
		t.Fatal(err)
	}
	listed, err := svc.ListMembers(ctxAs("user-a"), w.ID)
	if err != nil || len(listed) != 1 {
		t.Fatalf("list: %+v %v", listed, err)
	}
	// The members list answers usr- (w5/m277); the own- ids handed out before
	// still resolve.
	userID := ids.UserPublicID(listed[0].OwnerID)
	if _, err := svc.UpdateMember(ctxAs("user-a"), w.ID, listed[0].OwnerID, "ADMIN"); err != nil {
		t.Fatalf("legacy own- userId: %v", err)
	}

	for render, bex := range map[string]string{
		"ADMIN": "admin", "DEVELOPER": "developer", "WORKSPACE_CONTRIBUTOR": "contributor",
		"WORKSPACE_BILLING": "billing", "WORKSPACE_VIEWER": "viewer",
	} {
		m, err := svc.UpdateMember(ctxAs("user-a"), w.ID, userID, render)
		if err != nil || members.roles["user-a"] != bex || renderTeamMemberRole(m.Role) != render ||
			toRenderTeamMember(m).UserID != userID {
			t.Fatalf("%s: member %+v err %v changed %q", render, m, err, members.roles["user-a"])
		}
	}
	if _, err := svc.UpdateMember(ctxAs("user-a"), w.ID, userID, "admin"); !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("lowercase role: %v, want ErrBadRequest", err)
	}
	if _, err := svc.UpdateMember(ctxAs("user-a"), w.ID, "own-00000000000000000099", "ADMIN"); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("unknown userId: %v, want ErrNotFound", err)
	}
	// A raw subject is not a userId.
	if err := svc.RemoveMember(ctxAs("user-a"), w.ID, "user-a"); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("subject as userId: %v, want ErrNotFound", err)
	}

	members.err = core.ErrForbidden
	if err := svc.RemoveMember(ctxAs("user-a"), w.ID, userID); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("members refusal: %v, want it passed through", err)
	}
	members.err = nil
	if err := svc.RemoveMember(ctxAs("user-a"), w.ID, userID); err != nil || len(members.removed) != 1 || members.removed[0] != "user-a" {
		t.Fatalf("remove: %v %v", err, members.removed)
	}
}
