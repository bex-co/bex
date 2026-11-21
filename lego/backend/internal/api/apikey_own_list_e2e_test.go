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
	"github.com/bex-co/bex/lego/backend/internal/members"
	"github.com/bex-co/bex/lego/backend/internal/store"
	"github.com/bex-co/bex/lego/backend/internal/tenancy"
	"github.com/bex-co/bex/lego/backend/internal/testenv"
)

// apikey_own_list_e2e_test.go is w9/m539's proof against REAL Postgres and
// REAL OpenFGA, through the composed REST, GraphQL and MCP surfaces. Every
// role lists exactly the keys it minted, as each person does on Render's
// Account Settings (I10); only an Admin asks for every key in the workspace
// and revokes another member's or a creator-less key (I14); and editing a
// key's grant is its creator's alone, an Admin included.
//
//	BEX_TEST_DB_URI=postgres://…/postgres?sslmode=disable \
//	BEX_TEST_OPENFGA_URL=http://127.0.0.1:58085 \
//	  go test ./internal/api -run TestEveryRoleListsItsOwnAPIKeysE2E -v
func TestEveryRoleListsItsOwnAPIKeysE2E(t *testing.T) {
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
	admin, dev, viewer, contrib, bill := "admin-"+run, "dev-"+run, "viewer-"+run, "contrib-"+run, "billing-"+run
	checker := authz.NewOpenFGAChecker(fgaURL, os.Getenv("BEX_TEST_OPENFGA_TOKEN"))
	roles := checker.(store.MembershipGranter)

	// Viewer, Contributor and Billing are Scale roles (store.LimitsFor).
	ws, err := st.CreateWorkspace(ctx, "ownkeys-"+run, store.PlanScale, admin)
	if err != nil {
		t.Fatal(err)
	}
	if err := roles.GrantWorkspaceRole(ctx, ws.ID, "user:"+admin, "admin"); err != nil {
		t.Fatal(err)
	}
	roleOf := map[string]string{admin: "admin", dev: "developer", viewer: "viewer", contrib: "contributor", bill: "billing"}
	for subject, role := range roleOf {
		if subject == admin {
			continue
		}
		if err := st.AddMember(ctx, subject, ws.ID, role); err != nil {
			t.Fatal(err)
		}
		if err := roles.GrantWorkspaceRole(ctx, ws.ID, "user:"+subject, role); err != nil {
			t.Fatalf("grant %s %s: %v", subject, role, err)
		}
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
	tupleHeld := func(subject, relation string) bool {
		t.Helper()
		ok, err := checker.(core.FreshChecker).CheckFresh(ctx, "user:"+subject, relation, core.WorkspaceObject(ws.ID))
		if err != nil {
			t.Fatal(err)
		}
		return ok
	}
	idsOf := func(list []apikeys.APIKey) []string {
		ids := []string{}
		for _, k := range list {
			ids = append(ids, k.ID)
		}
		slices.Sort(ids)
		return ids
	}

	// Each role mints one key (All), the workspace's creation burst of five.
	keyOfSubject := map[string]string{}
	for _, subject := range []string{admin, dev, viewer, contrib, bill} {
		rec := rest(session(subject), http.MethodPost, "/v1/api-keys", `{"name":"own-`+roleOf[subject]+`","ownerId":"`+ws.ID+`"}`)
		var key apikeys.APIKey
		if rec.Code != http.StatusCreated || json.Unmarshal(rec.Body.Bytes(), &key) != nil || key.ID == "" {
			t.Fatalf("%s mints a key: %d %s", roleOf[subject], rec.Code, rec.Body)
		}
		keyOfSubject[subject] = key.ID
	}
	// A key with no recorded creator, bound as one minted before the stamp
	// was (I3 seats it as a Developer it never leaves).
	legacy := "legacy-" + run
	keys.mu.Lock()
	keys.keys[legacy] = apikeys.APIKey{ID: legacy, Name: "legacy"}
	keys.mu.Unlock()
	if err := resolver.BindKey(ctx, legacy, ws.ID, ""); err != nil {
		t.Fatalf("bind the creator-less key: %v", err)
	}
	every := []string{legacy}
	for _, id := range keyOfSubject {
		every = append(every, id)
	}
	slices.Sort(every)

	// The three surfaces' lists, as id sets; ok=false is a refusal.
	type answer struct {
		ids  []string
		code string // the refusal: REST status, GraphQL and MCP error text
	}
	list := map[string]func(id core.Identity, allMembers bool) answer{
		"REST": func(id core.Identity, allMembers bool) answer {
			path := "/v1/api-keys?ownerId=" + ws.ID
			if allMembers {
				path += "&allMembers=true"
			}
			rec := rest(id, http.MethodGet, path, "")
			var out []apikeys.APIKey
			if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &out) != nil {
				return answer{code: fmt.Sprintf("%d %s", rec.Code, rec.Body)}
			}
			return answer{ids: idsOf(out)}
		},
		"GraphQL": func(id core.Identity, allMembers bool) answer {
			out := graphqlDo(t, srv, id, fmt.Sprintf(`{ apiKeys(ownerId: %q, allMembers: %v) { id } }`, ws.ID, allMembers))
			if out["errors"] != nil {
				return answer{code: graphqlCode(out)}
			}
			raw, _ := json.Marshal(out["data"].(map[string]any)["apiKeys"])
			var got []apikeys.APIKey
			_ = json.Unmarshal(raw, &got)
			return answer{ids: idsOf(got)}
		},
		"MCP": func(id core.Identity, allMembers bool) answer {
			res := mcpCall(t, srv, id, "list_api_keys", map[string]any{"workspaceId": ws.ID, "allMembers": allMembers})
			if res.IsError {
				return answer{code: mcpText(res)}
			}
			var got struct{ APIKeys []apikeys.APIKey }
			_ = json.Unmarshal([]byte(mcpText(res)), &got)
			return answer{ids: idsOf(got.APIKeys)}
		},
	}
	forbidden := func(surface, code string) bool {
		switch surface {
		case "REST":
			return strings.HasPrefix(code, "403 ")
		case "GraphQL":
			return code == "FORBIDDEN"
		}
		return strings.Contains(strings.ToLower(code), "forbidden")
	}

	// I10: every role's default list is exactly its own key, on all three
	// surfaces. I14: only the Admin's allMembers list holds every key, the
	// creator-less one included; every other role is refused it.
	for surface, listOf := range list {
		for subject, role := range roleOf {
			if got := listOf(session(subject), false); got.code != "" || !slices.Equal(got.ids, []string{keyOfSubject[subject]}) {
				t.Errorf("%s: the %s lists %v (%s), want only their own %s", surface, role, got.ids, got.code, keyOfSubject[subject])
			}
			got := listOf(session(subject), true)
			if subject == admin {
				if got.code != "" || !slices.Equal(got.ids, every) {
					t.Errorf("%s: the Admin's allMembers list = %v (%s), want every key %v", surface, got.ids, got.code, every)
				}
			} else if !forbidden(surface, got.code) {
				t.Errorf("%s: the %s's allMembers list = %v (%s), want 403", surface, role, got.ids, got.code)
			}
		}
		// A key owns nothing: the Admin's All key lists nothing by default
		// and everything with allMembers; the Developer's is refused that.
		if got := listOf(keyOf(keyOfSubject[admin]), false); got.code != "" || len(got.ids) != 0 {
			t.Errorf("%s: the Admin's key lists %v (%s), want nothing", surface, got.ids, got.code)
		}
		if got := listOf(keyOf(keyOfSubject[admin]), true); got.code != "" || !slices.Equal(got.ids, every) {
			t.Errorf("%s: the Admin's key's allMembers list = %v (%s), want every key", surface, got.ids, got.code)
		}
		if got := listOf(keyOf(keyOfSubject[dev]), true); !forbidden(surface, got.code) {
			t.Errorf("%s: the Developer's key's allMembers list = %v (%s), want 403", surface, got.ids, got.code)
		}
	}
	if rec := rest(session(admin), http.MethodGet, "/v1/api-keys?ownerId="+ws.ID+"&allMembers=maybe", ""); rec.Code != http.StatusBadRequest {
		t.Errorf("allMembers=maybe = %d %s, want 400", rec.Code, rec.Body)
	}

	// A Developer edits and revokes no one else's key, on any surface: the
	// Viewer's, the Admin's and the creator-less one.
	patch := `{"permissions":{"grants":[{"permission":"services","level":"read"}]}}`
	for _, other := range []string{keyOfSubject[viewer], keyOfSubject[admin], legacy} {
		for _, c := range []struct{ name, method, body string }{
			{"edit", http.MethodPatch, patch},
			{"revoke", http.MethodDelete, ""},
		} {
			if rec := rest(session(dev), c.method, "/v1/api-keys/"+other+"?ownerId="+ws.ID, c.body); rec.Code != http.StatusForbidden {
				t.Errorf("REST: the Developer's %s of %s = %d %s, want 403", c.name, other, rec.Code, rec.Body)
			}
		}
		if out := graphqlDo(t, srv, session(dev), fmt.Sprintf(`mutation { revokeApiKey(id: %q, ownerId: %q) }`, other, ws.ID)); graphqlCode(out) != "FORBIDDEN" {
			t.Errorf("GraphQL: the Developer's revoke of %s = %v", other, out)
		}
		if out := graphqlDo(t, srv, session(dev), fmt.Sprintf(`mutation { updateApiKeyPermissions(id: %q, ownerId: %q, permissions: {all: true}) { id } }`, other, ws.ID)); graphqlCode(out) != "FORBIDDEN" {
			t.Errorf("GraphQL: the Developer's edit of %s = %v", other, out)
		}
		if res := mcpCall(t, srv, session(dev), "revoke_api_key", map[string]any{"workspaceId": ws.ID, "keyId": other}); !res.IsError || !forbidden("MCP", mcpText(res)) {
			t.Errorf("MCP: the Developer's revoke of %s = %s", other, mcpText(res))
		}
		if res := mcpCall(t, srv, session(dev), "update_api_key_permissions", map[string]any{"workspaceId": ws.ID, "keyId": other, "permissions": map[string]any{"all": true}}); !res.IsError || !forbidden("MCP", mcpText(res)) {
			t.Errorf("MCP: the Developer's edit of %s = %s", other, mcpText(res))
		}
		if _, ok := keys.keys[other]; !ok {
			t.Fatalf("a refused revoke deleted %s", other)
		}
	}
	// A Developer's key reaches no other key either.
	if rec := rest(keyOf(keyOfSubject[dev]), http.MethodDelete, "/v1/api-keys/"+keyOfSubject[viewer]+"?ownerId="+ws.ID, ""); rec.Code != http.StatusForbidden {
		t.Errorf("the Developer's key revokes the Viewer's key: %d %s", rec.Code, rec.Body)
	}

	// The Admin edits no one else's grant, on any surface.
	devKey := keyOfSubject[dev]
	if rec := rest(session(admin), http.MethodPatch, "/v1/api-keys/"+devKey+"?ownerId="+ws.ID, patch); rec.Code != http.StatusForbidden ||
		!strings.Contains(rec.Body.String(), apikeys.KeyEditorForbiddenCode) {
		t.Errorf("REST: the Admin edits the Developer's key = %d %s, want 403 %s", rec.Code, rec.Body, apikeys.KeyEditorForbiddenCode)
	}
	if out := graphqlDo(t, srv, session(admin), fmt.Sprintf(`mutation { updateApiKeyPermissions(id: %q, ownerId: %q, permissions: {grants: []}) { id } }`, devKey, ws.ID)); graphqlCode(out) != apikeys.KeyEditorForbiddenCode {
		t.Errorf("GraphQL: the Admin edits the Developer's key = %v", out)
	}
	if res := mcpCall(t, srv, session(admin), "update_api_key_permissions", map[string]any{"workspaceId": ws.ID, "keyId": devKey, "permissions": map[string]any{"grants": []any{}}}); !res.IsError ||
		!strings.Contains(mcpText(res), apikeys.KeyEditorForbiddenCode) {
		t.Errorf("MCP: the Admin edits the Developer's key = %s", mcpText(res))
	}
	if out := graphqlDo(t, srv, session(dev), fmt.Sprintf(`{ apiKeys(ownerId: %q) { permissions { all } } }`, ws.ID)); !strings.Contains(fmt.Sprint(out), "all:true") {
		t.Errorf("a refused edit changed the Developer's grant: %v", out)
	}
	// The creator still edits their own.
	if rec := rest(session(dev), http.MethodPatch, "/v1/api-keys/"+devKey+"?ownerId="+ws.ID, patch); rec.Code != http.StatusOK {
		t.Errorf("the Developer edits their own key: %d %s", rec.Code, rec.Body)
	}

	// The Admin revokes another member's key on each surface and the
	// creator-less key: each leaves the registry, loses its tuple and is
	// refused on its next request.
	revoked := map[string]string{
		"REST":    keyOfSubject[viewer],
		"GraphQL": keyOfSubject[contrib],
		"MCP":     legacy,
	}
	if rec := rest(session(admin), http.MethodDelete, "/v1/api-keys/"+revoked["REST"]+"?ownerId="+ws.ID, ""); rec.Code != http.StatusNoContent {
		t.Errorf("REST: the Admin revokes the Viewer's key: %d %s", rec.Code, rec.Body)
	}
	if out := graphqlDo(t, srv, session(admin), fmt.Sprintf(`mutation { revokeApiKey(id: %q, ownerId: %q) }`, revoked["GraphQL"], ws.ID)); out["errors"] != nil {
		t.Errorf("GraphQL: the Admin revokes the Contributor's key: %v", out)
	}
	if res := mcpCall(t, srv, session(admin), "revoke_api_key", map[string]any{"workspaceId": ws.ID, "keyId": revoked["MCP"]}); res.IsError {
		t.Errorf("MCP: the Admin revokes the creator-less key: %s", mcpText(res))
	}
	for surface, id := range revoked {
		if _, ok := keys.keys[id]; ok {
			t.Errorf("%s: the revoked key %s is still in the registry", surface, id)
		}
		for _, role := range members.Roles {
			if tupleHeld(id, role) {
				t.Errorf("%s: the revoked key %s still holds %s", surface, id, role)
			}
		}
		if rec := rest(keyOf(id), http.MethodGet, "/v1/services?ownerId="+ws.ID, ""); rec.Code != http.StatusForbidden {
			t.Errorf("%s: the revoked key %s lists services: %d", surface, id, rec.Code)
		}
	}
	// What is left: the Admin's, the Developer's and the Billing member's.
	if got := list["REST"](session(admin), true); !slices.Equal(got.ids, idsOf([]apikeys.APIKey{{ID: keyOfSubject[admin]}, {ID: devKey}, {ID: keyOfSubject[bill]}})) {
		t.Errorf("after the revokes the Admin's allMembers list = %v (%s)", got.ids, got.code)
	}
}
