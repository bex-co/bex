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

package store

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// key_grant_pg_test.go is w9/m496's store half against REAL Postgres: a key's
// grant lives on its machine row (migration 0300), NULL is All, an empty
// array is Clear all, and only a key's row can carry one.
func TestAPIKeyGrantLivesOnTheKeysRow(t *testing.T) {
	st := keyCreatorStore(t)
	ctx := context.Background()
	run := uniqueMachineRun()
	admin := "admin-" + run
	ws, err := st.CreateWorkspace(ctx, "keygrant-"+run, PlanPro, admin)
	if err != nil {
		t.Fatal(err)
	}
	key := "key-" + run
	if _, err := st.BindClientForCreator(ctx, key, ws.ID, admin); err != nil {
		t.Fatal(err)
	}
	if g, err := st.APIKeyGrant(ctx, key); err != nil || !g.All {
		t.Fatalf("a freshly bound key = %+v, %v; want All", g, err)
	}
	scoped, _ := core.ValidateKeyGrant(core.KeyGrant{Grants: []core.KeyGrantEntry{
		{Permission: "services", Level: core.KeyLevelRead}, {Permission: "deploys_run", Level: core.KeyLevelRun},
	}})
	if err := st.SetAPIKeyGrant(ctx, key, scoped); err != nil {
		t.Fatal(err)
	}
	if g, err := st.APIKeyGrant(ctx, key); err != nil || g.All || !slices.Equal(g.Entries(), scoped.Entries()) {
		t.Fatalf("read back %+v, %v; want %v", g, err, scoped.Entries())
	}
	if all, err := st.APIKeyGrantsOfTenant(ctx, ws.ID); err != nil || len(all) != 1 || !slices.Equal(all[key].Entries(), scoped.Entries()) {
		t.Fatalf("the workspace's grants = %+v, %v; want only %s's", all, err, key)
	}
	if err := st.SetAPIKeyGrant(ctx, key, core.KeyGrant{Grants: []core.KeyGrantEntry{}}); err != nil {
		t.Fatal(err)
	}
	if g, _ := st.APIKeyGrant(ctx, key); g.All || len(g.Grants) != 0 || g.Entries() == nil {
		t.Fatalf("Clear all read back as %+v", g)
	}
	if err := st.SetAPIKeyGrant(ctx, key, core.AllKeyGrant()); err != nil {
		t.Fatal(err)
	}
	if g, _ := st.APIKeyGrant(ctx, key); !g.All {
		t.Fatalf("All read back as %+v", g)
	}
	// A person has no grant to set, and an unbound key has no row.
	for _, subject := range []string{admin, "unbound-" + run} {
		if err := st.SetAPIKeyGrant(ctx, subject, scoped); !errors.Is(err, ErrNotFound) {
			t.Errorf("SetAPIKeyGrant(%s) = %v, want ErrNotFound", subject, err)
		}
	}
	// The column refuses a grant on a person's row even written directly.
	if _, err := st.Pool.Exec(ctx, `UPDATE tenant_members SET key_permissions = '{services:read}' WHERE subject = $1`, admin); err == nil {
		t.Error("a person's row took a grant")
	}
	// Re-binding a key (the mint's own retry path) starts it at All again.
	if err := st.SetAPIKeyGrant(ctx, key, scoped); err != nil {
		t.Fatal(err)
	}
	if _, err := st.BindClientForCreator(ctx, key, ws.ID, admin); err != nil {
		t.Fatal(err)
	}
	if g, _ := st.APIKeyGrant(ctx, key); !g.All {
		t.Errorf("a re-bound key kept its old grant: %+v", g)
	}
}
