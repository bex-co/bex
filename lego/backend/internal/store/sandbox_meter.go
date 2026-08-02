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
)

// SandboxTenantKey is one workspace's OpenSandbox tenant credential. It is
// used only by the internal metering poller and must never be logged.
type SandboxTenantKey struct {
	WorkspaceID string
	APIKey      string
}

// SandboxMeterObservation is one authoritative OpenSandbox phase sample.
// WeightMilli is the sandbox shape expressed as milli-vCPU equivalents, with
// memory folded in at the documented AgentCore CPU:memory price ratio.
type SandboxMeterObservation struct {
	WorkspaceID string
	SandboxID   string
	Phase       string
	Tier        string
	WeightMilli int64
	ObservedAt  time.Time
}

// ListSandboxTenantKeys lets the internal poller enumerate tenant-scoped
// OpenSandbox views without weakening the public workspace isolation seam.
func (s *PGStore) ListSandboxTenantKeys(ctx context.Context) ([]SandboxTenantKey, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT workspace_id, api_key
		FROM sandbox_tenant_keys
		ORDER BY workspace_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SandboxTenantKey
	for rows.Next() {
		var key SandboxTenantKey
		if err := rows.Scan(&key.WorkspaceID, &key.APIKey); err != nil {
			return nil, err
		}
		out = append(out, key)
	}
	return out, rows.Err()
}

type sandboxMeterState struct {
	phase, tier string
	weight      int64
	observedAt  time.Time
	remainder   int64
}

// ObserveSandboxMeter atomically advances one sandbox cursor and accrues the
// non-overlapping interval since its previous observation. Only an interval
// whose previous phase was running is charged; creating/resuming/suspended/
// errored/terminated intervals are zero. Replaying or racing an older sample is
// a no-op, which is the restart/retry no-double-count guarantee.
func (s *PGStore) ObserveSandboxMeter(ctx context.Context, obs SandboxMeterObservation) error {
	if obs.WorkspaceID == "" || obs.SandboxID == "" || obs.Phase == "" || obs.WeightMilli <= 0 || obs.WeightMilli > 1_000_000 || obs.ObservedAt.IsZero() {
		return fmt.Errorf("invalid sandbox meter observation")
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	// A row lock cannot serialize two concurrent first observations because the
	// row does not exist yet. This transaction-scoped advisory lock closes that
	// one gap without introducing a global meter lock.
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, obs.WorkspaceID+"|"+obs.SandboxID); err != nil {
		return err
	}

	var prior sandboxMeterState
	err = tx.QueryRow(ctx, `
		SELECT phase, tier, weight_milli, observed_at, remainder_nanos
		FROM sandbox_meter_states
		WHERE workspace_id = $1 AND sandbox_id = $2
		FOR UPDATE`, obs.WorkspaceID, obs.SandboxID,
	).Scan(&prior.phase, &prior.tier, &prior.weight, &prior.observedAt, &prior.remainder)
	if err == pgx.ErrNoRows {
		_, err = tx.Exec(ctx, `
			INSERT INTO sandbox_meter_states
			    (workspace_id, sandbox_id, phase, tier, weight_milli, observed_at)
			VALUES ($1, $2, $3, $4, $5, $6)`,
			obs.WorkspaceID, obs.SandboxID, obs.Phase, obs.Tier, obs.WeightMilli, obs.ObservedAt.UTC())
		if err != nil {
			return err
		}
		return tx.Commit(ctx)
	}
	if err != nil {
		return err
	}
	if !obs.ObservedAt.After(prior.observedAt) {
		return tx.Commit(ctx)
	}

	remainder := prior.remainder
	if prior.phase == "running" {
		remainder, err = accrueSandboxInterval(ctx, tx, obs.WorkspaceID, obs.SandboxID,
			prior.tier, prior.weight, prior.observedAt, obs.ObservedAt.UTC(), remainder)
		if err != nil {
			return err
		}
	}
	_, err = tx.Exec(ctx, `
		UPDATE sandbox_meter_states
		SET phase = $3, tier = $4, weight_milli = $5, observed_at = $6, remainder_nanos = $7
		WHERE workspace_id = $1 AND sandbox_id = $2`,
		obs.WorkspaceID, obs.SandboxID, obs.Phase, obs.Tier, obs.WeightMilli, obs.ObservedAt.UTC(), remainder)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// TerminateMissingSandboxMeters closes active cursors absent from one complete
// tenant-scoped OpenSandbox list. The caller invokes it only after a successful
// list, so an upstream outage never turns running sandboxes into terminated
// ones. The last known running interval accrues through observedAt.
func (s *PGStore) TerminateMissingSandboxMeters(ctx context.Context, workspaceID string, seen []string, observedAt time.Time) error {
	rows, err := s.Pool.Query(ctx, `
		SELECT sandbox_id
		FROM sandbox_meter_states
		WHERE workspace_id = $1 AND phase <> 'terminated'
		  AND NOT (sandbox_id = ANY($2::text[]))`, workspaceID, seen)
	if err != nil {
		return err
	}
	var missing []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		missing = append(missing, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	for _, id := range missing {
		if err := s.terminateSandboxMeter(ctx, workspaceID, id, observedAt); err != nil {
			return err
		}
	}
	return nil
}

func (s *PGStore) terminateSandboxMeter(ctx context.Context, workspaceID, sandboxID string, observedAt time.Time) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var prior sandboxMeterState
	err = tx.QueryRow(ctx, `
		SELECT phase, tier, weight_milli, observed_at, remainder_nanos
		FROM sandbox_meter_states
		WHERE workspace_id = $1 AND sandbox_id = $2
		FOR UPDATE`, workspaceID, sandboxID,
	).Scan(&prior.phase, &prior.tier, &prior.weight, &prior.observedAt, &prior.remainder)
	if err == pgx.ErrNoRows || !observedAt.After(prior.observedAt) || prior.phase == "terminated" {
		return tx.Commit(ctx)
	}
	if err != nil {
		return err
	}
	remainder := prior.remainder
	if prior.phase == "running" {
		remainder, err = accrueSandboxInterval(ctx, tx, workspaceID, sandboxID,
			prior.tier, prior.weight, prior.observedAt, observedAt.UTC(), remainder)
		if err != nil {
			return err
		}
	}
	_, err = tx.Exec(ctx, `
		UPDATE sandbox_meter_states
		SET phase = 'terminated', observed_at = $3, remainder_nanos = $4
		WHERE workspace_id = $1 AND sandbox_id = $2`, workspaceID, sandboxID, observedAt.UTC(), remainder)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// accrueSandboxInterval splits an interval at UTC hour boundaries. Quantity is
// milli-vCPU-equivalent seconds; remainder carries sub-unit nanoseconds across
// samples so a fast poll cadence cannot systematically round usage down.
type sandboxUsageChunk struct {
	window   time.Time
	quantity int64
}

func sandboxIntervalChunks(weight int64, start, end time.Time, remainder int64) ([]sandboxUsageChunk, int64) {
	var chunks []sandboxUsageChunk
	for cursor := start.UTC(); cursor.Before(end); {
		window := cursor.Truncate(time.Hour)
		segmentEnd := window.Add(time.Hour)
		if end.Before(segmentEnd) {
			segmentEnd = end
		}
		numerator := segmentEnd.Sub(cursor).Nanoseconds()*weight + remainder
		quantity := numerator / int64(time.Second)
		remainder = numerator % int64(time.Second)
		if quantity > 0 {
			chunks = append(chunks, sandboxUsageChunk{window: window, quantity: quantity})
		}
		cursor = segmentEnd
	}
	return chunks, remainder
}

func accrueSandboxInterval(ctx context.Context, tx pgx.Tx, workspaceID, sandboxID, tier string, weight int64, start, end time.Time, remainder int64) (int64, error) {
	chunks, remainder := sandboxIntervalChunks(weight, start, end, remainder)
	for _, chunk := range chunks {
		_, err := tx.Exec(ctx, `
				INSERT INTO usage_hourly
				    (workspace_id, service_id, kind, tier, resource_kind, window_start, quantity)
				VALUES ($1, $2, $3, $4, $5, $6, $7)
				ON CONFLICT (resource_kind, service_id, kind, tier, window_start)
				DO UPDATE SET quantity = usage_hourly.quantity + EXCLUDED.quantity`,
			workspaceID, sandboxID, UsageKindSandboxComputeSeconds, tier,
			ResourceKindSandbox, chunk.window, chunk.quantity)
		if err != nil {
			return remainder, err
		}
	}
	return remainder, nil
}
