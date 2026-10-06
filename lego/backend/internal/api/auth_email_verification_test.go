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
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// verifyKratos serves whoami for "live-session" with the trait email verified
// or not, so the gate sees exactly Kratos's verifiable_addresses state.
func verifyKratos(t *testing.T, verified bool) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/sessions/whoami" || r.Header.Get("X-Session-Token") != "live-session" {
			http.Error(w, `{"error":{"code":401}}`, http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"identity":{"id":"identity-1","traits":{"email":"Ada@Example.com"},"verifiable_addresses":[{"value":"ada@example.com","verified":%t},{"value":"other@example.com","verified":true}]}}`, verified)
	}))
	t.Cleanup(srv.Close)
	return srv
}

type countingOnboard struct{ calls atomic.Int32 }

func (o *countingOnboard) EnsureTenant(context.Context, string, string, bool) (string, error) {
	o.calls.Add(1)
	return "tea-1", nil
}

func serveGate(t *testing.T, s *Server, configure func(*http.Request)) *httptest.ResponseRecorder {
	t.Helper()
	gate, err := s.newAuthGate()
	if err != nil {
		t.Fatalf("newAuthGate: %v", err)
	}
	return serveWith(gate, configure)
}

func serveWith(gate *oryAuth, configure func(*http.Request)) *httptest.ResponseRecorder {
	r := httptest.NewRequest(http.MethodGet, "/v1/services", nil)
	configure(r)
	w := httptest.NewRecorder()
	gate.middleware(echoIdentity).ServeHTTP(w, r)
	return w
}

func withSession(r *http.Request) { r.Header.Set("X-Session-Token", "live-session") }
func withBearer(r *http.Request)  { r.Header.Set("Authorization", "Bearer "+testToken) }

func assertVerificationRefusal(t *testing.T, w *httptest.ResponseRecorder) {
	t.Helper()
	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403; body %q", w.Code, w.Body.String())
	}
	var body struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil || body.Code != core.CodeEmailVerificationRequired {
		t.Fatalf("body = %q, want code %s", w.Body.String(), core.CodeEmailVerificationRequired)
	}
}

// An unverified session is refused before onboarding, so no workspace is
// minted; a verification of some OTHER address on the identity does not count.
func TestEmailVerificationGateRefusesUnverifiedSessionBeforeOnboarding(t *testing.T) {
	onboard := &countingOnboard{}
	s := &Server{HydraAdminURL: fakeHydraURL(t), KratosURL: verifyKratos(t, false).URL, Onboard: onboard, RequireVerifiedEmail: true}
	assertVerificationRefusal(t, serveGate(t, s, withSession))
	if n := onboard.calls.Load(); n != 0 {
		t.Fatalf("EnsureTenant called %d times for an unverified caller", n)
	}
}

func TestEmailVerificationGateAdmitsVerifiedSession(t *testing.T) {
	onboard := &countingOnboard{}
	s := &Server{HydraAdminURL: fakeHydraURL(t), KratosURL: verifyKratos(t, true).URL, Onboard: onboard, RequireVerifiedEmail: true}
	w := serveGate(t, s, withSession)
	if w.Code != http.StatusOK || w.Body.String() != "session/identity-1" {
		t.Fatalf("status %d body %q", w.Code, w.Body.String())
	}
	if onboard.calls.Load() != 1 {
		t.Fatalf("verified caller was not onboarded")
	}
}

func TestEmailVerificationGateOffAdmitsUnverifiedSession(t *testing.T) {
	s := &Server{HydraAdminURL: fakeHydraURL(t), KratosURL: verifyKratos(t, false).URL}
	if w := serveGate(t, s, withSession); w.Code != http.StatusOK {
		t.Fatalf("gate off: status %d body %q", w.Code, w.Body.String())
	}
}

// A human OAuth token carries no email, so the gate resolves its subject via
// Kratos admin; machine tokens never consult it.
func TestEmailVerificationGateHumanOAuth(t *testing.T) {
	for _, tc := range []struct {
		name     string
		lookup   func(context.Context, string) (bool, bool)
		wantCode int
	}{
		{name: "verified", lookup: func(context.Context, string) (bool, bool) { return true, true }, wantCode: http.StatusOK},
		{name: "unverified", lookup: func(context.Context, string) (bool, bool) { return false, true }, wantCode: http.StatusForbidden},
		{name: "kratos admin miss fails closed", lookup: func(context.Context, string) (bool, bool) { return false, false }, wantCode: http.StatusServiceUnavailable},
		{name: "kratos admin unwired fails closed", lookup: nil, wantCode: http.StatusServiceUnavailable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := &Server{HydraAdminURL: fakeHumanHydra(t).URL, RequireVerifiedEmail: true, EmailVerified: tc.lookup}
			w := serveGate(t, s, withBearer)
			if tc.wantCode == http.StatusForbidden {
				assertVerificationRefusal(t, w)
				return
			}
			if w.Code != tc.wantCode {
				t.Fatalf("status %d, want %d; body %q", w.Code, tc.wantCode, w.Body.String())
			}
		})
	}
}

func TestEmailVerificationGateExemptsMachineTokens(t *testing.T) {
	var lookups atomic.Int32
	s := &Server{HydraAdminURL: fakeHydraURL(t), RequireVerifiedEmail: true,
		EmailVerified: func(context.Context, string) (bool, bool) { lookups.Add(1); return false, true }}
	w := serveGate(t, s, withBearer)
	if w.Code != http.StatusOK || w.Body.String() != "oauth2/client-1" {
		t.Fatalf("machine token: status %d body %q", w.Code, w.Body.String())
	}
	if lookups.Load() != 0 {
		t.Fatalf("machine token consulted the verification lookup")
	}
}

// A refusal is cached only for unverifiedTTL, not PositiveTTL: a retry loop
// is served from cache (no Kratos amplification), and once the window passes
// a freshly verified user is admitted.
func TestEmailVerificationGateRefusalCacheIsShort(t *testing.T) {
	var verified atomic.Bool
	var lookups atomic.Int32
	s := &Server{HydraAdminURL: fakeHumanHydra(t).URL, RequireVerifiedEmail: true,
		EmailVerified: func(context.Context, string) (bool, bool) { lookups.Add(1); return verified.Load(), true }}
	gate, err := s.newAuthGate()
	if err != nil {
		t.Fatalf("newAuthGate: %v", err)
	}
	gate.unverifiedTTL = 50 * time.Millisecond
	assertVerificationRefusal(t, serveWith(gate, withBearer))
	assertVerificationRefusal(t, serveWith(gate, withBearer))
	if n := lookups.Load(); n != 1 {
		t.Fatalf("retry within the refusal window made %d lookups, want 1", n)
	}
	verified.Store(true)
	time.Sleep(60 * time.Millisecond)
	if w := serveWith(gate, withBearer); w.Code != http.StatusOK {
		t.Fatalf("after verifying: status %d body %q", w.Code, w.Body.String())
	}
}
