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
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/workspaces"
)

// TestAnEmailLookupLoadsNoCredentials (w5/145): every email-only reader goes
// through the adapter's batched read, Kratos' list endpoint with no
// credential included, where the per-identity endpoint always lists
// credentials. An unknown or malformed subject and a failure still miss.
func TestAnEmailLookupLoadsNoCredentials(t *testing.T) {
	const alice, bob = "6f1d2c3b-4a5e-4f60-8a1b-2c3d4e5f6a7b", "0d9e8f7a-6b5c-4d3e-9f2a-1b0c9d8e7f6a"
	var requests []string
	fail := false
	kratos := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.URL.RequestURI())
		if fail {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path != "/admin/identities" { // the per-identity read
			_, _ = w.Write([]byte(`{"id":"` + alice + `","traits":{"email":"alice@example.com"}}`))
			return
		}
		_, _ = w.Write([]byte(`[{"id":"` + alice + `","traits":{"email":"alice@example.com"}},{"id":"` + bob + `","traits":{"email":"bob@example.com"}}]`))
	}))
	defer kratos.Close()
	lookup := identityEmailLookup{Identities: workspaces.NewKratosIdentities(kratos.URL)}
	ctx := context.Background()

	got := lookup.LookupEmails(ctx, []string{alice, bob, "bex-bootstrap"})
	if got[alice] != "alice@example.com" || got[bob] != "bob@example.com" || len(got) != 2 {
		t.Fatalf("LookupEmails = %v, want alice's and bob's addresses only", got)
	}
	if len(requests) != 1 || !strings.HasPrefix(requests[0], "/admin/identities?") || strings.Contains(requests[0], "include_credential") {
		t.Fatalf("Kratos saw %v, want one list read with no credential included", requests)
	}
	fail = true
	if got := lookup.LookupEmails(ctx, []string{alice}); len(got) != 0 {
		t.Errorf("a Kratos failure resolved %v, want a miss", got)
	}
}
