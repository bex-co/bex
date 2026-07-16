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
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	ids "github.com/bex-co/bex/lego/backend/internal/id"
)

// Job is a row of `jobs` — a one-off command run in a service's container
// (Render's /services/{id}/jobs). ServiceName is the bex App name (the
// workspace-scoped identifier the REST surface uses); TenantID scopes
// visibility. Status progresses pending → running → succeeded|failed|canceled.
type Job struct {
	ID           string     `json:"id"`
	ServiceName  string     `json:"serviceName"`
	TenantID     string     `json:"tenantId"`
	StartCommand string     `json:"startCommand"`
	PlanID       string     `json:"planId"`
	Status       string     `json:"status"`
	CreatedAt    time.Time  `json:"createdAt"`
	StartedAt    *time.Time `json:"startedAt,omitempty"`
	FinishedAt   *time.Time `json:"finishedAt,omitempty"`
}

// Job status constants — the Render job status enum.
const (
	JobPending   = "pending"
	JobRunning   = "running"
	JobSucceeded = "succeeded"
	JobFailed    = "failed"
	JobCanceled  = "canceled"
)

const jobColumns = `id, service_name, tenant_id, start_command, plan_id, status, created_at, started_at, finished_at`

func scanJob(row pgx.Row) (Job, error) {
	var j Job
	err := row.Scan(&j.ID, &j.ServiceName, &j.TenantID, &j.StartCommand, &j.PlanID, &j.Status, &j.CreatedAt, &j.StartedAt, &j.FinishedAt)
	return j, err
}

func (s *PGStore) CreateJob(ctx context.Context, serviceName, tenantID, startCommand, planID string) (Job, error) {
	j := Job{
		ID:           ids.New(ids.Job),
		ServiceName:  serviceName,
		TenantID:     tenantID,
		StartCommand: startCommand,
		PlanID:       planID,
		Status:       JobPending,
	}
	err := s.Pool.QueryRow(ctx,
		`INSERT INTO jobs (id, service_name, tenant_id, start_command, plan_id, status) VALUES ($1, $2, $3, $4, $5, $6) RETURNING created_at`,
		j.ID, j.ServiceName, j.TenantID, j.StartCommand, j.PlanID, j.Status,
	).Scan(&j.CreatedAt)
	if err != nil {
		return Job{}, classify("job", err)
	}
	return j, nil
}

// JobListFilter narrows ListJobs. Limit 0 means no cap (returns all).
type JobListFilter struct {
	Statuses       []string
	CreatedBefore  time.Time
	CreatedAfter   time.Time
	StartedBefore  time.Time
	StartedAfter   time.Time
	FinishedBefore time.Time
	FinishedAfter  time.Time
	Cursor         string
	Limit          int
}

func (s *PGStore) ListJobs(ctx context.Context, serviceName, tenantID string, filter JobListFilter) ([]Job, error) {
	query := `SELECT ` + jobColumns + ` FROM jobs WHERE service_name = $1 AND tenant_id = $2`
	args := []any{serviceName, tenantID}

	if len(filter.Statuses) > 0 {
		args = append(args, filter.Statuses)
		query += fmt.Sprintf(" AND status = ANY($%d)", len(args))
	}
	if !filter.CreatedBefore.IsZero() {
		args = append(args, filter.CreatedBefore)
		query += fmt.Sprintf(" AND created_at < $%d", len(args))
	}
	if !filter.CreatedAfter.IsZero() {
		args = append(args, filter.CreatedAfter)
		query += fmt.Sprintf(" AND created_at > $%d", len(args))
	}
	if !filter.StartedBefore.IsZero() {
		args = append(args, filter.StartedBefore)
		query += fmt.Sprintf(" AND started_at < $%d", len(args))
	}
	if !filter.StartedAfter.IsZero() {
		args = append(args, filter.StartedAfter)
		query += fmt.Sprintf(" AND started_at > $%d", len(args))
	}
	if !filter.FinishedBefore.IsZero() {
		args = append(args, filter.FinishedBefore)
		query += fmt.Sprintf(" AND finished_at < $%d", len(args))
	}
	if !filter.FinishedAfter.IsZero() {
		args = append(args, filter.FinishedAfter)
		query += fmt.Sprintf(" AND finished_at > $%d", len(args))
	}

	query, args = pageKeyset(query, args, "jobs", "created_at", filter.Cursor, clampPageLimit(filter.Limit), false)

	rows, err := s.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Job
	for rows.Next() {
		j, err := scanJob(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, j)
	}
	return out, rows.Err()
}

func (s *PGStore) GetJob(ctx context.Context, serviceName, tenantID, jobID string) (Job, error) {
	j, err := scanJob(s.Pool.QueryRow(ctx,
		`SELECT `+jobColumns+` FROM jobs WHERE id = $1 AND service_name = $2 AND tenant_id = $3`,
		jobID, serviceName, tenantID,
	))
	if err != nil {
		return Job{}, classify("job", err)
	}
	return j, nil
}

// UpdateJobStatus transitions a job to the given terminal or running status,
// stamping the appropriate timestamp. The UPDATE is a no-op on already-terminal
// rows (prevents double-close races) — callers check the returned row's status.
func (s *PGStore) UpdateJobStatus(ctx context.Context, jobID, status string) (Job, error) {
	var q string
	var args []any
	switch status {
	case JobRunning:
		q = `UPDATE jobs SET status = $2, started_at = now() WHERE id = $1 AND status = $3 RETURNING ` + jobColumns
		args = []any{jobID, status, JobPending}
	case JobSucceeded, JobFailed, JobCanceled:
		q = `UPDATE jobs SET status = $2, finished_at = now() WHERE id = $1 AND status NOT IN ('succeeded','failed','canceled') RETURNING ` + jobColumns
		args = []any{jobID, status}
	default:
		q = `UPDATE jobs SET status = $2 WHERE id = $1 RETURNING ` + jobColumns
		args = []any{jobID, status}
	}

	j, err := scanJob(s.Pool.QueryRow(ctx, q, args...))
	if err != nil {
		return Job{}, classify("job", err)
	}
	return j, nil
}
