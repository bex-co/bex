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
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// kratos_test.go covers the KratosIdentities admin-API reader: trait parsing
// (email + the optional w4/m25 name trait), MFA derivation, and the
// honest-omit contract on misses.

func TestKratosIdentitiesLookup(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/admin/identities/named":
			fmt.Fprint(w, `{"traits":{"email":"a@example.com","name":"Ada Lovelace"},"credentials":{"totp":{"type":"totp"}}}`)
		case "/admin/identities/legacy":
			// An identity minted before the name trait existed.
			fmt.Fprint(w, `{"traits":{"email":"b@example.com"}}`)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	k := NewKratosIdentities(srv.URL)

	named, ok := k.Lookup(context.Background(), "named")
	if !ok || named.Email != "a@example.com" || named.Name != "Ada Lovelace" || !named.MFAEnabled {
		t.Fatalf("named = %+v, %v", named, ok)
	}

	legacy, ok := k.Lookup(context.Background(), "legacy")
	if !ok || legacy.Email != "b@example.com" || legacy.Name != "" || legacy.MFAEnabled {
		t.Fatalf("legacy = %+v, %v", legacy, ok)
	}

	if attrs, ok := k.Lookup(context.Background(), "missing"); ok {
		t.Fatalf("missing identity: want ok=false, got %+v", attrs)
	}
}

// TestKratosIdentitiesMFADerivation pins mfaEnabled per credential shape
// (w4/020): Kratos mints a stub webauthn entry (config.user_handle only, no
// registered keys) at password registration, so webauthn counts only when
// config.credentials is non-empty; totp stays presence-based.
func TestKratosIdentitiesMFADerivation(t *testing.T) {
	shapes := map[string]struct {
		credentials string
		want        bool
	}{
		"password-only":       {`{}`, false},
		"webauthn-stub":       {`{"webauthn":{"type":"webauthn","config":{"user_handle":"dXNlcg=="}}}`, false},
		"webauthn-empty-list": {`{"webauthn":{"type":"webauthn","config":{"credentials":[]}}}`, false},
		"webauthn-enrolled":   {`{"webauthn":{"type":"webauthn","config":{"credentials":[{"id":"a2V5","display_name":"key"}]}}}`, true},
		"totp":                {`{"totp":{"type":"totp"}}`, true},
		"totp-plus-stub":      {`{"totp":{"type":"totp"},"webauthn":{"type":"webauthn","config":{"user_handle":"dXNlcg=="}}}`, true},
		"totp-plus-enrolled":  {`{"totp":{"type":"totp"},"webauthn":{"type":"webauthn","config":{"credentials":[{"id":"a2V5"}]}}}`, true},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		shape, ok := shapes[r.URL.Path[len("/admin/identities/"):]]
		if !ok {
			http.NotFound(w, r)
			return
		}
		fmt.Fprintf(w, `{"traits":{"email":"c@example.com"},"credentials":%s}`, shape.credentials)
	}))
	t.Cleanup(srv.Close)
	k := NewKratosIdentities(srv.URL)

	for name, shape := range shapes {
		attrs, ok := k.Lookup(context.Background(), name)
		if !ok || attrs.MFAEnabled != shape.want {
			t.Errorf("%s: mfaEnabled = %v (ok=%v), want %v", name, attrs.MFAEnabled, ok, shape.want)
		}
	}
}
