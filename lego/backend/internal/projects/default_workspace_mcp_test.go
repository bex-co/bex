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
package projects

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

func TestMCPDefaultsToCallerWorkspace(t *testing.T) {

	st := newFakeProjectStore(store.Project{ID: "prj-1", TenantID: "tea-a", Name: "default-project"}, store.Project{ID: "prj-2", TenantID: "tea-b", Name: "foreign-project"})
	svc := &Service{Base: &core.Base{Authz: allowChecker{}, Workspace: defaultProjectWorkspace{}}, Store: st}
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "member", Method: "session"})

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

	res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "list_projects", Arguments: map[string]any{}})
	if err != nil || res.IsError {
		t.Fatalf("list_projects: %+v %v", res, err)
	}
	b, err := json.Marshal(res.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "default-project") || strings.Contains(string(b), "foreign-project") {
		t.Fatalf("wrong workspace projects: %s", b)
	}
}

type defaultProjectWorkspace struct{}

func (defaultProjectWorkspace) Tenant(context.Context, core.Identity) (string, bool) {
	return "tea-a", true
}
func (defaultProjectWorkspace) IsMember(_ context.Context, _ core.Identity, workspace string) (bool, error) {
	return workspace == "tea-a", nil
}
