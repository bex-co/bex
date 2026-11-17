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
	"time"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// cron_minimum.go keeps what the cron job minimum (w9/m448, docs/ADR030 §
// Cron job minimum) is computed from: which cron jobs were active in each
// calendar month, and their run time there.

// CronJobMonth is one cron job seen active in a month, with its instance
// seconds there by tier (empty when it ran nothing).
type CronJobMonth struct {
	WorkspaceID   string
	ServiceID     string
	SecondsByTier map[string]int64
}

func monthDate(month time.Time) string { return MonthStartUTC(month).Format(time.DateOnly) }

// RecordActiveCronJobs records every cron job that is active now, created and
// not suspended, as active in month. A cron job deleted later in the month
// keeps its record, so its month still owes the minimum.
func (s *PGStore) RecordActiveCronJobs(ctx context.Context, month time.Time) error {
	_, err := s.Pool.Exec(ctx, `
		INSERT INTO cron_minimum_months (service_id, workspace_id, month)
		SELECT a.id, a.tenant_id, $2::date
		FROM apps a
		WHERE a.type = $1 AND NOT a.suspended AND NOT a.creation_pending
		ON CONFLICT DO NOTHING`, appv1alpha1.TypeCronJob, monthDate(month))
	return err
}

// CronJobMonths returns every cron job recorded active in month, with its
// instance seconds in the month by tier, ordered by workspace and service.
func (s *PGStore) CronJobMonths(ctx context.Context, month time.Time) ([]CronJobMonth, error) {
	from := MonthStartUTC(month)
	rows, err := s.Pool.Query(ctx, `
		SELECT m.workspace_id, m.service_id, u.tier, COALESCE(sum(u.quantity), 0)::bigint
		FROM cron_minimum_months m
		LEFT JOIN usage_hourly u
		  ON u.resource_kind = $1 AND u.service_id = m.service_id AND u.kind = $2
		 AND u.window_start >= $3 AND u.window_start < $4
		WHERE m.month = $5::date
		GROUP BY m.workspace_id, m.service_id, u.tier
		ORDER BY m.workspace_id, m.service_id, u.tier`,
		ResourceKindService, UsageKindInstanceSeconds, from, from.AddDate(0, 1, 0), monthDate(month))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CronJobMonth
	for rows.Next() {
		var workspace, service string
		var tier *string
		var seconds int64
		if err := rows.Scan(&workspace, &service, &tier, &seconds); err != nil {
			return nil, err
		}
		if n := len(out); n == 0 || out[n-1].ServiceID != service || out[n-1].WorkspaceID != workspace {
			out = append(out, CronJobMonth{WorkspaceID: workspace, ServiceID: service, SecondsByTier: map[string]int64{}})
		}
		if tier != nil && seconds > 0 {
			out[len(out)-1].SecondsByTier[*tier] += seconds
		}
	}
	return out, rows.Err()
}

// InsertCronMinimum writes one cron job's top-up row unless the row exists:
// once written it may already be on its way to Stripe, so it is never
// rewritten.
func (s *PGStore) InsertCronMinimum(ctx context.Context, row HourlyRow) error {
	_, err := s.Pool.Exec(ctx, `
		INSERT INTO usage_hourly (workspace_id, service_id, kind, tier, resource_kind, window_start, quantity)
		VALUES ($1, $2, $3, '', $4, $5, $6)
		ON CONFLICT (resource_kind, service_id, kind, tier, window_start) DO NOTHING`,
		row.WorkspaceID, row.ServiceID, UsageKindCronMinimumMicroUSD, ResourceKindService, row.WindowStart.UTC(), row.Quantity)
	return err
}
