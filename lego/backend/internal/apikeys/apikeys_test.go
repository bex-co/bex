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
	"time"

	"github.com/graphql-go/graphql"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// fakeKeyStore is the in-memory APIKeyStore for verb/adapter tests.
type fakeKeyStore struct {
	keys map[string]APIKey
	n    int
}

func newFakeKeyStore() *fakeKeyStore { return &fakeKeyStore{keys: map[string]APIKey{}} }

func (f *fakeKeyStore) Create(_ context.Context, name, createdBy string) (APIKey, error) {
	f.n++
	k := APIKey{ID: fmt.Sprintf("key-%d", f.n), Name: name, Secret: "s3cret", CreatedAt: "2026-01-01T00:00:00Z", CreatedBy: createdBy}
	f.keys[k.ID] = APIKey{ID: k.ID, Name: k.Name, CreatedAt: k.CreatedAt, CreatedBy: createdBy} // stored without secret
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

func (f *fakeKeyStore) Touch(_ context.Context, id string, at time.Time) error {
	if k, ok := f.keys[id]; ok {
		k.LastUsedAt = at.UTC().Format(time.RFC3339)
		f.keys[id] = k
	}
	return nil
}

func (f *fakeKeyStore) KeyOwner(_ context.Context, id string) (string, bool) {
	k, ok := f.keys[id]
	return k.CreatedBy, ok && k.CreatedBy != ""
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

// IsMember: a map-backed caller belongs to exactly the one workspace it
// resolves to — the single-membership case every pre-w6/m14 test is written
// against. Multi-membership callers use a richer fake (see the m14 tests).
func (f fakeWorkspace) IsMember(_ context.Context, id core.Identity, tenantID string) (bool, error) {
	tid, ok := f[id.Subject]
	return ok && tid == tenantID, nil
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

func (b *fakeBinder) TenantForKey(_ context.Context, clientID string) (string, bool) {
	tid, ok := b.bound[clientID]
	return tid, ok
}

// multiWorkspace is a core.WorkspaceResolver for a caller who belongs to
// MULTIPLE workspaces (w6/m18's List/Revoke scoping tests need this — plain
// fakeWorkspace only ever resolves one tenant per identity). memberships[0] is
// the default (what Tenant returns absent an explicit core.WithWorkspace).
type multiWorkspace map[string][]string

func (f multiWorkspace) Tenant(_ context.Context, id core.Identity) (string, bool) {
	m := f[id.Subject]
	if len(m) == 0 {
		return "", false
	}
	return m[0], true
}

func (f multiWorkspace) IsMember(_ context.Context, id core.Identity, tenantID string) (bool, error) {
	for _, tid := range f[id.Subject] {
		if tid == tenantID {
			return true, nil
		}
	}
	return false, nil
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

	created, err := svc.CreateAPIKey(ctx, "", "agent")
	if err != nil {
		t.Fatalf("CreateAPIKey: %v", err)
	}
	if binder.bound[created.ID] != "tea-a" {
		t.Errorf("bound[%s] = %q, want tea-a", created.ID, binder.bound[created.ID])
	}
}

func TestCreateAPIKeyEnforcesWorkspaceActiveQuota(t *testing.T) {
	store := newFakeKeyStore()
	binder := newFakeBinder()
	svc := &Service{
		Base:    &core.Base{Namespace: "default", Workspace: fakeWorkspace{"identity-a": "tea-a"}},
		APIKeys: store, Binding: binder, MaxActiveKeys: 1,
	}
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "identity-a", Method: "session"})
	if _, err := svc.CreateAPIKey(ctx, "", "first"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CreateAPIKey(ctx, "", "second"); !errors.Is(err, core.ErrConflict) {
		t.Fatalf("second key = %v, want active-key conflict", err)
	}
	if len(store.keys) != 1 {
		t.Fatalf("Hydra keys = %d, want 1", len(store.keys))
	}
}

func TestCreateAPIKeyEnforcesWorkspaceCreationRate(t *testing.T) {
	store := newFakeKeyStore()
	binder := newFakeBinder()
	svc := &Service{
		Base:    &core.Base{Namespace: "default", Workspace: fakeWorkspace{"identity-a": "tea-a"}},
		APIKeys: store, Binding: binder,
		CreationLimiter: core.NewKeyedRateLimiter[string](1, 1, time.Hour, time.Minute),
	}
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "identity-a", Method: "session"})
	if _, err := svc.CreateAPIKey(ctx, "", "first"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CreateAPIKey(ctx, "", "second"); !errors.Is(err, core.ErrConflict) {
		t.Fatalf("second key = %v, want creation-rate conflict", err)
	}
}

func TestCreateAPIKeyNoTenantIsRefused(t *testing.T) {
	// Binding wired (store on) but the caller resolves to no tenant — a
	// mint-eligible caller with no workspace to bind into (a machine caller is
	// already refused earlier by the credential-class gate, round-7 F3).
	// Minting must refuse, not produce an orphaned, tuple-less key nobody can
	// ever bind later.
	store := newFakeKeyStore()
	binder := newFakeBinder()
	svc := &Service{
		Base:    &core.Base{Namespace: "default", Workspace: fakeWorkspace{}},
		APIKeys: store,
		Binding: binder,
	}
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "unbound-caller", Method: "session"})

	if _, err := svc.CreateAPIKey(ctx, "", "agent"); !errors.Is(err, core.ErrBadRequest) {
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

	if _, err := svc.CreateAPIKey(ctx, "", "agent"); err == nil {
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

	created, err := svc.CreateAPIKey(ctx, "", "agent")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := svc.RevokeAPIKey(ctx, "", created.ID); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	if len(binder.unbound) != 1 || binder.unbound[0] != created.ID {
		t.Errorf("unbound = %v, want [%s]", binder.unbound, created.ID)
	}
}

// --- w6/m18: ListAPIKeys/RevokeAPIKey workspace scoping ---

func TestListAPIKeysScopedToTargetWorkspace(t *testing.T) {
	// dana belongs to both tea-a (her default) and tea-b; a key exists in
	// each. Listing must return ONLY the targeted workspace's key — before
	// this milestone ListAPIKeys had no tenant filter at all and returned
	// every workspace's keys to any caller who could manage their own.
	store := newFakeKeyStore()
	binder := newFakeBinder()
	svc := &Service{
		Base:    &core.Base{Namespace: "default", Workspace: multiWorkspace{"identity-a": {"tea-a", "tea-b"}}},
		APIKeys: store,
		Binding: binder,
	}
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "identity-a", Method: "session"})

	keyA, err := svc.CreateAPIKey(ctx, "", "agent-a") // no ownerId => default, tea-a
	if err != nil {
		t.Fatalf("create agent-a: %v", err)
	}
	keyB, err := svc.CreateAPIKey(ctx, "tea-b", "agent-b")
	if err != nil {
		t.Fatalf("create agent-b: %v", err)
	}

	listA, err := svc.ListAPIKeys(ctx, "")
	if err != nil {
		t.Fatalf("list default: %v", err)
	}
	if len(listA) != 1 || listA[0].ID != keyA.ID {
		t.Errorf("list default (tea-a) = %+v, want exactly [%s]", listA, keyA.ID)
	}

	listB, err := svc.ListAPIKeys(ctx, "tea-b")
	if err != nil {
		t.Fatalf("list tea-b: %v", err)
	}
	if len(listB) != 1 || listB[0].ID != keyB.ID {
		t.Errorf("list tea-b = %+v, want exactly [%s]", listB, keyB.ID)
	}
}

func TestRevokeAPIKeyRefusesCrossWorkspaceTarget(t *testing.T) {
	// A key bound to tea-a must not be revocable by naming tea-b, even though
	// dana administers both — the same cross-workspace gate w6/m14 gave
	// Apps/Databases/KeyValues. Before this milestone RevokeAPIKey had no
	// ownership check at all: any key id could be deleted by anyone who could
	// manage keys in ANY workspace.
	store := newFakeKeyStore()
	binder := newFakeBinder()
	svc := &Service{
		Base:    &core.Base{Namespace: "default", Workspace: multiWorkspace{"identity-a": {"tea-a", "tea-b"}}},
		APIKeys: store,
		Binding: binder,
	}
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "identity-a", Method: "session"})

	keyA, err := svc.CreateAPIKey(ctx, "", "agent-a") // tea-a
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if err := svc.RevokeAPIKey(ctx, "tea-b", keyA.ID); !errors.Is(err, core.ErrForbidden) {
		t.Errorf("revoke tea-a's key targeting tea-b: want ErrForbidden, got %v", err)
	}
	if _, ok := store.keys[keyA.ID]; !ok {
		t.Error("refused revoke must not delete the key")
	}

	// Targeting its own workspace succeeds.
	if err := svc.RevokeAPIKey(ctx, "tea-a", keyA.ID); err != nil {
		t.Errorf("revoke tea-a's key targeting tea-a: %v", err)
	}
}

func TestRevokeAPIKeyRefusesUnboundKey(t *testing.T) {
	// w1/m53: an unbound key (present in the store but with no tenant binding)
	// must not be deletable by a workspace-scoped caller. The old code fell
	// through to Delete, letting a can_manage_keys caller in any workspace remove
	// an orphaned credential that belongs to nobody.
	store := newFakeKeyStore()
	binder := newFakeBinder()
	svc := &Service{
		Base:    &core.Base{Namespace: "default", Workspace: multiWorkspace{"identity-a": {"tea-a", "tea-b"}}},
		APIKeys: store,
		Binding: binder,
	}
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "identity-a", Method: "session"})

	keyA, err := svc.CreateAPIKey(ctx, "", "agent-a") // bound to tea-a
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	_ = binder.UnbindKey(ctx, keyA.ID) // orphan it: binding gone, key remains

	if err := svc.RevokeAPIKey(ctx, "tea-a", keyA.ID); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("revoke unbound key: want ErrNotFound, got %v", err)
	}
	if _, ok := store.keys[keyA.ID]; !ok {
		t.Error("refused revoke must not delete the orphaned key")
	}
}

func TestCreateAPIKeyNilBindingMintsUnbound(t *testing.T) {
	// Store off (Binding nil): keys mint unbound, byte-identical to before
	// tenant onboarding existed — no tenant lookup, no refusal.
	store := newFakeKeyStore()
	svc := newService(store)
	ctx := context.Background() // no identity at all — must not matter

	created, err := svc.CreateAPIKey(ctx, "", "agent")
	if err != nil || created.ID == "" {
		t.Fatalf("nil Binding: %v %+v", err, created)
	}
}

// --- Verb behavior ---

func TestAPIKeyVerbs(t *testing.T) {
	store := newFakeKeyStore()
	svc := newService(store)
	ctx := context.Background()

	created, err := svc.CreateAPIKey(ctx, "", "ci-agent")
	if err != nil || created.ID == "" || created.Secret == "" {
		t.Fatalf("create must return id+secret: %v %+v", err, created)
	}
	listed, _ := svc.ListAPIKeys(ctx, "")
	if len(listed) != 1 || listed[0].Secret != "" {
		t.Fatalf("list must omit secrets: %+v", listed)
	}
	if err := svc.RevokeAPIKey(ctx, "", created.ID); err != nil || len(store.keys) != 0 {
		t.Fatalf("revoke should delete: %v store=%d", err, len(store.keys))
	}
	// blank name => ErrBadRequest.
	if _, err := svc.CreateAPIKey(ctx, "", "  "); !errors.Is(err, core.ErrBadRequest) {
		t.Errorf("blank name => ErrBadRequest, got %v", err)
	}
	// nil store => ErrAPIKeysUnavailable on every verb.
	nostore := newService(nil)
	if _, err := nostore.ListAPIKeys(ctx, ""); !errors.Is(err, core.ErrAPIKeysUnavailable) {
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
		case r.Method == http.MethodPatch && strings.HasPrefix(r.URL.Path, "/admin/clients/"):
			// Minimal JSON Patch (RFC 6902): apply `add` ops targeting a
			// /metadata/<escaped-key> pointer, which is all bex's Touch emits.
			id := strings.TrimPrefix(r.URL.Path, "/admin/clients/")
			c, ok := clients[id]
			if !ok {
				http.NotFound(w, r)
				return
			}
			var ops []struct {
				Op    string `json:"op"`
				Path  string `json:"path"`
				Value any    `json:"value"`
			}
			_ = json.NewDecoder(r.Body).Decode(&ops)
			for _, op := range ops {
				key, ok := strings.CutPrefix(op.Path, "/metadata/")
				if op.Op != "add" || !ok {
					http.Error(w, "unsupported patch op", http.StatusBadRequest)
					return
				}
				// Unescape JSON Pointer (RFC 6901): ~1 => /, ~0 => ~.
				key = strings.ReplaceAll(key, "~1", "/")
				key = strings.ReplaceAll(key, "~0", "~")
				if c.Metadata == nil {
					c.Metadata = map[string]any{}
				}
				c.Metadata[key] = op.Value
			}
			clients[id] = c
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

	created, err := store.Create(ctx, "agent-1", "user:minter")
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

// --- Metadata: created-by + last-used (w4/m13) ---

func TestHydraStoreMetadata(t *testing.T) {
	store := NewHydraAPIKeys(fakeHydraAdmin(t).URL)
	ctx := context.Background()

	created, err := store.Create(ctx, "agent-1", "user:minter")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	list := func() APIKey {
		keys, err := store.List(ctx)
		if err != nil || len(keys) != 1 {
			t.Fatalf("List: %v %+v", err, keys)
		}
		return keys[0]
	}
	// created-by is stamped at mint and surfaces on list; last-used starts empty.
	if k := list(); k.CreatedBy != "user:minter" || k.LastUsedAt != "" {
		t.Fatalf("after create: createdBy=%q lastUsedAt=%q, want minter/empty", k.CreatedBy, k.LastUsedAt)
	}

	// Touch stamps last-used; a second read reflects it, created-by unchanged.
	used := time.Date(2026, 7, 5, 12, 0, 0, 0, time.UTC)
	if err := store.Touch(ctx, created.ID, used); err != nil {
		t.Fatalf("Touch: %v", err)
	}
	if k := list(); k.LastUsedAt != used.Format(time.RFC3339) || k.CreatedBy != "user:minter" {
		t.Fatalf("after touch: lastUsedAt=%q createdBy=%q", k.LastUsedAt, k.CreatedBy)
	}

	// Touch is a no-op (no error, no write) for a non-api-key platform client and
	// for an unknown id — the auth gate calls it for every introspected token.
	if err := store.Touch(ctx, "platform-client", used); err != nil {
		t.Fatalf("Touch of platform client should no-op, got %v", err)
	}
	if err := store.Touch(ctx, "does-not-exist", used); err != nil {
		t.Fatalf("Touch of unknown id should no-op, got %v", err)
	}
}

func TestCreateAPIKeyStampsCaller(t *testing.T) {
	store := newFakeKeyStore()
	svc := newService(store)
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "user:alice", Method: "session"})

	created, err := svc.CreateAPIKey(ctx, "", "for-alice")
	if err != nil {
		t.Fatalf("CreateAPIKey: %v", err)
	}
	if created.CreatedBy != "user:alice" {
		t.Fatalf("createdBy = %q, want user:alice", created.CreatedBy)
	}
	// With no identity in context, created-by is simply empty (no crash).
	anon, err := svc.CreateAPIKey(context.Background(), "", "anon")
	if err != nil || anon.CreatedBy != "" {
		t.Fatalf("anon create: %v createdBy=%q, want empty", err, anon.CreatedBy)
	}
}

// recordingStore counts Touch calls and signals each on a channel, so the
// Service's async+throttled dispatch is observable deterministically.
type recordingStore struct {
	touchCh chan time.Time
}

func (recordingStore) Create(context.Context, string, string) (APIKey, error) {
	return APIKey{}, nil
}
func (recordingStore) List(context.Context) ([]APIKey, error)          { return nil, nil }
func (recordingStore) Delete(context.Context, string) error            { return nil }
func (recordingStore) KeyOwner(context.Context, string) (string, bool) { return "", false }
func (r recordingStore) Touch(_ context.Context, _ string, at time.Time) error {
	r.touchCh <- at
	return nil
}

func TestTouchAPIKeyThrottleAndAsync(t *testing.T) {
	st := recordingStore{touchCh: make(chan time.Time, 8)}
	now := time.Unix(1_000_000, 0).UTC()
	svc := &Service{Base: &core.Base{Namespace: "default", Clock: func() time.Time { return now }}, APIKeys: st}

	waitTouch := func() (time.Time, bool) {
		select {
		case at := <-st.touchCh:
			return at, true
		case <-time.After(time.Second):
			return time.Time{}, false
		}
	}
	noTouch := func() {
		select {
		case at := <-st.touchCh:
			t.Fatalf("unexpected touch dispatched at %v", at)
		case <-time.After(100 * time.Millisecond):
		}
	}

	// Two touches in the same minute dispatch exactly one store write.
	svc.TouchAPIKey("key-1")
	svc.TouchAPIKey("key-1")
	if at, ok := waitTouch(); !ok || !at.Equal(now) {
		t.Fatalf("first touch not dispatched with the call-time timestamp: %v %v", at, ok)
	}
	noTouch()

	// A different key is throttled independently.
	svc.TouchAPIKey("key-2")
	if _, ok := waitTouch(); !ok {
		t.Fatal("second key should dispatch its own touch")
	}

	// Past the throttle window, key-1 dispatches again — and the elapsed entries
	// are evicted so the throttle map stays bounded by keys used within the window
	// (only key-1 remains; key-2's window has passed).
	now = now.Add(touchThrottle + time.Second)
	svc.TouchAPIKey("key-1")
	if _, ok := waitTouch(); !ok {
		t.Fatal("touch after the throttle window should dispatch")
	}
	svc.touchMu.Lock()
	remaining := len(svc.nextTouch)
	svc.touchMu.Unlock()
	if remaining != 1 {
		t.Fatalf("throttle map should evict elapsed entries, len = %d, want 1", remaining)
	}

	// Guard rails: nil store and empty id never dispatch.
	(&Service{Base: svc.Base}).TouchAPIKey("key-1")
	svc.TouchAPIKey("")
	noTouch()
}
