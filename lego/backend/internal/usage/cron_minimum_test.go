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
	"math/big"
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

// A month is charged once its last hour is past the rollup's rewrite window,
// never earlier: before then a late rollup could still add run time.
func TestCronMinimumDelayCoversTheRewriteWindow(t *testing.T) {
	if cronMinimumDelay <= CatchupWindow {
		t.Fatalf("cronMinimumDelay %s must exceed the rollup's rewrite window %s", cronMinimumDelay, CatchupWindow)
	}
}
