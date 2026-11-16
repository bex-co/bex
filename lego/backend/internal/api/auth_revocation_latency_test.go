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
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// documentedRevocationLatency is the number docs/ADR012-auth.md §8d states as
// the revocation latency of an OAuth access token revoked at Hydra: every bex-api
// replica refuses it within this long. Change the ADR and this together.
const documentedRevocationLatency = 5 * time.Second

// stepClock is a clock the test advances by hand.
type stepClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *stepClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *stepClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

// revocableHydra is an in-memory Hydra: it models the behavior of the real Hydra
// v26.2.0 admin and public APIs that bex-api depends on, which was measured on
// the production deployment on 2026-11-15 (two Hydra replicas on one Postgres):
//
//   - POST /admin/oauth2/introspect answers the shape recorded below, and
//     {"active":false} for a revoked or unknown token;
//   - every way of revoking a token takes effect on the very next introspection,
//     on every Hydra replica: RFC 7009 POST /oauth2/revoke with the access token
//     (200) or with the refresh token (200, revokes the whole chain), admin
//     DELETE /admin/oauth2/tokens?client_id= (204), admin
//     DELETE /admin/oauth2/auth/sessions/consent?subject=&client= (204), and
//     admin DELETE /admin/clients/{id} (204).
//
// There is no real Hydra in the backend suite's dependencies (Postgres, OpenFGA,
// OpenBao, Loki), so this stands in for it; the live check in the ADR repeats the
// revocation against the real one.
type revocableHydra struct {
	*httptest.Server
	mu      sync.Mutex
	grants  map[string]*hydraGrant // access or refresh token -> its consent chain
	clients map[string]bool
}

type hydraGrant struct {
	client, sub string
	revoked     bool
}

func newRevocableHydra(t *testing.T) *revocableHydra {
	t.Helper()
	h := &revocableHydra{grants: map[string]*hydraGrant{}, clients: map[string]bool{}}
	h.Server = httptest.NewServer(http.HandlerFunc(h.serve))
	t.Cleanup(h.Close)
	return h
}

// issue mints one grant: an access token and the refresh token of its chain.
func (h *revocableHydra) issue(client, sub, access, refresh string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	g := &hydraGrant{client: client, sub: sub}
	h.grants[access], h.grants[refresh] = g, g
	h.clients[client] = true
}

func (h *revocableHydra) serve(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	h.mu.Lock()
	defer h.mu.Unlock()
	switch {
	case r.Method == http.MethodPost && r.URL.Path == "/admin/oauth2/introspect":
		w.Header().Set("Content-Type", "application/json")
		g := h.grants[r.PostFormValue("token")]
		if g == nil || g.revoked || !h.clients[g.client] {
			_, _ = w.Write([]byte(`{"active":false}` + "\n"))
			return
		}
		_, _ = w.Write([]byte(`{"active":true,"scope":"openid offline_access bex.read bex.write","client_id":"` + g.client +
			`","sub":"` + g.sub + `","exp":4102444800,"iat":1700000000,"nbf":1700000000,"aud":[],"iss":"https://oauth.bex.co",` +
			`"token_type":"Bearer","token_use":"access_token"}` + "\n"))
	case r.Method == http.MethodPost && r.URL.Path == "/oauth2/revoke":
		if g := h.grants[r.PostFormValue("token")]; g != nil && g.client == r.PostFormValue("client_id") {
			g.revoked = true
		}
		w.WriteHeader(http.StatusOK)
	case r.Method == http.MethodDelete && r.URL.Path == "/admin/oauth2/tokens":
		for _, g := range h.grants {
			if g.client == r.URL.Query().Get("client_id") {
				g.revoked = true
			}
		}
		w.WriteHeader(http.StatusNoContent)
	case r.Method == http.MethodDelete && r.URL.Path == "/admin/oauth2/auth/sessions/consent":
		for _, g := range h.grants {
			if g.sub == r.URL.Query().Get("subject") && g.client == r.URL.Query().Get("client") {
				g.revoked = true
			}
		}
		w.WriteHeader(http.StatusNoContent)
	case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/admin/clients/"):
		delete(h.clients, strings.TrimPrefix(r.URL.Path, "/admin/clients/"))
		w.WriteHeader(http.StatusNoContent)
	default:
		http.NotFound(w, r)
	}
}

// revoke calls one of Hydra's own revocation endpoints, as an operator or a
// client talking to Hydra directly would: bex-api is not involved.
func (h *revocableHydra) revoke(t *testing.T, method, path string, form url.Values) {
	t.Helper()
	req, err := http.NewRequest(method, h.URL+path, strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer core.DrainClose(resp)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		t.Fatalf("%s %s = %d", method, path, resp.StatusCode)
	}
}

// TestRevokedAtHydraIsRefusedWithinTheRevocationLatency pins the revocation
// latency of an OAuth access token revoked at Hydra by any path that bypasses
// bex-api: on every replica, including one that served the token a moment
// earlier, it is refused within documentedRevocationLatency. The replicas share
// the production revocation marker, which a direct Hydra revocation never writes,
// so only the introspection cache window is under test. Revert introspectionTTL
// to the old 30 s and this fails.
func TestRevokedAtHydraIsRefusedWithinTheRevocationLatency(t *testing.T) {
	if introspectionTTL > documentedRevocationLatency {
		t.Fatalf("introspectionTTL = %s exceeds the documented revocation latency %s (docs/ADR012-auth.md §8d)",
			introspectionTTL, documentedRevocationLatency)
	}
	const (
		cli   = "429024F5E608930E2A65EF92591A25CC"
		human = "identity-1"
	)
	paths := []struct {
		name string
		do   func(t *testing.T, h *revocableHydra)
	}{
		{"revoke the access token", func(t *testing.T, h *revocableHydra) {
			h.revoke(t, http.MethodPost, "/oauth2/revoke", url.Values{"token": {"access-1"}, "client_id": {cli}})
		}},
		{"revoke the refresh token", func(t *testing.T, h *revocableHydra) {
			h.revoke(t, http.MethodPost, "/oauth2/revoke", url.Values{"token": {"refresh-1"}, "client_id": {cli}})
		}},
		{"delete the client's tokens", func(t *testing.T, h *revocableHydra) {
			h.revoke(t, http.MethodDelete, "/admin/oauth2/tokens?client_id="+cli, nil)
		}},
		{"revoke the consent session", func(t *testing.T, h *revocableHydra) {
			h.revoke(t, http.MethodDelete, "/admin/oauth2/auth/sessions/consent?subject="+human+"&client="+cli, nil)
		}},
		{"delete the client", func(t *testing.T, h *revocableHydra) {
			h.revoke(t, http.MethodDelete, "/admin/clients/"+cli, nil)
		}},
	}
	for _, p := range paths {
		t.Run(p.name, func(t *testing.T) {
			hydra := newRevocableHydra(t)
			hydra.issue(cli, human, "access-1", "refresh-1")
			clock := &stepClock{t: time.Unix(1_700_000_000, 0)}
			shared := &memoryRevocations{}
			replicas := make([]*oryAuth, 2)
			for i := range replicas {
				g := newOryAuth(hydra.URL, "", "", "", "", false, nil, nil, nil, "")
				g.revocations, g.now, g.cache.Now = shared, clock.Now, clock.Now
				replicas[i] = g
				if id, err := g.introspect(introspectReq(), "access-1"); err != nil || id.Subject != human {
					t.Fatalf("replica %d before the revocation = %+v, %v", i, id, err)
				}
			}

			// The worst case: the revocation lands just after both replicas
			// filled their caches, so each holds a full window of positive.
			p.do(t, hydra)

			// A replica with a cold cache asks Hydra and refuses at once.
			cold := newOryAuth(hydra.URL, "", "", "", "", false, nil, nil, nil, "")
			cold.revocations, cold.now, cold.cache.Now = shared, clock.Now, clock.Now
			if id, err := cold.introspect(introspectReq(), "access-1"); err != nil || id != (core.Identity{}) {
				t.Fatalf("cold replica after the revocation = %+v, %v; want refused", id, err)
			}

			refusedAt := make([]time.Duration, len(replicas))
			for i := range refusedAt {
				refusedAt[i] = -1
			}
			// Poll once a second for twice the documented window, so a refusal
			// that the cache later undoes is caught as well.
			for elapsed := time.Duration(0); elapsed <= 2*documentedRevocationLatency; elapsed += time.Second {
				for i, g := range replicas {
					id, err := g.introspect(introspectReq(), "access-1")
					if err != nil {
						t.Fatalf("replica %d at +%s: %v", i, elapsed, err)
					}
					switch {
					case id == (core.Identity{}) && refusedAt[i] < 0:
						refusedAt[i] = elapsed
					case id != (core.Identity{}) && refusedAt[i] >= 0:
						t.Fatalf("replica %d refused the token at +%s and accepted it again at +%s", i, refusedAt[i], elapsed)
					}
				}
				clock.Advance(time.Second)
			}
			for i, at := range refusedAt {
				if at < 0 {
					t.Errorf("replica %d still accepted the revoked token %s after the revocation; want a refusal within %s", i, 2*documentedRevocationLatency, documentedRevocationLatency)
				} else if at > documentedRevocationLatency {
					t.Errorf("replica %d refused the revoked token %s after the revocation; want within %s", i, at, documentedRevocationLatency)
				}
			}
		})
	}
}
