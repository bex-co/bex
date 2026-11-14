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

package keyvalue

import (
	"encoding/json"
	"net/http"
	"reflect"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/rendertest"
	"github.com/bex-co/bex/lego/backend/internal/resourcemeta"
)

// kvMCPCall connects an in-memory MCP client to svc as user-a and returns a
// caller that fails the test on any tool error.
func kvMCPCall(t *testing.T, svc *Service) func(name string, args map[string]any) *mcp.CallToolResult {
	t.Helper()
	srv := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	svc.RegisterMCP(srv)
	ctx := ctxAs("user-a")
	serverT, clientT := mcp.NewInMemoryTransports()
	if _, err := srv.Connect(ctx, serverT, nil); err != nil {
		t.Fatal(err)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil).Connect(ctx, clientT, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return func(name string, args map[string]any) *mcp.CallToolResult {
		t.Helper()
		res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
		if err != nil || res.IsError {
			t.Fatalf("%s: err=%v result=%+v", name, err, res)
		}
		return res
	}
}

// TestMCPKeyValueToolsAnswerRESTsDetail pins w4/m378 t010: get_key_value,
// create_key_value and every list_key_value item are REST's Render-shaped
// detail (owner object, options), equal to the REST body for the same
// instance, with none of bex's flat ownerId/maxmemoryPolicy/persistenceMode.
// An empty workspace answers Render's sentence, not an empty array.
func TestMCPKeyValueToolsAnswerRESTsDetail(t *testing.T) {
	svc, _ := newService()
	svc.Workspace = fakeWorkspace{"user-a": "tea-a"}
	svc.Owners = fixedOwnerResolver{"tea-a": resourcemeta.Owner{ID: "tea-a", Name: "Acme", Email: "a@acme.test", Type: "team"}}
	svc.Metadata = resourcemeta.Config{Region: "fsn1", DashboardBaseURL: "https://dashboard.bex.co"}
	mcpCall := kvMCPCall(t, svc)
	call := func(name string, args map[string]any) *mcp.CallToolResult {
		t.Helper()
		res := mcpCall(name, args)
		rendertest.AssertRenderMCPResult(t, name, res, "createdAt") // the fake API server mints no creationTimestamp
		return res
	}

	if got := rendertest.MCPText(t, call("list_key_value", nil)); got != "No Key Value instances found" {
		t.Errorf("empty list_key_value = %q, want Render's sentence", got)
	}

	created := call("create_key_value", map[string]any{"name": "kv-mcp", "plan": "free", "maxmemoryPolicy": "allkeys_lru"})
	var createdBody map[string]any
	rendertest.DecodeMCPText(t, created, &createdBody)
	id, _ := createdBody["id"].(string)

	var restGet map[string]any
	if rec := serveREST(svc, http.MethodGet, "/v1/key-value/"+id, ""); rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &restGet) != nil {
		t.Fatalf("REST get = %d %s", rec.Code, rec.Body)
	}
	var restList []map[string]any
	if err := json.Unmarshal(serveREST(svc, http.MethodGet, "/v1/key-value", "").Body.Bytes(), &restList); err != nil || len(restList) != 1 {
		t.Fatalf("REST list = %v (%v)", restList, err)
	}

	var mcpGet map[string]any
	rendertest.DecodeMCPText(t, call("get_key_value", map[string]any{"keyValueId": id}), &mcpGet)
	var mcpList []map[string]any
	rendertest.DecodeMCPText(t, call("list_key_value", nil), &mcpList)
	if len(mcpList) != 1 {
		t.Fatalf("list_key_value = %v, want one item", mcpList)
	}

	for name, body := range map[string]map[string]any{"get_key_value": mcpGet, "list_key_value[0]": mcpList[0], "create_key_value": createdBody} {
		owner, _ := body["owner"].(map[string]any)
		options, _ := body["options"].(map[string]any)
		if owner["id"] != "tea-a" || owner["email"] != "a@acme.test" || owner["type"] != "team" || options["maxmemoryPolicy"] != "allkeys_lru" {
			t.Errorf("%s owner/options = %v/%v, want REST's nested objects", name, owner, options)
		}
		for _, flat := range []string{"ownerId", "maxmemoryPolicy", "persistenceMode"} {
			if _, ok := body[flat]; ok {
				t.Errorf("%s carries bex's flat %s: %v", name, flat, body)
			}
		}
	}
	if !reflect.DeepEqual(mcpGet, restGet) {
		t.Errorf("get_key_value = %v,\nwant REST's %v", mcpGet, restGet)
	}
	if !reflect.DeepEqual(mcpList[0], restList[0]["keyValue"]) {
		t.Errorf("list_key_value[0] = %v,\nwant REST's item %v", mcpList[0], restList[0]["keyValue"])
	}
}
