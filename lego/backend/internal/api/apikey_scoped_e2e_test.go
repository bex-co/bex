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
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bex-co/bex/lego/backend/internal/apikeys"
	"github.com/bex-co/bex/lego/backend/internal/authz"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
	"github.com/bex-co/bex/lego/backend/internal/tenancy"
	"github.com/bex-co/bex/lego/backend/internal/testenv"
)

// apikey_scoped_e2e_test.go is w9/m496's proof against REAL Postgres and REAL
// OpenFGA, through the composed REST, GraphQL and MCP surfaces: a key carries
// the permissions it was minted with (recorded on its machine row, migration
// 0300), is refused everything else on every surface, is narrowed on its next
// request, and is still never more than its creator now (I1, I6).
//
//	BEX_TEST_DB_URI=postgres://…/postgres?sslmode=disable \
//	BEX_TEST_OPENFGA_URL=http://127.0.0.1:58085 \
//	  go test ./internal/api -run TestScopedAPIKeyE2E -v
func TestScopedAPIKeyE2E(t *testing.T) {
	dbURI := os.Getenv("BEX_TEST_DB_URI")
	fgaURL := os.Getenv("BEX_TEST_OPENFGA_URL")
	if dbURI == "" || fgaURL == "" {
		testenv.Skip(t, "BEX_TEST_DB_URI and BEX_TEST_OPENFGA_URL not both set")
	}
	ctx := context.Background()
	if err := store.Migrate(dbURI); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	pool, err := pgxpool.New(ctx, dbURI)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	st := store.NewPGStore(pool)

	run := uniqueRunSuffix()
	admin, admin2, dev := "admin-"+run, "admin2-"+run, "dev-"+run
	checker := authz.NewOpenFGAChecker(fgaURL, os.Getenv("BEX_TEST_OPENFGA_TOKEN"))
	roles := checker.(store.MembershipGranter)
	ws, err := st.CreateWorkspace(ctx, "scoped-"+run, store.PlanPro, admin)
	if err != nil {
		t.Fatal(err)
	}
	seat := func(subject, role string) {
		t.Helper()
		if err := roles.GrantWorkspaceRole(ctx, ws.ID, "user:"+subject, role); err != nil {
			t.Fatalf("grant %s %s: %v", subject, role, err)
		}
	}
	seat(admin, "admin")
	for _, m := range []struct{ subject, role string }{{admin2, "admin"}, {dev, "developer"}} {
		if err := st.AddMember(ctx, m.subject, ws.ID, m.role); err != nil {
			t.Fatal(err)
		}
		seat(m.subject, m.role)
	}

	keys := &creatorKeys{run: run, keys: map[string]apikeys.APIKey{}}
	resolver := tenancy.NewTenantService(st, roles)
	base := &core.Base{
		Client: fakeClient(), Namespace: "default",
		Authz: checker, Workspace: resolver, KeyCreators: keys, KeyGrants: resolver,
	}
	srv := NewServer(base, Deps{
		Store: st, WorkspaceStore: st, MembersStore: st,
		MembersGranter: roles, MembersRevoker: roles,
		APIKeys: keys, KeyBinder: resolver,
		WebhookStore: st, RegistryCredsStore: st, Secrets: newMemSecretStore(),
		NotificationsStore: st, ProjectsStore: st, EnvironmentsStore: st, EnvGroupsStore: st,
	})
	mux := srv.restHandler()
	gated := srv.withScopeClassREST(mux, mux)
	schema, err := srv.newSchema()
	if err != nil {
		t.Fatal(err)
	}
	srv.schema = schema

	session := func(subject string) core.Identity { return core.Identity{Subject: subject, Method: "session"} }
	keyOf := func(id string) core.Identity {
		return core.Identity{Subject: id, ClientID: id, Method: core.MethodOAuth2}
	}
	rest := func(id core.Identity, method, path, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		gated.ServeHTTP(rec, req.WithContext(core.WithIdentity(ctx, id)))
		return rec
	}
	refusedByGrant := func(rec *httptest.ResponseRecorder) bool {
		return rec.Code == http.StatusForbidden && strings.Contains(rec.Body.String(), core.KeyPermissionDeniedCode)
	}
	mint := func(creator, permissions string) (*httptest.ResponseRecorder, apikeys.APIKey) {
		t.Helper()
		body := `{"name":"scoped","ownerId":"` + ws.ID + `"`
		if permissions != "" {
			body += `,"permissions":` + permissions
		}
		rec := rest(session(creator), http.MethodPost, "/v1/api-keys", body+"}")
		var key apikeys.APIKey
		_ = json.Unmarshal(rec.Body.Bytes(), &key)
		return rec, key
	}
	webhook := func(id core.Identity, n int) *httptest.ResponseRecorder {
		return rest(id, http.MethodPost, "/v1/webhooks",
			fmt.Sprintf(`{"ownerId":%q,"name":"scoped-hook-%d","url":"https://example.com/hook","enabled":true,"eventFilter":[]}`, ws.ID, n))
	}

	// A Developer's Services Read only key: recorded on its row, listed back.
	rec, readKey := mint(dev, `{"grants":[{"permission":"services","level":"read"}]}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("mint a Services Read only key: %d %s", rec.Code, rec.Body)
	}
	if g, err := st.APIKeyGrant(ctx, readKey.ID); err != nil || !slices.Equal(g.Entries(), []string{"services:read"}) {
		t.Fatalf("stored grant = %+v, %v", g, err)
	}
	listed := rest(session(dev), http.MethodGet, "/v1/api-keys?ownerId="+ws.ID, "")
	if !strings.Contains(listed.Body.String(), `"permissions":{"all":false,"grants":[{"permission":"services","level":"read"}]}`) {
		t.Errorf("list does not carry the grant: %s", listed.Body)
	}

	// It reads services on all three surfaces and is refused every write.
	key := keyOf(readKey.ID)
	if rec := rest(key, http.MethodGet, "/v1/services?ownerId="+ws.ID, ""); rec.Code != http.StatusOK {
		t.Errorf("REST list services = %d %s", rec.Code, rec.Body)
	}
	if out := graphqlDo(t, srv, key, fmt.Sprintf(`{ services(ownerId: %q) { id } }`, ws.ID)); out["errors"] != nil {
		t.Errorf("GraphQL services = %v", out)
	}
	if res := mcpCall(t, srv, key, "list_services", map[string]any{}); res.IsError {
		t.Errorf("MCP list_services = %s", mcpText(res))
	}
	for _, call := range []struct{ method, path, body string }{
		{http.MethodPost, "/v1/services", `{"type":"web_service","name":"x","ownerId":"` + ws.ID + `"}`},
		{http.MethodDelete, "/v1/services/srv-x", ""},
		{http.MethodPut, "/v1/services/srv-x/env-vars/K", `{"value":"v"}`},
		{http.MethodPost, "/v1/services/srv-x/deploys", `{}`},
		{http.MethodGet, "/v1/postgres?ownerId=" + ws.ID, ""},
	} {
		if rec := rest(key, call.method, call.path, call.body); !refusedByGrant(rec) {
			t.Errorf("Services Read only key %s %s = %d %s", call.method, call.path, rec.Code, rec.Body)
		}
	}
	if out := graphqlDo(t, srv, key, `mutation { deleteService(id: "srv-x") }`); graphqlCode(out) != core.KeyPermissionDeniedCode {
		t.Errorf("GraphQL deleteService = %v", out)
	}
	if res := mcpCall(t, srv, key, "trigger_deploy", map[string]any{"serviceId": "srv-x"}); !res.IsError || !strings.Contains(mcpText(res), "not granted") {
		t.Errorf("MCP trigger_deploy = %s", mcpText(res))
	}

	// The role caps the grant: a Developer cannot name the webhooks write.
	if rec, _ := mint(dev, `{"grants":[{"permission":"workspaces","level":"read_write"}]}`); rec.Code != http.StatusForbidden ||
		!strings.Contains(rec.Body.String(), core.KeyPermissionAboveRoleCode) {
		t.Errorf("Developer's Workspaces Read & write mint = %d %s", rec.Code, rec.Body)
	}
	// An Admin's key with Workspaces Read & write creates a webhook; the same
	// Admin's key without it is refused.
	_, hookKey := mint(admin, `{"grants":[{"permission":"workspaces","level":"read_write"}]}`)
	_, plainKey := mint(admin, `{"grants":[{"permission":"services","level":"read"}]}`)
	if rec := webhook(keyOf(hookKey.ID), 1); rec.Code != http.StatusCreated {
		t.Errorf("Workspaces Read & write key's webhook = %d %s", rec.Code, rec.Body)
	}
	if rec := webhook(keyOf(plainKey.ID), 2); !refusedByGrant(rec) {
		t.Errorf("Services Read key's webhook = %d %s", rec.Code, rec.Body)
	}

	// Editing applies on the next request, on every edit surface.
	rec = rest(session(admin), http.MethodPatch, "/v1/api-keys/"+hookKey.ID+"?ownerId="+ws.ID, `{"permissions":{"grants":[{"permission":"workspaces","level":"read"}]}}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("narrow over REST: %d %s", rec.Code, rec.Body)
	}
	if rec := webhook(keyOf(hookKey.ID), 3); !refusedByGrant(rec) {
		t.Errorf("narrowed key's webhook = %d %s", rec.Code, rec.Body)
	}
	out := graphqlDo(t, srv, session(admin), fmt.Sprintf(`mutation { updateApiKeyPermissions(id: %q, ownerId: %q, permissions: {grants: [{permission: "workspaces", level: "read_write"}]}) { permissions { all grants { permission level } } } }`, hookKey.ID, ws.ID))
	if out["errors"] != nil {
		t.Fatalf("widen over GraphQL: %v", out)
	}
	if rec := webhook(keyOf(hookKey.ID), 4); rec.Code != http.StatusCreated {
		t.Errorf("re-widened key's webhook = %d %s", rec.Code, rec.Body)
	}
	if res := mcpCall(t, srv, session(admin), "update_api_key_permissions", map[string]any{
		"keyId": hookKey.ID, "permissions": map[string]any{"grants": []map[string]any{{"permission": "services", "level": "read"}}},
	}); res.IsError {
		t.Fatalf("narrow over MCP: %s", mcpText(res))
	}
	if rec := webhook(keyOf(hookKey.ID), 5); !refusedByGrant(rec) {
		t.Errorf("MCP-narrowed key's webhook = %d %s", rec.Code, rec.Body)
	}
	// A key edits no grant, its own included (I8).
	if rec := rest(keyOf(hookKey.ID), http.MethodPatch, "/v1/api-keys/"+hookKey.ID+"?ownerId="+ws.ID, `{"permissions":{"all":true}}`); rec.Code != http.StatusForbidden {
		t.Errorf("a key widened its own grant: %d %s", rec.Code, rec.Body)
	}

	// I1, I6: the grant can only refuse. Widened back, then the Admin is
	// demoted: the key is refused the webhook on its next request though its
	// grant still names it.
	if rec := rest(session(admin), http.MethodPatch, "/v1/api-keys/"+hookKey.ID+"?ownerId="+ws.ID, `{"permissions":{"grants":[{"permission":"workspaces","level":"read_write"}]}}`); rec.Code != http.StatusOK {
		t.Fatalf("widen: %d %s", rec.Code, rec.Body)
	}
	if rec := rest(session(admin2), http.MethodPatch, "/v1/workspaces/"+ws.ID+"/members/"+admin, `{"role":"DEVELOPER"}`); rec.Code != http.StatusOK {
		t.Fatalf("demote: %d %s", rec.Code, rec.Body)
	}
	if rec := webhook(keyOf(hookKey.ID), 6); rec.Code != http.StatusForbidden {
		t.Errorf("a demoted Admin's key with the webhooks grant = %d %s", rec.Code, rec.Body)
	}
	// And the demoted creator can no longer widen it past their new role.
	if rec := rest(session(admin), http.MethodPatch, "/v1/api-keys/"+plainKey.ID+"?ownerId="+ws.ID, `{"permissions":{"grants":[{"permission":"workspaces","level":"read_write"}]}}`); rec.Code != http.StatusForbidden {
		t.Errorf("a demoted creator widened their key past their role: %d %s", rec.Code, rec.Body)
	}

	// A key minted with no permissions is All: today's behavior.
	_, allKey := mint(dev, "")
	if g, err := st.APIKeyGrant(ctx, allKey.ID); err != nil || !g.All || !allKey.Permissions.All {
		t.Errorf("unnamed grant = %+v / %+v, %v; want All", g, allKey.Permissions, err)
	}
	if rec := rest(keyOf(allKey.ID), http.MethodGet, "/v1/postgres?ownerId="+ws.ID, ""); refusedByGrant(rec) {
		t.Errorf("an All key was refused by the grant gate: %s", rec.Body)
	}

	// Revoked, the row and its grant go with the key.
	if rec := rest(session(dev), http.MethodDelete, "/v1/api-keys/"+readKey.ID+"?ownerId="+ws.ID, ""); rec.Code != http.StatusNoContent {
		t.Fatalf("revoke: %d %s", rec.Code, rec.Body)
	}
	if g, err := st.APIKeyGrant(ctx, readKey.ID); err != nil || !g.All {
		t.Errorf("a revoked key's grant = %+v, %v; want no row (All, refused at the gate by revocation)", g, err)
	}
}
