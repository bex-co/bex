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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/bex-co/bex/lego/backend/internal/apikeys"
	"github.com/bex-co/bex/lego/backend/internal/authz"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
	"github.com/bex-co/bex/lego/backend/internal/tenancy"
	"github.com/bex-co/bex/lego/backend/internal/testenv"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// activity_per_role_e2e_test.go is w9/m621's proof against REAL Postgres and
// REAL OpenFGA, through the composed REST, GraphQL and MCP surfaces. Render's
// role table (render.com/docs/team-members, read 2026-11-23) denies the
// Billing role "View service events", "View service metrics" and "View
// service logs", and grants it only "non-sensitive details of the
// workspace's resources (such as members and service names)". So in a Scale
// workspace a Billing member, and an API key it minted, still list and read
// the service by name, but are refused its events, its deploy history and
// every service, datastore and project metric and a cron job's run history
// (Render's cron_job_run events) with the same 403; an Admin, a
// Developer, a Contributor and a Viewer, and their keys, pass the role gate
// on every one of them; and a Viewer is still refused the logs.
//
//	BEX_TEST_DB_URI=postgres://…/postgres?sslmode=disable \
//	BEX_TEST_OPENFGA_URL=http://127.0.0.1:58085 \
//	  go test ./internal/api -run TestActivityPerRoleE2E -v
func TestActivityPerRoleE2E(t *testing.T) {
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
	ws, err := st.CreateWorkspace(ctx, "activity-"+run, store.PlanScale, admin)
	if err != nil {
		t.Fatal(err)
	}
	if err := roles.GrantWorkspaceRole(ctx, ws.ID, "user:"+admin, "admin"); err != nil {
		t.Fatal(err)
	}
	members := map[string]string{"admin": admin}
	for _, role := range []string{"developer", "contributor", "viewer", "billing"} {
		subject := role + "-" + run
		if err := st.AddMember(ctx, subject, ws.ID, role); err != nil {
			t.Fatal(err)
		}
		if err := roles.GrantWorkspaceRole(ctx, ws.ID, "user:"+subject, role); err != nil {
			t.Fatal(err)
		}
		members[role] = subject
	}

	dbID := "dpg-" + strings.ToLower(run)
	kube := fakeClient(&appv1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{Name: dbID, Namespace: "default", Labels: map[string]string{core.LabelTenant: ws.ID, core.LabelWorkspace: ws.ID}},
		Spec:       appv1alpha1.DatabaseSpec{Name: "orders", Plan: "basic-256mb", Version: "17"},
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

	// The Admin creates a web service (its first deploy is recorded with it)
	// and a project; the service's events then hold that deploy's start.
	name := "web-" + run
	rec := rest(session(admin), http.MethodPost, "/v1/services", `{"type":"web_service","name":"`+name+`","image":{"imagePath":"nginx"},"ownerId":"`+ws.ID+`","serviceDetails":{"plan":"starter"}}`)
	var created struct {
		Service  struct{ ID string }
		DeployID string
	}
	if rec.Code != http.StatusCreated || json.Unmarshal(rec.Body.Bytes(), &created) != nil || created.Service.ID == "" || created.DeployID == "" {
		t.Fatalf("the Admin creates a service: %d %s", rec.Code, rec.Body)
	}
	web, deployID := created.Service.ID, created.DeployID
	rec = rest(session(admin), http.MethodPost, "/v1/services", `{"type":"cron_job","name":"cron-`+run+`","image":{"imagePath":"nginx"},"ownerId":"`+ws.ID+`","serviceDetails":{"runtime":"image","schedule":"0 * * * *"}}`)
	var cron struct{ Service struct{ ID string } }
	if rec.Code != http.StatusCreated || json.Unmarshal(rec.Body.Bytes(), &cron) != nil || cron.Service.ID == "" {
		t.Fatalf("the Admin creates a cron job: %d %s", rec.Code, rec.Body)
	}
	rec = rest(session(admin), http.MethodPost, "/v1/projects", `{"name":"p-`+run+`","ownerId":"`+ws.ID+`","environments":[{"name":"production"}]}`)
	var project struct{ ID string }
	if rec.Code != http.StatusCreated || json.Unmarshal(rec.Body.Bytes(), &project) != nil || project.ID == "" {
		t.Fatalf("the Admin creates a project: %d %s", rec.Code, rec.Body)
	}

	// Every member mints an All key (any role may, w9/m514); each acts with
	// its creator's role (w9/m481).
	callers := map[string]core.Identity{}
	for role, subject := range members {
		callers[role] = session(subject)
		rec := rest(session(subject), http.MethodPost, "/v1/api-keys", `{"name":"`+role+`-key","ownerId":"`+ws.ID+`"}`)
		var key apikeys.APIKey
		if rec.Code != http.StatusCreated || json.Unmarshal(rec.Body.Bytes(), &key) != nil || !key.Permissions.All {
			t.Fatalf("the %s mints an All key: %d %s", role, rec.Code, rec.Body)
		}
		callers[role+"'s key"] = core.Identity{Subject: key.ID, ClientID: key.ID, Method: core.MethodOAuth2}
	}

	for who, id := range callers {
		role, _, _ := strings.Cut(who, "'")
		activity := role != "billing"
		logs := role != "billing" && role != "viewer"

		// The service itself, by name, reads for every role.
		if rec := rest(id, http.MethodGet, "/v1/services?ownerId="+ws.ID, ""); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), name) {
			t.Errorf("%s's GET /v1/services = %d %s, want 200 with the service's name", who, rec.Code, rec.Body)
		}
		if rec := rest(id, http.MethodGet, "/v1/services/"+web, ""); rec.Code != http.StatusOK {
			t.Errorf("%s's GET /v1/services/{id} = %d %s, want 200", who, rec.Code, rec.Body)
		}
		if out := graphqlDo(t, srv, id, fmt.Sprintf(`{ service(id: %q) { name } }`, web)); out["errors"] != nil || !strings.Contains(fmt.Sprint(out["data"]), name) {
			t.Errorf("%s's GraphQL service = %v, want its name", who, out)
		}

		// Events, deploy history and metrics, on each surface.
		restReads := map[string]*httptest.ResponseRecorder{
			"GET events":                   rest(id, http.MethodGet, "/v1/services/"+web+"/events", ""),
			"GET deploys":                  rest(id, http.MethodGet, "/v1/services/"+web+"/deploys", ""),
			"GET deploy":                   rest(id, http.MethodGet, "/v1/services/"+web+"/deploys/"+deployID, ""),
			"GET /v1/metrics/cpu":          rest(id, http.MethodGet, "/v1/metrics/cpu?resource="+web, ""),
			"GET /v1/metrics/cpu project":  rest(id, http.MethodGet, "/v1/metrics/cpu?project="+project.ID, ""),
			"GET /v1/metrics/disk-usage":   rest(id, http.MethodGet, "/v1/metrics/disk-usage?resource="+dbID, ""),
			"GET /v1/metrics/filters/http": rest(id, http.MethodGet, "/v1/metrics/filters/http?resource="+web, ""),
			"GET cron runs":                rest(id, http.MethodGet, "/v1/cron-jobs/"+cron.Service.ID+"/runs", ""),
		}
		gqlReads := map[string]map[string]any{
			"serviceEvents":    graphqlDo(t, srv, id, fmt.Sprintf(`{ serviceEvents(serviceId: %q) { id } }`, web)),
			"deploys":          graphqlDo(t, srv, id, fmt.Sprintf(`{ deploys(serviceId: %q) { id } }`, web)),
			"deploy":           graphqlDo(t, srv, id, fmt.Sprintf(`{ deploy(serviceId: %q, deployId: %q) { id } }`, web, deployID)),
			"metrics":          graphqlDo(t, srv, id, fmt.Sprintf(`{ metrics(query: {filters: [{field: "RESOURCE", values: [%q]}], name: "CPU"}) { unit } }`, web)),
			"datastoreMetrics": graphqlDo(t, srv, id, fmt.Sprintf(`{ datastoreMetrics(query: {kind: "database", resource: %q, name: "DB_CONNECTIONS"}) { unit } }`, dbID)),
			"projectMetrics":   graphqlDo(t, srv, id, fmt.Sprintf(`{ projectMetrics(query: {project: %q, name: "CPU"}) { series { unit } } }`, project.ID)),
			"metricsRetention": graphqlDo(t, srv, id, fmt.Sprintf(`{ metricsRetention(resource: %q) { days } }`, web)),
		}
		mcpReads := map[string]string{
			"list_events":        mcpText(mcpCall(t, srv, id, "list_events", map[string]any{"serviceId": web})),
			"list_deploys":       mcpText(mcpCall(t, srv, id, "list_deploys", map[string]any{"serviceId": web})),
			"get_deploy":         mcpText(mcpCall(t, srv, id, "get_deploy", map[string]any{"serviceId": web, "deployId": deployID})),
			"get_metrics":        mcpText(mcpCall(t, srv, id, "get_metrics", map[string]any{"resourceId": web, "metricTypes": []string{"cpu_usage"}})),
			"list_cron_job_runs": mcpText(mcpCall(t, srv, id, "list_cron_job_runs", map[string]any{"serviceId": cron.Service.ID})),
		}
		for read, rec := range restReads {
			if refused := rec.Code == http.StatusForbidden; refused == activity {
				t.Errorf("%s's %s = %d %s, want refused=%v", who, read, rec.Code, rec.Body, !activity)
			}
		}
		for read, out := range gqlReads {
			if refused := graphqlCode(out) == "FORBIDDEN"; refused == activity {
				t.Errorf("%s's GraphQL %s = %v, want refused=%v", who, read, out, !activity)
			}
		}
		for read, text := range mcpReads {
			if refused := strings.Contains(strings.ToLower(text), "forbidden"); refused == activity {
				t.Errorf("%s's MCP %s = %s, want refused=%v", who, read, text, !activity)
			}
		}
		// What an allowed role reads is the history itself.
		if activity {
			if rec := restReads["GET deploys"]; rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), deployID) {
				t.Errorf("%s's GET deploys = %d %s, want 200 with the first deploy", who, rec.Code, rec.Body)
			}
		} else if strings.Contains(restReads["GET deploys"].Body.String(), deployID) || strings.Contains(mcpReads["list_deploys"], deployID) {
			t.Errorf("%s's refused deploy reads named the deploy", who)
		}

		// Logs stay Contributor and up: a Viewer is still refused them.
		if rec := rest(id, http.MethodGet, "/v1/logs?ownerId="+ws.ID+"&resource="+web, ""); (rec.Code == http.StatusForbidden) == logs {
			t.Errorf("%s's GET /v1/logs = %d %s, want refused=%v", who, rec.Code, rec.Body, !logs)
		}

		// The capability read says the same, so the dashboard gates on it.
		caps := graphqlDo(t, srv, id, fmt.Sprintf(`{ viewerCapabilities(ownerId: %q) { canView canViewActivity canViewLogs } }`, ws.ID))
		got, _ := caps["data"].(map[string]any)["viewerCapabilities"].(map[string]any)
		if got["canView"] != true || got["canViewActivity"] != activity || got["canViewLogs"] != logs {
			t.Errorf("%s's viewerCapabilities = %v, want canView true, canViewActivity %v, canViewLogs %v", who, caps, activity, logs)
		}
	}
}
