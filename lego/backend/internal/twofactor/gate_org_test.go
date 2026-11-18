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

package twofactor

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// gate_org_test.go covers the org half of the gate (w7/m466,
// docs/ADR012-auth.md §9c): a workspace's effective requirement is its own
// setting ORed with its org's, for both requirements; a refusal the org's
// setting decided names the org; the org's own objects are held to the org's
// setting alone; an unreadable setting fails closed; and an org change takes
// effect at once on the replica that invalidates it.

const testOrg = "org-acme"

// newOrgFixture: tea-org is in testOrg, tea-alone is standalone. Neither
// requires anything of its own until a case sets it.
func newOrgFixture() *googleFixture {
	f := newGoogleFixture()
	f.settings.orgs = map[string]core.LoginOrganization{}
	f.settings.workspaceOrg = map[string]string{"tea-org": testOrg}
	return f
}

func (f *googleFixture) setOrg(twoFactor, google bool) {
	f.settings.mu.Lock()
	defer f.settings.mu.Unlock()
	f.settings.orgs[testOrg] = core.LoginOrganization{ID: testOrg, Name: "Acme", TwoFactor: twoFactor, GoogleLogin: google}
}

func (f *googleFixture) setOwn(workspaceID string, twoFactor, google bool) {
	f.settings.mu.Lock()
	defer f.settings.mu.Unlock()
	f.settings.on[workspaceID] = twoFactor
	f.settings.google[workspaceID] = google
}

// The four combinations of the workspace's own setting and its org's, for
// each requirement, on an org workspace and a standalone one: the requirement
// holds when either does; the refusal names the org exactly when the org's
// setting requires it.
func TestTheEffectiveRequirementIsTheWorkspacesOwnORItsOrganizations(t *testing.T) {
	type requirement struct {
		name    string
		code    string
		set     func(f *googleFixture, ws string, own, org bool)
		failing core.Identity // does not meet the requirement
		meeting core.Identity // meets it
	}
	requirements := []requirement{
		{
			name:    "2FA",
			code:    core.CodeWorkspaceTwoFactorRequired,
			failing: signedIn("google-member", core.SignInGoogle), // Google, no second factor
			meeting: signedIn("google-enrolled", core.SignInGoogle),
			set: func(f *googleFixture, ws string, own, org bool) {
				f.setOwn(ws, own, false)
				if ws == "tea-org" {
					f.setOrg(org, false)
				}
			},
		},
		{
			name:    "Google login",
			code:    core.CodeWorkspaceGoogleLoginRequired,
			failing: signedIn("google-enrolled", core.SignInPassword),
			meeting: signedIn("google-enrolled", core.SignInGoogle),
			set: func(f *googleFixture, ws string, own, org bool) {
				f.setOwn(ws, false, own)
				if ws == "tea-org" {
					f.setOrg(false, org)
				}
			},
		},
	}
	for _, r := range requirements {
		for _, ws := range []string{"tea-org", "tea-alone"} {
			for _, own := range []bool{false, true} {
				for _, org := range []bool{false, true} {
					if ws == "tea-alone" && org {
						continue // a standalone workspace has no org setting to OR in
					}
					t.Run(fmt.Sprintf("%s/%s/own=%v/org=%v", r.name, ws, own, org), func(t *testing.T) {
						f := newOrgFixture()
						r.set(f, ws, own, org)
						if err := f.gate.Check(context.Background(), ws, r.meeting); err != nil {
							t.Fatalf("a principal who meets it = %v", err)
						}
						err := f.gate.Check(context.Background(), ws, r.failing)
						if !own && !org {
							if err != nil {
								t.Fatalf("nothing required, refused: %v", err)
							}
							return
						}
						var coded *core.CodedError
						if !errors.Is(err, core.ErrForbidden) || !errors.As(err, &coded) || coded.Code != r.code {
							t.Fatalf("err = %v, want %s", err, r.code)
						}
						if coded.Params["workspaceId"] != ws {
							t.Fatalf("params = %v, want workspaceId %s", coded.Params, ws)
						}
						if named := coded.Params["organizationId"] == testOrg; named != org {
							t.Fatalf("params = %v: the org named = %v, want %v (only when the org's setting requires it)", coded.Params, named, org)
						}
					})
				}
			}
		}
	}
}

// An org object is held to the org's own setting, never a workspace's: an
// org whose workspace requires 2FA but that requires nothing itself refuses
// nobody on its own objects.
func TestTheOrgsOwnObjectsAreHeldToTheOrgsSettingAlone(t *testing.T) {
	f := newOrgFixture()
	member := signedIn("google-member", core.SignInGoogle)
	f.setOwn("tea-org", true, false)
	f.setOrg(false, false)
	if err := f.gate.CheckOrganization(context.Background(), testOrg, member); err != nil {
		t.Fatalf("an org requiring nothing = %v", err)
	}
	f.setOrg(true, false)
	f.gate.Invalidate(core.OrganizationObject(testOrg))
	err := f.gate.CheckOrganization(context.Background(), testOrg, member)
	var coded *core.CodedError
	if !errors.As(err, &coded) || coded.Code != core.CodeWorkspaceTwoFactorRequired || coded.Params["organizationId"] != testOrg || coded.Params["organizationName"] != "Acme" {
		t.Fatalf("an org requiring 2FA = %v, want the refusal naming it", err)
	}
	if _, named := coded.Params["workspaceId"]; named {
		t.Fatalf("params = %v, want no workspace on an org object", coded.Params)
	}
	if err := f.gate.CheckOrganization(context.Background(), testOrg, signedIn("google-enrolled", core.SignInGoogle)); err != nil {
		t.Fatalf("an enrolled member on the org = %v", err)
	}
	// A key with no recorded creator is refused naming the org.
	f.keys.keys["old-key"] = true
	err = f.gate.CheckOrganization(context.Background(), testOrg, core.Identity{Subject: "old-key", ClientID: "old-key", Method: "oauth2"})
	if !errors.As(err, &coded) || coded.Code != core.CodeWorkspaceTwoFactorRequired || coded.Params["organizationId"] != testOrg {
		t.Fatalf("an unattributed key on the org = %v", err)
	}
	// An empty org id is nothing to enforce.
	if err := f.gate.CheckOrganization(context.Background(), "", member); err != nil {
		t.Fatalf("empty org = %v", err)
	}
}

// An unreadable setting is never "not required": a workspace read and an org
// read that fail both fail closed, and are not cached.
func TestAnUnreadableOrganizationSettingFailsClosed(t *testing.T) {
	f := newOrgFixture()
	f.settings.err = errors.New("db down")
	member := signedIn("google-enrolled", core.SignInGoogle)
	if err := f.gate.Check(context.Background(), "tea-org", member); !errors.Is(err, core.ErrAuthzUnavailable) {
		t.Fatalf("Check with the store down = %v, want ErrAuthzUnavailable", err)
	}
	if err := f.gate.CheckOrganization(context.Background(), testOrg, member); !errors.Is(err, core.ErrAuthzUnavailable) {
		t.Fatalf("CheckOrganization with the store down = %v, want ErrAuthzUnavailable", err)
	}
	f.settings.err = nil
	if err := f.gate.CheckOrganization(context.Background(), testOrg, member); err != nil {
		t.Fatalf("after the store recovered = %v (an error must not be cached)", err)
	}
}

// An org change takes effect at once on the replica that invalidates the
// org's object and its workspaces, and within the TTL elsewhere.
func TestAnOrganizationChangeTakesEffectAtOnceWhereItIsInvalidated(t *testing.T) {
	f := newOrgFixture()
	member := signedIn("google-member", core.SignInGoogle)
	if err := f.gate.Check(context.Background(), "tea-org", member); err != nil {
		t.Fatal(err)
	}
	if err := f.gate.CheckOrganization(context.Background(), testOrg, member); err != nil {
		t.Fatal(err)
	}
	f.setOrg(true, false)
	// Cached: another replica still answers the old value until the TTL.
	if err := f.gate.Check(context.Background(), "tea-org", member); err != nil {
		t.Fatalf("before invalidation or TTL = %v, want the cached allow", err)
	}
	f.clock.advance(TTL)
	if err := f.gate.Check(context.Background(), "tea-org", member); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("after the TTL = %v, want refused", err)
	}
	// Turning it off and invalidating lets the member back in at once.
	f.setOrg(false, false)
	f.gate.Invalidate("tea-org")
	f.gate.Invalidate(core.OrganizationObject(testOrg))
	if err := f.gate.Check(context.Background(), "tea-org", member); err != nil {
		t.Fatalf("after invalidation = %v, want allowed at once", err)
	}
	if err := f.gate.CheckOrganization(context.Background(), testOrg, member); err != nil {
		t.Fatalf("the org after invalidation = %v", err)
	}
}
