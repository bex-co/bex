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
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/apikeys"
	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// auth_apikey_surfaces_test.go drives the composed server (docs/ADR012-auth.md
// §8e, w7/m426): a key minted through the API is one string that authenticates
// REST, GraphQL and MCP as the key's own workspace machine identity, cannot
// reach another workspace, is named by its id in the audit trail, and stops
// working on every surface the moment it is revoked.

// hydraKeys is apikeys.APIKeyStore over keyHydra, so a key the API mints is a
// client the fake Hydra exchanges and a key the API revokes is one it deletes.
type hydraKeys struct {
	hydra *keyHydra
	mu    sync.Mutex
	next  []string // the client ids Hydra hands out, in order
	live  map[string]apikeys.APIKey
}

func (s *hydraKeys) Create(_ context.Context, name, createdBy string, _ core.SignIn) (apikeys.APIKey, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	id := s.next[0]
	s.next = s.next[1:]
	_, secret, _ := apikeys.ParseBearerKey(s.hydra.add(id))
	if s.live == nil {
		s.live = map[string]apikeys.APIKey{}
	}
	s.live[id] = apikeys.APIKey{ID: id, Name: name, CreatedBy: createdBy}
	return apikeys.APIKey{ID: id, Name: name, Secret: secret, CreatedBy: createdBy}, nil
}

func (s *hydraKeys) List(context.Context) ([]apikeys.APIKey, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []apikeys.APIKey{}
	for _, key := range s.live {
		out = append(out, key)
	}
	return out, nil
}

func (s *hydraKeys) Delete(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.live[id]; !ok {
		return core.ErrNotFound
	}
	delete(s.live, id)
	s.hydra.deleteClient(id)
	return nil
}

func (*hydraKeys) Touch(context.Context, string, time.Time) error  { return nil }
func (*hydraKeys) KeyOwner(context.Context, string) (string, bool) { return "", false }

// keyWorkspaces is the control plane's part in a key: which workspace each
// subject belongs to (the binding a mint writes), the OpenFGA answer that
// follows from it, and the revocation tombstone an unbind leaves.
type keyWorkspaces struct {
	mu          sync.Mutex
	of          map[string]string // subject -> workspace
	revocations *memoryRevocations
}

func (k *keyWorkspaces) workspace(subject string) (string, bool) {
	k.mu.Lock()
	defer k.mu.Unlock()
	ws, ok := k.of[subject]
	return ws, ok
}

func (k *keyWorkspaces) Tenant(_ context.Context, id core.Identity) (string, bool) {
	return k.workspace(id.Subject)
}

func (k *keyWorkspaces) IsMember(_ context.Context, id core.Identity, tenantID string) (bool, error) {
	ws, ok := k.workspace(id.Subject)
	return ok && ws == tenantID, nil
}

// Check allows every relation on the subject's own workspace and nothing else.
func (k *keyWorkspaces) Check(_ context.Context, subject, _, object string) (bool, error) {
	ws, ok := k.workspace(strings.TrimPrefix(subject, "user:"))
	return ok && object == core.WorkspaceObject(ws), nil
}

func (k *keyWorkspaces) BindKey(_ context.Context, clientID, tenantID string) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.of[clientID] = tenantID
	return nil
}

// UnbindKey does what the Postgres binding does: the binding goes and the
// shared tombstone is written, which every replica reads on its next request.
func (k *keyWorkspaces) UnbindKey(ctx context.Context, clientID string) error {
	k.mu.Lock()
	delete(k.of, clientID)
	k.mu.Unlock()
	return k.revocations.BumpOAuthRevocation(ctx, clientID, clientID)
}

func (k *keyWorkspaces) TenantForKey(_ context.Context, clientID string) (string, bool) {
	return k.workspace(clientID)
}

// workspaceApp is a service in its workspace's own namespace (ADR043), where
// a workspace-scoped list looks for it.
func workspaceApp(name, tenantID string) *appv1alpha1.App {
	app := ownedApp(name, tenantID)
	app.Namespace = tenantID
	return app
}

// request is do with arbitrary headers, for a session cookie or MCP's Accept.
func request(h http.Handler, method, path, body string, headers map[string]string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	for name, value := range headers {
		r.Header.Set(name, value)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

// surfaceCalls is one read on each surface with the given bearer: REST's
// service list, the same list over GraphQL, and an MCP tools/call.
func surfaceCalls(h http.Handler, bearer string) map[string]*httptest.ResponseRecorder {
	auth := map[string]string{"Authorization": "Bearer " + bearer, "Content-Type": "application/json"}
	mcp := map[string]string{"Authorization": "Bearer " + bearer, "Content-Type": "application/json", "Accept": "application/json, text/event-stream"}
	return map[string]*httptest.ResponseRecorder{
		"REST":    request(h, http.MethodGet, "/v1/services", "", auth),
		"GraphQL": request(h, http.MethodPost, "/graphql", `{"query":"{ services { name } }"}`, auth),
		"MCP":     request(h, http.MethodPost, "/mcp", `{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"list_services","arguments":{}}}`, mcp),
	}
}

func TestAPIKeyBearerOnEverySurface(t *testing.T) {
	const sessionCookie = "ory_kratos_session=live" // fakeKratos: identity-1
	hydra := newKeyHydra(t)
	kratos := fakeKratos(t)
	revocations := &memoryRevocations{}
	workspaces := &keyWorkspaces{of: map[string]string{"identity-1": "tea-a"}, revocations: revocations}
	audit := &fakeAuditSink{}
	base := &core.Base{
		Client:    fakeClient(workspaceApp("alpha", "tea-a"), workspaceApp("beta", "tea-b")),
		Namespace: "default",
		Authz:     workspaces,
		Workspace: workspaces,
		Audit:     audit,
	}
	srv := NewServer(base, Deps{
		APIKeys:          &hydraKeys{hydra: hydra, next: []string{keyClientA}},
		KeyBinder:        workspaces,
		OAuthRevocations: revocations,
	})
	srv.HydraAdminURL, srv.OAuthIssuer, srv.KratosURL = hydra.URL, hydra.URL, kratos.URL
	h := buildHandler(t, srv)
	session := map[string]string{"Cookie": sessionCookie, "Content-Type": "application/json"}

	// A person mints the key. The create answer carries it once, as one string
	// composed from the id and secret that still work for the exchange.
	created := request(h, http.MethodPost, "/v1/api-keys", `{"name":"ci"}`, session)
	var minted struct{ ID, Secret, Key string }
	if err := json.Unmarshal(created.Body.Bytes(), &minted); err != nil || created.Code != http.StatusCreated {
		t.Fatalf("mint = %d %s (%v)", created.Code, created.Body.String(), err)
	}
	if minted.ID != keyClientA || minted.Key == "" || minted.Key != apikeys.BearerKey(minted.ID, minted.Secret) {
		t.Fatalf("mint answered id %q and a key that is not its id and secret in bearer form", minted.ID)
	}
	if listed := request(h, http.MethodGet, "/v1/api-keys", "", session); listed.Code != http.StatusOK ||
		strings.Contains(listed.Body.String(), minted.Secret) || strings.Contains(listed.Body.String(), `"key"`) {
		t.Fatalf("list = %d and carries the key or its secret: %s", listed.Code, listed.Body.String())
	}

	// The key alone authenticates every surface, as its own workspace: it sees
	// tea-a's service and not tea-b's.
	for surface, w := range surfaceCalls(h, minted.Key) {
		body := w.Body.String()
		if w.Code != http.StatusOK || !strings.Contains(body, "alpha") || strings.Contains(body, "beta") ||
			strings.Contains(body, `"errors"`) || strings.Contains(body, `"isError":true`) {
			t.Errorf("%s with the key = %d %s, want 200 listing alpha only", surface, w.Code, body)
		}
	}
	bearer := map[string]string{"Authorization": "Bearer " + minted.Key}
	// The Cloudflare-shaped object storage routes sit behind the same gate, so
	// wrangler's CLOUDFLARE_API_TOKEN takes the key too: the request passes
	// authentication and gets the route's own answer, while a wrong key gets
	// Cloudflare's authentication error.
	const buckets = "/cloudflare/client/v4/accounts/tea-a/r2/buckets"
	if w := request(h, http.MethodGet, buckets, "", bearer); w.Code == http.StatusUnauthorized {
		t.Errorf("the Cloudflare-shaped route refused the key: %d %s", w.Code, w.Body.String())
	}
	if w := request(h, http.MethodGet, buckets, "", map[string]string{"Authorization": "Bearer " + minted.Key + "x"}); w.Code != http.StatusUnauthorized {
		t.Errorf("the Cloudflare-shaped route with a wrong key = %d %s, want 401", w.Code, w.Body.String())
	}
	if w := request(h, http.MethodGet, "/v1/services/beta", "", bearer); w.Code != http.StatusForbidden && w.Code != http.StatusNotFound {
		t.Errorf("the key read another workspace's service: %d %s", w.Code, w.Body.String())
	}
	if w := request(h, http.MethodGet, "/v1/services?ownerId=tea-b", "", bearer); w.Code != http.StatusForbidden {
		t.Errorf("the key listed another workspace: %d %s, want 403", w.Code, w.Body.String())
	}

	// A write is audited under the key's id, and nothing in the trail holds the
	// key or its secret.
	audit.events = nil
	if w := request(h, http.MethodPost, "/v1/services/alpha/suspend", "", bearer); w.Code >= 300 {
		t.Fatalf("suspend with the key = %d %s", w.Code, w.Body.String())
	}
	if len(audit.events) == 0 {
		t.Fatal("a write with the key left no audit event")
	}
	for _, event := range audit.events {
		if event.Caller != keyClientA || event.CallerMethod != "oauth2" {
			t.Errorf("audit event %s names caller %q (%s), want the key's id", event.Verb, event.Caller, event.CallerMethod)
		}
		if row := fmt.Sprintf("%+v", event); strings.Contains(row, minted.Secret) || strings.Contains(row, minted.Key) {
			t.Errorf("audit event %s carries key material: %s", event.Verb, row)
		}
	}

	// The exchange a caller does for themselves still works with the same key.
	exchange, err := http.PostForm(hydra.URL+"/oauth2/token", map[string][]string{
		"grant_type": {"client_credentials"}, "client_id": {minted.ID}, "client_secret": {minted.Secret},
	})
	if err != nil {
		t.Fatal(err)
	}
	var token struct {
		AccessToken string `json:"access_token"`
	}
	_ = json.NewDecoder(exchange.Body).Decode(&token)
	_ = exchange.Body.Close()
	if w := do(t, h, http.MethodGet, "/v1/services", token.AccessToken, ""); w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "alpha") {
		t.Fatalf("an exchanged access token = %d %s, want 200", w.Code, w.Body.String())
	}

	// Revoked through the API, the key is refused on every surface at the next
	// request, and so is the token exchanged from it.
	if w := request(h, http.MethodDelete, "/v1/api-keys/"+minted.ID, "", session); w.Code != http.StatusNoContent {
		t.Fatalf("revoke = %d %s", w.Code, w.Body.String())
	}
	var refusal string
	for surface, w := range surfaceCalls(h, minted.Key) {
		if w.Code != http.StatusUnauthorized {
			t.Errorf("%s with the revoked key = %d %s, want 401", surface, w.Code, w.Body.String())
		}
		if refusal == "" {
			refusal = w.Body.String()
		}
		if w.Body.String() != refusal {
			t.Errorf("%s refused the revoked key with %q, the others with %q", surface, w.Body.String(), refusal)
		}
	}
	if w := do(t, h, http.MethodGet, "/v1/services", token.AccessToken, ""); w.Code != http.StatusUnauthorized {
		t.Errorf("the revoked key's exchanged token = %d, want 401", w.Code)
	}
}

// TestAPIKeyBearerLogoutRevokesTheKey: `POST /v1/oauth/revoke` is the CLI's
// logout. Called with a key it revokes that key (the machine branch it always
// had), never forwards the key to Hydra's revocation endpoint, and the key is
// refused afterwards.
func TestAPIKeyBearerLogoutRevokesTheKey(t *testing.T) {
	hydra := newKeyHydra(t)
	revocations := &memoryRevocations{}
	workspaces := &keyWorkspaces{of: map[string]string{}, revocations: revocations}
	store := &hydraKeys{hydra: hydra, next: []string{keyClientA}}
	minted, err := store.Create(context.Background(), "ci", "identity-1", core.SignInPassword)
	if err != nil {
		t.Fatal(err)
	}
	_ = workspaces.BindKey(context.Background(), minted.ID, "tea-a")
	key := apikeys.BearerKey(minted.ID, minted.Secret)

	base := &core.Base{Client: fakeClient(), Namespace: "default", Authz: workspaces, Workspace: workspaces}
	srv := NewServer(base, Deps{APIKeys: store, KeyBinder: workspaces, OAuthRevocations: revocations})
	srv.HydraAdminURL, srv.OAuthIssuer = hydra.URL, hydra.URL
	h := buildHandler(t, srv)

	if w := do(t, h, http.MethodGet, "/v1/services", key, ""); w.Code != http.StatusOK {
		t.Fatalf("before logout = %d %s", w.Code, w.Body.String())
	}
	if w := do(t, h, http.MethodPost, "/v1/oauth/revoke", key, ""); w.Code != http.StatusNoContent {
		t.Fatalf("logout with a key = %d %s, want 204", w.Code, w.Body.String())
	}
	if w := do(t, h, http.MethodGet, "/v1/services", key, ""); w.Code != http.StatusUnauthorized {
		t.Fatalf("after logout the key = %d, want 401", w.Code)
	}
	// keyHydra answers 404 for /oauth2/revoke, so the 204 above already shows
	// the key was never forwarded there. The token endpoint saw the key twice:
	// the exchange before the logout, and the refusal after it.
	wantExchanges(t, hydra, 2, "the exchange before the logout and the refusal after it")
}
