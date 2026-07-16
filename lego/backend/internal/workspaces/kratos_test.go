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
