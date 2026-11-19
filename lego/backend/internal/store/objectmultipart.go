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

// ErrMultipartQuota refuses an upload past the workspace's open-upload cap.
var ErrMultipartQuota = fmt.Errorf("multipart upload limit reached: %w", ErrConflict)

// ObjectMultipartUpload is one unfinished S3 multipart upload (ADR112 D3).
type ObjectMultipartUpload struct {
	ID             string
	WorkspaceID    string
	BucketID       string
	Region         string
	Key            string
	WasabiUploadID string
	CreatedBy      string
	CreatedAt      time.Time
	LastPartAt     time.Time
}

// ObjectMultipartPart is one uploaded part of an upload.
type ObjectMultipartPart struct {
	UploadID   string
	PartNumber int
	Size       int64
	ETag       string
	UploadedAt time.Time
}

const multipartColumns = `id, workspace_id, bucket_id, region, key, wasabi_upload_id, created_by, created_at, last_part_at`

func scanMultipart(row pgx.Row) (ObjectMultipartUpload, error) {
	var u ObjectMultipartUpload
	err := row.Scan(&u.ID, &u.WorkspaceID, &u.BucketID, &u.Region, &u.Key, &u.WasabiUploadID, &u.CreatedBy, &u.CreatedAt, &u.LastPartAt)
	return u, err
}

func (s *PGStore) queryMultipart(ctx context.Context, where string, args ...any) ([]ObjectMultipartUpload, error) {
	rows, err := s.Pool.Query(ctx, `SELECT `+multipartColumns+` FROM object_multipart_uploads `+where, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ObjectMultipartUpload
	for rows.Next() {
		u, err := scanMultipart(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// CreateObjectMultipartUpload inserts an upload row, refusing one past max open uploads of
// the workspace (counted under the workspace's advisory lock, so two creates cannot both
// pass).
func (s *PGStore) CreateObjectMultipartUpload(ctx context.Context, in ObjectMultipartUpload, maxOpen int) (ObjectMultipartUpload, error) {
	var out ObjectMultipartUpload
	err := pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext('object_multipart:' || $1))`, in.WorkspaceID); err != nil {
			return err
		}
		var open int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM object_multipart_uploads WHERE workspace_id = $1`, in.WorkspaceID).Scan(&open); err != nil {
			return err
		}
		if open >= maxOpen {
			return ErrMultipartQuota
		}
		var err error
		out, err = scanMultipart(tx.QueryRow(ctx, `
			INSERT INTO object_multipart_uploads (id, workspace_id, bucket_id, region, key, wasabi_upload_id, created_by)
			VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING `+multipartColumns,
			in.ID, in.WorkspaceID, in.BucketID, in.Region, in.Key, in.WasabiUploadID, in.CreatedBy))
		return err
	})
	return out, err
}

// GetObjectMultipartUpload is an upload of the workspace, or ErrNotFound: another
// workspace's id is simply not found.
func (s *PGStore) GetObjectMultipartUpload(ctx context.Context, workspaceID, id string) (ObjectMultipartUpload, error) {
	u, err := scanMultipart(s.Pool.QueryRow(ctx, `SELECT `+multipartColumns+` FROM object_multipart_uploads WHERE id = $1 AND workspace_id = $2`, id, workspaceID))
	if err != nil {
		return ObjectMultipartUpload{}, classify("multipart upload", err)
	}
	return u, nil
}

// ListObjectMultipartUploads lists a bucket's open uploads ordered by key and id, after the
// (key, id) marker, at most limit.
func (s *PGStore) ListObjectMultipartUploads(ctx context.Context, workspaceID, bucketID, prefix, afterKey, afterID string, limit int) ([]ObjectMultipartUpload, error) {
	return s.queryMultipart(ctx, `
		WHERE workspace_id = $1 AND bucket_id = $2 AND starts_with(key, $3) AND (key, id) > ($4, $5)
		ORDER BY key, id LIMIT $6`, workspaceID, bucketID, prefix, afterKey, afterID, limit)
}

// UpsertObjectMultipartPart records a part of an upload, replacing one of the same number,
// and moves the upload's last_part_at. An upload that is gone is ErrNotFound.
func (s *PGStore) UpsertObjectMultipartPart(ctx context.Context, uploadID string, part int, size int64, etag string) error {
	tag, err := s.Pool.Exec(ctx, `
		WITH touched AS (UPDATE object_multipart_uploads SET last_part_at = now() WHERE id = $1 RETURNING id)
		INSERT INTO object_multipart_parts (upload_id, part_number, size, etag)
		SELECT id, $2, $3, $4 FROM touched
		ON CONFLICT (upload_id, part_number) DO UPDATE SET size = EXCLUDED.size, etag = EXCLUDED.etag, uploaded_at = now()`,
		uploadID, part, size, etag)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("multipart upload %s: %w", uploadID, ErrNotFound)
	}
	return nil
}

// ListObjectMultipartParts lists an upload's parts by number, after a part number, at most limit.
func (s *PGStore) ListObjectMultipartParts(ctx context.Context, uploadID string, after, limit int) ([]ObjectMultipartPart, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT upload_id, part_number, size, etag, uploaded_at FROM object_multipart_parts
		WHERE upload_id = $1 AND part_number > $2 ORDER BY part_number LIMIT $3`, uploadID, after, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ObjectMultipartPart
	for rows.Next() {
		var p ObjectMultipartPart
		if err := rows.Scan(&p.UploadID, &p.PartNumber, &p.Size, &p.ETag, &p.UploadedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// ObjectMultipartUploadBytes is the bytes of one upload's parts other than part exceptPart
// (0 excepts none): a part sent again replaces the earlier one, so the earlier one is not
// part of what the new one adds to.
func (s *PGStore) ObjectMultipartUploadBytes(ctx context.Context, id string, exceptPart int) (int64, error) {
	var total int64
	err := s.Pool.QueryRow(ctx, `SELECT COALESCE(sum(size), 0)::bigint FROM object_multipart_parts WHERE upload_id = $1 AND part_number <> $2`, id, exceptPart).Scan(&total)
	return total, err
}

// DeleteObjectMultipartUpload removes an upload and, by cascade, its parts.
func (s *PGStore) DeleteObjectMultipartUpload(ctx context.Context, id string) error {
	_, err := s.Pool.Exec(ctx, `DELETE FROM object_multipart_uploads WHERE id = $1`, id)
	return err
}

// OpenMultipartBytes is the bytes of a workspace's unfinished uploads in a region, which
// the quota gate counts until they complete or abort.
func (s *PGStore) OpenMultipartBytes(ctx context.Context, workspaceID, region string) (int64, error) {
	var total int64
	err := s.Pool.QueryRow(ctx, `
		SELECT COALESCE(sum(p.size), 0)::bigint
		FROM object_multipart_parts p JOIN object_multipart_uploads u ON u.id = p.upload_id
		WHERE u.workspace_id = $1 AND u.region = $2`, workspaceID, region).Scan(&total)
	return total, err
}

// ObjectBucketHasMultipartUploads reports whether a bucket has an unfinished upload, which
// keeps it from being deleted, as on S3.
func (s *PGStore) ObjectBucketHasMultipartUploads(ctx context.Context, workspaceID, bucketID string) (bool, error) {
	var open bool
	err := s.Pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM object_multipart_uploads WHERE workspace_id = $1 AND bucket_id = $2)`, workspaceID, bucketID).Scan(&open)
	return open, err
}

// StaleObjectMultipartUploads lists uploads with no part since cutoff, oldest first, at most limit.
func (s *PGStore) StaleObjectMultipartUploads(ctx context.Context, cutoff time.Time, limit int) ([]ObjectMultipartUpload, error) {
	return s.queryMultipart(ctx, `WHERE last_part_at < $1 ORDER BY last_part_at LIMIT $2`, cutoff, limit)
}

// ObjectMultipartUploadsOfWorkspace lists a workspace's uploads, for its purge to abort them at Wasabi.
func (s *PGStore) ObjectMultipartUploadsOfWorkspace(ctx context.Context, workspaceID string, limit int) ([]ObjectMultipartUpload, error) {
	return s.queryMultipart(ctx, `WHERE workspace_id = $1 ORDER BY created_at LIMIT $2`, workspaceID, limit)
}

// ObjectMultipartUploadsStartedBefore lists a bucket's uploads started before cutoff under a
// key prefix: what a lifecycle AbortIncompleteMultipartUpload rule aborts.
func (s *PGStore) ObjectMultipartUploadsStartedBefore(ctx context.Context, workspaceID, bucketID, prefix string, cutoff time.Time, limit int) ([]ObjectMultipartUpload, error) {
	return s.queryMultipart(ctx, `WHERE workspace_id = $1 AND bucket_id = $2 AND starts_with(key, $3) AND created_at < $4 ORDER BY created_at LIMIT $5`, workspaceID, bucketID, prefix, cutoff, limit)
}
