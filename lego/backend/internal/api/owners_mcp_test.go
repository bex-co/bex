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
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// owners_mcp_test.go drives Render's stateless per-call workspaceId contract
// end-to-end over a real in-memory MCP connection.

// mcpSessionAs is mcpSession plus a caller Identity on the connect-time
// context — the in-memory transport skips the HTTP auth middleware entirely,
// so tool handlers that read core.IdentityFrom(ctx) (every workspaces verb)
// need it injected here instead. Go's context values propagate from a
// connection's context into each request's derived context, so this is
// visible to every tool call on the returned session.
func mcpSessionAs(t *testing.T, srv *Server, subject string) *mcp.ClientSession {
	t.Helper()
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: subject, Method: "session"})
	serverT, clientT := mcp.NewInMemoryTransports()
	if _, err := srv.MCPServer().Connect(ctx, serverT, nil); err != nil {
		t.Fatalf("server connect: %v", err)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil).Connect(ctx, clientT, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return cs
}

func callTool[T any](t *testing.T, cs *mcp.ClientSession, name string, args map[string]any) T {
	t.Helper()
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	if res.IsError {
		t.Fatalf("%s: tool error: %+v", name, res.Content)
	}
	var out T
	b, err := json.Marshal(res.StructuredContent)
	if err != nil {
		t.Fatalf("%s: marshal structured content: %v", name, err)
	}
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("%s: decode structured content: %v (%s)", name, err, b)
	}
	return out
}

func callToolError(t *testing.T, cs *mcp.ClientSession, name string, args map[string]any) {
	t.Helper()
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err == nil && !res.IsError {
		t.Fatalf("%s: want an error/tool-error, got a normal result", name)
	}
}

func TestMCP_ExplicitWorkspaceScopesListServices(t *testing.T) {
	st := newFakeWSStore()
	first := mustCreate(t, st, "first", "hobby", "client-1")
	second := mustCreate(t, st, "second", "hobby", "client-1")

	cl := fakeClient(
		appWithOwnerLabel("first-web", first.ID),
		appWithOwnerLabel("second-web", second.ID),
	)
	base := &core.Base{Client: cl, Namespace: "default", Workspace: &apiFakeResolver{store: st}}
	srv := NewServer(base, Deps{WorkspaceStore: st})
	cs := mcpSessionAs(t, srv, "client-1")

	// list_workspaces is caller-scoped discovery and carries no selection state.
	lw := callTool[struct {
		Workspaces []struct{ ID, Name string }
	}](t, cs, "list_workspaces", nil)
	if len(lw.Workspaces) != 2 {
		t.Fatalf("list_workspaces = %+v, want 2 workspaces", lw)
	}

	// Omitting workspaceId uses the caller's deterministic default workspace.
	defaulted := callTool[struct{ Services []struct{ Name string } }](t, cs, "list_services", nil)
	if len(defaulted.Services) != 1 || defaulted.Services[0].Name != "first-web" {
		t.Fatalf("default list_services = %+v, want only first-web", defaulted)
	}

	// The explicit workspace applies to this call only.
	scoped := callTool[struct{ Services []struct{ Name string } }](t, cs, "list_services", map[string]any{"workspaceId": second.ID})
	if len(scoped.Services) != 1 || scoped.Services[0].Name != "second-web" {
		t.Fatalf("scoped list_services = %+v, want only second-web", scoped)
	}
	callToolError(t, cs, "list_services", map[string]any{"workspaceId": "tea-does-not-exist"})
	callToolError(t, cs, "list_services", map[string]any{"ownerId": second.ID})
	callToolError(t, cs, "get_service", map[string]any{"serviceId": "second-web", "workspaceId": first.ID})
}

func TestMCPWorkspaceSchemasAreStatelessAndCanonical(t *testing.T) {
	srv := NewServer(&core.Base{}, Deps{})
	cs := mcpSessionAs(t, srv, "client-1")
	listed, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, tool := range listed.Tools {
		seen[tool.Name] = true
		if _, callerScoped := mcpCallerScopedTools[tool.Name]; callerScoped {
			continue
		}
		raw, _ := json.Marshal(tool.InputSchema)
		var schema struct {
			Properties map[string]any `json:"properties"`
		}
		if err := json.Unmarshal(raw, &schema); err != nil {
			t.Fatalf("%s schema: %v", tool.Name, err)
		}
		if schema.Properties["workspaceId"] == nil {
			t.Errorf("%s omits workspaceId", tool.Name)
		}
		if schema.Properties["ownerId"] != nil || schema.Properties["ownerID"] != nil {
			t.Errorf("%s still exposes an MCP ownerId alias", tool.Name)
		}
	}
	for _, removed := range []string{"select_workspace", "get_selected_workspace", "list_key_value_instances"} {
		if seen[removed] {
			t.Errorf("deprecated tool %s is still registered", removed)
		}
	}
}

// TestMCP_CreateDuplicateNameErrors is w4/m19's MCP leg: create_web_service
// twice with the same name in the same (default) workspace must reject the
// second call as a tool error — never a silent redeploy, matching the
// REST/GraphQL 409 shape (TestREST_CreateDuplicateNameIs409,
// TestGraphQL_CreateServiceDuplicateNameErrors in the apps package).
func TestMCP_CreateDuplicateNameErrors(t *testing.T) {
	st := newFakeWSStore()
	mustCreate(t, st, "acme", "hobby", "client-1")

	cl := fakeClient()
	base := &core.Base{Client: cl, Namespace: "default", Workspace: &apiFakeResolver{store: st}}
	srv := NewServer(base, Deps{WorkspaceStore: st})
	cs := mcpSessionAs(t, srv, "client-1")

	callTool[struct{ ID string }](t, cs, "create_web_service",
		map[string]any{"name": "web", "image": "nginx", "runtime": "image", "buildCommand": "", "startCommand": ""})

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "create_web_service", Arguments: map[string]any{"name": "web", "image": "nginx", "runtime": "image", "buildCommand": "", "startCommand": ""},
	})
	if err != nil {
		t.Fatalf("create_web_service duplicate: transport error %v", err)
	}
	if !res.IsError {
		t.Fatal("create_web_service duplicate: want a tool error, got a normal result")
	}
	var msg string
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			msg += tc.Text
		}
	}
	if !strings.Contains(strings.ToLower(msg), "already in use") {
		t.Errorf("tool error = %q, want it to say the name is already in use", msg)
	}
}

// appWithOwnerLabel builds an App CR carrying the tenant-id label the ownerId
// field/filter reads.
func appWithOwnerLabel(name, ownerID string) *appv1alpha1.App {
	a := sampleApp(name)
	a.Labels = map[string]string{core.LabelTenant: ownerID}
	return a
}

func TestMCP_CreateLandsInExplicitWorkspace(t *testing.T) {
	st := newFakeWSStore()
	first := mustCreate(t, st, "first", "hobby", "client-1")
	second := mustCreate(t, st, "second", "hobby", "client-1")

	cl := fakeClient()
	base := &core.Base{Client: cl, Namespace: "default", Workspace: &apiFakeResolver{store: st}}
	srv := NewServer(base, Deps{WorkspaceStore: st})
	cs := mcpSessionAs(t, srv, "client-1")

	// With no workspaceId, a create lands in the caller's DEFAULT workspace (the
	// oldest membership — `first`).
	callTool[struct{ ID string }](t, cs, "create_web_service",
		map[string]any{"name": "before", "image": "nginx", "runtime": "image", "buildCommand": "", "startCommand": ""})
	if got := appTenant(t, cl, "before"); got != first.ID {
		t.Errorf("unselected create landed in %q, want the default workspace %q", got, first.ID)
	}

	// workspaceId applies to this create and lands it in `second`.
	callTool[struct{ ID string }](t, cs, "create_web_service",
		map[string]any{"name": "after", "image": "nginx", "runtime": "image", "buildCommand": "", "startCommand": "", "workspaceId": second.ID})
	if got := appTenant(t, cl, "after"); got != second.ID {
		t.Errorf("explicit create landed in %q, want %q", got, second.ID)
	}

	// A workspace client-1 is not a member of: tool error, nothing created.
	callToolError(t, cs, "create_web_service",
		map[string]any{"name": "stranger", "image": "nginx", "runtime": "image", "buildCommand": "", "startCommand": "", "workspaceId": "tea-not-mine"})
	var a appv1alpha1.App
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "stranger"}, &a); err == nil {
		t.Errorf("a create into a non-member workspace wrote an App (%q) — it must be refused", a.Labels[core.LabelTenant])
	}
}

// appTenant reads back the tenant label the create stamped on the App CR.
// appTenant reads back which workspace a create landed an App in, by its
// public name — object.Name itself is no longer that name for a store-managed
// App (it's core.CRName(tenant, name), w4/m19), so this lists by
// LabelServiceName instead of a direct Get.
func appTenant(t *testing.T, cl client.Client, name string) string {
	t.Helper()
	var list appv1alpha1.AppList
	if err := cl.List(context.Background(), &list,
		client.InNamespace("default"), client.MatchingLabels{core.LabelServiceName: name}); err != nil {
		t.Fatalf("list Apps named %s: %v", name, err)
	}
	if len(list.Items) != 1 {
		t.Fatalf("Apps named %s = %d, want 1", name, len(list.Items))
	}
	return list.Items[0].Labels[core.LabelTenant]
}
