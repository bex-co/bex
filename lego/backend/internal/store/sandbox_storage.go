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

// The sandbox storage meter: sandbox_snapshot_gb_seconds, decimal GB x seconds
// an object is retained (docs/ADR023-usage-metering.md § Sandbox snapshot
// storage). An object is a suspended sandbox's retained rootfs image
// (StorageKindRootfs, driven by the phase cursor) or a snapshot
// (StorageKindSnapshot, which w32/m285 registers through the same exported
// calls). Start, End and SetSandboxStorageBytes are idempotent and safe to
// replay and to race, and an object that is never sized is never billed as zero
// silently: SandboxStorageStats counts it.
const (
	StorageKindRootfs   = "rootfs"
	StorageKindSnapshot = "snapshot"

	// MaxStorageObjectBytes bounds one object's size at 16 TiB so byte-seconds
	// for an hour always fit a bigint.
	MaxStorageObjectBytes = int64(1) << 44

	// BytesPerDecimalGB is the GB the storage meters and the cap count in.
	BytesPerDecimalGB = int64(1_000_000_000)

	// StorageAccrualInterval is how stale an open object's cursor may get
	// before the accrual pass advances it: usage is hour-bucketed and sealed
	// long after, so a quarter hour of lag costs nothing, and it keeps the
	// per-object transaction rate low. A closed object is settled at the next
	// pass regardless.
	StorageAccrualInterval = 15 * time.Minute

	// StorageRetention is how long a settled, closed object's row is kept after
	// it ended, so a late replay or a reopen still finds it.
	StorageRetention = 35 * 24 * time.Hour

	// StorageUnsizedGrace is how long a retained image may stay unsized before
	// closing without a size counts as a loss: a sandbox suspended and then
	// deleted or resumed within minutes may never have committed an image, and
	// its charge would be cents of a cent. It matches SandboxStorageUnsized's
	// 30-minute window, so the closed count only holds what that alert already
	// should have flagged.
	StorageUnsizedGrace = 30 * time.Minute
)

// SandboxStorageObject is one retained object's durable cursor.
type SandboxStorageObject struct {
	WorkspaceID    string
	ObjectID       string
	Kind           string
	Ref            string // where the registry holds it: the substrate sandbox id for a rootfs image
	Bytes          *int64 // nil until sized
	StartedAt      time.Time
	AccruedThrough time.Time
	EndedAt        *time.Time
	ReapPending    bool
	remainder      int64
}

// SandboxStorageStats is what the meter exposes about its own health.
type SandboxStorageStats struct {
	Open           int64 // objects being retained now
	UnsizedOpen    int64 // retained but not sized yet: not accruing
	UnsizedClosed  int64 // closed without ever being sized after StorageUnsizedGrace: never billed
	AccrualBacklog int64 // objects with seconds not yet added to usage_hourly
	ReapPending    int64 // images of gone sandboxes the registry still holds
	OpenBytes      int64 // stored bytes of the open, sized objects
}

// whole seconds, so byte-seconds stay integers.
func storageSecond(t time.Time) time.Time { return t.UTC().Truncate(time.Second) }

func storageSelect(where string) string {
	return `SELECT workspace_id, object_id, kind, ref, bytes, started_at, accrued_through, remainder, ended_at, reap_pending
		FROM sandbox_storage_objects WHERE ` + where
}

func scanStorageObject(row pgx.Row) (SandboxStorageObject, error) {
	var o SandboxStorageObject
	err := row.Scan(&o.WorkspaceID, &o.ObjectID, &o.Kind, &o.Ref, &o.Bytes,
		&o.StartedAt, &o.AccruedThrough, &o.remainder, &o.EndedAt, &o.ReapPending)
	return o, err
}

func validStorageKind(kind string) bool {
	return kind == StorageKindRootfs || kind == StorageKindSnapshot
}

// StartSandboxStorage opens an object. An object that is already open, or that
// closed at or after at (an older replay), is left alone; a closed one reopens
// after its last period is settled (a sandbox resumed and suspended again).
func (s *PGStore) StartSandboxStorage(ctx context.Context, workspaceID, objectID, kind, ref string, at time.Time) error {
	if workspaceID == "" || objectID == "" || !validStorageKind(kind) || at.IsZero() {
		return fmt.Errorf("invalid sandbox storage start")
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := lockSandboxMeter(ctx, tx, workspaceID, objectID); err != nil {
		return err
	}
	if err := startSandboxStorageTx(ctx, tx, workspaceID, objectID, kind, ref, at); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// EndSandboxStorage closes an open object at at (never before it started).
// Closing a closed or unknown object changes nothing.
func (s *PGStore) EndSandboxStorage(ctx context.Context, workspaceID, objectID string, at time.Time) error {
	if workspaceID == "" || objectID == "" || at.IsZero() {
		return fmt.Errorf("invalid sandbox storage end")
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := lockSandboxMeter(ctx, tx, workspaceID, objectID); err != nil {
		return err
	}
	if err := endSandboxStorageTx(ctx, tx, workspaceID, objectID, at); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// SetSandboxStorageBytes sizes the object that started at startedAt: its size
// never changes once set, and a size read for an earlier period of a reopened
// object is refused by the started_at guard.
func (s *PGStore) SetSandboxStorageBytes(ctx context.Context, workspaceID, objectID string, startedAt time.Time, bytes int64) error {
	if workspaceID == "" || objectID == "" || bytes < 0 || bytes > MaxStorageObjectBytes {
		return fmt.Errorf("invalid sandbox storage size")
	}
	_, err := s.Pool.Exec(ctx, `
		UPDATE sandbox_storage_objects SET bytes = $4
		WHERE workspace_id = $1 AND object_id = $2 AND started_at = $3 AND bytes IS NULL`,
		workspaceID, objectID, startedAt.UTC(), bytes)
	return err
}

// ClaimUnsizedSandboxStorage returns up to limit open rootfs objects still
// waiting for a size, the ones tried least recently first, and stamps them as
// tried now. The rotation keeps objects whose size can never be read (a missing
// image) from holding the head of the queue and starving newer ones.
func (s *PGStore) ClaimUnsizedSandboxStorage(ctx context.Context, now time.Time, limit int) ([]SandboxStorageObject, error) {
	rows, err := s.Pool.Query(ctx, `
		WITH due AS (
			SELECT workspace_id, object_id FROM sandbox_storage_objects
			WHERE bytes IS NULL AND ended_at IS NULL AND kind = 'rootfs'
			ORDER BY size_attempted_at NULLS FIRST, started_at
			LIMIT $2
			FOR UPDATE SKIP LOCKED)
		UPDATE sandbox_storage_objects o SET size_attempted_at = $1
		FROM due WHERE o.workspace_id = due.workspace_id AND o.object_id = due.object_id
		RETURNING o.workspace_id, o.object_id, o.kind, o.ref, o.bytes, o.started_at, o.accrued_through, o.remainder, o.ended_at, o.reap_pending`,
		now.UTC(), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SandboxStorageObject
	for rows.Next() {
		o, err := scanStorageObject(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

func (s *PGStore) listStorage(ctx context.Context, where string, limit int) ([]SandboxStorageObject, error) {
	rows, err := s.Pool.Query(ctx, storageSelect(where), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SandboxStorageObject
	for rows.Next() {
		o, err := scanStorageObject(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

// SandboxStorageStats counts what the meter holds, for its gauges. Each count
// reads one partial index, so the cost follows the objects with work left, not
// the table's history.
func (s *PGStore) SandboxStorageStats(ctx context.Context, now time.Time) (SandboxStorageStats, error) {
	var st SandboxStorageStats
	err := s.Pool.QueryRow(ctx, `
		SELECT
		  (SELECT count(*) FROM sandbox_storage_objects WHERE ended_at IS NULL),
		  (SELECT count(*) FROM sandbox_storage_objects WHERE bytes IS NULL AND ended_at IS NULL),
		  (SELECT count(*) FROM sandbox_storage_objects
		   WHERE bytes IS NULL AND ended_at IS NOT NULL
		     AND ended_at >= started_at + make_interval(secs => $2)),
		  (SELECT count(*) FROM sandbox_storage_objects
		   WHERE bytes IS NOT NULL AND (ended_at IS NULL OR accrued_through < ended_at)
		     AND accrued_through < COALESCE(ended_at, $1::timestamptz)),
		  (SELECT count(*) FROM sandbox_storage_objects WHERE reap_pending),
		  (SELECT COALESCE(sum(bytes), 0)::bigint FROM sandbox_storage_objects WHERE ended_at IS NULL)`,
		now.UTC(), StorageUnsizedGrace.Seconds()).Scan(&st.Open, &st.UnsizedOpen, &st.UnsizedClosed, &st.AccrualBacklog, &st.ReapPending, &st.OpenBytes)
	return st, err
}

// PruneSandboxStorage deletes up to limit objects that are settled for good: a
// closed object more than StorageRetention old whose seconds are all accrued (or
// that was never sized, and so never will be) and whose image is not waiting for
// the reaper. Without it the table grows with every sandbox ever suspended.
func (s *PGStore) PruneSandboxStorage(ctx context.Context, now time.Time, limit int) (int64, error) {
	tag, err := s.Pool.Exec(ctx, `
		DELETE FROM sandbox_storage_objects o USING (
			SELECT workspace_id, object_id FROM sandbox_storage_objects
			WHERE ended_at IS NOT NULL AND NOT reap_pending AND ended_at < $1
			  AND (bytes IS NULL OR accrued_through >= ended_at)
			ORDER BY ended_at LIMIT $2
		) gone WHERE o.workspace_id = gone.workspace_id AND o.object_id = gone.object_id`,
		now.UTC().Add(-StorageRetention), limit)
	return tag.RowsAffected(), err
}

// AccrueSandboxStorage adds the seconds each sized object has been retained
// since its cursor to usage_hourly, split at UTC hour boundaries, and advances
// the cursor in the same transaction, so a replayed pass adds nothing. It
// settles up to limit objects whose cursor is older than StorageAccrualInterval
// (or that have closed) and reports how many it advanced.
func (s *PGStore) AccrueSandboxStorage(ctx context.Context, now time.Time, limit int) (int, error) {
	now = storageSecond(now)
	rows, err := s.Pool.Query(ctx, `
		SELECT workspace_id, object_id
		FROM sandbox_storage_objects
		WHERE bytes IS NOT NULL
		  AND ((ended_at IS NULL AND accrued_through <= $1) OR accrued_through < ended_at)
		ORDER BY accrued_through
		LIMIT $2`, now.Add(-StorageAccrualInterval), limit)
	if err != nil {
		return 0, err
	}
	type key struct{ ws, id string }
	var keys []key
	for rows.Next() {
		var k key
		if err := rows.Scan(&k.ws, &k.id); err != nil {
			rows.Close()
			return 0, err
		}
		keys = append(keys, k)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}
	advanced := 0
	for _, k := range keys {
		ok, err := s.accrueOneSandboxStorage(ctx, k.ws, k.id, now)
		if err != nil {
			return advanced, err
		}
		if ok {
			advanced++
		}
	}
	return advanced, nil
}

func (s *PGStore) accrueOneSandboxStorage(ctx context.Context, workspaceID, objectID string, now time.Time) (bool, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := lockSandboxMeter(ctx, tx, workspaceID, objectID); err != nil {
		return false, err
	}
	o, err := scanStorageObject(tx.QueryRow(ctx,
		storageSelect(`workspace_id = $1 AND object_id = $2 FOR UPDATE`), workspaceID, objectID))
	if err == pgx.ErrNoRows {
		return false, tx.Commit(ctx)
	}
	if err != nil {
		return false, err
	}
	advanced, err := accrueSandboxStorageTx(ctx, tx, &o, now)
	if err != nil {
		return false, err
	}
	return advanced, tx.Commit(ctx)
}

// startSandboxStorageTx is StartSandboxStorage inside the caller's transaction
// (the phase cursor's), which already holds the object's advisory lock.
func startSandboxStorageTx(ctx context.Context, tx pgx.Tx, workspaceID, objectID, kind, ref string, at time.Time) error {
	at = storageSecond(at)
	// The common case is an object that is already open, seen again on every
	// poll: the insert takes no row lock and changes nothing then.
	tag, err := tx.Exec(ctx, `
		INSERT INTO sandbox_storage_objects (workspace_id, object_id, kind, ref, started_at, accrued_through)
		VALUES ($1, $2, $3, $4, $5, $5)
		ON CONFLICT (workspace_id, object_id) DO NOTHING`, workspaceID, objectID, kind, ref, at)
	if err != nil || tag.RowsAffected() == 1 {
		return err
	}
	// The caller holds the object's advisory lock, so no other writer races this read.
	o, err := scanStorageObject(tx.QueryRow(ctx, storageSelect(`workspace_id = $1 AND object_id = $2`), workspaceID, objectID))
	if err != nil {
		return err
	}
	if o.EndedAt == nil || !at.After(*o.EndedAt) {
		return nil // already open, or an older replay of a closed period
	}
	// Settle the closed period before the row is reused for the new one.
	if _, err := accrueSandboxStorageTx(ctx, tx, &o, *o.EndedAt); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
		UPDATE sandbox_storage_objects
		SET kind = $3, ref = $4, bytes = NULL, started_at = $5, accrued_through = $5,
		    remainder = 0, ended_at = NULL, reap_pending = false, size_attempted_at = NULL
		WHERE workspace_id = $1 AND object_id = $2`, workspaceID, objectID, kind, ref, at)
	return err
}

func endSandboxStorageTx(ctx context.Context, tx pgx.Tx, workspaceID, objectID string, at time.Time) error {
	_, err := tx.Exec(ctx, `
		UPDATE sandbox_storage_objects SET ended_at = GREATEST(started_at, $3)
		WHERE workspace_id = $1 AND object_id = $2 AND ended_at IS NULL`,
		workspaceID, objectID, storageSecond(at))
	return err
}

// queueSandboxImageReapTx queues a gone sandbox's image for the reaper, whether
// the object closed now or when the sandbox last resumed: a resumed sandbox that
// is later terminated still leaves its old tags in the registry. A resume alone
// does not queue it, because the pod may still be pulling the image.
func queueSandboxImageReapTx(ctx context.Context, tx pgx.Tx, workspaceID, objectID string) error {
	_, err := tx.Exec(ctx, `
		UPDATE sandbox_storage_objects SET reap_pending = true
		WHERE workspace_id = $1 AND object_id = $2 AND kind = $3 AND ref <> '' AND ended_at IS NOT NULL`,
		workspaceID, objectID, StorageKindRootfs)
	return err
}

// ListReapableSandboxStorage returns the objects whose sandbox is gone and whose
// image the registry may still hold, oldest first.
func (s *PGStore) ListReapableSandboxStorage(ctx context.Context, limit int) ([]SandboxStorageObject, error) {
	return s.listStorage(ctx, `reap_pending ORDER BY ended_at LIMIT $1`, limit)
}

// MarkSandboxStorageReaped records that the image of the period that started at
// startedAt is deleted from the registry.
func (s *PGStore) MarkSandboxStorageReaped(ctx context.Context, workspaceID, objectID string, startedAt time.Time) error {
	_, err := s.Pool.Exec(ctx, `
		UPDATE sandbox_storage_objects SET reap_pending = false
		WHERE workspace_id = $1 AND object_id = $2 AND started_at = $3`,
		workspaceID, objectID, startedAt.UTC())
	return err
}

// SandboxStorageBytes is the stored bytes of a workspace's open, sized objects
// plus its imported images (w2/m358), counting an import still copying at its
// declared size (round 24, S-3): what the per-workspace cap reads. Imported
// images count toward the cap but are not billed (ADR104 § Images). An object
// not sized yet counts for nothing, so a workspace can pass the cap by the
// images still waiting for a size.
func (s *PGStore) SandboxStorageBytes(ctx context.Context, workspaceID string) (int64, error) {
	var bytes int64
	err := s.Pool.QueryRow(ctx, sandboxStorageBytesSQL, workspaceID).Scan(&bytes)
	return bytes, err
}

// importedBuildSQL is the predicate of an import that holds registry bytes,
// stored or still copying at its declared size: not failed, not deleted. The
// sandbox_builds_stored index has exactly this predicate.
const importedBuildSQL = `status <> 'failed' AND deleted_at IS NULL`

// sandboxStorageBytesSQL is SandboxStorageBytes' sum, which CreateSandboxBuild
// and ReserveAgentRepoEnvironmentBuildImage also read under the workspace's
// storage lock. Repository environment build images count as imported ones do
// (ADR111 D9): reserved while their build runs, then until the registry has
// dropped them.
const sandboxStorageBytesSQL = `
	SELECT ((SELECT COALESCE(sum(bytes), 0) FROM sandbox_storage_objects
	        WHERE workspace_id = $1 AND ended_at IS NULL)
	     + (SELECT COALESCE(sum(size_bytes), 0) FROM sandbox_builds
	        WHERE workspace_id = $1 AND ` + importedBuildSQL + `)
	     + (SELECT COALESCE(sum(size_bytes), 0) FROM agent_repo_environment_builds
	        WHERE workspace_id = $1 AND (image_digest IS NOT NULL OR status = ` + buildBuildingSQL + `)))::bigint`

// platformImportedBytesSQL is the imported image bytes of every workspace
// together, which CreateSandboxBuild's platform budget counts (w5/m383).
const platformImportedBytesSQL = `SELECT COALESCE(sum(size_bytes), 0) FROM sandbox_builds WHERE ` + importedBuildSQL

// accrueSandboxStorageTx adds o's seconds from its cursor through min(now,
// ended_at) and advances the cursor. An unsized object accrues nothing and
// keeps its cursor, so its whole interval is covered once it is sized.
func accrueSandboxStorageTx(ctx context.Context, tx pgx.Tx, o *SandboxStorageObject, now time.Time) (bool, error) {
	if o.Bytes == nil {
		return false, nil
	}
	through := storageSecond(now)
	if o.EndedAt != nil && o.EndedAt.Before(through) {
		through = storageSecond(*o.EndedAt)
	}
	seconds := int64(through.Sub(o.AccruedThrough) / time.Second)
	if seconds <= 0 {
		return false, nil
	}
	chunks, remainder := storageIntervalChunks(*o.Bytes, o.AccruedThrough, seconds, o.remainder)
	if _, err := addSandboxUsage(ctx, tx, o.WorkspaceID, o.ObjectID, UsageKindSandboxSnapshotGBSeconds, "", chunks); err != nil {
		return false, err
	}
	o.AccruedThrough = o.AccruedThrough.Add(time.Duration(seconds) * time.Second)
	o.remainder = remainder
	if _, err := tx.Exec(ctx, `
		UPDATE sandbox_storage_objects SET accrued_through = $3, remainder = $4
		WHERE workspace_id = $1 AND object_id = $2`, o.WorkspaceID, o.ObjectID, o.AccruedThrough, remainder); err != nil {
		return false, err
	}
	return true, nil
}

// storageIntervalChunks splits `seconds` whole seconds from start at UTC hour
// boundaries into decimal GB-seconds (bytes x seconds / 1e9). The remainder
// carries the sub-unit byte-seconds across passes, so the pass cadence cannot
// systematically round usage down.
func storageIntervalChunks(bytes int64, start time.Time, seconds, remainder int64) ([]sandboxUsageChunk, int64) {
	var chunks []sandboxUsageChunk
	cursor := start.UTC()
	for seconds > 0 {
		window := cursor.Truncate(time.Hour)
		n := int64(window.Add(time.Hour).Sub(cursor) / time.Second)
		if n > seconds {
			n = seconds
		}
		numerator := n*bytes + remainder
		quantity := numerator / BytesPerDecimalGB
		remainder = numerator % BytesPerDecimalGB
		if quantity > 0 {
			chunks = append(chunks, sandboxUsageChunk{window: window, quantity: quantity})
		}
		cursor = cursor.Add(time.Duration(n) * time.Second)
		seconds -= n
	}
	return chunks, remainder
}
