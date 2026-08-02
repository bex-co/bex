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
)

// UsageKind names the metering dimensions. storage_gb_seconds is the average
// used datastore volume size in decimal GB multiplied by window seconds.
const (
	UsageKindInstanceSeconds  = "instance_seconds"
	UsageKindEgressBytes      = "egress_bytes"
	UsageKindBuildSeconds     = "build_seconds"
	UsageKindStorageGBSeconds = "storage_gb_seconds"
	// UsageKindSandboxComputeSeconds is measured in milli-vCPU-equivalent
	// seconds. Memory is folded into the weight at the AgentCore reference
	// ratio ($0.00945/GB-hour ÷ $0.0895/vCPU-hour); see ADR047 D6.
	UsageKindSandboxComputeSeconds = "sandbox_compute_seconds"
)

// NormalizeResourceKind preserves the pre-resource-kind behavior for callers
// that omit the field. Persisted rows always carry an explicit kind.
func NormalizeResourceKind(kind string) string {
	if kind == "" {
		return ResourceKindService
	}
	return kind
}

// ResourceKind identifies what type of resource a usage row belongs to.
// "service" is the default (App CRs); "postgres" and "key_value" are the
// managed-datastore kinds added in w8/m5.
const (
	ResourceKindService  = "service"   // App (web/worker/cron/static)
	ResourceKindPostgres = "postgres"  // Database CR → CNPG Cluster
	ResourceKindKeyValue = "key_value" // KeyValue CR → Valkey StatefulSet
	ResourceKindSandbox  = "sandbox"   // hosted OpenSandbox execution environment
)

// HourlyRow is one window of usage for one resource + meter kind.
type HourlyRow struct {
	WorkspaceID  string
	ServiceID    string
	Kind         string
	Tier         string    // non-empty only for instance_seconds
	ResourceKind string    // ResourceKindService / ResourceKindPostgres / ResourceKindKeyValue
	WindowStart  time.Time // truncated to the hour (UTC)
	Quantity     int64
}

// UsageSummaryRow is one resource/meter-kind/tier aggregate as returned by
// UsageMonthToDate — the raw numbers m2's core verb formats for adapters.
type UsageSummaryRow struct {
	ServiceID    string
	Kind         string
	Tier         string // non-empty only for instance_seconds
	ResourceKind string // ResourceKindService / ResourceKindPostgres / ResourceKindKeyValue
	Total        int64
}

// UsageCompaction reports what one CompactUsage pass did — the operational
// summary the compaction loop logs.
type UsageCompaction struct {
	Months     int64 // distinct calendar months compacted
	HourlyRows int64 // usage_hourly rows purged
}

// UpsertUsageHourly writes one window row, creating it or updating the
// quantity to the new value if the
// (resource_kind, service_id, kind, tier, window_start) key already exists.
// The ON CONFLICT … DO UPDATE makes the rollup loop idempotent.
func (s *PGStore) UpsertUsageHourly(ctx context.Context, row HourlyRow) error {
	rk := NormalizeResourceKind(row.ResourceKind)
	_, err := s.Pool.Exec(ctx, `
		INSERT INTO usage_hourly (workspace_id, service_id, kind, tier, resource_kind, window_start, quantity)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (resource_kind, service_id, kind, tier, window_start)
		DO UPDATE SET quantity = EXCLUDED.quantity`,
		row.WorkspaceID, row.ServiceID, row.Kind, row.Tier, rk, row.WindowStart, row.Quantity)
	return err
}

// LatestUsageWindow returns the most recent window_start for one resource and
// meter kind, or zero time if none exist. Tracking each meter independently
// lets a newly-added or temporarily unavailable meter catch up without being
// hidden by a newer row for another meter.
func (s *PGStore) LatestUsageWindow(ctx context.Context, resourceKind, serviceID, kind string) (time.Time, error) {
	var t time.Time
	err := s.Pool.QueryRow(ctx,
		`SELECT COALESCE(MAX(window_start), 'epoch') FROM usage_hourly WHERE resource_kind = $1 AND service_id = $2 AND kind = $3`,
		NormalizeResourceKind(resourceKind), serviceID, kind,
	).Scan(&t)
	if err != nil {
		return time.Time{}, err
	}
	return t, nil
}

// UsageMonthToDate returns month-to-date totals for all services in a
// workspace, grouped by (resource_kind, service_id, kind, tier). now is the
// caller-supplied clock value so tests can drive it without real time. "Month
// to date" is calendar-month-start (UTC) to now.
//
// The query sums usage_hourly and usage_monthly together (w8/m4): a hot month
// has only hourly rows, a compacted month has only its monthly aggregate, and
// a month caught mid-transition (partially compacted at the 48 h clamp) has
// both — the UNION ALL sum is exact in every state, so period queries never
// depend on whether compaction has run yet.
func (s *PGStore) UsageMonthToDate(ctx context.Context, workspaceID string, now time.Time) ([]UsageSummaryRow, error) {
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	rows, err := s.Pool.Query(ctx, `
		SELECT service_id, kind, tier, resource_kind, SUM(quantity)
		FROM (
			SELECT service_id, kind, tier, resource_kind, quantity
			FROM usage_hourly
			WHERE workspace_id = $1
			  AND window_start >= $2
			  AND window_start < $3
			UNION ALL
			SELECT service_id, kind, tier, resource_kind, quantity
			FROM usage_monthly
			WHERE workspace_id = $1
			  AND month = ($2::timestamptz AT TIME ZONE 'UTC')::date
		) u
		GROUP BY service_id, kind, tier, resource_kind
		HAVING kind = 'instance_seconds' OR SUM(quantity) <> 0
		ORDER BY service_id, kind, tier, resource_kind`,
		workspaceID, monthStart, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []UsageSummaryRow
	for rows.Next() {
		var r UsageSummaryRow
		if err := rows.Scan(&r.ServiceID, &r.Kind, &r.Tier, &r.ResourceKind, &r.Total); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// CompactUsage folds every usage_hourly row with window_start < before into
// usage_monthly and purges it (docs/ADR023-usage-metering.md § Retention). A single
// statement, so purge and aggregate are atomic and a re-run is a no-op; the
// additive ON CONFLICT means a straggler row compacted on a later pass adds
// to its month rather than overwriting it.
func (s *PGStore) CompactUsage(ctx context.Context, before time.Time) (UsageCompaction, error) {
	var res UsageCompaction
	err := s.Pool.QueryRow(ctx, `
		WITH purged AS (
			DELETE FROM usage_hourly
			WHERE window_start < $1
			RETURNING workspace_id, service_id, kind, tier, resource_kind, window_start, quantity
		), compacted AS (
			INSERT INTO usage_monthly (workspace_id, service_id, kind, tier, resource_kind, month, quantity)
			SELECT workspace_id, service_id, kind, tier, resource_kind,
			       date_trunc('month', window_start AT TIME ZONE 'UTC')::date,
			       SUM(quantity)
			FROM purged
			GROUP BY workspace_id, service_id, kind, tier, resource_kind,
			         date_trunc('month', window_start AT TIME ZONE 'UTC')
			ON CONFLICT (resource_kind, service_id, kind, tier, month)
			DO UPDATE SET quantity = usage_monthly.quantity + EXCLUDED.quantity
			RETURNING month
		)
		SELECT (SELECT COUNT(*) FROM purged),
		       (SELECT COUNT(DISTINCT month) FROM compacted)`,
		before,
	).Scan(&res.HourlyRows, &res.Months)
	return res, err
}
