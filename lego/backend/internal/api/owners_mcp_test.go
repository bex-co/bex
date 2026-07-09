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
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// owners_mcp_test.go drives the w6/m2/t005 MCP workspace trio end-to-end over
// one real (in-memory transport) MCP session: list_workspaces, select_workspace,
// get_selected_workspace, and the scoping it gives list_services — the DoD's
// "over MCP, an agent calls list_workspaces, select_workspace(ownerID), and
// subsequent tool calls (list services) are scoped to the selection" clause.

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

func TestMCP_WorkspaceSelectionScopesListServices(t *testing.T) {
	st := newFakeWSStore()
	first := mustCreate(t, st, "first", "hobby", "client-1")
	second := mustCreate(t, st, "second", "hobby", "client-1")

	cl := fakeClient(
		appWithOwnerLabel("first-web", first.ID),
		appWithOwnerLabel("second-web", second.ID),
	)
	base := &core.Base{Client: cl, Namespace: "default"}
	srv := NewServer(base, Deps{WorkspaceStore: st})
	cs := mcpSessionAs(t, srv, "client-1")

	// list_workspaces: both, unscoped (two workspaces => no auto-select).
	lw := callTool[struct {
		Workspaces []struct{ ID, Name string }
		Selected   *struct{ ID string }
	}](t, cs, "list_workspaces", nil)
	if len(lw.Workspaces) != 2 || lw.Selected != nil {
		t.Fatalf("list_workspaces = %+v, want 2 workspaces and no auto-selection", lw)
	}

	// Before any selection, list_services is unscoped: both Apps.
	all := callTool[struct{ Services []struct{ Name string } }](t, cs, "list_services", nil)
	if len(all.Services) != 2 {
		t.Fatalf("unscoped list_services = %+v, want 2", all)
	}

	// select_workspace(second) — the DoD flow.
	sel := callTool[struct {
		Selected struct{ ID, Name string }
	}](t, cs, "select_workspace", map[string]any{"ownerID": second.ID})
	if sel.Selected.ID != second.ID {
		t.Fatalf("select_workspace result = %+v", sel)
	}

	// get_selected_workspace echoes it.
	got := callTool[struct {
		Selected struct{ ID string }
	}](t, cs, "get_selected_workspace", nil)
	if got.Selected.ID != second.ID {
		t.Fatalf("get_selected_workspace = %+v, want %s", got, second.ID)
	}

	// list_services is now scoped to the selection: only second-web.
	scoped := callTool[struct{ Services []struct{ Name string } }](t, cs, "list_services", nil)
	if len(scoped.Services) != 1 || scoped.Services[0].Name != "second-web" {
		t.Fatalf("scoped list_services = %+v, want only second-web", scoped)
	}

	// Foreign/unknown workspace id: tool error, selection left untouched.
	callToolError(t, cs, "select_workspace", map[string]any{"ownerID": "tea-does-not-exist"})
	still := callTool[struct {
		Selected struct{ ID string }
	}](t, cs, "get_selected_workspace", nil)
	if still.Selected.ID != second.ID {
		t.Fatalf("selection changed after a failed select_workspace: %+v", still)
	}
}

// appWithOwnerLabel builds an App CR carrying the tenant-id label the ownerId
// field/filter reads.
func appWithOwnerLabel(name, ownerID string) *appv1alpha1.App {
	a := sampleApp(name)
	a.Labels = map[string]string{core.LabelTenant: ownerID}
	return a
}
