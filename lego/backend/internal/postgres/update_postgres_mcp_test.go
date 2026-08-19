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

package postgres

import (
	"context"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// update_postgres_mcp_test.go covers w1/m71's datastore fold: the two
// replace-shaped Postgres setters became arguments on one patch tool. Both were
// already "replace this whole list/map", so the property under test is that
// replace stayed replace and that mentioning one never rewrites the other.

func updatePostgresFixture(t *testing.T) (*Service, client.Client, func(string, map[string]any) *mcp.CallToolResult) {
	t.Helper()
	db := &appv1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{Name: "dpg-upd", Namespace: "default"},
		Spec: appv1alpha1.DatabaseSpec{
			Name: "dpg-upd", Plan: "free",
			IPAllowList: []appv1alpha1.IPAllowEntry{{CIDR: "203.0.113.0/24", Description: "office"}},
			Parameters:  map[string]string{"work_mem": "16MB"},
		},
		Status: appv1alpha1.DatabaseStatus{Phase: appv1alpha1.DBPhaseReady},
	}
	svc, cl := newService(db)
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
	call := func(name string, args map[string]any) *mcp.CallToolResult {
		t.Helper()
		res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
		if err != nil {
			t.Fatalf("%s: transport error: %v", name, err)
		}
		return res
	}
	return svc, cl, call
}

func postgresSpec(t *testing.T, cl client.Client) appv1alpha1.DatabaseSpec {
	t.Helper()
	var db appv1alpha1.Database
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "dpg-upd"}, &db); err != nil {
		t.Fatalf("get database: %v", err)
	}
	return db.Spec
}

func TestUpdatePostgresFoldsBothSetters(t *testing.T) {
	_, cl, call := updatePostgresFixture(t)

	// parameterOverrides alone: replaces the map, leaves the allowlist alone.
	if res := call("update_postgres", map[string]any{
		"postgresId":         "dpg-upd",
		"parameterOverrides": map[string]any{"max_connections": "200"},
	}); res.IsError {
		t.Fatalf("update_postgres parameterOverrides: %+v", res.Content)
	}
	spec := postgresSpec(t, cl)
	if spec.Parameters["max_connections"] != "200" || spec.Parameters["work_mem"] != "" {
		t.Fatalf("parameterOverrides did not REPLACE the map: %#v", spec.Parameters)
	}
	if len(spec.IPAllowList) != 1 || spec.IPAllowList[0].Description != "office" {
		t.Fatalf("parameterOverrides call touched the omitted allowlist: %#v", spec.IPAllowList)
	}

	// ipAllowList alone: replaces the list, leaves the parameters alone.
	if res := call("update_postgres", map[string]any{
		"postgresId":  "dpg-upd",
		"ipAllowList": []map[string]any{{"cidrBlock": "198.51.100.0/24", "description": "vpn"}},
	}); res.IsError {
		t.Fatalf("update_postgres ipAllowList: %+v", res.Content)
	}
	spec = postgresSpec(t, cl)
	if len(spec.IPAllowList) != 1 || spec.IPAllowList[0].CIDR != "198.51.100.0/24" {
		t.Fatalf("ipAllowList did not replace: %#v", spec.IPAllowList)
	}
	if spec.Parameters["max_connections"] != "200" {
		t.Fatalf("ipAllowList call touched the omitted parameters: %#v", spec.Parameters)
	}

	// An empty map/list clears, which is what the retired setters did.
	call("update_postgres", map[string]any{
		"postgresId":         "dpg-upd",
		"parameterOverrides": map[string]any{},
		"ipAllowListCidrs":   []string{},
	})
	spec = postgresSpec(t, cl)
	if len(spec.Parameters) != 0 || len(spec.IPAllowList) != 0 {
		t.Fatalf("empty arguments did not clear: params=%#v allowlist=%#v", spec.Parameters, spec.IPAllowList)
	}
}

func TestUpdatePostgresDryRunWritesNothing(t *testing.T) {
	_, cl, call := updatePostgresFixture(t)

	if res := call("update_postgres", map[string]any{
		"postgresId":         "dpg-upd",
		"parameterOverrides": map[string]any{"max_connections": "500"},
		"dryRun":             true,
	}); res.IsError {
		t.Fatalf("dry-run update_postgres: %+v", res.Content)
	}
	if got := postgresSpec(t, cl).Parameters["work_mem"]; got != "16MB" {
		t.Fatalf("dry run wrote through: parameters = %#v", postgresSpec(t, cl).Parameters)
	}
}

func TestUpdatePostgresRejectsConflictingAllowListForms(t *testing.T) {
	_, cl, call := updatePostgresFixture(t)

	res := call("update_postgres", map[string]any{
		"postgresId":       "dpg-upd",
		"ipAllowList":      []map[string]any{{"cidrBlock": "10.0.0.0/8"}},
		"ipAllowListCidrs": []string{"192.0.2.0/24"},
	})
	if !res.IsError {
		t.Fatalf("conflicting allowlist forms should be a tool error: %+v", res)
	}
	if got := postgresSpec(t, cl).IPAllowList; len(got) != 1 || got[0].CIDR != "203.0.113.0/24" {
		t.Fatalf("rejected call still wrote: %#v", got)
	}
}

// --- w1/m74: the four per-field tools this one absorbed ---

// TestUpdatePostgresCarriesTheFoldedFields covers what rename_postgres,
// update_postgres_plan, update_postgres_version and
// update_postgres_disk_autoscaling used to own. Each field is sent alone: the
// property at risk in a fold is a neighbour being written unasked.
func TestUpdatePostgresCarriesTheFoldedFields(t *testing.T) {
	t.Run("name", func(t *testing.T) {
		_, cl, call := updatePostgresFixture(t)
		if res := call("update_postgres", map[string]any{"postgresId": "dpg-upd", "name": "renamed"}); res.IsError {
			t.Fatalf("rename: %+v", res.Content)
		}
		spec := postgresSpec(t, cl)
		if spec.Name != "renamed" {
			t.Fatalf("spec.name = %q", spec.Name)
		}
		// A rename must not disturb the settings that were already there.
		if len(spec.IPAllowList) != 1 || spec.Parameters["work_mem"] != "16MB" {
			t.Fatalf("rename disturbed neighbouring fields: %#v / %#v", spec.IPAllowList, spec.Parameters)
		}
	})

	t.Run("disk autoscaling", func(t *testing.T) {
		_, cl, call := updatePostgresFixture(t)
		if res := call("update_postgres", map[string]any{"postgresId": "dpg-upd", "enableDiskAutoscaling": true}); res.IsError {
			t.Fatalf("disk autoscaling: %+v", res.Content)
		}
		if !postgresSpec(t, cl).DiskAutoscaling {
			t.Fatal("spec.diskAutoscaling not set")
		}
	})

	t.Run("plan", func(t *testing.T) {
		_, cl, call := updatePostgresFixture(t)
		if res := call("update_postgres", map[string]any{"postgresId": "dpg-upd", "plan": "basic-1gb"}); res.IsError {
			t.Fatalf("plan: %+v", res.Content)
		}
		if got := postgresSpec(t, cl).Plan; got != "basic-1gb" {
			t.Fatalf("spec.plan = %q", got)
		}
	})

	t.Run("dryRun still previews the folded fields", func(t *testing.T) {
		_, cl, call := updatePostgresFixture(t)
		if res := call("update_postgres", map[string]any{"postgresId": "dpg-upd", "plan": "basic-1gb", "dryRun": true}); res.IsError {
			t.Fatalf("dry run: %+v", res.Content)
		}
		if got := postgresSpec(t, cl).Plan; got != "free" {
			t.Fatalf("dry run wrote the plan: %q", got)
		}
	})
}
