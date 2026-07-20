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
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// newBillingTestStore is the shared harness for the billing outbox/exclusion
// integration tests: skip unless BEX_TEST_DB_URI is set, migrate, and truncate
// so each test starts clean. Same gating as TestPGStore.
func newBillingTestStore(t *testing.T) (*PGStore, context.Context) {
	t.Helper()
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		t.Skip("BEX_TEST_DB_URI not set")
	}
	ctx := context.Background()
	if err := Migrate(uri); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	pool, err := pgxpool.New(ctx, uri)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if _, err := pool.Exec(ctx, `TRUNCATE tenants CASCADE`); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `TRUNCATE audit_events`); err != nil {
		t.Fatal(err)
	}
	return NewPGStore(pool), ctx
}

func TestPGStoreBillingOutbox(t *testing.T) {
	s, ctx := newBillingTestStore(t)
	billable, err := s.CreateTenant(ctx, "billable", "hobby")
	if err != nil {
		t.Fatalf("create tenant: %v", err)
	}
	excluded, err := s.CreateTenant(ctx, "excluded", "hobby")
	if err != nil {
		t.Fatalf("create tenant: %v", err)
	}
	if _, err := s.SetTenantBillingExcluded(ctx, excluded.ID, true, "ops", time.Now().UTC()); err != nil {
		t.Fatalf("exclude tenant: %v", err)
	}

	now := time.Now().UTC().Truncate(time.Hour)
	sealed := now.Add(-72 * time.Hour) // older than the 48h seal horizon
	recent := now.Add(-1 * time.Hour)  // still inside the rewrite horizon
	belowFloor := now.Add(-40 * 24 * time.Hour)

	put := func(ws, svc, kind, tier string, window time.Time, qty int64) {
		t.Helper()
		if err := s.UpsertUsageHourly(ctx, HourlyRow{
			WorkspaceID: ws, ServiceID: svc, Kind: kind, Tier: tier,
			ResourceKind: ResourceKindService, WindowStart: window, Quantity: qty,
		}); err != nil {
			t.Fatalf("upsert usage: %v", err)
		}
	}
	put(billable.ID, "srv-1", UsageKindInstanceSeconds, "starter", sealed, 3600) // qualifies
	put(billable.ID, "srv-1", UsageKindInstanceSeconds, "starter", recent, 100)  // not sealed
	put(billable.ID, "srv-1", UsageKindInstanceSeconds, "starter", belowFloor, 5) // below floor
	put(excluded.ID, "srv-2", UsageKindInstanceSeconds, "starter", sealed, 999)   // excluded tenant

	floor := now.Add(-34 * 24 * time.Hour) // billing.BackfillHorizon; below `sealed`, above `belowFloor`
	sealBefore := now.Add(-48 * time.Hour)

	rows, err := s.SelectUnemittedUsage(ctx, floor, sealBefore, 100)
	if err != nil {
		t.Fatalf("select unemitted: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("selected %d rows, want 1 (only the sealed, above-floor, non-excluded one): %+v", len(rows), rows)
	}
	if rows[0].WorkspaceID != billable.ID || rows[0].Quantity != 3600 {
		t.Fatalf("selected wrong row: %+v", rows[0])
	}

	// Stamp, then re-select: exactly-once — the emitted row is gone.
	if err := s.MarkUsageEmitted(ctx, rows, now); err != nil {
		t.Fatalf("mark emitted: %v", err)
	}
	again, err := s.SelectUnemittedUsage(ctx, floor, sealBefore, 100)
	if err != nil {
		t.Fatalf("re-select: %v", err)
	}
	if len(again) != 0 {
		t.Fatalf("re-selected %d rows after stamping, want 0 (exactly-once)", len(again))
	}
	// Re-stamping the same rows is a harmless no-op (crash-recovery safety).
	if err := s.MarkUsageEmitted(ctx, rows, now); err != nil {
		t.Fatalf("re-mark emitted (idempotent): %v", err)
	}
}

func TestPGStoreBillingExclusionAudited(t *testing.T) {
	s, ctx := newBillingTestStore(t)
	ten, err := s.CreateTenant(ctx, "acme", "hobby")
	if err != nil {
		t.Fatalf("create tenant: %v", err)
	}
	at := time.Now().UTC()

	changed, err := s.SetTenantBillingExcluded(ctx, ten.ID, true, "admin@bex.co", at)
	if err != nil {
		t.Fatalf("exclude: %v", err)
	}
	if !changed {
		t.Fatal("first exclude reported changed=false, want true")
	}

	// The flag persisted, and an audit row records it with the value it was set to.
	var excluded bool
	if err := s.Pool.QueryRow(ctx, `SELECT billing_excluded FROM tenants WHERE id=$1`, ten.ID).Scan(&excluded); err != nil {
		t.Fatalf("read flag: %v", err)
	}
	if !excluded {
		t.Fatal("billing_excluded not persisted")
	}
	var verb string
	var to *bool
	if err := s.Pool.QueryRow(ctx,
		`SELECT verb, billing_excluded_to FROM audit_events WHERE workspace_id=$1 ORDER BY at DESC LIMIT 1`,
		ten.ID).Scan(&verb, &to); err != nil {
		t.Fatalf("read audit: %v", err)
	}
	if verb != "billing.SetExclusion" {
		t.Fatalf("audit verb = %q, want billing.SetExclusion", verb)
	}
	if to == nil || *to != true {
		t.Fatalf("audit billing_excluded_to = %v, want true", to)
	}

	// A no-op toggle changes nothing and writes no second audit row.
	changed, err = s.SetTenantBillingExcluded(ctx, ten.ID, true, "admin@bex.co", at)
	if err != nil {
		t.Fatalf("re-exclude: %v", err)
	}
	if changed {
		t.Fatal("no-op toggle reported changed=true, want false")
	}
	var n int
	if err := s.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM audit_events WHERE workspace_id=$1`, ten.ID).Scan(&n); err != nil {
		t.Fatalf("count audit: %v", err)
	}
	if n != 1 {
		t.Fatalf("audit rows = %d, want 1 (no-op wrote none)", n)
	}

	// Unknown workspace → ErrNotFound.
	if _, err := s.SetTenantBillingExcluded(ctx, "tea-nope", true, "admin", at); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown tenant err = %v, want ErrNotFound", err)
	}
}
