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
	"testing"
	"time"
)

// w4/m713 t002: from StorageCapacityBilledFrom a Postgres bills its storage
// capacity and its used bytes are a reading shown at $0, and a Key Value's used
// bytes are shown at $0 too; before it, the used bytes bill and the capacity
// is shown. Each hour bills on exactly one kind, on every read that prices,
// exports, settles or compacts it, and a compacted month keeps both readings
// and its billed total.
func TestStorageCapacitySwitchPG(t *testing.T) {
	s, ws := sandboxCPUTestStore(t, "storage-capacity-switch")
	ctx := context.Background()
	cutover := time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)
	s.StorageCapacityBilledFrom = cutover
	before, after := cutover.Add(-time.Hour), cutover // the last hour on used bytes, the first on capacity

	type meter struct {
		resourceKind, id, kind string
		quantity               int64
	}
	// A paid 10 GB Postgres with about 2 GB used, and a Key Value with 1 GB
	// used, each present the whole hour on both sides of the switch.
	meters := []meter{
		{ResourceKindPostgres, "dpg-paid", UsageKindStorageCapacityGBSeconds, 36000},
		{ResourceKindPostgres, "dpg-paid", UsageKindStorageGBSeconds, 7200},
		{ResourceKindKeyValue, "red-cache", UsageKindStorageGBSeconds, 3600},
	}
	for _, window := range []time.Time{before, after} {
		for _, m := range meters {
			if err := s.UpsertUsageHourly(ctx, HourlyRow{
				WorkspaceID: ws, ServiceID: m.id, ResourceKind: m.resourceKind, Kind: m.kind,
				Tier: "basic-1gb", WindowStart: window, Quantity: m.quantity,
			}); err != nil {
				t.Fatal(err)
			}
		}
	}
	type line struct {
		id, kind string
		unbilled bool
	}
	monthOf := func(st *PGStore, now time.Time) map[line]int64 {
		t.Helper()
		rows, err := st.UsageMonthToDate(ctx, ws, now)
		if err != nil {
			t.Fatal(err)
		}
		got := map[line]int64{}
		for _, r := range rows {
			got[line{r.ServiceID, r.Kind, r.Unbilled}] += r.Total
		}
		return got
	}
	november := map[line]int64{
		{"dpg-paid", UsageKindStorageGBSeconds, false}:        7200, // billed, as it was invoiced
		{"dpg-paid", UsageKindStorageCapacityGBSeconds, true}: 36000,
		{"red-cache", UsageKindStorageGBSeconds, false}:       3600,
	}
	december := map[line]int64{
		{"dpg-paid", UsageKindStorageCapacityGBSeconds, false}: 36000, // Render's basis
		{"dpg-paid", UsageKindStorageGBSeconds, true}:          7200,
		{"red-cache", UsageKindStorageGBSeconds, true}:         3600, // no Key Value storage charge
	}
	equal := func(name string, got, want map[line]int64) {
		t.Helper()
		if len(got) != len(want) {
			t.Errorf("%s = %v, want %v", name, got, want)
			return
		}
		for k, v := range want {
			if got[k] != v {
				t.Errorf("%s %+v = %d, want %d", name, k, got[k], v)
			}
		}
	}
	equal("November", monthOf(s, cutover.Add(-time.Minute)), november)
	equal("December", monthOf(s, cutover.Add(2*time.Hour)), december)

	// Without a switch instant nothing switches: used bytes bill, capacity
	// is shown.
	never := NewPGStore(s.Pool)
	equal("December without a switch", monthOf(never, cutover.Add(2*time.Hour)), map[line]int64{
		{"dpg-paid", UsageKindStorageGBSeconds, false}:        7200,
		{"dpg-paid", UsageKindStorageCapacityGBSeconds, true}: 36000,
		{"red-cache", UsageKindStorageGBSeconds, false}:       3600,
	})

	// The outbox hands the export the shown reading of each hour marked
	// unbilled, which it stamps emitted without sending.
	pending, err := s.SelectWorkspaceUnemittedUsage(ctx, ws, before.Add(-time.Hour), UsageSeal{Before: after.Add(time.Hour)}, 100, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 6 {
		t.Fatalf("outbox rows = %d, want 6: %+v", len(pending), pending)
	}
	sent := map[time.Time][]string{}
	for _, r := range pending {
		wantShown := (r.Kind == UsageKindStorageCapacityGBSeconds) == r.WindowStart.Before(cutover)
		if r.Unbilled != wantShown {
			t.Errorf("outbox %s %s at %s unbilled=%v, want %v", r.ServiceID, r.Kind, r.WindowStart, r.Unbilled, wantShown)
		}
		if !r.Unbilled {
			sent[r.WindowStart.UTC()] = append(sent[r.WindowStart.UTC()], r.ServiceID+"/"+r.Kind)
		}
	}
	if got := sent[before]; len(got) != 2 {
		t.Errorf("the hour before the switch sends %v, want both used-bytes rows", got)
	}
	if got := sent[after]; len(got) != 1 || got[0] != "dpg-paid/"+UsageKindStorageCapacityGBSeconds {
		t.Errorf("the hour from the switch sends %v, want the Postgres's capacity alone", got)
	}

	// Settle before delete counts only what was exported.
	if _, err := s.Pool.Exec(ctx, `UPDATE usage_hourly SET billing_export_state = 'emitted', emitted_at = $2 WHERE workspace_id = $1`, ws, after.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	exported, err := s.ExportedUsageTotals(ctx, ws, before.Add(-time.Hour), after.Add(time.Hour), after)
	if err != nil {
		t.Fatal(err)
	}
	gotExported := map[string]int64{}
	for _, e := range exported {
		gotExported[e.ResourceKind+"/"+e.Kind] += e.Quantity
	}
	wantExported := map[string]int64{
		ResourceKindPostgres + "/" + UsageKindStorageGBSeconds:         7200,
		ResourceKindKeyValue + "/" + UsageKindStorageGBSeconds:         3600,
		ResourceKindPostgres + "/" + UsageKindStorageCapacityGBSeconds: 36000,
	}
	if len(gotExported) != len(wantExported) {
		t.Errorf("exported totals = %v, want %v", gotExported, wantExported)
	}
	for k, v := range wantExported {
		if gotExported[k] != v {
			t.Errorf("exported %s = %d, want %d", k, gotExported[k], v)
		}
	}

	// Compaction keeps both readings of both months, each classified by its
	// month as its hours were, so the billed totals do not move.
	res, err := s.CompactUsage(ctx, after.Add(time.Hour))
	if err != nil || res.HourlyRows != 6 || res.Months != 2 {
		t.Fatalf("compact = %+v, %v; want 6 hourly rows into 2 months", res, err)
	}
	equal("November compacted", monthOf(s, cutover.Add(-time.Minute)), november)
	equal("December compacted", monthOf(s, cutover.Add(2*time.Hour)), december)
}

// Migration 0355 reverts and reapplies: its down removes the capacity kind's
// rows and coverage and refuses the kind again, its up admits it again.
func TestStorageCapacityMigrationDownUpPG(t *testing.T) {
	s, ws := sandboxCPUTestStore(t, "storage-capacity-migration")
	ctx := context.Background()
	window := time.Date(2026, 11, 28, 10, 0, 0, 0, time.UTC)
	row := HourlyRow{WorkspaceID: ws, ServiceID: "dpg-db", ResourceKind: ResourceKindPostgres,
		Kind: UsageKindStorageCapacityGBSeconds, Tier: "basic-1gb", WindowStart: window, Quantity: 36000,
		SourceHealth: []UsageSourceObservation{{Source: UsageSourceStorage, State: UsageSourceHealthy, ExpectedFrom: window}}}
	if err := s.UpsertUsageHourly(ctx, row); err != nil {
		t.Fatal(err)
	}
	read := func(name string) string {
		t.Helper()
		b, err := migrationsFS.ReadFile("migrations/0355_storage_capacity_meter." + name + ".sql")
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, read("down")); err != nil {
		t.Fatalf("down: %v", err)
	}
	var left int
	if err := tx.QueryRow(ctx, `SELECT (SELECT count(*) FROM usage_hourly WHERE kind = $1) + (SELECT count(*) FROM usage_source_streams WHERE kind = $1)`,
		UsageKindStorageCapacityGBSeconds).Scan(&left); err != nil || left != 0 {
		t.Fatalf("after down %d capacity rows and streams remain (%v)", left, err)
	}
	if _, err := tx.Exec(ctx, `SAVEPOINT refused`); err != nil {
		t.Fatal(err)
	}
	if err := upsertUsageHourly(ctx, tx, row); err == nil {
		t.Fatal("after down the capacity kind was accepted")
	}
	if _, err := tx.Exec(ctx, `ROLLBACK TO SAVEPOINT refused`); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, read("up")); err != nil {
		t.Fatalf("up: %v", err)
	}
	if err := upsertUsageHourly(ctx, tx, row); err != nil {
		t.Fatalf("after up the capacity kind was refused: %v", err)
	}
}
