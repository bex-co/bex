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
	// MemoryMiB is the sandbox's allocated memory, for the memory meter
	// (UsageKindSandboxMemoryGBSeconds). 0 means unknown: a sandbox created
	// before the shape was stamped is not memory-metered rather than guessed.
	MemoryMiB int64
	// StorageRef is where the registry holds this sandbox's suspend image: the
	// substrate sandbox id, which differs from SandboxID (the tenant-facing one).
	// Empty opts the sandbox out of the storage meter (agent-session sandboxes,
	// whose cost is the session's, ADR047 D6).
	StorageRef string
	// Polled marks a sample from the authoritative tenant-scoped list. Only a
	// polled `suspended` opens the sandbox's storage object: a request-path
	// `suspended` is recorded when the pause is accepted, before its image commit
	// has succeeded and while the registry holds no image to size.
	Polled     bool
	ObservedAt time.Time
}

// retainsImage reports whether a phase holds a suspend image the storage meter
// charges for. `suspending` is neutral: the image is not committed yet (or, on
// the way back, the sandbox has not resumed).
func retainsImage(phase string) bool { return phase == "suspended" || phase == "suspending" }

// syncSandboxStorage drives the storage object from a phase transition, inside
// the cursor's transaction: a polled `suspended` opens it (idempotently), and
// leaving suspended/suspending for anything else closes it.
func syncSandboxStorage(ctx context.Context, tx pgx.Tx, obs SandboxMeterObservation, priorPhase string) error {
	if obs.Polled && obs.Phase == "suspended" && obs.StorageRef != "" {
		return startSandboxStorageTx(ctx, tx, obs.WorkspaceID, obs.SandboxID, StorageKindRootfs, obs.StorageRef, obs.ObservedAt)
	}
	if retainsImage(priorPhase) && !retainsImage(obs.Phase) {
		if err := endSandboxStorageTx(ctx, tx, obs.WorkspaceID, obs.SandboxID, obs.ObservedAt); err != nil {
			return err
		}
	}
	if obs.Phase == "terminated" {
		return queueSandboxImageReapTx(ctx, tx, obs.WorkspaceID, obs.SandboxID)
	}
	return nil
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
	phase, tier     string
	weight          int64
	memoryMiB       int64
	observedAt      time.Time
	remainder       int64
	memoryRemainder int64
}

// loadSandboxMeterState reads and row-locks one sandbox's cursor; found is
// false for a sandbox the meter has not seen.
func loadSandboxMeterState(ctx context.Context, tx pgx.Tx, workspaceID, sandboxID string) (st sandboxMeterState, found bool, err error) {
	err = tx.QueryRow(ctx, `
		SELECT phase, tier, weight_milli, memory_mib, observed_at, remainder_nanos, memory_remainder
		FROM sandbox_meter_states
		WHERE workspace_id = $1 AND sandbox_id = $2
		FOR UPDATE`, workspaceID, sandboxID,
	).Scan(&st.phase, &st.tier, &st.weight, &st.memoryMiB, &st.observedAt, &st.remainder, &st.memoryRemainder)
	if err == pgx.ErrNoRows {
		return st, false, nil
	}
	return st, err == nil, err
}

// ObserveSandboxMeter atomically advances one sandbox cursor and accrues the
// non-overlapping interval since its previous observation. Only an interval
// whose previous phase was running is charged; creating/resuming/suspended/
// errored/terminated intervals are zero. Replaying or racing an older sample is
// a no-op, which is the restart/retry no-double-count guarantee.
func (s *PGStore) ObserveSandboxMeter(ctx context.Context, obs SandboxMeterObservation) error {
	if obs.WorkspaceID == "" || obs.SandboxID == "" || obs.Phase == "" || obs.WeightMilli <= 0 || obs.WeightMilli > 1_000_000 ||
		obs.MemoryMiB < 0 || obs.MemoryMiB > 1_000_000 || obs.ObservedAt.IsZero() {
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
	if err := lockSandboxMeter(ctx, tx, obs.WorkspaceID, obs.SandboxID); err != nil {
		return err
	}

	prior, found, err := loadSandboxMeterState(ctx, tx, obs.WorkspaceID, obs.SandboxID)
	if err != nil {
		return err
	}
	if !found {
		_, err = tx.Exec(ctx, `
			INSERT INTO sandbox_meter_states
			    (workspace_id, sandbox_id, phase, tier, weight_milli, memory_mib, observed_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			obs.WorkspaceID, obs.SandboxID, obs.Phase, obs.Tier, obs.WeightMilli, obs.MemoryMiB, obs.ObservedAt.UTC())
		if err != nil {
			return err
		}
		if err := syncSandboxStorage(ctx, tx, obs, ""); err != nil {
			return err
		}
		return tx.Commit(ctx)
	}
	if !obs.ObservedAt.After(prior.observedAt) {
		return tx.Commit(ctx)
	}

	if prior.phase == "running" {
		if err := accrueSandboxInterval(ctx, tx, obs.WorkspaceID, obs.SandboxID, &prior, obs.ObservedAt.UTC()); err != nil {
			return err
		}
	}
	_, err = tx.Exec(ctx, `
		UPDATE sandbox_meter_states
		SET phase = $3, tier = $4, weight_milli = $5, memory_mib = $6, observed_at = $7,
		    remainder_nanos = $8, memory_remainder = $9
		WHERE workspace_id = $1 AND sandbox_id = $2`,
		obs.WorkspaceID, obs.SandboxID, obs.Phase, obs.Tier, obs.WeightMilli, obs.MemoryMiB, obs.ObservedAt.UTC(),
		prior.remainder, prior.memoryRemainder)
	if err != nil {
		return err
	}
	if err := syncSandboxStorage(ctx, tx, obs, prior.phase); err != nil {
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
	if err := lockSandboxMeter(ctx, tx, workspaceID, sandboxID); err != nil {
		return err
	}
	prior, found, err := loadSandboxMeterState(ctx, tx, workspaceID, sandboxID)
	if err != nil {
		return err
	}
	if !found || !observedAt.After(prior.observedAt) || prior.phase == "terminated" {
		return tx.Commit(ctx)
	}
	if prior.phase == "running" {
		if err := accrueSandboxInterval(ctx, tx, workspaceID, sandboxID, &prior, observedAt.UTC()); err != nil {
			return err
		}
	}
	_, err = tx.Exec(ctx, `
		UPDATE sandbox_meter_states
		SET phase = 'terminated', observed_at = $3, remainder_nanos = $4, memory_remainder = $5
		WHERE workspace_id = $1 AND sandbox_id = $2`,
		workspaceID, sandboxID, observedAt.UTC(), prior.remainder, prior.memoryRemainder)
	if err != nil {
		return err
	}
	// A sandbox that vanished (terminated, or expired at pause + 30 days) no
	// longer retains its image, and what the registry still holds is queued for
	// deletion.
	if retainsImage(prior.phase) {
		if err := endSandboxStorageTx(ctx, tx, workspaceID, sandboxID, observedAt); err != nil {
			return err
		}
	}
	if err := queueSandboxImageReapTx(ctx, tx, workspaceID, sandboxID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// The two running meters divide nanoseconds x per-second amount by these:
// weighted milli-vCPU units are per second, memory is MiB per GiB-second.
const (
	sandboxWeightDivisor = int64(time.Second)
	sandboxMemoryDivisor = 1024 * int64(time.Second)
)

// sandboxUsageChunk is one UTC hour's share of an interval.
type sandboxUsageChunk struct {
	window   time.Time
	quantity int64
}

// sandboxIntervalChunks splits an interval at UTC hour boundaries into whole
// units of (nanoseconds x perSecond) / divisor. The remainder carries the
// sub-unit part across samples so a fast poll cadence cannot systematically
// round usage down.
func sandboxIntervalChunks(perSecond, divisor int64, start, end time.Time, remainder int64) ([]sandboxUsageChunk, int64) {
	var chunks []sandboxUsageChunk
	for cursor := start.UTC(); cursor.Before(end); {
		window := cursor.Truncate(time.Hour)
		segmentEnd := window.Add(time.Hour)
		if end.Before(segmentEnd) {
			segmentEnd = end
		}
		numerator := segmentEnd.Sub(cursor).Nanoseconds()*perSecond + remainder
		quantity := numerator / divisor
		remainder = numerator % divisor
		if quantity > 0 {
			chunks = append(chunks, sandboxUsageChunk{window: window, quantity: quantity})
		}
		cursor = segmentEnd
	}
	return chunks, remainder
}

// addSandboxUsage adds each chunk to the sandbox's usage_hourly row of one kind
// and returns the total added.
func addSandboxUsage(ctx context.Context, tx pgx.Tx, workspaceID, sandboxID, kind, tier string, chunks []sandboxUsageChunk) (int64, error) {
	var total int64
	for _, chunk := range chunks {
		total += chunk.quantity
		_, err := tx.Exec(ctx, `
				INSERT INTO usage_hourly
				    (workspace_id, service_id, kind, tier, resource_kind, window_start, quantity)
				VALUES ($1, $2, $3, $4, $5, $6, $7)
				ON CONFLICT (resource_kind, service_id, kind, tier, window_start)
				DO UPDATE SET quantity = usage_hourly.quantity + EXCLUDED.quantity`,
			workspaceID, sandboxID, kind, tier, ResourceKindSandbox, chunk.window, chunk.quantity)
		if err != nil {
			return total, err
		}
	}
	return total, nil
}

// accrueSandboxInterval adds the running interval prior.observedAt..end to the
// weighted compute meter and, when the sandbox's memory is known, to the memory
// meter, and advances prior's remainders. Both land in the caller's transaction
// with the cursor update.
func accrueSandboxInterval(ctx context.Context, tx pgx.Tx, workspaceID, sandboxID string, prior *sandboxMeterState, end time.Time) error {
	chunks, remainder := sandboxIntervalChunks(prior.weight, sandboxWeightDivisor, prior.observedAt, end, prior.remainder)
	total, err := addSandboxUsage(ctx, tx, workspaceID, sandboxID, UsageKindSandboxComputeSeconds, prior.tier, chunks)
	if err != nil {
		return err
	}
	prior.remainder = remainder
	if prior.memoryMiB > 0 {
		memChunks, memRemainder := sandboxIntervalChunks(prior.memoryMiB, sandboxMemoryDivisor, prior.observedAt, end, prior.memoryRemainder)
		if _, err := addSandboxUsage(ctx, tx, workspaceID, sandboxID, UsageKindSandboxMemoryGBSeconds, prior.tier, memChunks); err != nil {
			return err
		}
		prior.memoryRemainder = memRemainder
	}
	// The session that owns this sandbox, if any, carries a running total for
	// its outcome.cost (w9/m178); a sandbox no session adopted updates nothing.
	// It stays on the weighted compute basis: the memory meter never enters it.
	if total > 0 {
		if _, err := tx.Exec(ctx, `
			UPDATE agent_session_sandboxes
			SET compute_milli_seconds = compute_milli_seconds + $3
			WHERE workspace_id = $1 AND sandbox_id = $2`, workspaceID, sandboxID, total); err != nil {
			return err
		}
	}
	return nil
}

// lockSandboxMeter serializes one sandbox's accruals with each other and with
// the agent_session_bind_sandbox trigger (migration 0206), which seeds a newly
// adopted sandbox's session total from usage_hourly under the same key.
func lockSandboxMeter(ctx context.Context, tx pgx.Tx, workspaceID, sandboxID string) error {
	_, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, workspaceID+"|"+sandboxID)
	return err
}

// AgentSessionComputeMilliSeconds is the weighted milli-vCPU seconds the meter
// attributed to one agent session across its turns' sandboxes (w9/m178). A
// session whose sandboxes predate attribution reads 0.
func (s *PGStore) AgentSessionComputeMilliSeconds(ctx context.Context, sessionID string) (int64, error) {
	var total int64
	err := s.Pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(compute_milli_seconds), 0)::bigint
		FROM agent_session_sandboxes WHERE session_id = $1`, sessionID).Scan(&total)
	return total, err
}
