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

package store_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
	"github.com/bex-co/bex/lego/backend/internal/testenv"
)

// TestBuildSourceNamesAreUniquePG pins w9/m180 t004: a build source's name is
// unique among the workspace's build sources on create and rename, under a
// lock so concurrent creates cannot both take it. It is not a service name:
// a service and a build source may share one, as can two workspaces.
func TestBuildSourceNamesAreUniquePG(t *testing.T) {
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		testenv.Skip(t, "BEX_TEST_DB_URI not set")
	}
	if err := store.Migrate(uri); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, uri)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	st := store.NewPGStore(pool)
	stamp := fmt.Sprintf("%d", time.Now().UnixNano())
	ws, err := st.CreateWorkspace(ctx, "bsn-"+stamp, store.PlanHobby, "bsn-"+stamp)
	if err != nil {
		t.Fatal(err)
	}
	other, err := st.CreateWorkspace(ctx, "bsn2-"+stamp, store.PlanHobby, "bsn2-"+stamp)
	if err != nil {
		t.Fatal(err)
	}
	carrier := func(tenantID, name string) (store.App, error) {
		id := ids.New(ids.BuildSource)
		return st.CreateApp(ctx, store.App{ID: id, DisplayName: name, TenantID: tenantID, Name: id, Type: store.TypeBuildSource,
			Image: "nginx:1", Port: 10000, Replicas: 1, Tier: "starter"})
	}

	mono, err := carrier(ws.ID, "mono")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := carrier(ws.ID, "mono"); !errors.Is(err, store.ErrBuildSourceNameTaken) || !errors.Is(err, store.ErrConflict) {
		t.Fatalf("a second mono: err = %v, want ErrBuildSourceNameTaken (a conflict)", err)
	}
	if _, err := carrier(other.ID, "mono"); err != nil {
		t.Fatalf("mono in another workspace: %v", err)
	}
	if _, err := st.CreateApp(ctx, store.App{DisplayName: "", TenantID: ws.ID, Name: "mono", Type: "web_service",
		Image: "nginx:1", Port: 10000, Replicas: 1, Tier: "starter"}); err != nil {
		t.Fatalf("a service named mono beside the build source: %v", err)
	}

	img, err := carrier(ws.ID, "img")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SetAppDisplayName(ctx, img.ID, "mono"); !errors.Is(err, store.ErrBuildSourceNameTaken) {
		t.Fatalf("renaming img onto mono: err = %v, want ErrBuildSourceNameTaken", err)
	}
	if err := st.SetAppDisplayName(ctx, mono.ID, "mono"); err != nil {
		t.Fatalf("keeping its own name: %v", err)
	}

	// Concurrent creates of one name: exactly one lands.
	var wg sync.WaitGroup
	errs := make([]error, 4)
	for i := range errs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, errs[i] = carrier(ws.ID, "race")
		}(i)
	}
	wg.Wait()
	landed := 0
	for _, err := range errs {
		switch {
		case err == nil:
			landed++
		case !errors.Is(err, store.ErrBuildSourceNameTaken):
			t.Fatalf("a concurrent create failed otherwise: %v", err)
		}
	}
	if landed != 1 {
		t.Fatalf("%d concurrent creates of one name landed, want exactly 1", landed)
	}
}
