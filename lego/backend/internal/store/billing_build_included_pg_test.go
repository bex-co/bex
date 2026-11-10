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
	"testing"
	"time"
)

// w6/m308: the billing emitter sends only build minutes above the plan's
// included minutes, so it needs the month's earlier build minutes in a fixed
// order, and the export record keeps what was actually sent.
func TestPGStoreBuildSecondsBeforeAndBilledQuantity(t *testing.T) {
	s, ctx := newBillingTestStore(t)
	ws, err := s.CreateTenant(ctx, "builds", "hobby")
	if err != nil {
		t.Fatalf("create tenant: %v", err)
	}
	other, err := s.CreateTenant(ctx, "other", "hobby")
	if err != nil {
		t.Fatalf("create tenant: %v", err)
	}
	month := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	put := func(tenant, svc, kind string, window time.Time, qty int64) HourlyRow {
		t.Helper()
		r := HourlyRow{WorkspaceID: tenant, ServiceID: svc, Kind: kind, ResourceKind: ResourceKindService, WindowStart: window, Quantity: qty}
		if err := s.UpsertUsageHourly(ctx, r); err != nil {
			t.Fatalf("upsert usage: %v", err)
		}
		return r
	}
	put(ws.ID, "srv-a", UsageKindBuildSeconds, month.Add(-time.Hour), 999)              // last month
	put(ws.ID, "srv-a", UsageKindBuildSeconds, month.Add(2*time.Hour), 100)             // earlier this month
	put(ws.ID, "srv-a", UsageKindInstanceSeconds, month.Add(2*time.Hour), 3600)         // another meter
	put(other.ID, "srv-z", UsageKindBuildSeconds, month.Add(2*time.Hour), 500)          // another workspace
	sameHourA := put(ws.ID, "srv-a", UsageKindBuildSeconds, month.Add(5*time.Hour), 40) // same hour, sorts first
	sameHourB := put(ws.ID, "srv-b", UsageKindBuildSeconds, month.Add(5*time.Hour), 60)
	put(ws.ID, "srv-a", UsageKindBuildSeconds, month.Add(9*time.Hour), 7) // later

	for _, tc := range []struct {
		row  HourlyRow
		want int64
	}{{sameHourA, 100}, {sameHourB, 140}} {
		got, err := s.BuildSecondsBefore(ctx, tc.row)
		if err != nil {
			t.Fatalf("BuildSecondsBefore: %v", err)
		}
		if got != tc.want {
			t.Errorf("BuildSecondsBefore(%s) = %d, want %d: this month, this workspace, build minutes only, rows that sort before", tc.row.ServiceID, got, tc.want)
		}
	}

	// The first attempt's billed quantity is kept; a row sent whole keeps none.
	first, second := int64(25), int64(60)
	if err := s.MarkUsageAttempted(ctx, []UsageExportAttempt{
		{Row: sameHourB, TransactionID: "tx-b", EventName: "build_seconds", BilledQuantity: &first},
		{Row: sameHourA, TransactionID: "tx-a", EventName: "build_seconds"},
	}, month.Add(72*time.Hour)); err != nil {
		t.Fatalf("mark attempted: %v", err)
	}
	if err := s.MarkUsageAttempted(ctx, []UsageExportAttempt{
		{Row: sameHourB, TransactionID: "tx-b", EventName: "build_seconds", BilledQuantity: &second},
	}, month.Add(73*time.Hour)); err != nil {
		t.Fatalf("re-mark attempted: %v", err)
	}
	report, err := s.BillingExportReport(ctx, ws.ID, month, month.Add(24*time.Hour))
	if err != nil {
		t.Fatalf("export report: %v", err)
	}
	billed := map[string]*int64{}
	for _, r := range report.Rows {
		if r.Kind == UsageKindBuildSeconds && r.WindowStart.Equal(sameHourA.WindowStart) {
			billed[r.ServiceID] = r.BilledQuantity
		}
	}
	if b := billed["srv-b"]; b == nil || *b != first {
		t.Errorf("srv-b billed = %v, want the first attempt's %d", b, first)
	}
	if b, ok := billed["srv-a"]; !ok || b != nil {
		t.Errorf("srv-a billed = %v, want none recorded (sent whole)", b)
	}

	// The outbox read hands the frozen value back, so a retry resends it.
	pending, err := s.SelectUnemittedUsage(ctx, month.Add(-time.Hour), UsageSeal{Before: month.Add(24 * time.Hour)}, 100, false)
	if err != nil {
		t.Fatalf("select unemitted: %v", err)
	}
	var retried *HourlyRow
	for i := range pending {
		if pending[i].ServiceID == "srv-b" && pending[i].Kind == UsageKindBuildSeconds {
			retried = &pending[i]
		}
	}
	if retried == nil || retried.BilledQuantity == nil || *retried.BilledQuantity != first {
		t.Fatalf("pending srv-b = %+v, want its frozen billed quantity %d", retried, first)
	}

	// Settle compares Stripe with what was sent, not with what was used.
	if err := s.RecordUsageExportResult(ctx, []UsageExportAttempt{
		{Row: sameHourB, TransactionID: "tx-b", EventName: "build_seconds"},
		{Row: sameHourA, TransactionID: "tx-a", EventName: "build_seconds"},
	}, nil, month.Add(74*time.Hour)); err != nil {
		t.Fatalf("record export result: %v", err)
	}
	totals, err := s.ExportedUsageTotals(ctx, ws.ID, month, month.Add(24*time.Hour), month)
	if err != nil {
		t.Fatalf("exported totals: %v", err)
	}
	for _, total := range totals {
		if total.Kind == UsageKindBuildSeconds && total.Quantity != first+sameHourA.Quantity {
			t.Errorf("exported build_seconds = %d, want %d (srv-b's billed %d + srv-a sent whole %d)", total.Quantity, first+sameHourA.Quantity, first, sameHourA.Quantity)
		}
	}
}

// The build-minutes sweeper needs each workspace's month-to-date build minutes,
// its plan, and whether it can buy more; a workspace excluded from billing is
// never stopped.
func TestPGStoreBuildSecondsByWorkspace(t *testing.T) {
	s, ctx := newBillingTestStore(t)
	carded, err := s.CreateTenant(ctx, "carded", "pro")
	if err != nil {
		t.Fatal(err)
	}
	cardless, err := s.CreateTenant(ctx, "cardless", "hobby")
	if err != nil {
		t.Fatal(err)
	}
	excluded, err := s.CreateTenant(ctx, "excluded", "hobby")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.SetTenantBillingExcluded(ctx, excluded.ID, true, "ops", time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if err := s.UpsertBillingProviderMapping(ctx, BillingProviderMapping{WorkspaceID: carded.ID, CustomerID: "cus_carded", SubscriptionID: "sub_carded"}); err != nil {
		t.Fatal(err)
	}
	if err := s.SetPaymentMethodBound(ctx, carded.ID, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	month := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	put := func(ws, kind string, window time.Time, qty int64) {
		t.Helper()
		if err := s.UpsertUsageHourly(ctx, HourlyRow{WorkspaceID: ws, ServiceID: "srv-" + ws, Kind: kind, ResourceKind: ResourceKindService, WindowStart: window, Quantity: qty}); err != nil {
			t.Fatal(err)
		}
	}
	put(carded.ID, UsageKindBuildSeconds, month.Add(time.Hour), 600)
	put(cardless.ID, UsageKindBuildSeconds, month.Add(time.Hour), 100)
	put(cardless.ID, UsageKindBuildSeconds, month.Add(2*time.Hour), 50)
	put(cardless.ID, UsageKindBuildSeconds, month.Add(-time.Hour), 999)  // last month
	put(cardless.ID, UsageKindInstanceSeconds, month.Add(time.Hour), 77) // another meter
	put(excluded.ID, UsageKindBuildSeconds, month.Add(time.Hour), 5)

	usage, err := s.BuildSecondsByWorkspace(ctx, month, month.Add(24*time.Hour))
	if err != nil {
		t.Fatalf("BuildSecondsByWorkspace: %v", err)
	}
	got := map[string]BuildMinutesUsage{}
	for _, u := range usage {
		got[u.WorkspaceID] = u
	}
	if u := got[carded.ID]; u.Seconds != 600 || !u.CanPay {
		t.Errorf("carded = %+v, want 600s, able to pay", u)
	}
	if u := got[cardless.ID]; u.Seconds != 150 || u.CanPay {
		t.Errorf("cardless = %+v, want 150s this month, unable to pay", u)
	}
	if _, ok := got[excluded.ID]; ok {
		t.Error("a workspace excluded from billing must not be listed")
	}

	won, err := s.ClaimBuildMinutesNotice(ctx, cardless.ID, month, "warned")
	if err != nil || !won {
		t.Fatalf("first claim won=%v err=%v", won, err)
	}
	if won, err := s.ClaimBuildMinutesNotice(ctx, cardless.ID, month, "warned"); err != nil || won {
		t.Errorf("second claim won=%v err=%v, want the notice claimed once", won, err)
	}
}
