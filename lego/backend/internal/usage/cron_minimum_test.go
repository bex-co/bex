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

package usage

import (
	"context"
	"math"
	"math/big"
	"slices"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

// The in-memory store records no cron jobs: the cron job minimum's SQL and
// the whole rule run against Postgres (cron_minimum_pg_test.go).
func (m *memUsageStore) RecordActiveCronJobs(context.Context, time.Time) error { return nil }

func (m *memUsageStore) CronJobMonths(context.Context, time.Time) ([]store.CronJobMonth, error) {
	return nil, nil
}

func (m *memUsageStore) InsertCronMinimum(context.Context, store.HourlyRow) error { return nil }

// w9/m448: a cron job whose runs cost less than the minimum owes the
// difference, rounded to the micro-USD; one whose runs reach it owes nothing.
func TestCronMinimumTopUp(t *testing.T) {
	for _, tc := range []struct {
		name string
		run  *big.Rat
		want int64
	}{
		{"no runs", new(big.Rat), 700_000},
		{"some runs", big.NewRat(400_000, 1), 300_000},
		{"half a micro-USD rounds up", big.NewRat(400_001, 2), 499_999},
		{"exactly the minimum", big.NewRat(700_000, 1), 0},
		{"over the minimum", big.NewRat(3_000_000, 1), 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := cronMinimumTopUp(700_000, tc.run); got != tc.want {
				t.Errorf("cronMinimumTopUp(700000, %s) = %d, want %d", tc.run.FloatString(1), got, tc.want)
			}
		})
	}
}

// w9/m448 as properties: the top-up is the complement of the rounded run
// cost to the minimum (so the month's estimate shows max(run, minimum)),
// bounded by it, monotonic in the runs, and nothing is owed when no minimum
// is configured. The expected rounding is exact integer arithmetic
// (quotient/remainder, ties up), independent of the FloatString path the
// implementation takes. Hand-rolled and deterministic, like the pricing
// package's property tests.
func TestCronMinimumTopUpComplement(t *testing.T) {
	const minimum = 700_000
	exact := func(micros int64) *big.Rat { return big.NewRat(micros, 1) }
	rats := []*big.Rat{
		exact(0), exact(1), exact(2), exact(100),
		exact(400_000), exact(499_999), exact(500_000),
		exact(699_998), exact(699_999), exact(minimum), exact(700_001),
		exact(1_000_000), exact(3_000_000), exact(1_000_000_000_000),
	}
	ties := []int64{0, 1, 199_999, 200_000, 699_998, 699_999}
	for _, m := range ties {
		rats = append(rats, big.NewRat(2*m+1, 2)) // m + 1/2 micro-USD rounds up
	}
	rats = append(rats,
		big.NewRat(1, 3), big.NewRat(2, 3),
		big.NewRat(2_099_998, 3), // 699999.33 rounds down
		big.NewRat(2_099_999, 3), // 699999.67 rounds up
	)
	huge, ok := new(big.Rat).SetString("1000000000000000000000000000000")
	if !ok {
		t.Fatal("bad huge run literal")
	}
	rats = append(rats, huge)
	slices.SortFunc(rats, (*big.Rat).Cmp)

	prev := int64(math.MaxInt64)
	for _, run := range rats {
		got := cronMinimumTopUp(minimum, run)
		num, den := run.Num(), run.Denom()
		q, r := new(big.Int).QuoRem(num, den, new(big.Int))
		rounded := new(big.Int).Set(q)
		if new(big.Int).Mul(r, big.NewInt(2)).Cmp(den) >= 0 {
			rounded.Add(rounded, big.NewInt(1))
		}
		want := int64(0)
		if rounded.Cmp(big.NewInt(minimum)) < 0 {
			want = minimum - rounded.Int64()
		}
		if got != want {
			t.Errorf("cronMinimumTopUp(%d, %s) = %d, want complement %d", minimum, run.FloatString(2), got, want)
		}
		if got < 0 || got > minimum {
			t.Errorf("cronMinimumTopUp(%d, %s) = %d, want within [0, %d]", minimum, run.FloatString(2), got, minimum)
		}
		if got > prev {
			t.Errorf("top-up rose to %d after %d as runs grew: not monotonic", got, prev)
		}
		prev = got
		if z := cronMinimumTopUp(0, run); z != 0 {
			t.Errorf("cronMinimumTopUp(0, %s) = %d, want 0: no minimum bills nothing", run.FloatString(2), z)
		}
	}
}

// A month is charged once its last hour is past the rollup's rewrite window,
// never earlier: before then a late rollup could still add run time.
func TestCronMinimumDelayCoversTheRewriteWindow(t *testing.T) {
	if cronMinimumDelay <= CatchupWindow {
		t.Fatalf("cronMinimumDelay %s must exceed the rollup's rewrite window %s", cronMinimumDelay, CatchupWindow)
	}
}
