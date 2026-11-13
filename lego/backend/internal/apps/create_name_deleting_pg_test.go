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
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
	"github.com/bex-co/bex/lego/backend/internal/testenv"
)

// TestPGCreateUnderAStillDeletingNameIsNamedThenSucceeds is the w2/075 dev-2
// walk on real Postgres: create a service, delete it (its App held
// Terminating by the operator's finalizer), create one under the same name
// at once. With the service's row already gone, the create — dry run and
// real alike — answers 409 SERVICE_NAME_STILL_DELETING and writes no row;
// once the teardown finishes the same create succeeds.
func TestPGCreateUnderAStillDeletingNameIsNamedThenSucceeds(t *testing.T) {
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		testenv.Skip(t, "BEX_TEST_DB_URI not set")
	}
	if err := store.Migrate(uri); err != nil {
		t.Fatal(err)
	}
	const owner = "name-deleting-owner"
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: owner, Method: "session"})
	pool, err := pgxpool.New(ctx, uri)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	st := store.NewPGStore(pool)
	workspace, err := st.CreateWorkspace(ctx, "name-deleting-"+id.New(id.Owner), store.PlanHobby, owner)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.DeleteTenant(ctx, workspace.ID) })
	// A teardown that outlasts the create's wait (w2/m361), kept short here.
	svc := &Service{Base: &core.Base{Client: fakeClient(), Namespace: "default", Workspace: fakeWorkspace{owner: workspace.ID}}, Store: st, DomainTraffic: allowDomainTraffic{}, NameReleaseWait: 100 * time.Millisecond}
	// Storage refuses the held name as the API server does.
	svc.Client = apiServerDryRuns{svc.Client}
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

	req := CreateRequest{Name: "api", Image: "nginx:1"}
	if _, err := svc.Create(ctx, req); err != nil {
		t.Fatalf("create: %v", err)
	}
	key := appKeyIn(t, svc, workspace.ID, "api")
	holdAppFinalizer(t, ctx, svc.Client, key)
	if err := svc.Delete(ctx, "api"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if n := rows(); n != 0 {
		t.Fatalf("%d service rows after the delete, want 0", n)
	}

	for _, dryRun := range []bool{true, false} {
		again := req
		again.DryRun = dryRun
		_, err := svc.Create(ctx, again)
		if err == nil || coded(t, err) != ServiceNameStillDeletingCode {
			t.Fatalf("create (dryRun=%v) during the teardown: err = %v, want %s", dryRun, err, ServiceNameStillDeletingCode)
		}
		if n := rows(); n != 0 {
			t.Fatalf("the refused create (dryRun=%v) left %d service rows", dryRun, n)
		}
	}

	releaseAppFinalizer(t, ctx, svc.Client, key)
	if _, err := svc.Create(ctx, req); err != nil {
		t.Fatalf("create after the teardown: %v", err)
	}
	if n := rows(); n != 1 {
		t.Fatalf("%d service rows after the re-create, want 1", n)
	}
}
