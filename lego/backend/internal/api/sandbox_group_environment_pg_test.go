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
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/sandbox"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// TestSandboxGroupPlacementThroughTheComposedAPIPG (w22/m412): a sandbox group
// is placed in a project environment through the composed server, whose wiring
// hands the sandbox feature the same resolver every other resource kind uses.
// REST (through Render's request gate), GraphQL and MCP create it in an
// environment of the workspace and read it back; another workspace's
// environment is the not-found of an id that does not exist; a Developer is
// refused a protected environment and an Admin is not; an official Render
// client's create, which names no environment, is unchanged.
func TestSandboxGroupPlacementThroughTheComposedAPIPG(t *testing.T) {
	ctx, _, st := ownerGuardPG(t)
	workspace := func(owner string) store.Tenant {
		t.Helper()
		tenant, err := st.CreateWorkspace(ctx, "sbgroup-env-"+ids.New(ids.Workspace), store.PlanHobby, owner)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = st.DeleteTenant(context.Background(), tenant.ID) })
		return tenant
	}
	mine, theirs := workspace("client-1"), workspace("client-2")
	environment := func(tenant store.Tenant, name, protectedStatus string) string {
		t.Helper()
		project, err := st.CreateProject(ctx, tenant.ID, name)
		if err != nil {
			t.Fatal(err)
		}
		e, err := st.CreateEnvironment(ctx, project.ID, tenant.ID, name)
		if err != nil {
			t.Fatal(err)
		}
		if err := st.SetEnvironmentACL(ctx, e.ID, protectedStatus, false, e.IPAllowList); err != nil {
			t.Fatal(err)
		}
		return ids.EnvironmentPublicID(e.ID)
	}
	staging, prod := environment(mine, "staging", "unprotected"), environment(mine, "production", "protected")
	foreign := environment(theirs, "theirs", "unprotected")

	// No sandbox is started, so the OpenSandbox endpoint only has to exist.
	upstream := httptest.NewServer(http.NotFoundHandler())
	t.Cleanup(upstream.Close)
	role := &placementRoleChecker{}
	base := serverBase(t, st)
	base.Client = fakeClient()
	base.Authz = role
	handler, srv := serverWith(t, base, Deps{
		WorkspaceStore: st, Store: st, ProjectsStore: st, EnvironmentsStore: st,
		SandboxClient: sandbox.NewClient(upstream.URL), SandboxGroups: st,
	})
	session := mcpSessionAs(t, srv, "client-1")

	// Each surface creates a group in the staging environment and answers its
	// public evm- id; REST goes through Render's request gate, which widens the
	// create body for the field.
	rest := decodeObject(t, do(t, handler, http.MethodPost, "/v1/sandbox-groups", testToken,
		fmt.Sprintf(`{"ownerId":%q,"name":"rest","environmentId":%q}`, mine.ID, staging)).Body.Bytes())
	if rest["environmentId"] != staging {
		t.Fatalf("REST create = %v, want environmentId %s", rest, staging)
	}
	gqlGroup := gql(t, handler, fmt.Sprintf(`mutation { createSandboxGroup(name: "graphql", ownerId: %q, environmentId: %q) { id environmentId } }`, mine.ID, staging))["createSandboxGroup"].(map[string]any)
	if gqlGroup["environmentId"] != staging {
		t.Fatalf("GraphQL create = %v, want environmentId %s", gqlGroup, staging)
	}
	mcpGroup := callTool[map[string]any](t, session, "create_sandbox_group", map[string]any{"name": "mcp", "workspaceId": mine.ID, "environmentId": staging})
	if mcpGroup["environmentId"] != staging {
		t.Fatalf("MCP create = %v, want environmentId %s", mcpGroup, staging)
	}

	// Reads echo it on every surface.
	restID, gqlID, mcpID := rest["id"].(string), gqlGroup["id"].(string), mcpGroup["id"].(string)
	got := decodeObject(t, do(t, handler, http.MethodGet, "/v1/sandbox-groups/"+restID, testToken, "").Body.Bytes())
	if got["environmentId"] != staging {
		t.Fatalf("REST get = %v, want environmentId %s", got, staging)
	}
	read := gql(t, handler, fmt.Sprintf(`{ sandboxGroup(id: %q) { environmentId } sandboxGroups { id environmentId } }`, gqlID))
	if read["sandboxGroup"].(map[string]any)["environmentId"] != staging {
		t.Fatalf("GraphQL get = %v, want environmentId %s", read, staging)
	}
	if listed := callTool[map[string]any](t, session, "get_sandbox_group", map[string]any{"id": mcpID}); listed["environmentId"] != staging {
		t.Fatalf("MCP get = %v, want environmentId %s", listed, staging)
	}

	// An official Render client's create names no environment: unchanged.
	plain := decodeObject(t, do(t, handler, http.MethodPost, "/v1/sandbox-groups", testToken,
		fmt.Sprintf(`{"ownerId":%q,"name":"plain","region":"oregon"}`, mine.ID)).Body.Bytes())
	if v, present := plain["environmentId"]; !present || v != nil {
		t.Fatalf("a create with no environment = %v, want an explicit null environmentId", plain)
	}

	// Another workspace's environment is not found, as an unknown id is, on
	// every surface, and no group is written.
	before := len(mustGroups(t, st, mine.ID))
	notFound := codedRefusal{status: http.StatusNotFound, code: core.NotFoundCode}
	unknown := ids.EnvironmentPublicID("env-" + "aaaaaaaaaaaaaaaaaaaa")
	for _, env := range []string{foreign, unknown} {
		notFound.onREST(t, handler, http.MethodPost, "/v1/sandbox-groups", fmt.Sprintf(`{"ownerId":%q,"name":"nope","environmentId":%q}`, mine.ID, env))
		notFound.onGraphQL(t, handler, fmt.Sprintf(`mutation { createSandboxGroup(name: "nope", ownerId: %q, environmentId: %q) { id } }`, mine.ID, env))
		notFound.onMCP(t, session, "create_sandbox_group", map[string]any{"name": "nope", "workspaceId": mine.ID, "environmentId": env})
	}

	// A protected environment needs an Admin on every surface.
	refused := codedRefusal{status: http.StatusForbidden, code: core.ProtectedPlacementCode, params: map[string]any{"environmentId": prod}}
	refused.onREST(t, handler, http.MethodPost, "/v1/sandbox-groups", fmt.Sprintf(`{"ownerId":%q,"name":"prod","environmentId":%q}`, mine.ID, prod))
	refused.onGraphQL(t, handler, fmt.Sprintf(`mutation { createSandboxGroup(name: "prod", ownerId: %q, environmentId: %q) { id } }`, mine.ID, prod))
	refused.onMCP(t, session, "create_sandbox_group", map[string]any{"name": "prod", "workspaceId": mine.ID, "environmentId": prod})
	if after := len(mustGroups(t, st, mine.ID)); after != before {
		t.Fatalf("the refused creates left %d groups, want none", after-before)
	}
	role.admin = true
	admin := decodeObject(t, do(t, handler, http.MethodPost, "/v1/sandbox-groups", testToken,
		fmt.Sprintf(`{"ownerId":%q,"name":"prod","environmentId":%q}`, mine.ID, prod)).Body.Bytes())
	if admin["environmentId"] != prod {
		t.Fatalf("an Admin's create = %v, want environmentId %s", admin, prod)
	}

	// Deleting the environment keeps the group and clears the field.
	if err := st.DeleteEnvironment(ctx, ids.EnvironmentStorageID(staging)); err != nil {
		t.Fatal(err)
	}
	after := decodeObject(t, do(t, handler, http.MethodGet, "/v1/sandbox-groups/"+restID, testToken, "").Body.Bytes())
	if v, present := after["environmentId"]; after["id"] != restID || !present || v != nil {
		t.Fatalf("the group after its environment's delete = %v, want the same group with a null environmentId", after)
	}
}

func mustGroups(t *testing.T, st *store.PGStore, workspace string) []store.SandboxGroup {
	t.Helper()
	groups, err := st.SandboxGroups(context.Background(), workspace)
	if err != nil {
		t.Fatal(err)
	}
	return groups
}
