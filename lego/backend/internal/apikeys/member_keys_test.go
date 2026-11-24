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
	"fmt"
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// member_keys_test.go is w9/m514 and w9/m539: every member role mints an
// API key, as on Render, and lists, edits and revokes the keys they minted
// (I10); only an Admin lists every key or revokes another member's (I14), and
// only a key's creator edits its grant. What a key may do is its creator's
// role and its grant (I9); the e2e TestBelowDeveloperAPIKeysE2E and
// TestEveryRoleListsItsOwnAPIKeysE2E prove both against real OpenFGA.

// relationAudit records the relation of every audit row.
type relationAudit struct{ rows []core.AuditEvent }

func (a *relationAudit) Record(_ context.Context, ev core.AuditEvent) error {
	a.rows = append(a.rows, ev)
	return nil
}

func memberRoles() roleChecker {
	return roleChecker{"viewer": "viewer", "viewer2": "viewer", "contrib": "contributor", "bill": "billing", "dev": "developer", "boss": "admin"}
}

func keyIDs(keys []APIKey) []string {
	out := make([]string, 0, len(keys))
	for _, k := range keys {
		out = append(out, k.ID)
	}
	slices.Sort(out)
	return out
}

// Render: "Go to your Account Settings page … and click Create API key", for
// every member; "Your member role in the selected workspace determines which
// permissions you can grant an API key".
func TestEveryMemberRoleMintsAKeyCappedAtTheirRole(t *testing.T) {
	svc, _, binder := scopedService(memberRoles())
	for _, subject := range []string{"viewer", "contrib", "bill"} {
		all, err := svc.CreateAPIKey(asPerson(subject), "", "status-page", nil)
		if err != nil || !all.Permissions.All {
			t.Fatalf("%s mints an All key: %+v, %v", subject, all.Permissions, err)
		}
		if binder.creators[all.ID] != subject || binder.bound[all.ID] != "tea-a" {
			t.Fatalf("%s's key bound to %q as %q, want tea-a as its creator", subject, binder.bound[all.ID], binder.creators[all.ID])
		}
		if _, err := svc.CreateAPIKey(asPerson(subject), "", "read", grantEntries("services:read")); err != nil {
			t.Errorf("%s's Read only key: %v", subject, err)
		}
	}
	// Deployment information is Render's service events (w9/m621): a Viewer
	// and a Contributor grant it, Billing does not hold it to grant.
	for _, subject := range []string{"viewer", "contrib"} {
		if _, err := svc.CreateAPIKey(asPerson(subject), "", "deploys", grantEntries("services:read", "deploys:read")); err != nil {
			t.Errorf("%s's Deploys Read: %v", subject, err)
		}
	}
	if _, err := svc.CreateAPIKey(asPerson("bill"), "", "deploys", grantEntries("deploys:read")); codeOf(err) != core.KeyPermissionAboveRoleCode {
		t.Errorf("Billing's Deploys Read = %v, want refused", err)
	}
	// The role caps the grant: a Viewer names no Write, Run or Read & write.
	for _, entry := range []string{"services:read_write", "deploys_run:run", "services_env:write", "monitoring:read"} {
		if _, err := svc.CreateAPIKey(asPerson("viewer"), "", "too-much", grantEntries(entry)); codeOf(err) != core.KeyPermissionAboveRoleCode {
			t.Errorf("Viewer's %s = %v, want 403 %s", entry, err, core.KeyPermissionAboveRoleCode)
		}
	}
	// A Contributor reads logs and runs deploys; it creates nothing.
	if _, err := svc.CreateAPIKey(asPerson("contrib"), "", "ops", grantEntries("monitoring:read", "deploys_run:run")); err != nil {
		t.Errorf("Contributor's logs and deploys: %v", err)
	}
	if _, err := svc.CreateAPIKey(asPerson("contrib"), "", "create", grantEntries("services:read_write")); codeOf(err) != core.KeyPermissionAboveRoleCode {
		t.Errorf("Contributor's Services Read & write = %v, want refused", err)
	}
	// Billing grants what it holds: no logs, events or metrics.
	if _, err := svc.CreateAPIKey(asPerson("bill"), "", "logs", grantEntries("monitoring:read")); codeOf(err) != core.KeyPermissionAboveRoleCode {
		t.Errorf("Billing's Monitoring Read = %v, want refused", err)
	}
	if _, err := svc.CreateAPIKey(asPerson("bill"), "", "usage", grantEntries("workspaces:read")); err != nil {
		t.Errorf("Billing's Workspaces Read: %v", err)
	}
	// Not a member: nothing to mint in.
	if _, err := svc.CreateAPIKey(asPerson("stranger"), "tea-a", "x", nil); !errors.Is(err, core.ErrForbidden) {
		t.Errorf("a non-member's mint = %v, want 403", err)
	}
}

func TestCatalogMarksAViewersAndABillingMembersLevels(t *testing.T) {
	svc, _, _ := scopedService(memberRoles())
	levels := func(subject string) map[string]PermissionLevelInfo {
		t.Helper()
		areas, err := svc.PermissionCatalog(asPerson(subject), "")
		if err != nil {
			t.Fatalf("%s reads the catalog: %v", subject, err)
		}
		out := map[string]PermissionLevelInfo{}
		for _, a := range areas {
			for _, p := range a.Permissions {
				for _, lv := range p.Levels {
					out[p.ID+":"+string(lv.Level)] = lv
				}
			}
		}
		return out
	}
	for _, subject := range []string{"viewer", "bill"} {
		got := levels(subject)
		for entry, want := range map[string]struct {
			grantable bool
			role      string
		}{
			"services:read":       {true, "viewer"},
			"services:read_write": {false, "developer"},
			"monitoring:read":     {false, "contributor"},
			"deploys_run:run":     {false, "contributor"},
			"workspaces:read":     {true, "viewer"},
			"audit_logs:read":     {false, "admin"},
		} {
			if lv := got[entry]; lv.Grantable != want.grantable || lv.RequiredRole != want.role {
				t.Errorf("%s's %s = grantable %v needs %q, want %v %q", subject, entry, lv.Grantable, lv.RequiredRole, want.grantable, want.role)
			}
		}
	}
	if lv := levels("contrib")["deploys_run:run"]; !lv.Grantable {
		t.Errorf("Contributor's Run deployments = %+v, want grantable", lv)
	}
	// Deployment information is the one Viewer level a Billing member does
	// not hold (w9/m621): it names can_view_activity, and the level it shows
	// disabled still names the Viewer as a role that may grant it.
	for subject, grantable := range map[string]bool{"viewer": true, "bill": false} {
		if lv := levels(subject)["deploys:read"]; lv.Grantable != grantable || lv.RequiredRole != "viewer" {
			t.Errorf("%s's Deploys Read = grantable %v needs %q, want %v %q", subject, lv.Grantable, lv.RequiredRole, grantable, "viewer")
		}
	}
}

// I10 (w9/m539): every role's list is the keys one minted, as on Render's
// Account Settings. I14: only an Admin asks for every key in the workspace,
// a creator-less key and a key minted by a key included; every other role is
// refused it once, audited against can_manage.
func TestEveryMemberListsOnlyTheirOwnKeys(t *testing.T) {
	svc, keys, binder := scopedService(memberRoles())
	audit := &relationAudit{}
	svc.Audit = audit
	mint := func(subject string) string {
		t.Helper()
		k, err := svc.CreateAPIKey(asPerson(subject), "", subject+"-key", nil)
		if err != nil {
			t.Fatal(err)
		}
		return k.ID
	}
	viewerKey, viewer2Key, devKey, bossKey := mint("viewer"), mint("viewer2"), mint("dev"), mint("boss")
	// A key with no recorded creator (minted before the stamp) is no one's own.
	keys.keys["key-legacy"] = APIKey{ID: "key-legacy", Name: "legacy"}
	binder.bound["key-legacy"] = "tea-a"
	// A key minted by another key (before codex round-7 F3) is no person's.
	keys.keys["key-chained"] = APIKey{ID: "key-chained", Name: "chained", CreatedBy: viewerKey}
	binder.bound["key-chained"] = "tea-a"
	// Another workspace's key is in no list of this one.
	keys.keys["key-elsewhere"] = APIKey{ID: "key-elsewhere", Name: "elsewhere", CreatedBy: "boss"}
	binder.bound["key-elsewhere"] = "tea-b"

	every := []string{bossKey, devKey, "key-chained", "key-legacy", viewer2Key, viewerKey}
	slices.Sort(every)
	for subject, want := range map[string][]string{
		"viewer":  {viewerKey},
		"viewer2": {viewer2Key},
		"contrib": {},
		"bill":    {},
		"dev":     {devKey},
		"boss":    {bossKey},
	} {
		listed, err := svc.ListAPIKeys(asPerson(subject), "", false)
		if err != nil {
			t.Fatalf("%s lists: %v", subject, err)
		}
		if got := keyIDs(listed); !slices.Equal(got, want) {
			t.Errorf("%s lists %v, want their own %v", subject, got, want)
		}

		audit.rows = nil
		all, err := svc.ListAPIKeys(asPerson(subject), "", true)
		if subject == "boss" {
			if got := keyIDs(all); err != nil || !slices.Equal(got, every) {
				t.Errorf("the Admin's allMembers list = %v (%v), want every key %v", got, err, every)
			}
			if len(audit.rows) != 0 {
				t.Errorf("an allowed list wrote audit rows %+v, want none (a read verb records its denials)", audit.rows)
			}
			continue
		}
		if !errors.Is(err, core.ErrForbidden) || all != nil {
			t.Errorf("%s's allMembers list = %v, %v; want 403", subject, keyIDs(all), err)
		}
		if len(audit.rows) != 1 || audit.rows[0].Outcome != core.AuditDenied || audit.rows[0].Relation != core.RelCanManage {
			t.Errorf("%s's refused allMembers list audited %+v, want one denied %s row", subject, audit.rows, core.RelCanManage)
		}
	}

	// A machine caller owns nothing, even the key its own client id minted.
	// An Admin's key passes allMembers, as its creator does; a Developer's
	// key is refused it, as its creator is.
	roles := memberRoles()
	roles[viewerKey], roles[bossKey], roles[devKey] = "viewer", "admin", "developer"
	svc.Authz = roles
	svc.Workspace = fakeWorkspace{viewerKey: "tea-a", bossKey: "tea-a", devKey: "tea-a"}
	for _, id := range []string{viewerKey, bossKey, devKey} {
		asKey := core.WithIdentity(context.Background(), core.Identity{Subject: id, ClientID: id, Method: core.MethodOAuth2})
		if listed, err := svc.ListAPIKeys(asKey, "", false); err != nil || len(listed) != 0 {
			t.Errorf("key %s lists %v (%v), want nothing", id, keyIDs(listed), err)
		}
		all, err := svc.ListAPIKeys(asKey, "", true)
		if id == bossKey {
			if got := keyIDs(all); err != nil || !slices.Equal(got, every) {
				t.Errorf("the Admin's key's allMembers list = %v (%v), want every key", got, err)
			}
		} else if !errors.Is(err, core.ErrForbidden) {
			t.Errorf("key %s's allMembers list = %v, %v; want 403", id, keyIDs(all), err)
		}
	}
}

// I10 and I14: a member below Admin edits and revokes only the key they
// minted, and is refused anyone else's key and a creator-less key with one
// 403 recorded against can_manage. Only an Admin revokes another member's
// key, and editing a key's grant is its creator's alone, an Admin included.
func TestOnlyTheCreatorEditsAndOnlyAnAdminRevokesAnothersKey(t *testing.T) {
	svc, keys, binder := scopedService(memberRoles())
	audit := &relationAudit{}
	svc.Audit = audit
	mint := func(subject string) APIKey {
		t.Helper()
		k, err := svc.CreateAPIKey(asPerson(subject), "", subject, nil)
		if err != nil {
			t.Fatal(err)
		}
		return k
	}
	viewerKey, viewer2Key, devKey, bossKey := mint("viewer"), mint("viewer2"), mint("dev"), mint("boss")
	keys.keys["key-legacy"] = APIKey{ID: "key-legacy", Name: "legacy"}
	binder.bound["key-legacy"] = "tea-a"

	for subject, own := range map[string]APIKey{"viewer": viewerKey, "dev": devKey} {
		caller := asPerson(subject)
		edited, err := svc.UpdateAPIKeyPermissions(caller, "", own.ID, *grantEntries("services:read"))
		if err != nil || !slices.Equal(edited.Permissions.Entries(), []string{"services:read"}) {
			t.Fatalf("%s edits their own key: %+v, %v", subject, edited.Permissions, err)
		}
		for _, other := range []string{viewer2Key.ID, bossKey.ID, "key-legacy", devKey.ID, viewerKey.ID} {
			if other == own.ID {
				continue
			}
			audit.rows = nil
			if _, err := svc.UpdateAPIKeyPermissions(caller, "", other, *grantEntries()); !errors.Is(err, core.ErrForbidden) {
				t.Errorf("%s edits %s = %v, want 403", subject, other, err)
			}
			if err := svc.RevokeAPIKey(caller, "", other); !errors.Is(err, core.ErrForbidden) {
				t.Errorf("%s revokes %s = %v, want 403", subject, other, err)
			}
			if _, ok := keys.keys[other]; !ok {
				t.Fatalf("a refused revoke deleted %s", other)
			}
			if len(audit.rows) != 2 {
				t.Errorf("two refused verbs on %s wrote %d audit rows, want 2", other, len(audit.rows))
			}
			for _, row := range audit.rows {
				if row.Outcome != core.AuditDenied || row.Relation != core.RelCanManage {
					t.Errorf("%s's refused verb on %s audited %s %s, want denied %s", subject, other, row.Outcome, row.Relation, core.RelCanManage)
				}
			}
		}
	}
	if _, err := svc.UpdateAPIKeyPermissions(asPerson("viewer"), "", viewerKey.ID, *grantEntries("services:read_write")); codeOf(err) != core.KeyPermissionAboveRoleCode {
		t.Errorf("Viewer widens their own key past Viewer = %v, want refused", err)
	}
	// An unknown key is no one's own: a Developer is refused it as they are
	// refused anyone's; an Admin is told it does not exist.
	if err := svc.RevokeAPIKey(asPerson("dev"), "", "key-nope"); !errors.Is(err, core.ErrForbidden) {
		t.Errorf("Developer revokes an unknown key = %v, want 403", err)
	}
	if err := svc.RevokeAPIKey(asPerson("boss"), "", "key-nope"); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("Admin revokes an unknown key = %v, want 404", err)
	}

	// The Admin edits no one else's grant: another member's key answers the
	// editor code, a creator-less key the "records no creator" 400 first.
	boss := asPerson("boss")
	if _, err := svc.UpdateAPIKeyPermissions(boss, "", devKey.ID, *grantEntries("services:read")); codeOf(err) != KeyEditorForbiddenCode {
		t.Errorf("Admin edits a Developer's key = %v, want 403 %s", err, KeyEditorForbiddenCode)
	}
	if !slices.Equal(binder.grants[devKey.ID].Entries(), []string{"services:read"}) || binder.grants[devKey.ID].All {
		t.Errorf("a refused edit changed the Developer's grant to %+v", binder.grants[devKey.ID])
	}
	if _, err := svc.UpdateAPIKeyPermissions(boss, "", "key-legacy", *grantEntries("services:read")); !errors.Is(err, core.ErrBadRequest) {
		t.Errorf("Admin scopes a creator-less key = %v, want 400", err)
	}
	if _, err := svc.UpdateAPIKeyPermissions(boss, "", "key-legacy", core.KeyGrant{All: true}); codeOf(err) != KeyEditorForbiddenCode {
		t.Errorf("Admin sets a creator-less key to All = %v, want 403 %s", err, KeyEditorForbiddenCode)
	}

	// The creator revokes their own under the own-keys relation; the Admin
	// revokes another member's key and a creator-less one under can_manage.
	for _, c := range []struct {
		who, id, relation string
	}{
		{"viewer", viewerKey.ID, relOwnKeys},
		{"boss", viewer2Key.ID, core.RelCanManage},
		{"boss", devKey.ID, core.RelCanManage},
		{"boss", "key-legacy", core.RelCanManage},
		{"boss", bossKey.ID, relOwnKeys},
	} {
		audit.rows = nil
		if err := svc.RevokeAPIKey(asPerson(c.who), "", c.id); err != nil {
			t.Fatalf("%s revokes %s: %v", c.who, c.id, err)
		}
		if _, ok := keys.keys[c.id]; ok || binder.bound[c.id] != "" {
			t.Errorf("revoked key %s is still in the registry or bound", c.id)
		}
		if len(audit.rows) != 1 || audit.rows[0].Outcome == core.AuditDenied || audit.rows[0].Relation != c.relation {
			t.Errorf("%s's revoke of %s audited %+v, want one allowed %s row", c.who, c.id, audit.rows, c.relation)
		}
	}
}

// I10 holds the own path to the key's own workspace: a creator who names
// another workspace, or who left the key's, reaches nothing.
func TestOwnKeysStayInTheirWorkspace(t *testing.T) {
	svc, _, binder := scopedService(memberRoles())
	own, err := svc.CreateAPIKey(asPerson("viewer"), "", "mine", nil)
	if err != nil {
		t.Fatal(err)
	}
	// The key lives in tea-b; its creator is a member of tea-a only.
	binder.bound[own.ID] = "tea-b"
	if err := svc.RevokeAPIKey(asPerson("viewer"), "", own.ID); !errors.Is(err, core.ErrForbidden) {
		t.Errorf("revoke in a workspace the creator is not in = %v, want 403", err)
	}
	if err := svc.RevokeAPIKey(asPerson("viewer"), "tea-a", own.ID); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("revoke naming the creator's workspace for a key bound elsewhere = %v, want not found", err)
	}
	if _, ok := binder.bound[own.ID]; !ok {
		t.Fatal("a refused revoke unbound the key")
	}
}

// A machine caller is never a key's minter, so a key's own client id gives
// it no own-key path: a Viewer's All key revokes nothing.
func TestAKeyOwnsNoKey(t *testing.T) {
	svc, keys, binder := scopedService(memberRoles())
	viewerKey, err := svc.CreateAPIKey(asPerson("viewer"), "", "mine", nil)
	if err != nil {
		t.Fatal(err)
	}
	keys.keys["key-chained"] = APIKey{ID: "key-chained", Name: "chained", CreatedBy: viewerKey.ID}
	binder.bound["key-chained"] = "tea-a"
	svc.Authz = roleChecker{"viewer": "viewer", viewerKey.ID: "viewer"}
	svc.Workspace = fakeWorkspace{"viewer": "tea-a", viewerKey.ID: "tea-a"}
	asKey := core.WithIdentity(context.Background(), core.Identity{Subject: viewerKey.ID, ClientID: viewerKey.ID, Method: core.MethodOAuth2})
	for _, id := range []string{"key-chained", viewerKey.ID} {
		if err := svc.RevokeAPIKey(asKey, "", id); !errors.Is(err, core.ErrForbidden) {
			t.Errorf("a Viewer's key revokes %s = %v, want 403", id, err)
		}
	}
	if _, err := svc.CreateAPIKey(asKey, "", "again", nil); !errors.Is(err, core.ErrForbidden) {
		t.Errorf("a Viewer's key mints = %v, want 403 (I4)", err)
	}
}

// A key reaches another key only as its creator would (I14): a Developer's
// key revokes no one else's, an Admin's revokes any member's. A key revoking
// itself, the CLI's logout with a key, keeps the gate it had before w9/m539:
// a Developer's key may, a Viewer's may not (TestAKeyOwnsNoKey).
func TestAKeyRevokesAnotherKeyOnlyAsAnAdmin(t *testing.T) {
	svc, keys, _ := scopedService(memberRoles())
	mint := func(subject string) string {
		t.Helper()
		k, err := svc.CreateAPIKey(asPerson(subject), "", subject, nil)
		if err != nil {
			t.Fatal(err)
		}
		return k.ID
	}
	devKey, dev2Key, bossKey, viewerKey := mint("dev"), mint("dev"), mint("boss"), mint("viewer")
	roles := memberRoles()
	roles[devKey], roles[dev2Key], roles[bossKey] = "developer", "developer", "admin"
	svc.Authz = roles
	svc.Workspace = fakeWorkspace{devKey: "tea-a", dev2Key: "tea-a", bossKey: "tea-a"}
	as := func(id string) context.Context {
		return core.WithIdentity(context.Background(), core.Identity{Subject: id, ClientID: id, Method: core.MethodOAuth2})
	}
	// Even a key its own creator minted is not a Developer's key's to revoke.
	for _, other := range []string{dev2Key, viewerKey, bossKey} {
		if err := svc.RevokeAPIKey(as(devKey), "", other); !errors.Is(err, core.ErrForbidden) {
			t.Errorf("a Developer's key revokes %s = %v, want 403", other, err)
		}
	}
	if err := svc.RevokeAPIKey(as(bossKey), "", viewerKey); err != nil {
		t.Errorf("an Admin's key revokes a Viewer's key: %v", err)
	}
	if err := svc.RevokeAPIKey(as(devKey), "", devKey); err != nil {
		t.Errorf("a Developer's key revokes itself (logout): %v", err)
	}
	for _, id := range []string{viewerKey, devKey} {
		if _, ok := keys.keys[id]; ok {
			t.Errorf("key %s survived its revoke", id)
		}
	}
	if _, ok := keys.keys[dev2Key]; !ok {
		t.Error("a refused revoke deleted the second Developer key")
	}
}

// I10 and I14 over random histories (w9/m539): whatever mints, revokes, role
// changes and creator-less keys happen, every person's default list is
// exactly the keys they minted in the workspace, and the allMembers list is
// answered only to an Admin, holding every key bound there.
func TestOnlyAnAdminsAllMembersListHoldsOthersKeysOverRandomHistories(t *testing.T) {
	roles := []string{"admin", "developer", "contributor", "viewer", "billing"}
	people := []string{"p1", "p2", "p3"}
	rng := rand.New(rand.NewPCG(539, 1))
	for history := range 200 {
		checker := roleChecker{}
		for _, p := range people {
			checker[p] = roles[rng.IntN(len(roles))]
		}
		svc, keys, binder := scopedService(checker)
		legacy := 0
		for step := range 25 {
			switch op := rng.IntN(4); op {
			case 0: // a member mints
				p := people[rng.IntN(len(people))]
				if _, err := svc.CreateAPIKey(asPerson(p), "", "k", nil); err != nil {
					t.Fatalf("history %d step %d: %s (%s) mints: %v", history, step, p, checker[p], err)
				}
			case 1: // a role change (the key follows; listing does not care)
				checker[people[rng.IntN(len(people))]] = roles[rng.IntN(len(roles))]
			case 2: // a creator-less key, or one in another workspace
				legacy++
				id := fmt.Sprintf("legacy-%d", legacy)
				keys.keys[id] = APIKey{ID: id}
				binder.bound[id] = []string{"tea-a", "tea-b"}[rng.IntN(2)]
			case 3: // someone revokes some key; refusals are fine here
				if ids := boundIn(keys, binder, "tea-a"); len(ids) > 0 {
					_ = svc.RevokeAPIKey(asPerson(people[rng.IntN(len(people))]), "", ids[rng.IntN(len(ids))])
				}
			}
			boundHere := boundIn(keys, binder, "tea-a")
			for _, p := range people {
				own := []string{}
				for _, id := range boundHere {
					if keys.keys[id].CreatedBy == p {
						own = append(own, id)
					}
				}
				listed, err := svc.ListAPIKeys(asPerson(p), "", false)
				if got := keyIDs(listed); err != nil || !slices.Equal(got, own) {
					t.Fatalf("history %d step %d: %s (%s) lists %v (%v), want their own %v", history, step, p, checker[p], got, err, own)
				}
				all, err := svc.ListAPIKeys(asPerson(p), "", true)
				if checker[p] == "admin" {
					if got := keyIDs(all); err != nil || !slices.Equal(got, boundHere) {
						t.Fatalf("history %d step %d: the Admin %s lists every key as %v (%v), want %v", history, step, p, got, err, boundHere)
					}
				} else if !errors.Is(err, core.ErrForbidden) || all != nil {
					t.Fatalf("history %d step %d: %s (%s) got the allMembers list %v (%v)", history, step, p, checker[p], keyIDs(all), err)
				}
			}
		}
	}
}

// boundIn is the ids of the registry's keys bound to workspace, sorted.
func boundIn(keys *fakeKeyStore, binder *grantBinder, workspace string) []string {
	out := []string{}
	for id, ws := range binder.bound {
		if _, ok := keys.keys[id]; ok && ws == workspace {
			out = append(out, id)
		}
	}
	slices.Sort(out)
	return out
}
