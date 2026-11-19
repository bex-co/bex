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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/apikeys"
	"github.com/bex-co/bex/lego/backend/internal/authz"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
	"github.com/bex-co/bex/lego/backend/internal/tenancy"
	"github.com/bex-co/bex/lego/backend/internal/testenv"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// apikey_postgres_user_e2e_test.go is w9/117's proof against REAL Postgres and
// REAL OpenFGA, through the composed REST, GraphQL and MCP surfaces: an API key
// whose creator may create a Postgres user does so and gets its password once,
// as Render's key with "Create and delete database credentials" does. A key
// whose grant lacks that permission, a key whose creator lacks can_create, a
// key with no recorded creator and a delegated third-party token are refused;
// a demoted or removed creator's key is refused on its next request.
//
//	BEX_TEST_DB_URI=postgres://…/postgres?sslmode=disable \
//	BEX_TEST_OPENFGA_URL=http://127.0.0.1:58085 \
//	  go test ./internal/api -run TestAPIKeyCreatesPostgresUsersAsItsCreatorE2E -v
func TestAPIKeyCreatesPostgresUsersAsItsCreatorE2E(t *testing.T) {
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
	admin, dev, contrib := "admin-"+run, "dev-"+run, "contrib-"+run
	checker := authz.NewOpenFGAChecker(fgaURL, os.Getenv("BEX_TEST_OPENFGA_TOKEN"))
	roles := checker.(store.MembershipGranter)
	ws, err := st.CreateWorkspace(ctx, "keypgusers-"+run, store.PlanScale, admin)
	if err != nil {
		t.Fatal(err)
	}
	if err := roles.GrantWorkspaceRole(ctx, ws.ID, "user:"+admin, "admin"); err != nil {
		t.Fatal(err)
	}
	for subject, role := range map[string]string{dev: "developer", contrib: "contributor"} {
		if err := st.AddMember(ctx, subject, ws.ID, role); err != nil {
			t.Fatal(err)
		}
		if err := roles.GrantWorkspaceRole(ctx, ws.ID, "user:"+subject, role); err != nil {
			t.Fatal(err)
		}
	}

	dbID := "dpg-" + strings.ToLower(run)
	kube := fakeClient(&appv1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{Name: dbID, Namespace: "default", Labels: map[string]string{core.LabelTenant: ws.ID, core.LabelWorkspace: ws.ID}},
		Spec:       appv1alpha1.DatabaseSpec{Name: "orders", Plan: "free"},
		Status:     appv1alpha1.DatabaseStatus{Phase: appv1alpha1.DBPhaseReady},
	})
	keys := &creatorKeys{run: run, keys: map[string]apikeys.APIKey{}}
	resolver := tenancy.NewTenantService(st, roles)
	base := &core.Base{
		Client: kube, Namespace: "default",
		Authz: checker, Workspace: resolver, KeyCreators: keys, KeyGrants: resolver,
	}
	srv := NewServer(base, Deps{
		Store: st, WorkspaceStore: st, MembersStore: st,
		MembersGranter: roles, MembersRevoker: roles,
		APIKeys: keys, KeyBinder: resolver,
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
	mint := func(creator, permissions string) string {
		t.Helper()
		body := `{"name":"pg-users","ownerId":"` + ws.ID + `"`
		if permissions != "" {
			body += `,"permissions":` + permissions
		}
		rec := rest(session(creator), http.MethodPost, "/v1/api-keys", body+"}")
		var key apikeys.APIKey
		if rec.Code != http.StatusCreated || json.Unmarshal(rec.Body.Bytes(), &key) != nil {
			t.Fatalf("%s mints a key: %d %s", creator, rec.Code, rec.Body)
		}
		return key.ID
	}
	logins := func() []string {
		t.Helper()
		var db appv1alpha1.Database
		if err := kube.Get(ctx, client.ObjectKey{Namespace: "default", Name: dbID}, &db); err != nil {
			t.Fatal(err)
		}
		var out []string
		for _, u := range db.Spec.Users {
			out = append(out, u.Name)
		}
		return out
	}
	n := 0
	nextLogin := func() string {
		n++
		return fmt.Sprintf("login_%d", n)
	}

	// Each surface creates one login and answers "" (and checks the password
	// came back) when allowed, else the refusal's code ("FORBIDDEN" for a bare
	// 403). Render's own credentials route reveals no password and has no
	// class gate (w5/m265); it is held to the creator and the grant all the same.
	type surface struct {
		name       string
		classGated bool
		create     func(id core.Identity, login string) string
	}
	restCode := func(rec *httptest.ResponseRecorder, want int, needPassword bool) string {
		switch {
		case rec.Code == want:
			if needPassword {
				var res struct{ Name, Password string }
				if json.Unmarshal(rec.Body.Bytes(), &res) != nil || res.Password == "" {
					t.Fatalf("REST created a login without its password: %s", rec.Body)
				}
			}
			return ""
		case rec.Code == http.StatusForbidden:
			var body struct {
				Code string `json:"code"`
			}
			if json.Unmarshal(rec.Body.Bytes(), &body) == nil && body.Code != "" {
				return body.Code
			}
			return "FORBIDDEN"
		}
		t.Fatalf("REST answered %d %s", rec.Code, rec.Body)
		return ""
	}
	surfaces := []surface{
		{"REST /v1/postgres/{id}/users", true, func(id core.Identity, login string) string {
			return restCode(rest(id, http.MethodPost, "/v1/postgres/"+dbID+"/users", `{"name":"`+login+`"}`), http.StatusCreated, true)
		}},
		{"GraphQL createDatabaseUser", true, func(id core.Identity, login string) string {
			out := graphqlDo(t, srv, id, fmt.Sprintf(`mutation { createDatabaseUser(id: %q, name: %q) { name password } }`, dbID, login))
			if code := graphqlCode(out); code != "" || out["errors"] != nil {
				if code == "" {
					t.Fatalf("GraphQL answered %v", out)
				}
				return code
			}
			created, _ := out["data"].(map[string]any)["createDatabaseUser"].(map[string]any)
			if pw, _ := created["password"].(string); pw == "" {
				t.Fatalf("GraphQL created a login without its password: %v", out)
			}
			return ""
		}},
		{"MCP create_postgres_user", true, func(id core.Identity, login string) string {
			res := mcpCall(t, srv, id, "create_postgres_user", map[string]any{"postgresId": dbID, "name": login})
			text := mcpText(res)
			switch {
			case !res.IsError:
				if !strings.Contains(text, `"password":"`) || strings.Contains(text, `"password":""`) {
					t.Fatalf("MCP created a login without its password: %s", text)
				}
				return ""
			case strings.Contains(text, core.KeyPermissionDeniedCode) || strings.Contains(text, "not granted"):
				return core.KeyPermissionDeniedCode
			case strings.Contains(strings.ToLower(text), "forbidden"):
				return "FORBIDDEN"
			}
			t.Fatalf("MCP answered %s", text)
			return ""
		}},
		{"REST /v1/postgres/{id}/credentials (Render)", false, func(id core.Identity, login string) string {
			return restCode(rest(id, http.MethodPost, "/v1/postgres/"+dbID+"/credentials", `{"username":"`+login+`"}`), http.StatusAccepted, false)
		}},
	}
	allowed := func(who string, id core.Identity) {
		t.Helper()
		for _, s := range surfaces {
			login := nextLogin()
			if code := s.create(id, login); code != "" {
				t.Fatalf("%s: %s = %s, want the login created", s.name, who, code)
			}
			if !slices.Contains(logins(), login) {
				t.Fatalf("%s: %s's login %s is not on the Database", s.name, who, login)
			}
		}
	}
	refused := func(who string, id core.Identity, want string, classOnly bool) {
		t.Helper()
		for _, s := range surfaces {
			if classOnly && !s.classGated {
				continue
			}
			login := nextLogin()
			if code := s.create(id, login); code != want {
				t.Errorf("%s: %s = %q, want %q", s.name, who, code, want)
			}
			if slices.Contains(logins(), login) {
				t.Errorf("%s: %s's refused login %s is on the Database", s.name, who, login)
			}
		}
	}

	// Five mints, the workspace's creation burst.
	keyD := mint(dev, "")
	keyCredWrite := mint(dev, `{"grants":[{"permission":"postgres_credentials","level":"write"}]}`)
	keyPgOnly := mint(dev, `{"grants":[{"permission":"postgres","level":"read_write"},{"permission":"postgres_credentials","level":"read"}]}`)
	keyC := mint(contrib, "")
	keyLegacy := mint(dev, "")
	// keyLegacy as minted before the created-by stamp: Hydra names no creator,
	// so I3 binds it as the Developer it never leaves, can_create included.
	keys.mu.Lock()
	legacy := keys.keys[keyLegacy]
	legacy.CreatedBy = ""
	keys.keys[keyLegacy] = legacy
	keys.mu.Unlock()

	// A Developer's key creates logins on every surface, and a key granted
	// only "Create and delete database credentials" does too.
	allowed("a Developer's key", keyOf(keyD))
	allowed("a Postgres credentials Write key", keyOf(keyCredWrite))

	// A key whose grant lacks that permission is refused under a Developer.
	refused("a Postgres Read & write key", keyOf(keyPgOnly), core.KeyPermissionDeniedCode, false)
	// A Contributor holds no can_create, so neither does their key.
	refused("a Contributor's key", keyOf(keyC), "FORBIDDEN", false)
	// The class gate: a key with no recorded creator, and a human token from a
	// client bex did not provision, though both hold can_create.
	refused("a key with no recorded creator", keyOf(keyLegacy), "FORBIDDEN", true)
	refused("a delegated third-party token", core.Identity{
		Subject: dev, Method: core.MethodOAuth2, ClientID: "dcr-agent-" + run, Human: true, CanonicalScopes: core.ScopeWrite,
	}, "FORBIDDEN", true)

	// The Developer is demoted: their key is refused on its very next request,
	// on every surface, and follows them back up.
	setRole := func(subject, role string) {
		t.Helper()
		if rec := rest(session(admin), http.MethodPatch, "/v1/workspaces/"+ws.ID+"/members/"+subject, `{"role":"`+role+`"}`); rec.Code != http.StatusOK {
			t.Fatalf("set %s %s: %d %s", subject, role, rec.Code, rec.Body)
		}
	}
	setRole(dev, "CONTRIBUTOR")
	refused("the demoted Developer's key", keyOf(keyD), "FORBIDDEN", false)
	setRole(dev, "DEVELOPER")
	allowed("the re-promoted Developer's key", keyOf(keyD))

	// The Developer is removed: their key is refused from then on.
	if rec := rest(session(admin), http.MethodDelete, "/v1/workspaces/"+ws.ID+"/members/"+dev, ""); rec.Code >= 300 {
		t.Fatalf("remove %s: %d %s", dev, rec.Code, rec.Body)
	}
	login := nextLogin()
	for path, field := range map[string]string{"/users": "name", "/credentials": "username"} {
		rec := rest(keyOf(keyD), http.MethodPost, "/v1/postgres/"+dbID+path, `{"`+field+`":"`+login+`"}`)
		if rec.Code != http.StatusForbidden && rec.Code != http.StatusUnauthorized {
			t.Errorf("REST %s: the removed Developer's key = %d %s", path, rec.Code, rec.Body)
		}
	}
	if out := graphqlDo(t, srv, keyOf(keyD), fmt.Sprintf(`mutation { createDatabaseUser(id: %q, name: %q) { name } }`, dbID, login)); out["errors"] == nil {
		t.Errorf("GraphQL: the removed Developer's key = %v", out)
	}
	if res := mcpCall(t, srv, keyOf(keyD), "create_postgres_user", map[string]any{"postgresId": dbID, "name": login}); !res.IsError {
		t.Errorf("MCP: the removed Developer's key = %s", mcpText(res))
	}
	if slices.Contains(logins(), login) {
		t.Errorf("the removed Developer's key created %s", login)
	}
}
