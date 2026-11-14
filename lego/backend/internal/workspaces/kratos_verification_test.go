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
	"net/http"
	"net/http/httptest"
	"testing"
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
