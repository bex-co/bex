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
	"reflect"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/apikeys"
	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/members"
	"github.com/bex-co/bex/lego/backend/internal/store"
	"github.com/bex-co/bex/lego/backend/internal/twofactor"
	"github.com/bex-co/bex/lego/backend/internal/workspaces"
)

// login_requirement_impact_e2e_test.go walks the enable confirmation's two
// reads (w7/m471, docs/ADR012-auth.md §9c) the way bex-api serves them: over
// GraphQL, on real Postgres and OpenFGA, through the composition root's own
// key read (the bindings and ssh_keys tables, a Hydra stand-in for names and
// stamps) and the real twofactor.Gate.

// impactIdentities is Kratos: every subject is a person with an email and
// the matching Google account, enrolled unless listed.
type impactIdentities struct {
	metadataIdentities
	unenrolled map[string]bool
}

func (i impactIdentities) MFAEnrolled(_ context.Context, subject string) (bool, bool, error) {
	return !i.unenrolled[subject], true, nil
}

// GoogleAccountMatches: everyone linked the Google account of their email.
func (impactIdentities) GoogleAccountMatches(context.Context, string) (bool, bool, error) {
	return true, true, nil
}

// impactE2EFields is the dashboard's selection.
const impactE2EFields = `{ members { userId email losesAccess apiKeys { id name workspaceName } sshKeys { id name fingerprint } } unattributedApiKeys { id name workspaceName } }`

type impactAnswer struct {
	Members []struct {
		UserID      string                         `json:"userId"`
		Email       string                         `json:"email"`
		LosesAccess bool                           `json:"losesAccess"`
		APIKeys     []workspaces.LoginImpactAPIKey `json:"apiKeys"`
		SSHKeys     []workspaces.LoginImpactSSHKey `json:"sshKeys"`
	} `json:"members"`
	Unattributed []workspaces.LoginImpactAPIKey `json:"unattributedApiKeys"`
}

func decodeImpact(t *testing.T, data map[string]any, field string) impactAnswer {
	t.Helper()
	raw, _ := json.Marshal(data[field])
	var out impactAnswer
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("%s = %s: %v", field, raw, err)
	}
	return out
}

func TestLoginRequirementImpactE2E(t *testing.T) {
	identities := impactIdentities{unenrolled: map[string]bool{}}
	hydra := newFakeKeyStore()
	e := newOrgE2E(t, func(base *core.Base, d *Deps) {
		d.Identities = identities
		d.APIKeys = hydra
		d.LoginAssessor = twofactor.New(twofactor.Config{Settings: d.WorkspaceStore.(*store.PGStore), Enrollment: identities, Google: identities})
	})
	ctx := context.Background()
	alice, bob, carol := e.person("alice"), e.person("bob"), e.person("carol")
	identities.unenrolled[bob.Subject] = true

	// acme: alice the admin, bob a developer with a bound key and an SSH key,
	// and a key minted before the created-by stamp.
	acme, err := e.st.CreateWorkspace(ctx, "impact-"+e.stamp, store.PlanPro, alice.Subject)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = e.st.DeleteTenant(context.Background(), acme.ID) })
	if err := e.fga.(workspaces.WorkspaceGranter).GrantWorkspaceAdmin(ctx, acme.ID, "user:"+alice.Subject); err != nil {
		t.Fatal(err)
	}
	if err := e.st.AddMember(ctx, bob.Subject, acme.ID, "developer"); err != nil {
		t.Fatal(err)
	}
	if err := e.fga.(members.RoleGranter).GrantWorkspaceRole(ctx, acme.ID, "user:"+bob.Subject, "developer"); err != nil {
		t.Fatal(err)
	}
	bindKey := func(name, workspaceID, creator string) string {
		t.Helper()
		id := "client-" + name + "-" + e.stamp
		if err := e.st.BindClient(ctx, id, workspaceID); err != nil {
			t.Fatal(err)
		}
		hydra.keys[id] = apikeys.APIKey{ID: id, Name: name, CreatedBy: creator}
		return id
	}
	bobKey := bindKey("bob-ci", acme.ID, bob.Subject)
	orphanKey := bindKey("legacy", acme.ID, "")
	bindKey("alice-ci", acme.ID, alice.Subject) // enrolled creator: keeps working
	// bob's key in no workspace of this read is never listed.
	hydra.keys["client-elsewhere-"+e.stamp] = apikeys.APIKey{ID: "client-elsewhere-" + e.stamp, Name: "elsewhere", CreatedBy: bob.Subject}
	bobSSH := ids.New(ids.SSHKey)
	if _, err := e.st.CreateSSHKey(ctx, store.SSHKey{ID: bobSSH, Subject: bob.Subject, Name: "bob laptop", PublicKey: "ssh-ed25519 AAAATEST", Fingerprint: "SHA256:" + bobSSH}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = e.st.Pool.Exec(context.Background(), `DELETE FROM ssh_keys WHERE id = $1`, bobSSH) })
	bobUserID, err := e.st.OwnerIDForSubject(ctx, bob.Subject)
	if err != nil {
		t.Fatal(err)
	}

	// The workspace read: bob with both keys, the unattributed key alone.
	q := fmt.Sprintf(`{ workspaceLoginRequirementImpact(workspaceId: %q, requirement: TWO_FACTOR) %s }`, acme.ID, impactE2EFields)
	got := decodeImpact(t, e.ok(alice, q), "workspaceLoginRequirementImpact")
	if len(got.Members) != 1 || got.Members[0].UserID != bobUserID || got.Members[0].Email != bob.Subject+"@example.com" || !got.Members[0].LosesAccess {
		t.Fatalf("members = %+v, want bob alone, losing access", got.Members)
	}
	if !reflect.DeepEqual(got.Members[0].APIKeys, []workspaces.LoginImpactAPIKey{{ID: bobKey, Name: "bob-ci"}}) ||
		!reflect.DeepEqual(got.Members[0].SSHKeys, []workspaces.LoginImpactSSHKey{{ID: bobSSH, Name: "bob laptop", Fingerprint: "SHA256:" + bobSSH}}) {
		t.Fatalf("bob's keys = %+v / %+v", got.Members[0].APIKeys, got.Members[0].SSHKeys)
	}
	if !reflect.DeepEqual(got.Unattributed, []workspaces.LoginImpactAPIKey{{ID: orphanKey, Name: "legacy"}}) {
		t.Fatalf("unattributed = %+v, want the key with no recorded creator", got.Unattributed)
	}
	// Only an admin may ask; Google login exempts every unstamped key.
	e.refused(bob, q, "FORBIDDEN")
	google := decodeImpact(t, e.ok(alice, fmt.Sprintf(`{ workspaceLoginRequirementImpact(workspaceId: %q, requirement: GOOGLE_LOGIN) %s }`, acme.ID, impactE2EFields)), "workspaceLoginRequirementImpact")
	if len(google.Unattributed) != 0 {
		t.Fatalf("Google login listed unstamped keys: %+v", google.Unattributed)
	}

	// The org read: acme becomes an org's primary, alice its Owner; carol is
	// in a secondary with a key bound there.
	e.ok(alice, fmt.Sprintf(`mutation { changeWorkspacePlan(id: %q, plan: "scale") { id } }`, acme.ID))
	orgID := e.ok(alice, fmt.Sprintf(`{ workspace(id: %q) { organization { id } } }`, acme.ID))["workspace"].(map[string]any)["organization"].(map[string]any)["id"].(string)
	side := e.ok(alice, fmt.Sprintf(`mutation { createOrganizationWorkspace(organizationId: %q, name: "side-%s") { id name } }`, orgID, e.stamp))["createOrganizationWorkspace"].(map[string]any)
	sideID := side["id"].(string)
	t.Cleanup(func() { _ = e.st.DeleteTenant(context.Background(), sideID) })
	if err := e.st.AddMember(ctx, carol.Subject, sideID, "developer"); err != nil {
		t.Fatal(err)
	}
	identities.unenrolled[carol.Subject] = true
	carolKey := bindKey("carol-ci", sideID, carol.Subject)

	oq := fmt.Sprintf(`{ organizationLoginRequirementImpact(organizationId: %q, requirement: TWO_FACTOR) %s }`, orgID, impactE2EFields)
	org := decodeImpact(t, e.ok(alice, oq), "organizationLoginRequirementImpact")
	byEmail := map[string]int{}
	for i, m := range org.Members {
		byEmail[m.Email] = i
	}
	if len(org.Members) != 2 {
		t.Fatalf("org members = %+v, want bob and carol", org.Members)
	}
	acmeName := e.ok(alice, fmt.Sprintf(`{ workspace(id: %q) { name } }`, acme.ID))["workspace"].(map[string]any)["name"].(string)
	b, c := org.Members[byEmail[bob.Subject+"@example.com"]], org.Members[byEmail[carol.Subject+"@example.com"]]
	if !reflect.DeepEqual(b.APIKeys, []workspaces.LoginImpactAPIKey{{ID: bobKey, Name: "bob-ci", WorkspaceName: acmeName}}) || len(b.SSHKeys) != 1 {
		t.Fatalf("bob on the org read = %+v, want his acme key named by its workspace and his SSH key", b)
	}
	if !reflect.DeepEqual(c.APIKeys, []workspaces.LoginImpactAPIKey{{ID: carolKey, Name: "carol-ci", WorkspaceName: side["name"].(string)}}) || !c.LosesAccess {
		t.Fatalf("carol on the org read = %+v, want her key in the secondary", c)
	}
	if !reflect.DeepEqual(org.Unattributed, []workspaces.LoginImpactAPIKey{{ID: orphanKey, Name: "legacy", WorkspaceName: acmeName}}) {
		t.Fatalf("org unattributed = %+v", org.Unattributed)
	}
	// bob is an org Member, not an Owner.
	e.refused(bob, oq, "FORBIDDEN")
}
