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
