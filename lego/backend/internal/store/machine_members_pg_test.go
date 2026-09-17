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
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	ids "github.com/bex-co/bex/lego/backend/internal/id"
)

// machine_members_pg_test.go is w5/m103's store half against REAL Postgres: a
// bound API key must authorize exactly as before while disappearing from every
// read that answers "who belongs to this workspace" — the member list, the seat
// count, and the admin count the last-admin rules consult.
//
// Additive: subjects are unique per run, and it never truncates.
func TestMachineBindingsAreNotMembers(t *testing.T) {
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		t.Skip("BEX_TEST_DB_URI not set")
	}
	if err := Migrate(uri); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, uri)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	st := NewPGStore(pool)

	run := uniqueMachineRun()
	human, second, client := "human-"+run, "second-"+run, "client-"+run

	tenant, err := st.CreateWorkspace(ctx, "machines-"+run, PlanPro, human)
	if err != nil {
		t.Fatalf("create workspace: %v", err)
	}
	if err := st.AddMember(ctx, second, tenant.ID, "admin"); err != nil {
		t.Fatalf("add second admin: %v", err)
	}

	membersBefore, err := st.CountTenantMembers(ctx, tenant.ID)
	if err != nil {
		t.Fatal(err)
	}
	adminsBefore, err := st.CountTenantAdmins(ctx, tenant.ID)
	if err != nil {
		t.Fatal(err)
	}

	// Bind an API key — the same call CreateAPIKey makes.
	if err := st.BindClient(ctx, client, tenant.ID); err != nil {
		t.Fatalf("bind client: %v", err)
	}

	// 1. The counts that charge and that guard must not move.
	if got, err := st.CountTenantMembers(ctx, tenant.ID); err != nil || got != membersBefore {
		t.Errorf("seat count = %d (err %v), want %d — a key must not consume a seat", got, err, membersBefore)
	}
	if got, err := st.CountTenantAdmins(ctx, tenant.ID); err != nil || got != adminsBefore {
		t.Errorf("admin count = %d (err %v), want %d", got, err, adminsBefore)
	}

	// 2. The key is not a member of the list the Team surface renders.
	list, err := st.ListTenantMembers(ctx, tenant.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range list {
		if m.Subject == client {
			t.Errorf("machine binding listed as a member: %+v", m)
		}
		if m.Kind != MemberKindUser {
			t.Errorf("member list carried a non-user row: %+v", m)
		}
	}
	if len(list) != membersBefore {
		t.Errorf("member list = %d rows, want %d", len(list), membersBefore)
	}

	// 3. But it still authorizes: the binding reads back, marked machine, and
	//    the subject still resolves to this workspace. Hiding it here would
	//    break every API key.
	bound, err := st.GetTenantMember(ctx, tenant.ID, client)
	if err != nil {
		t.Fatalf("bound key's row is unreadable — the key would stop working: %v", err)
	}
	if bound.Kind != MemberKindMachine || bound.Role != "developer" {
		t.Errorf("binding = %+v, want developer/machine", bound)
	}
	if member, err := st.IsMember(ctx, client, tenant.ID); err != nil || !member {
		t.Errorf("IsMember = %v (err %v) for a bound key, want true", member, err)
	}
	if got, err := st.TenantForIdentity(ctx, client); err != nil || got.ID != tenant.ID {
		t.Errorf("TenantForIdentity = %+v (err %v), want %s", got, err, tenant.ID)
	}

	// 4. Unbinding removes it, as before.
	if err := st.UnbindClient(ctx, client); err != nil {
		t.Fatalf("unbind: %v", err)
	}
	if _, err := st.GetTenantMember(ctx, tenant.ID, client); err == nil {
		t.Error("binding survived UnbindClient")
	}
}

// The backfill is what makes the migration true for keys bound BEFORE it: those
// rows defaulted to 'user', and only Hydra knows which subjects are clients.
func TestMarkMachineMembershipsReclassifiesLegacyBindings(t *testing.T) {
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		t.Skip("BEX_TEST_DB_URI not set")
	}
	if err := Migrate(uri); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, uri)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	st := NewPGStore(pool)

	run := uniqueMachineRun()
	human, legacyKey := "human-"+run, "legacy-client-"+run

	tenant, err := st.CreateWorkspace(ctx, "legacy-"+run, PlanPro, human)
	if err != nil {
		t.Fatalf("create workspace: %v", err)
	}
	// A pre-m103 binding: the row exists with the column's default.
	if _, err := pool.Exec(ctx,
		`INSERT INTO tenant_members (tenant_id, subject, role) VALUES ($1, $2, 'developer')`,
		tenant.ID, legacyKey); err != nil {
		t.Fatalf("seed legacy binding: %v", err)
	}
	before, err := st.CountTenantMembers(ctx, tenant.ID)
	if err != nil {
		t.Fatal(err)
	}
	if before != 2 {
		t.Fatalf("legacy binding does not look like a member (%d) — the premise changed", before)
	}

	n, err := st.MarkMachineMemberships(ctx, []string{legacyKey, "not-a-subject-" + run})
	if err != nil {
		t.Fatalf("backfill: %v", err)
	}
	if n != 1 {
		t.Errorf("reclassified %d rows, want exactly 1 (the unknown id must match nothing)", n)
	}
	if got, err := st.CountTenantMembers(ctx, tenant.ID); err != nil || got != 1 {
		t.Errorf("seat count after backfill = %d (err %v), want 1", got, err)
	}

	// Idempotent — a second start must be a no-op, not another round of writes.
	again, err := st.MarkMachineMemberships(ctx, []string{legacyKey})
	if err != nil || again != 0 {
		t.Errorf("second backfill changed %d rows (err %v), want 0", again, err)
	}
	// And it touches nothing it was not given: the workspace's human is still a
	// member afterwards. (The function trusts its input by design — the caller
	// passes ids straight from the key registry — so this pins that it acts on
	// exactly the subjects listed, not on a pattern it infers.)
	owner, err := st.GetTenantMember(ctx, tenant.ID, human)
	if err != nil || owner.Kind != MemberKindUser {
		t.Errorf("human member = %+v (err %v), want kind=user", owner, err)
	}
}

// uniqueMachineRun keeps this additive test's subjects distinct per run (the
// trailing xid chars are the counter/pid, so two runs in the same second still
// differ).
func uniqueMachineRun() string {
	full := ids.New(ids.Workspace)
	return full[len(full)-8:]
}
