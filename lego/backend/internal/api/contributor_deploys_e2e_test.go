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

// contributor_deploys_e2e_test.go is w9/m622's proof against REAL Postgres and
// REAL OpenFGA, through the composed REST, GraphQL and MCP surfaces. Render's
// role table gives "Trigger service deploys" and "Trigger rollbacks" to an
// Admin, a Developer and a Contributor and denies both to a Viewer and a
// Billing member (render.com/docs/team-members, read 2026-11-23). So in a
// Scale workspace a Contributor, and a key the Contributor minted with only
// Deploys "Run deployments", roll back to an earlier deploy and deploy a chosen
// commit, another tag of the configured image and a cleared build cache on
// every surface, and the deploy projection offers them Rollback. A different
// image is still refused by the same-image rule, and a Viewer and a Billing
// member are refused every one of these with nothing written.
//
//	BEX_TEST_DB_URI=postgres://…/postgres?sslmode=disable \
//	BEX_TEST_OPENFGA_URL=http://127.0.0.1:58085 \
//	  go test ./internal/api -run TestAContributorDeploysAndRollsBackE2E -v
func TestAContributorDeploysAndRollsBackE2E(t *testing.T) {
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
	checker := authz.NewOpenFGAChecker(fgaURL, os.Getenv("BEX_TEST_OPENFGA_TOKEN"))
	roles := checker.(store.MembershipGranter)
	admin := "admin-" + run
	// Contributor, Viewer and Billing are Scale roles (store.LimitsFor).
	ws, err := st.CreateWorkspace(ctx, "contribdeploys-"+run, store.PlanScale, admin)
	if err != nil {
		t.Fatal(err)
	}
	if err := roles.GrantWorkspaceRole(ctx, ws.ID, "user:"+admin, "admin"); err != nil {
		t.Fatal(err)
	}
	members := map[string]string{}
	for _, role := range []string{"contributor", "viewer", "billing"} {
		subject := role + "-" + run
		if err := st.AddMember(ctx, subject, ws.ID, role); err != nil {
			t.Fatal(err)
		}
		if err := roles.GrantWorkspaceRole(ctx, ws.ID, "user:"+subject, role); err != nil {
			t.Fatal(err)
		}
		members[role] = subject
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
		ProjectsStore: st, EnvironmentsStore: st, DeployStore: st, EventStore: st,
	})
	mux := srv.restHandler()
	gated := srv.withScopeClassREST(mux, mux)
	schema, err := srv.newSchema()
	if err != nil {
		t.Fatal(err)
	}
	srv.schema = schema

	session := func(subject string) core.Identity { return core.Identity{Subject: subject, Method: "session"} }
	rest := func(id core.Identity, method, path, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		gated.ServeHTTP(rec, req.WithContext(core.WithIdentity(ctx, id)))
		return rec
	}
	// create makes a service as the Admin and settles its first deploy live
	// on resolved, as the operator would.
	create := func(body, resolved string) (string, string) {
		t.Helper()
		rec := rest(session(admin), http.MethodPost, "/v1/services", body)
		var created struct {
			Service  struct{ ID string }
			DeployID string
		}
		if rec.Code != http.StatusCreated || json.Unmarshal(rec.Body.Bytes(), &created) != nil || created.Service.ID == "" || created.DeployID == "" {
			t.Fatalf("the Admin creates a service: %d %s", rec.Code, rec.Body)
		}
		if ok, err := st.CloseDeploy(ctx, created.DeployID, store.DeployLive, resolved); err != nil || !ok {
			t.Fatalf("settle %s live: %v %v", created.DeployID, ok, err)
		}
		return created.Service.ID, created.DeployID
	}
	digest := func(c string) string { return "docker.io/library/nginx@sha256:" + strings.Repeat(c, 64) }
	img, first := create(`{"type":"web_service","name":"img-`+run+`","image":{"imagePath":"docker.io/library/nginx:1.27"},"ownerId":"`+ws.ID+`","serviceDetails":{"plan":"starter"}}`, digest("a"))
	repo, _ := create(`{"type":"web_service","name":"repo-`+run+`","repo":"https://github.com/acme/app","branch":"main","ownerId":"`+ws.ID+`","serviceDetails":{"plan":"starter","runtime":"docker"}}`, "registry.local/repo-"+run+"@sha256:"+strings.Repeat("b", 64))
	// The Admin deploys another tag, which goes live: the first deploy is now
	// the previous one, the target Render's Rollback offers.
	rec := rest(session(admin), http.MethodPost, "/v1/services/"+img+"/deploys", `{"imageUrl":"docker.io/library/nginx:1.26"}`)
	var second struct{ ID string }
	if rec.Code != http.StatusCreated || json.Unmarshal(rec.Body.Bytes(), &second) != nil || second.ID == "" {
		t.Fatalf("the Admin deploys another tag: %d %s", rec.Code, rec.Body)
	}
	if ok, err := st.CloseDeploy(ctx, second.ID, store.DeployLive, digest("c")); err != nil || !ok {
		t.Fatalf("settle %s live: %v %v", second.ID, ok, err)
	}

	// The Contributor mints a key with Deploys "Run deployments" alone.
	rec = rest(session(members["contributor"]), http.MethodPost, "/v1/api-keys",
		`{"name":"deploy-key","ownerId":"`+ws.ID+`","permissions":{"grants":[{"permission":"deploys_run","level":"run"}]}}`)
	var key apikeys.APIKey
	if rec.Code != http.StatusCreated || json.Unmarshal(rec.Body.Bytes(), &key) != nil || key.Permissions.All {
		t.Fatalf("the Contributor mints a Run deployments key: %d %s", rec.Code, rec.Body)
	}
	contribKey := core.Identity{Subject: key.ID, ClientID: key.ID, Method: core.MethodOAuth2}

	// Every deploy and rollback, on each surface.
	const commit = "0123456789abcdef0123456789abcdef01234567"
	type call struct{ name, path, body string }
	restCalls := []call{
		{"rollback", "/v1/services/" + img + "/rollback", `{"deployId":"` + first + `"}`},
		{"deploy of another tag", "/v1/services/" + img + "/deploys", `{"imageUrl":"docker.io/library/nginx:1.25"}`},
		{"deploy of a chosen commit", "/v1/services/" + repo + "/deploys", `{"commitId":"` + commit + `"}`},
		{"deploy with the build cache cleared", "/v1/services/" + repo + "/deploys", `{"clearCache":"clear"}`},
		{"deploy of the latest commit", "/v1/services/" + repo + "/deploys", `{}`},
	}
	gqlCalls := []struct{ name, query string }{
		{"rollbackService", fmt.Sprintf(`mutation { rollbackService(serviceId: %q, deployId: %q, disableAutoDeploy: true) { id } }`, img, first)},
		{"triggerDeploy imageUrl", fmt.Sprintf(`mutation { triggerDeploy(serviceId: %q, imageUrl: "docker.io/library/nginx:1.24") { id } }`, img)},
		{"triggerDeploy commitId", fmt.Sprintf(`mutation { triggerDeploy(serviceId: %q, commitId: %q, disableAutoDeploy: true) { id } }`, repo, commit)},
		{"triggerDeploy clearCache", fmt.Sprintf(`mutation { triggerDeploy(serviceId: %q, clearCache: "clear") { id } }`, repo)},
	}
	mcpCalls := []struct {
		tool string
		args map[string]any
	}{
		{"rollback_deploy", map[string]any{"serviceId": img, "deployId": first}},
		{"trigger_deploy", map[string]any{"serviceId": img, "imageUrl": "docker.io/library/nginx:1.23"}},
		{"trigger_deploy", map[string]any{"serviceId": repo, "commitId": commit}},
		{"trigger_deploy", map[string]any{"serviceId": repo, "clearCache": true}},
	}

	// What the services and their deploy histories read as, to the Admin. A
	// refused call leaves every one of these as it was.
	readBack := func() map[string]string {
		t.Helper()
		out := map[string]string{}
		for _, path := range []string{
			"/v1/services/" + img, "/v1/services/" + repo,
			"/v1/services/" + img + "/deploys?limit=100", "/v1/services/" + repo + "/deploys?limit=100",
		} {
			rec := rest(session(admin), http.MethodGet, path, "")
			if rec.Code != http.StatusOK {
				t.Fatalf("the Admin reads %s: %d %s", path, rec.Code, rec.Body)
			}
			out[path] = rec.Body.String()
		}
		return out
	}
	projection := func(id core.Identity) map[string]string {
		t.Helper()
		out := graphqlDo(t, srv, id, fmt.Sprintf(`{ deployActions(serviceId: %q, ownerId: %q) { action outcome } }`, img, ws.ID))
		data, _ := out["data"].(map[string]any)
		rows, _ := data["deployActions"].([]any)
		got := map[string]string{}
		for _, r := range rows {
			row, _ := r.(map[string]any)
			action, _ := row["action"].(string)
			got[action], _ = row["outcome"].(string)
		}
		return got
	}

	before := readBack()
	for _, role := range []string{"viewer", "billing"} {
		id := session(members[role])
		for _, c := range restCalls {
			if rec := rest(id, http.MethodPost, c.path, c.body); rec.Code != http.StatusForbidden {
				t.Errorf("the %s's REST %s = %d %s, want 403", role, c.name, rec.Code, rec.Body)
			}
		}
		for _, m := range gqlCalls {
			if out := graphqlDo(t, srv, id, m.query); graphqlCode(out) != "FORBIDDEN" {
				t.Errorf("the %s's GraphQL %s = %v, want FORBIDDEN", role, m.name, out)
			}
		}
		for _, m := range mcpCalls {
			if res := mcpCall(t, srv, id, m.tool, m.args); !res.IsError || !strings.Contains(strings.ToLower(mcpText(res)), "forbidden") {
				t.Errorf("the %s's MCP %s %v = %s, want forbidden", role, m.tool, m.args, mcpText(res))
			}
		}
	}
	// The deploy projection stays a can_view read (ADR012 § Activity), and
	// tells both that neither deploy nor Rollback is theirs.
	for _, role := range []string{"viewer", "billing"} {
		if got := projection(session(members[role])); got["rollback"] != "denied" || got["deploy"] != "denied" {
			t.Errorf("the %s's deployActions = %v, want deploy and rollback denied", role, got)
		}
	}
	// A different image is refused to the Contributor by the same-image rule,
	// not by the role: only the tag or digest may change per deploy.
	for who, id := range map[string]core.Identity{"the Contributor": session(members["contributor"]), "the Contributor's key": contribKey} {
		if rec := rest(id, http.MethodPost, "/v1/services/"+img+"/deploys", `{"imageUrl":"ghcr.io/evil/x:latest"}`); rec.Code != http.StatusBadRequest ||
			!strings.Contains(rec.Body.String(), "configured image") {
			t.Errorf("%s's deploy of another image = %d %s, want 400 naming the configured image", who, rec.Code, rec.Body)
		}
	}
	after := readBack()
	for path, was := range before {
		if after[path] != was {
			t.Errorf("a refused deploy or rollback wrote %s:\n  before %s\n  after  %s", path, was, after[path])
		}
	}

	// The Contributor and their key: every deploy and rollback goes through.
	if got := projection(session(members["contributor"])); got["rollback"] != "allowed" || got["deploy"] != "allowed" {
		t.Errorf("the Contributor's deployActions = %v, want deploy and rollback allowed", got)
	}
	opened := 0
	for who, id := range map[string]core.Identity{"the Contributor": session(members["contributor"]), "the Contributor's key": contribKey} {
		for _, c := range restCalls {
			rec := rest(id, http.MethodPost, c.path, c.body)
			if rec.Code != http.StatusCreated && rec.Code != http.StatusAccepted {
				t.Errorf("%s's REST %s = %d %s, want 201 or 202", who, c.name, rec.Code, rec.Body)
				continue
			}
			opened++
		}
		for _, m := range gqlCalls {
			if out := graphqlDo(t, srv, id, m.query); out["errors"] != nil {
				t.Errorf("%s's GraphQL %s = %v", who, m.name, out)
				continue
			}
			opened++
		}
		for _, m := range mcpCalls {
			if res := mcpCall(t, srv, id, m.tool, m.args); res.IsError {
				t.Errorf("%s's MCP %s %v = %s", who, m.tool, m.args, mcpText(res))
				continue
			}
			opened++
		}
	}
	// Each accepted call opened its own deploy row: 3 deploys at setup.
	deploys, err := st.ListDeploys(ctx, img, store.DeployFilter{Limit: 200})
	if err != nil {
		t.Fatal(err)
	}
	repoDeploys, err := st.ListDeploys(ctx, repo, store.DeployFilter{Limit: 200})
	if err != nil {
		t.Fatal(err)
	}
	if got := len(deploys) + len(repoDeploys) - 3; got != opened {
		t.Errorf("the accepted calls opened %d deploys, want %d (one per call)", got, opened)
	}
	var rollbacks int
	for _, d := range deploys {
		if d.RollbackOf == first {
			rollbacks++
		}
	}
	if rollbacks != 6 {
		t.Errorf("%d rollback deploys name %s, want 6 (REST, GraphQL and MCP, for the Contributor and the key)", rollbacks, first)
	}
}
