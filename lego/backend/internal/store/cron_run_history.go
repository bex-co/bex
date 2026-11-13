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

// CronRunRecord is one finished cron run rebuilt from its durable
// cron_job_run_started/_ended facts (w3/m366). App.status.runs keeps only the
// operator's ten newest runs; these records are the history past that window.
type CronRunRecord struct {
	ID   string // the run's public crr- id
	Name string // the backing Job's name
	// StartedAt is zero for a run canceled before it started.
	StartedAt  time.Time
	FinishedAt time.Time
	// Status is EventStatusSucceeded, EventStatusFailed or EventStatusCanceled.
	Status string
}

// CronRunHistory reads an App's finished cron runs from their facts. Only runs
// with a recorded outcome are returned: a run started but never seen ending,
// once it has left the status window, has no outcome bex can state.
type CronRunHistory interface {
	// ListCronRunRecords returns up to limit runs, newest first, skipping the
	// exclude ids (the runs status still carries). A non-empty after is a run id:
	// the page starts past it, and an unknown one yields no runs.
	ListCronRunRecords(ctx context.Context, appID, after string, exclude []string, limit int) ([]CronRunRecord, error)
	// CronRunRecord finds one run by id; ok is false when it has no outcome.
	CronRunRecord(ctx context.Context, appID, runID string) (CronRunRecord, bool, error)
}

// Every finished run has exactly one ended fact, so a run is one row of
// those, ordered by (at, source_key): the order service_event_facts_app_at_idx
// holds and, for a cron job (never concurrent), the order its runs started in.
// Its start, when it had one, joins by primary key on the sibling source key.
// No anti-join: the planner turned one into a scan of the whole table.
const cronRunRecordSQL = `
SELECT e.run_id, e.source_key, s.at, e.at, e.status
FROM service_event_facts e
LEFT JOIN service_event_facts s
  ON s.source_key = left(e.source_key, -length(':` + cronRunEnded + `')) || ':` + cronRunStarted + `'
WHERE e.app_id = $1
  AND e.fact_type = 'cron_job_run_ended'
  AND e.run_id <> ''`

// Two shapes rather than "$2 IS NULL OR …", which a generic plan turns into a
// filter over every older fact instead of an index condition.
const (
	firstCronRunRecordsSQL = cronRunRecordSQL + `
  AND NOT (e.run_id = ANY($2::text[]))
ORDER BY e.at DESC, e.source_key DESC
LIMIT $3`
	laterCronRunRecordsSQL = cronRunRecordSQL + `
  AND (e.at, e.source_key) < ($2::timestamptz, $3::text)
  AND NOT (e.run_id = ANY($4::text[]))
ORDER BY e.at DESC, e.source_key DESC
LIMIT $5`
)

const cronRunRecordByIDSQL = cronRunRecordSQL + `
  AND e.run_id = $2
LIMIT 1`

// cronRunPositionSQL finds where a page past a run starts: its ended fact, or
// the start of a run evicted without one. run_id <> ” lets a generic plan use
// the partial index.
const cronRunPositionSQL = `
SELECT a.at, a.source_key
FROM service_event_facts a
WHERE a.app_id = $1 AND a.run_id = $2 AND a.run_id <> ''
  AND a.fact_type IN ('cron_job_run_started', 'cron_job_run_ended')
ORDER BY (a.fact_type = 'cron_job_run_ended') DESC
LIMIT 1`

func (s *PGStore) ListCronRunRecords(ctx context.Context, appID, after string, exclude []string, limit int) ([]CronRunRecord, error) {
	if limit < 1 {
		return nil, nil
	}
	if exclude == nil {
		exclude = []string{} // = ANY(NULL) is NULL, which would exclude every run
	}
	query, args := firstCronRunRecordsSQL, []any{appID, exclude, limit}
	if after != "" {
		var (
			at  time.Time
			key string
		)
		err := s.Pool.QueryRow(ctx, cronRunPositionSQL, appID, after).Scan(&at, &key)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		if err != nil {
			return nil, fmt.Errorf("cron run cursor %s: %w", after, err)
		}
		query, args = laterCronRunRecordsSQL, []any{appID, at, key, exclude, limit}
	}
	rows, err := s.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list cron runs of %s: %w", appID, err)
	}
	defer rows.Close()
	var out []CronRunRecord
	for rows.Next() {
		rec, err := scanCronRunRecord(rows, appID)
		if err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	return out, rows.Err()
}

func (s *PGStore) CronRunRecord(ctx context.Context, appID, runID string) (CronRunRecord, bool, error) {
	rec, err := scanCronRunRecord(s.Pool.QueryRow(ctx, cronRunRecordByIDSQL, appID, runID), appID)
	if errors.Is(err, pgx.ErrNoRows) {
		return CronRunRecord{}, false, nil
	}
	if err != nil {
		return CronRunRecord{}, false, fmt.Errorf("cron run %s of %s: %w", runID, appID, err)
	}
	return rec, true, nil
}

func scanCronRunRecord(row pgx.Row, appID string) (CronRunRecord, error) {
	var (
		rec       CronRunRecord
		key       string
		startedAt *time.Time
	)
	if err := row.Scan(&rec.ID, &key, &startedAt, &rec.FinishedAt, &rec.Status); err != nil {
		return CronRunRecord{}, err
	}
	if startedAt != nil {
		rec.StartedAt = startedAt.UTC()
	}
	rec.FinishedAt = rec.FinishedAt.UTC()
	rec.Name = strings.TrimSuffix(strings.TrimPrefix(key, "cron:"+appID+":"), ":"+cronRunEnded)
	return rec, nil
}
