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
	"log"
	"math/big"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/pricing"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// cron_minimum.go is the cron job minimum (w9/m448, docs/ADR030 § Cron job
// minimum). Render bills a cron job by the second of run time with "a minimum
// monthly charge of $1 per cron job service" (render.com/docs/cronjobs); bex
// charges that minimum × 0.70 like every Render price. A cron job active in a
// calendar month whose runs there cost less is charged the difference, as one
// cron_minimum_microusd row in the month's last hour. The Stripe export sends
// that row as a "Cron job minimum" invoice item.
//
// Who owes it. Every cron job seen active (created, not suspended) by the
// hourly sweep at any point in the month, deleted since or not, owes it once,
// unprorated: Render publishes no proration of the minimum, and suspending a
// service stops Render's billing for it, so a cron job suspended all month
// owes nothing.
//
// When. The month's run time is final once the rollup can no longer rewrite
// its last hour (CatchupWindow), so a month is charged an hour after that,
// once, and its own cursor records it (usage_meter_cursors). The row is never
// rewritten, so a second pass, a second replica or a manual settle charges
// nothing more.

// cronMinimumDelay is how long after a month ends its cron minimums are
// charged: past the rollup's rewrite window, so the run time is final.
const cronMinimumDelay = catchupLimit + time.Hour

// cronMinimumTopUp is what a cron job whose runs in a month cost run
// micro-USD owes to reach minimum micro-USD; zero when the runs reach it.
func cronMinimumTopUp(minimum int64, run *big.Rat) int64 {
	rounded, _ := new(big.Int).SetString(run.FloatString(0), 10)
	if rounded == nil || !rounded.IsInt64() {
		return 0
	}
	return max(minimum-rounded.Int64(), 0)
}

// chargeCronMinimums records the cron jobs active at now and charges the
// minimum for every month whose run time has become final since the last
// month charged.
func (s *Service) chargeCronMinimums(ctx context.Context, now time.Time) {
	if s.Store == nil {
		return
	}
	now = now.UTC()
	if err := s.Store.RecordActiveCronJobs(ctx, now); err != nil {
		log.Printf("usage: cron minimum: record active cron jobs: %v", err)
	}
	minimum := pricing.Default.CronMinimumMicroUSD()
	if minimum <= 0 {
		return
	}
	last := store.MonthStartUTC(now.Add(-cronMinimumDelay)).AddDate(0, -1, 0)
	month := last
	cursor, ok, err := s.Store.UsageMeterCursor(ctx, store.UsageKindCronMinimumMicroUSD)
	if err != nil {
		log.Printf("usage: cron minimum cursor: %v", err)
		return
	}
	if ok {
		month = store.MonthStartUTC(cursor).AddDate(0, 1, 0)
	}
	for ; !month.After(last); month = month.AddDate(0, 1, 0) {
		if err := s.chargeCronMinimumMonth(ctx, month, minimum); err != nil {
			log.Printf("usage: cron minimum %s: %v", month.Format("2006-01"), err)
			return
		}
		if err := s.Store.AdvanceUsageMeterCursor(ctx, store.UsageKindCronMinimumMicroUSD, month); err != nil {
			log.Printf("usage: cron minimum cursor %s: %v", month.Format("2006-01"), err)
			return
		}
	}
}

// chargeCronMinimumMonth writes the top-up row of every cron job active in
// month whose runs there cost less than minimum.
func (s *Service) chargeCronMinimumMonth(ctx context.Context, month time.Time, minimum int64) error {
	jobs, err := s.Store.CronJobMonths(ctx, month)
	if err != nil {
		return err
	}
	window := month.AddDate(0, 1, 0).Add(-time.Hour)
	for _, job := range jobs {
		// No plan applies to the default tenant, as for every plan charge.
		if job.WorkspaceID == core.DefaultTenant {
			continue
		}
		run := new(big.Rat)
		for tier, seconds := range job.SecondsByTier {
			run.Add(run, pricing.Default.ServiceInstanceMicroUSD(tier, seconds))
		}
		topUp := cronMinimumTopUp(minimum, run)
		if topUp == 0 {
			continue
		}
		if err := s.Store.InsertCronMinimum(ctx, store.HourlyRow{
			WorkspaceID:  job.WorkspaceID,
			ServiceID:    job.ServiceID,
			Kind:         store.UsageKindCronMinimumMicroUSD,
			ResourceKind: store.ResourceKindService,
			WindowStart:  window,
			Quantity:     topUp,
		}); err != nil {
			return err
		}
	}
	return nil
}
