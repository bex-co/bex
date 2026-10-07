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
	"maps"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bex-co/bex/lego/backend/internal/testenv"
)

// TestPGTenantContactSubjects (w5/126): one query answers each workspace's
// contact subject. A bound owner answers, even beside an older admin. An unbound
// workspace answers its oldest user admin: an older machine admin and an older
// viewer are passed over. A workspace with no admin, and an unknown id,
// answer nothing.
func TestPGTenantContactSubjects(t *testing.T) {
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		testenv.Skip(t, "BEX_TEST_DB_URI not set")
	}
	if err := Migrate(uri); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, uri)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close) // registered first, so it runs after the deletes
	st := NewPGStore(pool)
	run := uniqueMachineRun()
	create := func(t *testing.T, name string) Tenant {
		t.Helper()
		tenant, err := st.CreateTenant(ctx, name+run, PlanPro)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM tenants WHERE id = $1`, tenant.ID) })
		return tenant
	}

	// The bound workspace's owner is not its oldest admin (w5/m103).
	bound := create(t, "bound-")
	if err := st.AddMember(ctx, "founder-"+run, bound.ID, "admin"); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE tenants SET owner_identity_id = $2 WHERE id = $1`, bound.ID, "owner-"+run); err != nil {
		t.Fatal(err)
	}
	unbound := create(t, "unbound-")
	// BindClient writes machines as developers; an admin machine row is what
	// the kind filter, ListTenantMembers' own, passes over.
	if _, err := pool.Exec(ctx, `INSERT INTO tenant_members (tenant_id, subject, role, kind) VALUES ($1, $2, 'admin', 'machine')`, unbound.ID, "client-"+run); err != nil {
		t.Fatal(err)
	}
	if err := st.AddMember(ctx, "viewer-"+run, unbound.ID, "viewer"); err != nil {
		t.Fatal(err)
	}
	// The older admin sorts last by name, so only the creation order picks it.
	for _, subject := range []string{"z-old-admin-" + run, "a-new-admin-" + run} {
		if err := st.AddMember(ctx, subject, unbound.ID, "admin"); err != nil {
			t.Fatal(err)
		}
	}
	empty := create(t, "empty-")

	got, err := st.TenantContactSubjects(ctx, []string{bound.ID, unbound.ID, empty.ID, "tea-unknown-" + run})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{bound.ID: "owner-" + run, unbound.ID: "z-old-admin-" + run}
	if !maps.Equal(got, want) {
		t.Fatalf("contacts = %v, want %v", got, want)
	}
}
