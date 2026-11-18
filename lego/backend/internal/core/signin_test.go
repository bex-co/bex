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
	"testing"
)

// w7/m465 (docs/ADR012-auth.md §9b): the sign-in vocabulary is closed, so a
// stored or transported value can never read as Google unless it is exactly
// Google.
func TestParseSignInIsClosed(t *testing.T) {
	for in, want := range map[string]SignIn{
		"":             SignInUnrecorded,
		"oidc:google":  SignInGoogle,
		"oidc:github":  SignInGitHub,
		"password":     SignInPassword,
		"other":        SignInOther,
		"OIDC:GOOGLE":  SignInOther,
		"oidc:google ": SignInOther,
		"google":       SignInOther,
		"oidc:gitlab":  SignInOther,
		// w7/m468: GitLab and Bitbucket are sign-ins, but never Google's.
		"oidc:bitbucket": SignInOther,
	} {
		if got := ParseSignIn(in); got != want {
			t.Errorf("ParseSignIn(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSignInFromKratosPrefersGoogleAndNeverLeavesASessionUnrecorded(t *testing.T) {
	m := func(method, provider string) KratosAuthMethod {
		return KratosAuthMethod{Method: method, Provider: provider}
	}
	for name, tc := range map[string]struct {
		methods []KratosAuthMethod
		want    SignIn
	}{
		"google":                            {[]KratosAuthMethod{m("oidc", "google")}, SignInGoogle},
		"google then a second factor":       {[]KratosAuthMethod{m("oidc", "google"), m("totp", "")}, SignInGoogle},
		"password, then google refreshed":   {[]KratosAuthMethod{m("password", ""), m("oidc", "google")}, SignInGoogle},
		"password":                          {[]KratosAuthMethod{m("password", ""), m("webauthn", "")}, SignInPassword},
		"github":                            {[]KratosAuthMethod{m("oidc", "github")}, SignInGitHub},
		"github then password":              {[]KratosAuthMethod{m("oidc", "github"), m("password", "")}, SignInGitHub},
		"another oidc provider":             {[]KratosAuthMethod{m("oidc", "gitlab")}, SignInOther},
		"bitbucket, through Dex (w7/m468)":  {[]KratosAuthMethod{m("oidc", "bitbucket")}, SignInOther},
		"gitlab, then google refreshed":     {[]KratosAuthMethod{m("oidc", "gitlab"), m("oidc", "google")}, SignInGoogle},
		"a one-time code":                   {[]KratosAuthMethod{m("code", "")}, SignInOther},
		"a provider named google, not oidc": {[]KratosAuthMethod{m("password", "google")}, SignInPassword},
		"nothing recorded":                  {nil, SignInOther},
	} {
		if got := SignInFromKratos(tc.methods); got != tc.want {
			t.Errorf("%s: SignInFromKratos = %q, want %q", name, got, tc.want)
		}
	}
}

// A key created now is stamped with how its creator signed in; a creator with
// no recorded sign-in stamps "other", because only a key that predates the
// stamp may read as unrecorded (which the Google-login requirement exempts).
func TestKeySignInStampIsNeverUnrecorded(t *testing.T) {
	for in, want := range map[SignIn]SignIn{
		SignInGoogle:     SignInGoogle,
		SignInPassword:   SignInPassword,
		SignInUnrecorded: SignInOther,
	} {
		ctx := WithIdentity(context.Background(), Identity{Subject: "member", Method: MethodOAuth2, Human: true, SignIn: in})
		if got := KeySignInStamp(ctx); got != want {
			t.Errorf("KeySignInStamp(%q) = %q, want %q", in, got, want)
		}
	}
	if got := KeySignInStamp(context.Background()); got != SignInOther {
		t.Errorf("KeySignInStamp with no caller = %q, want %q", got, SignInOther)
	}
}
