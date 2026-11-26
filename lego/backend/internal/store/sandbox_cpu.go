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
	"math"
	"time"

	"github.com/jackc/pgx/v5"
)

// SandboxIntervalRetention is how long a closed running interval is kept after
// it ended: past the usage catch-up window and a billing cycle, so the pod that
// ran a charged hour can still be named when an invoice is questioned.
const SandboxIntervalRetention = 35 * 24 * time.Hour

// sandboxCPUMicros is the sub-unit the used-CPU meter carries between windows:
// millionths of a CPU-second.
const sandboxCPUMicros = 1_000_000

// SandboxCPUWindow is the next stretch of one running interval the used-CPU
// meter has to read: From..To lies inside one UTC hour and inside the interval,
// and has ended. StartedAt identifies the interval.
type SandboxCPUWindow struct {
	WorkspaceID string
	SandboxID   string
	PodName     string
	Tier        string
	StartedAt   time.Time
	From, To    time.Time
	// DeferredSince is since when this window has been waiting for a reading
	// (the interval is partial); nil for a window not yet tried.
	DeferredSince *time.Time
}

// SandboxCPUStats counts the used-CPU meter's work, for its gauges.
type SandboxCPUStats struct {
	// Pending intervals still have CPU to read; Deferred ones of those are
	// waiting for Prometheus (their usage is partial).
	Pending, Deferred int64
}

// syncSandboxInterval drives the sandbox's running interval from one phase
// sample, inside the cursor's transaction: a running sample with a pod name
// opens an interval (or continues the open one), and anything else, or another
// pod name, closes it at the sample's instant. A pod recreated while the sandbox
// stays running therefore ends one interval and starts the next at the same
// instant: no gap and no overlap. The CPU meter reads every interval; one of a
// sandbox not billed on use (an agent session's) records its CPU unbilled
// before AgentSandboxUsageBilledFrom (w6/m566).
func syncSandboxInterval(ctx context.Context, tx pgx.Tx, obs SandboxMeterObservation) error {
	at := obs.ObservedAt.UTC()
	running := obs.Phase == "running" && obs.PodName != ""
	var openPod string
	err := tx.QueryRow(ctx, `
		SELECT pod_name FROM sandbox_running_intervals
		WHERE workspace_id = $1 AND sandbox_id = $2 AND ended_at IS NULL
		FOR UPDATE`, obs.WorkspaceID, obs.SandboxID).Scan(&openPod)
	open := err == nil
	if err != nil && err != pgx.ErrNoRows {
		return err
	}
	if open && running && openPod == obs.PodName {
		return nil
	}
	if open {
		if err := endSandboxIntervalTx(ctx, tx, obs.WorkspaceID, obs.SandboxID, at); err != nil {
			return err
		}
	}
	if !running {
		return nil
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO sandbox_running_intervals
		    (workspace_id, sandbox_id, pod_name, tier, usage_billed, started_at, cpu_metered_through, cpu_settled)
		VALUES ($1, $2, $3, $4, $5, $6, $6, false)`,
		obs.WorkspaceID, obs.SandboxID, obs.PodName, obs.Tier, obs.UsageBilled, at)
	return err
}

// endSandboxIntervalTx closes the sandbox's open running interval at at (never
// before it started); none open is a no-op. An interval with no CPU left to
// read is settled here.
func endSandboxIntervalTx(ctx context.Context, tx pgx.Tx, workspaceID, sandboxID string, at time.Time) error {
	_, err := tx.Exec(ctx, `
		UPDATE sandbox_running_intervals
		SET ended_at = GREATEST($3, started_at),
		    cpu_settled = cpu_settled OR cpu_metered_through >= GREATEST($3, started_at)
		WHERE workspace_id = $1 AND sandbox_id = $2 AND ended_at IS NULL`,
		workspaceID, sandboxID, at.UTC())
	return err
}

// ClaimSandboxCPUWindows returns the next window of up to limit running
// intervals, the ones tried least recently first, and stamps them
// as tried now. A window is due once it ended before readyBefore: the hour
// closed or the interval did, and Prometheus has had time to scrape its last
// sample. The rotation keeps an interval Prometheus cannot answer for from
// starving the others.
func (s *PGStore) ClaimSandboxCPUWindows(ctx context.Context, now, readyBefore time.Time, limit int) ([]SandboxCPUWindow, error) {
	rows, err := s.Pool.Query(ctx, `
		WITH due AS (
			SELECT workspace_id, sandbox_id, started_at,
			       LEAST((date_trunc('hour', cpu_metered_through AT TIME ZONE 'UTC') + interval '1 hour') AT TIME ZONE 'UTC',
			             COALESCE(ended_at, 'infinity')) AS window_end
			FROM sandbox_running_intervals
			WHERE NOT cpu_settled
			  AND LEAST((date_trunc('hour', cpu_metered_through AT TIME ZONE 'UTC') + interval '1 hour') AT TIME ZONE 'UTC',
			            COALESCE(ended_at, 'infinity')) <= $2
			ORDER BY cpu_attempted_at NULLS FIRST, cpu_metered_through
			LIMIT $3
			FOR UPDATE SKIP LOCKED)
		UPDATE sandbox_running_intervals i SET cpu_attempted_at = $1
		FROM due
		WHERE i.workspace_id = due.workspace_id AND i.sandbox_id = due.sandbox_id AND i.started_at = due.started_at
		RETURNING i.workspace_id, i.sandbox_id, i.pod_name, i.tier, i.started_at, i.cpu_metered_through,
		          due.window_end, i.cpu_deferred_since`,
		now.UTC(), readyBefore.UTC(), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SandboxCPUWindow
	for rows.Next() {
		var w SandboxCPUWindow
		if err := rows.Scan(&w.WorkspaceID, &w.SandboxID, &w.PodName, &w.Tier, &w.StartedAt, &w.From, &w.To, &w.DeferredSince); err != nil {
			return nil, err
		}
		w.StartedAt, w.From, w.To = w.StartedAt.UTC(), w.From.UTC(), w.To.UTC()
		out = append(out, w)
	}
	return out, rows.Err()
}

// AccrueSandboxCPUWindow adds the CPU-seconds read for one window to the
// sandbox's sandbox_cpu_seconds row of the window's hour and advances the
// interval past it, in one transaction. The row of an agent session's sandbox
// is marked unbilled before AgentSandboxUsageBilledFrom, which its weighted
// meter charged for; from then it is billed and added to its session's total
// (w6/m566). It reports false, adding nothing, when
// the interval is no longer at the window's start: another replica read the
// same window first, so a replay or a race never counts it twice. Zero seconds
// is a reading too (an idle window); a window with no reading is deferred with
// DeferSandboxCPUWindow instead, never passed here as zero.
func (s *PGStore) AccrueSandboxCPUWindow(ctx context.Context, w SandboxCPUWindow, cpuSeconds float64) (bool, error) {
	if math.IsNaN(cpuSeconds) || math.IsInf(cpuSeconds, 0) || cpuSeconds < 0 || cpuSeconds > 1e9 || !w.From.Before(w.To) {
		return false, fmt.Errorf("invalid sandbox CPU window reading")
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	// The meter's per-sandbox lock, taken before the interval row as the
	// cursor's accrual takes it: the agent_session_bind_sandbox trigger seeds a
	// newly adopted sandbox's session totals under it, so a reading either
	// lands in that seed or updates the new binding, never neither.
	if err := lockSandboxMeter(ctx, tx, w.WorkspaceID, w.SandboxID); err != nil {
		return false, err
	}
	var through time.Time
	var remainder int64
	var usageBilled bool
	err = tx.QueryRow(ctx, `
		SELECT cpu_metered_through, cpu_remainder_micros, usage_billed
		FROM sandbox_running_intervals
		WHERE workspace_id = $1 AND sandbox_id = $2 AND started_at = $3 AND NOT cpu_settled
		FOR UPDATE`, w.WorkspaceID, w.SandboxID, w.StartedAt.UTC()).Scan(&through, &remainder, &usageBilled)
	if err == pgx.ErrNoRows || (err == nil && !through.Equal(w.From)) {
		return false, tx.Commit(ctx)
	}
	if err != nil {
		return false, err
	}
	micros := int64(math.Round(cpuSeconds*sandboxCPUMicros)) + remainder
	window := w.From.UTC().Truncate(time.Hour)
	chunk := sandboxUsageChunk{window: window, quantity: micros / sandboxCPUMicros, unbilled: s.sandboxUsageUnbilled(usageBilled, window)}
	if chunk.quantity > 0 {
		billed, err := addSandboxUsage(ctx, tx, w.WorkspaceID, w.SandboxID, UsageKindSandboxCPUSeconds, w.Tier, []sandboxUsageChunk{chunk})
		if err != nil {
			return false, err
		}
		if !usageBilled {
			if err := addAgentSessionSandboxUsage(ctx, tx, w.WorkspaceID, w.SandboxID, AgentSessionSandboxUsage{CPUSeconds: billed}); err != nil {
				return false, err
			}
		}
	}
	if _, err := tx.Exec(ctx, `
		UPDATE sandbox_running_intervals
		SET cpu_metered_through = $4, cpu_remainder_micros = $5, cpu_deferred_since = NULL,
		    cpu_settled = ended_at IS NOT NULL AND $4 >= ended_at
		WHERE workspace_id = $1 AND sandbox_id = $2 AND started_at = $3`,
		w.WorkspaceID, w.SandboxID, w.StartedAt.UTC(), w.To.UTC(), micros%sandboxCPUMicros); err != nil {
		return false, err
	}
	return true, tx.Commit(ctx)
}

// DeferSandboxCPUWindow records that a window could not be read yet: the
// interval stays where it is and is marked partial from the first failed try.
func (s *PGStore) DeferSandboxCPUWindow(ctx context.Context, w SandboxCPUWindow, now time.Time) error {
	_, err := s.Pool.Exec(ctx, `
		UPDATE sandbox_running_intervals
		SET cpu_deferred_since = COALESCE(cpu_deferred_since, $5)
		WHERE workspace_id = $1 AND sandbox_id = $2 AND started_at = $3 AND cpu_metered_through = $4 AND NOT cpu_settled`,
		w.WorkspaceID, w.SandboxID, w.StartedAt.UTC(), w.From.UTC(), now.UTC())
	return err
}

// SandboxCPUStats counts the intervals with CPU left to read and how many of
// them are waiting for Prometheus. Both read the meter's partial index.
func (s *PGStore) SandboxCPUStats(ctx context.Context) (SandboxCPUStats, error) {
	var st SandboxCPUStats
	err := s.Pool.QueryRow(ctx, `
		SELECT count(*), count(*) FILTER (WHERE cpu_deferred_since IS NOT NULL)
		FROM sandbox_running_intervals WHERE NOT cpu_settled`).Scan(&st.Pending, &st.Deferred)
	return st, err
}

// PruneSandboxIntervals deletes up to limit running intervals that ended more
// than SandboxIntervalRetention ago. Without it the table grows with every
// sandbox ever run.
func (s *PGStore) PruneSandboxIntervals(ctx context.Context, now time.Time, limit int) (int64, error) {
	tag, err := s.Pool.Exec(ctx, `
		DELETE FROM sandbox_running_intervals i USING (
			SELECT workspace_id, sandbox_id, started_at FROM sandbox_running_intervals
			WHERE ended_at IS NOT NULL AND ended_at < $1
			ORDER BY ended_at LIMIT $2
		) gone
		WHERE i.workspace_id = gone.workspace_id AND i.sandbox_id = gone.sandbox_id AND i.started_at = gone.started_at`,
		now.UTC().Add(-SandboxIntervalRetention), limit)
	return tag.RowsAffected(), err
}
