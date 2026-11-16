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
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	ids "github.com/bex-co/bex/lego/backend/internal/id"
)

var (
	// ErrObjectBucketExists refuses a bucket whose name a live bucket of the
	// workspace already holds.
	ErrObjectBucketExists = fmt.Errorf("object bucket name is taken: %w", ErrConflict)
	// ErrObjectBucketQuota refuses a bucket past the workspace's bucket cap.
	ErrObjectBucketQuota = fmt.Errorf("object bucket limit reached: %w", ErrConflict)
	// ErrObjectBucketDeleting refuses a put into a bucket a delete is checking.
	ErrObjectBucketDeleting = fmt.Errorf("object bucket is being deleted: %w", ErrConflict)
)

// ObjectBucket is one row of object_buckets (ADR112 D1). Its ID, not its Name,
// is the second segment of every object key in Wasabi.
type ObjectBucket struct {
	ID          string
	WorkspaceID string
	Name        string
	Region      string
	// LocationHint is the Cloudflare hint the bucket was created with, kept so
	// `bucket info` can report it; bex's region is exact.
	LocationHint string
	// System marks the workspace's render-<region> bucket, the namespace of
	// Render's /v1/objects routes. It is created on first use.
	System    bool
	Public    bool
	CORS      json.RawMessage
	Lifecycle json.RawMessage
	CreatedAt time.Time
}

const objectBucketColumns = `id, workspace_id, name, region, location_hint, system, public, cors, lifecycle, created_at`

// objectBucketQuerier is the QueryRow both a pool and a transaction have.
type objectBucketQuerier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// insertObjectBucket adds a live row with a fresh id; public, cors and
// lifecycle take the migration's defaults.
func insertObjectBucket(ctx context.Context, q objectBucketQuerier, in ObjectBucket) (ObjectBucket, error) {
	return scanObjectBucket(q.QueryRow(ctx, `
		INSERT INTO object_buckets (id, workspace_id, name, region, location_hint, system)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING `+objectBucketColumns,
		ids.New(ids.ObjectBucket), in.WorkspaceID, in.Name, in.Region, in.LocationHint, in.System))
}

func selectLiveObjectBucket(ctx context.Context, q objectBucketQuerier, workspaceID, name string) (ObjectBucket, error) {
	return scanObjectBucket(q.QueryRow(ctx,
		`SELECT `+objectBucketColumns+` FROM object_buckets WHERE workspace_id = $1 AND name = $2 AND deleted_at IS NULL`,
		workspaceID, name))
}

func scanObjectBucket(row pgx.Row) (ObjectBucket, error) {
	var b ObjectBucket
	err := row.Scan(&b.ID, &b.WorkspaceID, &b.Name, &b.Region, &b.LocationHint, &b.System, &b.Public, &b.CORS, &b.Lifecycle, &b.CreatedAt)
	return b, err
}

// CreateObjectBucket inserts a live bucket row for in.WorkspaceID, minting its
// id. A user bucket (System false) is refused past userCap live user buckets;
// the count and the insert run under one per-workspace lock, so concurrent
// creates cannot pass the cap together. ErrObjectBucketExists answers a taken
// name.
func (s *PGStore) CreateObjectBucket(ctx context.Context, in ObjectBucket, userCap int) (ObjectBucket, error) {
	var out ObjectBucket
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		if err := lockObjectBuckets(ctx, tx, in.WorkspaceID); err != nil {
			return err
		}
		if !in.System {
			n, err := countUserObjectBuckets(ctx, tx, in.WorkspaceID)
			if err != nil {
				return err
			}
			if n >= userCap {
				return ErrObjectBucketQuota
			}
		}
		b, err := insertObjectBucket(ctx, tx, in)
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "object_buckets_live_name" {
			return ErrObjectBucketExists
		}
		out = b
		return err
	})
	if err != nil {
		return ObjectBucket{}, classify("object bucket", err)
	}
	return out, nil
}

// EnsureSystemObjectBucket returns the workspace's system bucket called name,
// creating it on first use. Concurrent callers get the same row. A live
// bucket of that name that is not a system bucket is a conflict: the render-
// prefix is reserved, so none should exist.
func (s *PGStore) EnsureSystemObjectBucket(ctx context.Context, workspaceID, name, region string) (ObjectBucket, error) {
	// Every object request resolves its system bucket, and the row nearly
	// always exists: only creating it needs the lock. A miss, a non-system row
	// or any error falls through to the locked path below.
	if b, err := s.GetObjectBucket(ctx, workspaceID, name); err == nil && b.System {
		return b, nil
	}
	var out ObjectBucket
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		if err := lockObjectBuckets(ctx, tx, workspaceID); err != nil {
			return err
		}
		b, err := selectLiveObjectBucket(ctx, tx, workspaceID, name)
		if errors.Is(err, pgx.ErrNoRows) {
			b, err = insertObjectBucket(ctx, tx, ObjectBucket{WorkspaceID: workspaceID, Name: name, Region: region, System: true})
		}
		if err != nil {
			return err
		}
		if !b.System {
			return ErrObjectBucketExists
		}
		out = b
		return nil
	})
	if err != nil {
		return ObjectBucket{}, classify("object bucket", err)
	}
	return out, nil
}

// GetObjectBucket returns the live bucket of workspaceID called name.
func (s *PGStore) GetObjectBucket(ctx context.Context, workspaceID, name string) (ObjectBucket, error) {
	b, err := selectLiveObjectBucket(ctx, s.Pool, workspaceID, name)
	if err != nil {
		return ObjectBucket{}, classify("object bucket", err)
	}
	return b, nil
}

// GetObjectBucketByID returns the live bucket with the given id, whichever
// workspace owns it. It is what resolves a public host to its bucket.
func (s *PGStore) GetObjectBucketByID(ctx context.Context, id string) (ObjectBucket, error) {
	b, err := scanObjectBucket(s.Pool.QueryRow(ctx,
		`SELECT `+objectBucketColumns+` FROM object_buckets WHERE id = $1 AND deleted_at IS NULL`, id))
	if err != nil {
		return ObjectBucket{}, classify("object bucket", err)
	}
	return b, nil
}

// ListObjectBucketSummaries is ListObjectBuckets without each bucket's CORS and
// lifecycle rules, which are up to 64 KiB and 1 MiB a bucket: a listing of a
// full workspace must not load a gigabyte to print names. CORS and Lifecycle
// are nil in the result.
func (s *PGStore) ListObjectBucketSummaries(ctx context.Context, workspaceID string) ([]ObjectBucket, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT id, workspace_id, name, region, location_hint, system, public, created_at
		 FROM object_buckets WHERE workspace_id = $1 AND deleted_at IS NULL ORDER BY created_at, id`,
		workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ObjectBucket
	for rows.Next() {
		var b ObjectBucket
		if err := rows.Scan(&b.ID, &b.WorkspaceID, &b.Name, &b.Region, &b.LocationHint, &b.System, &b.Public, &b.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// ListObjectBuckets returns the workspace's live buckets, oldest first.
func (s *PGStore) ListObjectBuckets(ctx context.Context, workspaceID string) ([]ObjectBucket, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT `+objectBucketColumns+` FROM object_buckets WHERE workspace_id = $1 AND deleted_at IS NULL ORDER BY created_at, id`,
		workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ObjectBucket
	for rows.Next() {
		b, err := scanObjectBucket(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// objectBucketDeletingTTL is how long a delete's mark (deleting_since) refuses
// puts. A delete lists one key under a deadline far inside it, so a mark this
// old is a delete that died mid-way and refuses nothing.
const objectBucketDeletingTTL = 2 * time.Minute

// DeleteObjectBucketIfEmpty removes the live bucket row and its usage row when
// empty reports the bucket's prefix holds no object. The row is the only
// record that maps the objects' key prefix, so deleting a non-empty bucket, or
// one an upload lands in during the check, would orphan them (w4/678):
//
//   - The row is marked deleting first, in its own statement, so a put that
//     starts during the delete refuses before its first byte
//     (CheckObjectBucketWritable).
//   - empty runs and the row goes while the delete holds the row FOR UPDATE,
//     which a finished put's FOR SHARE read waits on (ConfirmObjectBucketLive).
//     A put that confirms first has its object in Wasabi before empty lists, so
//     the delete refuses; one that confirms after finds no row and removes its
//     object.
//
// A bucket that is not empty, or whose check failed, keeps its row and loses
// its mark; deleted is false and err is empty's error.
func (s *PGStore) DeleteObjectBucketIfEmpty(ctx context.Context, workspaceID, id string, empty func(context.Context) (bool, error)) (deleted bool, err error) {
	tag, err := s.Pool.Exec(ctx,
		`UPDATE object_buckets SET deleting_since = now() WHERE id = $1 AND workspace_id = $2 AND deleted_at IS NULL`,
		id, workspaceID)
	if err != nil {
		return false, classify("object bucket", err)
	}
	if tag.RowsAffected() == 0 {
		return false, fmt.Errorf("object bucket %s: %w", id, ErrNotFound)
	}
	var checkErr error
	err = s.withTx(ctx, func(tx pgx.Tx) error {
		var one int
		if err := tx.QueryRow(ctx,
			`SELECT 1 FROM object_buckets WHERE id = $1 AND workspace_id = $2 AND deleted_at IS NULL FOR UPDATE`,
			id, workspaceID).Scan(&one); err != nil {
			return err
		}
		ok, err := empty(ctx)
		if err != nil || !ok {
			checkErr = err
			_, err := tx.Exec(ctx, `UPDATE object_buckets SET deleting_since = NULL WHERE id = $1`, id)
			return err
		}
		if _, err := tx.Exec(ctx, `DELETE FROM object_buckets WHERE id = $1`, id); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `DELETE FROM object_storage_usage WHERE bucket_id = $1`, id); err != nil {
			return err
		}
		deleted = true
		return nil
	})
	if err != nil {
		return false, classify("object bucket", err)
	}
	return deleted, checkErr
}

// CheckObjectBucketWritable is the check a streamed put makes before its first
// byte: ErrNotFound for a bucket with no live row, ErrObjectBucketDeleting for
// one a delete is checking.
func (s *PGStore) CheckObjectBucketWritable(ctx context.Context, workspaceID, id string) error {
	var deleting bool
	err := s.Pool.QueryRow(ctx,
		`SELECT deleting_since IS NOT NULL AND deleting_since > now() - $3 * interval '1 second'
		 FROM object_buckets WHERE id = $1 AND workspace_id = $2 AND deleted_at IS NULL`,
		id, workspaceID, objectBucketDeletingTTL.Seconds()).Scan(&deleting)
	if err != nil {
		return classify("object bucket", err)
	}
	if deleting {
		return ErrObjectBucketDeleting
	}
	return nil
}

// ConfirmObjectBucketLive is the check a streamed put makes once its object is
// in Wasabi: ErrNotFound when the bucket's live row is gone. Its FOR SHARE read
// waits for a delete's emptiness check to decide (DeleteObjectBucketIfEmpty),
// so a row it finds, marked or not, is one no delete can drop without seeing
// the object. Puts do not wait on each other.
func (s *PGStore) ConfirmObjectBucketLive(ctx context.Context, workspaceID, id string) error {
	var one int
	err := s.Pool.QueryRow(ctx,
		`SELECT 1 FROM object_buckets WHERE id = $1 AND workspace_id = $2 AND deleted_at IS NULL FOR SHARE`,
		id, workspaceID).Scan(&one)
	return classify("object bucket", err)
}

// SetObjectBucketPublic turns public reads of a bucket on or off (ADR112 D4).
func (s *PGStore) SetObjectBucketPublic(ctx context.Context, workspaceID, id string, public bool) error {
	return s.updateObjectBucket(ctx, workspaceID, id, `public = $3`, public)
}

// SetObjectBucketCORS stores the bucket's CORS rules as JSON.
func (s *PGStore) SetObjectBucketCORS(ctx context.Context, workspaceID, id string, rules json.RawMessage) error {
	return s.updateObjectBucket(ctx, workspaceID, id, `cors = $3`, orEmptyRules(rules))
}

// SetObjectBucketLifecycle stores the bucket's lifecycle rules as JSON.
func (s *PGStore) SetObjectBucketLifecycle(ctx context.Context, workspaceID, id string, rules json.RawMessage) error {
	return s.updateObjectBucket(ctx, workspaceID, id, `lifecycle = $3`, orEmptyRules(rules))
}

// orEmptyRules turns "no rules" into the empty array the NOT NULL columns hold.
func orEmptyRules(rules json.RawMessage) json.RawMessage {
	if len(rules) == 0 {
		return json.RawMessage("[]")
	}
	return rules
}

func (s *PGStore) updateObjectBucket(ctx context.Context, workspaceID, id, set string, value any) error {
	tag, err := s.Pool.Exec(ctx,
		`UPDATE object_buckets SET `+set+` WHERE id = $1 AND workspace_id = $2 AND deleted_at IS NULL`,
		id, workspaceID, value)
	if err != nil {
		return classify("object bucket", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("object bucket %s: %w", id, ErrNotFound)
	}
	return nil
}

func lockObjectBuckets(ctx context.Context, tx pgx.Tx, workspaceID string) error {
	_, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('object_buckets:' || $1, 0))`, workspaceID)
	return err
}

func countUserObjectBuckets(ctx context.Context, tx pgx.Tx, workspaceID string) (int, error) {
	var n int
	err := tx.QueryRow(ctx,
		`SELECT count(*) FROM object_buckets WHERE workspace_id = $1 AND NOT system AND deleted_at IS NULL`,
		workspaceID).Scan(&n)
	return n, err
}

// ObjectStorageUsage is what the hourly sweep last saw under one bucket's key
// prefix (ADR112 D8).
type ObjectStorageUsage struct {
	BucketID    string
	WorkspaceID string
	Region      string
	Bytes       int64
	Objects     int64
	ObservedAt  time.Time
}

// UpsertObjectStorageUsage records the sweep's observation of one bucket.
func (s *PGStore) UpsertObjectStorageUsage(ctx context.Context, u ObjectStorageUsage) error {
	_, err := s.Pool.Exec(ctx, `
		INSERT INTO object_storage_usage (bucket_id, workspace_id, region, bytes, objects, observed_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (bucket_id) DO UPDATE
		SET workspace_id = EXCLUDED.workspace_id, region = EXCLUDED.region,
		    bytes = EXCLUDED.bytes, objects = EXCLUDED.objects, observed_at = EXCLUDED.observed_at`,
		u.BucketID, u.WorkspaceID, u.Region, u.Bytes, u.Objects, u.ObservedAt)
	return err
}

// ObjectStorageUsageTotals sums the last observed bytes and objects of every
// bucket of the workspace in one region. A workspace the sweep has not seen
// reads as zero.
func (s *PGStore) ObjectStorageUsageTotals(ctx context.Context, workspaceID, region string) (bytes, objects int64, err error) {
	err = s.Pool.QueryRow(ctx, `
		SELECT COALESCE(sum(bytes), 0)::bigint, COALESCE(sum(objects), 0)::bigint
		FROM object_storage_usage WHERE workspace_id = $1 AND region = $2`,
		workspaceID, region).Scan(&bytes, &objects)
	return bytes, objects, err
}

// ListObjectStorageUsage returns the last observation of each of the
// workspace's buckets, in bucket id order.
func (s *PGStore) ListObjectStorageUsage(ctx context.Context, workspaceID string) ([]ObjectStorageUsage, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT bucket_id, workspace_id, region, bytes, objects, observed_at
		FROM object_storage_usage WHERE workspace_id = $1 ORDER BY bucket_id`, workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ObjectStorageUsage
	for rows.Next() {
		var u ObjectStorageUsage
		if err := rows.Scan(&u.BucketID, &u.WorkspaceID, &u.Region, &u.Bytes, &u.Objects, &u.ObservedAt); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}
