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

import "context"

// SignIn is how the person behind a credential signed in when the credential
// was obtained (w7/m465, docs/ADR012-auth.md §9b): the Kratos session's own
// sign-in, the browser session a human OAuth token was consented from, or the
// session an API key or SSH key was created in. It is a closed vocabulary:
// every producer goes through ParseSignIn or SignInFromKratos, so a value read
// back from a token, a key's metadata or a request is never free text.
type SignIn string

const (
	// SignInUnrecorded means the credential carries no sign-in of its own:
	// background work acting as a member, a gateway ticket, or a key or token
	// minted before the stamp existed. What it means for the Google-login
	// requirement depends on the principal (twofactor.Gate).
	SignInUnrecorded SignIn = ""
	// SignInGoogle is a sign-in with the Kratos oidc provider "google".
	SignInGoogle SignIn = "oidc:google"
	// SignInGitHub is a sign-in with the Kratos oidc provider "github".
	SignInGitHub SignIn = "oidc:github"
	// SignInPassword is an email and password sign-in.
	SignInPassword SignIn = "password"
	// SignInOther is any other recorded sign-in: another oidc provider, a
	// one-time code, a passkey. It never satisfies the Google-login requirement.
	SignInOther SignIn = "other"
)

// ParseSignIn maps a stored or transported value back into the vocabulary.
// "" stays SignInUnrecorded; anything that is not one of the known values is
// SignInOther, so a malformed stamp can never read as Google.
func ParseSignIn(s string) SignIn {
	switch v := SignIn(s); v {
	case SignInUnrecorded, SignInGoogle, SignInGitHub, SignInPassword, SignInOther:
		return v
	}
	return SignInOther
}

// KratosAuthMethod is one entry of a Kratos session's authentication_methods:
// method is Kratos's name for it (password, oidc, totp, webauthn, code,
// lookup_secret, passkey), provider the oidc provider id.
type KratosAuthMethod struct {
	Method   string `json:"method"`
	Provider string `json:"provider,omitempty"`
}

// SignInFromKratos derives a session's sign-in from its authentication
// methods. A session that used Google at any point (its first factor, or a
// later re-authentication) signed in with Google; a second factor on top
// (totp, webauthn, lookup_secret) does not change the answer. A session with
// no recognized first factor is SignInOther, never unrecorded: a session
// always has a sign-in.
func SignInFromKratos(methods []KratosAuthMethod) SignIn {
	best := SignInOther
	for _, m := range methods {
		var got SignIn
		switch {
		case m.Method == "oidc" && m.Provider == "google":
			return SignInGoogle
		case m.Method == "oidc" && m.Provider == "github":
			got = SignInGitHub
		case m.Method == "password":
			got = SignInPassword
		default:
			continue
		}
		if best == SignInOther {
			best = got
		}
	}
	return best
}

// KeySignInStamp is what an API key or SSH key created by this request is
// stamped with: the caller's sign-in, or SignInOther when the caller has none
// recorded (a human token minted before the stamp existed). Only a key that
// predates the stamp may read as unrecorded, which the Google-login
// requirement exempts, so a new key must never be stamped that way.
func KeySignInStamp(ctx context.Context) SignIn {
	id, _ := IdentityFrom(ctx)
	if id.SignIn == SignInUnrecorded {
		return SignInOther
	}
	return id.SignIn
}
