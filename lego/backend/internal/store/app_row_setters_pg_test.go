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
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bex-co/bex/lego/backend/internal/store"
	"github.com/bex-co/bex/lego/backend/internal/testenv"
)

// w9/092: the tier, replica and source setters write a row only when it does
// not already hold the value, and report which: a Blueprint apply retried
// before the projector copied its first write onto the App records each
// change once. A missing row is still not found, and a repeat leaves
// updated_at and the source revision as they were.
func TestPGAppRowSettersReportWhetherTheyChanged(t *testing.T) {
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
	ws, err := st.CreateWorkspace(ctx, "rows-"+stamp, store.PlanHobby, "rows-"+stamp)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.DeleteTenant(context.Background(), ws.ID) })
	app, err := st.CreateApp(ctx, store.App{TenantID: ws.ID, Name: "rows-" + stamp, Type: "web_service",
		Repo: "https://github.com/acme/app", Branch: "main", Port: 10000, Replicas: 1, Tier: "starter"})
	if err != nil {
		t.Fatal(err)
	}
	stateOf := func() (updatedAt time.Time, revision int64) {
		t.Helper()
		if err := pool.QueryRow(ctx, `SELECT updated_at, source_revision FROM apps WHERE id = $1`, app.ID).Scan(&updatedAt, &revision); err != nil {
			t.Fatal(err)
		}
		return updatedAt, revision
	}
	credential := ""
	for _, step := range []struct {
		name  string
		write func(id string) (bool, error)
	}{
		{"tier", func(id string) (bool, error) { return st.SetAppTier(ctx, id, "standard") }},
		{"replicas", func(id string) (bool, error) { return st.SetAppReplicas(ctx, id, 3) }},
		{"source", func(id string) (bool, error) {
			return st.SetAppSource(ctx, id, "https://github.com/acme/app", "", "dev", nil)
		}},
		{"registry credential", func(id string) (bool, error) {
			return st.SetAppSource(ctx, id, "https://github.com/acme/app", "", "dev", &credential)
		}},
	} {
		if changed, err := step.write(app.ID); err != nil || !changed {
			t.Fatalf("%s: first write = changed %t, err %v; want a change", step.name, changed, err)
		}
		updatedAt, revision := stateOf()
		if changed, err := step.write(app.ID); err != nil || changed {
			t.Fatalf("%s: repeated write = changed %t, err %v; want no change", step.name, changed, err)
		}
		if gotAt, gotRevision := stateOf(); !gotAt.Equal(updatedAt) || gotRevision != revision {
			t.Fatalf("%s: a repeat moved updated_at %v -> %v, source revision %d -> %d", step.name, updatedAt, gotAt, revision, gotRevision)
		}
		if _, err := step.write("srv-missing-" + stamp); !errors.Is(err, store.ErrNotFound) {
			t.Fatalf("%s: a missing row = %v, want not found", step.name, err)
		}
	}
	got, err := st.GetApp(ctx, app.ID)
	if err != nil || got.Tier != "standard" || got.Replicas != 3 || got.Branch != "dev" ||
		got.RegistryCredentialID == nil || *got.RegistryCredentialID != "" {
		t.Fatalf("row = %+v err=%v, want every first write kept", got, err)
	}
}
