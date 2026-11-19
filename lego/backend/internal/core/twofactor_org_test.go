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

package core

import (
	"context"
	"errors"
	"testing"
)

// w7/m466 (docs/ADR012-auth.md §9c): an org object's positive decision is
// handed to the gate's org capability with the org's id, never to Check as a
// workspace; a gate without the capability (the SSH gateway's) does not
// enforce org objects; the org refusal names the org.

type orgRecordingGate struct {
	recordingGate
	orgCalls []string
	refuse   bool
}

func (g *orgRecordingGate) CheckOrganization(_ context.Context, orgID string, id Identity) error {
	g.orgCalls = append(g.orgCalls, orgID+"|"+id.Subject)
	if g.refuse {
		return NewLoginTwoFactorRequiredError("", LoginRequirements{}.WithOrganization(orgID, "Acme", true, false))
	}
	return nil
}

func TestAnOrgObjectIsCheckedAgainstTheOrgsOwnRequirement(t *testing.T) {
	ctx := WithIdentity(context.Background(), Identity{Subject: "member", Method: "session"})
	gate := &orgRecordingGate{refuse: true}
	b := &Base{TwoFactor: gate}
	err := b.AuthorizeOn(ctx, RelCanView, OrganizationObject("org-1"))
	var coded *CodedError
	if !errors.Is(err, ErrForbidden) || !errors.As(err, &coded) || coded.Code != CodeWorkspaceTwoFactorRequired ||
		coded.Params["organizationId"] != "org-1" || coded.Params["organizationName"] != "Acme" {
		t.Fatalf("AuthorizeOn(org) = %v, want the 2FA refusal naming the org", err)
	}
	if _, named := coded.Params["workspaceId"]; named {
		t.Fatalf("params = %v, want no workspace", coded.Params)
	}
	if len(gate.orgCalls) != 1 || gate.orgCalls[0] != "org-1|member" || len(gate.seen()) != 0 {
		t.Fatalf("org calls %v, workspace calls %v: want one org check and no workspace check", gate.orgCalls, gate.seen())
	}
	if !IsLoginRefusal(err) || IsLoginRefusal(ErrForbidden) || IsLoginRefusal(NewOrganizationNotFoundError("org-1")) {
		t.Fatal("IsLoginRefusal must recognize the gate's refusal and nothing else")
	}
	// An exempt read skips it.
	if err := b.AuthorizeOn(WithTwoFactorExempt(ctx), RelCanView, OrganizationObject("org-1")); err != nil {
		t.Fatalf("an exempt org read = %v", err)
	}
	// A gate without the capability does not enforce org objects.
	plain := &recordingGate{refuse: map[string]bool{"org-1": true}}
	if err := (&Base{TwoFactor: plain}).AuthorizeOn(ctx, RelCanView, OrganizationObject("org-1")); err != nil || len(plain.seen()) != 0 {
		t.Fatalf("a gate without the org capability = %v, calls %v", err, plain.seen())
	}
}

// A workspace refusal names the org only when the org's own setting requires
// what was refused.
func TestTheLoginRefusalsNameTheOrgOnlyWhenItRequiresIt(t *testing.T) {
	own := LoginRequirements{TwoFactor: true, GoogleLogin: true}
	if p := NewLoginTwoFactorRequiredError("tea-1", own).Params; p["organizationId"] != nil || p["workspaceId"] != "tea-1" {
		t.Fatalf("a workspace's own 2FA = %v", p)
	}
	orgGoogle := LoginRequirements{TwoFactor: true}.WithOrganization("org-1", "Acme", false, true)
	if p := NewLoginTwoFactorRequiredError("tea-1", orgGoogle).Params; p["organizationId"] != nil {
		t.Fatalf("2FA refused where the org requires only Google = %v, want the workspace alone", p)
	}
	for _, key := range []bool{false, true} {
		err := NewLoginGoogleLoginRequiredError("tea-1", orgGoogle, key)
		if err.Code != CodeWorkspaceGoogleLoginRequired || err.Params["organizationId"] != "org-1" || err.Params["workspaceId"] != "tea-1" {
			t.Fatalf("Google refused (key=%v) where the org requires it = %v %v", key, err, err.Params)
		}
	}
	if r := (LoginRequirements{TwoFactor: true}).WithOrganization("org-1", "Acme", false, false); r != (LoginRequirements{TwoFactor: true}) {
		t.Fatalf("an org requiring nothing changed the requirements: %+v", r)
	}
}

// w7/090: a login refusal survives the trip to the SSH gateway as a
// LoginRefusal. Read back into an error it is the refusal bex-api answered,
// naming the org exactly when the original did; anything else is no refusal.
func TestALoginRefusalRoundTripsThroughLoginRefusal(t *testing.T) {
	org := LoginRequirements{}.WithOrganization("org-1", "Acme", true, true)
	for name, original := range map[string]*CodedError{
		"workspace 2FA":    NewWorkspaceTwoFactorRequiredError("tea-1"),
		"workspace Google": NewWorkspaceGoogleLoginRequiredError("tea-1"),
		"org 2FA":          NewLoginTwoFactorRequiredError("tea-1", org),
		"org Google":       NewLoginGoogleLoginRequiredError("tea-1", org, false),
	} {
		t.Run(name, func(t *testing.T) {
			r, ok := LoginRefusalOf(errors.Join(errors.New("resolve"), original))
			if !ok || r.Code != original.Code {
				t.Fatalf("LoginRefusalOf = (%+v, %v), want the code %s", r, ok, original.Code)
			}
			var back *CodedError
			if err := r.Err("tea-1"); !errors.Is(err, ErrForbidden) || !errors.As(err, &back) {
				t.Fatalf("Err = %v, want a coded 403", err)
			}
			if back.Code != original.Code || back.Error() != original.Error() ||
				back.Params["organizationId"] != original.Params["organizationId"] || back.Params["organizationName"] != original.Params["organizationName"] ||
				back.Params["workspaceId"] != "tea-1" {
				t.Fatalf("Err = %v %v, want %v %v", back, back.Params, original, original.Params)
			}
		})
	}
	for _, err := range []error{ErrForbidden, NewEmailVerificationRequiredError(), errors.New(CodeWorkspaceTwoFactorRequired)} {
		if r, ok := LoginRefusalOf(err); ok || r != (LoginRefusal{}) {
			t.Fatalf("LoginRefusalOf(%v) = (%+v, %v), want no refusal", err, r, ok)
		}
	}
	if err := (LoginRefusal{}).Err("tea-1"); err != nil {
		t.Fatalf("the zero LoginRefusal = %v, want allowed", err)
	}
}
