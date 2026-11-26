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
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

func TestKratosEmailVerification(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/admin/identities/verified":
			_, _ = w.Write([]byte(`{"traits":{"email":"Ada@Example.com"},"verifiable_addresses":[{"value":"ada@example.com","verified":true}]}`))
		case "/admin/identities/other-address":
			_, _ = w.Write([]byte(`{"traits":{"email":"ada@example.com"},"verifiable_addresses":[{"value":"ada@example.com","verified":false},{"value":"old@example.com","verified":true}]}`))
		case "/admin/identities/broken":
			http.Error(w, "boom", http.StatusInternalServerError)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	k := NewKratosIdentities(srv.URL)
	for subject, want := range map[string]struct{ human, verified, err bool }{
		"verified":      {human: true, verified: true},
		"other-address": {human: true},
		"bex-bootstrap": {},
		"broken":        {err: true},
	} {
		human, verified, err := k.EmailVerification(context.Background(), subject)
		if (err != nil) != want.err || human != want.human || verified != want.verified {
			t.Errorf("%s: got human=%v verified=%v err=%v, want %+v", subject, human, verified, err, want)
		}
	}
}

// TestKratosMFAEnrolled pins the enrollment fact the workspace 2FA gate reads
// (w22/m402, ADR012 §9a): the same rule as the member badge (a TOTP credential,
// or a WebAuthn one that carries a registered key), a Kratos 404 or a subject
// that is not a UUID is "not human" (a platform principal), and an upstream
// failure is an error rather than a quiet "not enrolled".
func TestKratosMFAEnrolled(t *testing.T) {
	const (
		totp      = "11111111-1111-4111-8111-111111111111"
		passkey   = "22222222-2222-4222-8222-222222222222"
		stub      = "33333333-3333-4333-8333-333333333333"
		plain     = "44444444-4444-4444-8444-444444444444"
		missing   = "55555555-5555-4555-8555-555555555555"
		broken    = "66666666-6666-4666-8666-666666666666"
		malformed = "77777777-7777-4777-8777-777777777777"
	)
	var asked []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked = append(asked, r.URL.Path)
		// Only webauthn's config is needed; asking for TOTP's would load its secret.
		if got := r.URL.Query()["include_credential"]; len(got) != 1 || got[0] != "webauthn" {
			t.Errorf("%s includes %v, want only webauthn", r.URL, got)
		}
		switch r.URL.Path {
		case "/admin/identities/" + totp:
			_, _ = w.Write([]byte(`{"credentials":{"password":{"type":"password"},"totp":{"type":"totp"}}}`))
		case "/admin/identities/" + passkey:
			_, _ = w.Write([]byte(`{"credentials":{"webauthn":{"type":"webauthn","config":{"credentials":[{"id":"a2V5"}]}}}}`))
		case "/admin/identities/" + stub:
			_, _ = w.Write([]byte(`{"credentials":{"password":{"type":"password"},"webauthn":{"type":"webauthn","config":{"user_handle":"dXNlcg=="}}}}`))
		case "/admin/identities/" + plain:
			_, _ = w.Write([]byte(`{"credentials":{"password":{"type":"password"}}}`))
		case "/admin/identities/" + broken:
			http.Error(w, "boom", http.StatusInternalServerError)
		case "/admin/identities/" + malformed:
			_, _ = w.Write([]byte(`{not json`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	k := NewKratosIdentities(srv.URL)
	for name, tc := range map[string]struct {
		subject                string
		enrolled, human, isErr bool
	}{
		"totp enrolled":                 {subject: totp, enrolled: true, human: true},
		"passkey with a registered key": {subject: passkey, enrolled: true, human: true},
		"the stub webauthn entry":       {subject: stub, human: true},
		"password only":                 {subject: plain, human: true},
		"unknown to Kratos":             {subject: missing},
		"upstream failure":              {subject: broken, isErr: true},
		"undecodable answer":            {subject: malformed, isErr: true},
	} {
		enrolled, human, err := k.MFAEnrolled(context.Background(), tc.subject)
		if (err != nil) != tc.isErr || enrolled != tc.enrolled || human != tc.human {
			t.Errorf("%s: got enrolled=%v human=%v err=%v, want enrolled=%v human=%v err=%v", name, enrolled, human, err, tc.enrolled, tc.human, tc.isErr)
		}
	}
	// A named platform client is never sent to Kratos: not a UUID, not human.
	asked = nil
	if enrolled, human, err := k.MFAEnrolled(context.Background(), "bex-bootstrap"); enrolled || human || err != nil {
		t.Errorf("non-UUID subject = (%v, %v, %v), want (false, false, nil)", enrolled, human, err)
	}
	if len(asked) != 0 {
		t.Errorf("a non-UUID subject reached Kratos: %v", asked)
	}
}

// idToken is an unsigned JWT carrying claims, the shape of the
// initial_id_token Kratos stores beside a linked oidc account.
func idToken(t *testing.T, claims map[string]any) string {
	t.Helper()
	payload, err := json.Marshal(claims)
	if err != nil {
		t.Fatal(err)
	}
	return "eyJhbGciOiJSUzI1NiJ9." + base64.RawURLEncoding.EncodeToString(payload) + ".c2ln"
}

// w7/m465 (docs/ADR012-auth.md §9b): a Google sign-in counts only with the
// Google account of the member's own email, Render's "the Google account that
// matches the invited email address". The email is read from the ID token
// Kratos stored when the account was linked.
func TestKratosGoogleAccountMatches(t *testing.T) {
	const (
		matching   = "11111111-1111-4111-8111-111111111111"
		gmail      = "22222222-2222-4222-8222-222222222222"
		github     = "33333333-3333-4333-8333-333333333333"
		unverified = "44444444-4444-4444-8444-444444444444"
		password   = "55555555-5555-4555-8555-555555555555"
		missing    = "66666666-6666-4666-8666-666666666666"
		broken     = "77777777-7777-4777-8777-777777777777"
		garbled    = "88888888-8888-4888-8888-888888888888"
	)
	provider := func(name string, claims map[string]any) map[string]any {
		return map[string]any{"provider": name, "subject": "sub", "initial_id_token": idToken(t, claims)}
	}
	identity := func(email string, providers ...map[string]any) []byte {
		raw, _ := json.Marshal(map[string]any{
			"traits":      map[string]any{"email": email},
			"credentials": map[string]any{"oidc": map[string]any{"type": "oidc", "config": map[string]any{"providers": providers}}},
		})
		return raw
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query()["include_credential"]; len(got) != 1 || got[0] != "oidc" {
			t.Errorf("%s includes %v, want only oidc", r.URL, got)
		}
		switch r.URL.Path {
		case "/admin/identities/" + matching:
			_, _ = w.Write(identity("Person@Acme.example", provider("github", map[string]any{"email": "person@acme.example"}), provider("google", map[string]any{"email": "person@acme.example", "email_verified": true})))
		case "/admin/identities/" + gmail:
			_, _ = w.Write(identity("person@acme.example", provider("google", map[string]any{"email": "person@gmail.com", "email_verified": true})))
		case "/admin/identities/" + github:
			_, _ = w.Write(identity("person@acme.example", provider("github", map[string]any{"email": "person@acme.example"})))
		case "/admin/identities/" + unverified:
			_, _ = w.Write(identity("person@acme.example", provider("google", map[string]any{"email": "person@acme.example", "email_verified": false})))
		case "/admin/identities/" + password:
			_, _ = w.Write([]byte(`{"traits":{"email":"person@acme.example"},"credentials":{"password":{"type":"password"}}}`))
		case "/admin/identities/" + garbled:
			_, _ = w.Write(identity("person@acme.example", map[string]any{"provider": "google", "initial_id_token": "not.a-jwt"}))
		case "/admin/identities/" + broken:
			http.Error(w, "boom", http.StatusInternalServerError)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	k := NewKratosIdentities(srv.URL)
	for name, tc := range map[string]struct {
		subject                 string
		matches, human, wantErr bool
	}{
		"the Google account of the member's email": {subject: matching, matches: true, human: true},
		"a personal Gmail account":                 {subject: gmail, human: true},
		"GitHub only":                              {subject: github, human: true},
		"Google says the email is unverified":      {subject: unverified, human: true},
		"password only":                            {subject: password, human: true},
		"a token that is not a JWT":                {subject: garbled, human: true},
		"not a Kratos identity":                    {subject: missing},
		"not a UUID":                               {subject: "bex-bootstrap"},
		"Kratos down":                              {subject: broken, wantErr: true},
	} {
		t.Run(name, func(t *testing.T) {
			matches, human, err := k.GoogleAccountMatches(context.Background(), tc.subject)
			if (err != nil) != tc.wantErr || matches != tc.matches || human != tc.human {
				t.Fatalf("GoogleAccountMatches = (%v, %v, %v), want (%v, %v, err=%v)", matches, human, err, tc.matches, tc.human, tc.wantErr)
			}
		})
	}
}

// The settings guard reads the stored email and whether Google is linked from
// the oidc identifiers, without asking Kratos for the credential's tokens.
func TestKratosSettingsFacts(t *testing.T) {
	const linked, unlinked = "11111111-1111-4111-8111-111111111111", "22222222-2222-4222-8222-222222222222"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.RawQuery != "" {
			t.Errorf("%s asks for %q, want no credential included", r.URL.Path, r.URL.RawQuery)
		}
		switch r.URL.Path {
		case "/admin/identities/" + linked:
			_, _ = w.Write([]byte(`{"traits":{"email":"person@acme.example"},"credentials":{"oidc":{"type":"oidc","identifiers":["github:1","google:abc"]}}}`))
		case "/admin/identities/" + unlinked:
			_, _ = w.Write([]byte(`{"traits":{"email":"other@acme.example"},"credentials":{"oidc":{"type":"oidc","identifiers":["github:1"]}}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	k := NewKratosIdentities(srv.URL)
	if email, google, found, err := k.SettingsFacts(context.Background(), linked); err != nil || !found || !google || email != "person@acme.example" {
		t.Fatalf("linked = (%q, %v, %v, %v)", email, google, found, err)
	}
	if email, google, found, err := k.SettingsFacts(context.Background(), unlinked); err != nil || !found || google || email != "other@acme.example" {
		t.Fatalf("unlinked = (%q, %v, %v, %v)", email, google, found, err)
	}
	if _, _, found, err := k.SettingsFacts(context.Background(), "33333333-3333-4333-8333-333333333333"); err != nil || found {
		t.Fatalf("missing = (found %v, %v), want not found", found, err)
	}
}

// TestKratosLoginStatus pins what an Org Owner's View details reads for a
// member (w1/m657, Render's orgUserAuthStatus): a password credential with an
// identifier, each linked oidc provider from its "<provider>:<subject>"
// identifier, and a second factor by the 2FA gate's rule, from one admin read
// that loads no TOTP secret. A Kratos 404 is "not human" and an upstream
// failure an error, never an account with nothing to sign in with.
func TestKratosLoginStatus(t *testing.T) {
	const (
		everything = "11111111-1111-4111-8111-111111111111"
		oauthOnly  = "22222222-2222-4222-8222-222222222222"
		missing    = "33333333-3333-4333-8333-333333333333"
		broken     = "44444444-4444-4444-8444-444444444444"
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query()["include_credential"]; len(got) != 1 || got[0] != "webauthn" {
			t.Errorf("%s includes %v, want only webauthn", r.URL, got)
		}
		switch r.URL.Path {
		case "/admin/identities/" + everything:
			_, _ = w.Write([]byte(`{"credentials":{
				"password":{"type":"password","identifiers":["ada@example.com"]},
				"oidc":{"type":"oidc","identifiers":["google:1093","github:4242"]},
				"totp":{"type":"totp"}}}`))
		case "/admin/identities/" + oauthOnly:
			// A webauthn stub with no key is not a second factor, and a
			// password credential with no identifier is no password.
			_, _ = w.Write([]byte(`{"credentials":{
				"password":{"type":"password","identifiers":[]},
				"oidc":{"type":"oidc","identifiers":["github:7"]},
				"webauthn":{"type":"webauthn","config":{"credentials":[]}}}}`))
		case "/admin/identities/" + broken:
			http.Error(w, "boom", http.StatusBadGateway)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	k := NewKratosIdentities(srv.URL)
	for subject, want := range map[string]struct {
		status     core.LoginStatus
		human, err bool
	}{
		everything:      {status: core.LoginStatus{Password: true, Google: true, GitHub: true, TwoFactor: true}, human: true},
		oauthOnly:       {status: core.LoginStatus{GitHub: true}, human: true},
		missing:         {},
		"bex-bootstrap": {},
		broken:          {err: true},
	} {
		status, human, err := k.LoginStatus(context.Background(), subject)
		if status != want.status || human != want.human || (err != nil) != want.err {
			t.Errorf("%s: %+v human=%v err=%v, want %+v", subject, status, human, err, want)
		}
	}
}
