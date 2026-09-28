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
package members

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

func TestMCPDefaultsToCallerWorkspace(t *testing.T) {

	st := newFakeStore(store.PlanPro)
	st.members["member"] = store.TenantMember{TenantID: "tea-1", Subject: "member", Role: "admin"}
	svc := svc(st, newFakeGranter(), nil, roleChecker{relation: "admin"})
	svc.Workspace = membershipResolver{"member": "tea-1"}
	ctx := ctxWith("member")

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

	for _, tool := range []string{"list_workspace_members", "list_workspace_invites", "get_workspace_seat_usage"} {
		t.Run(tool, func(t *testing.T) {
			res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: tool, Arguments: map[string]any{}})
			if err != nil || res.IsError {
				t.Fatalf("%s: %+v %v", tool, res, err)
			}
			b, err := json.Marshal(res.StructuredContent)
			if err != nil {
				t.Fatal(err)
			}
			var result map[string]any
			if err := json.Unmarshal(b, &result); err != nil {
				t.Fatal(err)
			}
			if tool == "get_workspace_seat_usage" && result["used"] != float64(1) {
				t.Fatalf("default workspace seats = %s", b)
			}
			if tool == "list_workspace_members" && !strings.Contains(string(b), "member") {
				t.Fatalf("default workspace members = %s", b)
			}
		})
	}
}
