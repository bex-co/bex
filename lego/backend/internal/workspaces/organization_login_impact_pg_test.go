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
	"errors"
	"reflect"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// organization_login_impact_pg_test.go covers the org read behind the org
// Security card's confirmation (w7/m471, docs/ADR012-auth.md §9c): who an
// org-wide requirement would block across every org workspace, over real
// Postgres and the real twofactor.Gate (orgLoginFixture).

// newOrgImpactFixture enrolls every person but the Guest and the Member, and
// binds the Guest's key to the secondary and the Member's to their standalone
// workspace, which is not the org's.
func newOrgImpactFixture(t *testing.T) (*orgLoginFixture, *fakeImpactKeys) {
	t.Helper()
	f := newOrgLoginFixture(t)
	identities := f.svc.Identities.(fakeIdentities)
	for _, enrolled := range []string{f.owner2, f.wsAdmin} {
		identities[enrolled] = IdentityAttrs{MFAEnabled: true}
	}
	identities[f.member] = IdentityAttrs{Email: "member@acme.test"}
	identities[f.guest] = IdentityAttrs{Email: "guest@acme.test", Name: "Guest"}
	keys := &fakeImpactKeys{apiKeys: []BoundAPIKey{
		{ID: "key-guest", Name: "guest key", WorkspaceID: f.secondary, CreatedBy: f.guest},
		{ID: "key-alone", Name: "side project", WorkspaceID: f.standalone, CreatedBy: f.member},
	}}
	f.svc.LoginAssessor, f.svc.LoginImpactKeys = f.gate, keys
	return f, keys
}

func TestOrganizationLoginRequirementImpact_ListsEveryOrgPersonOnceWithTheirWorkspacesKeys(t *testing.T) {
	f, _ := newOrgImpactFixture(t)
	got, err := f.svc.OrganizationLoginRequirementImpact(ctxAs(f.owner), f.org.ID, LoginRequirementTwoFactor)
	if err != nil {
		t.Fatal(err)
	}
	secondary, err := f.pg.GetTenant(t.Context(), f.secondary)
	if err != nil {
		t.Fatal(err)
	}
	userIDs, err := f.pg.OwnerIDsForSubjects(t.Context(), []string{f.member, f.guest})
	if err != nil {
		t.Fatal(err)
	}
	// The Member is in the primary and the secondary: listed once, with no
	// key, since their only key is bound outside the org. The Guest's key
	// names its workspace.
	want := LoginRequirementImpact{
		Members: []LoginImpactMember{
			{UserID: userIDs[f.member], Email: "member@acme.test", LosesAccess: true, APIKeys: []LoginImpactAPIKey{}, SSHKeys: []LoginImpactSSHKey{}},
			{UserID: userIDs[f.guest], Email: "guest@acme.test", Name: "Guest", LosesAccess: true,
				APIKeys: []LoginImpactAPIKey{{ID: "key-guest", Name: "guest key", WorkspaceName: secondary.Label()}}, SSHKeys: []LoginImpactSSHKey{}},
		},
		UnattributedAPIKeys: []LoginImpactAPIKey{},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("impact =\n%+v\nwant\n%+v", got, want)
	}
}

func TestOrganizationLoginRequirementImpact_AnOrgWhereEveryoneIsEnrolledListsNobody(t *testing.T) {
	f, _ := newOrgImpactFixture(t)
	identities := f.svc.Identities.(fakeIdentities)
	identities[f.member] = IdentityAttrs{MFAEnabled: true}
	identities[f.guest] = IdentityAttrs{MFAEnabled: true}
	got, err := f.svc.OrganizationLoginRequirementImpact(ctxAs(f.owner), f.org.ID, LoginRequirementTwoFactor)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Members) != 0 || len(got.UnattributedAPIKeys) != 0 {
		t.Fatalf("impact = %+v, want nobody", got)
	}
}

// The org's own setting is part of what it would enforce: with Google login
// already required, a 2FA confirmation lists who fails either.
func TestOrganizationLoginRequirementImpact_AssessesWithTheOrgsOwnSettings(t *testing.T) {
	f, _ := newOrgImpactFixture(t)
	identities := f.svc.Identities.(fakeIdentities)
	identities[f.member] = IdentityAttrs{MFAEnabled: true}
	identities[f.guest] = IdentityAttrs{MFAEnabled: true}
	if _, err := f.pg.SetOrganizationRequireGoogleLogin(t.Context(), f.org.ID, true); err != nil {
		t.Fatal(err)
	}
	got, err := f.svc.OrganizationLoginRequirementImpact(ctxAs(f.owner), f.org.ID, LoginRequirementTwoFactor)
	if err != nil {
		t.Fatal(err)
	}
	// Only the first Owner has the matching Google account.
	if len(got.Members) != 4 {
		t.Fatalf("impact = %+v, want the four people without a matching Google account", got)
	}
}

func TestOrganizationLoginRequirementImpact_OnlyAnOwnerMayAsk(t *testing.T) {
	f, _ := newOrgImpactFixture(t)
	for _, subject := range []string{f.member, f.guest, f.wsAdmin} {
		if _, err := f.svc.OrganizationLoginRequirementImpact(ctxAs(subject), f.org.ID, LoginRequirementTwoFactor); !errors.Is(err, core.ErrForbidden) {
			t.Fatalf("%s = %v, want ErrForbidden", subject, err)
		}
	}
	_, err := f.svc.OrganizationLoginRequirementImpact(ctxAs("stranger"), f.org.ID, LoginRequirementGoogleLogin)
	var coded *core.CodedError
	if !errors.As(err, &coded) || coded.Code != core.CodeOrganizationNotFound {
		t.Fatalf("a stranger = %v, want ORGANIZATION_NOT_FOUND", err)
	}
	if _, err := f.svc.OrganizationLoginRequirementImpact(ctxAs(f.owner), f.org.ID, "SAML"); !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("an unknown requirement = %v, want ErrBadRequest", err)
	}
	f.svc.LoginImpactKeys = nil
	if _, err := f.svc.OrganizationLoginRequirementImpact(ctxAs(f.owner), f.org.ID, LoginRequirementTwoFactor); !errors.Is(err, core.ErrAuthzUnavailable) {
		t.Fatalf("no key read wired = %v, want ErrAuthzUnavailable", err)
	}
}
