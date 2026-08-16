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
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
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

// Usage source-health vocabulary. Keep this closed and presentation-safe: the
// values are returned to clients when a current-month total is partial, so
// they must never contain provider errors, metric selectors, or tenant data.
const (
	UsageSourceInstance  = "instance"
	UsageSourceBuild     = "build"
	UsageSourceStorage   = "storage"
	UsageSourceHTTP      = "http"
	UsageSourceWebSocket = "websocket"
	UsageSourceDirect    = "direct"
	UsageSourcePostgres  = "postgres"
	UsageSourceKeyValue  = "key_value"
	// UsageSourceSandbox is response-only for now: sandbox compute uses its
	// own durable lifecycle cursor rather than usage.Service's hourly source
	// observations, so any current-month sandbox total conservatively degrades
	// otherwise-known workspace coverage.
	UsageSourceSandbox = "sandbox"

	UsageSourceHealthy     = "healthy"
	UsageSourceDegraded    = "degraded"
	UsageSourceUnavailable = "unavailable"
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
	// SourceHealth is durable evidence for this exact window. nil means the
	// caller has no modern coverage evidence (not healthy by default).
	SourceHealth []UsageSourceObservation
}

// UsageSourceObservation is one source's accounting health for an hourly
// usage row. ExpectedFrom is the earliest hour the resource was expected to
// contribute; it prevents a mid-month migration from blessing legacy gaps.
type UsageSourceObservation struct {
	Source       string
	State        string
	ExpectedFrom time.Time
}

// UsageSourceRecord persists health when no usage row can be written (for
// example a transport error). It makes partial evidence explicit instead of
// relying on a missing row, which remains unknown/legacy.
type UsageSourceRecord struct {
	WorkspaceID  string
	ResourceKind string
	ServiceID    string
	Kind         string
	WindowStart  time.Time
	UsageSourceObservation
}

// UsageResourceRef is the current collector inventory. Reconciliation closes
// streams for deleted resources so they do not hold the common watermark back
// forever; their historical health rows remain intact.
type UsageResourceRef struct {
	ResourceKind string `json:"resourceKind"`
	ServiceID    string `json:"serviceId"`
}

// UsageCoverage is the store's current-month evidence aggregate.
type UsageCoverage struct {
	Known           bool
	Complete        bool
	Through         time.Time
	DegradedSources []string
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
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = tx.Exec(ctx, `
		INSERT INTO usage_hourly (workspace_id, service_id, kind, tier, resource_kind, window_start, quantity)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (resource_kind, service_id, kind, tier, window_start)
		DO UPDATE SET quantity = EXCLUDED.quantity`,
		row.WorkspaceID, row.ServiceID, row.Kind, row.Tier, rk, row.WindowStart, row.Quantity); err != nil {
		return err
	}
	for _, observation := range row.SourceHealth {
		if err = upsertUsageSourceObservation(ctx, tx, UsageSourceRecord{
			WorkspaceID: row.WorkspaceID, ResourceKind: rk, ServiceID: row.ServiceID,
			Kind: row.Kind, WindowStart: row.WindowStart, UsageSourceObservation: observation,
		}); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

type usageSourceExecer interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}

func upsertUsageSourceObservation(ctx context.Context, exec usageSourceExecer, record UsageSourceRecord) error {
	if !validUsageSource(record.Source) || !validUsageSourceState(record.State) {
		return fmt.Errorf("invalid usage source health")
	}
	rk := NormalizeResourceKind(record.ResourceKind)
	expectedFrom := record.ExpectedFrom.UTC().Truncate(time.Hour)
	if expectedFrom.IsZero() {
		expectedFrom = record.WindowStart.UTC().Truncate(time.Hour)
	}
	if _, err := exec.Exec(ctx, `
		INSERT INTO usage_source_streams
			(workspace_id, resource_kind, service_id, kind, source, expected_from, expected_through)
		VALUES ($1, $2, $3, $4, $5, $6, NULL)
		ON CONFLICT (resource_kind, service_id, kind, source) DO UPDATE
		SET workspace_id = EXCLUDED.workspace_id,
		    expected_from = LEAST(usage_source_streams.expected_from, EXCLUDED.expected_from),
		    expected_through = NULL`,
		record.WorkspaceID, rk, record.ServiceID, record.Kind, record.Source, expectedFrom); err != nil {
		return err
	}
	_, err := exec.Exec(ctx, `
		INSERT INTO usage_source_health
			(workspace_id, resource_kind, service_id, kind, source, window_start, state)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (resource_kind, service_id, kind, source, window_start) DO UPDATE
		SET workspace_id = EXCLUDED.workspace_id, state = EXCLUDED.state`,
		record.WorkspaceID, rk, record.ServiceID, record.Kind, record.Source,
		record.WindowStart.UTC().Truncate(time.Hour), record.State)
	return err
}

func validUsageSource(source string) bool {
	switch source {
	case UsageSourceInstance, UsageSourceBuild, UsageSourceStorage, UsageSourceHTTP,
		UsageSourceWebSocket, UsageSourceDirect, UsageSourcePostgres, UsageSourceKeyValue:
		return true
	default:
		return false
	}
}

func validUsageSourceState(state string) bool {
	return state == UsageSourceHealthy || state == UsageSourceDegraded || state == UsageSourceUnavailable
}

// RecordUsageSourceHealth records an explicit degraded/unavailable attempt
// when no usage row can be committed. A later retry overwrites the state for
// the same source-window and reopens a stream closed by inventory reconciliation.
func (s *PGStore) RecordUsageSourceHealth(ctx context.Context, records []UsageSourceRecord) error {
	if len(records) == 0 {
		return nil
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	for _, record := range records {
		if err := upsertUsageSourceObservation(ctx, tx, record); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// ReconcileUsageSourceStreams closes open streams whose resource has left the
// collector inventory. The observed health rows are retained for the rest of
// the month; only the stream's expected range becomes finite.
func (s *PGStore) ReconcileUsageSourceStreams(ctx context.Context, active []UsageResourceRef, through time.Time) error {
	encoded, err := json.Marshal(active)
	if err != nil {
		return err
	}
	_, err = s.Pool.Exec(ctx, `
		UPDATE usage_source_streams AS stream
		SET expected_through = $2
		WHERE expected_through IS NULL
		  AND NOT EXISTS (
			SELECT 1
			FROM jsonb_to_recordset($1::jsonb) AS active("resourceKind" text, "serviceId" text)
			WHERE active."resourceKind" = stream.resource_kind
			  AND active."serviceId" = stream.service_id
		  )`, string(encoded), through.UTC().Truncate(time.Hour))
	return err
}

type usageCoverageStream struct {
	ResourceKind    string
	ServiceID       string
	Kind            string
	Source          string
	ExpectedFrom    time.Time
	ExpectedThrough *time.Time
	Windows         map[time.Time]string
}

// CurrentUsageCoverage aggregates only explicit source-health evidence. It
// intentionally cannot infer health from legacy usage rows or monthly totals.
func (s *PGStore) CurrentUsageCoverage(ctx context.Context, workspaceID string, now time.Time) (UsageCoverage, error) {
	now = now.UTC()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	latestClosed := now.Truncate(time.Hour)
	rows, err := s.Pool.Query(ctx, `
		SELECT stream.resource_kind, stream.service_id, stream.kind, stream.source,
		       stream.expected_from, stream.expected_through,
		       health.window_start, health.state
		FROM usage_source_streams AS stream
		LEFT JOIN usage_source_health AS health
		  ON health.resource_kind = stream.resource_kind
		 AND health.service_id = stream.service_id
		 AND health.kind = stream.kind
		 AND health.source = stream.source
		 AND health.window_start >= $2
		 AND health.window_start < $3
		WHERE stream.workspace_id = $1
		  AND stream.expected_from < $3
		  AND (stream.expected_through IS NULL OR stream.expected_through > $2)
		ORDER BY stream.resource_kind, stream.service_id, stream.kind, stream.source,
		         health.window_start`,
		workspaceID, monthStart, latestClosed)
	if err != nil {
		return UsageCoverage{}, err
	}
	defer rows.Close()
	streamByKey := map[string]*usageCoverageStream{}
	var streamOrder []string
	for rows.Next() {
		var resourceKind, serviceID, kind, source string
		var expectedFrom time.Time
		var expectedThrough, window *time.Time
		var state *string
		if err := rows.Scan(&resourceKind, &serviceID, &kind, &source, &expectedFrom,
			&expectedThrough, &window, &state); err != nil {
			return UsageCoverage{}, err
		}
		key := resourceKind + "\x00" + serviceID + "\x00" + kind + "\x00" + source
		stream := streamByKey[key]
		if stream == nil {
			stream = &usageCoverageStream{
				ResourceKind: resourceKind, ServiceID: serviceID, Kind: kind,
				Source: source, ExpectedFrom: expectedFrom, ExpectedThrough: expectedThrough,
				Windows: map[time.Time]string{},
			}
			streamByKey[key] = stream
			streamOrder = append(streamOrder, key)
		}
		if window != nil && state != nil {
			stream.Windows[window.UTC().Truncate(time.Hour)] = *state
		}
	}
	if err := rows.Err(); err != nil {
		return UsageCoverage{}, err
	}
	var sandboxTotals bool
	if err := s.Pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM usage_hourly
			WHERE workspace_id = $1 AND resource_kind = 'sandbox'
			  AND window_start >= $2 AND window_start < $3
		)`, workspaceID, monthStart, latestClosed).Scan(&sandboxTotals); err != nil {
		return UsageCoverage{}, err
	}
	return aggregateUsageCoverage(streamByKey, streamOrder, monthStart, latestClosed, sandboxTotals), nil
}

// maxDegradedSources caps how many degraded source names a coverage answer
// names individually; the rest collapse into a trailing "other".
const maxDegradedSources = 7

// evaluateStreamCoverage walks one stream's expected window range and reports
// how far it is contiguously observed-healthy (observedThrough), how far it was
// expected to reach (target), and whether every window it does have is healthy.
//
// The watermark and the health scan are deliberately separate passes: the
// watermark must stop at the first gap or degraded window, while degradation
// *after* an earlier gap is still useful diagnostic evidence and must not be
// lost just because the watermark already stopped short of it.
func evaluateStreamCoverage(stream *usageCoverageStream, monthStart, latestClosed time.Time) (observedThrough, target time.Time, healthy bool) {
	expectedStart := stream.ExpectedFrom.UTC().Truncate(time.Hour)
	if expectedStart.Before(monthStart) {
		expectedStart = monthStart
	}
	target = latestClosed
	if stream.ExpectedThrough != nil && stream.ExpectedThrough.Before(target) {
		target = stream.ExpectedThrough.UTC().Truncate(time.Hour)
	}
	observedThrough = expectedStart
	for observedThrough.Before(target) {
		if state, exists := stream.Windows[observedThrough]; !exists || state != UsageSourceHealthy {
			break
		}
		observedThrough = observedThrough.Add(time.Hour)
	}
	healthy = true
	for _, state := range stream.Windows {
		if state != UsageSourceHealthy {
			healthy = false
			break
		}
	}
	return observedThrough, target, healthy
}

func aggregateUsageCoverage(streamByKey map[string]*usageCoverageStream, streamOrder []string, monthStart, latestClosed time.Time, sandboxTotals bool) UsageCoverage {
	if len(streamOrder) == 0 {
		return UsageCoverage{}
	}
	coverage := UsageCoverage{Known: true, Complete: true, Through: latestClosed}
	degraded := map[string]struct{}{}
	if sandboxTotals {
		coverage.Complete = false
		degraded[UsageSourceSandbox] = struct{}{}
	}
	activeStreams := 0
	for _, key := range streamOrder {
		stream := streamByKey[key]
		observedThrough, target, healthy := evaluateStreamCoverage(stream, monthStart, latestClosed)
		if !healthy {
			degraded[stream.Source] = struct{}{}
		}
		behind := observedThrough.Before(target)
		if behind || !healthy {
			coverage.Complete = false
		}
		// A stream with an open-ended expectation, or one still catching up, keeps
		// dragging `through` back to what it has actually observed.
		if stream.ExpectedThrough == nil || behind || !healthy {
			activeStreams++
			if observedThrough.Before(coverage.Through) {
				coverage.Through = observedThrough
			}
		}
	}
	if activeStreams == 0 {
		// Every stream ended. If their finite expected ranges are sound, the
		// workspace is a healthy empty current inventory through the last closed
		// hour; otherwise it remains partial but no stale stream drags `through`.
		coverage.Through = latestClosed
	}
	coverage.DegradedSources = slices.Sorted(maps.Keys(degraded))
	if len(coverage.DegradedSources) > maxDegradedSources {
		coverage.DegradedSources = append(coverage.DegradedSources[:maxDegradedSources], "other")
	}
	if coverage.Through.Before(monthStart) {
		coverage.Through = time.Time{}
	}
	return coverage
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
		WITH purged_health AS (
			DELETE FROM usage_source_health
			WHERE window_start < $1
		), purged_streams AS (
			DELETE FROM usage_source_streams
			WHERE expected_through IS NOT NULL AND expected_through <= $1
		), purged AS (
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
