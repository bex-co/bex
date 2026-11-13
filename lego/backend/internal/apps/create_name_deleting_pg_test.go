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
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
	"github.com/bex-co/bex/lego/backend/internal/testenv"
)

// TestPGCreateUnderADeletedNameSucceedsAtOnce is the w2/075 dev-2 walk on
// real Postgres, with Render's answer (w2/m365): create a service, delete it
// (its App held Terminating by the operator's finalizer), create one under the
// same name at once. The create, dry run and real alike, succeeds; the new
// row records its own App object name and a fresh slug; the name resolves to
// it; and a repeated delete of the old service's id touches nothing.
func TestPGCreateUnderADeletedNameSucceedsAtOnce(t *testing.T) {
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
	svc := &Service{Base: &core.Base{Client: fakeClient(), Namespace: "default", Workspace: fakeWorkspace{owner: workspace.ID}}, Store: st, DomainTraffic: allowDomainTraffic{}}
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
	first, err := svc.Create(ctx, req)
	if err != nil {
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

	dry := req
	dry.DryRun = true
	if _, err := svc.Create(ctx, dry); err != nil {
		t.Fatalf("dry-run create during the teardown: %v", err)
	}
	if n := rows(); n != 0 {
		t.Fatalf("the dry run left %d service rows", n)
	}
	second, err := svc.Create(ctx, req)
	if err != nil {
		t.Fatalf("create during the teardown: %v", err)
	}
	if second.ID == first.ID {
		t.Fatalf("the re-created service reuses the deleted one's id %s", first.ID)
	}
	row, err := st.GetApp(ctx, second.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(row.ObjectName, key.Name+"-") || row.AppObjectName() != row.ObjectName {
		t.Fatalf("object name = %q, want %s-xxxx beside the old App", row.ObjectName, key.Name)
	}
	if row.Slug == "api" || !strings.HasPrefix(row.Slug, "api-") {
		t.Fatalf("slug = %q, want a fresh api-xxxx: the old App still holds its host", row.Slug)
	}
	if got, err := svc.Get(ctx, "api"); err != nil || got.ID != second.ID {
		t.Fatalf("GET api = %+v, %v; want the new service %s", got, err, second.ID)
	}
	// A repeated delete of the old service, still reachable by id, is done
	// already and touches nothing (it would purge the name's secrets).
	if err := svc.Delete(ctx, first.ID); err != nil {
		t.Fatalf("repeat delete of %s: %v", first.ID, err)
	}
	if n := rows(); n != 1 {
		t.Fatalf("%d service rows after the repeat delete, want the new one", n)
	}
	releaseAppFinalizer(t, ctx, svc.Client, key)
	if got, err := svc.Get(ctx, "api"); err != nil || got.ID != second.ID {
		t.Fatalf("GET api after the old teardown = %+v, %v", got, err)
	}
}
