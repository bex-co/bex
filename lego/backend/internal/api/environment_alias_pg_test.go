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
	"regexp"
	"slices"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/apps"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/environments"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// Real SQL rows/FKs and the composed transports share the same legacy identity.
// Kubernetes and Hydra are test doubles; the live CLI walk is recorded separately.
func TestEnvironmentAliasesPreserveDurableMembershipPG(t *testing.T) {
	ctx, pool, st := ownerGuardPG(t)
	owner, err := st.CreateWorkspace(ctx, "alias-"+ids.New(ids.Workspace), store.PlanHobby, "client-1")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.DeleteTenant(context.Background(), owner.ID) })
	project, err := st.CreateProject(ctx, owner.ID, "legacy")
	if err != nil {
		t.Fatal(err)
	}
	canonical := ids.New(ids.Environment)
	legacy := ids.EnvironmentStorageID(canonical)
	// Seed the shape persisted before aliases existed, including its controls.
	if _, err := pool.Exec(ctx, `INSERT INTO environments(id,project_id,tenant_id,name,protected_status,network_isolation_enabled)
 VALUES($1,$2,$3,'production','protected',true)`, legacy, project.ID, owner.ID); err != nil {
		t.Fatal(err)
	}
	appRow, err := st.CreateApp(ctx, store.App{TenantID: owner.ID, Name: "web", Image: "nginx:1", Type: "web_service", Branch: "main", Port: 80, Replicas: 1, Tier: "starter"})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SetAppEnvironment(ctx, appRow.ID, project.ID, legacy); err != nil {
		t.Fatal(err)
	}
	app, pg, kv := metadataResources(owner.ID)
	app.Name, pg.Name, kv.Name = appRow.ID, ids.New(ids.Postgres), ids.New(ids.KeyValue)
	app.Labels = core.TenantLabels(owner.ID)
	app.Labels[store.LabelManagedBy], app.Labels[store.LabelAppID] = store.ManagedByValue, appRow.ID
	for _, object := range []client.Object{app, pg, kv} {
		object.SetNamespace(owner.ID)
		object.GetLabels()[core.LabelEnvironment] = legacy
		object.GetLabels()[core.LabelProject] = project.ID
	}
	base := serverBase(t, st)
	base.Client = fakeClient(app, pg, kv)
	base.Authz = &fakeMembershipChecker{memberOf: map[string]string{owner.ID: "user:client-1"}}
	handler, srv := serverWith(t, base, Deps{WorkspaceStore: st, Store: st, ProjectsStore: st, EnvironmentsStore: st, AppPlacements: st})
	session := mcpSessionAs(t, srv, "client-1")
	request := func(method, path, body string, status int) []byte {
		t.Helper()
		rec := do(t, handler, method, path, testToken, body)
		if rec.Code != status {
			t.Fatalf("%s %s = %d, want %d: %s", method, path, rec.Code, status, rec.Body.String())
		}
		return rec.Body.Bytes()
	}
	for _, input := range []string{legacy, canonical} {
		var env environments.EnvironmentView
		if err := json.Unmarshal(request(http.MethodGet, "/v1/environments/"+input, "", http.StatusOK), &env); err != nil {
			t.Fatal(err)
		}
		if env.ID != canonical || !slices.Equal(env.ServiceIDs, []string{appRow.ID}) || !slices.Equal(env.DatabaseIDs, []string{pg.Name}) || !slices.Equal(env.KeyValueIDs, []string{kv.Name}) || env.ProtectedStatus != "protected" {
			t.Fatalf("legacy memberships/control changed: %+v", env)
		}
		gqlEnv := gql(t, handler, fmt.Sprintf(`{ environment(id:%q) { id serviceIds databaseIds keyValueIds protectedStatus } }`, input))["environment"].(map[string]any)
		if gqlEnv["id"] != canonical || gqlEnv["protectedStatus"] != "protected" {
			t.Fatalf("GraphQL: %+v", gqlEnv)
		}
		mcpEnv := callTool[environments.EnvironmentView](t, session, "get_environment", map[string]any{"id": input})
		if mcpEnv.ID != canonical || !slices.Equal(mcpEnv.DatabaseIDs, env.DatabaseIDs) {
			t.Fatalf("MCP: %+v", mcpEnv)
		}
		stored, err := st.GetEnvironment(ctx, input)
		if err != nil || stored.ID != legacy {
			t.Fatalf("stored identity: %+v %v", stored, err)
		}
	}
	for _, resource := range []struct{ path, id string }{{"services", appRow.ID}, {"postgres", pg.Name}, {"key-value", kv.Name}} {
		got := decodeObject(t, request(http.MethodGet, "/v1/"+resource.path+"/"+resource.id, "", http.StatusOK))
		if got["environmentId"] != canonical || got["projectId"] != project.ID {
			t.Fatalf("%s placement: %+v", resource.path, got)
		}
		for _, input := range []string{legacy, canonical} {
			var listed []json.RawMessage
			if err := json.Unmarshal(request(http.MethodGet, "/v1/"+resource.path+"?ownerId="+owner.ID+"&environmentId="+input, "", http.StatusOK), &listed); err != nil || len(listed) != 1 {
				t.Fatalf("%s alias filter: %d entries, %v", resource.path, len(listed), err)
			}
		}
	}
	projectView := decodeObject(t, request(http.MethodGet, "/v1/projects/"+project.ID, "", http.StatusOK))
	if refs := projectView["environmentIds"].([]any); len(refs) != 1 || refs[0] != canonical {
		t.Fatalf("project refs: %+v", refs)
	}
	request(http.MethodPatch, "/v1/environments/"+canonical, `{"name":"renamed"}`, http.StatusOK)
	request(http.MethodPost, "/v1/postgres/"+pg.Name+"/suspend", "", http.StatusBadRequest)
	request(http.MethodPost, "/v1/key-value/"+kv.Name+"/suspend", "", http.StatusBadRequest)
	placement, err := st.GetAppPlacements(ctx, []string{appRow.ID})
	if err != nil || placement[appRow.ID].EnvironmentID != legacy {
		t.Fatalf("FK changed: %+v %v", placement, err)
	}
	for _, original := range []client.Object{app, pg, kv} {
		current := original.DeepCopyObject().(client.Object)
		if err := base.Client.Get(ctx, client.ObjectKeyFromObject(original), current); err != nil {
			t.Fatal(err)
		}
		if current.GetLabels()[core.LabelEnvironment] != legacy {
			t.Fatalf("%T label identity changed: %+v", current, current.GetLabels())
		}
	}
	// A second row can never occupy the public spelling, even in another project.
	_, err = pool.Exec(ctx, `INSERT INTO environments(id,project_id,tenant_id,name) VALUES($1,$2,$3,'collision')`, canonical, project.ID, owner.ID)
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23514" || pgErr.ConstraintName != "environments_storage_id_namespace" {
		t.Fatalf("alias collision accepted or failed for unrelated reason: %v", err)
	}

	// All three creation families use the same stored namespace and public view.
	direct := decodeObject(t, request(http.MethodPost, "/v1/environments", fmt.Sprintf(`{"name":"direct","projectId":%q}`, project.ID), http.StatusCreated))
	nested := decodeObject(t, request(http.MethodPost, "/v1/projects", fmt.Sprintf(`{"name":"nested","ownerId":%q,"environments":[{"name":"staging"}]}`, owner.ID), http.StatusCreated))
	nestedID := nested["environmentIds"].([]any)[0].(string)
	for _, public := range []string{direct["id"].(string), nestedID} {
		if !regexp.MustCompile(`^evm-[a-z0-9]{20}$`).MatchString(public) {
			t.Fatalf("noncanonical create: %q", public)
		}
		row, err := st.GetEnvironment(ctx, public)
		if err != nil || row.ID != ids.EnvironmentStorageID(public) {
			t.Fatalf("create storage: %+v %v", row, err)
		}
	}
	caller := core.WithIdentity(ctx, core.Identity{Subject: "client-1", Method: "session"})
	_, err = srv.Apps.DeployStack(caller, apps.DeployRequest{OwnerID: owner.ID, Manifest: `projects:
  - name: blueprint
    environments:
      - name: staging
        services:
          - type: web
            name: blueprint-web
            runtime: image
            image:
              url: docker.io/library/nginx:alpine
`})
	if err != nil {
		t.Fatal(err)
	}
	all, err := srv.Environments.ListWorkspace(caller, owner.ID)
	if err != nil || len(all) != 4 {
		t.Fatalf("all creation families: %+v %v", all, err)
	}
	for _, env := range all {
		if !regexp.MustCompile(`^evm-[a-z0-9]{20}$`).MatchString(env.ID) {
			t.Fatalf("noncanonical family: %+v", env)
		}
	}
	var blueprintEnv string
	if err := pool.QueryRow(ctx, `SELECT environment_id FROM apps WHERE tenant_id=$1 AND name='blueprint-web'`, owner.ID).Scan(&blueprintEnv); err != nil || blueprintEnv == "" || blueprintEnv != ids.EnvironmentStorageID(blueprintEnv) {
		t.Fatalf("Blueprint stored placement: %q %v", blueprintEnv, err)
	}
	var untouched appv1alpha1.Database
	if err := base.Client.Get(ctx, client.ObjectKeyFromObject(pg), &untouched); err != nil || untouched.Spec.Suspended {
		t.Fatalf("protected suspend changed resource: %+v %v", untouched.Spec, err)
	}

	// Simulate an imported pre-upgrade collision without changing the shared
	// schema: every DDL/data change below rolls back in this transaction.
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `ALTER TABLE environments DROP CONSTRAINT environments_storage_id_namespace`); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO environments(id,project_id,tenant_id,name) VALUES($1,$2,$3,'imported-collision')`, canonical, project.ID, owner.ID); err != nil {
		t.Fatal(err)
	}
	_, err = tx.Exec(ctx, `ALTER TABLE environments ADD CONSTRAINT environments_storage_id_namespace CHECK (id NOT LIKE 'evm-%')`)
	if !errors.As(err, &pgErr) || pgErr.Code != "23514" {
		t.Fatalf("upgrade accepted an ambiguous imported identity: %v", err)
	}
}
