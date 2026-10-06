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

package apps

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
	"github.com/bex-co/bex/lego/backend/internal/testenv"
)

// TestPGCreateDryRunMeetsTheStoreRefusals is w5/m116 with the store on: a
// create's dry-run refuses exactly what its real create refuses — a host
// another service claims while its DNS verification is still pending, and a
// name another service is displayed as — and writes no service row.
func TestPGCreateDryRunMeetsTheStoreRefusals(t *testing.T) {
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		testenv.Skip(t, "BEX_TEST_DB_URI not set")
	}
	if err := store.Migrate(uri); err != nil {
		t.Fatal(err)
	}
	const owner = "dryrun-store-owner"
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: owner, Method: "session"})
	pool, err := pgxpool.New(ctx, uri)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	st := store.NewPGStore(pool)
	workspace, err := st.CreateWorkspace(ctx, "dryrun-store-"+id.New(id.Owner), store.PlanHobby, owner)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.DeleteTenant(ctx, workspace.ID) })
	svc := &Service{Base: &core.Base{Client: fakeClient(), Namespace: "default", Workspace: fakeWorkspace{owner: workspace.ID}}, Store: st}

	// The holder claims the host (pending: nothing verifies DNS here) and is
	// displayed under another service's would-be name.
	host := "shop-" + id.New(id.Owner) + ".example.com"
	if _, err := svc.Create(ctx, CreateRequest{Name: "holder", Image: "nginx:1", Hosts: []string{host}}); err != nil {
		t.Fatalf("create holder: %v", err)
	}
	if _, err := svc.SetDisplayName(ctx, "holder", "storefront"); err != nil {
		t.Fatalf("display holder as storefront: %v", err)
	}
	rows := func() int {
		t.Helper()
		all, err := st.ListApps(ctx)
		if err != nil {
			t.Fatal(err)
		}
		n := 0
		for _, row := range all {
			if row.TenantID == workspace.ID {
				n++
			}
		}
		return n
	}
	before := rows()

	for name, req := range map[string]CreateRequest{
		"a host claimed and pending verification": {Name: "second", Image: "nginx:1", Hosts: []string{host}},
		"a name another service is displayed as":  {Name: "storefront", Image: "nginx:1"},
	} {
		t.Run(name, func(t *testing.T) {
			dry := req
			dry.DryRun = true
			_, dryErr := svc.Create(ctx, dry)
			if after := rows(); after != before {
				t.Fatalf("dry-run left %d service rows, want %d", after, before)
			}
			_, realErr := svc.Create(ctx, req)
			if realErr == nil {
				t.Fatal("the real create succeeded; the case does not exercise a refusal")
			}
			if dryErr == nil || dryErr.Error() != realErr.Error() {
				t.Fatalf("dry-run error = %v\nreal error    = %v", dryErr, realErr)
			}
		})
	}

	// A rename to a name another service is displayed as: the store refuses
	// it only as it writes, and the patch preflight reads that refusal ahead.
	t.Run("a rename to a name another service is displayed as", func(t *testing.T) {
		if _, err := svc.Create(ctx, CreateRequest{Name: "renamer", Image: "nginx:1"}); err != nil {
			t.Fatalf("create renamer: %v", err)
		}
		name := "storefront"
		_, dryErr := svc.ApplyServicePatchDryRun(ctx, "renamer", ServicePatch{DisplayName: &name})
		_, realErr := svc.ApplyServicePatch(ctx, "renamer", ServicePatch{DisplayName: &name})
		if realErr == nil {
			t.Fatal("the real rename succeeded; the case does not exercise a refusal")
		}
		if dryErr == nil || dryErr.Error() != realErr.Error() {
			t.Fatalf("dry-run error = %v\nreal error    = %v", dryErr, realErr)
		}
	})
}
