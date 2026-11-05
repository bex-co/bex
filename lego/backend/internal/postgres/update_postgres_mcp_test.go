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
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// update_postgres_mcp_test.go covers w1/m71's datastore fold: the
// replace-shaped Postgres setters became arguments on one patch tool, so the
// property under test is that replace stayed replace and that mentioning one
// field never rewrites another. The parameterOverrides half of that fold was
// retired at w2/066 and is now refused by name.

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

func TestUpdatePostgresIPAllowListLeavesDeclaredParametersAlone(t *testing.T) {
	_, cl, call := updatePostgresFixture(t)

	// ipAllowList alone: replaces the list, leaves the declared parameters alone.
	if res := call("update_postgres", map[string]any{
		"postgresId":  "dpg-upd",
		"ipAllowList": []map[string]any{{"cidrBlock": "198.51.100.0/24", "description": "vpn"}},
	}); res.IsError {
		t.Fatalf("update_postgres ipAllowList: %+v", res.Content)
	}
	spec := postgresSpec(t, cl)
	if len(spec.IPAllowList) != 1 || spec.IPAllowList[0].CIDR != "198.51.100.0/24" {
		t.Fatalf("ipAllowList did not replace: %#v", spec.IPAllowList)
	}
	if len(spec.Parameters) != 1 || spec.Parameters["work_mem"] != "16MB" {
		t.Fatalf("ipAllowList call touched the declared parameters: %#v", spec.Parameters)
	}

	// An empty list clears, which is what the retired setter did.
	if res := call("update_postgres", map[string]any{"postgresId": "dpg-upd", "ipAllowListCidrs": []string{}}); res.IsError {
		t.Fatalf("update_postgres empty ipAllowListCidrs: %+v", res.Content)
	}
	spec = postgresSpec(t, cl)
	if len(spec.IPAllowList) != 0 || spec.Parameters["work_mem"] != "16MB" {
		t.Fatalf("empty allowlist did not clear only the allowlist: params=%#v allowlist=%#v", spec.Parameters, spec.IPAllowList)
	}
}

// TestUpdatePostgresRefusesRetiredParameterOverrides — w2/066 (user decision
// D2=A): Render retired parameterOverrides, so update_postgres refuses it by
// name with guidance toward ALTER DATABASE/ROLE … SET, for a write and a dry
// run alike, with or without a neighbouring field, and {} included. The
// database's existing spec.parameters must survive every refusal untouched.
func TestUpdatePostgresRefusesRetiredParameterOverrides(t *testing.T) {
	for name, args := range map[string]map[string]any{
		"replace":       {"parameterOverrides": map[string]any{"max_connections": "200"}},
		"empty map":     {"parameterOverrides": map[string]any{}},
		"dry run":       {"parameterOverrides": map[string]any{"max_connections": "500"}, "dryRun": true},
		"with neighbor": {"parameterOverrides": map[string]any{"work_mem": "8MB"}, "name": "renamed"},
	} {
		t.Run(name, func(t *testing.T) {
			_, cl, call := updatePostgresFixture(t)
			args["postgresId"] = "dpg-upd"
			res := call("update_postgres", args)
			if !res.IsError {
				t.Fatalf("parameterOverrides should be refused: %+v", res.Content)
			}
			text := mcpResultText(res)
			if !strings.Contains(text, "parameterOverrides") || !strings.Contains(text, "ALTER DATABASE") {
				t.Fatalf("refusal must name parameterOverrides and point at ALTER DATABASE: %q", text)
			}
			spec := postgresSpec(t, cl)
			if len(spec.Parameters) != 1 || spec.Parameters["work_mem"] != "16MB" || spec.Name != "dpg-upd" {
				t.Fatalf("refused call changed the database: name=%q params=%#v", spec.Name, spec.Parameters)
			}
		})
	}
}

func mcpResultText(res *mcp.CallToolResult) string {
	var b strings.Builder
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			b.WriteString(tc.Text)
		}
	}
	return b.String()
}

func TestUpdatePostgresDryRunWritesNothing(t *testing.T) {
	_, cl, call := updatePostgresFixture(t)

	if res := call("update_postgres", map[string]any{
		"postgresId": "dpg-upd",
		"name":       "previewed",
		"dryRun":     true,
	}); res.IsError {
		t.Fatalf("dry-run update_postgres: %+v", res.Content)
	}
	if got := postgresSpec(t, cl).Name; got != "dpg-upd" {
		t.Fatalf("dry run wrote through: name = %q", got)
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
		db := getDatabase(t, cl, "dpg-upd")
		db.Spec.Plan = "basic-256mb"
		if err := cl.Update(context.Background(), db); err != nil {
			t.Fatal(err)
		}
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
