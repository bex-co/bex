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
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// Service disk usage events (w2/m255): Render's service_disk_usage_high and
// service_disk_usage_recovered, with their typed details.
const (
	EventFactServiceDiskUsageHigh      ServiceEventFactType = "service_disk_usage_high"
	EventFactServiceDiskUsageRecovered ServiceEventFactType = "service_disk_usage_recovered"
)

// DiskUsageDetails is a disk-usage event's Render details.
type DiskUsageDetails struct {
	DiskID           string
	DiskName         string
	MountPath        string
	UsagePercent     float64
	ThresholdPercent float64
}

// DiskUsageState is the sweeper's view of one disk: over the threshold or not,
// and a crossing seen once but not yet confirmed.
type DiskUsageState struct {
	High         bool
	PendingHigh  *bool
	PendingSince *time.Time
}

func isDiskUsageFact(t ServiceEventFactType) bool {
	return t == EventFactServiceDiskUsageHigh || t == EventFactServiceDiskUsageRecovered
}

func validateDiskUsageDetails(f ServiceEventFact) error {
	if isDiskUsageFact(f.Type) != (f.DiskUsage != nil) {
		return fmt.Errorf("service event fact %q: disk usage details mismatch", f.Type)
	}
	if d := f.DiskUsage; d != nil && (d.DiskID == "" || d.UsagePercent < 0 || d.ThresholdPercent <= 0 || d.ThresholdPercent > 100) {
		return fmt.Errorf("service event fact %q: invalid disk usage details", f.Type)
	}
	return nil
}

const insertDiskUsageDetailsSQL = `
INSERT INTO service_event_disk_usage (source_key, disk_id, disk_name, mount_path, usage_percent, threshold_percent)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (source_key) DO NOTHING`

// GetDiskUsageState reads a disk's state; a disk never evaluated reads as
// under the threshold with nothing pending.
func (s *PGStore) GetDiskUsageState(ctx context.Context, diskID string) (DiskUsageState, error) {
	var st DiskUsageState
	err := s.Pool.QueryRow(ctx,
		`SELECT high, pending_high, pending_since FROM service_disk_usage_state WHERE disk_id = $1`, diskID).
		Scan(&st.High, &st.PendingHigh, &st.PendingSince)
	if errors.Is(err, pgx.ErrNoRows) {
		return DiskUsageState{}, nil
	}
	return st, err
}

// SetDiskUsagePending records (or, with nil, clears) a crossing seen once. An
// existing pending crossing in the same direction keeps its first time.
func (s *PGStore) SetDiskUsagePending(ctx context.Context, diskID string, pendingHigh *bool, at time.Time) error {
	_, err := s.Pool.Exec(ctx, `
		INSERT INTO service_disk_usage_state AS st (disk_id, pending_high, pending_since, updated_at)
		VALUES ($1, $2, CASE WHEN $2::boolean IS NULL THEN NULL ELSE $3::timestamptz END, now())
		ON CONFLICT (disk_id) DO UPDATE SET
			pending_since = CASE
				WHEN EXCLUDED.pending_high IS NULL THEN NULL
				WHEN st.pending_high IS NOT DISTINCT FROM EXCLUDED.pending_high THEN st.pending_since
				ELSE EXCLUDED.pending_since
			END,
			pending_high = EXCLUDED.pending_high,
			updated_at = now()`, diskID, pendingHigh, at)
	return err
}

// RecordDiskUsageCrossing confirms the disk's pending crossing: it flips the
// state and appends the event fact with its details, in one transaction. It
// reports false (writing nothing) when no such crossing is pending, e.g.
// another replica already confirmed it.
func (s *PGStore) RecordDiskUsageCrossing(ctx context.Context, fact ServiceEventFact) (bool, error) {
	if err := validateServiceEventFact(fact); err != nil {
		return false, err
	}
	if fact.DiskUsage == nil {
		return false, fmt.Errorf("disk usage crossing: %w", ErrInvalid)
	}
	high := fact.Type == EventFactServiceDiskUsageHigh
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	// Only the crossing that is pending flips the state: a replica that lost
	// the race, or a crossing nobody sighted, writes nothing.
	tag, err := tx.Exec(ctx, `
		UPDATE service_disk_usage_state SET high = $2, pending_high = NULL, pending_since = NULL, updated_at = now()
		WHERE disk_id = $1 AND high <> $2 AND pending_high = $2`, fact.DiskUsage.DiskID, high)
	if err != nil {
		return false, err
	}
	if tag.RowsAffected() == 0 {
		return false, nil
	}
	if _, err := tx.Exec(ctx, insertServiceEventFactSQL, fact.insertArgs()...); err != nil {
		return false, classify("service event fact", err)
	}
	d := fact.DiskUsage
	if _, err := tx.Exec(ctx, insertDiskUsageDetailsSQL,
		fact.SourceKey, d.DiskID, d.DiskName, d.MountPath, d.UsagePercent, d.ThresholdPercent); err != nil {
		return false, err
	}
	return true, tx.Commit(ctx)
}

// attachDiskUsage fills DiskUsage on a page's disk-usage fact rows from the
// side table in one read, so the feed's UNION projection stays as it is.
func (s *PGStore) attachDiskUsage(ctx context.Context, rows []ServiceEventRow) error {
	at := map[string][]int{}
	for i, r := range rows {
		if r.Source == EventSourceFact && isDiskUsageFact(ServiceEventFactType(r.FactType)) {
			key := factSourceKey(r.Key)
			at[key] = append(at[key], i)
		}
	}
	if len(at) == 0 {
		return nil
	}
	keys := make([]string, 0, len(at))
	for k := range at {
		keys = append(keys, k)
	}
	found, err := s.Pool.Query(ctx, `
		SELECT source_key, disk_id, disk_name, mount_path, usage_percent, threshold_percent
		FROM service_event_disk_usage WHERE source_key = ANY($1)`, keys)
	if err != nil {
		return fmt.Errorf("read disk usage details: %w", err)
	}
	defer found.Close()
	for found.Next() {
		var key string
		var d DiskUsageDetails
		if err := found.Scan(&key, &d.DiskID, &d.DiskName, &d.MountPath, &d.UsagePercent, &d.ThresholdPercent); err != nil {
			return fmt.Errorf("read disk usage details: %w", err)
		}
		for _, i := range at[key] {
			details := d
			rows[i].DiskUsage = &details
		}
	}
	return found.Err()
}

// factSourceKey is a fact row's source key from its feed key ("fact:<key>").
func factSourceKey(feedKey string) string { return strings.TrimPrefix(feedKey, "fact:") }
