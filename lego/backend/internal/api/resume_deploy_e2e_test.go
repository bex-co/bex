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

// TestAResumeOpensAServiceResumedDeployE2E is w3/m643 against REAL Postgres
// and REAL OpenFGA through the composed REST, GraphQL and MCP surfaces.
// Render's resume opens a deploy: its deploy trigger enum carries
// service_resumed, and its dashboard opens the deploy (a cron job's build) its
// resumeService mutation answers. So a resume on every surface opens exactly
// one deploy, of the release that was serving, which REST and MCP report as
// service_resumed and GraphQL resumeService answers as latestDeployId; a
// resume a Viewer is refused, and one while a deploy is in flight, opens none.
//
//	BEX_TEST_DB_URI=postgres://…/postgres?sslmode=disable \
//	BEX_TEST_OPENFGA_URL=http://127.0.0.1:58085 \
//	  go test ./internal/api -run TestAResumeOpensAServiceResumedDeployE2E -v
func TestAResumeOpensAServiceResumedDeployE2E(t *testing.T) {
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
	admin, viewer := "admin-"+run, "viewer-"+run
	ws, err := st.CreateWorkspace(ctx, "resumedeploy-"+run, store.PlanScale, admin)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.DeleteTenant(context.Background(), ws.ID) })
	if err := roles.GrantWorkspaceRole(ctx, ws.ID, "user:"+admin, "admin"); err != nil {
		t.Fatal(err)
	}
	if err := st.AddMember(ctx, viewer, ws.ID, "viewer"); err != nil {
		t.Fatal(err)
	}
	if err := roles.GrantWorkspaceRole(ctx, ws.ID, "user:"+viewer, "viewer"); err != nil {
		t.Fatal(err)
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

	asAdmin := core.Identity{Subject: admin, Method: "session"}
	asViewer := core.Identity{Subject: viewer, Method: "session"}
	rest := func(id core.Identity, method, path, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		gated.ServeHTTP(rec, req.WithContext(core.WithIdentity(ctx, id)))
		return rec
	}
	settle := func(deployID, image string) {
		t.Helper()
		if ok, err := st.CloseDeploy(ctx, deployID, store.DeployLive, image); err != nil || !ok {
			t.Fatalf("settle %s live: %v %v", deployID, ok, err)
		}
	}
	create := func(body, image string) string {
		t.Helper()
		rec := rest(asAdmin, http.MethodPost, "/v1/services", body)
		var created struct {
			Service  struct{ ID string }
			DeployID string
		}
		if rec.Code != http.StatusCreated || json.Unmarshal(rec.Body.Bytes(), &created) != nil || created.Service.ID == "" || created.DeployID == "" {
			t.Fatalf("create a service: %d %s", rec.Code, rec.Body)
		}
		settle(created.DeployID, image)
		return created.Service.ID
	}
	deploys := func(service string) []store.Deploy {
		t.Helper()
		rows, err := st.ListDeploys(ctx, service, store.DeployFilter{Limit: 100})
		if err != nil {
			t.Fatal(err)
		}
		return rows
	}
	suspend := func(service string) {
		t.Helper()
		if rec := rest(asAdmin, http.MethodPost, "/v1/services/"+service+"/suspend", ""); rec.Code != http.StatusAccepted {
			t.Fatalf("suspend %s: %d %s", service, rec.Code, rec.Body)
		}
	}
	// opened is the one deploy a resume opened since before, of the serving
	// image, with Render's trigger.
	opened := func(service string, before int, image string) store.Deploy {
		t.Helper()
		rows := deploys(service)
		if len(rows) != before+1 {
			t.Fatalf("%s has %d deploys after the resume, want %d (one opened)", service, len(rows), before+1)
		}
		d := rows[0]
		if d.Trigger != store.TriggerServiceResumed || d.Image != image || d.TriggeredBy != admin {
			t.Fatalf("the resume's deploy = trigger %q image %q by %q, want service_resumed of %q by the Admin", d.Trigger, d.Image, d.TriggeredBy, image)
		}
		return d
	}

	webImage := "docker.io/library/nginx@sha256:" + strings.Repeat("a", 64)
	cronImage := "docker.io/library/busybox@sha256:" + strings.Repeat("b", 64)
	web := create(`{"type":"web_service","name":"web-`+run+`","image":{"imagePath":"docker.io/library/nginx:1.27"},"ownerId":"`+ws.ID+`","serviceDetails":{"plan":"starter"}}`, webImage)
	cron := create(`{"type":"cron_job","name":"cron-`+run+`","image":{"imagePath":"docker.io/library/busybox:1.36"},"ownerId":"`+ws.ID+`","serviceDetails":{"plan":"starter","schedule":"0 9 * * 1-5"}}`, cronImage)

	// REST: one deploy each, a cron job's too, read back as Render's trigger.
	for service, image := range map[string]string{web: webImage, cron: cronImage} {
		suspend(service)
		before := len(deploys(service))
		if rec := rest(asAdmin, http.MethodPost, "/v1/services/"+service+"/resume", ""); rec.Code != http.StatusAccepted {
			t.Fatalf("REST resume %s: %d %s", service, rec.Code, rec.Body)
		}
		d := opened(service, before, image)
		rec := rest(asAdmin, http.MethodGet, "/v1/services/"+service+"/deploys?limit=1", "")
		var listed []struct {
			Deploy struct{ ID, Trigger string }
		}
		if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &listed) != nil || len(listed) != 1 ||
			listed[0].Deploy.ID != d.ID || listed[0].Deploy.Trigger != "service_resumed" {
			t.Errorf("REST list deploys of %s = %d %s, want %s with trigger service_resumed", service, rec.Code, rec.Body, d.ID)
		}
		res := mcpCall(t, srv, asAdmin, "list_deploys", map[string]any{"serviceId": service, "limit": 1})
		if res.IsError || !strings.Contains(mcpText(res), d.ID) || !strings.Contains(mcpText(res), `"trigger":"service_resumed"`) {
			t.Errorf("MCP list_deploys of %s = %s, want %s with trigger service_resumed", service, mcpText(res), d.ID)
		}
		settle(d.ID, image)
	}

	// GraphQL resumeService answers the deploy it opened, which deploys() reads.
	suspend(web)
	before := len(deploys(web))
	out := graphqlDo(t, srv, asAdmin, fmt.Sprintf(`mutation { resumeService(id: %q) { suspended latestDeployId } }`, web))
	if out["errors"] != nil {
		t.Fatalf("GraphQL resumeService: %v", out)
	}
	answered, _ := out["data"].(map[string]any)["resumeService"].(map[string]any)
	d := opened(web, before, webImage)
	if answered["latestDeployId"] != d.ID || answered["suspended"] != "not_suspended" {
		t.Errorf("resumeService = %v, want latestDeployId %s", answered, d.ID)
	}
	out = graphqlDo(t, srv, asAdmin, fmt.Sprintf(`{ deploys(serviceId: %q, limit: 1) { id trigger } }`, web))
	if rows, _ := out["data"].(map[string]any)["deploys"].([]any); len(rows) != 1 ||
		rows[0].(map[string]any)["id"] != d.ID || rows[0].(map[string]any)["trigger"] != "service_resumed" {
		t.Errorf("GraphQL deploys = %v, want %s with trigger service_resumed", out, d.ID)
	}
	settle(d.ID, webImage)

	// MCP resume_service opens one too.
	suspend(web)
	before = len(deploys(web))
	if res := mcpCall(t, srv, asAdmin, "resume_service", map[string]any{"serviceId": web}); res.IsError {
		t.Fatalf("MCP resume_service: %s", mcpText(res))
	}
	settle(opened(web, before, webImage).ID, webImage)

	// A Viewer's resume is refused and opens nothing.
	suspend(web)
	before = len(deploys(web))
	if rec := rest(asViewer, http.MethodPost, "/v1/services/"+web+"/resume", ""); rec.Code != http.StatusForbidden {
		t.Errorf("the Viewer's resume = %d %s, want 403", rec.Code, rec.Body)
	}
	if n := len(deploys(web)); n != before {
		t.Errorf("a refused resume opened %d deploys", n-before)
	}

	// A resume while a deploy is in flight resumes without opening another:
	// the deploy in flight brings the service back, and a new release would
	// replace it.
	inFlight, err := st.CreateDeploy(ctx, web, store.TriggerAPI, webImage, 99, store.CommitInfo{}, admin)
	if err != nil {
		t.Fatal(err)
	}
	before = len(deploys(web))
	rec := rest(asAdmin, http.MethodPost, "/v1/services/"+web+"/resume", "")
	if rec.Code != http.StatusAccepted || !strings.Contains(rec.Body.String(), `"suspended":"not_suspended"`) {
		t.Fatalf("resume with a deploy in flight = %d %s, want 202 and resumed", rec.Code, rec.Body)
	}
	if rows := deploys(web); len(rows) != before || rows[0].ID != inFlight.ID {
		t.Errorf("a resume with %s in flight opened another deploy: %d rows, newest %s", inFlight.ID, len(rows), rows[0].ID)
	}
}
