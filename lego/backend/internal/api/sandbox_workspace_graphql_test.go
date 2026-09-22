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
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/sandbox"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Composed dispatch checks the OAuth class of the new singular query, then
// compares GraphQL ownerId with MCP's actual workspaceId middleware.
func TestSandboxWorkspaceSelectionThroughComposedAPI(t *testing.T) {
	var hits atomic.Int32
	item := func(id, workspace string) map[string]any {
		return map[string]any{"id": id, "metadata": map[string]string{"bex.co/owner": "identity-1", "bex.co/workspace": workspace, "app.bex.co/regime": "sandbox", "bex.co/network-policy": "deny-all"}}
	}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		switch r.URL.Path {
		case "/sandboxes":
			_ = json.NewEncoder(w).Encode([]any{item("os-a", "tea-a"), item("os-b", "tea-b")})
		case "/sandboxes/os-b":
			_ = json.NewEncoder(w).Encode(item("os-b", "tea-b"))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(upstream.Close)
	hydra := newClassHydraScoped(t, "identity-1", "dcr-client", "openid "+core.ScopeRead, []string{bexResource}, map[string]bool{"dcr-client": false})
	base := &core.Base{Client: fakeClient(), Namespace: "default", Workspace: twoWorkspaceResolver{}, Authz: &fakeChecker{allow: true}}
	srv := NewServer(base, Deps{SandboxClient: sandbox.NewClient(upstream.URL)})
	srv.HydraAdminURL, srv.OAuthResource, srv.OAuthRequireAudience, srv.OAuthPlatformClients = hydra.url, bexResource, true, hydra.platformClientIDs
	handler, err := srv.Handler()
	if err != nil {
		t.Fatal(err)
	}
	query := func(text string) map[string]any {
		t.Helper()
		body, _ := json.Marshal(map[string]string{"query": text})
		response := do(t, handler, http.MethodPost, "/graphql", testToken, string(body))
		if response.Code != http.StatusOK {
			t.Fatalf("GraphQL status%d:%s", response.Code, response.Body.String())
		}
		return decodeObject(t, response.Body.Bytes())
	}
	result := query(`{sandbox(id:"os-b",ownerId:"tea-b"){id} sandboxes(ownerId:"tea-b"){id}}`)
	if errs, _ := result["errors"].([]any); len(errs) > 0 {
		t.Fatalf("read-scoped sandbox query refused:%v", errs)
	}
	data := result["data"].(map[string]any)
	if data["sandbox"].(map[string]any)["id"] != "os-b" {
		t.Fatalf("singular query:%v", data)
	}
	listed := data["sandboxes"].([]any)
	if len(listed) != 1 || listed[0].(map[string]any)["id"] != "os-b" {
		t.Fatalf("scoped list:%v", listed)
	}
	cs := mcpSessionAs(t, srv, "identity-1")
	listedMCP := callTool[map[string][]map[string]any](t, cs, "list_sandboxes", map[string]any{"workspaceId": "tea-b"})["sandboxes"]
	if len(listedMCP) != 1 || listedMCP[0]["id"] != "os-b" {
		t.Fatalf("MCP workspace middleware:%v", listedMCP)
	}
	before := hits.Load()
	result = query(`{sandbox(id:"os-b",ownerId:"tea-foreign"){id} sandboxes(ownerId:"tea-foreign"){id}}`)
	if errs, _ := result["errors"].([]any); len(errs) != 2 {
		t.Fatalf("foreign workspace queries not refused:%v", result)
	}
	foreign, err := cs.CallTool(t.Context(), &mcp.CallToolParams{Name: "list_sandboxes", Arguments: map[string]any{"workspaceId": "tea-foreign"}})
	if err != nil || !foreign.IsError {
		t.Fatalf("MCP foreign workspace:%v,%v", foreign, err)
	}
	mutation := do(t, handler, http.MethodPost, "/graphql", testToken, `{"query":"mutation {terminateSandbox(id:\"os-b\",ownerId:\"tea-b\")}"}`)
	assertGQLInsufficientScope(t, mutation, core.ScopeWrite)
	if hits.Load() != before {
		t.Fatal("denied workspace/scope reached upstream")
	}
}
