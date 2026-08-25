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
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// countingIdentityProvider stands in for Hydra + Kratos and counts the upstream
// calls each one receives — the quantity the w1/m67 F1 bound exists to cap.
type countingIdentityProvider struct {
	url          string
	introspects  atomic.Int32
	whoamis      atomic.Int32
	validToken   string
	validSession string
}

func newCountingIdentityProvider(t *testing.T, validToken, validSession string) *countingIdentityProvider {
	t.Helper()
	p := &countingIdentityProvider{validToken: validToken, validSession: validSession}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/admin/oauth2/introspect":
			p.introspects.Add(1)
			_ = r.ParseForm()
			if p.validToken != "" && r.PostFormValue("token") == p.validToken {
				_, _ = fmt.Fprint(w, `{"active":true,"sub":"machine-1","client_id":"machine-1"}`)
				return
			}
			_, _ = fmt.Fprint(w, `{"active":false}`)
		case "/sessions/whoami":
			p.whoamis.Add(1)
			if p.validSession != "" && r.Header.Get("X-Session-Token") == p.validSession {
				_, _ = fmt.Fprint(w, `{"identity":{"id":"identity-1"}}`)
				return
			}
			w.WriteHeader(http.StatusUnauthorized)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	p.url = srv.URL
	return p
}

func gateWithAdmission(p *countingIdentityProvider, adm *AuthAdmission) http.Handler {
	return newOryAuth(p.url, p.url, "", "", "", false, adm, nil, nil, "").middleware(echoIdentity)
}

func bearerRequest(token, sourceIP string) *http.Request {
	r := httptest.NewRequest(http.MethodGet, "/v1/services", nil)
	r.Header.Set("Authorization", "Bearer "+token)
	r.RemoteAddr = sourceIP + ":40000"
	return r
}

// TestInvalidCredentialFloodIsBoundedBeforeUpstream is the w1/m67 F1 regression.
// Auth wraps the identity-keyed limiter, inactive tokens are never cached, and
// singleflight only coalesces identical tokens — so before this bound, N unique
// invalid bearers cost exactly N Hydra round trips, with nothing in the request
// path able to stop them.
func TestInvalidCredentialFloodIsBoundedBeforeUpstream(t *testing.T) {
	p := newCountingIdentityProvider(t, "", "")
	const budget = 5
	mw := gateWithAdmission(p, NewAuthAdmission(budget, budget, 0))

	const attempts = 50
	var shed int
	for i := range attempts {
		w := httptest.NewRecorder()
		mw.ServeHTTP(w, bearerRequest(fmt.Sprintf("unique-invalid-%d", i), "203.0.113.10"))
		switch w.Code {
		case http.StatusTooManyRequests:
			shed++
		case http.StatusUnauthorized:
		default:
			t.Fatalf("attempt %d: status = %d, want 401 or 429", i, w.Code)
		}
	}

	if got := int(p.introspects.Load()); got > budget+1 {
		t.Errorf("upstream introspections = %d for %d unique invalid tokens; want ≤ %d — the flood must be shed BEFORE Hydra",
			got, attempts, budget+1)
	}
	if shed == 0 {
		t.Error("no request was shed; the bound never engaged")
	}
}

// Valid credentials are isolated from one another, so many users behind one
// source IP do not share a bucket even though each credential is bounded.
func TestValidCredentialsAreIsolated(t *testing.T) {
	p := newCountingIdentityProvider(t, "good-token", "")
	mw := gateWithAdmission(p, NewAuthAdmission(2, 2, 0))

	for i := range 20 {
		w := httptest.NewRecorder()
		// Distinct tokens so nothing is served from the positive cache: every one
		// of these is a real upstream call, and every one authenticates.
		p.validToken = fmt.Sprintf("good-token-%d", i)
		mw.ServeHTTP(w, bearerRequest(p.validToken, "203.0.113.20"))
		if w.Code != http.StatusOK {
			t.Fatalf("valid credential %d: status = %d, want 200", i, w.Code)
		}
	}
}

func TestValidSessionIsRejectedBeforeAnotherKratosCallWhenOverBudget(t *testing.T) {
	p := newCountingIdentityProvider(t, "", "live-session")
	mw := gateWithAdmission(p, NewAuthAdmission(2, 2, 0))
	for i := range 3 {
		r := httptest.NewRequest(http.MethodGet, "/v1/services", nil)
		r.Header.Set("X-Session-Token", "live-session")
		r.RemoteAddr = "203.0.113.21:40000"
		w := httptest.NewRecorder()
		mw.ServeHTTP(w, r)
		if i < 2 && w.Code != http.StatusOK {
			t.Fatalf("valid session request %d = %d, want 200", i, w.Code)
		}
		if i == 2 && w.Code != http.StatusTooManyRequests {
			t.Fatalf("over-budget valid session = %d, want 429", w.Code)
		}
	}
	if got := p.whoamis.Load(); got != 2 {
		t.Fatalf("Kratos calls = %d, want 2; rejection must happen before upstream I/O", got)
	}
}

func TestSessionCookieBudgetCannotBeBypassedWithIrrelevantCookies(t *testing.T) {
	adm := NewAuthAdmission(1, 1, 0)
	request := func(extra string) *http.Request {
		r := httptest.NewRequest(http.MethodGet, "/v1/services", nil)
		r.Header.Set("Cookie", "ory_kratos_session=live-session; noise="+extra)
		r.RemoteAddr = "203.0.113.21:40000"
		return r
	}
	release, err := adm.admit(request("one"))
	if err != nil {
		t.Fatal(err)
	}
	release()
	if _, err := adm.admit(request("two")); !errors.Is(err, errAuthOverloaded) {
		t.Fatalf("same session with changed irrelevant cookie = %v, want errAuthOverloaded", err)
	}
}

func TestOneCredentialCannotOccupyTheGlobalAuthenticationPool(t *testing.T) {
	adm := NewAuthAdmission(10_000, 10_000, 64)
	r := bearerRequest("one-credential", "203.0.113.22")
	var releases []func()
	for range 8 {
		release, err := adm.admit(r)
		if err != nil {
			t.Fatalf("credential slot: %v", err)
		}
		releases = append(releases, release)
	}
	if _, err := adm.admit(r); !errors.Is(err, errAuthOverloaded) {
		t.Fatalf("ninth concurrent call = %v, want errAuthOverloaded", err)
	}
	other := bearerRequest("other-credential", "203.0.113.22")
	releaseOther, err := adm.admit(other)
	if err != nil {
		t.Fatalf("other credential was denied while global pool had capacity: %v", err)
	}
	releaseOther()
	for _, release := range releases {
		release()
	}
}

// Sessions are the other upstream. Kratos is not positively cached, so the
// failure budget is what keeps a session flood off it.
func TestInvalidSessionFloodIsBounded(t *testing.T) {
	p := newCountingIdentityProvider(t, "", "live-session")
	const budget = 4
	mw := gateWithAdmission(p, NewAuthAdmission(budget, budget, 0))

	for i := range 30 {
		r := httptest.NewRequest(http.MethodGet, "/v1/services", nil)
		r.Header.Set("X-Session-Token", fmt.Sprintf("bogus-%d", i))
		r.RemoteAddr = "203.0.113.30:40000"
		mw.ServeHTTP(httptest.NewRecorder(), r)
	}
	if got := int(p.whoamis.Load()); got > budget+1 {
		t.Errorf("upstream whoami calls = %d; want ≤ %d", got, budget+1)
	}
}

// One source exhausting its budget must not shed another's traffic.
func TestBudgetIsPerSource(t *testing.T) {
	p := newCountingIdentityProvider(t, "good", "")
	mw := gateWithAdmission(p, NewAuthAdmission(3, 3, 0))

	for i := range 20 {
		mw.ServeHTTP(httptest.NewRecorder(), bearerRequest(fmt.Sprintf("bad-%d", i), "203.0.113.40"))
	}
	w := httptest.NewRecorder()
	mw.ServeHTTP(w, bearerRequest("good", "203.0.113.41"))
	if w.Code != http.StatusOK {
		t.Errorf("second source: status = %d, want 200 — budgets are per client IP", w.Code)
	}
}

// Behind a trusted proxy the budget must key on the forwarded client, not on the
// edge's single address (which would let one abusive client shed everyone).
func TestBudgetKeysOnForwardedClientBehindTrustedProxy(t *testing.T) {
	p := newCountingIdentityProvider(t, "good", "")
	adm := NewAuthAdmission(3, 3, 0)
	trusted, err := core.ParseTrustedProxies("192.0.2.0/24")
	if err != nil {
		t.Fatalf("parse trusted proxies: %v", err)
	}
	adm.TrustedProxies = trusted
	mw := gateWithAdmission(p, adm)

	exhaust := func(clientIP string) {
		for i := range 20 {
			r := bearerRequest(fmt.Sprintf("bad-%d", i), "192.0.2.7") // the edge proxy
			r.Header.Set("X-Forwarded-For", clientIP)
			mw.ServeHTTP(httptest.NewRecorder(), r)
		}
	}
	exhaust("198.51.100.5")

	r := bearerRequest("good", "192.0.2.7")
	r.Header.Set("X-Forwarded-For", "198.51.100.6") // a different real client
	w := httptest.NewRecorder()
	mw.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("other forwarded client: status = %d, want 200", w.Code)
	}
}

// An absurd credential must be refused before it is allocated into an upstream
// request or forwarded as a header.
func TestOversizedCredentialNeverReachesUpstream(t *testing.T) {
	p := newCountingIdentityProvider(t, "good", "live-session")
	mw := gateWithAdmission(p, nil) // bound off: the size check is unconditional

	w := httptest.NewRecorder()
	mw.ServeHTTP(w, bearerRequest(strings.Repeat("A", maxCredentialBytes+1), "203.0.113.50"))
	if w.Code != http.StatusUnauthorized {
		t.Errorf("oversized bearer: status = %d, want 401", w.Code)
	}

	r := httptest.NewRequest(http.MethodGet, "/v1/services", nil)
	r.Header.Set("X-Session-Token", strings.Repeat("B", maxCredentialBytes+1))
	mw.ServeHTTP(httptest.NewRecorder(), r)

	if got := p.introspects.Load() + p.whoamis.Load(); got != 0 {
		t.Errorf("upstream calls = %d; an oversized credential must never be forwarded", got)
	}
}

// With no admission configured the gate behaves exactly as it did before m67.
func TestAdmissionOffIsUnchangedBehavior(t *testing.T) {
	p := newCountingIdentityProvider(t, "", "")
	mw := gateWithAdmission(p, nil)

	for i := range 10 {
		w := httptest.NewRecorder()
		mw.ServeHTTP(w, bearerRequest(fmt.Sprintf("bad-%d", i), "203.0.113.60"))
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", w.Code)
		}
	}
	if got := int(p.introspects.Load()); got != 10 {
		t.Errorf("upstream introspections = %d, want 10 (unmetered)", got)
	}
}

// TestShedRequestsDoNotAllocateCredentialEntries is the codex-security
// 2026-08 F1 ordering regression: once a source has spent its failure budget,
// further requests with distinct bearers must not lazily insert per-credential
// limiter entries — a shed request performs no allocation. Before the fix,
// admit() created (and spent) the credential bucket before reading the
// per-source budget, so an anonymous flood of unique tokens grew the map
// one entry per request even while being correctly shed.
func TestShedRequestsDoNotAllocateCredentialEntries(t *testing.T) {
	adm := NewAuthAdmission(1, 1, 0)
	// Drain the source's failure budget the way the real path does: one
	// admitted attempt whose upstream verdict was invalid (penalize).
	first := bearerRequest("first-token", "203.0.113.70")
	release, err := adm.admit(first)
	if err != nil {
		t.Fatalf("first admit: %v", err)
	}
	release()
	adm.penalize(first)
	for i := range 10 {
		r := bearerRequest(fmt.Sprintf("unique-token-%d", i), "203.0.113.70")
		if _, err := adm.admit(r); !errors.Is(err, errAuthOverloaded) {
			t.Fatalf("attempt %d after budget exhaustion: err = %v, want errAuthOverloaded", i, err)
		}
	}
	if got := adm.credentials.Entries(); got > 1 {
		t.Errorf("credentials limiter holds %d entries after an exhausted-source flood; want ≤ 1 — shed requests must not allocate", got)
	}
}
