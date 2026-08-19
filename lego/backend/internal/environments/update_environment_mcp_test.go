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

package environments

import (
	"context"
	"encoding/json"
	"slices"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

// update_environment_mcp_test.go covers w1/m71's environment fold: the ACL
// setter and the four membership setters became optional arguments on the patch
// tool that already existed. set_environment_acl was full-replace ("pass the
// current value of anything you don't mean to change"); the point of the fold is
// that this trap is gone, so that is what these tests assert.

func updateEnvironmentFixture(t *testing.T) (*Service, string, func(map[string]any) EnvironmentView) {
	t.Helper()
	st := newFakeStore()
	st.addProject(store.Project{ID: "prj-1", TenantID: "tea-a", Name: "web-stack"})
	svc, _ := newServiceWithClient(st, sampleApp("web"), sampleApp("worker"))
	e, err := svc.Create(ctxAs("user-a"), "prj-1", "production")
	if err != nil {
		t.Fatalf("Create environment: %v", err)
	}
	if _, err := svc.SetServices(ctxAs("user-a"), e.ID, []string{"web"}); err != nil {
		t.Fatalf("seed membership: %v", err)
	}
	// The in-memory MCP transport does not propagate caller context values, so
	// disable the checker to exercise the adapter rather than session auth —
	// the same shape TestEnvGroupMembershipSurfaces/MCP uses.
	svc.Authz = nil

	ctx := context.Background()
	srv := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	svc.RegisterMCP(srv)
	serverT, clientT := mcp.NewInMemoryTransports()
	if _, err := srv.Connect(ctx, serverT, nil); err != nil {
		t.Fatalf("server connect: %v", err)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil).Connect(ctx, clientT, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })

	call := func(args map[string]any) EnvironmentView {
		t.Helper()
		res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "update_environment", Arguments: args})
		if err != nil || res.IsError {
			t.Fatalf("update_environment %v: err=%v isError=%v", args, err, res != nil && res.IsError)
		}
		b, _ := json.Marshal(res.StructuredContent)
		var out EnvironmentView
		if err := json.Unmarshal(b, &out); err != nil {
			t.Fatalf("decode update_environment: %v", err)
		}
		return out
	}
	return svc, e.ID, call
}

func TestUpdateEnvironmentFoldsMembershipSetters(t *testing.T) {
	_, id, call := updateEnvironmentFixture(t)

	got := call(map[string]any{"id": id, "serviceIds": []string{"worker"}})
	if !slices.Equal(got.ServiceIDs, []string{"worker"}) {
		t.Fatalf("serviceIds = %v, want [worker] (a present list REPLACES)", got.ServiceIDs)
	}
	if got.Name != "production" {
		t.Fatalf("membership change renamed the environment: %q", got.Name)
	}

	// An explicit empty list clears the membership, as the setter did.
	got = call(map[string]any{"id": id, "serviceIds": []string{}})
	if len(got.ServiceIDs) != 0 {
		t.Fatalf("serviceIds = %v, want empty", got.ServiceIDs)
	}
}

// TestUpdateEnvironmentACLNoLongerNeedsARoundTrip is the fold's user-visible
// win: set_environment_acl required all three ACL values on every call, so
// changing one meant reading and re-sending the other two. The patch tool
// changes one and leaves the rest.
func TestUpdateEnvironmentACLNoLongerNeedsARoundTrip(t *testing.T) {
	_, id, call := updateEnvironmentFixture(t)

	call(map[string]any{
		"id":                      id,
		"protectedStatus":         string(ProtectedStatusProtected),
		"networkIsolationEnabled": true,
		"ipAllowListCidrs":        []string{"203.0.113.0/24"},
	})

	// Flip ONE field; the other two must survive.
	got := call(map[string]any{"id": id, "networkIsolationEnabled": false})
	if got.NetworkIsolationEnabled {
		t.Fatalf("networkIsolationEnabled = true, want false")
	}
	if got.ProtectedStatus != ProtectedStatusProtected {
		t.Fatalf("protectedStatus = %q, want protected (an omitted ACL field is not a write)", got.ProtectedStatus)
	}
	if len(got.IPAllowList) != 1 || got.IPAllowList[0].CIDRBlock != "203.0.113.0/24" {
		t.Fatalf("ipAllowList = %+v, want the untouched 203.0.113.0/24 entry", got.IPAllowList)
	}
}

func TestUpdateEnvironmentRejectsConflictingAllowListForms(t *testing.T) {
	svc, envID, _ := updateEnvironmentFixture(t)
	ctx := context.Background()
	srv := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	svc.RegisterMCP(srv)
	serverT, clientT := mcp.NewInMemoryTransports()
	if _, err := srv.Connect(ctx, serverT, nil); err != nil {
		t.Fatalf("server connect: %v", err)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil).Connect(ctx, clientT, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })

	// The two spellings disagree, so the call is refused rather than one form
	// being silently dropped.
	res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "update_environment", Arguments: map[string]any{
		"id":               envID,
		"ipAllowList":      []map[string]any{{"cidrBlock": "10.0.0.0/8"}},
		"ipAllowListCidrs": []string{"192.0.2.0/24"},
	}})
	if err != nil {
		t.Fatalf("transport error: %v", err)
	}
	if !res.IsError {
		t.Fatalf("conflicting allowlist forms should be a tool error: %#v", res)
	}
}

func TestUpdateEnvironmentWithNoFieldsIsAReadOnlyNoOp(t *testing.T) {
	_, id, call := updateEnvironmentFixture(t)

	got := call(map[string]any{"id": id})
	if got.Name != "production" || !slices.Equal(got.ServiceIDs, []string{"web"}) {
		t.Fatalf("empty update_environment changed state: %+v", got)
	}
}
