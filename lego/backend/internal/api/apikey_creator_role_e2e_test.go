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
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bex-co/bex/lego/backend/internal/apikeys"
	"github.com/bex-co/bex/lego/backend/internal/authz"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/members"
	"github.com/bex-co/bex/lego/backend/internal/store"
	"github.com/bex-co/bex/lego/backend/internal/tenancy"
	"github.com/bex-co/bex/lego/backend/internal/testenv"
)

// apikey_creator_role_e2e_test.go is w9/m481's proof against REAL Postgres and
// REAL OpenFGA, through the composed REST, GraphQL and MCP surfaces: an API
// key acts with its creator's current member role in the workspace it was
// minted in, as on Render. An Admin's key makes the four Admin-only writes
// Render's Terraform provider makes (a webhook, a registry credential, the
// owner notification settings, a protected environment); a Developer's key is
// refused each; once the Admin is demoted their key is refused on its next
// request, even when the key's own tuple failed to move.
//
//	BEX_TEST_DB_URI=postgres://…/postgres?sslmode=disable \
//	BEX_TEST_OPENFGA_URL=http://127.0.0.1:58085 \
//	  go test ./internal/api -run TestAPIKeyActsWithItsCreatorsRoleE2E -v
//
// Additive like its siblings: every subject is unique to the run.

// creatorKeys is a Hydra key registry that records who minted each key, and
// answers core.KeyCreatorResolver from it as twofactor.Gate does from Hydra.
type creatorKeys struct {
	mu   sync.Mutex
	run  string
	n    int
	keys map[string]apikeys.APIKey
}

func (k *creatorKeys) Create(_ context.Context, name, createdBy string, _ core.SignIn) (apikeys.APIKey, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.n++
	id := fmt.Sprintf("key%d-%s", k.n, k.run)
	key := apikeys.APIKey{ID: id, Name: name, CreatedBy: createdBy}
	k.keys[id] = key
	key.Secret = "secret-" + id
	return key, nil
}

func (k *creatorKeys) List(context.Context) ([]apikeys.APIKey, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	out := []apikeys.APIKey{}
	for _, key := range k.keys {
		out = append(out, key)
	}
	return out, nil
}

func (k *creatorKeys) Delete(_ context.Context, id string) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	delete(k.keys, id)
	return nil
}

func (*creatorKeys) Touch(context.Context, string, time.Time) error { return nil }

func (k *creatorKeys) KeyOwner(_ context.Context, id string) (string, bool) {
	k.mu.Lock()
	defer k.mu.Unlock()
	key, ok := k.keys[id]
	return key.CreatedBy, ok && key.CreatedBy != ""
}

func (k *creatorKeys) APIKeyCreator(_ context.Context, clientID string) (string, bool, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	key, ok := k.keys[clientID]
	return key.CreatedBy, ok, nil
}

// flakyRevoker is OpenFGA's revoke with one subject's revokes failing, the
// partial failure invariant I1 has to survive.
type flakyRevoker struct {
	members.RoleRevoker
	mu   sync.Mutex
	fail string
}

func (f *flakyRevoker) failing(subject string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.fail = subject
}

func (f *flakyRevoker) RevokeWorkspaceMember(ctx context.Context, tenantID, subject, relation string) error {
	f.mu.Lock()
	fail := f.fail
	f.mu.Unlock()
	if fail != "" && subject == fail {
		return errors.New("openfga: injected revoke failure")
	}
	return f.RoleRevoker.RevokeWorkspaceMember(ctx, tenantID, subject, relation)
}

func TestAPIKeyActsWithItsCreatorsRoleE2E(t *testing.T) {
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
	revoker := &flakyRevoker{RoleRevoker: roles}

	ws, err := st.CreateWorkspace(ctx, "keyrole-"+run, store.PlanPro, admin)
	if err != nil {
		t.Fatal(err)
	}
	other, err := st.CreateWorkspace(ctx, "keyrole-other-"+run, store.PlanPro, admin)
	if err != nil {
		t.Fatal(err)
	}
	seat := func(workspace, subject, role string) {
		t.Helper()
		if err := roles.GrantWorkspaceRole(ctx, workspace, "user:"+subject, role); err != nil {
			t.Fatalf("grant %s %s: %v", subject, role, err)
		}
	}
	seat(ws.ID, admin, "admin")
	seat(other.ID, admin, "admin")
	for _, m := range []struct{ subject, role string }{{admin2, "admin"}, {dev, "developer"}} {
		if err := st.AddMember(ctx, m.subject, ws.ID, m.role); err != nil {
			t.Fatal(err)
		}
		seat(ws.ID, m.subject, m.role)
	}

	keys := &creatorKeys{run: run, keys: map[string]apikeys.APIKey{}}
	resolver := tenancy.NewTenantService(st, roles)
	base := &core.Base{
		Client: fakeClient(), Namespace: "default",
		Authz: checker, Workspace: resolver, KeyCreators: keys,
	}
	srv := NewServer(base, Deps{
		Store: st, WorkspaceStore: st, MembersStore: st,
		MembersGranter: roles, MembersRevoker: revoker,
		APIKeys: keys, KeyBinder: resolver,
		WebhookStore: st, RegistryCredsStore: st, Secrets: newMemSecretStore(),
		NotificationsStore: st, ProjectsStore: st, EnvironmentsStore: st, EnvGroupsStore: st,
	})
	mux := srv.restHandler()
	schema, err := srv.newSchema()
	if err != nil {
		t.Fatal(err)
	}
	srv.schema = schema // Handler() compiles it; the test drives the GraphQL handler directly

	session := func(subject string) core.Identity { return core.Identity{Subject: subject, Method: "session"} }
	keyOf := func(id string) core.Identity {
		return core.Identity{Subject: id, ClientID: id, Method: core.MethodOAuth2}
	}
	rest := func(id core.Identity, method, path, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req.WithContext(core.WithIdentity(ctx, id)))
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
	mint := func(creator string) string {
		t.Helper()
		rec := rest(session(creator), http.MethodPost, "/v1/api-keys", `{"name":"terraform","ownerId":"`+ws.ID+`"}`)
		var key apikeys.APIKey
		if rec.Code != http.StatusCreated || json.Unmarshal(rec.Body.Bytes(), &key) != nil {
			t.Fatalf("%s mints a key: %d %s", creator, rec.Code, rec.Body)
		}
		return key.ID
	}
	rec := rest(session(admin), http.MethodPost, "/v1/projects", `{"name":"p-`+run+`","ownerId":"`+ws.ID+`","environments":[{"name":"staging"}]}`)
	var project struct{ ID string }
	if rec.Code != http.StatusCreated || json.Unmarshal(rec.Body.Bytes(), &project) != nil {
		t.Fatalf("project: %d %s", rec.Code, rec.Body)
	}

	// t002: each key is seated at its creator's role, in the row and in OpenFGA.
	keyA, keyD := mint(admin), mint(dev)
	for key, want := range map[string]string{keyA: "admin", keyD: "developer"} {
		row, err := st.GetTenantMember(ctx, ws.ID, key)
		if err != nil || row.Role != want || row.Kind != store.MemberKindMachine {
			t.Fatalf("%s's binding = %+v (%v), want a %s machine row", key, row, err, want)
		}
		if !tupleHeld(key, want) {
			t.Fatalf("%s holds no %s tuple", key, want)
		}
	}
	if tupleHeld(keyD, "admin") {
		t.Fatal("a Developer's key holds an admin tuple")
	}

	// The four Admin-only writes Render's Terraform provider makes, on each
	// surface. write answers whether the write was allowed, and fails the test
	// on anything that is neither an allow nor a 403.
	type surface struct {
		name  string
		write func(id core.Identity, n int) map[string]bool
	}
	n := 0
	surfaces := []surface{
		{"REST", func(id core.Identity, n int) map[string]bool {
			calls := map[string]*httptest.ResponseRecorder{
				"webhook": rest(id, http.MethodPost, "/v1/webhooks",
					fmt.Sprintf(`{"ownerId":%q,"name":"hook-%d","url":"https://example.com/hook","enabled":true,"eventFilter":[]}`, ws.ID, n)),
				"registry credential": rest(id, http.MethodPost, "/v1/registrycredentials",
					fmt.Sprintf(`{"ownerId":%q,"name":"cred-%d","registry":"DOCKER","username":"u","authToken":"t"}`, ws.ID, n)),
				"owner notification settings": rest(id, http.MethodPatch, "/v1/notification-settings/owners/"+ws.ID,
					`{"emailEnabled":true}`),
				"protected environment": rest(id, http.MethodPost, "/v1/environments",
					fmt.Sprintf(`{"name":"prod-%d","projectId":%q,"protectedStatus":"protected"}`, n, project.ID)),
			}
			out := map[string]bool{}
			for what, rec := range calls {
				switch {
				case rec.Code < 300:
					out[what] = true
				case rec.Code == http.StatusForbidden:
					out[what] = false
				default:
					t.Fatalf("REST %s as %s = %d %s", what, id.Subject, rec.Code, rec.Body)
				}
			}
			if out["webhook"] {
				if body := calls["webhook"].Body.String(); !strings.Contains(body, `"secret":"whsec_`) {
					t.Errorf("an allowed webhook create answered no signing secret: %s", body)
				}
			}
			return out
		}},
		{"GraphQL", func(id core.Identity, n int) map[string]bool {
			calls := map[string]string{
				"webhook":                     fmt.Sprintf(`mutation { createWebhookEndpoint(ownerId: %q, name: "ghook-%d", url: "https://example.com/hook", eventTypes: []) { id } }`, ws.ID, n),
				"registry credential":         fmt.Sprintf(`mutation { createRegistryCredential(ownerId: %q, name: "gcred-%d", host: "docker.io", username: "u", authToken: "t") { id } }`, ws.ID, n),
				"owner notification settings": fmt.Sprintf(`mutation { updateOwnerNotificationSettings(ownerId: %q, input: {emailEnabled: true}) { emailEnabled } }`, ws.ID),
				"protected environment":       fmt.Sprintf(`mutation { createEnvironment(name: "gprod-%d", projectId: %q, protectedStatus: "protected") { id } }`, n, project.ID),
			}
			out := map[string]bool{}
			for what, query := range calls {
				answer := graphqlDo(t, srv, id, query)
				switch code := graphqlCode(answer); code {
				case "":
					if errs, _ := answer["errors"].([]any); len(errs) > 0 {
						t.Fatalf("GraphQL %s as %s = %v", what, id.Subject, answer)
					}
					out[what] = true
				case "FORBIDDEN":
					out[what] = false
				default:
					t.Fatalf("GraphQL %s as %s = %v", what, id.Subject, answer)
				}
			}
			return out
		}},
		{"MCP", func(id core.Identity, n int) map[string]bool {
			calls := map[string]struct {
				tool string
				args map[string]any
			}{
				"webhook":                     {"create_webhook_endpoint", map[string]any{"name": fmt.Sprintf("mhook-%d", n), "url": "https://example.com/hook", "eventTypes": []string{}, "enabled": true}},
				"registry credential":         {"create_registry_credential", map[string]any{"name": fmt.Sprintf("mcred-%d", n), "host": "docker.io", "username": "u", "authToken": "t"}},
				"owner notification settings": {"update_owner_notification_settings", map[string]any{"emailEnabled": true}},
				"protected environment":       {"create_environment", map[string]any{"name": fmt.Sprintf("mprod-%d", n), "projectId": project.ID, "protectedStatus": "protected"}},
			}
			out := map[string]bool{}
			for what, call := range calls {
				res := mcpCall(t, srv, id, call.tool, call.args)
				text := mcpText(res)
				switch {
				case !res.IsError:
					out[what] = true
				case strings.Contains(strings.ToLower(text), "forbidden"):
					out[what] = false
				default:
					t.Fatalf("MCP %s as %s = %s", what, id.Subject, text)
				}
			}
			return out
		}},
	}
	expect := func(label string, id core.Identity, allowed bool) {
		t.Helper()
		for _, s := range surfaces {
			n++
			for what, got := range s.write(id, n) {
				if got != allowed {
					t.Errorf("%s: %s %s allowed=%v, want %v", label, s.name, what, got, allowed)
				}
			}
		}
	}

	expect("an Admin's key", keyOf(keyA), true)
	expect("a Developer's key", keyOf(keyD), false)

	// I2: the Admin is an Admin of another workspace too; their key is not.
	if rec := rest(keyOf(keyA), http.MethodGet, "/v1/services?ownerId="+other.ID, ""); rec.Code != http.StatusForbidden {
		t.Errorf("the key listed its creator's other workspace: %d %s", rec.Code, rec.Body)
	}

	// I1 under a failed revoke: the key's own tuple cannot move, the Admin is
	// demoted anyway. The key's row and queued intent follow at once; its
	// admin tuple is stranded; and it is refused every Admin-only write on its
	// very next request.
	revoker.failing("user:" + keyA)
	if rec := rest(session(admin2), http.MethodPatch, "/v1/workspaces/"+ws.ID+"/members/"+admin, `{"role":"DEVELOPER"}`); rec.Code != http.StatusOK {
		t.Fatalf("demote: %d %s", rec.Code, rec.Body)
	}
	if row, _ := st.GetTenantMember(ctx, ws.ID, keyA); row.Role != "developer" {
		t.Errorf("the demoted Admin's key row = %s, want developer", row.Role)
	}
	if !tupleHeld(keyA, "admin") {
		t.Fatal("the injected revoke failure did not strand the key's admin tuple; the test proves nothing")
	}
	if pending, err := st.HasPendingRoleReconciliation(ctx, ws.ID, keyA); err != nil || !pending {
		t.Fatalf("the key's failed re-seat left no queued intent: %v %v", pending, err)
	}
	expect("the demoted Admin's key (stale admin tuple)", keyOf(keyA), false)

	// A retry of the same role change converges the key, as it does a member.
	revoker.failing("")
	if rec := rest(session(admin2), http.MethodPatch, "/v1/workspaces/"+ws.ID+"/members/"+admin, `{"role":"DEVELOPER"}`); rec.Code != http.StatusOK {
		t.Fatalf("retry the demotion: %d %s", rec.Code, rec.Body)
	}
	if tupleHeld(keyA, "admin") || !tupleHeld(keyA, "developer") {
		t.Error("a retried role change left the key's tuples unconverged")
	}
	if pending, _ := st.HasPendingRoleReconciliation(ctx, ws.ID, keyA); pending {
		t.Error("the converged key still has a queued intent")
	}

	// Promoted back, the key follows up as Render's does; the Developer's key
	// never moved.
	if rec := rest(session(admin2), http.MethodPatch, "/v1/workspaces/"+ws.ID+"/members/"+admin, `{"role":"ADMIN"}`); rec.Code != http.StatusOK {
		t.Fatalf("promote: %d %s", rec.Code, rec.Body)
	}
	if !tupleHeld(keyA, "admin") || tupleHeld(keyA, "developer") {
		t.Error("the re-promoted Admin's key did not follow to exactly admin")
	}
	if tupleHeld(keyD, "admin") {
		t.Error("another member's key moved")
	}
	expect("the re-promoted Admin's key", keyOf(keyA), true)

	// I4/I5: the key still mints no credential and is no member to re-role.
	for _, call := range []struct{ method, path, body string }{
		{http.MethodPost, "/v1/api-keys", `{"name":"again","ownerId":"` + ws.ID + `"}`},
		{http.MethodPost, "/v1/workspaces/" + ws.ID + "/members", `{"email":"x-` + run + `@example.com","role":"ADMIN"}`},
		{http.MethodPatch, "/v1/workspaces/" + ws.ID + "/members/" + dev, `{"role":"ADMIN"}`},
	} {
		if rec := rest(keyOf(keyA), call.method, call.path, call.body); rec.Code != http.StatusForbidden {
			t.Errorf("an Admin's key %s %s = %d %s, want 403", call.method, call.path, rec.Code, rec.Body)
		}
	}
	if rec := rest(session(admin2), http.MethodPatch, "/v1/workspaces/"+ws.ID+"/members/"+keyA, `{"role":"VIEWER"}`); rec.Code == http.StatusOK {
		t.Errorf("a key was re-roled through the member verbs: %s", rec.Body)
	}

	// Revoked, the key loses every tuple whatever its role.
	if rec := rest(session(admin), http.MethodDelete, "/v1/api-keys/"+keyA+"?ownerId="+ws.ID, ""); rec.Code != http.StatusNoContent {
		t.Fatalf("revoke: %d %s", rec.Code, rec.Body)
	}
	for _, role := range members.Roles {
		if tupleHeld(keyA, role) {
			t.Errorf("the revoked key still holds %s", role)
		}
	}
}
