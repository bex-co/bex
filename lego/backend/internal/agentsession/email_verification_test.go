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

package agentsession

import (
	"context"
	"errors"
	"net/http/httptest"
	"testing"
)

func TestEmailVerificationRoundTrip(t *testing.T) {
	secret := []byte("exec-secret")
	answers := map[string]struct {
		human, verified bool
		err             error
	}{
		"verified-human":   {human: true, verified: true},
		"unverified-human": {human: true},
		"machine-client":   {},
		"kratos-down":      {err: errors.New("kratos unreachable")},
	}
	handler := &EmailVerificationHandler{Secret: secret, Resolve: func(_ context.Context, subject string) (bool, bool, error) {
		a := answers[subject]
		return a.human, a.verified, a.err
	}}
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	client := &EmailVerificationClient{URL: srv.URL, Secret: secret}

	for subject, want := range map[string]string{
		"verified-human":   "allow",
		"machine-client":   "allow",
		"unverified-human": "forbidden",
		"kratos-down":      "error",
	} {
		t.Run(subject, func(t *testing.T) {
			err := client.Check(context.Background(), subject)
			switch want {
			case "allow":
				if err != nil {
					t.Fatalf("Check = %v, want allowed", err)
				}
			case "forbidden":
				if !errors.Is(err, ErrForbidden) {
					t.Fatalf("Check = %v, want ErrForbidden", err)
				}
			default:
				if err == nil {
					t.Fatal("Check allowed a subject whose state could not be resolved")
				}
			}
		})
	}
}

// Every way the verb can fail to answer must refuse, never allow.
func TestEmailVerificationFailsClosed(t *testing.T) {
	allow := func(context.Context, string) (bool, bool, error) { return true, true, nil }
	for name, tc := range map[string]struct {
		handler      *EmailVerificationHandler
		clientSecret string
	}{
		"unwired resolver": {handler: &EmailVerificationHandler{Secret: []byte("s")}, clientSecret: "s"},
		"wrong secret":     {handler: &EmailVerificationHandler{Secret: []byte("s"), Resolve: allow}, clientSecret: "other"},
		"no server secret": {handler: &EmailVerificationHandler{Resolve: allow}, clientSecret: "s"},
	} {
		t.Run(name, func(t *testing.T) {
			srv := httptest.NewServer(tc.handler)
			t.Cleanup(srv.Close)
			client := &EmailVerificationClient{URL: srv.URL, Secret: []byte(tc.clientSecret)}
			if err := client.Check(context.Background(), "verified-human"); err == nil {
				t.Fatal("Check allowed without a trustworthy answer")
			}
		})
	}
}
