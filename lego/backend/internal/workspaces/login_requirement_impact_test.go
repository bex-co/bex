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
	"reflect"
	"slices"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
	"github.com/bex-co/bex/lego/backend/internal/twofactor"
)

// login_requirement_impact_test.go covers the workspace read behind the enable
// confirmation (w7/m471, docs/ADR012-auth.md §9c): who turning a requirement
// on would block, decided by the real twofactor.Gate over fake Kratos lookups.

// fakeImpactKeys is the credential read: API keys bound to workspaces and
// people's SSH keys.
type fakeImpactKeys struct {
	apiKeys []BoundAPIKey
	sshKeys []store.SSHKey
	err     error
}

func (f *fakeImpactKeys) BoundAPIKeys(_ context.Context, workspaceIDs []string) ([]BoundAPIKey, error) {
	var out []BoundAPIKey
	for _, k := range f.apiKeys {
		if slices.Contains(workspaceIDs, k.WorkspaceID) {
			out = append(out, k)
		}
	}
	return out, f.err
}

func (f *fakeImpactKeys) SSHKeysOf(_ context.Context, subjects []string) ([]store.SSHKey, error) {
	var out []store.SSHKey
	for _, k := range f.sshKeys {
		if slices.Contains(subjects, k.Subject) {
			out = append(out, k)
		}
	}
	return out, f.err
}

// failingEnrollment is Kratos down.
type failingEnrollment struct{}

func (failingEnrollment) MFAEnrolled(context.Context, string) (bool, bool, error) {
	return false, false, errors.New("kratos down")
}

type impactFixture struct {
	*twoFactorFixture
	keys *fakeImpactKeys
}

// newImpactFixture: admin-1 is enrolled with the matching Google account,
// dev-1 is enrolled with the matching Google account, admin-2 has neither.
// All three are members of the workspace.
func newImpactFixture(t *testing.T) *impactFixture {
	t.Helper()
	f := newTwoFactorFixture(t)
	f.identities["admin-1"] = IdentityAttrs{Email: "one@acme.test", Name: "One", MFAEnabled: true}
	f.identities["admin-2"] = IdentityAttrs{Email: "two@acme.test", Name: "Two"}
	f.store.members[f.workspace] = append(f.store.members[f.workspace],
		store.TenantMember{TenantID: f.workspace, Subject: "dev-1", Role: "developer", Kind: store.MemberKindUser},
		store.TenantMember{TenantID: f.workspace, Subject: "admin-2", Role: "admin", Kind: store.MemberKindUser},
	)
	f.svc.LoginAssessor = twofactor.New(twofactor.Config{
		Settings: storeSettings{f.store}, Enrollment: f.identities,
		Google: googleAccounts{"admin-1": true, "dev-1": true, "admin-2": false},
	})
	keys := &fakeImpactKeys{
		apiKeys: []BoundAPIKey{
			{ID: "key-two", Name: "deploy bot", WorkspaceID: f.workspace, CreatedBy: "admin-2"},
			{ID: "key-dev", Name: "ci", WorkspaceID: f.workspace, CreatedBy: "dev-1", CreatedSignIn: core.SignInPassword},
			{ID: "key-orphan", Name: "legacy", WorkspaceID: f.workspace},
			{ID: "key-elsewhere", Name: "other", WorkspaceID: "tea-other", CreatedBy: "admin-2"},
		},
		sshKeys: []store.SSHKey{
			{ID: "ssh-two", Subject: "admin-2", Name: "laptop", Fingerprint: "SHA256:two"},
			{ID: "ssh-one", Subject: "admin-1", Name: "desk", Fingerprint: "SHA256:one", CreatedSignIn: core.SignInGoogle},
		},
	}
	f.svc.LoginImpactKeys = keys
	return &impactFixture{twoFactorFixture: f, keys: keys}
}

func TestLoginRequirementImpact_TwoFactorListsTheUnenrolledMemberWithTheirKeys(t *testing.T) {
	f := newImpactFixture(t)
	got, err := f.svc.LoginRequirementImpact(ctxAs("admin-1"), f.workspace, LoginRequirementTwoFactor)
	if err != nil {
		t.Fatal(err)
	}
	userIDs, _ := f.store.OwnerIDsForSubjects(context.Background(), []string{"admin-2"})
	want := LoginRequirementImpact{
		Members: []LoginImpactMember{{
			UserID: userIDs["admin-2"], Email: "two@acme.test", Name: "Two", LosesAccess: true,
			APIKeys: []LoginImpactAPIKey{{ID: "key-two", Name: "deploy bot"}},
			SSHKeys: []LoginImpactSSHKey{{ID: "ssh-two", Name: "laptop", Fingerprint: "SHA256:two"}},
		}},
		// No recorded creator: refused under 2FA whoever holds it.
		UnattributedAPIKeys: []LoginImpactAPIKey{{ID: "key-orphan", Name: "legacy"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("impact =\n%+v\nwant\n%+v", got, want)
	}
	if got.Members[0].UserID == "admin-2" || got.Members[0].UserID == "" {
		t.Fatalf("userId = %q, want the opaque own- id, never the raw subject", got.Members[0].UserID)
	}
}

func TestLoginRequirementImpact_AWorkspaceWhereEveryoneMeetsItListsNobody(t *testing.T) {
	f := newImpactFixture(t)
	f.store.members[f.workspace] = f.store.members[f.workspace][:2] // admin-1 and dev-1, both enrolled
	f.keys.apiKeys = f.keys.apiKeys[1:2]                            // dev-1's key
	got, err := f.svc.LoginRequirementImpact(ctxAs("admin-1"), f.workspace, LoginRequirementTwoFactor)
	if err != nil {
		t.Fatal(err)
	}
	if got.Members == nil || got.UnattributedAPIKeys == nil || len(got.Members) != 0 || len(got.UnattributedAPIKeys) != 0 {
		t.Fatalf("impact = %+v, want two empty lists (nobody would be blocked)", got)
	}
}

// Google login lists the member whose Google account does not match and the
// keys a recorded stamp refuses; an unstamped key is exempt, so not listed.
func TestLoginRequirementImpact_GoogleLoginSkipsUnstampedKeys(t *testing.T) {
	f := newImpactFixture(t)
	got, err := f.svc.LoginRequirementImpact(ctxAs("admin-1"), f.workspace, LoginRequirementGoogleLogin)
	if err != nil {
		t.Fatal(err)
	}
	byEmail := map[string]LoginImpactMember{}
	for _, m := range got.Members {
		byEmail[m.Email] = m
	}
	two := byEmail["two@acme.test"]
	if !two.LosesAccess || len(two.APIKeys) != 0 || len(two.SSHKeys) != 0 {
		t.Fatalf("admin-2 = %+v, want losing access with no key listed: both their keys are unstamped", two)
	}
	var dev LoginImpactMember
	for _, m := range got.Members {
		if m.Email == "" {
			dev = m
		}
	}
	if dev.LosesAccess || !reflect.DeepEqual(dev.APIKeys, []LoginImpactAPIKey{{ID: "key-dev", Name: "ci"}}) || len(dev.SSHKeys) != 0 {
		t.Fatalf("dev-1 = %+v, want keeping access with the password-stamped key listed", dev)
	}
	if _, listed := byEmail["one@acme.test"]; listed {
		t.Fatalf("admin-1 meets Google login with a Google-stamped SSH key, yet is listed: %+v", got.Members)
	}
	if len(got.Members) != 2 || len(got.UnattributedAPIKeys) != 0 {
		t.Fatalf("impact = %+v, want admin-2 and dev-1 and no unattributed key (unstamped)", got)
	}
}

// The list is what the workspace would enforce after the change: the new
// requirement ORed with the one it already has.
func TestLoginRequirementImpact_AssessesWithWhatTheWorkspaceAlreadyRequires(t *testing.T) {
	f := newImpactFixture(t)
	f.store.members[f.workspace] = f.store.members[f.workspace][:2]
	f.keys.apiKeys, f.keys.sshKeys = f.keys.apiKeys[1:2], nil
	if _, _, err := f.store.SetTenantRequireGoogleLogin(context.Background(), f.workspace, true); err != nil {
		t.Fatal(err)
	}
	got, err := f.svc.LoginRequirementImpact(ctxAs("admin-1"), f.workspace, LoginRequirementTwoFactor)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Members) != 1 || got.Members[0].LosesAccess || len(got.Members[0].APIKeys) != 1 {
		t.Fatalf("impact = %+v, want dev-1's password-stamped key, refused by the Google login already required", got)
	}
}

func TestLoginRequirementImpact_OnlyAnAdminMayAsk(t *testing.T) {
	f := newImpactFixture(t)
	for _, subject := range []string{"dev-1", "viewer-1", "stranger"} {
		if _, err := f.svc.LoginRequirementImpact(ctxAs(subject), f.workspace, LoginRequirementTwoFactor); !errors.Is(err, core.ErrForbidden) {
			t.Fatalf("%s = %v, want ErrForbidden", subject, err)
		}
	}
	// A read: each refusal is on the record, an allowed read is not.
	before := len(f.audit.events)
	if _, err := f.svc.LoginRequirementImpact(ctxAs("admin-1"), f.workspace, LoginRequirementTwoFactor); err != nil {
		t.Fatal(err)
	}
	f.audit.mu.Lock()
	defer f.audit.mu.Unlock()
	if len(f.audit.events) != before || before != 3 {
		t.Fatalf("audit rows = %d before the allowed read, %d after; want 3 denials and no allowed row", before, len(f.audit.events))
	}
	for _, e := range f.audit.events {
		if e.Outcome != core.AuditDenied {
			t.Fatalf("audit row %+v, want only denials", e)
		}
	}
}

// A lookup that cannot answer fails the read: the dialog says it could not
// check, never that nobody would be blocked.
func TestLoginRequirementImpact_AFailedLookupIsUnavailableNotEmpty(t *testing.T) {
	cases := map[string]func(*impactFixture){
		"kratos": func(f *impactFixture) {
			f.svc.LoginAssessor = twofactor.New(twofactor.Config{Settings: storeSettings{f.store}, Enrollment: failingEnrollment{}})
		},
		"keys":        func(f *impactFixture) { f.keys.err = errors.New("hydra down") },
		"no assessor": func(f *impactFixture) { f.svc.LoginAssessor = nil },
		"no keys":     func(f *impactFixture) { f.svc.LoginImpactKeys = nil },
	}
	for name, breakIt := range cases {
		t.Run(name, func(t *testing.T) {
			f := newImpactFixture(t)
			breakIt(f)
			got, err := f.svc.LoginRequirementImpact(ctxAs("admin-1"), f.workspace, LoginRequirementTwoFactor)
			if !errors.Is(err, core.ErrAuthzUnavailable) {
				t.Fatalf("impact = (%+v, %v), want ErrAuthzUnavailable", got, err)
			}
		})
	}
}

func TestLoginRequirementImpact_RefusesAnUnknownRequirement(t *testing.T) {
	f := newImpactFixture(t)
	if _, err := f.svc.LoginRequirementImpact(ctxAs("admin-1"), f.workspace, "SAML"); !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("err = %v, want ErrBadRequest", err)
	}
}
