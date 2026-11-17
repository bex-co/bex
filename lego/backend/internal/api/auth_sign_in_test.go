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
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// w7/m465 (docs/ADR012-auth.md §9b): every caller carries how it signed in.
// A Kratos session reads it from its own authentication methods; a human OAuth
// token from the bex_sign_in claim the consent acceptor stamped; a machine API
// key carries none on the identity (its stamp is on the Hydra client).

func signInOf(t *testing.T, gate *oryAuth, header, value string) core.SignIn {
	t.Helper()
	r := httptest.NewRequest(http.MethodGet, "/probe", nil)
	r.Header.Set(header, value)
	w := httptest.NewRecorder()
	var got core.SignIn
	var seen bool
	gate.middleware(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		id, _ := core.IdentityFrom(r.Context())
		got, seen = id.SignIn, true
	})).ServeHTTP(w, r)
	if !seen {
		t.Fatalf("%s %q was not admitted: %d %s", header, value, w.Code, w.Body.String())
	}
	return got
}

func TestASessionCarriesItsOwnSignIn(t *testing.T) {
	methods := map[string]string{
		"google":        `[{"method":"oidc","provider":"google","aal":"aal1"}]`,
		"google+totp":   `[{"method":"oidc","provider":"google","aal":"aal1"},{"method":"totp","aal":"aal2"}]`,
		"password":      `[{"method":"password","aal":"aal1"}]`,
		"github":        `[{"method":"oidc","provider":"github","aal":"aal1"}]`,
		"code":          `[{"method":"code","aal":"aal1"}]`,
		"none recorded": `[]`,
	}
	kratos := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"id":"ses-1","authentication_methods":%s,"identity":{"id":"identity-1","traits":{"email":"ada@example.com"}}}`,
			methods[r.Header.Get("X-Session-Token")])
	}))
	t.Cleanup(kratos.Close)
	s := &Server{HydraAdminURL: fakeHydraURL(t), KratosURL: kratos.URL}
	gate, err := s.newAuthGate()
	if err != nil {
		t.Fatal(err)
	}
	for session, want := range map[string]core.SignIn{
		"google":        core.SignInGoogle,
		"google+totp":   core.SignInGoogle,
		"password":      core.SignInPassword,
		"github":        core.SignInGitHub,
		"code":          core.SignInOther,
		"none recorded": core.SignInOther, // a session always has a sign-in
	} {
		if got := signInOf(t, gate, "X-Session-Token", session); got != want {
			t.Errorf("session %s signed in as %q, want %q", session, got, want)
		}
	}
}

func TestAHumanTokenCarriesTheConsentingSessionsSignInAndAKeyNone(t *testing.T) {
	tokens := map[string]string{
		"human-google":  `{"active":true,"sub":"identity-1","client_id":"bex-cli","scope":"bex.read","ext":{"bex_sign_in":"oidc:google"}}`,
		"human-old":     `{"active":true,"sub":"identity-1","client_id":"bex-cli","scope":"bex.read"}`,
		"human-garbled": `{"active":true,"sub":"identity-1","client_id":"bex-cli","scope":"bex.read","ext":{"bex_sign_in":"Google"}}`,
		// A machine token's ext is never read: an API key's sign-in is the
		// stamp on its Hydra client, which only bex-api writes.
		"machine": `{"active":true,"sub":"key-1","client_id":"key-1","ext":{"bex_sign_in":"oidc:google"}}`,
	}
	hydra := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		w.Header().Set("Content-Type", "application/json")
		body, ok := tokens[r.PostFormValue("token")]
		if !ok {
			body = `{"active":false}`
		}
		_, _ = fmt.Fprint(w, body)
	}))
	t.Cleanup(hydra.Close)
	s := &Server{HydraAdminURL: hydra.URL, KratosURL: fakeKratos(t).URL}
	gate, err := s.newAuthGate()
	if err != nil {
		t.Fatal(err)
	}
	for token, want := range map[string]core.SignIn{
		"human-google":  core.SignInGoogle,
		"human-old":     core.SignInUnrecorded,
		"human-garbled": core.SignInOther,
		"machine":       core.SignInUnrecorded,
	} {
		if got := signInOf(t, gate, "Authorization", "Bearer "+token); got != want {
			t.Errorf("token %s signed in as %q, want %q", token, got, want)
		}
	}
}
