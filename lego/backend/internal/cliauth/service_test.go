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

package cliauth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

func renderRequest(method, path, body string) *http.Request {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	// The official CLI labels its JSON body as form data; the adapter must honor
	// the actual bytes, not trust this misleading header.
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return r
}

func TestRenderProtocolAdapters(t *testing.T) {
	var calls []url.Values
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parse upstream form: %v", err)
		}
		calls = append(calls, r.Form)
		switch {
		case r.URL.Path == "/oauth2/device/auth":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"device_code": "device-1", "user_code": "ABCDEF",
				"verification_uri":          "https://dashboard.bex.co/auth/device",
				"verification_uri_complete": "https://dashboard.bex.co/auth/device?user_code=ABCDEF",
				"expires_in":                600, "interval": 5,
			})
		case r.URL.Path == "/oauth2/token" && r.Form.Get("grant_type") == deviceGrantType:
			_ = json.NewEncoder(w).Encode(map[string]any{
				"access_token": "access-1", "refresh_token": "refresh-1",
				"token_type": "bearer", "expires_in": 900,
			})
		case r.URL.Path == "/oauth2/token" && r.Form.Get("grant_type") == "refresh_token":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"access_token": "access-2", "refresh_token": "refresh-2",
				"token_type": "bearer", "expires_in": 900,
			})
		default:
			http.Error(w, "unexpected upstream request", http.StatusBadRequest)
		}
	}))
	defer upstream.Close()

	svc := New(upstream.URL, "", nil, nil)
	mux := http.NewServeMux()
	svc.RegisterPublic(mux)

	cases := []struct {
		path string
		body string
		want string
	}{
		{"/v1/device-grant", `{"client_id":"` + RenderCLIClientID + `"}`, `"device_code":"device-1"`},
		{"/v1/device-token", `{"grant_type":"` + deviceGrantType + `","client_id":"` + RenderCLIClientID + `","device_code":"device-1"}`, `"refresh_token":"refresh-1"`},
		{"/v1/token/refresh/", `{"grant_type":"refresh_token","refresh_token":"refresh-1"}`, `"refresh_token":"refresh-2"`},
	}
	for _, tc := range cases {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, renderRequest(http.MethodPost, tc.path, tc.body))
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), tc.want) {
			t.Fatalf("%s => %d %s", tc.path, rec.Code, rec.Body.String())
		}
	}
	if len(calls) != 3 {
		t.Fatalf("upstream calls = %d, want 3", len(calls))
	}
	if calls[0].Get("client_id") != RenderCLIClientID || calls[0].Get("scope") != "openid offline_access" {
		t.Errorf("device grant form = %v", calls[0])
	}
	if calls[1].Get("device_code") != "device-1" || calls[2].Get("client_id") != RenderCLIClientID {
		t.Errorf("token forms = %v / %v", calls[1], calls[2])
	}
}

func TestRenderProtocolRejectsWrongClientBeforeHydra(t *testing.T) {
	called := false
	upstream := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }))
	defer upstream.Close()
	svc := New(upstream.URL, "", nil, nil)
	mux := http.NewServeMux()
	svc.RegisterPublic(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, renderRequest(http.MethodPost, "/v1/device-grant", `{"client_id":"attacker"}`))
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "invalid_client") || called {
		t.Fatalf("wrong client => %d %q called=%v", rec.Code, rec.Body.String(), called)
	}
}

type fakeRevoker struct {
	id  string
	err error
}

func (f *fakeRevoker) RevokeAPIKey(_ context.Context, _ string, id string) error {
	f.id = id
	return f.err
}

func TestLogoutRevokesPlatformHumanConsentChainAndKeepsSharedClient(t *testing.T) {
	for _, clientID := range []string{RenderCLIClientID, MobileClientID} {
		t.Run(clientID, func(t *testing.T) {
			var subject, client string
			admin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodDelete || r.URL.Path != "/admin/oauth2/auth/sessions/consent" {
					t.Fatalf("unexpected admin request %s %s", r.Method, r.URL.Path)
				}
				subject, client = r.URL.Query().Get("subject"), r.URL.Query().Get("client")
				w.WriteHeader(http.StatusNoContent)
			}))
			defer admin.Close()

			invalidated := ""
			var invalidatedIdentity core.Identity
			svc := New("", admin.URL, nil, func(token string, identity core.Identity) {
				invalidated = token
				invalidatedIdentity = identity
			})
			h := http.HandlerFunc(svc.revoke)
			id := core.Identity{Subject: "kratos-user-a", Method: "oauth2", ClientID: clientID, Human: true}
			req := httptest.NewRequest(http.MethodPost, "/v1/oauth/revoke", nil)
			req.Header.Set("Authorization", "Bearer access-a")
			req = req.WithContext(core.WithIdentity(req.Context(), id))
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			if rec.Code != http.StatusNoContent {
				t.Fatalf("revoke => %d %s", rec.Code, rec.Body.String())
			}
			if subject != "kratos-user-a" || client != clientID || invalidated != "access-a" || invalidatedIdentity != id {
				t.Fatalf("subject=%q client=%q invalidated=%q identity=%+v", subject, client, invalidated, invalidatedIdentity)
			}
		})
	}
}

func TestLogoutRetainsAPIKeySelfRevoke(t *testing.T) {
	revoker := &fakeRevoker{}
	svc := New("", "", revoker, nil)
	h := http.HandlerFunc(svc.revoke)
	id := core.Identity{Subject: "api-key-1", Method: "oauth2", ClientID: "api-key-1"}
	req := httptest.NewRequest(http.MethodPost, "/v1/oauth/revoke", nil)
	req = req.WithContext(core.WithIdentity(req.Context(), id))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent || revoker.id != "api-key-1" {
		t.Fatalf("revoke => %d id=%q body=%s", rec.Code, revoker.id, rec.Body.String())
	}
}

func TestLogoutFailsClosed(t *testing.T) {
	t.Run("no identity", func(t *testing.T) {
		rec := httptest.NewRecorder()
		http.HandlerFunc(New("", "", nil, nil).revoke).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/oauth/revoke", nil))
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d", rec.Code)
		}
	})
	t.Run("admin unavailable", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/v1/oauth/revoke", nil)
		id := core.Identity{Subject: "user", Method: "oauth2", ClientID: RenderCLIClientID, Human: true}
		req = req.WithContext(core.WithIdentity(req.Context(), id))
		http.HandlerFunc(New("", "", nil, nil).revoke).ServeHTTP(rec, req)
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
		}
		// /v1/oauth/revoke speaks the Render {"error","message","id"} dialect on
		// every branch, not the OAuth {"error":"temporarily_unavailable"} body
		// (w9/m38, w9/008).
		var body map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("body not JSON: %v (%s)", err, rec.Body.String())
		}
		if body["message"] == nil || body["id"] != "unavailable" || body["error"] == "temporarily_unavailable" {
			t.Fatalf("revoke 503 not Render-shaped: %s", rec.Body.String())
		}
	})
	t.Run("api key error", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/v1/oauth/revoke", nil)
		req = req.WithContext(core.WithIdentity(req.Context(), core.Identity{Subject: "key", Method: "oauth2", ClientID: "key"}))
		http.HandlerFunc(New("", "", &fakeRevoker{err: errors.New("boom")}, nil).revoke).ServeHTTP(rec, req)
		if rec.Code == http.StatusNoContent {
			t.Fatal("unexpected success")
		}
	})
	t.Run("other human oauth client is not treated as an api key", func(t *testing.T) {
		revoker := &fakeRevoker{}
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/v1/oauth/revoke", nil)
		req = req.WithContext(core.WithIdentity(req.Context(), core.Identity{
			Subject: "human", Method: "oauth2", ClientID: "other-public-client", Human: true,
		}))
		http.HandlerFunc(New("", "", revoker, nil).revoke).ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest || revoker.id != "" {
			t.Fatalf("status = %d revoked=%q body=%s", rec.Code, revoker.id, rec.Body.String())
		}
	})
}

// deviceGrantRequest builds a valid device-grant request from sourceIP —
// the smallest of the three protocol bodies, used by the rate-limiter tests
// below where the request's content doesn't matter, only whether it reaches
// the (fake) upstream at all.
func deviceGrantRequest(sourceIP string) *http.Request {
	r := renderRequest(http.MethodPost, "/v1/device-grant", `{"client_id":"`+RenderCLIClientID+`"}`)
	r.RemoteAddr = sourceIP + ":54321"
	return r
}

// countingUpstream is a minimal Hydra stand-in that always answers the
// device-grant shape and counts how many requests actually reached it — the
// rate-limiter tests assert this count to prove a shed request never
// touches Hydra.
func countingUpstream(t *testing.T, calls *atomic.Int32) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"device_code": "device-1", "user_code": "ABCDEF",
			"verification_uri": "https://dashboard.bex.co/auth/device",
			"expires_in":       600, "interval": 5,
		})
	}))
	t.Cleanup(srv.Close)
	return srv
}

// newDeviceLimitedMux wires a Service (backed by a counting fake Hydra) with
// its RegisterPublic routes behind a device rate limiter at rpm/burst — the
// shared setup every rate-limiter test below needs, varying only the
// rpm/burst under test.
func newDeviceLimitedMux(t *testing.T, rpm float64, burst int) (*http.ServeMux, *atomic.Int32) {
	t.Helper()
	var calls atomic.Int32
	upstream := countingUpstream(t, &calls)
	svc := New(upstream.URL, "", nil, nil)
	svc.RateLimiter = NewDeviceRateLimiter(rpm, burst)
	mux := http.NewServeMux()
	svc.RegisterPublic(mux)
	return mux, &calls
}

// TestDeviceRateLimiterShedsFloodPerIPNotGlobally is w4/m31/t002's core
// acceptance criterion: a flood from one IP is shed before any Hydra call,
// and a second IP is entirely unaffected — proving the limiter is IP-keyed,
// not a single shared bucket every anonymous caller starves.
func TestDeviceRateLimiterShedsFloodPerIPNotGlobally(t *testing.T) {
	mux, calls := newDeviceLimitedMux(t, 60, 3) // burst 3, refills at 1/s

	for i := range 3 {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, deviceGrantRequest("203.0.113.1"))
		if rec.Code != http.StatusOK {
			t.Fatalf("request %d from the flooding IP = %d, want 200 (within burst)", i, rec.Code)
		}
	}
	if got := calls.Load(); got != 3 {
		t.Fatalf("upstream calls after burst = %d, want 3", got)
	}

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, deviceGrantRequest("203.0.113.1"))
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("4th request from the flooding IP = %d, want 429", rec.Code)
	}
	if got := calls.Load(); got != 3 {
		t.Fatalf("upstream calls after the shed request = %d, want still 3 — a shed request must never reach Hydra", got)
	}

	// A different IP has its own, untouched bucket.
	rec2 := httptest.NewRecorder()
	mux.ServeHTTP(rec2, deviceGrantRequest("198.51.100.7"))
	if rec2.Code != http.StatusOK {
		t.Fatalf("request from the second IP = %d, want 200 (unaffected by the first IP's flood)", rec2.Code)
	}
	if got := calls.Load(); got != 4 {
		t.Fatalf("upstream calls after the second IP's request = %d, want 4", got)
	}
}

// TestDeviceRateLimiterTrustedProxyXFF is the .pm/w4/029.md report-#10 fix on
// the device routes: with BEX_TRUSTED_PROXY_CIDRS configured, each real client
// behind the trusted Traefik peer gets its own bucket (so one attacker can no
// longer hold the 30/min device budget platform-wide), while a spoofed
// X-Forwarded-For from an UNTRUSTED peer is ignored.
func TestDeviceRateLimiterTrustedProxyXFF(t *testing.T) {
	rl := NewDeviceRateLimiter(60000, 1) // burst=1: a repeat for a key is shed
	tp, err := core.ParseTrustedProxies("10.0.0.0/8")
	if err != nil {
		t.Fatalf("ParseTrustedProxies: %v", err)
	}
	rl.TrustedProxies = tp

	from := func(peer, xff string) *http.Request {
		r := deviceGrantRequest(peer)
		if xff != "" {
			r.Header.Set("X-Forwarded-For", xff)
		}
		return r
	}

	// Two different clients behind the same trusted peer: independent buckets.
	if ok, _ := rl.allow(from("10.0.0.1", "203.0.113.1")); !ok {
		t.Fatal("first request from client A behind the trusted proxy: want allowed")
	}
	if ok, _ := rl.allow(from("10.0.0.1", "203.0.113.2")); !ok {
		t.Fatal("first request from client B behind the trusted proxy: want allowed (own bucket)")
	}
	// Client A's repeat exhausts only client A's bucket.
	if ok, _ := rl.allow(from("10.0.0.1", "203.0.113.1")); ok {
		t.Fatal("second request from client A: want shed")
	}

	// An untrusted peer's spoofed header is fiction: two requests claiming
	// different clients from the same untrusted peer share the peer's bucket.
	if ok, _ := rl.allow(from("192.0.2.9", "203.0.113.50")); !ok {
		t.Fatal("first request from the untrusted peer: want allowed")
	}
	if ok, _ := rl.allow(from("192.0.2.9", "203.0.113.51")); ok {
		t.Fatal("spoofed second client from the same untrusted peer: want shed (keyed on the peer)")
	}

	// Unset TrustedProxies stays byte-identical: headers ignored, peer keyed.
	plain := NewDeviceRateLimiter(60000, 1)
	if ok, _ := plain.allow(from("10.0.0.1", "203.0.113.60")); !ok {
		t.Fatal("no trusted proxies, first request: want allowed")
	}
	if ok, _ := plain.allow(from("10.0.0.1", "203.0.113.61")); ok {
		t.Fatal("no trusted proxies, same peer different XFF: want shed (headers ignored)")
	}
}

// TestDeviceCeremonyAtPollingCadenceIsNotThrottled proves a steady arrival
// rate at (not bursting past) the limiter's sustained rate is never shed —
// modeling the official CLI's device-token poll loop (a fixed interval, RFC
// 8628 default 5s) without waiting out a real ceremony: a limiter configured
// for N requests/second only ever throttles a caller who arrives FASTER than
// that, so proving steady arrivals at the limit succeed generalizes to any
// slower real polling interval.
func TestDeviceCeremonyAtPollingCadenceIsNotThrottled(t *testing.T) {
	// 600 rpm = 10/s, burst 1 — forces every request past the first to wait
	// out a real refill tick rather than coast on burst capacity, so this
	// only passes if the sustained-rate admission actually works.
	mux, calls := newDeviceLimitedMux(t, 600, 1)

	const ceremonyRequests = 8
	for i := range ceremonyRequests {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, deviceGrantRequest("203.0.113.1"))
		if rec.Code != http.StatusOK {
			t.Fatalf("ceremony request %d = %d, want 200 (arriving no faster than the sustained rate)", i, rec.Code)
		}
		time.Sleep(110 * time.Millisecond) // just over one 100ms (10/s) refill tick
	}
	if got := calls.Load(); got != ceremonyRequests {
		t.Fatalf("upstream calls = %d, want %d — every ceremony request should have reached Hydra", got, ceremonyRequests)
	}
}

// TestDeviceRateLimit429IsOAuthDialect proves a shed request on every one of
// the three device-flow routes gets the OAuth-shaped {"error":"slow_down"}
// body (RFC 8628's polling-backoff signal, understood by the official CLI)
// plus Retry-After — never Render's REST error envelope (id/message).
func TestDeviceRateLimit429IsOAuthDialect(t *testing.T) {
	mux, _ := newDeviceLimitedMux(t, 60, 1) // burst 1 — a single priming request exhausts it

	for i, path := range []string{"/v1/device-grant", "/v1/device-token", "/v1/token/refresh/"} {
		t.Run(path, func(t *testing.T) {
			// A distinct source IP per subtest — each gets its own fresh bucket,
			// since all three routes share one IP-keyed limiter (one abuse
			// surface) and this test only wants to isolate the response DIALECT,
			// not re-prove the per-IP isolation TestDeviceRateLimiterShedsFloodPerIPNotGlobally
			// already covers.
			ip := fmt.Sprintf("203.0.113.%d", 20+i)
			first := httptest.NewRecorder()
			mux.ServeHTTP(first, deviceGrantRequest(ip))
			if first.Code != http.StatusOK {
				t.Fatalf("priming request = %d, want 200", first.Code)
			}

			rec := httptest.NewRecorder()
			req := renderRequest(http.MethodPost, path, `{}`)
			req.RemoteAddr = ip + ":54321"
			mux.ServeHTTP(rec, req)
			if rec.Code != http.StatusTooManyRequests {
				t.Fatalf("%s shed request = %d, want 429", path, rec.Code)
			}
			if rec.Header().Get("Retry-After") == "" {
				t.Errorf("%s: missing Retry-After header", path)
			}
			var body map[string]string
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("%s: decode body: %v", path, err)
			}
			if body["error"] != "slow_down" {
				t.Errorf("%s body = %v, want OAuth-shaped {\"error\":\"slow_down\"}", path, body)
			}
			if _, hasID := body["id"]; hasID {
				t.Errorf("%s body carries Render's \"id\" field — leaked the wrong error dialect: %v", path, body)
			}
		})
	}
}
