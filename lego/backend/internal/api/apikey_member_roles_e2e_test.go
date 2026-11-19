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

// apikey_member_roles_e2e_test.go is w9/m514's proof against REAL Postgres
// and REAL OpenFGA, through the composed REST, GraphQL and MCP surfaces. It
// extends TestAPIKeyActsWithItsCreatorsRoleE2E (w9/m481) to the three roles
// below Developer: a Viewer, a Contributor and a Billing member each mint a
// key, as on Render; each key is allowed exactly what its creator is and
// refused the rest with the same answer (I9); the role caps a chosen grant;
// the key follows its creator up, down and out; and a member below Developer
// lists, edits and revokes the keys they minted and no other (I10).
//
//	BEX_TEST_DB_URI=postgres://…/postgres?sslmode=disable \
//	BEX_TEST_OPENFGA_URL=http://127.0.0.1:58085 \
//	  go test ./internal/api -run TestBelowDeveloperAPIKeysE2E -v
func TestBelowDeveloperAPIKeysE2E(t *testing.T) {
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
	admin, dev := "admin-"+run, "dev-"+run
	viewer, viewer2, contrib, bill := "viewer-"+run, "viewer2-"+run, "contrib-"+run, "billing-"+run
	checker := authz.NewOpenFGAChecker(fgaURL, os.Getenv("BEX_TEST_OPENFGA_TOKEN"))
	roles := checker.(store.MembershipGranter)

	// Viewer, Contributor and Billing are Scale roles (store.LimitsFor).
	ws, err := st.CreateWorkspace(ctx, "memberkeys-"+run, store.PlanScale, admin)
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
	for _, m := range []struct{ subject, role string }{
		{dev, "developer"}, {viewer, "viewer"}, {viewer2, "viewer"}, {contrib, "contributor"}, {bill, "billing"},
	} {
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
	tupleHeld := func(subject, relation string) bool {
		t.Helper()
		ok, err := checker.(core.FreshChecker).CheckFresh(ctx, "user:"+subject, relation, core.WorkspaceObject(ws.ID))
		if err != nil {
			t.Fatal(err)
		}
		return ok
	}
	mint := func(creator, permissions string) (*httptest.ResponseRecorder, apikeys.APIKey) {
		t.Helper()
		body := `{"name":"member-key","ownerId":"` + ws.ID + `"`
		if permissions != "" {
			body += `,"permissions":` + permissions
		}
		rec := rest(session(creator), http.MethodPost, "/v1/api-keys", body+"}")
		var key apikeys.APIKey
		_ = json.Unmarshal(rec.Body.Bytes(), &key)
		return rec, key
	}
	listed := func(id core.Identity) []string {
		t.Helper()
		rec := rest(id, http.MethodGet, "/v1/api-keys?ownerId="+ws.ID, "")
		var out []apikeys.APIKey
		if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &out) != nil {
			t.Fatalf("%s lists keys: %d %s", id.Subject, rec.Code, rec.Body)
		}
		ids := []string{}
		for _, k := range out {
			ids = append(ids, k.ID)
		}
		slices.Sort(ids)
		return ids
	}
	sorted := func(ids ...string) []string { slices.Sort(ids); return ids }

	// A service to act on, created by the Admin.
	rec := rest(session(admin), http.MethodPost, "/v1/services", `{"name":"web-`+run+`","image":{"imagePath":"nginx"},"ownerId":"`+ws.ID+`"}`)
	var created struct{ Service struct{ ID string } }
	if rec.Code != http.StatusCreated || json.Unmarshal(rec.Body.Bytes(), &created) != nil || created.Service.ID == "" {
		t.Fatalf("Admin creates a service: %d %s", rec.Code, rec.Body)
	}
	service := created.Service.ID

	// t002: each member mints a key with no permissions (All), seated at
	// their own role in the row and in OpenFGA, and at nothing above it.
	keyOfRole := map[string]string{}
	for subject, role := range map[string]string{viewer: "viewer", contrib: "contributor", bill: "billing"} {
		rec, key := mint(subject, "")
		if rec.Code != http.StatusCreated || !key.Permissions.All {
			t.Fatalf("%s mints a key: %d %s", role, rec.Code, rec.Body)
		}
		row, err := st.GetTenantMember(ctx, ws.ID, key.ID)
		if err != nil || row.Role != role || row.Kind != store.MemberKindMachine {
			t.Fatalf("%s's key binding = %+v (%v), want a %s machine row", role, row, err, role)
		}
		if !tupleHeld(key.ID, role) || tupleHeld(key.ID, "developer") || tupleHeld(key.ID, "admin") {
			t.Fatalf("%s's key is not seated at exactly %s", role, role)
		}
		keyOfRole[role] = key.ID
	}

	// I9: each key gets exactly its creator's answer, call by call, and the
	// Definition of done's named answers hold.
	type call struct{ name, method, path, body string }
	calls := []call{
		{"list services", http.MethodGet, "/v1/services?ownerId=" + ws.ID, ""},
		{"service logs", http.MethodGet, "/v1/logs?ownerId=" + ws.ID + "&resource=" + service, ""},
		{"restart", http.MethodPost, "/v1/services/" + service + "/restart", ""},
		{"create service", http.MethodPost, "/v1/services", `{"name":"x-` + run + `","image":{"imagePath":"nginx"},"ownerId":"` + ws.ID + `"}`},
		{"env var read", http.MethodGet, "/v1/services/" + service + "/env-vars", ""},
		{"deploy", http.MethodPost, "/v1/services/" + service + "/deploys", `{}`},
	}
	codes := map[string]map[string]int{}
	for creator, role := range map[string]string{viewer: "viewer", contrib: "contributor", bill: "billing"} {
		codes[role] = map[string]int{}
		for _, c := range calls {
			person := rest(session(creator), c.method, c.path, c.body).Code
			key := rest(keyOf(keyOfRole[role]), c.method, c.path, c.body).Code
			if person != key {
				t.Errorf("%s: the key's %s = %d, its creator's %d", role, c.name, key, person)
			}
			codes[role][c.name] = key
		}
	}
	for _, want := range []struct {
		role, call string
		allowed    bool
	}{
		{"viewer", "list services", true}, {"viewer", "service logs", false}, {"viewer", "restart", false}, {"viewer", "create service", false},
		{"contributor", "service logs", true}, {"contributor", "restart", true}, {"contributor", "create service", false}, {"contributor", "env var read", false},
		{"billing", "list services", true}, {"billing", "service logs", false}, {"billing", "deploy", false},
	} {
		if got := codes[want.role][want.call]; (got != http.StatusForbidden) != want.allowed {
			t.Errorf("%s's key %s = %d, want allowed=%v", want.role, want.call, got, want.allowed)
		}
	}
	// Billing data: the Billing member's key reads the Build Pipeline
	// settings (can_manage_billing); the Viewer's is refused them.
	pipeline := fmt.Sprintf(`{ pipelineSettings(workspaceId: %q) { workspaceId } }`, ws.ID)
	if out := graphqlDo(t, srv, keyOf(keyOfRole["billing"]), pipeline); out["errors"] != nil {
		t.Errorf("Billing's key reads the pipeline settings: %v", out)
	}
	if out := graphqlDo(t, srv, keyOf(keyOfRole["viewer"]), pipeline); graphqlCode(out) != "FORBIDDEN" {
		t.Errorf("Viewer's key reads the pipeline settings: %v", out)
	}
	// The same on GraphQL and MCP: the Viewer's key reads and is refused a
	// restart; the Contributor's restarts.
	vk := keyOf(keyOfRole["viewer"])
	if out := graphqlDo(t, srv, vk, fmt.Sprintf(`{ services(ownerId: %q) { id } }`, ws.ID)); out["errors"] != nil {
		t.Errorf("Viewer's key GraphQL services = %v", out)
	}
	if out := graphqlDo(t, srv, vk, fmt.Sprintf(`mutation { resumeService(id: %q) { id } }`, service)); graphqlCode(out) != "FORBIDDEN" {
		t.Errorf("Viewer's key GraphQL resumeService = %v", out)
	}
	if res := mcpCall(t, srv, vk, "list_services", map[string]any{}); res.IsError {
		t.Errorf("Viewer's key MCP list_services = %s", mcpText(res))
	}
	if res := mcpCall(t, srv, vk, "restart_service", map[string]any{"serviceId": service}); !res.IsError || !strings.Contains(strings.ToLower(mcpText(res)), "forbidden") {
		t.Errorf("Viewer's key MCP restart_service = %s", mcpText(res))
	}
	if res := mcpCall(t, srv, keyOf(keyOfRole["contributor"]), "restart_service", map[string]any{"serviceId": service}); res.IsError {
		t.Errorf("Contributor's key MCP restart_service = %s", mcpText(res))
	}

	// The role caps a chosen grant: a Viewer names no Write, Run or Read &
	// write level; Read only passes.
	for _, grant := range []string{
		`{"grants":[{"permission":"services","level":"read_write"}]}`,
		`{"grants":[{"permission":"deploys_run","level":"run"}]}`,
		`{"grants":[{"permission":"services_env","level":"write"}]}`,
	} {
		if rec, _ := mint(viewer, grant); rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), core.KeyPermissionAboveRoleCode) {
			t.Errorf("Viewer mints %s = %d %s", grant, rec.Code, rec.Body)
		}
	}
	// A workspace mints five keys in a burst (apikeys.NewCreationRateLimiter),
	// so the rest of the test mints two more, on GraphQL and MCP.
	out := graphqlDo(t, srv, session(viewer), fmt.Sprintf(`mutation { createApiKey(name: "read", ownerId: %q, permissions: {grants: [{permission: "services", level: "read"}, {permission: "deploys", level: "read"}]}) { id permissions { all } } }`, ws.ID))
	readOnly, _ := out["data"].(map[string]any)["createApiKey"].(map[string]any)["id"].(string)
	if readOnly == "" {
		t.Fatalf("Viewer's Read only key over GraphQL: %v", out)
	}

	// I10: below Developer, the list is one's own keys, on every surface; a
	// Developer and an Admin see every key.
	viewerOwn := sorted(keyOfRole["viewer"], readOnly)
	if got := listed(session(viewer)); !slices.Equal(got, viewerOwn) {
		t.Errorf("Viewer lists %v, want their own %v", got, viewerOwn)
	}
	if got := listed(session(viewer2)); len(got) != 0 {
		t.Errorf("a Viewer with no keys lists %v", got)
	}
	out = graphqlDo(t, srv, session(viewer), fmt.Sprintf(`{ apiKeys(ownerId: %q) { id } }`, ws.ID))
	if gqlKeys, _ := out["data"].(map[string]any)["apiKeys"].([]any); len(gqlKeys) != len(viewerOwn) {
		t.Errorf("Viewer's GraphQL apiKeys = %v, want %d keys", out, len(viewerOwn))
	}
	if res := mcpCall(t, srv, session(viewer), "list_api_keys", map[string]any{"workspaceId": ws.ID}); res.IsError ||
		strings.Contains(mcpText(res), keyOfRole["contributor"]) || !strings.Contains(mcpText(res), readOnly) {
		t.Errorf("Viewer's MCP list_api_keys = %s", mcpText(res))
	}
	every := sorted(keyOfRole["viewer"], readOnly, keyOfRole["contributor"], keyOfRole["billing"])
	for _, who := range []string{dev, admin} {
		if got := listed(session(who)); !slices.Equal(got, every) {
			t.Errorf("%s lists %v, want every key %v", who, got, every)
		}
	}

	// The Viewer edits their own key's grant and is refused another
	// member's key, to edit or to revoke, with the 403 a Viewer always got.
	patch := `{"permissions":{"grants":[{"permission":"services","level":"read"}]}}`
	if rec := rest(session(viewer), http.MethodPatch, "/v1/api-keys/"+readOnly+"?ownerId="+ws.ID, patch); rec.Code != http.StatusOK {
		t.Errorf("Viewer edits their own key: %d %s", rec.Code, rec.Body)
	}
	if rec := rest(session(viewer), http.MethodPatch, "/v1/api-keys/"+readOnly+"?ownerId="+ws.ID,
		`{"permissions":{"grants":[{"permission":"services","level":"read_write"}]}}`); rec.Code != http.StatusForbidden {
		t.Errorf("Viewer widens their own key past Viewer: %d %s", rec.Code, rec.Body)
	}
	other := keyOfRole["contributor"]
	for _, c := range []call{
		{"edit", http.MethodPatch, "/v1/api-keys/" + other + "?ownerId=" + ws.ID, patch},
		{"revoke", http.MethodDelete, "/v1/api-keys/" + other + "?ownerId=" + ws.ID, ""},
	} {
		if rec := rest(session(viewer), c.method, c.path, c.body); rec.Code != http.StatusForbidden {
			t.Errorf("Viewer's %s of another member's key = %d %s", c.name, rec.Code, rec.Body)
		}
	}
	if res := mcpCall(t, srv, session(viewer), "revoke_api_key", map[string]any{"keyId": other}); !res.IsError {
		t.Errorf("Viewer's MCP revoke of another member's key succeeded: %s", mcpText(res))
	}
	if out := graphqlDo(t, srv, session(viewer), fmt.Sprintf(`mutation { revokeApiKey(id: %q) }`, other)); graphqlCode(out) != "FORBIDDEN" {
		t.Errorf("Viewer's GraphQL revoke of another member's key = %v", out)
	}
	if _, ok := keys.keys[other]; !ok {
		t.Fatal("a refused revoke deleted the Contributor's key")
	}
	// Mint and revoke one's own on each surface: the Viewer's Read only key
	// over MCP, a key minted over MCP over GraphQL, and the Billing member's
	// over REST.
	if res := mcpCall(t, srv, session(viewer), "revoke_api_key", map[string]any{"keyId": readOnly}); res.IsError {
		t.Errorf("Viewer revokes their own key over MCP: %s", mcpText(res))
	}
	res := mcpCall(t, srv, session(viewer), "create_api_key", map[string]any{"name": "mcp", "workspaceId": ws.ID})
	var mcpKey apikeys.APIKey
	if res.IsError || json.Unmarshal([]byte(mcpText(res)), &mcpKey) != nil || mcpKey.ID == "" {
		t.Fatalf("Viewer mints over MCP: %s", mcpText(res))
	}
	if out := graphqlDo(t, srv, session(viewer), fmt.Sprintf(`mutation { revokeApiKey(id: %q) }`, mcpKey.ID)); out["errors"] != nil {
		t.Errorf("Viewer revokes their own key over GraphQL: %v", out)
	}
	if rec := rest(session(bill), http.MethodDelete, "/v1/api-keys/"+keyOfRole["billing"]+"?ownerId="+ws.ID, ""); rec.Code != http.StatusNoContent {
		t.Errorf("the Billing member revokes their own key over REST: %d %s", rec.Code, rec.Body)
	}
	for _, id := range []string{readOnly, mcpKey.ID, keyOfRole["billing"]} {
		if _, ok := keys.keys[id]; ok {
			t.Errorf("revoked key %s is still in the registry", id)
		}
		for _, role := range members.Roles {
			if tupleHeld(id, role) {
				t.Errorf("revoked key %s still holds %s", id, role)
			}
		}
	}

	// I4, I5: a Viewer's key mints no credential and re-roles no member.
	for _, c := range []call{
		{"key mint", http.MethodPost, "/v1/api-keys", `{"name":"again","ownerId":"` + ws.ID + `"}`},
		{"SSH key", http.MethodPost, "/v1/ssh-keys", `{"name":"k","publicKey":"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIOMqqnkVzrm0SdG6UOoqKLsabgH5C9okWi0dh2l9GKJl x"}`},
		{"member role", http.MethodPatch, "/v1/workspaces/" + ws.ID + "/members/" + viewer2, `{"role":"ADMIN"}`},
	} {
		if rec := rest(vk, c.method, c.path, c.body); rec.Code != http.StatusForbidden {
			t.Errorf("a Viewer's key %s = %d %s, want 403", c.name, rec.Code, rec.Body)
		}
	}

	// The key follows its creator: promoted to Developer, the Viewer's key
	// creates a service on its next write; demoted back, it is refused;
	// removed from the workspace, the key is revoked.
	role := func(subject, to string) {
		t.Helper()
		if rec := rest(session(admin), http.MethodPatch, "/v1/workspaces/"+ws.ID+"/members/"+subject, `{"role":"`+to+`"}`); rec.Code != http.StatusOK {
			t.Fatalf("set %s to %s: %d %s", subject, to, rec.Code, rec.Body)
		}
	}
	createAs := func(id core.Identity, name string) int {
		return rest(id, http.MethodPost, "/v1/services", `{"name":"`+name+`-`+run+`","image":{"imagePath":"nginx"},"ownerId":"`+ws.ID+`"}`).Code
	}
	role(viewer, "DEVELOPER")
	if code := createAs(vk, "promoted"); code != http.StatusCreated {
		t.Errorf("the promoted Viewer's key creates a service = %d", code)
	}
	role(viewer, "VIEWER")
	if code := createAs(vk, "demoted"); code != http.StatusForbidden {
		t.Errorf("the demoted Viewer's key creates a service = %d", code)
	}
	if rec := rest(session(admin), http.MethodDelete, "/v1/workspaces/"+ws.ID+"/members/"+viewer, ""); rec.Code >= 300 {
		t.Fatalf("remove the Viewer: %d %s", rec.Code, rec.Body)
	}
	if _, ok := keys.keys[keyOfRole["viewer"]]; ok {
		t.Error("the removed Viewer's key is still in the registry")
	}
	if tupleHeld(keyOfRole["viewer"], "viewer") {
		t.Error("the removed Viewer's key still holds its tuple")
	}
	if rec := rest(vk, http.MethodGet, "/v1/services?ownerId="+ws.ID, ""); rec.Code != http.StatusForbidden {
		t.Errorf("the removed Viewer's key lists services: %d", rec.Code)
	}
}
