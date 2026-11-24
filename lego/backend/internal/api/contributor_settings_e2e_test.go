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

// contributor_settings_e2e_test.go is w9/m619's proof against REAL Postgres
// and REAL OpenFGA, through the composed REST, GraphQL and MCP surfaces.
// Render's role table gives a Contributor no "Modify service configuration"
// and no "Manage inbound IP rules" (render.com/docs/team-members, read
// 2026-11-23). So in a Scale workspace a Contributor, and an API key the
// Contributor minted, are refused every service and datastore setting with
// the same 403, and nothing is written (each resource reads back unchanged);
// the Contributor still restarts, suspends and resumes, runs and cancels a
// cron job, and purges a cache; and a Developer's setting changes pass the
// role gate.
//
//	BEX_TEST_DB_URI=postgres://…/postgres?sslmode=disable \
//	BEX_TEST_OPENFGA_URL=http://127.0.0.1:58085 \
//	  go test ./internal/api -run TestAContributorChangesNoSettingE2E -v
func TestAContributorChangesNoSettingE2E(t *testing.T) {
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
	// The Contributor is a Scale role (store.LimitsFor).
	ws, err := st.CreateWorkspace(ctx, "contribsettings-"+run, store.PlanScale, admin)
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

	labels := map[string]string{core.LabelTenant: ws.ID, core.LabelWorkspace: ws.ID}
	dbID, kvID := "dpg-"+strings.ToLower(run), "red-"+strings.ToLower(run)
	kube := fakeClient(
		&appv1alpha1.Database{
			ObjectMeta: metav1.ObjectMeta{Name: dbID, Namespace: "default", Labels: labels},
			Spec:       appv1alpha1.DatabaseSpec{Name: "orders", Plan: "basic-256mb", Version: "17"},
			Status:     appv1alpha1.DatabaseStatus{Phase: appv1alpha1.DBPhaseReady},
		},
		&appv1alpha1.KeyValue{
			ObjectMeta: metav1.ObjectMeta{Name: kvID, Namespace: "default", Labels: labels},
			Spec:       appv1alpha1.KeyValueSpec{Name: "cache", Plan: "starter"},
		},
	)
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
	rest := func(id core.Identity, method, path, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		gated.ServeHTTP(rec, req.WithContext(core.WithIdentity(ctx, id)))
		return rec
	}
	create := func(body string) string {
		t.Helper()
		rec := rest(session(admin), http.MethodPost, "/v1/services", body)
		var created struct{ Service struct{ ID string } }
		if rec.Code != http.StatusCreated || json.Unmarshal(rec.Body.Bytes(), &created) != nil || created.Service.ID == "" {
			t.Fatalf("the Admin creates a service: %d %s", rec.Code, rec.Body)
		}
		return created.Service.ID
	}
	web := create(`{"type":"web_service","name":"web-` + run + `","image":{"imagePath":"nginx"},"ownerId":"` + ws.ID + `","serviceDetails":{"plan":"starter"}}`)
	cron := create(`{"type":"cron_job","name":"cron-` + run + `","image":{"imagePath":"nginx"},"ownerId":"` + ws.ID + `","serviceDetails":{"runtime":"image","schedule":"0 * * * *"}}`)

	rec := rest(session(contrib), http.MethodPost, "/v1/api-keys", `{"name":"contrib-key","ownerId":"`+ws.ID+`"}`)
	var key apikeys.APIKey
	if rec.Code != http.StatusCreated || json.Unmarshal(rec.Body.Bytes(), &key) != nil || !key.Permissions.All {
		t.Fatalf("the Contributor mints an All key: %d %s", rec.Code, rec.Body)
	}
	contribKey := core.Identity{Subject: key.ID, ClientID: key.ID, Method: core.MethodOAuth2}

	// What each resource reads as, to the Admin. A refused write leaves every
	// one of these byte for byte as it was.
	readBack := func() map[string]string {
		t.Helper()
		out := map[string]string{}
		for _, path := range []string{
			"/v1/services/" + web, "/v1/services/" + cron,
			"/v1/services/" + web + "/custom-domains", "/v1/services/" + web + "/autoscaling",
			"/v1/notification-settings/overrides/services/" + web,
			"/v1/postgres/" + dbID, "/v1/key-value/" + kvID,
		} {
			rec := rest(session(admin), http.MethodGet, path, "")
			if rec.Code != http.StatusOK {
				t.Fatalf("the Admin reads %s: %d %s", path, rec.Code, rec.Body)
			}
			out[path] = rec.Body.String()
		}
		return out
	}

	type call struct{ name, method, path, body string }
	settings := []call{
		{"rename", http.MethodPatch, "/v1/services/" + web, `{"name":"renamed"}`},
		{"plan", http.MethodPatch, "/v1/services/" + web, `{"serviceDetails":{"plan":"standard"}}`},
		{"health check path", http.MethodPatch, "/v1/services/" + web, `{"serviceDetails":{"healthCheckPath":"/livez"}}`},
		{"max shutdown delay", http.MethodPatch, "/v1/services/" + web, `{"serviceDetails":{"maxShutdownDelaySeconds":60}}`},
		{"build filter", http.MethodPatch, "/v1/services/" + web, `{"buildFilter":{"paths":["src/**"],"ignoredPaths":[]}}`},
		{"auto-deploy", http.MethodPatch, "/v1/services/" + web, `{"autoDeploy":"no"}`},
		{"maintenance mode", http.MethodPatch, "/v1/services/" + web, `{"serviceDetails":{"maintenanceMode":{"enabled":true,"uri":""}}}`},
		{"edge cache profile", http.MethodPatch, "/v1/services/" + web, `{"serviceDetails":{"cache":{"profile":"aggressive"}}}`},
		{"subdomain policy", http.MethodPatch, "/v1/services/" + web, `{"renderSubdomainPolicy":"disabled"}`},
		{"IP rules", http.MethodPatch, "/v1/services/" + web, `{"serviceDetails":{"ipAllowList":[{"cidrBlock":"10.0.0.0/8","description":"office"}]}}`},
		{"notify on fail", http.MethodPatch, "/v1/services/" + web, `{"notifyOnFail":"ignore"}`},
		{"cron schedule", http.MethodPatch, "/v1/services/" + cron, `{"serviceDetails":{"schedule":"5 * * * *"}}`},
		{"instance count", http.MethodPost, "/v1/services/" + web + "/scale", `{"numInstances":2}`},
		{"autoscaling", http.MethodPut, "/v1/services/" + web + "/autoscaling", `{"enabled":true,"min":1,"max":3,"criteria":{"cpu":{"enabled":true,"percentage":70},"memory":{"enabled":false,"percentage":0}}}`},
		{"autoscaling off", http.MethodDelete, "/v1/services/" + web + "/autoscaling", ""},
		{"custom domain add", http.MethodPost, "/v1/services/" + web + "/custom-domains", `{"name":"www.example.com"}`},
		{"custom domain verify", http.MethodPost, "/v1/services/" + web + "/custom-domains/www.example.com/verify", ""},
		{"custom domain delete", http.MethodDelete, "/v1/services/" + web + "/custom-domains/www.example.com", ""},
		{"notification override", http.MethodPatch, "/v1/notification-settings/overrides/services/" + web, `{"notificationsToSend":"failure"}`},
		{"Postgres rename", http.MethodPatch, "/v1/postgres/" + dbID, `{"name":"renamed"}`},
		{"Postgres plan", http.MethodPatch, "/v1/postgres/" + dbID, `{"plan":"basic_1gb"}`},
		{"Postgres version", http.MethodPatch, "/v1/postgres/" + dbID, `{"version":"18"}`},
		{"Postgres IP rules", http.MethodPatch, "/v1/postgres/" + dbID, `{"ipAllowList":[]}`},
		{"Key Value rename", http.MethodPatch, "/v1/key-value/" + kvID, `{"name":"renamed"}`},
		{"Key Value plan", http.MethodPatch, "/v1/key-value/" + kvID, `{"plan":"standard"}`},
		{"Key Value IP rules", http.MethodPatch, "/v1/key-value/" + kvID, `{"ipAllowList":[]}`},
	}
	gql := []struct{ name, query string }{
		{"setDisplayName", fmt.Sprintf(`mutation { setDisplayName(id: %q, displayName: "renamed") { id } }`, web)},
		{"updateServicePlan", fmt.Sprintf(`mutation { updateServicePlan(id: %q, plan: "standard") { id } }`, web)},
		{"scaleService", fmt.Sprintf(`mutation { scaleService(id: %q, numInstances: 2) { id } }`, web)},
		{"setAutoscaling", fmt.Sprintf(`mutation { setAutoscaling(id: %q, minInstances: 1, maxInstances: 3, targetCPUPercent: 70) { enabled } }`, web)},
		{"disableAutoscaling", fmt.Sprintf(`mutation { disableAutoscaling(id: %q) }`, web)},
		{"setHealthCheckPath", fmt.Sprintf(`mutation { setHealthCheckPath(id: %q, path: "/livez") { id } }`, web)},
		{"setIdleTimeout", fmt.Sprintf(`mutation { setIdleTimeout(id: %q, idleTTLSeconds: 600) { id } }`, web)},
		{"setMaxShutdownDelay", fmt.Sprintf(`mutation { setMaxShutdownDelay(id: %q, seconds: 60) { id } }`, web)},
		{"setAutoDeploy", fmt.Sprintf(`mutation { setAutoDeploy(id: %q, enabled: false) { id } }`, web)},
		{"setMaintenanceMode", fmt.Sprintf(`mutation { setMaintenanceMode(id: %q, maintenanceMode: {enabled: true}) { id } }`, web)},
		{"setCacheProfile", fmt.Sprintf(`mutation { setCacheProfile(id: %q, profile: "aggressive") { id } }`, web)},
		{"setSubdomainPolicy", fmt.Sprintf(`mutation { setSubdomainPolicy(id: %q, policy: "disabled") { id } }`, web)},
		{"setServiceIpAllowList", fmt.Sprintf(`mutation { setServiceIpAllowList(id: %q, cidrs: ["10.0.0.0/8"]) { id } }`, web)},
		{"setNotificationsToSend", fmt.Sprintf(`mutation { setNotificationsToSend(id: %q, value: "failure") { id } }`, web)},
		{"addCustomDomain", fmt.Sprintf(`mutation { addCustomDomain(id: %q, name: "www.example.com") { name } }`, web)},
		{"updateCronJob", fmt.Sprintf(`mutation { updateCronJob(id: %q, schedule: "5 * * * *") { id } }`, cron)},
		{"renameDatabase", fmt.Sprintf(`mutation { renameDatabase(id: %q, name: "renamed") { id } }`, dbID)},
		{"updateDatabasePlan", fmt.Sprintf(`mutation { updateDatabasePlan(id: %q, plan: "basic_1gb") { id } }`, dbID)},
		{"updateDatabaseVersion", fmt.Sprintf(`mutation { updateDatabaseVersion(id: %q, version: "18") { id } }`, dbID)},
		{"updateDatabaseHighAvailability", fmt.Sprintf(`mutation { updateDatabaseHighAvailability(id: %q, enabled: true) { id } }`, dbID)},
		{"renameKeyValue", fmt.Sprintf(`mutation { renameKeyValue(id: %q, name: "renamed") { id } }`, kvID)},
		{"updateKeyValuePlan", fmt.Sprintf(`mutation { updateKeyValuePlan(id: %q, plan: "standard") { id } }`, kvID)},
		{"setKeyValueIpAllowList", fmt.Sprintf(`mutation { setKeyValueIpAllowList(id: %q, cidrs: []) { id } }`, kvID)},
	}
	mcpTools := []struct {
		tool string
		args map[string]any
	}{
		{"update_service", map[string]any{"serviceId": web, "displayName": "renamed"}},
		{"update_service", map[string]any{"serviceId": web, "plan": "standard"}},
		{"update_service", map[string]any{"serviceId": web, "healthCheckPath": "/livez"}},
		{"update_service", map[string]any{"serviceId": cron, "schedule": "5 * * * *"}},
		{"scale_service", map[string]any{"serviceId": web, "numInstances": 2}},
		{"disable_autoscaling", map[string]any{"serviceId": web}},
		{"add_custom_domain", map[string]any{"serviceId": web, "name": "www.example.com"}},
		{"verify_custom_domain", map[string]any{"serviceId": web, "name": "www.example.com"}},
		{"delete_custom_domain", map[string]any{"serviceId": web, "name": "www.example.com"}},
		{"update_postgres", map[string]any{"postgresId": dbID, "name": "renamed"}},
		{"update_key_value", map[string]any{"keyValueId": kvID, "name": "renamed"}},
	}

	before := readBack()
	for who, id := range map[string]core.Identity{"the Contributor": session(contrib), "the Contributor's key": contribKey} {
		for _, c := range settings {
			if rec := rest(id, c.method, c.path, c.body); rec.Code != http.StatusForbidden {
				t.Errorf("%s's REST %s = %d %s, want 403", who, c.name, rec.Code, rec.Body)
			}
		}
		for _, m := range gql {
			if out := graphqlDo(t, srv, id, m.query); graphqlCode(out) != "FORBIDDEN" {
				t.Errorf("%s's GraphQL %s = %v, want FORBIDDEN", who, m.name, out)
			}
		}
		for _, m := range mcpTools {
			if res := mcpCall(t, srv, id, m.tool, m.args); !res.IsError || !strings.Contains(strings.ToLower(mcpText(res)), "forbidden") {
				t.Errorf("%s's MCP %s %v = %s, want forbidden", who, m.tool, m.args, mcpText(res))
			}
		}
	}
	after := readBack()
	for path, was := range before {
		if after[path] != was {
			t.Errorf("a refused setting wrote %s:\n  before %s\n  after  %s", path, was, after[path])
		}
	}

	// Runtime actions stay a Contributor's, as on Render: none is refused for
	// the role (a verb may still answer on state, such as a cache that is off).
	runtime := []call{
		{"restart", http.MethodPost, "/v1/services/" + web + "/restart", ""},
		{"suspend", http.MethodPost, "/v1/services/" + web + "/suspend", ""},
		{"resume", http.MethodPost, "/v1/services/" + web + "/resume", ""},
		{"deploy", http.MethodPost, "/v1/services/" + web + "/deploys", `{}`},
		{"cache purge", http.MethodPost, "/v1/services/" + web + "/cache/purge", ""},
		{"cron run", http.MethodPost, "/v1/services/" + cron + "/runs", ""},
		{"Postgres restart", http.MethodPost, "/v1/postgres/" + dbID + "/restart", ""},
		{"Postgres suspend", http.MethodPost, "/v1/postgres/" + dbID + "/suspend", ""},
		{"Postgres resume", http.MethodPost, "/v1/postgres/" + dbID + "/resume", ""},
		{"Key Value suspend", http.MethodPost, "/v1/key-value/" + kvID + "/suspend", ""},
		{"Key Value resume", http.MethodPost, "/v1/key-value/" + kvID + "/resume", ""},
	}
	for who, id := range map[string]core.Identity{"the Contributor": session(contrib), "the Contributor's key": contribKey} {
		for _, c := range runtime {
			if rec := rest(id, c.method, c.path, c.body); rec.Code == http.StatusForbidden {
				t.Errorf("%s's %s = 403 %s, want it allowed", who, c.name, rec.Body)
			}
		}
	}
	if res := mcpCall(t, srv, contribKey, "restart_service", map[string]any{"serviceId": web}); res.IsError {
		t.Errorf("the Contributor's key MCP restart_service = %s", mcpText(res))
	}
	if out := graphqlDo(t, srv, session(contrib), fmt.Sprintf(`mutation { purgeServiceCache(id: %q) { id } }`, web)); graphqlCode(out) == "FORBIDDEN" {
		t.Errorf("the Contributor's GraphQL purgeServiceCache = %v", out)
	}

	// A Developer passes the role gate on every one of them; the simplest
	// land, and read back.
	for _, c := range settings {
		if rec := rest(session(dev), c.method, c.path, c.body); rec.Code == http.StatusForbidden {
			t.Errorf("the Developer's REST %s = 403 %s", c.name, rec.Body)
		}
	}
	if rec := rest(session(dev), http.MethodPatch, "/v1/services/"+web, `{"serviceDetails":{"healthCheckPath":"/healthz"}}`); rec.Code != http.StatusOK ||
		!strings.Contains(rest(session(admin), http.MethodGet, "/v1/services/"+web, "").Body.String(), `"/healthz"`) {
		t.Errorf("the Developer's health check path change: %d %s", rec.Code, rec.Body)
	}
	if out := graphqlDo(t, srv, session(dev), fmt.Sprintf(`mutation { updateCronJob(id: %q, schedule: "15 * * * *") { id } }`, cron)); out["errors"] != nil {
		t.Errorf("the Developer's GraphQL updateCronJob = %v", out)
	}
	if res := mcpCall(t, srv, session(dev), "update_key_value", map[string]any{"keyValueId": kvID, "name": "sessions"}); res.IsError {
		t.Errorf("the Developer's MCP update_key_value = %s", mcpText(res))
	}
}
