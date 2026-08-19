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
	"slices"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// update_project_mcp_test.go covers w1/m71's grouping fold: three membership
// setters became optional arguments on one patch tool. Membership is
// replace-shaped, so the tests assert that a present list still replaces, an
// absent one is not a write, and the rename verb still exists beside the patch.

func updateProjectFixture(t *testing.T) (*Service, func(map[string]any) ProjectView) {
	t.Helper()
	st := newFakeProjectStore(store.Project{ID: "prj-1", TenantID: "tea-a", Name: "web-stack"})
	st.services["prj-1"] = []string{"web"}
	svc := &Service{Base: &core.Base{Authz: allowChecker{}}, Store: st}
	// Databases and key-value instances are label-backed on their CRs, so their
	// membership comes from the resource indexes rather than the store.
	svc.Databases = newFakeResourceIndex("tea-a",
		projectResource{id: "dpg-one", projectID: "prj-1"},
		projectResource{id: "dpg-two"})
	svc.KeyValues = newFakeResourceIndex("tea-a",
		projectResource{id: "red-one", projectID: "prj-1"})

	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "user-a", Method: "session"})
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

	call := func(args map[string]any) ProjectView {
		t.Helper()
		res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "update_project", Arguments: args})
		if err != nil || res.IsError {
			t.Fatalf("update_project %v: err=%v isError=%v", args, err, res != nil && res.IsError)
		}
		b, _ := json.Marshal(res.StructuredContent)
		var out ProjectView
		if err := json.Unmarshal(b, &out); err != nil {
			t.Fatalf("decode update_project: %v", err)
		}
		return out
	}
	return svc, call
}

func TestUpdateProjectFoldsMembershipSetters(t *testing.T) {
	_, call := updateProjectFixture(t)

	got := call(map[string]any{"id": "prj-1", "serviceIds": []string{"worker"}})
	if !slices.Equal(got.ServiceIDs, []string{"worker"}) {
		t.Fatalf("serviceIds = %v, want [worker] (a present list REPLACES)", got.ServiceIDs)
	}
	// The other two memberships were not mentioned, so they are untouched.
	if !slices.Equal(got.DatabaseIDs, []string{"dpg-one"}) || !slices.Equal(got.KeyValueIDs, []string{"red-one"}) {
		t.Fatalf("omitted memberships changed: databases=%v keyValues=%v", got.DatabaseIDs, got.KeyValueIDs)
	}

	got = call(map[string]any{"id": "prj-1", "databaseIds": []string{"dpg-two"}, "keyValueIds": []string{}})
	if !slices.Equal(got.DatabaseIDs, []string{"dpg-two"}) {
		t.Fatalf("databaseIds = %v, want [dpg-two]", got.DatabaseIDs)
	}
	if len(got.KeyValueIDs) != 0 {
		t.Fatalf("keyValueIds = %v, want empty (an explicit [] clears)", got.KeyValueIDs)
	}
	if !slices.Equal(got.ServiceIDs, []string{"worker"}) {
		t.Fatalf("service membership changed while setting datastores: %v", got.ServiceIDs)
	}
}

func TestUpdateProjectRenamesAndLeavesMembershipAlone(t *testing.T) {
	_, call := updateProjectFixture(t)

	got := call(map[string]any{"id": "prj-1", "name": "renamed-stack"})
	if got.Name != "renamed-stack" {
		t.Fatalf("name = %q", got.Name)
	}
	if !slices.Equal(got.ServiceIDs, []string{"web"}) {
		t.Fatalf("rename cleared service membership: %v", got.ServiceIDs)
	}
}

// TestUpdateProjectRenameReplacesTheRetiredVerb: w1/m74 retired rename_project
// because update_project's own name field already did the job — this asserts the
// capability survived the tool.
func TestUpdateProjectRenameReplacesTheRetiredVerb(t *testing.T) {
	_, call := updateProjectFixture(t)

	got := call(map[string]any{"id": "prj-1", "name": "renamed-by-patch", "serviceIds": []string{"web", "worker"}})
	if got.Name != "renamed-by-patch" {
		t.Fatalf("name = %q", got.Name)
	}
	if !slices.Equal(got.ServiceIDs, []string{"web", "worker"}) {
		t.Fatalf("a rename in the same call as membership lost the membership: %v", got.ServiceIDs)
	}
}

func TestUpdateProjectWithNoFieldsIsAReadOnlyNoOp(t *testing.T) {
	_, call := updateProjectFixture(t)

	got := call(map[string]any{"id": "prj-1"})
	if got.Name != "web-stack" || !slices.Equal(got.ServiceIDs, []string{"web"}) ||
		!slices.Equal(got.DatabaseIDs, []string{"dpg-one"}) || !slices.Equal(got.KeyValueIDs, []string{"red-one"}) {
		t.Fatalf("empty update_project changed state: %+v", got)
	}
}
