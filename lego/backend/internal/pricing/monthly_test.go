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

package pricing

import (
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

func TestMonthlyEstimateTierFamilies(t *testing.T) {
	got := Default.MonthlyEstimate([]MonthlyResource{
		{Name: "web", ResourceKind: store.ResourceKindService, Tier: "standard"},
		{Name: "db", ResourceKind: store.ResourceKindPostgres, Tier: "basic-1gb", StorageGB: 5},
		{Name: "cache", ResourceKind: store.ResourceKindKeyValue, Tier: "standard", StorageGB: 5},
	})
	if len(got.Lines) != 3 {
		t.Fatalf("lines = %d, want 3: %+v", len(got.Lines), got.Lines)
	}
	wantLines := []struct{ name, monthly, instance, storage string }{
		{"web", "17.50", "17.50", ""},
		{"db", "15.05", "14.00", "1.05"},
		{"cache", "22.05", "21.00", "1.05"},
	}
	for i, w := range wantLines {
		l := got.Lines[i]
		if l.Name != w.name || l.MonthlyUSD != w.monthly || l.InstanceUSD != w.instance || l.StorageUSD != w.storage {
			t.Errorf("line %d = %+v, want %+v", i, l, w)
		}
	}
	if got.TotalUSD != "54.60" {
		t.Errorf("TotalUSD = %s, want 54.60", got.TotalUSD)
	}
	if len(got.Variable) != 0 {
		t.Errorf("Variable = %+v, want empty", got.Variable)
	}
}

func TestMonthlyEstimateFreeAndUnknownTiersProduceNothing(t *testing.T) {
	got := Default.MonthlyEstimate([]MonthlyResource{
		{Name: "web", ResourceKind: store.ResourceKindService, Tier: "free"},
		// A free datastore's storage floor must not surface as a paid line.
		{Name: "db", ResourceKind: store.ResourceKindPostgres, Tier: "free", StorageGB: 1},
		{Name: "mystery", ResourceKind: store.ResourceKindService, Tier: "does-not-exist"},
	})
	if len(got.Lines) != 0 || got.TotalUSD != "0.00" {
		t.Fatalf("free/unknown tiers must contribute nothing, got lines=%+v total=%s", got.Lines, got.TotalUSD)
	}
	if got.Lines == nil || got.Variable == nil {
		t.Fatal("Lines and Variable must be non-nil slices")
	}
}

func TestMonthlyEstimateHighAvailabilityDoublesAndReplicasAdd(t *testing.T) {
	got := Default.MonthlyEstimate([]MonthlyResource{
		{Name: "db", ResourceKind: store.ResourceKindPostgres, Tier: "basic-1gb", StorageGB: 5, HighAvailability: true, ReadReplicas: 2},
	})
	// primary + standby + 2 replicas, each $15.05
	if len(got.Lines) != 4 {
		t.Fatalf("lines = %d, want 4: %+v", len(got.Lines), got.Lines)
	}
	wantNames := []string{"db", "db (standby)", "db (replica)", "db (replica)"}
	for i, name := range wantNames {
		if got.Lines[i].Name != name || got.Lines[i].MonthlyUSD != "15.05" {
			t.Errorf("line %d = %+v, want name %q at 15.05", i, got.Lines[i], name)
		}
	}
	if got.TotalUSD != "60.20" {
		t.Errorf("TotalUSD = %s, want 60.20", got.TotalUSD)
	}
}

func TestMonthlyEstimateVariableClassification(t *testing.T) {
	got := Default.MonthlyEstimate([]MonthlyResource{
		{Name: "auto", ResourceKind: store.ResourceKindService, Tier: "starter", Autoscaling: true, Instances: 3},
		{Name: "fixed", ResourceKind: store.ResourceKindService, Tier: "starter", Instances: 3},
		{Name: "nightly", ResourceKind: store.ResourceKindService, Tier: "starter", Cron: true},
	})
	// Autoscaling wins over multi-instance; cron is variable-only (no line);
	// scaled services still price one base instance toward the total.
	wantVar := []VariableCost{
		{Name: "auto", Reason: VariableAutoscaling},
		{Name: "fixed", Reason: VariableMultiInstance},
		{Name: "nightly", Reason: VariableCron},
	}
	if len(got.Variable) != len(wantVar) {
		t.Fatalf("Variable = %+v, want %+v", got.Variable, wantVar)
	}
	for i, w := range wantVar {
		if got.Variable[i] != w {
			t.Errorf("Variable[%d] = %+v, want %+v", i, got.Variable[i], w)
		}
	}
	if len(got.Lines) != 2 {
		t.Fatalf("lines = %d, want 2 (cron unpriced): %+v", len(got.Lines), got.Lines)
	}
	if got.TotalUSD != "9.80" { // 2 × starter $4.90
		t.Errorf("TotalUSD = %s, want 9.80", got.TotalUSD)
	}
}

func TestMonthlyEstimateStorageOnlyForDatastores(t *testing.T) {
	got := Default.MonthlyEstimate([]MonthlyResource{
		// StorageGB on a service kind must be ignored (services have no
		// provisioned-storage line — persistent disks are a non-goal).
		{Name: "web", ResourceKind: store.ResourceKindService, Tier: "starter", StorageGB: 100},
	})
	if len(got.Lines) != 1 {
		t.Fatalf("lines = %d, want 1: %+v", len(got.Lines), got.Lines)
	}
	if got.Lines[0].MonthlyUSD != "4.90" || got.Lines[0].StorageUSD != "" || got.Lines[0].StorageGB != 0 {
		t.Errorf("service line must carry no storage component: %+v", got.Lines[0])
	}
}
