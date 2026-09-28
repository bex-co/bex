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
package workspaces

import (
	"encoding/json"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

func TestMCPDefaultsToCallerWorkspace(t *testing.T) {

	st := newFakeStore()
	svc := allowSvc(st, &fakeGranter{}, nil, nil)
	ctx := ctxAs("member")
	if _, err := svc.Create(ctx, "default-workspace", "hobby"); err != nil {
		t.Fatal(err)
	}

	srv := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	svc.RegisterMCP(srv)
	serverT, clientT := mcp.NewInMemoryTransports()
	if _, err := srv.Connect(ctx, serverT, nil); err != nil {
		t.Fatal(err)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil).Connect(ctx, clientT, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()

	res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "get_workspace_limits", Arguments: map[string]any{}})
	if err != nil || res.IsError {
		t.Fatalf("get_workspace_limits: %+v %v", res, err)
	}
	b, err := json.Marshal(res.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	var result ResourceLimitsView
	if err := json.Unmarshal(b, &result); err != nil {
		t.Fatal(err)
	}
	if result.Services.Limit != int(store.QuotaCapsForPlan("hobby").Services) {
		t.Fatalf("wrong default workspace limits: %s", b)
	}
}
