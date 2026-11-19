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

// monthly_property_test.go encodes the MonthlyEstimate model as properties
// (ADR030): the always-on estimate is monotonic in provisioned storage and in
// the resource set, free/unknown tiers contribute nothing at any storage
// size, high availability doubles the primary, cron never prices, scaling
// prices exactly one instance, and the total is the sum of the displayed
// lines. Hand-rolled generators keep the cases deterministic; they persist
// as durable coverage for the billing domain.

import (
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

// monthlyCents parses a "0.00"-formatted total into integer cents.
func monthlyCents(t *testing.T, usd string) int64 {
	t.Helper()
	parts := strings.Split(usd, ".")
	if len(parts) != 2 || len(parts[1]) != 2 {
		t.Fatalf("TotalUSD %q is not cents-formatted", usd)
	}
	dollars, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		t.Fatalf("TotalUSD %q: %v", usd, err)
	}
	cents, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		t.Fatalf("TotalUSD %q: %v", usd, err)
	}
	if dollars < 0 {
		return dollars*100 - cents
	}
	return dollars*100 + cents
}

// propFamilies is the (tier × kind) space the properties sweep: paid, free
// and unknown tiers in every datastore family.
var propFamilies = []MonthlyResource{
	{Name: "r", ResourceKind: store.ResourceKindService, Tier: "starter"},
	{Name: "r", ResourceKind: store.ResourceKindService, Tier: "standard"},
	{Name: "r", ResourceKind: store.ResourceKindService, Tier: "free"},
	{Name: "r", ResourceKind: store.ResourceKindService, Tier: "no-such-tier"},
	{Name: "r", ResourceKind: store.ResourceKindPostgres, Tier: "basic-256mb"},
	{Name: "r", ResourceKind: store.ResourceKindPostgres, Tier: "1c-2g"},
	{Name: "r", ResourceKind: store.ResourceKindPostgres, Tier: "free"},
	{Name: "r", ResourceKind: store.ResourceKindKeyValue, Tier: "starter"},
	{Name: "r", ResourceKind: store.ResourceKindKeyValue, Tier: "free"},
}

// Adding resources never decreases the total: every priced line is
// non-negative, and free/unknown resources contribute nothing.
func TestMonthlyEstimateSupersetNeverPricesLess(t *testing.T) {
	bases := [][]MonthlyResource{
		{},
		{{Name: "web", ResourceKind: store.ResourceKindService, Tier: "starter"}},
		{
			{Name: "web", ResourceKind: store.ResourceKindService, Tier: "starter", StorageGB: 10},
			{Name: "db", ResourceKind: store.ResourceKindPostgres, Tier: "basic-1gb", StorageGB: 5},
		},
	}
	extras := []MonthlyResource{
		{Name: "x", ResourceKind: store.ResourceKindService, Tier: "starter"},
		{Name: "x", ResourceKind: store.ResourceKindService, Tier: "free", StorageGB: 1000},
		{Name: "x", ResourceKind: store.ResourceKindService, Tier: "no-such-tier"},
		{Name: "x", ResourceKind: store.ResourceKindPostgres, Tier: "basic-256mb", StorageGB: 50},
		{Name: "x", ResourceKind: store.ResourceKindPostgres, Tier: "free", StorageGB: 1000},
		{Name: "x", ResourceKind: store.ResourceKindKeyValue, Tier: "standard", StorageGB: 20},
		{Name: "x", ResourceKind: store.ResourceKindService, Tier: "starter", Cron: true},
		{Name: "x", ResourceKind: store.ResourceKindService, Tier: "starter", Autoscaling: true},
		{Name: "x", ResourceKind: store.ResourceKindService, Tier: "starter", Instances: 9},
	}
	for _, base := range bases {
		baseCents := monthlyCents(t, Default.MonthlyEstimate(base).TotalUSD)
		for _, extra := range extras {
			got := Default.MonthlyEstimate(append(append([]MonthlyResource{}, base...), extra))
			if c := monthlyCents(t, got.TotalUSD); c < baseCents {
				t.Errorf("base %+v (%d cents) + extra %+v = %s: total decreased",
					base, baseCents, extra, got.TotalUSD)
			}
		}
	}
}

// Holding tier and kind fixed, more provisioned storage never prices less.
// Storage rates are non-negative, so the storage component grows with size.
func TestMonthlyEstimateStorageMonotonic(t *testing.T) {
	sizes := []int32{0, 1, 5, 10, 100, 1000, 100000}
	for _, fam := range propFamilies {
		var prev int64 = -1
		for _, size := range sizes {
			r := fam
			r.StorageGB = size
			got := monthlyCents(t, Default.MonthlyEstimate([]MonthlyResource{r}).TotalUSD)
			if prev >= 0 && got < prev {
				t.Errorf("%s/%s storage %dGB prices below a smaller size", fam.Tier, fam.ResourceKind, size)
			}
			prev = got
		}
	}
}

// A free or unknown tier contributes nothing at any storage size: the whole
// line is filtered out, including its storage floor (ADR030 §2, §59).
func TestMonthlyEstimateFreeAndUnknownIgnoreAnyStorage(t *testing.T) {
	tiers := []MonthlyResource{
		{Name: "r", ResourceKind: store.ResourceKindService, Tier: "free"},
		{Name: "r", ResourceKind: store.ResourceKindPostgres, Tier: "free"},
		{Name: "r", ResourceKind: store.ResourceKindKeyValue, Tier: "free"},
		{Name: "r", ResourceKind: store.ResourceKindService, Tier: "no-such-tier"},
		{Name: "r", ResourceKind: "mystery", Tier: "starter"},
	}
	for _, base := range tiers {
		for _, size := range []int32{0, 1, 100, 100000} {
			r := base
			r.StorageGB = size
			got := Default.MonthlyEstimate([]MonthlyResource{r})
			if got.TotalUSD != "0.00" || len(got.Lines) != 0 {
				t.Errorf("%s/%s at %dGB = total %s lines %+v, want nothing priced",
					base.Tier, base.ResourceKind, size, got.TotalUSD, got.Lines)
			}
		}
	}
}

// High availability doubles the primary: the standby is one more line at the
// same price (ADR030 §47: Render bills the standby like the primary).
func TestMonthlyEstimateHighAvailabilityEqualsTwoPrimaries(t *testing.T) {
	for _, tier := range []string{"basic-256mb", "basic-1gb", "1c-2g"} {
		ha := Default.MonthlyEstimate([]MonthlyResource{{
			Name: "db", ResourceKind: store.ResourceKindPostgres, Tier: tier,
			StorageGB: 10, HighAvailability: true,
		}})
		two := Default.MonthlyEstimate([]MonthlyResource{
			{Name: "db", ResourceKind: store.ResourceKindPostgres, Tier: tier, StorageGB: 10},
			{Name: "db", ResourceKind: store.ResourceKindPostgres, Tier: tier, StorageGB: 10},
		})
		if ha.TotalUSD != two.TotalUSD {
			t.Errorf("tier %s HA total %s != two primaries %s", tier, ha.TotalUSD, two.TotalUSD)
		}
		if len(ha.Lines) != 2 {
			t.Errorf("tier %s HA lines = %+v, want primary + standby", tier, ha.Lines)
		}
		if ha.Lines[1].Name != "db (standby)" || ha.Lines[1].MonthlyUSD != ha.Lines[0].MonthlyUSD {
			t.Errorf("tier %s standby line = %+v, want the primary's price", tier, ha.Lines[1])
		}
	}
}

// Each read replica adds one primary-priced line.
func TestMonthlyEstimateReadReplicasAddPrimaryPricedLines(t *testing.T) {
	got := Default.MonthlyEstimate([]MonthlyResource{{
		Name: "db", ResourceKind: store.ResourceKindPostgres, Tier: "1c-2g",
		StorageGB: 10, ReadReplicas: 2,
	}})
	if len(got.Lines) != 3 {
		t.Fatalf("lines = %+v, want primary + 2 replicas", got.Lines)
	}
	for i, line := range got.Lines {
		if line.MonthlyUSD != got.Lines[0].MonthlyUSD {
			t.Errorf("line %d = %+v, want the primary's price %s", i, line, got.Lines[0].MonthlyUSD)
		}
	}
}

// A cron job never prices — not its instance, not its storage, not HA or
// replicas: it bills per second while runs execute (ADR030 §7, §110), so the
// always-on figure excludes it and lists it as variable instead.
func TestMonthlyEstimateCronNeverPrices(t *testing.T) {
	cron := MonthlyResource{
		Name: "nightly", ResourceKind: store.ResourceKindService, Tier: "pro-max",
		Cron:      true,
		StorageGB: 100, HighAvailability: true, ReadReplicas: 3, Instances: 5,
	}
	for _, extra := range []MonthlyResource{cron, {Name: "n", ResourceKind: store.ResourceKindService, Tier: "starter", Cron: true, Autoscaling: true}} {
		got := Default.MonthlyEstimate([]MonthlyResource{extra})
		if got.TotalUSD != "0.00" || len(got.Lines) != 0 {
			t.Errorf("cron %+v = total %s lines %+v, want nothing priced", extra, got.TotalUSD, got.Lines)
		}
		if len(got.Variable) != 1 || got.Variable[0].Reason != VariableCron {
			t.Errorf("cron %+v variable = %+v, want one cron entry", extra, got.Variable)
		}
	}
	// A cron alongside a paid service leaves that service's figure untouched.
	with := Default.MonthlyEstimate([]MonthlyResource{
		{Name: "web", ResourceKind: store.ResourceKindService, Tier: "starter"},
		cron,
	})
	without := Default.MonthlyEstimate([]MonthlyResource{
		{Name: "web", ResourceKind: store.ResourceKindService, Tier: "starter"},
	})
	if with.TotalUSD != without.TotalUSD || !reflect.DeepEqual(with.Lines, without.Lines) {
		t.Errorf("cron changed a paid service's figure: %+v vs %+v", with, without)
	}
}

// Autoscaling and multi-instance price exactly one base instance: the count
// is runtime behavior, listed as variable rather than multiplied in.
func TestMonthlyEstimateScaledServicePricesOneInstance(t *testing.T) {
	base := MonthlyResource{Name: "web", ResourceKind: store.ResourceKindService, Tier: "starter", StorageGB: 10}
	want := Default.MonthlyEstimate([]MonthlyResource{base})
	scaled := []MonthlyResource{
		{Name: "web", ResourceKind: store.ResourceKindService, Tier: "starter", StorageGB: 10, Autoscaling: true, Instances: 5},
		{Name: "web", ResourceKind: store.ResourceKindService, Tier: "starter", StorageGB: 10, Instances: 5},
	}
	for _, r := range scaled {
		got := Default.MonthlyEstimate([]MonthlyResource{r})
		if got.TotalUSD != want.TotalUSD || !reflect.DeepEqual(got.Lines, want.Lines) {
			t.Errorf("%+v prices %+v, want one base instance %+v", r, got, want)
		}
	}
	if got := Default.MonthlyEstimate(scaled[:1]); len(got.Variable) != 1 || got.Variable[0].Reason != VariableAutoscaling {
		t.Errorf("autoscaling variable = %+v, want one autoscaling entry", got.Variable)
	}
	if got := Default.MonthlyEstimate(scaled[1:]); len(got.Variable) != 1 || got.Variable[0].Reason != VariableMultiInstance {
		t.Errorf("multi-instance variable = %+v, want one multi_instance entry", got.Variable)
	}
}

// The total is the sum of the displayed lines, to the cent: the panel's
// TotalUSD is what a reader gets adding the line items (ADR030). Each line
// rounds its own figure, so the total must sum the rounded lines, not round
// the raw float sum — otherwise multi-line stacks drift by a cent.
func TestMonthlyEstimateTotalEqualsSumOfLines(t *testing.T) {
	sumLines := func(t *testing.T, est MonthlyEstimate) int64 {
		t.Helper()
		var sum int64
		for _, l := range est.Lines {
			sum += monthlyCents(t, l.MonthlyUSD)
		}
		return sum
	}
	for _, fam := range propFamilies {
		for _, size := range []int32{0, 1, 7, 10, 100, 1000} {
			r := fam
			r.StorageGB = size
			got := Default.MonthlyEstimate([]MonthlyResource{r})
			if total, sum := monthlyCents(t, got.TotalUSD), sumLines(t, got); total != sum {
				t.Errorf("%s/%s at %dGB: total %s != sum of lines (%d cents vs %d)",
					fam.Tier, fam.ResourceKind, size, got.TotalUSD, total, sum)
			}
		}
	}
	stacks := [][]MonthlyResource{
		{
			{Name: "web", ResourceKind: store.ResourceKindService, Tier: "starter", StorageGB: 17},
			{Name: "db", ResourceKind: store.ResourceKindPostgres, Tier: "basic-256mb", StorageGB: 23, HighAvailability: true, ReadReplicas: 2},
			{Name: "cache", ResourceKind: store.ResourceKindKeyValue, Tier: "starter", StorageGB: 29},
		},
		{
			{Name: "web", ResourceKind: store.ResourceKindService, Tier: "standard", StorageGB: 3},
			{Name: "db", ResourceKind: store.ResourceKindPostgres, Tier: "1c-2g", StorageGB: 41, ReadReplicas: 1},
			{Name: "cache", ResourceKind: store.ResourceKindKeyValue, Tier: "standard", StorageGB: 13},
			{Name: "worker", ResourceKind: store.ResourceKindService, Tier: "standard", StorageGB: 7, Instances: 4},
		},
	}
	for i, stack := range stacks {
		got := Default.MonthlyEstimate(stack)
		if total, sum := monthlyCents(t, got.TotalUSD), sumLines(t, got); total != sum {
			t.Errorf("stack %d: total %s != sum of lines (%d cents vs %d): %+v",
				i, got.TotalUSD, total, sum, got.Lines)
		}
	}
}

// The estimate is deterministic: same resources, same figure, every time.
func TestMonthlyEstimateDeterministic(t *testing.T) {
	in := []MonthlyResource{
		{Name: "web", ResourceKind: store.ResourceKindService, Tier: "standard", StorageGB: 10, Instances: 3},
		{Name: "db", ResourceKind: store.ResourceKindPostgres, Tier: "1c-2g", StorageGB: 10, HighAvailability: true, ReadReplicas: 1},
		{Name: "cache", ResourceKind: store.ResourceKindKeyValue, Tier: "standard", StorageGB: 5},
		{Name: "nightly", ResourceKind: store.ResourceKindService, Tier: "starter", Cron: true},
	}
	want := Default.MonthlyEstimate(in)
	for i := 0; i < 25; i++ {
		if got := Default.MonthlyEstimate(in); !reflect.DeepEqual(got, want) {
			t.Fatalf("run %d differs: %+v vs %+v", i, got, want)
		}
	}
}
