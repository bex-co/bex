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

	"github.com/bex-co/bex/lego/backend/internal/testenv"
)

// billing_trust (ADR091 D10.1): payments count once per invoice; a failure
// locks indefinitely until THAT invoice is paid (then paid_at + 30 days);
// another invoice's payment, an older payment delivered late, or a failure
// of an already-paid invoice never moves the lock; a dispute holds T0 until
// it is won; and the record goes with its workspace.
func TestBillingTrustLedgerAndLock(t *testing.T) {
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		testenv.Skip(t, "BEX_TEST_DB_URI not set")
	}
	if err := Migrate(uri); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, uri)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	st := NewPGStore(pool)
	tenant, err := st.CreateTenant(ctx, "billing-trust", PlanPro)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM tenants WHERE id = $1`, tenant.ID) })
	inv := func(n string) string { return "in_" + tenant.ID + "_" + n }
	pay := func(invoice string, cents int64, at time.Time) BillingTrust {
		t.Helper()
		_, after, _, err := st.RecordBillingTrustPayment(ctx, BillingTrustPayment{InvoiceID: invoice, WorkspaceID: tenant.ID, AmountCents: cents, PaidAt: at})
		if err != nil {
			t.Fatal(err)
		}
		return after
	}
	fail := func(invoice string, at time.Time) (BillingTrust, bool) {
		t.Helper()
		_, after, applied, err := st.RecordBillingTrustFailure(ctx, tenant.ID, invoice, at)
		if err != nil {
			t.Fatal(err)
		}
		return after, applied
	}

	if got, err := st.GetBillingTrust(ctx, tenant.ID); err != nil || got != (BillingTrust{WorkspaceID: tenant.ID}) {
		t.Fatalf("fresh trust = %+v err=%v", got, err)
	}
	t0 := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	first := BillingTrustPayment{InvoiceID: inv("1"), WorkspaceID: tenant.ID, AmountCents: 1500, Cycle: true, PaidAt: t0}
	for i, wantCounted := range []bool{true, false} {
		_, after, counted, err := st.RecordBillingTrustPayment(ctx, first)
		if err != nil || counted != wantCounted || after.LifetimePaidCents != 1500 || after.PaidCycleInvoices != 1 {
			t.Fatalf("payment %d = %+v counted=%v err=%v", i, after, counted, err)
		}
	}
	// A failure of an invoice already paid is a late redelivery.
	if after, applied := fail(inv("1"), t0.Add(time.Hour)); applied || !after.LockedUntil.IsZero() {
		t.Fatalf("failure of a paid invoice = %+v applied=%v", after, applied)
	}

	// Out of order: B fails at t1; A, paid at t0 < t1, is delivered later.
	// Neither the other invoice nor the older payment unlocks.
	t1 := t0.Add(24 * time.Hour)
	locked, applied := fail(inv("B"), t1)
	if !applied || !locked.Lock().Equal(LockedIndefinitely) || locked.FailedInvoiceID != inv("B") || !locked.LastFailureAt.Equal(t1) {
		t.Fatalf("failure = %+v applied=%v", locked, applied)
	}
	if after := pay(inv("A"), 200, t0); !after.LockedUntil.Equal(LockedIndefinitely) || after.LifetimePaidCents != 1700 {
		t.Fatalf("an older payment of another invoice unlocked: %+v", after)
	}
	// A newer payment of another invoice does not unlock either.
	if after := pay(inv("C"), 100, t1.Add(time.Hour)); !after.LockedUntil.Equal(LockedIndefinitely) || after.FailedInvoiceID != inv("B") {
		t.Fatalf("another invoice's payment unlocked: %+v", after)
	}
	// An older failure delivered late keeps the newer failure's invoice.
	if after, applied := fail(inv("D"), t0.Add(2*time.Hour)); !applied || after.FailedInvoiceID != inv("B") {
		t.Fatalf("older failure = %+v applied=%v", after, applied)
	}
	// Paying B settles it: T0 until paid_at + 30 days.
	settle := t1.Add(48 * time.Hour)
	after := pay(inv("B"), 100, settle)
	if !after.LockedUntil.Equal(settle.Add(TrustLockAfterSettlement)) || after.FailedInvoiceID != "" {
		t.Fatalf("settling payment = %+v", after)
	}
	// A later payment does not move the settled hold; the stale failure of
	// the now-paid B changes nothing.
	if after = pay(inv("E"), 100, settle.Add(time.Hour)); !after.LockedUntil.Equal(settle.Add(TrustLockAfterSettlement)) {
		t.Fatalf("later payment = %+v", after)
	}
	if after, applied := fail(inv("B"), t1); applied || !after.LockedUntil.Equal(settle.Add(TrustLockAfterSettlement)) {
		t.Fatalf("stale failure = %+v applied=%v", after, applied)
	}

	// Disputes: their own hold, cleared only by a win of that dispute.
	dispute := func(id string) BillingTrust {
		t.Helper()
		_, after, err := st.RecordBillingTrustDispute(ctx, tenant.ID, id)
		if err != nil {
			t.Fatal(err)
		}
		return after
	}
	won := func(id string) BillingTrust {
		t.Helper()
		_, after, err := st.RecordBillingTrustDisputeWon(ctx, tenant.ID, id)
		if err != nil {
			t.Fatal(err)
		}
		return after
	}
	if after = dispute("dp_1"); !after.Lock().Equal(LockedIndefinitely) || after.DisputeID != "dp_1" {
		t.Fatalf("dispute = %+v", after)
	}
	if after = pay(inv("F"), 100, settle.Add(2*time.Hour)); !after.Lock().Equal(LockedIndefinitely) {
		t.Fatalf("a payment cleared a dispute: %+v", after)
	}
	if after = won("dp_other"); after.DisputeID != "dp_1" {
		t.Fatalf("another dispute's win cleared dp_1: %+v", after)
	}
	if after = won("dp_1"); after.DisputeID != "" || !after.Lock().Equal(settle.Add(TrustLockAfterSettlement)) {
		t.Fatalf("won dispute = %+v", after)
	}
	// A creation delivered after its win does not re-lock.
	if after = dispute("dp_1"); after.DisputeID != "" {
		t.Fatalf("late dispute creation re-locked: %+v", after)
	}
	// A lost dispute (no win) keeps the hold.
	if after = dispute("dp_2"); after.DisputeID != "dp_2" {
		t.Fatalf("second dispute = %+v", after)
	}
	if in, err := st.AgentSpendInputs(ctx, tenant.ID, settle); err != nil || in.Trust != after || in.Plan != PlanPro || in.OverrideMicroUSD != nil {
		t.Fatalf("spend inputs = %+v err=%v, want trust %+v", in, err, after)
	}
	if ws, found, err := st.BillingWorkspaceForCustomer(ctx, "cus_nobody", false); err != nil || found || ws != "" {
		t.Fatalf("unmapped customer = %q %v %v", ws, found, err)
	}

	if _, err := pool.Exec(ctx, `DELETE FROM tenants WHERE id = $1`, tenant.ID); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := st.RecordBillingTrustFailure(ctx, tenant.ID, inv("Z"), settle); !errors.Is(err, ErrWorkspaceGone) {
		t.Fatalf("failure for a deleted workspace = %v, want ErrWorkspaceGone", err)
	}
	if _, err := st.AgentSpendInputs(ctx, tenant.ID, settle); !errors.Is(err, ErrNotFound) {
		t.Fatalf("spend inputs for a deleted workspace = %v, want ErrNotFound", err)
	}
}
