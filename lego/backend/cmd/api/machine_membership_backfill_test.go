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

package main

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bex-co/bex/lego/backend/internal/apikeys"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
	"github.com/bex-co/bex/lego/backend/internal/testenv"
)

// listedKeys is the key registry as the backfill sees it: List is the only verb
// it calls, so the embedded nil store makes any other call fail the test loudly.
type listedKeys struct {
	apikeys.APIKeyStore
	keys  []apikeys.APIKey
	lists int
}

func (k *listedKeys) List(context.Context) ([]apikeys.APIKey, error) {
	k.lists++
	return k.keys, nil
}

// w7/064: with BEX_CP_DB_URI unset, main holds a nil *store.PGStore. Once Hydra
// held one API key, the backfill called MarkMachineMemberships on it and the
// process panicked before it listened. With the store off there is nothing to
// reclassify, so it must not even ask Hydra.
func TestMachineMembershipBackfillSkipsWithoutTheStore(t *testing.T) {
	var st *store.PGStore
	keys := &listedKeys{keys: []apikeys.APIKey{{ID: "client-1"}}}

	backfillMachineMemberships(context.Background(), st, keys)

	if keys.lists != 0 {
		t.Errorf("listed API keys %d time(s) with the store off, want 0", keys.lists)
	}
}

// The other side of the guard: with the store on, every listed key's legacy
// binding (a pre-w5/m103 row that defaulted to kind 'user') becomes a machine
// binding, and the workspace's human stays a member.
func TestMachineMembershipBackfillReclassifiesListedKeys(t *testing.T) {
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

	full := ids.New(ids.Workspace)
	run := full[len(full)-8:]
	human, legacyKey := "human-"+run, "legacy-client-"+run
	tenant, err := st.CreateWorkspace(ctx, "backfill-"+run, store.PlanPro, human)
	if err != nil {
		t.Fatalf("create workspace: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO tenant_members (tenant_id, subject, role) VALUES ($1, $2, 'developer')`,
		tenant.ID, legacyKey); err != nil {
		t.Fatalf("seed legacy binding: %v", err)
	}

	keys := &listedKeys{keys: []apikeys.APIKey{{ID: legacyKey}}}
	backfillMachineMemberships(ctx, st, keys)

	if keys.lists != 1 {
		t.Errorf("listed API keys %d time(s), want 1", keys.lists)
	}
	if m, err := st.GetTenantMember(ctx, tenant.ID, legacyKey); err != nil || m.Kind != store.MemberKindMachine {
		t.Errorf("legacy key binding = %+v (err %v), want kind=%s", m, err, store.MemberKindMachine)
	}
	if m, err := st.GetTenantMember(ctx, tenant.ID, human); err != nil || m.Kind != store.MemberKindUser {
		t.Errorf("human member = %+v (err %v), want kind=%s", m, err, store.MemberKindUser)
	}
}
