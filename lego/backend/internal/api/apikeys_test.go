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
	"testing"
)

// fakeKeyStore is the in-memory APIKeyStore for adapter tests.
type fakeKeyStore struct {
	keys map[string]APIKey
	n    int
}

func newFakeKeyStore() *fakeKeyStore { return &fakeKeyStore{keys: map[string]APIKey{}} }

func (f *fakeKeyStore) Create(_ context.Context, name string) (APIKey, error) {
	f.n++
	k := APIKey{ID: fmt.Sprintf("key-%d", f.n), Name: name, Secret: "s3cret", CreatedAt: "2026-01-01T00:00:00Z"}
	f.keys[k.ID] = APIKey{ID: k.ID, Name: k.Name, CreatedAt: k.CreatedAt} // stored without secret
	return k, nil
}

func (f *fakeKeyStore) List(context.Context) ([]APIKey, error) {
	out := make([]APIKey, 0, len(f.keys))
	for _, k := range f.keys {
		out = append(out, k)
	}
	return out, nil
}

func (f *fakeKeyStore) Delete(_ context.Context, id string) error {
	if _, ok := f.keys[id]; !ok {
		return ErrNotFound
	}
	delete(f.keys, id)
	return nil
}

func newKeyServer(t *testing.T) (http.Handler, *fakeKeyStore) {
	t.Helper()
	store := newFakeKeyStore()
	srv := &Server{
		Core:          &Core{Client: fakeAppClient(t), Namespace: "default", APIKeys: store},
		HydraAdminURL: fakeHydraURL(t),
	}
	h, err := srv.Handler()
	if err != nil {
		t.Fatalf("Handler: %v", err)
	}
	return h, store
}

func TestREST_APIKeyLifecycle(t *testing.T) {
	h, store := newKeyServer(t)

	// Create returns the secret exactly once.
	w := do(t, h, "POST", "/v1/api-keys", testToken, `{"name":"ci-agent"}`)
	if w.Code != 201 {
		t.Fatalf("create => 201, got %d (%s)", w.Code, w.Body.String())
	}
	var created APIKey
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if created.ID == "" || created.Name != "ci-agent" || created.Secret == "" {
		t.Fatalf("create must return id+name+secret: %+v", created)
	}

	// List never exposes secrets.
	var listed []APIKey
	decode(t, do(t, h, "GET", "/v1/api-keys", testToken, ""), &listed)
	if len(listed) != 1 || listed[0].ID != created.ID || listed[0].Secret != "" {
		t.Fatalf("list should show the key without its secret: %+v", listed)
	}

	// Revoke deletes; a second revoke is 404.
	if code := do(t, h, "DELETE", "/v1/api-keys/"+created.ID, testToken, "").Code; code != 204 {
		t.Fatalf("revoke => 204, got %d", code)
	}
	if len(store.keys) != 0 {
		t.Fatal("revoke should remove the key from the store")
	}
	if code := do(t, h, "DELETE", "/v1/api-keys/"+created.ID, testToken, "").Code; code != 404 {
		t.Fatalf("revoking a revoked key => 404, got %d", code)
	}

	// Bad input is a 400, not a 500.
	if code := do(t, h, "POST", "/v1/api-keys", testToken, `{"name":"  "}`).Code; code != 400 {
		t.Fatalf("blank name => 400, got %d", code)
	}
}

func TestREST_APIKeysUnavailableWithoutStore(t *testing.T) {
	srv := &Server{
		Core:          &Core{Client: fakeAppClient(t), Namespace: "default"}, // no APIKeys wired
		HydraAdminURL: fakeHydraURL(t),
	}
	h, err := srv.Handler()
	if err != nil {
		t.Fatalf("Handler: %v", err)
	}
	if code := do(t, h, "GET", "/v1/api-keys", testToken, "").Code; code != 503 {
		t.Fatalf("no store => 503, got %d", code)
	}
}

func TestGraphQL_APIKeys(t *testing.T) {
	h, store := newKeyServer(t)

	data := gql(t, h, `mutation { createApiKey(name:"dash") { id name secret } }`)
	key := data["createApiKey"].(map[string]any)
	if key["name"] != "dash" || key["secret"] == "" {
		t.Fatalf("createApiKey must return the secret once: %+v", key)
	}

	data = gql(t, h, `{ apiKeys { id name secret } }`)
	keys := data["apiKeys"].([]any)
	if len(keys) != 1 || keys[0].(map[string]any)["secret"] != "" {
		t.Fatalf("apiKeys list must omit secrets: %+v", keys)
	}

	data = gql(t, h, fmt.Sprintf(`mutation { revokeApiKey(id:%q) }`, key["id"]))
	if data["revokeApiKey"] != true {
		t.Fatalf("revokeApiKey should be true, got %+v", data)
	}
	if len(store.keys) != 0 {
		t.Fatal("revokeApiKey should delete from the store")
	}
}

func TestMCP_APIKeyTools(t *testing.T) {
	store := newFakeKeyStore()
	cs := mcpClient(t, &Core{Client: fakeAppClient(t), Namespace: "default", APIKeys: store})

	var created APIKey
	callTool(t, cs, "create_api_key", map[string]any{"name": "mcp-agent"}, &created)
	if created.ID == "" || created.Secret == "" {
		t.Fatalf("create_api_key must return id+secret: %+v", created)
	}

	var listed listAPIKeysResult
	callTool(t, cs, "list_api_keys", nil, &listed)
	if len(listed.APIKeys) != 1 || listed.APIKeys[0].Secret != "" {
		t.Fatalf("list_api_keys must show the key without its secret: %+v", listed)
	}

	var revoked revokeAPIKeyResult
	callTool(t, cs, "revoke_api_key", map[string]any{"keyId": created.ID}, &revoked)
	if !revoked.Revoked || len(store.keys) != 0 {
		t.Fatalf("revoke_api_key should delete the key: %+v, store=%d", revoked, len(store.keys))
	}
}

// fakeHydraAdmin fakes the client-management slice of Hydra's admin API for
// the production store implementation.
func fakeHydraAdmin(t *testing.T) *httptest.Server {
	t.Helper()
	// A platform client (like bex-bootstrap) that is NOT a bex API key: List
	// must hide it, Delete must refuse it.
	clients := map[string]hydraClient{
		"platform-client": {ClientID: "platform-client", ClientName: "bex bootstrap"},
	}
	n := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/admin/clients":
			var c hydraClient
			_ = json.NewDecoder(r.Body).Decode(&c)
			// bex keys are marker-stamped client_credentials machine clients —
			// reject anything else so a payload regression fails the test.
			if len(c.GrantTypes) != 1 || c.GrantTypes[0] != "client_credentials" ||
				c.AuthMethod != "client_secret_post" || !isAPIKey(c) {
				http.Error(w, "unexpected client shape", http.StatusBadRequest)
				return
			}
			n++
			c.ClientID = fmt.Sprintf("client-%d", n)
			c.ClientSecret = "generated-secret"
			clients[c.ClientID] = c
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(c)
		case r.Method == http.MethodGet && r.URL.Path == "/admin/clients":
			out := make([]hydraClient, 0, len(clients))
			for _, c := range clients {
				c.ClientSecret = "" // hydra omits secrets on list
				out = append(out, c)
			}
			_ = json.NewEncoder(w).Encode(out)
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/admin/clients/"):
			c, ok := clients[strings.TrimPrefix(r.URL.Path, "/admin/clients/")]
			if !ok {
				http.NotFound(w, r)
				return
			}
			c.ClientSecret = ""
			_ = json.NewEncoder(w).Encode(c)
		case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/admin/clients/"):
			id := strings.TrimPrefix(r.URL.Path, "/admin/clients/")
			if _, ok := clients[id]; !ok {
				http.NotFound(w, r)
				return
			}
			delete(clients, id)
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestHydraAPIKeysStore(t *testing.T) {
	store := NewHydraAPIKeys(fakeHydraAdmin(t).URL)
	ctx := context.Background()

	created, err := store.Create(ctx, "agent-1")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.ID == "" || created.Secret == "" || created.Name != "agent-1" {
		t.Fatalf("Create should return id+secret+name: %+v", created)
	}

	keys, err := store.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(keys) != 1 || keys[0].Secret != "" || keys[0].ID != created.ID {
		t.Fatalf("List should return only the bex-minted key, without secret: %+v", keys)
	}

	// Platform clients (no marker) are not API keys: invisible and irrevocable here.
	if err := store.Delete(ctx, "platform-client"); err != ErrNotFound {
		t.Fatalf("deleting a platform client must be ErrNotFound, got %v", err)
	}

	if err := store.Delete(ctx, created.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if err := store.Delete(ctx, created.ID); err != ErrNotFound {
		t.Fatalf("Delete of missing key should be ErrNotFound, got %v", err)
	}
}
