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

package objectstorage

import (
	"strings"
	"testing"
	"time"

	ids "github.com/bex-co/bex/lego/backend/internal/id"
)

// The arithmetic of ADR112 D8's listing budget (w21/095). Wasabi counts what a
// listing reads as egress, and the shared account's free egress is its active
// storage, 67 GB when measured on 2026-11-16. These tests hold the sweep's
// scaling, the planned scale and the alert thresholds to each other, so a
// change to one that breaks the others fails here and not in a Wasabi warning.

const (
	// accountAllowanceBytes is the shared Wasabi account's active storage on
	// 2026-11-16 (ADR112 D4), the monthly egress Wasabi's policy lets it have.
	gb                    = 1_000_000_000
	accountAllowanceBytes = 67 * gb
	// egressCriticalBytes is ObjectStoragePublicEgressCritical's threshold.
	egressCriticalBytes = 50 * gb
	month               = 30 * 24 * time.Hour
)

// monthlyListingBytes is what listing a region of the given size costs in a
// month at the gap the sweep leaves between listings: the most listings its gap
// allows, each reading SweepKeyBytes per object. It is an upper bound, since a
// listing comes at the first pass after the gap, not at the gap.
func monthlyListingBytes(sw *Sweeper, objects int64) float64 {
	return float64(month) / float64(sw.regionGap(objects)) * float64(objects*SweepKeyBytes)
}

func TestListResponseBytesMatchesWasabi(t *testing.T) {
	// One full page of 1,000 keys of the shape <tea-id>/<obk-id>/k00000, listed
	// under the prefix <tea-id>/<obk-id>/, was 249,306 bytes from Wasabi's
	// us-west-1 on 2026-11-16 (a throwaway bucket, 400 pages, the SDK's own
	// client).
	const measured = 249_306
	prefix := "tea-d9aaaaaaaaaaaaaaaaaa/obk-d9bbbbbbbbbbbbbbbbbb/"
	objects := make([]ObjectInfo, 1000)
	for i := range objects {
		objects[i].Key = prefix + "k00000"
	}
	got := listResponseBytes(prefix, objects)
	if diff := got - measured; diff < -100 || diff > 100 {
		t.Errorf("listResponseBytes of a full page = %d, Wasabi sent %d", got, measured)
	}
	// The planning figure is that page per key, rounded up.
	if per := float64(measured) / 1000; per > float64(SweepKeyBytes) || per < float64(SweepKeyBytes)-5 {
		t.Errorf("SweepKeyBytes = %d, but a measured page is %.1f bytes a key", SweepKeyBytes, per)
	}
	// An empty listing is a few hundred bytes (274 bytes measured on production
	// for a workspace prefix with nothing under it).
	if got := listResponseBytes(strings.Repeat("x", 24)+"/", nil); got < 200 || got > 400 {
		t.Errorf("listResponseBytes of an empty listing = %d, want about 274", got)
	}
}

func TestTheGapGrowsWithTheRegionAndStopsAtItsCeiling(t *testing.T) {
	sw := NewSweeper(nil, nil)
	if got := sw.regionGap(0); got != sweepMinGap {
		t.Errorf("an empty region's gap = %s, want MinGap %s", got, sweepMinGap)
	}
	if got := sw.regionGap(SweepHourlyObjects); got != sweepMinGap {
		t.Errorf("the gap at %d objects = %s, want MinGap %s", SweepHourlyObjects, got, sweepMinGap)
	}
	if got, want := sw.regionGap(SweepHourlyObjects+1), sweepMinGap+SweepInterval; got != want {
		t.Errorf("the gap at %d objects = %s, want %s", SweepHourlyObjects+1, got, want)
	}
	ceiling := SweepMaxInterval - (SweepInterval - sweepMinGap)
	var last time.Duration
	for n := int64(0); n <= 2*MaxWorkspaceObjects; n += 17 {
		g := sw.regionGap(n)
		if g < last {
			t.Fatalf("the gap fell from %s to %s at %d objects", last, g, n)
		}
		if g > ceiling {
			t.Fatalf("the gap at %d objects is %s, past the ceiling %s", n, g, ceiling)
		}
		last = g
	}
	if got := sw.regionGap(MaxSweepListedObjects); got != ceiling {
		t.Errorf("the gap at the listing stop = %s, want the ceiling %s", got, ceiling)
	}
	// SweepMaxInterval is reached at 7,101 objects, as its comment says: 72 hours
	// for each of the 72 sets of 100 beyond the first 7,100.
	if got := sw.regionGap(71*SweepHourlyObjects + 1); got != ceiling {
		t.Errorf("the gap at 7,101 objects = %s, want the ceiling %s", got, ceiling)
	}
	if got := sw.regionGap(71 * SweepHourlyObjects); got >= ceiling {
		t.Errorf("the gap at 7,100 objects = %s, want less than the ceiling %s", got, ceiling)
	}
}

func TestListingEveryHourWouldHaveBeenNearlyThreeAllowances(t *testing.T) {
	// The cost of what the sweep did before w21/095: one workspace at the object
	// cap, in one region, listed every hour, as it was.
	hourly := float64(month/SweepInterval) * float64(MaxWorkspaceObjects*SweepKeyBytes)
	if hourly < 2.5*accountAllowanceBytes {
		t.Errorf("an hourly listing of one workspace at the cap is %.0f GB a month, the ADR says nearly three allowances (%d GB each)",
			hourly/1e9, accountAllowanceBytes/gb)
	}
}

func TestPlannedScaleStaysInsideTheListingBudget(t *testing.T) {
	sw := NewSweeper(nil, nil)

	// ADR112 D8: the planned scale is two hundred workspace regions at the hourly
	// ceiling (the worst the small ones cost), five of 100,000 objects and one at
	// the cap.
	planned := []struct {
		regions int
		objects int64
	}{
		{200, SweepHourlyObjects},
		{5, 100_000},
		{1, MaxWorkspaceObjects},
	}
	var total float64
	for _, p := range planned {
		cost := float64(p.regions) * monthlyListingBytes(sw, p.objects)
		t.Logf("%d region(s) of %d objects: %.2f GB a month", p.regions, p.objects, cost/1e9)
		total += cost
	}
	// The look under purged workspaces' prefixes (D9) ignores the gaps: up to
	// MaxPurgedPrefixChecks workspaces a pass, one listing in each region, on
	// each of the two replicas. A listing nearly always finds nothing; one key
	// each is the planning figure.
	ws := ids.New(ids.Workspace)
	look := listResponseBytes(WorkspacePrefix(ws), []ObjectInfo{{Key: ws + "/" + ids.New(ids.ObjectBucket) + "/k00000"}})
	looks := 2 * float64(month/SweepInterval) * float64(MaxPurgedPrefixChecks*len(Regions)) * float64(look)
	t.Logf("looks under purged prefixes: at most %.3f GB a month", looks/1e9)
	total += looks
	t.Logf("planned scale: %.2f GB a month against a budget of %.0f GB", total/1e9, float64(SweepMonthlyBudgetBytes)/1e9)
	if total > float64(SweepMonthlyBudgetBytes) {
		t.Errorf("the planned scale lists %.2f GB a month, over the budget of %d GB", total/1e9, SweepMonthlyBudgetBytes/gb)
	}

	// What a small region costs is the same however small the cap of its band:
	// no size from one object to the point where the ceiling starts costs more
	// than the hourly ceiling does.
	worst := monthlyListingBytes(sw, SweepHourlyObjects)
	for n := int64(1); n <= 71*SweepHourlyObjects+1; n++ {
		if c := monthlyListingBytes(sw, n); c > worst {
			t.Fatalf("a region of %d objects costs %.2f MB a month, more than the hourly ceiling's %.2f MB", n, c/1e6, worst/1e6)
		}
	}
	// Past it the cost grows with the objects, ten full listings a month.
	if c := monthlyListingBytes(sw, MaxWorkspaceObjects); c > 3e9 {
		t.Errorf("a region at the object cap costs %.2f GB a month, want at most 3 GB", c/1e9)
	}
}

func TestTheBudgetAndTheEgressAlertsFitTheAllowanceTogether(t *testing.T) {
	// The egress alert's critical threshold is what bex serves to tenants (the
	// shadow meter); Wasabi counts the sweep's listings on top of it. Both must
	// stay under the allowance, and the sweep's share is no more than 15%.
	if SweepMonthlyBudgetBytes*100 > 15*accountAllowanceBytes {
		t.Errorf("the sweep's budget %d GB is more than 15%% of the %d GB allowance", SweepMonthlyBudgetBytes/gb, accountAllowanceBytes/gb)
	}
	if egressCriticalBytes+SweepMonthlyBudgetBytes > accountAllowanceBytes {
		t.Errorf("the critical egress threshold (%d GB) and the sweep's budget (%d GB) add up to more than the %d GB allowance",
			egressCriticalBytes/gb, SweepMonthlyBudgetBytes/gb, accountAllowanceBytes/gb)
	}
}
