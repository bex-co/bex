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
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// member_keys_test.go is w9/m514: every member role mints an API key, as on
// Render, and a member below Developer lists, edits and revokes the keys they
// minted and no other (I10). What a key may do is its creator's role and its
// grant (I9); the e2e TestBelowDeveloperAPIKeysE2E proves that half against
// real OpenFGA.

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
		if _, err := svc.CreateAPIKey(asPerson(subject), "", "read", grantEntries("services:read", "deploys:read")); err != nil {
			t.Errorf("%s's Read only key: %v", subject, err)
		}
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
	// Billing grants what it holds: the Read levels a Viewer may, no logs.
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
}

// I10: below Developer, the list is the keys one minted; a Developer and an
// Admin still see every key in the workspace.
func TestABelowDeveloperMemberListsOnlyTheirOwnKeys(t *testing.T) {
	svc, keys, binder := scopedService(memberRoles())
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

	every := []string{bossKey, devKey, "key-chained", "key-legacy", viewer2Key, viewerKey}
	slices.Sort(every)
	for subject, want := range map[string][]string{
		"viewer":  {viewerKey},
		"viewer2": {viewer2Key},
		"contrib": {},
		"bill":    {},
		"dev":     every,
		"boss":    every,
	} {
		listed, err := svc.ListAPIKeys(asPerson(subject), "")
		if err != nil {
			t.Fatalf("%s lists: %v", subject, err)
		}
		if got := keyIDs(listed); !slices.Equal(got, want) {
			t.Errorf("%s lists %v, want %v", subject, got, want)
		}
	}
	// A machine caller owns nothing, even the key its own client id minted.
	svc.Authz = roleChecker{"viewer": "viewer", viewerKey: "viewer"}
	svc.Workspace = fakeWorkspace{"viewer": "tea-a", viewerKey: "tea-a"}
	asKey := core.WithIdentity(context.Background(), core.Identity{Subject: viewerKey, ClientID: viewerKey, Method: core.MethodOAuth2})
	if listed, err := svc.ListAPIKeys(asKey, ""); err != nil || len(listed) != 0 {
		t.Errorf("a Viewer's key lists %v (%v), want nothing", keyIDs(listed), err)
	}
}

// I10: a Viewer edits and revokes the key they minted, and is refused
// another member's with the 403 every member below Developer got before,
// recorded once against can_manage_keys.
func TestABelowDeveloperMemberEditsAndRevokesOnlyTheirOwnKeys(t *testing.T) {
	svc, keys, binder := scopedService(memberRoles())
	audit := &relationAudit{}
	svc.Audit = audit
	own, err := svc.CreateAPIKey(asPerson("viewer"), "", "mine", nil)
	if err != nil {
		t.Fatal(err)
	}
	theirs, err := svc.CreateAPIKey(asPerson("viewer2"), "", "theirs", nil)
	if err != nil {
		t.Fatal(err)
	}
	devKey, err := svc.CreateAPIKey(asPerson("dev"), "", "dev", nil)
	if err != nil {
		t.Fatal(err)
	}
	keys.keys["key-legacy"] = APIKey{ID: "key-legacy", Name: "legacy"}
	binder.bound["key-legacy"] = "tea-a"
	viewer := asPerson("viewer")

	edited, err := svc.UpdateAPIKeyPermissions(viewer, "", own.ID, *grantEntries("services:read"))
	if err != nil || !slices.Equal(edited.Permissions.Entries(), []string{"services:read"}) {
		t.Fatalf("Viewer edits their own key: %+v, %v", edited.Permissions, err)
	}
	if _, err := svc.UpdateAPIKeyPermissions(viewer, "", own.ID, *grantEntries("services:read_write")); codeOf(err) != core.KeyPermissionAboveRoleCode {
		t.Errorf("Viewer widens their own key past Viewer = %v, want refused", err)
	}
	for _, other := range []string{theirs.ID, devKey.ID, "key-legacy"} {
		audit.rows = nil
		if _, err := svc.UpdateAPIKeyPermissions(viewer, "", other, *grantEntries()); !errors.Is(err, core.ErrForbidden) {
			t.Errorf("Viewer edits %s = %v, want 403", other, err)
		}
		if err := svc.RevokeAPIKey(viewer, "", other); !errors.Is(err, core.ErrForbidden) {
			t.Errorf("Viewer revokes %s = %v, want 403", other, err)
		}
		if _, ok := keys.keys[other]; !ok {
			t.Fatalf("a refused revoke deleted %s", other)
		}
		for _, row := range audit.rows {
			if row.Outcome != core.AuditDenied || row.Relation != core.RelCanManageKeys {
				t.Errorf("Viewer's refused verb on %s audited %s %s, want denied can_manage_keys", other, row.Outcome, row.Relation)
			}
		}
		if len(audit.rows) != 2 {
			t.Errorf("two refused verbs on %s wrote %d audit rows, want 2", other, len(audit.rows))
		}
	}
	// An unknown key answers as before: not found, after the same refusal a
	// Developer's request would not get.
	if err := svc.RevokeAPIKey(asPerson("dev"), "", "key-nope"); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("Developer revokes an unknown key = %v, want 404", err)
	}

	audit.rows = nil
	if err := svc.RevokeAPIKey(viewer, "", own.ID); err != nil {
		t.Fatalf("Viewer revokes their own key: %v", err)
	}
	if _, ok := keys.keys[own.ID]; ok || binder.bound[own.ID] != "" {
		t.Error("the revoked key is still in the registry or bound")
	}
	if len(audit.rows) != 1 || audit.rows[0].Outcome == core.AuditDenied || audit.rows[0].Relation != relOwnKeys {
		t.Errorf("own revoke audited %+v, want one allowed %s row", audit.rows, relOwnKeys)
	}
	// A Developer still revokes another member's key.
	if err := svc.RevokeAPIKey(asPerson("dev"), "", theirs.ID); err != nil {
		t.Errorf("Developer revokes a Viewer's key: %v", err)
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
