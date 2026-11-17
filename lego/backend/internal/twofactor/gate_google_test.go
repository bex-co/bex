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
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// gate_google_test.go pins the Google-login half of the gate
// (docs/ADR012-auth.md §9b, w7/m465): which sign-in decides for each kind of
// principal, the invited-email rule, the key exemption, the order against
// 2FA, the cache and the failure mode.

// fakeGoogle answers per subject: matching holds the identities whose linked
// Google account has their own email, humans every Kratos identity.
type fakeGoogle struct {
	mu       sync.Mutex
	matching map[string]bool
	humans   map[string]bool
	err      error
	calls    int
}

func (f *fakeGoogle) GoogleAccountMatches(_ context.Context, subject string) (bool, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	if f.err != nil {
		return false, false, f.err
	}
	return f.matching[subject], f.humans[subject], nil
}

func (f *fakeGoogle) link(subject string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.matching[subject] = true
}

type googleFixture struct {
	*fixture
	google  *fakeGoogle
	metrics *Metrics
	reg     *prometheus.Registry
}

// newGoogleFixture: tea-google requires Google login, tea-both requires it and
// 2FA. "gmail-member" linked a Google account whose email is not their bex
// email; "google-member" linked the matching one.
func newGoogleFixture() *googleFixture {
	f := newFixture()
	g := &fakeGoogle{
		matching: map[string]bool{"google-member": true, "google-enrolled": true},
		humans:   map[string]bool{"google-member": true, "google-enrolled": true, "gmail-member": true, "password-member": true},
	}
	f.settings.google["tea-google"] = true
	f.settings.google["tea-both"] = true
	f.settings.on["tea-both"] = true
	f.enrollment.humans["google-enrolled"] = true
	f.enrollment.humans["google-member"] = true
	f.enrollment.enroll("google-enrolled")
	reg := prometheus.NewRegistry()
	metrics := NewMetrics(reg)
	f.gate = New(Config{
		Settings: f.settings, Enrollment: f.enrollment, Google: g, Keys: f.keys,
		Sessions: fakeSessions{"ags-1": "tea-google"}, Now: f.clock.now, Metrics: metrics,
	})
	return &googleFixture{fixture: f, google: g, metrics: metrics, reg: reg}
}

func signedIn(subject string, how core.SignIn) core.Identity {
	return core.Identity{Subject: subject, Method: core.MethodSession, Human: true, SignIn: how}
}

func humanToken(subject string, how core.SignIn) core.Identity {
	return core.Identity{Subject: subject, Method: core.MethodOAuth2, ClientID: "bex-cli", Human: true, SignIn: how}
}

func apiKey(clientID string) core.Identity {
	return core.Identity{Subject: clientID, ClientID: clientID, Method: core.MethodOAuth2}
}

func requireCode(t *testing.T, err error, code, workspaceID string) {
	t.Helper()
	var coded *core.CodedError
	if !errors.Is(err, core.ErrForbidden) || !errors.As(err, &coded) || coded.Code != code {
		t.Fatalf("err = %v, want the coded %s 403", err, code)
	}
	if coded.Params["workspaceId"] != workspaceID {
		t.Fatalf("params = %v, want workspaceId %q", coded.Params, workspaceID)
	}
}

func TestGoogleLoginDecidesByHowEachPrincipalSignedIn(t *testing.T) {
	f := newGoogleFixture()
	f.keys.creators["key-predates"] = "password-member" // no sign-in stamp: minted before it existed
	f.keys.creators["key-password"] = "google-member"
	f.keys.signIns["key-password"] = core.SignInPassword
	f.keys.creators["key-google"] = "google-member"
	f.keys.signIns["key-google"] = core.SignInGoogle
	f.keys.creators["key-gmail"] = "gmail-member"
	f.keys.signIns["key-gmail"] = core.SignInGoogle
	f.keys.creators["key-orphan"] = "deleted-member" // Kratos no longer knows the creator
	f.keys.signIns["key-orphan"] = core.SignInGoogle
	f.keys.signIns["key-nocreator"] = core.SignInGoogle
	f.keys.keys["key-nocreator"] = true

	const member, key = core.CodeWorkspaceGoogleLoginRequired, core.CodeWorkspaceGoogleLoginRequired
	for name, tc := range map[string]struct {
		id     core.Identity
		refuse string // "" = allowed
	}{
		// A dashboard session: its own sign-in, and the account behind it.
		"session google, matching account":      {id: signedIn("google-member", core.SignInGoogle)},
		"session google, personal Gmail":        {id: signedIn("gmail-member", core.SignInGoogle), refuse: member},
		"session password":                      {id: signedIn("google-member", core.SignInPassword), refuse: member},
		"session github":                        {id: signedIn("google-member", core.SignInGitHub), refuse: member},
		"session other":                         {id: signedIn("google-member", core.SignInOther), refuse: member},
		"human token from a Google session":     {id: humanToken("google-member", core.SignInGoogle)},
		"human token from a password session":   {id: humanToken("google-member", core.SignInPassword), refuse: member},
		"human token minted before the stamp":   {id: humanToken("google-member", core.SignInUnrecorded), refuse: member},
		"API key created before the stamp":      {id: apiKey("key-predates")},
		"API key created in a password sign-in": {id: apiKey("key-password"), refuse: key},
		"API key created in a Google sign-in":   {id: apiKey("key-google")},
		"API key of a personal Gmail sign-in":   {id: apiKey("key-gmail"), refuse: key},
		"API key whose creator is gone":         {id: apiKey("key-orphan"), refuse: key},
		"API key with no recorded creator":      {id: apiKey("key-nocreator"), refuse: key},
		"platform client":                       {id: apiKey("platform-client")},
		"SSH key added before the stamp":        {id: core.Identity{Subject: "password-member", Method: core.MethodSSH}},
		"SSH key added in a password sign-in":   {id: core.Identity{Subject: "google-member", Method: core.MethodSSH, SignIn: core.SignInPassword}, refuse: key},
		"SSH key added in a Google sign-in":     {id: core.Identity{Subject: "google-member", Method: core.MethodSSH, SignIn: core.SignInGoogle}},
		"SSH key of a personal Gmail sign-in":   {id: core.Identity{Subject: "gmail-member", Method: core.MethodSSH, SignIn: core.SignInGoogle}, refuse: key},
		// No sign-in of their own: the member's account decides.
		"web shell of a matching member":       {id: core.Identity{Subject: "google-member", Method: core.MethodShell}},
		"web shell of a password-only member":  {id: core.Identity{Subject: "password-member", Method: core.MethodShell}, refuse: member},
		"agent attach of a Gmail member":       {id: core.Identity{Subject: "gmail-member", Method: core.MethodAgentAttach}, refuse: member},
		"background work of a matching member": {id: core.Identity{Subject: "google-member", Method: core.MethodSession}},
		"background work of a password member": {id: core.Identity{Subject: "password-member", Method: core.MethodSession}, refuse: member},
		"platform background work":             {id: core.Identity{Subject: "platform-worker", Method: core.MethodSession}},
	} {
		t.Run(name, func(t *testing.T) {
			err := f.gate.Check(context.Background(), "tea-google", tc.id)
			if tc.refuse == "" {
				if err != nil {
					t.Fatalf("Check = %v, want allowed", err)
				}
				return
			}
			requireCode(t, err, tc.refuse, "tea-google")
		})
	}
}

// A key refusal tells the caller to make a new key; a member's tells them to
// sign in again.
func TestGoogleLoginRefusalNamesWhatToDo(t *testing.T) {
	f := newGoogleFixture()
	f.keys.creators["key-password"] = "google-member"
	f.keys.signIns["key-password"] = core.SignInPassword
	keyErr := f.gate.Check(context.Background(), "tea-google", apiKey("key-password"))
	memberErr := f.gate.Check(context.Background(), "tea-google", signedIn("google-member", core.SignInPassword))
	if keyErr == nil || memberErr == nil || keyErr.Error() == memberErr.Error() {
		t.Fatalf("key refusal %q and member refusal %q should both exist and differ", keyErr, memberErr)
	}
}

func TestGoogleLoginComesBeforeTwoFactorAndBothApply(t *testing.T) {
	f := newGoogleFixture()
	ctx := context.Background()
	// Neither: the Google refusal first, signing in again being the first step.
	requireCode(t, f.gate.Check(ctx, "tea-both", signedIn("password-member", core.SignInPassword)), core.CodeWorkspaceGoogleLoginRequired, "tea-both")
	// Google, no second factor: now the 2FA refusal.
	requireCode(t, f.gate.Check(ctx, "tea-both", signedIn("google-member", core.SignInGoogle)), core.CodeWorkspaceTwoFactorRequired, "tea-both")
	// Both met.
	if err := f.gate.Check(ctx, "tea-both", signedIn("google-enrolled", core.SignInGoogle)); err != nil {
		t.Fatalf("a Google-signed-in, enrolled member = %v, want allowed", err)
	}
	// A 2FA-only workspace never asks about Google.
	calls := f.google.calls
	if err := f.gate.Check(ctx, "tea-on", signedIn("human-enrolled", core.SignInPassword)); err != nil {
		t.Fatalf("2FA-only workspace = %v", err)
	}
	if f.google.calls != calls {
		t.Fatal("a workspace that requires only 2FA looked up a Google account")
	}
}

func TestGoogleLoginFailsClosed(t *testing.T) {
	f := newGoogleFixture()
	f.google.err = errors.New("kratos down")
	ctx := context.Background()
	for _, id := range []core.Identity{
		signedIn("google-member", core.SignInGoogle),
		{Subject: "google-member", Method: core.MethodShell},
	} {
		if err := f.gate.Check(ctx, "tea-google", id); !errors.Is(err, core.ErrAuthzUnavailable) {
			t.Fatalf("Check with Kratos down (%s) = %v, want ErrAuthzUnavailable", id.Method, err)
		}
	}
	// No lookup wired at all is the same.
	gate := New(Config{Settings: f.settings, Now: f.clock.now})
	if err := gate.Check(ctx, "tea-google", signedIn("google-member", core.SignInGoogle)); !errors.Is(err, core.ErrAuthzUnavailable) {
		t.Fatalf("Check with no Google lookup = %v, want ErrAuthzUnavailable", err)
	}
	// A refusal that needs no lookup is still a refusal, not an outage.
	requireCode(t, f.gate.Check(ctx, "tea-google", signedIn("google-member", core.SignInPassword)), core.CodeWorkspaceGoogleLoginRequired, "tea-google")
}

// The account fact is cached per subject for TTL: linking the matching Google
// account takes effect within it, and an error is not cached.
func TestGoogleAccountIsCachedForTheTTL(t *testing.T) {
	f := newGoogleFixture()
	ctx := context.Background()
	shell := core.Identity{Subject: "password-member", Method: core.MethodShell}
	requireCode(t, f.gate.Check(ctx, "tea-google", shell), core.CodeWorkspaceGoogleLoginRequired, "tea-google")
	f.google.link("password-member")
	requireCode(t, f.gate.Check(ctx, "tea-google", shell), core.CodeWorkspaceGoogleLoginRequired, "tea-google")
	if f.google.calls != 1 {
		t.Fatalf("Google lookups within the TTL = %d, want 1", f.google.calls)
	}
	f.clock.advance(TTL + time.Second)
	if err := f.gate.Check(ctx, "tea-google", shell); err != nil {
		t.Fatalf("after linking and the TTL = %v, want allowed", err)
	}
}

func TestSignedInWithGoogleIsTheGatesOwnCheck(t *testing.T) {
	f := newGoogleFixture()
	ctx := context.Background()
	for id, want := range map[core.Identity]bool{
		signedIn("google-member", core.SignInGoogle):   true,
		signedIn("gmail-member", core.SignInGoogle):    false,
		signedIn("google-member", core.SignInPassword): false,
	} {
		if got, err := f.gate.SignedInWithGoogle(ctx, id); err != nil || got != want {
			t.Fatalf("SignedInWithGoogle(%+v) = (%v, %v), want %v", id, got, err, want)
		}
	}
	f.clock.advance(TTL + time.Second)
	f.google.err = errors.New("kratos down")
	if _, err := f.gate.SignedInWithGoogle(ctx, signedIn("google-member", core.SignInGoogle)); err == nil {
		t.Fatal("SignedInWithGoogle with Kratos down = nil error, want the outage")
	}
}

func TestGoogleRefusalsAreCountedByClosedReason(t *testing.T) {
	f := newGoogleFixture()
	f.keys.creators["key-password"] = "google-member"
	f.keys.signIns["key-password"] = core.SignInPassword
	ctx := context.Background()
	_ = f.gate.Check(ctx, "tea-google", signedIn("google-member", core.SignInPassword))
	_ = f.gate.Check(ctx, "tea-google", apiKey("key-password"))
	families, err := f.reg.Gather()
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]float64{}
	for _, fam := range families {
		for _, m := range fam.GetMetric() {
			for _, l := range m.GetLabel() {
				if l.GetName() == "reason" {
					got[l.GetValue()] = m.GetCounter().GetValue()
				}
			}
		}
	}
	if got["not_google"] != 1 || got["key_not_google"] != 1 {
		t.Fatalf("refusals = %v, want not_google=1 key_not_google=1", got)
	}
}
