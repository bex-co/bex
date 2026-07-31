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
	"net/http"
	"sync/atomic"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// TestWebhookRateLimiterShedsBeforeHandler is w7/m60 t001: a flood against a
// webhook intake sheds with 429 BEFORE the handler runs — i.e. before the body
// read and HMAC/signature verification the handler performs — while an in-budget
// delivery always passes, and a nil limiter (BEX_WEBHOOK_RATE_LIMIT=0) disables
// metering byte-identically. The spy handler stands in for the real
// signature-verifying handler: "handler not reached" == "no HMAC computed".
func TestWebhookRateLimiterShedsBeforeHandler(t *testing.T) {
	var calls int32
	spy := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusNoContent)
	})
	build := func(rl *RateLimiter) http.Handler {
		srv := NewServer(&core.Base{Client: fakeClient(), Namespace: "default"}, Deps{StripeWebhook: spy})
		srv.HydraAdminURL = fakeHydraURL(t)
		srv.WebhookRateLimiter = rl
		h, err := srv.Handler()
		if err != nil {
			t.Fatalf("Handler: %v", err)
		}
		return h
	}

	// Budget of 1 token: the first delivery passes; the flood behind it (same
	// source IP — httptest's default RemoteAddr) sheds with 429 and never reaches
	// the handler, so no signature is ever computed for a shed request.
	atomic.StoreInt32(&calls, 0)
	h := build(NewRateLimiter(60, 1))
	codes := make([]int, 4)
	for i := range codes {
		codes[i] = do(t, h, http.MethodPost, "/v1/webhooks/stripe", "", "{}").Code
	}
	if codes[0] != http.StatusNoContent {
		t.Fatalf("first in-budget delivery = %d, want 204", codes[0])
	}
	for i := 1; i < len(codes); i++ {
		if codes[i] != http.StatusTooManyRequests {
			t.Fatalf("flood request %d = %d, want 429", i, codes[i])
		}
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("handler reached %d times; shed requests must not reach HMAC verification (want 1)", got)
	}

	// nil limiter => disabled: every delivery reaches the handler (byte-identical
	// to pre-m60 behavior).
	atomic.StoreInt32(&calls, 0)
	h = build(nil)
	for i := 0; i < 4; i++ {
		if code := do(t, h, http.MethodPost, "/v1/webhooks/stripe", "", "{}").Code; code != http.StatusNoContent {
			t.Fatalf("disabled-limiter request %d = %d, want 204", i, code)
		}
	}
	if got := atomic.LoadInt32(&calls); got != 4 {
		t.Fatalf("disabled limiter reached handler %d times, want 4 (0 must disable metering)", got)
	}

	// The git intake shares the same limiter wiring; a smoke check that its mount
	// is limiter-wrapped too (the git handler 503s with no secret, but only if the
	// limiter lets it through). One request is within a fresh budget.
	srv := NewServer(&core.Base{Client: fakeClient(sampleApp("web")), Namespace: "default"}, Deps{})
	srv.HydraAdminURL = fakeHydraURL(t)
	srv.WebhookRateLimiter = NewRateLimiter(60, 1)
	gh, err := srv.Handler()
	if err != nil {
		t.Fatalf("Handler: %v", err)
	}
	first := do(t, gh, http.MethodPost, "/v1/webhooks/git", "", "{}").Code
	second := do(t, gh, http.MethodPost, "/v1/webhooks/git", "", "{}").Code
	if first == http.StatusTooManyRequests {
		t.Fatalf("first git webhook delivery shed (%d); the budget must admit it", first)
	}
	if second != http.StatusTooManyRequests {
		t.Fatalf("second git webhook delivery = %d, want 429 (limiter must wrap this mount too)", second)
	}
}

// alwaysPublicInventory is the CI-enforced census (ADR045 Finding 8, w7/m60 t002)
// of every route the composed rootMux mounts OUTSIDE the auth gate, each with its
// credential + limiter classification. It mirrors the prose always-public
// inventory in internal/api/CLAUDE.md. A new directly-mounted route absent here —
// or a stale entry no longer mounted — turns TestComposedMuxAlwaysPublicInventory
// red, forcing a classification decision rather than a silent outside-gate mount.
var alwaysPublicInventory = map[string]string{
	"GET /healthz":                              "liveness probe; no credential; constant-cost, unmetered",
	"POST /v1/device-grant":                     "RFC 8628 device flow; IP-keyed DeviceRateLimiter (OAuth slow_down 429)",
	"POST /v1/device-token":                     "RFC 8628 device flow; IP-keyed DeviceRateLimiter",
	"POST /v1/token/refresh/":                   "RFC 8628 device flow; IP-keyed DeviceRateLimiter",
	"POST /v1/webhooks/git":                     "HMAC signature; IP-keyed WebhookRateLimiter, sheds pre-HMAC (w7/m60)",
	"POST /v1/webhooks/stripe":                  "Stripe-Signature HMAC; IP-keyed WebhookRateLimiter, sheds pre-HMAC (w7/m60)",
	"/v1/deploy-hooks":                          "unguessable URL token; per-hook token bucket",
	"/v1/deploy-hooks/":                         "unguessable URL token; per-hook token bucket",
	"GET /.well-known/oauth-protected-resource": "RFC 9728 discovery; public by spec; no credential, unmetered",
}

// gatedWildcards are the three surfaces behind the OAuth gate + identity-keyed
// RateLimiter. They are present on rootMux but are NOT always-public, so the
// census skips them (they are not an unmetered outside-gate hazard).
var gatedWildcards = map[string]bool{
	"/v1/":     true,
	"/graphql": true,
	"/mcp":     true,
}

// fullyMountedRootMux builds a rootMux with every optional dep present so every
// always-public route mounts — the fixture the census diffs both ways.
func fullyMountedRootMux(t *testing.T) *http.ServeMux {
	t.Helper()
	srv := NewServer(&core.Base{Client: fakeClient(sampleApp("web")), Namespace: "default"}, Deps{
		StripeWebhook: http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}),
	})
	srv.HydraAdminURL = fakeHydraURL(t)
	// Enables the RFC 9728 discovery mount (resourceMetadataURL != "").
	srv.OAuthIssuer = "https://oauth.example"
	srv.OAuthResource = "https://api.example/mcp"
	mux, err := srv.rootMux()
	if err != nil {
		t.Fatalf("rootMux: %v", err)
	}
	return mux
}

// TestComposedMuxAlwaysPublicInventory is w7/m60 t002 (the w7/012 fix): every
// directly-mounted route on the composed rootMux must be a classified
// always-public inventory entry, and every inventory entry must actually be
// mounted. The m55 REST matrix guards only feature RegisterREST routes; this
// guards the top-level mux where the outside-gate routes live.
func TestComposedMuxAlwaysPublicInventory(t *testing.T) {
	mux := fullyMountedRootMux(t)
	patterns := serveMuxPatterns(mux)

	// Floor guard: a Go release that reshapes ServeMux internals would return zero
	// patterns and pass this vacuously — fail loudly instead (mirrors the m55 walk).
	if len(patterns) < len(alwaysPublicInventory)+len(gatedWildcards) {
		t.Fatalf("walked only %d rootMux patterns (< %d expected); ServeMux internals may have changed",
			len(patterns), len(alwaysPublicInventory)+len(gatedWildcards))
	}

	seen := make(map[string]bool, len(patterns))
	for _, p := range patterns {
		seen[p] = true
		if gatedWildcards[p] {
			continue
		}
		if _, ok := alwaysPublicInventory[p]; !ok {
			t.Errorf("directly-mounted route %q is not in the always-public inventory — classify it "+
				"(credential + limiter) in alwaysPublicInventory and internal/api/CLAUDE.md, or gate it behind /v1/", p)
		}
	}
	// Stale-entry direction: an inventory entry no longer mounted is a lie.
	for p := range alwaysPublicInventory {
		if !seen[p] {
			t.Errorf("always-public inventory lists %q but rootMux did not mount it (stale entry)", p)
		}
	}
	for p := range gatedWildcards {
		if !seen[p] {
			t.Errorf("gated wildcard %q not mounted", p)
		}
	}
}

// TestAlwaysPublicGuardCatchesUnclassifiedMount is the anti-tautology self-check:
// the exact classification the census applies must flag a bogus unclassified
// directly-mounted route, proving the guard is not vacuous.
func TestAlwaysPublicGuardCatchesUnclassifiedMount(t *testing.T) {
	mux := http.NewServeMux()
	for p := range alwaysPublicInventory {
		mux.Handle(p, http.NotFoundHandler())
	}
	for p := range gatedWildcards {
		mux.Handle(p, http.NotFoundHandler())
	}
	mux.Handle("POST /v1/webhooks/evil", http.NotFoundHandler()) // unclassified, outside gate

	var unclassified []string
	for _, p := range serveMuxPatterns(mux) {
		if gatedWildcards[p] {
			continue
		}
		if _, ok := alwaysPublicInventory[p]; !ok {
			unclassified = append(unclassified, p)
		}
	}
	if len(unclassified) != 1 || unclassified[0] != "POST /v1/webhooks/evil" {
		t.Fatalf("census failed to single out the unclassified mount; flagged %v", unclassified)
	}

	// Stale-entry direction: an inventory entry no longer mounted must be caught.
	// The real mux never lists "POST /v1/webhooks/evil", so treating the augmented
	// inventory as the census over the real fixture flags it as stale.
	real := fullyMountedRootMux(t)
	seen := make(map[string]bool)
	for _, p := range serveMuxPatterns(real) {
		seen[p] = true
	}
	augmented := map[string]bool{"POST /v1/webhooks/evil": true}
	for p := range alwaysPublicInventory {
		augmented[p] = true
	}
	var stale []string
	for p := range augmented {
		if !seen[p] {
			stale = append(stale, p)
		}
	}
	if len(stale) != 1 || stale[0] != "POST /v1/webhooks/evil" {
		t.Fatalf("census failed to single out the stale inventory entry; flagged %v", stale)
	}
}
