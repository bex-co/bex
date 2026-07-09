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

package apikeys

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/graphql-go/graphql"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// fakeKeyStore is the in-memory APIKeyStore for verb/adapter tests.
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
		return core.ErrNotFound
	}
	delete(f.keys, id)
	return nil
}

func newService(store APIKeyStore) *Service {
	return &Service{Base: &core.Base{Namespace: "default"}, APIKeys: store}
}

// fakeWorkspace is a map-backed core.WorkspaceResolver: identities not in the
// map resolve ok=false — an unbound/session-less caller, the case CreateAPIKey
// must refuse rather than mint an orphaned, tuple-less key.
type fakeWorkspace map[string]string

func (f fakeWorkspace) Tenant(_ context.Context, id core.Identity) (string, bool) {
	tid, ok := f[id.Subject]
	return tid, ok
}

// fakeBinder is the in-memory apikeys.KeyBinder for the Binding seam's tests —
// records bind/unbind calls and can be told to fail the next N binds, the way
// tests simulate an OpenFGA write failure during key mint (t002/t006: no
// orphaned Hydra client survives a failed bind).
type fakeBinder struct {
	failNext int
	bound    map[string]string // clientID -> tenantID
	unbound  []string
}

func newFakeBinder() *fakeBinder { return &fakeBinder{bound: map[string]string{}} }

func (b *fakeBinder) BindKey(_ context.Context, clientID, tenantID string) error {
	if b.failNext > 0 {
		b.failNext--
		return errors.New("fga write failed")
	}
	b.bound[clientID] = tenantID
	return nil
}

func (b *fakeBinder) UnbindKey(_ context.Context, clientID string) error {
	delete(b.bound, clientID)
	b.unbound = append(b.unbound, clientID)
	return nil
}

// --- Key→tenant binding (w1/m9) ---

func TestCreateAPIKeyBindsToCallerTenant(t *testing.T) {
	store := newFakeKeyStore()
	binder := newFakeBinder()
	svc := &Service{
		Base:    &core.Base{Namespace: "default", Workspace: fakeWorkspace{"identity-a": "tea-a"}},
		APIKeys: store,
		Binding: binder,
	}
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "identity-a", Method: "session"})

	created, err := svc.CreateAPIKey(ctx, "agent")
	if err != nil {
		t.Fatalf("CreateAPIKey: %v", err)
	}
	if binder.bound[created.ID] != "tea-a" {
		t.Errorf("bound[%s] = %q, want tea-a", created.ID, binder.bound[created.ID])
	}
}

func TestCreateAPIKeyNoTenantIsRefused(t *testing.T) {
	// Binding wired (store on) but the caller resolves to no tenant — a
	// session-less machine caller. Minting must refuse, not produce an
	// orphaned, tuple-less key nobody can ever bind later.
	store := newFakeKeyStore()
	binder := newFakeBinder()
	svc := &Service{
		Base:    &core.Base{Namespace: "default", Workspace: fakeWorkspace{}},
		APIKeys: store,
		Binding: binder,
	}
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "unbound-caller", Method: "oauth2"})

	if _, err := svc.CreateAPIKey(ctx, "agent"); !errors.Is(err, core.ErrBadRequest) {
		t.Errorf("no-tenant caller: want ErrBadRequest, got %v", err)
	}
	if len(store.keys) != 0 {
		t.Errorf("no Hydra client should have been minted, store has %d", len(store.keys))
	}
}

func TestCreateAPIKeyBindFailureRollsBackHydraClient(t *testing.T) {
	store := newFakeKeyStore()
	binder := &fakeBinder{failNext: 1, bound: map[string]string{}}
	svc := &Service{
		Base:    &core.Base{Namespace: "default", Workspace: fakeWorkspace{"identity-a": "tea-a"}},
		APIKeys: store,
		Binding: binder,
	}
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "identity-a", Method: "session"})

	if _, err := svc.CreateAPIKey(ctx, "agent"); err == nil {
		t.Fatal("bind failure must surface as an error")
	}
	// No orphaned credential: the Hydra client created before the failed bind
	// must be deleted, not left live with no tenant to authorize against.
	if len(store.keys) != 0 {
		t.Errorf("Hydra client left behind after bind failure: %d keys in store", len(store.keys))
	}
}

func TestRevokeAPIKeyUnbindsFromTenant(t *testing.T) {
	store := newFakeKeyStore()
	binder := newFakeBinder()
	svc := &Service{
		Base:    &core.Base{Namespace: "default", Workspace: fakeWorkspace{"identity-a": "tea-a"}},
		APIKeys: store,
		Binding: binder,
	}
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "identity-a", Method: "session"})

	created, err := svc.CreateAPIKey(ctx, "agent")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := svc.RevokeAPIKey(ctx, created.ID); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	if len(binder.unbound) != 1 || binder.unbound[0] != created.ID {
		t.Errorf("unbound = %v, want [%s]", binder.unbound, created.ID)
	}
}

func TestCreateAPIKeyNilBindingMintsUnbound(t *testing.T) {
	// Store off (Binding nil): keys mint unbound, byte-identical to before
	// tenant onboarding existed — no tenant lookup, no refusal.
	store := newFakeKeyStore()
	svc := newService(store)
	ctx := context.Background() // no identity at all — must not matter

	created, err := svc.CreateAPIKey(ctx, "agent")
	if err != nil || created.ID == "" {
		t.Fatalf("nil Binding: %v %+v", err, created)
	}
}

// --- Verb behavior ---

func TestAPIKeyVerbs(t *testing.T) {
	store := newFakeKeyStore()
	svc := newService(store)
	ctx := context.Background()

	created, err := svc.CreateAPIKey(ctx, "ci-agent")
	if err != nil || created.ID == "" || created.Secret == "" {
		t.Fatalf("create must return id+secret: %v %+v", err, created)
	}
	listed, _ := svc.ListAPIKeys(ctx)
	if len(listed) != 1 || listed[0].Secret != "" {
		t.Fatalf("list must omit secrets: %+v", listed)
	}
	if err := svc.RevokeAPIKey(ctx, created.ID); err != nil || len(store.keys) != 0 {
		t.Fatalf("revoke should delete: %v store=%d", err, len(store.keys))
	}
	// blank name => ErrBadRequest.
	if _, err := svc.CreateAPIKey(ctx, "  "); !errors.Is(err, core.ErrBadRequest) {
		t.Errorf("blank name => ErrBadRequest, got %v", err)
	}
	// nil store => ErrAPIKeysUnavailable on every verb.
	nostore := newService(nil)
	if _, err := nostore.ListAPIKeys(ctx); !errors.Is(err, core.ErrAPIKeysUnavailable) {
		t.Errorf("nil store => ErrAPIKeysUnavailable, got %v", err)
	}
}

// --- REST + GraphQL fragments ---

func serveREST(svc *Service, method, path, body string) *httptest.ResponseRecorder {
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	var r *http.Request
	if body == "" {
		r = httptest.NewRequest(method, path, nil)
	} else {
		r = httptest.NewRequest(method, path, strings.NewReader(body))
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, r)
	return rec
}

func TestRESTAPIKeyLifecycle(t *testing.T) {
	svc := newService(newFakeKeyStore())

	w := serveREST(svc, "POST", "/v1/api-keys", `{"name":"ci"}`)
	if w.Code != 201 {
		t.Fatalf("create => 201, got %d", w.Code)
	}
	var created APIKey
	_ = json.Unmarshal(w.Body.Bytes(), &created)
	if created.Secret == "" {
		t.Fatalf("create must return secret: %+v", created)
	}
	var listed []APIKey
	_ = json.Unmarshal(serveREST(svc, "GET", "/v1/api-keys", "").Body.Bytes(), &listed)
	if len(listed) != 1 || listed[0].Secret != "" {
		t.Fatalf("list omits secrets: %+v", listed)
	}
	if serveREST(svc, "DELETE", "/v1/api-keys/"+created.ID, "").Code != 204 {
		t.Error("revoke => 204")
	}
	if serveREST(svc, "DELETE", "/v1/api-keys/"+created.ID, "").Code != 404 {
		t.Error("revoke of missing => 404")
	}
	if serveREST(svc, "POST", "/v1/api-keys", `{"name":"  "}`).Code != 400 {
		t.Error("blank name => 400")
	}
	// nil store => 503.
	if serveREST(newService(nil), "GET", "/v1/api-keys", "").Code != 503 {
		t.Error("no store => 503")
	}
}

func TestGraphQLAPIKeys(t *testing.T) {
	store := newFakeKeyStore()
	svc := newService(store)
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
		Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: svc.GraphQLMutation()}),
	})
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	run := func(q string) map[string]any {
		res := graphql.Do(graphql.Params{Schema: schema, RequestString: q, Context: context.Background()})
		if len(res.Errors) > 0 {
			t.Fatalf("gql %q: %v", q, res.Errors)
		}
		return res.Data.(map[string]any)
	}

	key := run(`mutation { createApiKey(name:"dash") { id name secret } }`)["createApiKey"].(map[string]any)
	if key["secret"] == "" {
		t.Fatalf("createApiKey must return secret: %+v", key)
	}
	keys := run(`{ apiKeys { id secret } }`)["apiKeys"].([]any)
	if len(keys) != 1 || keys[0].(map[string]any)["secret"] != "" {
		t.Fatalf("apiKeys must omit secrets: %+v", keys)
	}
	if run(fmt.Sprintf(`mutation { revokeApiKey(id:%q) }`, key["id"]))["revokeApiKey"] != true {
		t.Fatal("revokeApiKey should be true")
	}
}

// --- Production Hydra store ---

// fakeHydraAdmin fakes the client-management slice of Hydra's admin API.
func fakeHydraAdmin(t *testing.T) *httptest.Server {
	t.Helper()
	// A platform client (not a bex API key): List hides it, Delete refuses it.
	clients := map[string]hydraClient{
		"platform-client": {ClientID: "platform-client", ClientName: "bex bootstrap"},
	}
	n := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/admin/clients":
			var c hydraClient
			_ = json.NewDecoder(r.Body).Decode(&c)
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
				c.ClientSecret = ""
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
	if err != nil || created.ID == "" || created.Secret == "" || created.Name != "agent-1" {
		t.Fatalf("Create: %v %+v", err, created)
	}
	keys, err := store.List(ctx)
	if err != nil || len(keys) != 1 || keys[0].Secret != "" || keys[0].ID != created.ID {
		t.Fatalf("List should return only the bex key without secret: %v %+v", err, keys)
	}
	// Platform clients are invisible + irrevocable here.
	if err := store.Delete(ctx, "platform-client"); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("deleting a platform client => ErrNotFound, got %v", err)
	}
	if err := store.Delete(ctx, created.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if err := store.Delete(ctx, created.ID); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("Delete of missing => ErrNotFound, got %v", err)
	}
}
