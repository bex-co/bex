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
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bex-co/bex/lego/backend/internal/apps"
	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// placementRoleChecker is a workspace member whose role decides can_manage:
// a Developer (admin false) holds every other relation, an Admin all of them.
type placementRoleChecker struct {
	admin bool
	// manageChecks counts can_manage decisions, so a test can pin how many
	// fresh Admin checks a verb runs.
	manageChecks int
}

func (c *placementRoleChecker) Check(_ context.Context, _, relation, _ string) (bool, error) {
	if relation == core.RelCanManage {
		c.manageChecks++
	}
	return c.admin || relation != core.RelCanManage, nil
}

// TestProtectedEnvironmentPlacementNeedsAnAdminOnEverySurfacePG (w3/m281):
// Render lets only workspace Admins create resources in a protected
// environment, or move them into or out of it (render.com/docs/projects). A
// Developer gets the same coded 403 on REST, GraphQL and MCP for each kind; an
// Admin succeeds; unprotected placements are unchanged.
func TestProtectedEnvironmentPlacementNeedsAnAdminOnEverySurfacePG(t *testing.T) {
	ctx, pool, st := ownerGuardPG(t)
	owner, err := st.CreateWorkspace(ctx, "placement-"+ids.New(ids.Workspace), store.PlanHobby, "client-1")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.DeleteTenant(context.Background(), owner.ID) })
	project, err := st.CreateProject(ctx, owner.ID, "shop")
	if err != nil {
		t.Fatal(err)
	}
	// Created as the API creates them (allow-all inbound seed), so staging is
	// a bare environment and production carries only its protected status.
	environment := func(name, protectedStatus string) string {
		e, err := st.CreateEnvironment(ctx, project.ID, owner.ID, name)
		if err != nil {
			t.Fatal(err)
		}
		if err := st.SetEnvironmentACL(ctx, e.ID, protectedStatus, false, e.IPAllowList); err != nil {
			t.Fatal(err)
		}
		return ids.EnvironmentPublicID(e.ID)
	}
	prod, staging := environment("production", "protected"), environment("staging", "unprotected")

	role := &placementRoleChecker{}
	base := serverBase(t, st)
	base.Client = fakeClient()
	base.Authz = role
	handler, srv := serverWith(t, base, Deps{
		WorkspaceStore: st, Store: st, ProjectsStore: st, EnvironmentsStore: st, AppPlacements: st,
		EnvGroupsStore: st, Secrets: newMemSecretStore(),
	})
	session := mcpSessionAs(t, srv, "client-1")
	refused := codedRefusal{status: http.StatusForbidden, code: core.ProtectedPlacementCode, params: map[string]any{"environmentId": prod}}
	request := func(method, path, body string, status int) map[string]any {
		t.Helper()
		rec := do(t, handler, method, path, testToken, body)
		if rec.Code != status {
			t.Fatalf("%s %s = %d, want %d: %s", method, path, rec.Code, status, rec.Body.String())
		}
		if rec.Body.Len() == 0 {
			return nil
		}
		return decodeObject(t, rec.Body.Bytes())
	}
	serviceBody := func(name, environmentID string) string {
		return fmt.Sprintf(`{"name":%q,"ownerId":%q,"environmentId":%q,"image":{"ownerId":%q,"imagePath":"nginx"}}`, name, owner.ID, environmentID, owner.ID)
	}

	// A Developer cannot create any kind in the protected environment.
	refused.onREST(t, handler, http.MethodPost, "/v1/services", serviceBody("web-dev", prod))
	refused.onGraphQL(t, handler, fmt.Sprintf(`mutation { createService(name: "web-dev", image: "nginx", ownerId: %q, environmentId: %q) { id } }`, owner.ID, prod))
	refused.onMCP(t, session, "create_web_service", map[string]any{"name": "web-dev", "image": "nginx", "runtime": "image", "workspaceId": owner.ID, "environmentId": prod})
	refused.onREST(t, handler, http.MethodPost, "/v1/postgres", fmt.Sprintf(`{"name":"orders-dev","plan":"free","ownerId":%q,"environmentId":%q}`, owner.ID, prod))
	refused.onGraphQL(t, handler, fmt.Sprintf(`mutation { createDatabase(name: "orders-dev", plan: "free", ownerId: %q, environmentId: %q) { id } }`, owner.ID, prod))
	refused.onMCP(t, session, "create_postgres", map[string]any{"name": "orders-dev", "plan": "free", "workspaceId": owner.ID, "environmentId": prod})
	refused.onREST(t, handler, http.MethodPost, "/v1/key-value", fmt.Sprintf(`{"name":"cache-dev","plan":"free","ownerId":%q,"environmentId":%q}`, owner.ID, prod))
	refused.onGraphQL(t, handler, fmt.Sprintf(`mutation { createKeyValue(name: "cache-dev", plan: "free", ownerId: %q, environmentId: %q) { id } }`, owner.ID, prod))
	refused.onMCP(t, session, "create_key_value", map[string]any{"name": "cache-dev", "plan": "free", "workspaceId": owner.ID, "environmentId": prod})
	refused.onREST(t, handler, http.MethodPost, "/v1/env-groups", fmt.Sprintf(`{"name":"shared-dev","ownerId":%q,"environmentId":%q}`, owner.ID, prod))
	refused.onGraphQL(t, handler, fmt.Sprintf(`mutation { createEnvGroup(name: "shared-dev", ownerId: %q, environmentId: %q) { id } }`, owner.ID, prod))
	refused.onMCP(t, session, "create_env_group", map[string]any{"name": "shared-dev", "workspaceId": owner.ID, "environmentId": prod})
	if n := serviceRows(t, pool, owner.ID); n != 0 {
		t.Fatalf("a refused create wrote %d services", n)
	}

	// An unprotected environment, and none, are unchanged for a Developer.
	staged := request(http.MethodPost, "/v1/services", serviceBody("web-staging", staging), http.StatusCreated)
	group := request(http.MethodPost, "/v1/env-groups", fmt.Sprintf(`{"name":"shared","ownerId":%q}`, owner.ID), http.StatusCreated)
	stagedID, groupID := staged["service"].(map[string]any)["id"].(string), group["id"].(string)

	// A Developer cannot move a resource into the protected environment.
	refused.onREST(t, handler, http.MethodPost, "/v1/environments/"+prod+"/resources", fmt.Sprintf(`{"resourceIds":[%q]}`, stagedID))
	refused.onGraphQL(t, handler, fmt.Sprintf(`mutation { moveEnvGroup(id: %q, environmentId: %q) { id } }`, groupID, prod))
	refused.onMCP(t, session, "move_env_group", map[string]any{"id": groupID, "environmentId": prod})

	// An Admin creates and moves in.
	role.admin = true
	admin := request(http.MethodPost, "/v1/services", serviceBody("web-admin", prod), http.StatusCreated)
	if got := admin["service"].(map[string]any)["environmentId"]; got != prod {
		t.Fatalf("admin create landed in %v, want %s", got, prod)
	}
	request(http.MethodPost, "/v1/environments/"+prod+"/resources", fmt.Sprintf(`{"resourceIds":[%q]}`, stagedID), http.StatusOK)
	gql(t, handler, fmt.Sprintf(`mutation { moveEnvGroup(id: %q, environmentId: %q) { id } }`, groupID, prod))

	// A Developer cannot move them out again: to another environment or to
	// Unassigned.
	role.admin = false
	refused.onREST(t, handler, http.MethodDelete, "/v1/environments/"+prod+"/resources?resourceIds="+stagedID, "")
	refused.onGraphQL(t, handler, fmt.Sprintf(`mutation { setEnvironmentServices(id: %q, serviceIds: [%q]) { id } }`, staging, stagedID))
	refused.onMCP(t, session, "move_env_group", map[string]any{"id": groupID, "environmentId": staging})
	refused.onMCP(t, session, "move_env_group", map[string]any{"id": groupID})
	placement, err := st.GetAppPlacements(ctx, []string{stagedID})
	if err != nil || ids.EnvironmentPublicID(placement[stagedID].EnvironmentID) != prod {
		t.Fatalf("a refused move-out changed placement: %+v %v", placement, err)
	}
}

// TestBlueprintPlacementIntoAProtectedEnvironmentNeedsAnAdminPG (w3/m281): a
// Developer's Blueprint that creates a resource in a protected environment
// shows the refusal in its validation plan and its apply writes nothing; an
// Admin's applies; a Developer's re-apply that leaves the resource in place is
// open, as Render's Blueprint exception allows.
func TestBlueprintPlacementIntoAProtectedEnvironmentNeedsAnAdminPG(t *testing.T) {
	ctx, pool, st := ownerGuardPG(t)
	owner, err := st.CreateWorkspace(ctx, "placement-bp-"+ids.New(ids.Workspace), store.PlanHobby, "client-1")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.DeleteTenant(context.Background(), owner.ID) })
	project, err := st.CreateProject(ctx, owner.ID, "shop")
	if err != nil {
		t.Fatal(err)
	}
	prod := ids.New(ids.Environment)
	if _, err := pool.Exec(ctx, `INSERT INTO environments(id,project_id,tenant_id,name,protected_status) VALUES($1,$2,$3,'production','protected')`,
		ids.EnvironmentStorageID(prod), project.ID, owner.ID); err != nil {
		t.Fatal(err)
	}
	role := &placementRoleChecker{}
	base := serverBase(t, st)
	base.Client = fakeClient()
	base.Authz = role
	_, srv := serverWith(t, base, Deps{WorkspaceStore: st, Store: st, ProjectsStore: st, EnvironmentsStore: st, AppPlacements: st})
	caller := core.WithIdentity(ctx, core.Identity{Subject: "client-1", Method: "session"})
	manifest := `projects:
  - name: shop
    environments:
      - name: production
        services:
          - type: web
            name: storefront
            runtime: image
            image:
              url: docker.io/library/nginx:alpine
`
	validation, err := srv.Apps.ValidateBlueprint(caller, owner.ID, manifest, "")
	if err != nil {
		t.Fatal(err)
	}
	if validation.Valid || len(validation.Errors) != 1 || validation.Errors[0].Code != core.ProtectedPlacementCode ||
		validation.Errors[0].Path == nil || !strings.Contains(*validation.Errors[0].Path, "services") {
		t.Fatalf("developer validation = %+v, want one located %s entry", validation, core.ProtectedPlacementCode)
	}
	_, err = srv.Apps.DeployStack(caller, apps.DeployRequest{OwnerID: owner.ID, Manifest: manifest})
	if !isPlacementRefusal(err) {
		t.Fatalf("developer apply = %v, want %s", err, core.ProtectedPlacementCode)
	}
	if n := serviceRows(t, pool, owner.ID); n != 0 {
		t.Fatalf("a refused Blueprint apply wrote %d services", n)
	}

	role.admin = true
	if validation, err := srv.Apps.ValidateBlueprint(caller, owner.ID, manifest, ""); err != nil || !validation.Valid {
		t.Fatalf("admin validation = %+v, %v; want valid", validation, err)
	}
	role.manageChecks = 0
	if _, err := srv.Apps.DeployStack(caller, apps.DeployRequest{OwnerID: owner.ID, Manifest: manifest}); err != nil {
		t.Fatalf("admin apply: %v", err)
	}
	// w3/m292: the whole-manifest pre-check's fresh Admin decision covers the
	// apply's creates; they do not each repeat it.
	if role.manageChecks != 1 {
		t.Fatalf("admin apply ran %d can_manage checks, want 1", role.manageChecks)
	}

	// The service now lives in production; a Developer's re-apply leaves it
	// there, which places nothing.
	role.admin = false
	if validation, err := srv.Apps.ValidateBlueprint(caller, owner.ID, manifest, ""); err != nil || !validation.Valid {
		t.Fatalf("developer re-validation = %+v, %v; want valid", validation, err)
	}
	if _, err := srv.Apps.DeployStack(caller, apps.DeployRequest{OwnerID: owner.ID, Manifest: manifest}); err != nil {
		t.Fatalf("developer re-apply of an in-place service: %v", err)
	}

	// w3/m292: the Blueprint-sync mark admits only the identity-less
	// auto-sync (apps and environments tests); a member's call carrying it is
	// still gated.
	pushed := manifest + `          - type: web
            name: api
            runtime: image
            image:
              url: docker.io/library/nginx:alpine
`
	marked := core.WithBlueprintSync(caller)
	if _, err := srv.Apps.DeployStack(marked, apps.DeployRequest{OwnerID: owner.ID, Manifest: pushed}); !isPlacementRefusal(err) {
		t.Fatalf("a member's marked apply = %v, want %s", err, core.ProtectedPlacementCode)
	}
}

func isPlacementRefusal(err error) bool {
	var coded *core.CodedError
	return errors.As(err, &coded) && coded.Code == core.ProtectedPlacementCode && errors.Is(err, core.ErrForbidden)
}

func serviceRows(t *testing.T, pool *pgxpool.Pool, workspaceID string) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM apps WHERE tenant_id=$1`, workspaceID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}
