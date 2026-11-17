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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/bex-co/bex/lego/backend/internal/apikeys"
	"github.com/bex-co/bex/lego/backend/internal/core"
)

// auth_apikey_test.go covers an API key sent as the bearer (docs/ADR012-auth.md
// §8e, w7/m426) at the gate. The composed REST, GraphQL and MCP surfaces are
// in auth_apikey_surfaces_test.go.

// Client ids shaped like the UUIDs Hydra mints.
const (
	keyClientA = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	keyClientB = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
)

// keyHydra is an in-memory Hydra for bearer keys. It models what the real
// Hydra v26.2.0 answered when probed on 2026-11-16 with clients registered the
// way apikeys.Create registers them (client_credentials, client_secret_post):
//
//   - POST /oauth2/token answers 200 {"access_token":"ory_at_…","expires_in":899,
//     "scope":"","token_type":"bearer"} for the right secret, and one identical
//     401 {"error":"invalid_client",…} for a wrong secret, an empty secret, an
//     unknown client and a deleted client;
//   - POST /admin/oauth2/introspect answers active with sub = client_id for a
//     live client_credentials token, and {"active":false} once its client is
//     deleted.
//
// The backend suite has no real Hydra among its dependencies; the same legs
// run against one in scripts/auth-oauth21-e2e.sh.
type keyHydra struct {
	*httptest.Server

	mu       sync.Mutex
	secrets  map[string]string // client id -> secret
	tokens   map[string]string // access token -> client id
	issued   int
	lifetime int // expires_in, seconds

	// down makes both endpoints answer 503; edgeRefuses makes the token endpoint
	// answer a 403 that is not an OAuth error, as a proxy in front of it would.
	down, edgeRefuses bool
	// hold, when set, blocks every exchange until it is closed.
	hold chan struct{}
	// introspectAs, when set, is the client every token introspects as: a
	// token endpoint that is not the introspected Hydra's own.
	introspectAs string

	exchanges, introspections int
	// lastIssued is the newest access token; lastIntrospected the token of the
	// newest introspection.
	lastIssued, lastIntrospected string
}

func newKeyHydra(t *testing.T) *keyHydra {
	t.Helper()
	h := &keyHydra{secrets: map[string]string{}, tokens: map[string]string{}, lifetime: 899}
	h.Server = httptest.NewServer(http.HandlerFunc(h.serve))
	t.Cleanup(h.Close)
	return h
}

// add registers a client and returns its bearer key.
func (h *keyHydra) add(clientID string) string {
	h.mu.Lock()
	defer h.mu.Unlock()
	secret := "secret." + clientID[:8] + "~x_y"
	h.secrets[clientID] = secret
	return apikeys.BearerKey(clientID, secret)
}

// deleteClient is admin DELETE /admin/clients/{id}: the client and its tokens go.
func (h *keyHydra) deleteClient(clientID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.secrets, clientID)
	h.dropTokens(clientID)
}

// revokeTokens is admin DELETE /admin/oauth2/tokens?client_id=: the tokens go,
// the client stays.
func (h *keyHydra) revokeTokens(clientID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.dropTokens(clientID)
}

func (h *keyHydra) dropTokens(clientID string) {
	for token, owner := range h.tokens {
		if owner == clientID {
			delete(h.tokens, token)
		}
	}
}

func (h *keyHydra) set(change func(*keyHydra)) {
	h.mu.Lock()
	defer h.mu.Unlock()
	change(h)
}

func (h *keyHydra) counts() (exchanges, introspections int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.exchanges, h.introspections
}

// newestTokens returns the last token issued and the last one introspected.
func (h *keyHydra) newestTokens() (issued, introspected string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.lastIssued, h.lastIntrospected
}

func (h *keyHydra) serve(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	h.mu.Lock()
	hold := h.hold
	h.mu.Unlock()
	if hold != nil && r.URL.Path == "/oauth2/token" {
		<-hold
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.down {
		http.Error(w, "upstream unavailable", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	switch r.Method + " " + r.URL.Path {
	case "POST /oauth2/token":
		h.exchanges++
		if h.edgeRefuses {
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(http.StatusForbidden)
			_, _ = io.WriteString(w, "<html>blocked</html>")
			return
		}
		// The secret travels in the form body only: a query string or a Basic
		// header would put it in an access log.
		if r.URL.RawQuery != "" || r.Header.Get("Authorization") != "" {
			http.Error(w, "the key left the form body", http.StatusTeapot)
			return
		}
		clientID := r.PostForm.Get("client_id")
		secret, known := h.secrets[clientID]
		if r.PostForm.Get("grant_type") != "client_credentials" || !known || secret != r.PostForm.Get("client_secret") {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = io.WriteString(w, `{"error":"invalid_client","error_description":"Client authentication failed (e.g., unknown client, no client authentication included, or unsupported authentication method)."}`)
			return
		}
		h.issued++
		token := fmt.Sprintf("ory_at_%d", h.issued)
		h.tokens[token] = clientID
		h.lastIssued = token
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": token, "expires_in": h.lifetime, "scope": "", "token_type": "bearer"})
	case "POST /admin/oauth2/introspect":
		h.introspections++
		h.lastIntrospected = r.PostForm.Get("token")
		clientID, active := h.tokens[h.lastIntrospected]
		if !active {
			_, _ = io.WriteString(w, `{"active":false}`)
			return
		}
		if h.introspectAs != "" {
			clientID = h.introspectAs
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"active": true, "sub": clientID, "client_id": clientID, "token_type": "Bearer", "token_use": "access_token"})
	default:
		http.NotFound(w, r)
	}
}

// keyGate is one bex-api replica's auth gate in front of hydra, on clock.
func keyGate(hydra *keyHydra, clock *stepClock, admission *AuthAdmission) *oryAuth {
	gate := newOryAuth(hydra.URL, "", "", hydra.URL, "", false, admission, nil, nil, "")
	gate.now, gate.cache.Now = clock.Now, clock.Now
	return gate
}

func newStepClock() *stepClock { return &stepClock{t: time.Unix(1_700_000_000, 0)} }

// probe sends one request with the bearer through the gate, from sourceIP.
func probe(gate *oryAuth, bearer, sourceIP string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	gate.middleware(echoIdentity).ServeHTTP(w, bearerRequest(bearer, sourceIP))
	return w
}

func wantCounts(t *testing.T, h *keyHydra, exchanges, introspections int, when string) {
	t.Helper()
	if gotX, gotI := h.counts(); gotX != exchanges || gotI != introspections {
		t.Fatalf("%s: Hydra saw %d exchanges and %d introspections, want %d and %d", when, gotX, gotI, exchanges, introspections)
	}
}

// wantExchanges is wantCounts for the token endpoint alone.
func wantExchanges(t *testing.T, h *keyHydra, exchanges int, when string) {
	t.Helper()
	if got, _ := h.counts(); got != exchanges {
		t.Fatalf("%s: Hydra's token endpoint was asked %d times, want %d", when, got, exchanges)
	}
}

func TestAPIKeyBearerAuthenticatesAsTheKeysClient(t *testing.T) {
	hydra := newKeyHydra(t)
	key := hydra.add(keyClientA)
	clock := newStepClock()
	touched := make(chan string, 8)
	gate := keyGate(hydra, clock, nil)
	gate.touch = func(clientID string) { touched <- clientID }

	w := probe(gate, key, "203.0.113.1")
	if w.Code != http.StatusOK || w.Body.String() != "oauth2/"+keyClientA {
		t.Fatalf("a valid key = %d %q, want 200 as the key's own client", w.Code, w.Body.String())
	}
	wantCounts(t, hydra, 1, 1, "first request")
	select {
	case got := <-touched:
		if got != keyClientA {
			t.Fatalf("last-used stamped %q, want the key's id", got)
		}
	case <-time.After(time.Second):
		t.Fatal("a key used as a bearer did not stamp its last-used")
	}

	// The next request spends nothing at Hydra: the token is remembered for the
	// key and its introspection for introspectionTTL.
	if w := probe(gate, key, "203.0.113.1"); w.Code != http.StatusOK {
		t.Fatalf("second request = %d", w.Code)
	}
	wantCounts(t, hydra, 1, 1, "second request")

	// Past introspectionTTL the token is introspected again, not exchanged again.
	clock.Advance(introspectionTTL + time.Second)
	if w := probe(gate, key, "203.0.113.1"); w.Code != http.StatusOK {
		t.Fatalf("after the introspection window = %d", w.Code)
	}
	wantCounts(t, hydra, 1, 2, "after the introspection window")
}

func TestAPIKeyBearerExchangesAgainBeforeItsTokenExpires(t *testing.T) {
	hydra := newKeyHydra(t)
	key := hydra.add(keyClientA)
	clock := newStepClock()
	gate := keyGate(hydra, clock, nil)

	if w := probe(gate, key, "203.0.113.1"); w.Code != http.StatusOK {
		t.Fatalf("first request = %d", w.Code)
	}
	first, _ := hydra.newestTokens()

	// One second short of the margin the remembered token is still the one used.
	lifetime := 899 * time.Second
	clock.Advance(lifetime - keyTokenMargin - time.Second)
	if w := probe(gate, key, "203.0.113.1"); w.Code != http.StatusOK {
		t.Fatalf("inside the token's use = %d", w.Code)
	}
	wantExchanges(t, hydra, 1, "inside the token's use")
	if _, sent := hydra.newestTokens(); sent != first {
		t.Fatalf("inside its use the key sent token %q, want its first token %q", sent, first)
	}

	// At the margin, a minute before Hydra would expire it, the key is exchanged
	// for a new token and the old one is never sent again.
	clock.Advance(time.Second)
	if w := probe(gate, key, "203.0.113.1"); w.Code != http.StatusOK {
		t.Fatalf("at the margin = %d", w.Code)
	}
	wantExchanges(t, hydra, 2, "at the margin")
	if issued, sent := hydra.newestTokens(); sent != issued || sent == first {
		t.Fatalf("at the margin the key sent token %q, want the new one %q and never the first %q again", sent, issued, first)
	}
}

func TestKeyTokenUse(t *testing.T) {
	for lifetime, want := range map[time.Duration]time.Duration{
		899 * time.Second: 839 * time.Second, // Hydra's 15 minutes: until a minute before expiry
		100 * time.Second: 50 * time.Second,  // shorter than twice the margin: half its life
		0:                 keyTokenMargin,    // no lifetime stated: briefly, never forever and never not at all
	} {
		if got := keyTokenUse(lifetime); got != want {
			t.Errorf("keyTokenUse(%s) = %s, want %s", lifetime, got, want)
		}
	}
}

// TestAPIKeyBearerFailuresAreOneAnswer: a wrong secret, an unknown client, a
// malformed key and random text are refused with the same status, body and
// challenge, so the answer cannot say which part was wrong.
func TestAPIKeyBearerFailuresAreOneAnswer(t *testing.T) {
	hydra := newKeyHydra(t)
	key := hydra.add(keyClientA)
	gate := keyGate(hydra, newStepClock(), nil)

	cases := []struct {
		name, bearer string
		exchanges    int // how many times Hydra's token endpoint may be asked
	}{
		{"wrong secret", key + "x", 1},
		{"unknown client", apikeys.BearerKey(keyClientB, "anything"), 1},
		{"no secret", "bex_" + keyClientA, 0},
		{"empty secret", apikeys.BearerKey(keyClientA, ""), 0},
		{"no client id", apikeys.BearerKey("", "anything"), 0},
		{"a character no key holds", apikeys.BearerKey(keyClientA, "se cret"), 0},
		{"prefix alone", "bex_", 0},
		{"random text", "not-a-key-and-not-a-token", 0},
	}
	var body, challenge string
	for i, tc := range cases {
		before, _ := hydra.counts()
		w := probe(gate, tc.bearer, "203.0.113.1")
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("%s = %d %s, want 401", tc.name, w.Code, w.Body.String())
		}
		if after, _ := hydra.counts(); after-before != tc.exchanges {
			t.Errorf("%s reached the token endpoint %d times, want %d", tc.name, after-before, tc.exchanges)
		}
		if i == 0 {
			body, challenge = w.Body.String(), w.Header().Get("WWW-Authenticate")
			continue
		}
		if w.Body.String() != body || w.Header().Get("WWW-Authenticate") != challenge {
			t.Errorf("%s answered %q (%q), want the one refusal %q (%q)", tc.name,
				w.Body.String(), w.Header().Get("WWW-Authenticate"), body, challenge)
		}
	}
	// The key itself still works after its neighbours were refused.
	if w := probe(gate, key, "203.0.113.1"); w.Code != http.StatusOK {
		t.Fatalf("the valid key after the refusals = %d", w.Code)
	}
}

// TestAPIKeyBearerRefusalIsRememberedBriefly: a refused key does not make
// Hydra hash its secret again until keyRefusedTTL has passed.
func TestAPIKeyBearerRefusalIsRememberedBriefly(t *testing.T) {
	hydra := newKeyHydra(t)
	wrong := hydra.add(keyClientA) + "x"
	clock := newStepClock()
	gate := keyGate(hydra, clock, nil)

	for i := range 5 {
		if w := probe(gate, wrong, "203.0.113.1"); w.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d = %d, want 401", i, w.Code)
		}
	}
	wantCounts(t, hydra, 1, 0, "five attempts with one wrong key")

	clock.Advance(keyRefusedTTL + time.Second)
	if w := probe(gate, wrong, "203.0.113.1"); w.Code != http.StatusUnauthorized {
		t.Fatalf("after the refusal lapsed = %d, want 401", w.Code)
	}
	wantCounts(t, hydra, 2, 0, "after the refusal lapsed")
}

// TestAPIKeyBearerHydraOutageIsNotARefusal: when Hydra cannot answer, a key is
// neither refused (401) nor remembered as refused.
func TestAPIKeyBearerHydraOutageIsNotARefusal(t *testing.T) {
	for _, tc := range []struct {
		name   string
		broken func(*keyHydra)
		fixed  func(*keyHydra)
	}{
		{"Hydra answers 503", func(h *keyHydra) { h.down = true }, func(h *keyHydra) { h.down = false }},
		{"a proxy in front of the token endpoint answers 403", func(h *keyHydra) { h.edgeRefuses = true }, func(h *keyHydra) { h.edgeRefuses = false }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			hydra := newKeyHydra(t)
			key := hydra.add(keyClientA)
			gate := keyGate(hydra, newStepClock(), nil)

			hydra.set(tc.broken)
			if w := probe(gate, key, "203.0.113.1"); w.Code != http.StatusServiceUnavailable {
				t.Fatalf("during the outage = %d %s, want 503", w.Code, w.Body.String())
			}
			hydra.set(tc.fixed)
			if w := probe(gate, key, "203.0.113.1"); w.Code != http.StatusOK {
				t.Fatalf("after the outage = %d %s, want 200: the outage was remembered as a refusal", w.Code, w.Body.String())
			}
		})
	}

	t.Run("introspection fails after a good exchange", func(t *testing.T) {
		hydra := newKeyHydra(t)
		key := hydra.add(keyClientA)
		clock := newStepClock()
		gate := keyGate(hydra, clock, nil)
		if w := probe(gate, key, "203.0.113.1"); w.Code != http.StatusOK {
			t.Fatalf("warm = %d", w.Code)
		}
		clock.Advance(introspectionTTL + time.Second)
		hydra.set(func(h *keyHydra) { h.down = true })
		if w := probe(gate, key, "203.0.113.1"); w.Code != http.StatusServiceUnavailable {
			t.Fatalf("introspection down = %d, want 503", w.Code)
		}
		hydra.set(func(h *keyHydra) { h.down = false })
		if w := probe(gate, key, "203.0.113.1"); w.Code != http.StatusOK {
			t.Fatalf("after the outage = %d, want 200", w.Code)
		}
		wantExchanges(t, hydra, 1, "after an introspection outage (the remembered token is kept)")
	})
}

// TestAPIKeyBearerRevokedThroughBexIsRefusedAtOnce: revoking a key through bex
// writes the shared tombstone, so every replica refuses the key at its next
// request, with no window (ADR012 §8d), while another key keeps working.
func TestAPIKeyBearerRevokedThroughBexIsRefusedAtOnce(t *testing.T) {
	hydra := newKeyHydra(t)
	departing, remaining := hydra.add(keyClientA), hydra.add(keyClientB)
	clock := newStepClock()
	shared := &memoryRevocations{}
	replicas := make([]*oryAuth, 2)
	for i := range replicas {
		replicas[i] = keyGate(hydra, clock, nil)
		replicas[i].revocations = shared
		for _, key := range []string{departing, remaining} {
			if w := probe(replicas[i], key, "203.0.113.1"); w.Code != http.StatusOK {
				t.Fatalf("replica %d warm = %d", i, w.Code)
			}
		}
	}

	// apikeys.RevokeAPIKey: the tombstone first, then the Hydra client.
	if err := shared.BumpOAuthRevocation(context.Background(), keyClientA, keyClientA); err != nil {
		t.Fatal(err)
	}
	hydra.deleteClient(keyClientA)

	for i, gate := range replicas {
		if w := probe(gate, departing, "203.0.113.1"); w.Code != http.StatusUnauthorized {
			t.Errorf("replica %d still accepts the revoked key: %d", i, w.Code)
		}
		if w := probe(gate, remaining, "203.0.113.1"); w.Code != http.StatusOK {
			t.Errorf("replica %d refuses the other key: %d", i, w.Code)
		}
	}
}

// TestAPIKeyBearerRevokedKeyWhoseClientOutlivesIt: bex revokes by tombstone
// first and deletes the Hydra client second, and the second step can fail. The
// key is then refused although Hydra still exchanges it, and the gate stops
// asking Hydra to hash its secret on every request.
func TestAPIKeyBearerRevokedKeyWhoseClientOutlivesIt(t *testing.T) {
	hydra := newKeyHydra(t)
	key := hydra.add(keyClientA)
	shared := &memoryRevocations{}
	gate := keyGate(hydra, newStepClock(), nil)
	gate.revocations = shared
	if err := shared.BumpOAuthRevocation(context.Background(), keyClientA, keyClientA); err != nil {
		t.Fatal(err)
	}
	for i := range 4 {
		if w := probe(gate, key, "203.0.113.1"); w.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d with a tombstoned key = %d, want 401", i, w.Code)
		}
	}
	wantExchanges(t, hydra, 1, "four requests with a tombstoned key")
}

// TestAPIKeyBearerDeletedAtHydraIsRefusedWithinTheRevocationLatency: a key
// whose client is deleted at Hydra directly, with no tombstone, is refused by
// every replica within the latency ADR012 §8d states for an access token, and
// never accepted again after its first refusal.
func TestAPIKeyBearerDeletedAtHydraIsRefusedWithinTheRevocationLatency(t *testing.T) {
	hydra := newKeyHydra(t)
	key := hydra.add(keyClientA)
	clock := newStepClock()
	shared := &memoryRevocations{}
	replicas := make([]*oryAuth, 2)
	for i := range replicas {
		replicas[i] = keyGate(hydra, clock, nil)
		replicas[i].revocations = shared
		if w := probe(replicas[i], key, "203.0.113.1"); w.Code != http.StatusOK {
			t.Fatalf("replica %d warm = %d", i, w.Code)
		}
	}
	hydra.deleteClient(keyClientA)

	refusedAfter := []time.Duration{-1, -1}
	for waited := time.Duration(0); waited <= documentedRevocationLatency+2*time.Second; waited += time.Second {
		for i, gate := range replicas {
			code := probe(gate, key, "203.0.113.1").Code
			switch {
			case code == http.StatusUnauthorized && refusedAfter[i] < 0:
				refusedAfter[i] = waited
			case code == http.StatusOK && refusedAfter[i] >= 0:
				t.Fatalf("replica %d accepted the key again %s after the deletion, having refused it at %s", i, waited, refusedAfter[i])
			}
		}
		clock.Advance(time.Second)
	}
	for i, after := range refusedAfter {
		if after < 0 || after > documentedRevocationLatency {
			t.Errorf("replica %d first refused the deleted key after %s, want within %s", i, after, documentedRevocationLatency)
		}
	}
}

// TestAPIKeyBearerSurvivesItsTokenBeingRevoked: the key is the credential, not
// the token bex holds for it. Revoking that token at Hydra while the client
// stands costs one more exchange and never a 401.
func TestAPIKeyBearerSurvivesItsTokenBeingRevoked(t *testing.T) {
	hydra := newKeyHydra(t)
	key := hydra.add(keyClientA)
	clock := newStepClock()
	gate := keyGate(hydra, clock, nil)
	if w := probe(gate, key, "203.0.113.1"); w.Code != http.StatusOK {
		t.Fatalf("warm = %d", w.Code)
	}
	hydra.revokeTokens(keyClientA)
	clock.Advance(introspectionTTL + time.Second)
	if w := probe(gate, key, "203.0.113.1"); w.Code != http.StatusOK {
		t.Fatalf("after its token was revoked the key = %d %s, want 200", w.Code, w.Body.String())
	}
	wantExchanges(t, hydra, 2, "after the key's token was revoked (one more for the dead token)")
}

// TestAPIKeyBearerCollapsesConcurrentExchanges: many requests arriving with a
// key the replica has no token for cost one exchange.
func TestAPIKeyBearerCollapsesConcurrentExchanges(t *testing.T) {
	hydra := newKeyHydra(t)
	key := hydra.add(keyClientA)
	gate := keyGate(hydra, newStepClock(), nil)
	hold := make(chan struct{})
	hydra.set(func(h *keyHydra) { h.hold = hold })

	const callers = 12
	codes := make(chan int, callers)
	for range callers {
		go func() { codes <- probe(gate, key, "203.0.113.1").Code }()
	}
	// Give every caller time to reach the exchange before Hydra answers one.
	time.Sleep(200 * time.Millisecond)
	close(hold)
	for range callers {
		if code := <-codes; code != http.StatusOK {
			t.Fatalf("a concurrent caller = %d, want 200", code)
		}
	}
	wantExchanges(t, hydra, 1, "twelve concurrent callers")
}

// TestAPIKeyBearerRepeatedFailuresAreRateLimited: wrong keys from one source
// are answered 401, the repeats from memory, until the source's failure budget
// is spent; then every bad attempt is 429 with Retry-After and one body,
// whether the key names a client that exists or not. A valid key the replica
// already holds a token for keeps working from that source throughout, another
// source is unaffected, and a key that needs Hydra works again from the
// limited source once its budget refills.
func TestAPIKeyBearerRepeatedFailuresAreRateLimited(t *testing.T) {
	hydra := newKeyHydra(t)
	key := hydra.add(keyClientA)
	const budget = 4
	// 60 a minute refills one token a second: slow enough that the limited
	// attempts below land on an empty budget, fast enough to watch it refill.
	gate := keyGate(hydra, newStepClock(), NewAuthAdmission(60, budget, 0, 0))
	wrongSecret := key + "x"
	unknownClient := apikeys.BearerKey(keyClientB, "anything")
	if w := probe(gate, key, "203.0.113.7"); w.Code != http.StatusOK {
		t.Fatalf("warm = %d", w.Code)
	}

	for i := range budget {
		if w := probe(gate, wrongSecret, "203.0.113.7"); w.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d = %d, want 401 inside the budget", i, w.Code)
		}
	}
	wantExchanges(t, hydra, 2, "the valid key once and the repeated wrong key once")

	limited := map[string]*httptest.ResponseRecorder{
		"a wrong secret":    probe(gate, wrongSecret, "203.0.113.7"),
		"an unknown client": probe(gate, unknownClient, "203.0.113.7"),
		"a malformed key":   probe(gate, "bex_!", "203.0.113.7"),
	}
	var body string
	for name, w := range limited {
		if w.Code != http.StatusTooManyRequests || w.Header().Get("Retry-After") == "" {
			t.Fatalf("%s past the budget = %d (Retry-After %q), want 429 with Retry-After", name, w.Code, w.Header().Get("Retry-After"))
		}
		if body == "" {
			body = w.Body.String()
		}
		if w.Body.String() != body {
			t.Errorf("%s past the budget answered %q, want the one body %q", name, w.Body.String(), body)
		}
		for _, leak := range []string{keyClientA, keyClientB, "secret", "client"} {
			if strings.Contains(w.Body.String(), leak) {
				t.Errorf("%s: the 429 body %q says %q", name, w.Body.String(), leak)
			}
		}
	}
	wantExchanges(t, hydra, 2, "bad keys from a limited source")

	// The key this replica already holds a token for needs nothing from Hydra,
	// so the source's spent budget does not touch it.
	if w := probe(gate, key, "203.0.113.7"); w.Code != http.StatusOK {
		t.Fatalf("the warm valid key from the limited source = %d, want 200", w.Code)
	}
	// A valid key that has to be exchanged is held back with the rest of the
	// source's upstream work, from this source only, until the budget refills.
	cold := hydra.add(keyClientB)
	if w := probe(gate, cold, "203.0.113.7"); w.Code != http.StatusTooManyRequests {
		t.Fatalf("a valid key needing an exchange from the limited source = %d, want 429", w.Code)
	}
	if w := probe(gate, cold, "198.51.100.9"); w.Code != http.StatusOK {
		t.Fatalf("the same key from another source = %d, want 200", w.Code)
	}
	hydra.deleteClient(keyClientB)
	third := hydra.add("cccccccc-cccc-4ccc-8ccc-cccccccccccc")
	var code int
	for range 50 {
		time.Sleep(100 * time.Millisecond)
		if code = probe(gate, third, "203.0.113.7").Code; code == http.StatusOK {
			break
		}
	}
	if code != http.StatusOK {
		t.Fatalf("a valid key from the limited source never worked again: %d", code)
	}
}

// TestAPIKeyBearerFailuresBehindCloudflareAddUp is the production shape of the
// test above. api.bex.co sits behind Cloudflare, and Cloudflare reaches the
// origin from many edges, so one client's attempts arrive through a Traefik pod
// with a different Cloudflare address in X-Forwarded-For each time and the same
// client in the edge's True-Client-IP. On production (w7/m426 t006, pin
// db48cde2bae9) 550 repeats of one wrong key from one client all answered 401:
// the budget keyed on the Cloudflare hop, a bucket per edge. Keyed on the
// client, the repeats spend one budget and end in 429, and another client
// behind the same edges keeps its own.
func TestAPIKeyBearerFailuresBehindCloudflareAddUp(t *testing.T) {
	hydra := newKeyHydra(t)
	key := hydra.add(keyClientA)
	const budget = 4
	admission := NewAuthAdmission(60, budget, 0, 0)
	trusted, err := core.ParseTrustedProxies("10.244.0.0/16")
	if err != nil {
		t.Fatal(err)
	}
	admission.TrustedProxies = trusted
	gate := keyGate(hydra, newStepClock(), admission)
	viaEdge := func(bearer, edge, client string) int {
		r := bearerRequest(bearer, "10.244.3.17") // a Traefik pod
		r.Header.Set("X-Forwarded-For", edge)
		r.Header.Set("True-Client-IP", client)
		w := httptest.NewRecorder()
		gate.middleware(echoIdentity).ServeHTTP(w, r)
		return w.Code
	}
	const client, neighbour = "2001:db8:45f0:963f::131", "198.51.100.23"
	wrongSecret := key + "x"
	for i := range budget {
		if code := viaEdge(wrongSecret, fmt.Sprintf("172.70.%d.2", i), client); code != http.StatusUnauthorized {
			t.Fatalf("attempt %d = %d, want 401 inside the budget", i, code)
		}
	}
	if code := viaEdge(wrongSecret, "172.71.200.9", client); code != http.StatusTooManyRequests {
		t.Fatalf("the client's next attempt through a new Cloudflare edge = %d, want 429: the budget is the client's, not the edge's", code)
	}
	if code := viaEdge(key, "172.70.0.2", neighbour); code != http.StatusOK {
		t.Fatalf("another client behind the same edge = %d, want 200", code)
	}
}

// TestAPIKeyBearerGuessesAtOneClientAreBounded: distinct wrong secrets for one
// client id, each from a new source so no source budget stops them, reach
// Hydra keyFailuresPerMinute times and are then shed with 429 before it. The
// key's owner, whose key this replica already knows, is not locked out.
func TestAPIKeyBearerGuessesAtOneClientAreBounded(t *testing.T) {
	hydra := newKeyHydra(t)
	key := hydra.add(keyClientA)
	other := hydra.add(keyClientB)
	clock := newStepClock()
	gate := keyGate(hydra, clock, NewAuthAdmission(600, 100, 0, 0))
	registry := prometheus.NewRegistry()
	gate.metrics = NewOriginMetrics(registry)
	if w := probe(gate, key, "198.51.100.1"); w.Code != http.StatusOK {
		t.Fatalf("warm = %d", w.Code)
	}

	guess := func(i int) *httptest.ResponseRecorder {
		return probe(gate, apikeys.BearerKey(keyClientA, fmt.Sprintf("guess-%d", i)), fmt.Sprintf("203.0.113.%d", i+1))
	}
	for i := range keyFailuresPerMinute {
		if w := guess(i); w.Code != http.StatusUnauthorized {
			t.Fatalf("guess %d = %d, want 401 inside the client's budget", i, w.Code)
		}
	}
	before, _ := hydra.counts()
	for i := keyFailuresPerMinute; i < keyFailuresPerMinute+5; i++ {
		if w := guess(i); w.Code != http.StatusTooManyRequests {
			t.Fatalf("guess %d = %d, want 429 past the client's budget", i, w.Code)
		}
	}
	if after, _ := hydra.counts(); after != before {
		t.Fatalf("guesses past the client's budget reached Hydra %d times", after-before)
	}
	// Each is a 429 the gate answered, counted under the budget that refused it.
	if got := counterValue(t, registry, "bex_api_auth_admission_sheds_total",
		map[string]string{"credential": credentialBearer, "cause": shedKeyID, "caller": callerUnverified}); got != 5 {
		t.Errorf("sheds counted under cause %q = %v, want 5", shedKeyID, got)
	}

	// The owner's remembered token dies (revoked at Hydra, the client left
	// standing), so the key must exchange again: it may, because the replica
	// knows this key.
	hydra.revokeTokens(keyClientA)
	clock.Advance(introspectionTTL + time.Second)
	if w := probe(gate, key, "198.51.100.1"); w.Code != http.StatusOK {
		t.Fatalf("the owner's key with a dead token during the guessing = %d %s, want 200", w.Code, w.Body.String())
	}
	// The same once the token's use has simply ended.
	clock.Advance(899 * time.Second)
	if w := probe(gate, key, "198.51.100.1"); w.Code != http.StatusOK {
		t.Fatalf("the owner's key with a lapsed token during the guessing = %d %s, want 200", w.Code, w.Body.String())
	}
	// Another client's key is untouched by this one's budget.
	if w := probe(gate, other, "198.51.100.2"); w.Code != http.StatusOK {
		t.Fatalf("another client's key = %d, want 200", w.Code)
	}
}

// TestShedKeysAllocateNothing: a source that has spent its failure budget and
// keeps sending well-formed keys for client ids the replica has never seen is
// shed before a per-client-id budget is created for any of them (the rule
// authadmission.go states for every shed request).
func TestShedKeysAllocateNothing(t *testing.T) {
	hydra := newKeyHydra(t)
	// One failure every ten seconds: the budget stays spent for the test.
	gate := keyGate(hydra, newStepClock(), NewAuthAdmission(6, 2, 0, 0))
	for i := range 2 {
		if w := probe(gate, apikeys.BearerKey(fmt.Sprintf("spend-%d", i), "x"), "203.0.113.9"); w.Code != http.StatusUnauthorized {
			t.Fatalf("spending the budget, attempt %d = %d", i, w.Code)
		}
	}
	before := gate.keys.failures.Entries()
	for i := range 50 {
		if w := probe(gate, apikeys.BearerKey(fmt.Sprintf("fresh-%d", i), "x"), "203.0.113.9"); w.Code != http.StatusTooManyRequests {
			t.Fatalf("a key from the spent source = %d, want 429", w.Code)
		}
	}
	if after := gate.keys.failures.Entries(); after != before {
		t.Fatalf("50 shed keys created %d per-client-id budgets, want none", after-before)
	}
	wantExchanges(t, hydra, 2, "after the budget was spent")
}

// TestAPIKeyBearerTokenMustBeTheKeysOwnClient: the identity a key acts as is
// the one its id names. A token that introspects as any other client is not
// accepted for it, whatever the token endpoint handed back.
func TestAPIKeyBearerTokenMustBeTheKeysOwnClient(t *testing.T) {
	hydra := newKeyHydra(t)
	key := hydra.add(keyClientA)
	hydra.set(func(h *keyHydra) { h.introspectAs = keyClientB })
	gate := keyGate(hydra, newStepClock(), nil)
	if w := probe(gate, key, "203.0.113.1"); w.Code != http.StatusUnauthorized {
		t.Fatalf("a key whose token belongs to another client = %d %q, want 401", w.Code, w.Body.String())
	}
}

// TestAPIKeyBearerPassesTheFreshBearerValidator: the durable-credential mint
// verbs re-validate the caller's bearer uncached. For a key that is its access
// token, which stops validating the moment the key's client is gone.
func TestAPIKeyBearerPassesTheFreshBearerValidator(t *testing.T) {
	hydra := newKeyHydra(t)
	key := hydra.add(keyClientA)
	gate := keyGate(hydra, newStepClock(), nil)
	var validate core.FreshBearerValidator
	handler := gate.middleware(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		validate, _ = core.FreshBearerValidatorFrom(r.Context())
	}))
	handler.ServeHTTP(httptest.NewRecorder(), bearerRequest(key, "203.0.113.1"))
	if validate == nil {
		t.Fatal("a key bearer got no fresh validator")
	}
	if err := validate(context.Background()); err != nil {
		t.Fatalf("fresh validation of a live key: %v", err)
	}
	_, introspections := hydra.counts()
	if introspections != 2 {
		t.Fatalf("fresh validation made %d introspections in total, want 2 (it must bypass the cache)", introspections)
	}
	hydra.deleteClient(keyClientA)
	if err := validate(context.Background()); err == nil {
		t.Fatal("fresh validation passed for a key whose client was deleted")
	}
}

// TestInvalidateForgetsTheKeysToken: logout and revoke hand the caller's
// identity to invalidate. For a key's client it drops the token remembered for
// the key, so the key's next request asks Hydra again, and leaves every other
// client's token alone.
func TestInvalidateForgetsTheKeysToken(t *testing.T) {
	hydra := newKeyHydra(t)
	key, other := hydra.add(keyClientA), hydra.add(keyClientB)
	gate := keyGate(hydra, newStepClock(), nil)
	for _, k := range []string{key, other} {
		if w := probe(gate, k, "203.0.113.1"); w.Code != http.StatusOK {
			t.Fatalf("warm = %d", w.Code)
		}
	}
	gate.invalidate(key, core.Identity{Subject: keyClientA, ClientID: keyClientA, Method: "oauth2"})
	if w := probe(gate, other, "203.0.113.1"); w.Code != http.StatusOK {
		t.Fatalf("another client's key after the invalidation = %d", w.Code)
	}
	wantExchanges(t, hydra, 2, "another client's key after the invalidation (its token is still remembered)")
	if w := probe(gate, key, "203.0.113.1"); w.Code != http.StatusOK {
		t.Fatalf("the invalidated key, its client still standing = %d", w.Code)
	}
	wantExchanges(t, hydra, 3, "the invalidated key's next request (its token was forgotten)")
}

// TestAPIKeyBearerNeedsATokenEndpoint: without BEX_OAUTH_ISSUER there is
// nowhere to exchange a key, so it is one more string Hydra does not know.
func TestAPIKeyBearerNeedsATokenEndpoint(t *testing.T) {
	hydra := newKeyHydra(t)
	key := hydra.add(keyClientA)
	gate := newOryAuth(hydra.URL, "", "", "", "", false, nil, nil, nil, "")
	if w := probe(gate, key, "203.0.113.1"); w.Code != http.StatusUnauthorized {
		t.Fatalf("a key with no issuer configured = %d, want 401", w.Code)
	}
	wantCounts(t, hydra, 0, 1, "no issuer")
}

// TestAPIKeyBearerLeavesOtherBearersAlone: a bearer without the key prefix is
// introspected as it always was and never reaches the token endpoint.
func TestAPIKeyBearerLeavesOtherBearersAlone(t *testing.T) {
	hydra := newKeyHydra(t)
	key := hydra.add(keyClientA)
	gate := keyGate(hydra, newStepClock(), nil)
	if w := probe(gate, key, "203.0.113.1"); w.Code != http.StatusOK {
		t.Fatalf("warm = %d", w.Code)
	}
	// The access token the exchange produced is itself a valid bearer: the
	// client_credentials exchange a caller does for themselves keeps working.
	token, _ := hydra.newestTokens()
	if w := probe(gate, token, "203.0.113.1"); w.Code != http.StatusOK || w.Body.String() != "oauth2/"+keyClientA {
		t.Fatalf("the exchanged access token as a bearer = %d %q", w.Code, w.Body.String())
	}
	if w := probe(gate, "ory_at_unknown", "203.0.113.1"); w.Code != http.StatusUnauthorized {
		t.Fatalf("an unknown access token = %d, want 401", w.Code)
	}
	wantExchanges(t, hydra, 1, "plain bearers (they never reach the token endpoint)")
}

// TestAPIKeyBearerNeverLeavesTheGate: across accepted, refused, limited and
// failing requests, neither the key nor its secret appears in a log line, an
// error, a response, or a metric label, and the failures are counted by reason.
func TestAPIKeyBearerNeverLeavesTheGate(t *testing.T) {
	hydra := newKeyHydra(t)
	key := hydra.add(keyClientA)
	_, secret, _ := apikeys.ParseBearerKey(key)
	wrong := key + "x"

	var logged bytes.Buffer
	previous := log.Writer()
	log.SetOutput(&logged)
	t.Cleanup(func() { log.SetOutput(previous) })

	registry := prometheus.NewRegistry()
	metrics := NewOriginMetrics(registry)
	clock := newStepClock()
	// Six a minute: the source's four failures are not refilled during the test.
	gate := keyGate(hydra, clock, NewAuthAdmission(6, 4, 0, 0))
	gate.metrics = metrics

	var responses strings.Builder
	record := func(bearer, source string) {
		w := probe(gate, bearer, source)
		fmt.Fprintf(&responses, "%d %v %s\n", w.Code, w.Header(), w.Body.String())
	}
	record(key, "203.0.113.1")           // accepted
	record(wrong, "203.0.113.1")         // refused by Hydra
	record(wrong, "203.0.113.1")         // refused from memory
	record("bex_"+secret, "203.0.113.1") // malformed, carrying the secret
	record(wrong, "203.0.113.1")         // refused from memory
	record(wrong, "203.0.113.1")         // limited: the source's budget is spent
	hydra.set(func(h *keyHydra) { h.down = true })
	clock.Advance(900 * time.Second)
	record(key, "198.51.100.1") // the exchange fails upstream
	if _, _, err := gate.authenticateKey(bearerRequest(key, "198.51.100.2"), key); err == nil {
		t.Fatal("an exchange against a Hydra that is down returned no error")
	} else {
		fmt.Fprintf(&responses, "error: %v\n", err)
	}

	families, err := registry.Gather()
	if err != nil {
		t.Fatal(err)
	}
	var series strings.Builder
	counted := map[string]float64{}
	for _, family := range families {
		for _, metric := range family.GetMetric() {
			for _, label := range metric.GetLabel() {
				fmt.Fprintf(&series, "%s{%s=%q}\n", family.GetName(), label.GetName(), label.GetValue())
				if family.GetName() == "bex_api_apikey_bearer_failures_total" && label.GetName() == "reason" {
					counted[label.GetValue()] = metric.GetCounter().GetValue()
				}
			}
		}
	}
	for where, text := range map[string]string{"the log": logged.String(), "a response or error": responses.String(), "a metric label": series.String()} {
		for what, material := range map[string]string{"the key": key, "the secret": secret, "the client id": keyClientA} {
			if what == "the client id" && where != "a metric label" {
				continue // the id is not secret; it only must not become a series
			}
			if strings.Contains(text, material) {
				t.Errorf("%s carries %s:\n%s", where, what, text)
			}
		}
	}
	// The one 429 is also a shed, by the source budget, of a credential this
	// replica never resolved.
	if got := counterValue(t, registry, "bex_api_auth_admission_sheds_total",
		map[string]string{"credential": credentialBearer, "cause": shedSource, "caller": callerUnverified}); got != 1 {
		t.Errorf("sheds counted under cause %q = %v, want 1", shedSource, got)
	}
	want := map[string]float64{keyFailureRefused: 1, keyFailureCached: 2, keyFailureMalformed: 1, keyFailureLimited: 1}
	for reason, n := range want {
		if counted[reason] != n {
			t.Errorf("bex_api_apikey_bearer_failures_total{reason=%q} = %v, want %v (all: %v)", reason, counted[reason], n, counted)
		}
	}
}
