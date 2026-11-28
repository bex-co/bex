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
	"strings"
	"testing"
	"time"
)

// w4/m709 (docs/ADR023-usage-metering.md § Bandwidth served to a DDoS attack
// is not billed): an operator declares an attack window, and every egress hour
// a live window overlaps is left off the bill while usage_hourly keeps it.

// attackNow is the clock the windows' bounds are read against.
var attackNow = time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)

func putEgress(t *testing.T, s *PGStore, ctx context.Context, ws, svc, rk string, window time.Time, qty int64) HourlyRow {
	t.Helper()
	r := HourlyRow{WorkspaceID: ws, ServiceID: svc, Kind: UsageKindEgressBytes, ResourceKind: rk, WindowStart: window, Quantity: qty}
	if err := s.UpsertUsageHourly(ctx, r); err != nil {
		t.Fatalf("upsert usage: %v", err)
	}
	return r
}

func declare(t *testing.T, s *PGStore, ctx context.Context, ws string, start, end time.Time, services ...string) EgressAttackWindow {
	t.Helper()
	w, err := s.DeclareEgressAttackWindow(ctx, EgressAttackWindowDeclaration{
		WorkspaceID: ws, ServiceIDs: services, StartsAt: start, EndsAt: end, Actor: "ops@bex.co", Reason: "Hetzner DDoS notice",
	}, attackNow)
	if err != nil {
		t.Fatalf("declare: %v", err)
	}
	return w
}

func TestPGStoreEgressAttackWindowLifecycle(t *testing.T) {
	s, ctx := newBillingTestStore(t)
	ws, err := s.CreateTenant(ctx, "flooded", "pro")
	if err != nil {
		t.Fatal(err)
	}
	other, err := s.CreateTenant(ctx, "neighbour", "pro")
	if err != nil {
		t.Fatal(err)
	}
	day := time.Date(2026, 11, 18, 0, 0, 0, 0, time.UTC)
	putEgress(t, s, ctx, ws.ID, "srv-web", ResourceKindService, day, 1)
	putEgress(t, s, ctx, ws.ID, "dpg-db", ResourceKindPostgres, day, 1)
	putEgress(t, s, ctx, other.ID, "srv-theirs", ResourceKindService, day, 1)

	valid := EgressAttackWindowDeclaration{WorkspaceID: ws.ID, StartsAt: day.Add(3 * time.Hour), EndsAt: day.Add(9 * time.Hour), Actor: "ops@bex.co", Reason: "router request spike"}
	for name, tc := range map[string]struct {
		mutate func(*EgressAttackWindowDeclaration)
		want   error
		says   string
	}{
		"start after end":    {func(d *EgressAttackWindowDeclaration) { d.StartsAt, d.EndsAt = d.EndsAt, d.StartsAt }, ErrInvalid, "startsAt must be before endsAt"},
		"empty span":         {func(d *EgressAttackWindowDeclaration) { d.EndsAt = d.StartsAt }, ErrInvalid, "startsAt must be before endsAt"},
		"longer than 7 days": {func(d *EgressAttackWindowDeclaration) { d.EndsAt = d.StartsAt.Add(8 * 24 * time.Hour) }, ErrInvalid, "at most"},
		"ends far ahead":     {func(d *EgressAttackWindowDeclaration) { d.EndsAt = attackNow.Add(25 * time.Hour) }, ErrInvalid, "from now"},
		"starts too long ago": {func(d *EgressAttackWindowDeclaration) {
			d.StartsAt, d.EndsAt = attackNow.Add(-36*24*time.Hour), attackNow.Add(-35*24*time.Hour)
		}, ErrInvalid, "ago"},
		"no actor":             {func(d *EgressAttackWindowDeclaration) { d.Actor = " " }, ErrInvalid, "actor and reason are required"},
		"no reason":            {func(d *EgressAttackWindowDeclaration) { d.Reason = "" }, ErrInvalid, "actor and reason are required"},
		"unknown workspace":    {func(d *EgressAttackWindowDeclaration) { d.WorkspaceID = "tea-missing" }, ErrNotFound, "tea-missing"},
		"another's service":    {func(d *EgressAttackWindowDeclaration) { d.ServiceIDs = []string{"srv-web", "srv-theirs"} }, ErrInvalid, "srv-theirs is not a service"},
		"an unmetered service": {func(d *EgressAttackWindowDeclaration) { d.ServiceIDs = []string{"srv-nope"} }, ErrInvalid, "srv-nope is not a service"},
	} {
		d := valid
		tc.mutate(&d)
		_, err := s.DeclareEgressAttackWindow(ctx, d, attackNow)
		if !errors.Is(err, tc.want) || !strings.Contains(err.Error(), tc.says) {
			t.Errorf("%s: err = %v, want %v saying %q", name, err, tc.want, tc.says)
		}
	}

	scoped := declare(t, s, ctx, ws.ID, day.Add(3*time.Hour+30*time.Minute), day.Add(9*time.Hour), " srv-web", "dpg-db", "srv-web")
	if strings.Join(scoped.ServiceIDs, ",") != "dpg-db,srv-web" || !strings.HasPrefix(scoped.ID, "eaw-") || scoped.WithdrawnAt != nil {
		t.Fatalf("declared window = %+v, want deduplicated sorted services and an eaw- id", scoped)
	}
	if len(scoped.Changes) != 1 || scoped.Changes[0].Action != "declare" || scoped.Changes[0].Actor != "ops@bex.co" {
		t.Fatalf("changes = %+v, want the declaration with its actor", scoped.Changes)
	}

	// An extension only widens.
	if _, err := s.ExtendEgressAttackWindow(ctx, ws.ID, scoped.ID, day.Add(4*time.Hour), time.Time{}, "ops", "narrow", attackNow); !errors.Is(err, ErrInvalid) {
		t.Errorf("narrowing extension err = %v, want ErrInvalid", err)
	}
	if _, err := s.ExtendEgressAttackWindow(ctx, ws.ID, scoped.ID, time.Time{}, time.Time{}, "ops", "same", attackNow); !errors.Is(err, ErrInvalid) {
		t.Errorf("empty extension err = %v, want ErrInvalid", err)
	}
	if _, err := s.ExtendEgressAttackWindow(ctx, ws.ID, scoped.ID, time.Time{}, day.Add(9*24*time.Hour), "ops", "too long", attackNow); !errors.Is(err, ErrInvalid) {
		t.Errorf("extension past the span bound err = %v, want ErrInvalid", err)
	}
	if _, err := s.ExtendEgressAttackWindow(ctx, other.ID, scoped.ID, time.Time{}, day.Add(10*time.Hour), "ops", "wrong workspace", attackNow); !errors.Is(err, ErrNotFound) {
		t.Errorf("extension through another workspace err = %v, want ErrNotFound", err)
	}
	extended, err := s.ExtendEgressAttackWindow(ctx, ws.ID, scoped.ID, time.Time{}, day.Add(14*time.Hour), "oncall", "attack still running", attackNow)
	if err != nil {
		t.Fatalf("extend: %v", err)
	}
	if !extended.EndsAt.Equal(day.Add(14*time.Hour)) || !extended.StartsAt.Equal(scoped.StartsAt) || len(extended.Changes) != 2 || extended.Changes[1].Reason != "attack still running" {
		t.Fatalf("extended = %+v, want the new end, the old start and the extension recorded", extended)
	}

	whole := declare(t, s, ctx, ws.ID, day, day.Add(time.Hour))
	withdrawn, changed, err := s.WithdrawEgressAttackWindow(ctx, ws.ID, whole.ID, "ops", "customer's own load test", attackNow)
	if err != nil || !changed || withdrawn.WithdrawnAt == nil || withdrawn.WithdrawnBy != "ops" || withdrawn.WithdrawReason != "customer's own load test" {
		t.Fatalf("withdraw = %+v, %v, %v", withdrawn, changed, err)
	}
	if _, changed, err := s.WithdrawEgressAttackWindow(ctx, ws.ID, whole.ID, "ops", "again", attackNow); err != nil || changed {
		t.Errorf("second withdrawal changed=%v err=%v, want a no-op", changed, err)
	}
	if _, err := s.ExtendEgressAttackWindow(ctx, ws.ID, whole.ID, time.Time{}, day.Add(2*time.Hour), "ops", "revive", attackNow); !errors.Is(err, ErrConflict) {
		t.Errorf("extending a withdrawn window err = %v, want ErrConflict", err)
	}

	list, err := s.ListEgressAttackWindows(ctx, ws.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 || list[0].ID != scoped.ID || list[1].ID != whole.ID || len(list[1].Changes) != 2 || list[1].Changes[1].Action != "withdraw" {
		t.Fatalf("list = %+v, want both windows newest first, each with its actor-and-reason trail", list)
	}
	if theirs, err := s.ListEgressAttackWindows(ctx, other.ID); err != nil || len(theirs) != 0 {
		t.Errorf("another workspace's list = %+v, %v; want empty", theirs, err)
	}
	if _, err := s.ListEgressAttackWindows(ctx, "tea-missing"); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown workspace list err = %v, want ErrNotFound", err)
	}
}

// The overlap read returns exactly the live windows touching an hour of the
// range for the workspace and the service.
func TestPGStoreEgressAttackWindowsOverlapping(t *testing.T) {
	s, ctx := newBillingTestStore(t)
	ws, err := s.CreateTenant(ctx, "flooded", "pro")
	if err != nil {
		t.Fatal(err)
	}
	other, err := s.CreateTenant(ctx, "neighbour", "pro")
	if err != nil {
		t.Fatal(err)
	}
	day := time.Date(2026, 11, 18, 0, 0, 0, 0, time.UTC)
	putEgress(t, s, ctx, ws.ID, "srv-a", ResourceKindService, day, 1)
	putEgress(t, s, ctx, ws.ID, "srv-b", ResourceKindService, day, 1)

	all := declare(t, s, ctx, ws.ID, day.Add(2*time.Hour+50*time.Minute), day.Add(3*time.Hour+10*time.Minute))
	onlyB := declare(t, s, ctx, ws.ID, day.Add(3*time.Hour), day.Add(5*time.Hour), "srv-b")
	gone := declare(t, s, ctx, ws.ID, day.Add(3*time.Hour), day.Add(4*time.Hour))
	if _, _, err := s.WithdrawEgressAttackWindow(ctx, ws.ID, gone.ID, "ops", "mistake", attackNow); err != nil {
		t.Fatal(err)
	}
	declare(t, s, ctx, other.ID, day, day.Add(12*time.Hour))

	ids := func(serviceID string, from, to time.Time) string {
		t.Helper()
		windows, err := s.EgressAttackWindowsOverlapping(ctx, ws.ID, serviceID, from, to)
		if err != nil {
			t.Fatal(err)
		}
		var out []string
		for _, w := range windows {
			out = append(out, w.ID)
		}
		return strings.Join(out, ",")
	}
	hour := func(h int) (time.Time, time.Time) {
		return day.Add(time.Duration(h) * time.Hour), day.Add(time.Duration(h+1) * time.Hour)
	}
	for _, tc := range []struct {
		service string
		hour    int
		want    string
	}{
		{"srv-a", 1, ""},
		{"srv-a", 2, all.ID},                  // the window starts inside the hour
		{"srv-a", 3, all.ID},                  // and ends inside the next
		{"srv-b", 3, all.ID + "," + onlyB.ID}, // both cover srv-b
		{"srv-a", 4, ""},                      // onlyB is srv-b's alone; the withdrawn one counts for none
		{"srv-b", 4, onlyB.ID},
		{"srv-b", 5, ""}, // onlyB ends on the hour
	} {
		from, to := hour(tc.hour)
		if got := ids(tc.service, from, to); got != tc.want {
			t.Errorf("%s hour %d: windows %q, want %q", tc.service, tc.hour, got, tc.want)
		}
	}
	// A range ending inside an hour reaches the window that starts later in it.
	if got := ids("srv-a", day, day.Add(2*time.Hour+10*time.Minute)); got != all.ID {
		t.Errorf("range ending inside hour 2: windows %q, want %q", got, all.ID)
	}

	spans := EgressAttackHourSpans([]EgressAttackWindow{all, onlyB}, day, day.Add(24*time.Hour))
	if len(spans) != 1 || !spans[0].Start.Equal(day.Add(2*time.Hour)) || !spans[0].End.Equal(day.Add(5*time.Hour)) {
		t.Errorf("hour spans = %+v, want one merged span 02:00-05:00", spans)
	}
	clipped := EgressAttackHourSpans([]EgressAttackWindow{onlyB}, day.Add(3*time.Hour+20*time.Minute), day.Add(4*time.Hour+30*time.Minute))
	if len(clipped) != 1 || !clipped[0].Start.Equal(day.Add(3*time.Hour+20*time.Minute)) || !clipped[0].End.Equal(day.Add(4*time.Hour+30*time.Minute)) {
		t.Errorf("clipped spans = %+v, want the range itself", clipped)
	}
}

// Every read that bills or counts against the allowance leaves a live
// window's hours out; usage_hourly keeps them, an hour outside every window
// and a withdrawn window's hours bill as before.
func TestPGStoreAttackHoursLeaveEveryBillingRead(t *testing.T) {
	s, ctx := newBillingTestStore(t)
	ws, err := s.CreateTenant(ctx, "flooded", "hobby")
	if err != nil {
		t.Fatal(err)
	}
	const gib = int64(1 << 30)
	month := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	before := putEgress(t, s, ctx, ws.ID, "srv-web", ResourceKindService, month.Add(2*time.Hour), 1*gib)
	attackWeb := putEgress(t, s, ctx, ws.ID, "srv-web", ResourceKindService, month.Add(3*time.Hour), 40*gib)
	attackDB := putEgress(t, s, ctx, ws.ID, "dpg-db", ResourceKindPostgres, month.Add(3*time.Hour), 7*gib)
	attackKV := putEgress(t, s, ctx, ws.ID, "red-kv", ResourceKindKeyValue, month.Add(4*time.Hour), 5*gib)
	bystander := putEgress(t, s, ctx, ws.ID, "srv-api", ResourceKindService, month.Add(3*time.Hour), 2*gib)
	after := putEgress(t, s, ctx, ws.ID, "srv-web", ResourceKindService, month.Add(6*time.Hour), 3*gib)
	withdrawnHour := putEgress(t, s, ctx, ws.ID, "srv-web", ResourceKindService, month.Add(8*time.Hour), 4*gib)
	lastMonth := putEgress(t, s, ctx, ws.ID, "srv-web", ResourceKindService, month.Add(-time.Hour), 9*gib)
	if err := s.UpsertUsageHourly(ctx, HourlyRow{WorkspaceID: ws.ID, ServiceID: "srv-web", Kind: UsageKindBuildSeconds, ResourceKind: ResourceKindService, WindowStart: month.Add(3 * time.Hour), Quantity: 60}); err != nil {
		t.Fatal(err)
	}
	now := month.Add(10 * 24 * time.Hour)
	// The window starts and ends mid-hour, narrowed to the flooded services:
	// hours 03:00 and 04:00 whole, srv-api untouched. A second window crosses
	// the month turn into October's last hour. A third is withdrawn.
	if _, err := s.DeclareEgressAttackWindow(ctx, EgressAttackWindowDeclaration{
		WorkspaceID: ws.ID, ServiceIDs: []string{"srv-web", "dpg-db", "red-kv"},
		StartsAt: month.Add(3*time.Hour + 15*time.Minute), EndsAt: month.Add(4*time.Hour + 5*time.Minute),
		Actor: "ops", Reason: "flood",
	}, now); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DeclareEgressAttackWindow(ctx, EgressAttackWindowDeclaration{
		WorkspaceID: ws.ID, StartsAt: month.Add(-30 * time.Minute), EndsAt: month.Add(time.Minute), Actor: "ops", Reason: "flood across the month turn",
	}, now); err != nil {
		t.Fatal(err)
	}
	// An overlapping second window over the flood hour changes nothing: an
	// hour leaves the bill once.
	if _, err := s.DeclareEgressAttackWindow(ctx, EgressAttackWindowDeclaration{
		WorkspaceID: ws.ID, ServiceIDs: []string{"srv-web"},
		StartsAt: month.Add(3 * time.Hour), EndsAt: month.Add(3*time.Hour + 30*time.Minute),
		Actor: "ops", Reason: "same flood, second report",
	}, now); err != nil {
		t.Fatal(err)
	}
	mistaken, err := s.DeclareEgressAttackWindow(ctx, EgressAttackWindowDeclaration{
		WorkspaceID: ws.ID, StartsAt: month.Add(8 * time.Hour), EndsAt: month.Add(9 * time.Hour), Actor: "ops", Reason: "load test, not an attack",
	}, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.WithdrawEgressAttackWindow(ctx, ws.ID, mistaken.ID, "ops", "customer's load test", now); err != nil {
		t.Fatal(err)
	}

	// The outbox flags exactly the attack hours.
	pending, err := s.SelectWorkspaceUnemittedUsage(ctx, ws.ID, month.Add(-48*time.Hour), UsageSeal{Before: now}, 100, false)
	if err != nil {
		t.Fatal(err)
	}
	attackRows := map[string]bool{}
	for _, r := range pending {
		attackRows[r.Kind+":"+r.ServiceID+"@"+r.WindowStart.UTC().Format("01-02T15")] = r.AttackHour
	}
	key := func(r HourlyRow) string {
		return r.Kind + ":" + r.ServiceID + "@" + r.WindowStart.UTC().Format("01-02T15")
	}
	for _, r := range []HourlyRow{attackWeb, attackDB, attackKV, lastMonth} {
		if !attackRows[key(r)] {
			t.Errorf("outbox row %s: AttackHour false, want true", key(r))
		}
	}
	for _, r := range []HourlyRow{before, bystander, after, withdrawnHour} {
		if v, ok := attackRows[key(r)]; !ok || v {
			t.Errorf("outbox row %s: AttackHour %v (present %v), want false", key(r), v, ok)
		}
	}

	if v, ok := attackRows["build_seconds:srv-web@11-01T03"]; !ok || v {
		t.Errorf("build row in the attack hour: AttackHour %v (present %v), want false: only egress is touched", v, ok)
	}

	// The allowance: an hour after the attack counts only the hours outside it.
	got, err := s.UsageBefore(ctx, after)
	if err != nil {
		t.Fatal(err)
	}
	if want := before.Quantity + bystander.Quantity; got != want {
		t.Errorf("UsageBefore(after) = %d, want %d: the attack's bytes use none of the allowance", got, want)
	}
	if got, err := s.UsageBefore(ctx, withdrawnHour); err != nil || got != before.Quantity+bystander.Quantity+after.Quantity {
		t.Errorf("UsageBefore(withdrawn hour) = %d, %v", got, err)
	}

	// The spin-down sum.
	wantMonth := before.Quantity + bystander.Quantity + after.Quantity + withdrawnHour.Quantity
	egress, err := s.EgressBytesByWorkspace(ctx, month, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(egress) != 1 || egress[0].Quantity != wantMonth {
		t.Errorf("EgressBytesByWorkspace = %+v, want %d bytes", egress, wantMonth)
	}

	// The usage read.
	readEgress := func() map[string]int64 {
		t.Helper()
		rows, err := s.UsageMonthToDate(ctx, ws.ID, now)
		if err != nil {
			t.Fatal(err)
		}
		out := map[string]int64{}
		for _, r := range rows {
			out[r.Kind+":"+r.ServiceID] += r.Total
			if r.Kind == UsageKindEgressBytes && r.Unbilled {
				t.Errorf("an attack hour came back as an unbilled line for %s", r.ServiceID)
			}
		}
		return out
	}
	read := readEgress()
	if read["egress_bytes:srv-web"] != before.Quantity+after.Quantity+withdrawnHour.Quantity || read["egress_bytes:srv-api"] != bystander.Quantity {
		t.Errorf("usage read = %v, want srv-web without its attack hour and srv-api whole", read)
	}
	if _, ok := read["egress_bytes:dpg-db"]; ok {
		t.Errorf("usage read lists dpg-db's attack-only egress: %v", read)
	}
	if read["build_seconds:srv-web"] != 60 {
		t.Errorf("usage read build seconds = %d, want 60: only egress is touched", read["build_seconds:srv-web"])
	}

	// The billing-export report still shows the served bytes, marked.
	report, err := s.BillingExportReport(ctx, ws.ID, month, now)
	if err != nil {
		t.Fatal(err)
	}
	marked := 0
	for _, r := range report.Rows {
		if r.AttackHour {
			marked++
			if r.Quantity == 0 {
				t.Errorf("report row %s lost its served bytes", r.ServiceID)
			}
		}
	}
	if marked != 3 {
		t.Errorf("report marks %d attack rows this month, want 3", marked)
	}

	// Compaction keeps them out of the month's total.
	if _, err := s.CompactUsage(ctx, month.AddDate(0, 1, 0)); err != nil {
		t.Fatal(err)
	}
	if after := readEgress(); after["egress_bytes:srv-web"] != read["egress_bytes:srv-web"] || after["egress_bytes:srv-api"] != read["egress_bytes:srv-api"] || after["egress_bytes:dpg-db"] != 0 {
		t.Errorf("usage read after compaction = %v, want %v", after, read)
	}
	if egress, err := s.EgressBytesByWorkspace(ctx, month, now); err != nil || len(egress) != 1 || egress[0].Quantity != wantMonth {
		t.Errorf("EgressBytesByWorkspace after compaction = %+v, %v; want %d", egress, err, wantMonth)
	}
}

// A workspace whose month's bandwidth is all inside a window has none to be
// spun down for.
func TestPGStoreAttackOnlyWorkspaceLeavesTheSpinDownSum(t *testing.T) {
	s, ctx := newBillingTestStore(t)
	ws, err := s.CreateTenant(ctx, "free-flooded", "hobby")
	if err != nil {
		t.Fatal(err)
	}
	month := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	putEgress(t, s, ctx, ws.ID, "srv-web", ResourceKindService, month.Add(5*time.Hour), 50<<30)
	now := month.Add(5 * 24 * time.Hour)
	if egress, err := s.EgressBytesByWorkspace(ctx, month, now); err != nil || len(egress) != 1 {
		t.Fatalf("before the window: %+v, %v", egress, err)
	}
	if _, err := s.DeclareEgressAttackWindow(ctx, EgressAttackWindowDeclaration{
		WorkspaceID: ws.ID, StartsAt: month.Add(5 * time.Hour), EndsAt: month.Add(6 * time.Hour), Actor: "ops", Reason: "flood",
	}, now); err != nil {
		t.Fatal(err)
	}
	if egress, err := s.EgressBytesByWorkspace(ctx, month, now); err != nil || len(egress) != 0 {
		t.Errorf("after the window: %+v, %v; want the workspace gone from the sum", egress, err)
	}
}

func TestPGStoreEgressAttackCreditLedger(t *testing.T) {
	s, ctx := newBillingTestStore(t)
	ws, err := s.CreateTenant(ctx, "flooded", "pro")
	if err != nil {
		t.Fatal(err)
	}
	comped, err := s.CreateTenant(ctx, "comped", "pro")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.SetBillingException(ctx, comped.ID, BillingComped, true, "ops", "partner", attackNow); err != nil {
		t.Fatal(err)
	}
	const gib = int64(1 << 30)
	month := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	attack := putEgress(t, s, ctx, ws.ID, "srv-web", ResourceKindService, month.Add(time.Hour), 40*gib)
	later := putEgress(t, s, ctx, ws.ID, "srv-web", ResourceKindService, month.Add(2*time.Hour), 3*gib)
	putEgress(t, s, ctx, ws.ID, "srv-web", ResourceKindService, month.Add(3*time.Hour), 2*gib) // still pending
	theirs := putEgress(t, s, ctx, comped.ID, "srv-c", ResourceKindService, month.Add(time.Hour), 9*gib)
	billed := 15 * gib
	exported := []UsageExportAttempt{
		{Row: attack, TransactionID: "tx-attack", EventName: "egress_gib", BilledQuantity: &billed},
		{Row: later, TransactionID: "tx-later", EventName: "egress_gib"},
		{Row: theirs, TransactionID: "tx-theirs", EventName: "egress_gib"},
	}
	if err := s.MarkUsageAttempted(ctx, exported, attackNow); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordUsageExportResult(ctx, exported, nil, attackNow); err != nil {
		t.Fatal(err)
	}
	if months, err := s.EgressAttackCreditMonths(ctx); err != nil || len(months) != 0 {
		t.Fatalf("credit months with no window = %+v, %v", months, err)
	}
	declare(t, s, ctx, ws.ID, month.Add(time.Hour), month.Add(2*time.Hour))
	declare(t, s, ctx, comped.ID, month.Add(time.Hour), month.Add(2*time.Hour))

	months, err := s.EgressAttackCreditMonths(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(months) != 1 || months[0].WorkspaceID != ws.ID || !months[0].Month.Equal(month) {
		t.Fatalf("credit months = %+v, want only the billed workspace's November", months)
	}
	ledger, err := s.EgressAttackLedger(ctx, ws.ID, month.Add(9*24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if ledger.NextSeq != 1 || ledger.CreditedBytes != 0 || ledger.Compacted || len(ledger.Rows) != 3 {
		t.Fatalf("ledger = %+v", ledger)
	}
	if r := ledger.Rows[0]; !r.AttackHour || !r.Emitted || r.Billed != billed || r.Quantity != 40*gib {
		t.Errorf("attack row = %+v, want the frozen 15 GiB billed", r)
	}
	if r := ledger.Rows[1]; r.AttackHour || !r.Emitted || r.Billed != 3*gib {
		t.Errorf("later row = %+v, want its whole quantity billed", r)
	}
	if r := ledger.Rows[2]; r.Emitted {
		t.Errorf("pending row = %+v, want not emitted", r)
	}

	credit := EgressAttackCredit{WorkspaceID: ws.ID, Month: month, Seq: 1, Bytes: 15 * gib, AmountCents: 23}
	if won, err := s.RecordEgressAttackCredit(ctx, credit); err != nil || !won {
		t.Fatalf("record = %v, %v", won, err)
	}
	if won, err := s.RecordEgressAttackCredit(ctx, credit); err != nil || won {
		t.Errorf("a second replica's record of the same seq won=%v err=%v, want nothing inserted", won, err)
	}
	ledger, err = s.EgressAttackLedger(ctx, ws.ID, month)
	if err != nil || ledger.NextSeq != 2 || ledger.CreditedBytes != 15*gib || ledger.CreditedCents != 23 {
		t.Fatalf("ledger after the credit = %+v, %v", ledger, err)
	}
	unsent, err := s.UnsentEgressAttackCredits(ctx, 10)
	if err != nil || len(unsent) != 1 || unsent[0].Seq != 1 || !unsent[0].Month.Equal(month) {
		t.Fatalf("unsent = %+v, %v", unsent, err)
	}
	if err := s.MarkEgressAttackCreditSent(ctx, unsent[0], "ii_1", attackNow); err != nil {
		t.Fatal(err)
	}
	if err := s.MarkEgressAttackCreditSent(ctx, unsent[0], "ii_other", attackNow); err != nil {
		t.Fatal(err)
	}
	if unsent, err := s.UnsentEgressAttackCredits(ctx, 10); err != nil || len(unsent) != 0 {
		t.Errorf("unsent after marking = %+v, %v", unsent, err)
	}
	var item string
	if err := s.Pool.QueryRow(context.Background(), `SELECT invoice_item_id FROM egress_attack_credits WHERE workspace_id = $1`, ws.ID).Scan(&item); err != nil || item != "ii_1" {
		t.Errorf("invoice item = %q, %v; want the first kept", item, err)
	}
}
