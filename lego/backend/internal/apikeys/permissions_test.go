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

package apikeys

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// permissions_test.go is w9/m496 t006: a key is minted with a chosen grant
// and its grant changed later, by a person, never above its creator's role.

// grantBinder is fakeBinder that also records grants (KeyGrantBinder).
type grantBinder struct {
	*fakeBinder
	grants   map[string]core.KeyGrant
	failSet  bool
	setCalls int
}

func newGrantBinder() *grantBinder {
	return &grantBinder{fakeBinder: newFakeBinder(), grants: map[string]core.KeyGrant{}}
}

func (b *grantBinder) SetKeyGrant(_ context.Context, clientID string, g core.KeyGrant) error {
	b.setCalls++
	if b.failSet {
		return errors.New("postgres: write failed")
	}
	if _, ok := b.bound[clientID]; !ok {
		return core.ErrNotFound
	}
	b.grants[clientID] = g
	return nil
}

func (b *grantBinder) APIKeyGrantsOfTenant(_ context.Context, tenantID string) (map[string]core.KeyGrant, error) {
	out := map[string]core.KeyGrant{}
	for id, g := range b.grants {
		if b.bound[id] == tenantID {
			out[id] = g
		}
	}
	return out, nil
}

// roleChecker answers OpenFGA from each subject's workspace role in tea-a,
// with deploy/gitops/authz/model.fga's derived relations.
type roleChecker map[string]string

var roleRelations = map[string][]string{
	"viewer":      {core.RelCanView, core.RelCanViewActivity, core.RelCanManageSSHKeys},
	"contributor": {core.RelCanView, core.RelCanViewActivity, core.RelCanViewLogs, core.RelCanOperate, core.RelCanManageSSHKeys},
	"billing":     {core.RelCanView, core.RelCanManageSSHKeys, core.RelCanManageBilling},
	"developer": {core.RelCanView, core.RelCanViewActivity, core.RelCanViewLogs, core.RelCanOperate, core.RelCanCreate, core.RelCanViewSensitive,
		core.RelCanManageKeys, core.RelCanManageSSHKeys},
	"admin": {core.RelCanView, core.RelCanViewActivity, core.RelCanViewLogs, core.RelCanOperate, core.RelCanCreate, core.RelCanViewSensitive,
		core.RelCanManageKeys, core.RelCanManageSSHKeys, core.RelCanManage, core.RelCanManageBilling},
}

func (r roleChecker) Check(_ context.Context, subject, relation, object string) (bool, error) {
	if object != core.WorkspaceObject("tea-a") {
		return false, nil
	}
	return slices.Contains(roleRelations[r[strings.TrimPrefix(subject, "user:")]], relation), nil
}

func scopedService(roles roleChecker) (*Service, *fakeKeyStore, *grantBinder) {
	keys := newFakeKeyStore()
	binder := newGrantBinder()
	members := fakeWorkspace{}
	for subject := range roles {
		members[subject] = "tea-a"
	}
	svc := &Service{
		Base:    &core.Base{Namespace: "default", Workspace: members, Authz: roles},
		APIKeys: keys,
		Binding: binder,
	}
	return svc, keys, binder
}

func asPerson(subject string) context.Context {
	return core.WithIdentity(context.Background(), core.Identity{Subject: subject, Method: "session"})
}

func grantEntries(entries ...string) *core.KeyGrant {
	g := core.KeyGrantFromEntries(append([]string{}, entries...))
	return &g
}

func codeOf(err error) string {
	var coded *core.CodedError
	if errors.As(err, &coded) {
		return coded.Code
	}
	return ""
}

func TestMintRecordsTheChosenGrantAndListsIt(t *testing.T) {
	svc, _, binder := scopedService(roleChecker{"dev": "developer"})
	ctx := asPerson("dev")
	key, err := svc.CreateAPIKey(ctx, "", "ci", grantEntries("deploys_run:run", "services:read"))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"deploys_run:run", "services:read"}
	if !slices.Equal(key.Permissions.Entries(), want) || key.Permissions.All {
		t.Fatalf("minted grant = %+v, want %v", key.Permissions, want)
	}
	// Catalog order, not request order.
	if key.Permissions.Grants[0].Permission != "services" {
		t.Errorf("grants not in catalog order: %+v", key.Permissions.Grants)
	}
	if !slices.Equal(binder.grants[key.ID].Entries(), want) {
		t.Fatalf("recorded grant = %v", binder.grants[key.ID].Entries())
	}
	listed, err := svc.ListAPIKeys(ctx, "", false)
	if err != nil || len(listed) != 1 || !slices.Equal(listed[0].Permissions.Entries(), want) {
		t.Fatalf("listed = %+v (%v)", listed, err)
	}
}

func TestMintWithoutPermissionsIsAll(t *testing.T) {
	svc, _, binder := scopedService(roleChecker{"dev": "developer"})
	key, err := svc.CreateAPIKey(asPerson("dev"), "", "terraform", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !key.Permissions.All || binder.setCalls != 0 {
		t.Fatalf("an unnamed grant = %+v with %d writes, want All and none", key.Permissions, binder.setCalls)
	}
	listed, _ := svc.ListAPIKeys(asPerson("dev"), "", false)
	if len(listed) != 1 || !listed[0].Permissions.All {
		t.Fatalf("listed = %+v, want All", listed)
	}
}

// Render: "Your member role in the selected workspace determines which
// permissions you can grant an API key." A Developer cannot grant the
// Workspaces write (webhooks, member roles); an Admin can.
func TestMintRefusesAGrantAboveTheCreatorsRole(t *testing.T) {
	svc, keys, _ := scopedService(roleChecker{"dev": "developer", "boss": "admin"})
	_, err := svc.CreateAPIKey(asPerson("dev"), "", "hooks", grantEntries("workspaces:read_write"))
	if !errors.Is(err, core.ErrForbidden) || codeOf(err) != core.KeyPermissionAboveRoleCode ||
		!strings.Contains(err.Error(), "Members, notifications, webhooks") || !strings.Contains(err.Error(), "admin") {
		t.Fatalf("Developer's Workspaces Read & write = %v, want 403 %s naming the permission and role", err, core.KeyPermissionAboveRoleCode)
	}
	if len(keys.keys) != 0 {
		t.Fatal("a refused grant still minted a credential")
	}
	if _, err := svc.CreateAPIKey(asPerson("dev"), "", "audit", grantEntries("audit_logs:read")); codeOf(err) != core.KeyPermissionAboveRoleCode {
		t.Fatalf("Developer's audit logs read = %v, want refused", err)
	}
	key, err := svc.CreateAPIKey(asPerson("boss"), "", "hooks", grantEntries("workspaces:read_write"))
	if err != nil || key.Permissions.Entries()[0] != "workspaces:read_write" {
		t.Fatalf("Admin's Workspaces Read & write = %+v, %v", key.Permissions, err)
	}
}

func TestMintRefusesAMalformedGrant(t *testing.T) {
	svc, keys, _ := scopedService(roleChecker{"dev": "developer"})
	for _, g := range []core.KeyGrant{
		{Grants: []core.KeyGrantEntry{{Permission: "workflows", Level: core.KeyLevelRead}}},
		{Grants: []core.KeyGrantEntry{{Permission: "deploys_run", Level: core.KeyLevelRead}}},
		{Grants: []core.KeyGrantEntry{{Permission: "services", Level: core.KeyLevelWrite}}},
		{Grants: []core.KeyGrantEntry{{Permission: "services", Level: core.KeyLevelRead}, {Permission: "services", Level: core.KeyLevelReadWrite}}},
		{All: true, Grants: []core.KeyGrantEntry{{Permission: "services", Level: core.KeyLevelRead}}},
	} {
		if _, err := svc.CreateAPIKey(asPerson("dev"), "", "bad", &g); !errors.Is(err, core.ErrBadRequest) {
			t.Errorf("grant %+v = %v, want 400", g, err)
		}
	}
	if len(keys.keys) != 0 {
		t.Fatal("a malformed grant minted a credential")
	}
}

// Render's "Clear all" is a grant too: the key may only identify itself.
func TestMintAcceptsClearAll(t *testing.T) {
	svc, _, binder := scopedService(roleChecker{"dev": "developer"})
	key, err := svc.CreateAPIKey(asPerson("dev"), "", "nothing", grantEntries())
	if err != nil || key.Permissions.All || len(key.Permissions.Grants) != 0 {
		t.Fatalf("Clear all = %+v, %v", key.Permissions, err)
	}
	if g := binder.grants[key.ID]; g.All || g.Entries() == nil {
		t.Fatalf("recorded Clear all = %+v, want an empty grant, not All", g)
	}
}

// A grant that cannot be recorded takes the key back: no credential with All
// survives a scoped mint.
func TestMintRollsBackWhenTheGrantCannotBeRecorded(t *testing.T) {
	svc, keys, binder := scopedService(roleChecker{"dev": "developer"})
	binder.failSet = true
	if _, err := svc.CreateAPIKey(asPerson("dev"), "", "ci", grantEntries("services:read")); err == nil {
		t.Fatal("a failed grant write minted a key")
	}
	if len(keys.keys) != 0 || len(binder.bound) != 0 {
		t.Fatalf("a failed grant write left keys %v bound %v", keys.keys, binder.bound)
	}
}

func TestStoreOffRefusesAScopedMint(t *testing.T) {
	svc := &Service{Base: &core.Base{Namespace: "default"}, APIKeys: newFakeKeyStore()}
	if _, err := svc.CreateAPIKey(asPerson("dev"), "", "ci", grantEntries("services:read")); !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("store-off scoped mint = %v, want 400", err)
	}
	key, err := svc.CreateAPIKey(asPerson("dev"), "", "ci", nil)
	if err != nil || !key.Permissions.All {
		t.Fatalf("store-off All mint = %+v, %v", key.Permissions, err)
	}
}

func TestEditNarrowsAndWidensWithinTheCreatorsRole(t *testing.T) {
	svc, _, binder := scopedService(roleChecker{"dev": "developer", "dev2": "developer", "boss": "admin"})
	key, err := svc.CreateAPIKey(asPerson("dev"), "", "ci", grantEntries("services:read_write"))
	if err != nil {
		t.Fatal(err)
	}
	edited, err := svc.UpdateAPIKeyPermissions(asPerson("dev"), "", key.ID, *grantEntries("services:read"))
	if err != nil || !slices.Equal(edited.Permissions.Entries(), []string{"services:read"}) || edited.Name != "ci" {
		t.Fatalf("narrow = %+v, %v", edited, err)
	}
	if !slices.Equal(binder.grants[key.ID].Entries(), []string{"services:read"}) {
		t.Fatalf("recorded = %v", binder.grants[key.ID].Entries())
	}
	if _, err := svc.UpdateAPIKeyPermissions(asPerson("dev"), "", key.ID, *grantEntries("services:read", "deploys_run:run")); err != nil {
		t.Fatalf("widen within the role: %v", err)
	}
	if _, err := svc.UpdateAPIKeyPermissions(asPerson("dev"), "", key.ID, core.KeyGrant{All: true}); err != nil || !binder.grants[key.ID].All {
		t.Fatalf("back to All: %v", err)
	}

	// Only the creator edits it, as Render's gear is on their own Account
	// Settings (w9/m539): an Admin is refused with the editor code, another
	// Developer with the 403 any other member's key answers.
	for _, grant := range []*core.KeyGrant{grantEntries("services:read"), grantEntries("workspaces:read_write")} {
		if _, err := svc.UpdateAPIKeyPermissions(asPerson("boss"), "", key.ID, *grant); codeOf(err) != KeyEditorForbiddenCode {
			t.Fatalf("Admin edits a Developer's key to %v = %v, want %s", grant.Entries(), err, KeyEditorForbiddenCode)
		}
	}
	if _, err := svc.UpdateAPIKeyPermissions(asPerson("dev2"), "", key.ID, *grantEntries()); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("another Developer's edit = %v, want 403", err)
	}
	if !binder.grants[key.ID].All {
		t.Fatalf("a refused edit changed the grant to %v", binder.grants[key.ID].Entries())
	}
	if _, err := svc.UpdateAPIKeyPermissions(asPerson("dev"), "", key.ID, core.KeyGrant{Grants: []core.KeyGrantEntry{{Permission: "nope", Level: core.KeyLevelRead}}}); !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("unknown permission = %v, want 400", err)
	}
	// An unknown key is no one's own: a Developer is refused it as they are
	// any key but their own, so the answer does not tell them it is missing.
	if _, err := svc.UpdateAPIKeyPermissions(asPerson("dev"), "", "key-nope", *grantEntries()); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("unknown key = %v, want 403", err)
	}
}

// I8: no key edits a grant, its own or another's, whatever its own grant.
func TestAKeyCannotEditAGrant(t *testing.T) {
	roles := roleChecker{"boss": "admin"}
	svc, _, binder := scopedService(roles)
	key, err := svc.CreateAPIKey(asPerson("boss"), "", "ci", grantEntries("services:read"))
	if err != nil {
		t.Fatal(err)
	}
	// The key's own tuple is an Admin's, as its creator's is: only the
	// credential class refuses it.
	roles[key.ID] = "admin"
	svc.Workspace = fakeWorkspace{"boss": "tea-a", key.ID: "tea-a"}
	asKey := core.WithIdentity(context.Background(), core.Identity{Subject: key.ID, ClientID: key.ID, Method: core.MethodOAuth2})
	if _, err := svc.UpdateAPIKeyPermissions(asKey, "", key.ID, core.KeyGrant{All: true}); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("a key widened its own grant: %v", err)
	}
	if binder.grants[key.ID].All {
		t.Fatal("the key's grant changed")
	}
}

func TestCatalogMarksWhatTheCallersRoleMayGrant(t *testing.T) {
	svc, _, _ := scopedService(roleChecker{"dev": "developer", "boss": "admin"})
	level := func(subject, permission string, l core.KeyLevel) PermissionLevelInfo {
		t.Helper()
		areas, err := svc.PermissionCatalog(asPerson(subject), "")
		if err != nil {
			t.Fatal(err)
		}
		for _, a := range areas {
			for _, p := range a.Permissions {
				for _, lv := range p.Levels {
					if p.ID == permission && lv.Level == l {
						return lv
					}
				}
			}
		}
		t.Fatalf("no %s %s in the catalog", permission, l)
		return PermissionLevelInfo{}
	}
	if lv := level("dev", "workspaces", core.KeyLevelReadWrite); lv.Grantable || lv.RequiredRole != "admin" {
		t.Errorf("Developer's Workspaces Read & write = %+v, want not grantable, admin", lv)
	}
	if lv := level("dev", "workspaces", core.KeyLevelRead); !lv.Grantable {
		t.Errorf("Developer's Workspaces Read = %+v, want grantable", lv)
	}
	if lv := level("boss", "workspaces", core.KeyLevelReadWrite); !lv.Grantable || len(lv.Actions) == 0 || lv.Name != "Read & write" {
		t.Errorf("Admin's Workspaces Read & write = %+v", lv)
	}
	if lv := level("dev", "services_env", core.KeyLevelWrite); !lv.Grantable || lv.Actions[0].Name != "Set environment variable and secret file values" {
		t.Errorf("Developer's env-var Write = %+v", lv)
	}
}
