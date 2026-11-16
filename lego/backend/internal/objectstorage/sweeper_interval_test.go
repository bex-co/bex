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

package objectstorage_test

import (
	"context"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/objectstorage"
)

// These tests drive the sweep's pass against the in-memory repository of
// sweeper_pool_test.go to prove that a region is listed less often the more
// objects its last observation counted (ADR112 D8, w21/095): Wasabi counts the
// bytes a listing reads as egress, so the sweep spends them in proportion.

// passAt runs one pass with the clock at now and returns the workspaces it listed.
func (e *poolEnv) passAt(now time.Time) []string {
	e.t.Helper()
	e.now = now
	e.gate.resetStarted()
	if err := e.sweeper.Pass(context.Background()); err != nil {
		e.t.Fatalf("Pass at %s = %v", now.Format(time.TimeOnly), err)
	}
	listed := e.gate.startedOrder()
	slices.Sort(listed)
	return listed
}

func TestOverThreeDaysARegionIsListedInProportionToItsSize(t *testing.T) {
	e := newPoolEnv(t, 3, 3)
	e.sweeper.MinGap = 45 * time.Minute // production's, which newPoolEnv turns off
	empty, thousand, atCap := e.repo.order[0], e.repo.order[1], e.repo.order[2]
	e.repo.objects[thousand] = 1_000
	e.repo.objects[atCap] = objectstorage.MaxWorkspaceObjects

	// An hourly pass for three days and the pass that ends them: 73 passes.
	listings := map[string]int{}
	for h := 0; h <= 72; h++ {
		for _, ws := range e.passAt(testNow.Add(time.Duration(h) * time.Hour)) {
			listings[ws]++
		}
	}
	for _, tc := range []struct {
		name string
		ws   string
		want int
	}{
		// Never observed, then every hour: 73.
		{"a region with nothing in it", empty, 73},
		// Never observed, then once the 9h45m gap has passed, which an hourly pass
		// reaches at 10 hours: hours 0, 10, ... 70.
		{"1,000 objects", thousand, 8},
		// Never observed, then the 71h45m ceiling, which an hourly pass reaches at
		// hour 72.
		{"the object cap", atCap, 2},
	} {
		if got := listings[tc.ws]; got != tc.want {
			t.Errorf("%s: listed %d times in 73 hourly passes, want %d", tc.name, got, tc.want)
		}
	}
}

func TestARegionIsListedExactlyWhenItsGapHasPassed(t *testing.T) {
	for _, tc := range []struct {
		name    string
		objects int64
		gap     time.Duration
	}{
		{"empty", 0, 45 * time.Minute},
		{"exactly the hourly count", objectstorage.SweepHourlyObjects, 45 * time.Minute},
		{"one over", objectstorage.SweepHourlyObjects + 1, time.Hour + 45*time.Minute},
		{"1,000 objects", 1_000, 9*time.Hour + 45*time.Minute},
		{"7,100 objects, the last under the ceiling", 7_100, 70*time.Hour + 45*time.Minute},
		{"7,101 objects, where the ceiling starts", 7_101, 71*time.Hour + 45*time.Minute},
		{"7,201 objects, past it", 7_201, 71*time.Hour + 45*time.Minute},
		{"the object cap", objectstorage.MaxWorkspaceObjects, 71*time.Hour + 45*time.Minute},
		{"the listing stop", objectstorage.MaxSweepListedObjects, 71*time.Hour + 45*time.Minute},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newPoolEnv(t, 1, 1)
			e.sweeper.MinGap = 45 * time.Minute
			ws := e.repo.order[0]
			if got := e.passAt(testNow); len(got) != 1 {
				t.Fatalf("the first pass listed %v, want the one workspace", got)
			}
			e.repo.objects[ws] = tc.objects
			// The observation is stamped with the sweep's clock, testNow.
			if got := e.passAt(testNow.Add(tc.gap - time.Nanosecond)); len(got) != 0 {
				t.Errorf("a pass %s after the observation listed %v: the region is due only after %s", tc.gap-time.Nanosecond, got, tc.gap)
			}
			if got := e.passAt(testNow.Add(tc.gap)); len(got) != 1 {
				t.Errorf("a pass %s after the observation listed %v, want the region", tc.gap, got)
			}
		})
	}
}

func TestTheGapNeverScalesInTests(t *testing.T) {
	// A zero MinGap lists every region every pass, which every other test of the
	// sweep relies on, however large the region was.
	e := newPoolEnv(t, 1, 1)
	ws := e.repo.order[0]
	e.passAt(testNow)
	e.repo.objects[ws] = objectstorage.MaxWorkspaceObjects
	if got := e.passAt(testNow); len(got) != 1 {
		t.Fatalf("with MinGap 0 a pass listed %v, want the region again at once", got)
	}
}

func TestAPassLogsAndCountsWhatItsListingsRead(t *testing.T) {
	e := newPoolEnv(t, 3, 2)
	if got := e.passAt(testNow); len(got) != 3 {
		t.Fatalf("listed %v", got)
	}
	// listGate answers one object whose key is <tea-id>/<obk-id>/k under the
	// prefix <tea-id>/; each listing is estimated at the response's fixed bytes,
	// the prefix, and that one entry (sweepbudget.go).
	var want float64
	for _, ws := range e.repo.order {
		key, err := objectstorage.ObjectKey(ws, e.repo.buckets[ws], "k")
		if err != nil {
			t.Fatal(err)
		}
		want += float64(250 + len(objectstorage.WorkspacePrefix(ws)) + 193 + len(key))
	}
	got := e.metric("bex_usage_object_storage_sweep_listed_bytes_total", map[string]string{"region": "oregon"})
	if got != want {
		t.Errorf("sweep_listed_bytes_total{region=oregon} = %v, want %v for three one-object listings", got, want)
	}
	if !e.logged(fmt.Sprintf("listings read about %d bytes", int64(want))) {
		t.Errorf("the pass did not log what its listings read: %q", e.logs)
	}
	// A pass that lists nothing adds nothing.
	e.sweeper.MinGap = 45 * time.Minute
	if got := e.passAt(testNow.Add(time.Minute)); len(got) != 0 {
		t.Fatalf("a pass a minute later listed %v", got)
	}
	if again := e.metric("bex_usage_object_storage_sweep_listed_bytes_total", map[string]string{"region": "oregon"}); again != want {
		t.Errorf("sweep_listed_bytes_total grew from %v to %v on a pass that listed nothing", want, again)
	}
}
