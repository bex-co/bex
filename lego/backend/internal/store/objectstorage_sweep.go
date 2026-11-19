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
	"slices"
	"time"

	"github.com/jackc/pgx/v5"
)

// The object storage sweep's side of the store (ADR112 D5, D8, D9): which
// workspaces it has work for, what it records about each one's Wasabi prefix,
// the shadow meters it feeds, and the retire-then-purge state of a deleted
// workspace's bucket rows.

// ObjectStorageWorkspace is one workspace the object storage sweep has work
// for. Retired means the workspace is gone: a bucket row was retired by its
// deletion (DeleteTenant), or the tenant row no longer exists. A retired
// workspace gets its Wasabi prefix purged, then its rows dropped; any other
// workspace gets a usage and lifecycle sweep.
type ObjectStorageWorkspace struct {
	ID      string
	Retired bool
	// LastObserved is the oldest observation among the workspace's usage rows,
	// so a workspace with one stale region counts as stale. Zero when it has no
	// usage row: the sweep has never recorded it.
	LastObserved time.Time
}

// ListObjectStorageWorkspaces returns every workspace that has a bucket row, live
// or retired, or a usage row, so the sweep never lists Wasabi for a workspace
// that never used it. Retired workspaces come first, the one whose purge was
// attempted longest ago (or never) leading, so a purge that keeps failing cannot
// starve the rest. The live ones follow in id order, each with its oldest
// observation, which the sweep sorts them by (Sweeper.Pass).
func (s *PGStore) ListObjectStorageWorkspaces(ctx context.Context) ([]ObjectStorageWorkspace, error) {
	rows, err := s.Pool.Query(ctx, `
		WITH ws AS (
			SELECT workspace_id FROM object_buckets
			UNION
			SELECT workspace_id FROM object_storage_usage
		)
		SELECT ws.workspace_id,
		       (EXISTS (SELECT 1 FROM object_buckets b WHERE b.workspace_id = ws.workspace_id AND b.deleted_at IS NOT NULL)
		        OR NOT EXISTS (SELECT 1 FROM tenants t WHERE t.id = ws.workspace_id)) AS retired,
		       (SELECT min(u.observed_at) FROM object_storage_usage u WHERE u.workspace_id = ws.workspace_id) AS last_observed
		FROM ws
		ORDER BY retired DESC,
		         COALESCE((SELECT max(b.purge_attempted_at) FROM object_buckets b
		                   WHERE b.workspace_id = ws.workspace_id AND b.deleted_at IS NOT NULL), '-infinity'::timestamptz),
		         ws.workspace_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ObjectStorageWorkspace
	for rows.Next() {
		var w ObjectStorageWorkspace
		var lastObserved *time.Time
		if err := rows.Scan(&w.ID, &w.Retired, &lastObserved); err != nil {
			return nil, err
		}
		if lastObserved != nil {
			w.LastObserved = *lastObserved
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

const retireObjectBucketsSQL = `UPDATE object_buckets SET deleted_at = now() WHERE workspace_id = $1 AND deleted_at IS NULL`

// retireWorkspaceObjectBuckets runs inside the delete that would otherwise
// detach the rows from their objects: object_buckets has no tenant foreign key
// (migration 0273), so DeleteTenant retires the rows (no pub- host resolves a
// retired one) and the object storage sweep purges the workspace's Wasabi
// prefix, then drops them (ADR112 D9).
func retireWorkspaceObjectBuckets(ctx context.Context, tx pgx.Tx, workspaceID string) error {
	_, err := tx.Exec(ctx, retireObjectBucketsSQL, workspaceID)
	return err
}

// RetireObjectBuckets retires the workspace's live bucket rows and reports how
// many it retired. The sweep calls it for a workspace whose tenant row is gone
// without DeleteTenant having retired them (a bucket created while the workspace
// was being deleted, or one deleted before this code shipped).
func (s *PGStore) RetireObjectBuckets(ctx context.Context, workspaceID string) (int64, error) {
	tag, err := s.Pool.Exec(ctx, retireObjectBucketsSQL, workspaceID)
	return tag.RowsAffected(), err
}

// MarkObjectStoragePurgeAttempt stamps the retired rows of a workspace whose
// purge is being tried now.
func (s *PGStore) MarkObjectStoragePurgeAttempt(ctx context.Context, workspaceID string, at time.Time) error {
	_, err := s.Pool.Exec(ctx,
		`UPDATE object_buckets SET purge_attempted_at = $2 WHERE workspace_id = $1 AND deleted_at IS NOT NULL`,
		workspaceID, at.UTC())
	return err
}

// FinishObjectStoragePurge drops a deleted workspace's retired bucket rows and
// its usage rows, once its Wasabi prefix is empty in every region. A live row
// (a bucket created in the race with the delete) stays, with the usage rows
// beside it, until the sweep retires it and purges again.
func (s *PGStore) FinishObjectStoragePurge(ctx context.Context, workspaceID string) error {
	return s.withTx(ctx, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx,
			`DELETE FROM object_buckets WHERE workspace_id = $1 AND deleted_at IS NOT NULL`, workspaceID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			DELETE FROM object_storage_writes w WHERE w.workspace_id = $1
			AND NOT EXISTS (SELECT 1 FROM object_buckets b WHERE b.workspace_id = w.workspace_id)`, workspaceID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			DELETE FROM object_multipart_uploads m WHERE m.workspace_id = $1
			AND NOT EXISTS (SELECT 1 FROM object_buckets b WHERE b.workspace_id = m.workspace_id)`, workspaceID); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `
			DELETE FROM object_storage_usage u WHERE u.workspace_id = $1
			AND NOT EXISTS (SELECT 1 FROM object_buckets b WHERE b.workspace_id = u.workspace_id)`, workspaceID)
		return err
	})
}

// priorObjectStorageUsage is one usage row as the previous sweep left it.
type priorObjectStorageUsage struct {
	bytes      int64
	observedAt time.Time
	remainder  int64
}

// RecordObjectStorageObservation replaces what the sweep knows about one
// workspace's prefix in one region with a new observation, and accrues the
// object_storage_gb_seconds shadow meter in the same transaction (ADR112 D8).
//
// observed holds one row per usage key the sweep saw (a bucket id, or the
// orphan marker); a row the previous sweep wrote and this one did not see is
// dropped, its objects being gone. For each key that has a previous row, the
// meter adds the decimal GB-seconds of the interval between the two
// observations at the mean of the two byte counts (a trapezoid, since storage
// changes continuously between sweeps), split at UTC hour boundaries. The
// first observation of a key accrues nothing, so no time is invented before it.
// The previous rows are locked, so two sweeps racing each other add each
// interval once.
func (s *PGStore) RecordObjectStorageObservation(ctx context.Context, workspaceID, region string, observed []ObjectStorageUsage, at time.Time) error {
	at = storageSecond(at)
	return s.withTx(ctx, func(tx pgx.Tx) error {
		prior, err := lockObjectStorageUsage(ctx, tx, workspaceID, region)
		if err != nil {
			return err
		}
		// What bex wrote before this listing started is in it (ADR112 D8).
		if _, err := tx.Exec(ctx, `DELETE FROM object_storage_writes WHERE workspace_id = $1 AND region = $2 AND updated_at <= $3`, workspaceID, region, at); err != nil {
			return err
		}
		accrued := map[time.Time]int64{}
		accrue := func(p priorObjectStorageUsage, now time.Time, bytes int64) (remainder int64) {
			seconds := int64(now.Sub(p.observedAt) / time.Second)
			if seconds <= 0 {
				return p.remainder
			}
			chunks, rem := storageIntervalChunks((p.bytes+bytes)/2, p.observedAt, seconds, p.remainder)
			for _, c := range chunks {
				accrued[c.window] += c.quantity
			}
			return rem
		}
		for _, u := range observed {
			p, had := prior[u.BucketID]
			delete(prior, u.BucketID)
			observedAt, remainder := at, int64(0)
			if had {
				// A clock that stepped back keeps the row's later stamp, so the
				// same seconds are not accrued twice.
				observedAt = laterOf(at, p.observedAt)
				remainder = accrue(p, observedAt, u.Bytes)
			}
			if _, err := tx.Exec(ctx, `
				INSERT INTO object_storage_usage (bucket_id, workspace_id, region, bytes, objects, observed_at, gb_remainder)
				VALUES ($1, $2, $3, $4, $5, $6, $7)
				ON CONFLICT (bucket_id) DO UPDATE
				SET workspace_id = EXCLUDED.workspace_id, region = EXCLUDED.region, bytes = EXCLUDED.bytes,
				    objects = EXCLUDED.objects, observed_at = EXCLUDED.observed_at, gb_remainder = EXCLUDED.gb_remainder`,
				u.BucketID, workspaceID, region, u.Bytes, u.Objects, observedAt, remainder); err != nil {
				return err
			}
		}
		for bucketID, p := range prior {
			accrue(p, laterOf(at, p.observedAt), 0)
			if _, err := tx.Exec(ctx, `DELETE FROM object_storage_usage WHERE bucket_id = $1`, bucketID); err != nil {
				return err
			}
		}
		windows := make([]time.Time, 0, len(accrued))
		for w := range accrued {
			windows = append(windows, w)
		}
		slices.SortFunc(windows, time.Time.Compare)
		for _, w := range windows {
			if err := addObjectStorageUsageTx(ctx, tx, workspaceID, UsageKindObjectStorageGBSeconds, w, accrued[w]); err != nil {
				return err
			}
		}
		return nil
	})
}

func laterOf(a, b time.Time) time.Time {
	if b.After(a) {
		return b
	}
	return a
}

// lockObjectStorageUsage reads and row-locks the usage rows of one workspace
// and region, keyed by bucket id.
func lockObjectStorageUsage(ctx context.Context, tx pgx.Tx, workspaceID, region string) (map[string]priorObjectStorageUsage, error) {
	rows, err := tx.Query(ctx, `
		SELECT bucket_id, bytes, observed_at, gb_remainder FROM object_storage_usage
		WHERE workspace_id = $1 AND region = $2 FOR UPDATE`, workspaceID, region)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	prior := map[string]priorObjectStorageUsage{}
	for rows.Next() {
		var id string
		var p priorObjectStorageUsage
		if err := rows.Scan(&id, &p.bytes, &p.observedAt, &p.remainder); err != nil {
			return nil, err
		}
		prior[id] = p
	}
	return prior, rows.Err()
}

// ObjectStorageUsageDelta is quantity of one object storage meter to add to a
// workspace's usage_hourly row of the hour starting at Window.
type ObjectStorageUsageDelta struct {
	WorkspaceID string
	Kind        string
	Window      time.Time
	Quantity    int64
}

func validObjectStorageKind(kind string) bool {
	switch kind {
	case UsageKindObjectStorageGBSeconds, UsageKindObjectStorageIngressBytes, UsageKindObjectStorageEgressBytes:
		return true
	}
	return false
}

// AddObjectStorageUsage adds each delta to its workspace's usage_hourly row in
// one transaction. A delta for a workspace that no longer exists is dropped:
// usage_hourly cascades with the tenant, and bytes counted in memory just before
// a delete may be flushed just after it.
func (s *PGStore) AddObjectStorageUsage(ctx context.Context, deltas []ObjectStorageUsageDelta) error {
	for _, d := range deltas {
		if !validObjectStorageKind(d.Kind) {
			return fmt.Errorf("%w: %q is not an object storage usage kind", ErrInvalid, d.Kind)
		}
	}
	return s.withTx(ctx, func(tx pgx.Tx) error {
		for _, d := range deltas {
			if err := addObjectStorageUsageTx(ctx, tx, d.WorkspaceID, d.Kind, d.Window, d.Quantity); err != nil {
				return err
			}
		}
		return nil
	})
}

// addObjectStorageUsageTx adds quantity to the workspace's row of one kind and
// hour. The row is the workspace's own (service_id is the workspace id), and a
// workspace with no tenant row gets nothing.
func addObjectStorageUsageTx(ctx context.Context, tx pgx.Tx, workspaceID, kind string, window time.Time, quantity int64) error {
	if quantity <= 0 {
		return nil
	}
	_, err := tx.Exec(ctx, `
		INSERT INTO usage_hourly (workspace_id, service_id, kind, tier, resource_kind, window_start, quantity)
		SELECT t.id, t.id, $2, '', $3, $4, $5 FROM tenants t WHERE t.id = $1
		ON CONFLICT (resource_kind, service_id, kind, tier, window_start)
		DO UPDATE SET quantity = usage_hourly.quantity + EXCLUDED.quantity`,
		workspaceID, kind, ResourceKindObjectStorage, window.UTC().Truncate(time.Hour), quantity)
	return err
}

// ObjectStorageEgressSince is the bytes every workspace read out of object
// storage since the start of the given hour, summed from the shadow meter's
// hourly rows (all three paths: Render's presigned reads, the S3 surface's
// streams and public reads). It feeds the month-to-date egress alert, the number Wasabi's
// fair-use rule is about (ADR112 D4).
func (s *PGStore) ObjectStorageEgressSince(ctx context.Context, since time.Time) (int64, error) {
	var total int64
	err := s.Pool.QueryRow(ctx, `
		SELECT COALESCE(sum(quantity), 0)::bigint FROM usage_hourly
		WHERE resource_kind = $1 AND kind = $2 AND window_start >= $3`,
		ResourceKindObjectStorage, UsageKindObjectStorageEgressBytes, since.UTC().Truncate(time.Hour)).Scan(&total)
	return total, err
}

// AddObjectStorageWrite counts a completed write (a put, a copy or a multipart
// completion) of bytes and objects into a workspace's region, for the quota gate to
// add to the sweep's last observation (ADR112 D8).
func (s *PGStore) AddObjectStorageWrite(ctx context.Context, workspaceID, region string, bytes, objects int64) error {
	_, err := s.Pool.Exec(ctx, `
		INSERT INTO object_storage_writes (workspace_id, region, bytes, objects, updated_at)
		VALUES ($1, $2, $3, $4, now())
		ON CONFLICT (workspace_id, region)
		DO UPDATE SET bytes = object_storage_writes.bytes + EXCLUDED.bytes,
		              objects = object_storage_writes.objects + EXCLUDED.objects, updated_at = now()`,
		workspaceID, region, max(bytes, 0), max(objects, 0))
	return err
}

// ObjectStorageWriteTotals is what bex has written into a workspace's region since the
// sweep's last observation of it.
func (s *PGStore) ObjectStorageWriteTotals(ctx context.Context, workspaceID, region string) (bytes, objects int64, err error) {
	err = s.Pool.QueryRow(ctx, `
		SELECT COALESCE(sum(bytes), 0)::bigint, COALESCE(sum(objects), 0)::bigint
		FROM object_storage_writes WHERE workspace_id = $1 AND region = $2`, workspaceID, region).Scan(&bytes, &objects)
	return bytes, objects, err
}

// ObjectStorageWorkspaceEgressSince is the bytes a workspace read out of object storage
// since the start of the given hour: the S3 surface's monthly egress budget reads it.
func (s *PGStore) ObjectStorageWorkspaceEgressSince(ctx context.Context, workspaceID string, since time.Time) (int64, error) {
	var total int64
	err := s.Pool.QueryRow(ctx, `
		SELECT COALESCE(sum(quantity), 0)::bigint FROM usage_hourly
		WHERE resource_kind = $1 AND kind = $2 AND service_id = $3 AND window_start >= $4`,
		ResourceKindObjectStorage, UsageKindObjectStorageEgressBytes, workspaceID, since.UTC().Truncate(time.Hour)).Scan(&total)
	return total, err
}
