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

package disksnapshot

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/s3/transfermanager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// Store is the S3-compatible object store snapshots live in. It is the same
// bucket contract the KeyValue backups use (BackupStore), reached with an
// in-process client rather than the AWS CLI: the encrypted stream is produced
// and uploaded by one process, so the plaintext never lands on a disk or
// crosses a container boundary on its way out.
type Store struct {
	Endpoint string
	Bucket   string
	Prefix   string
	Region   string
}

// SnapshotSuffix is the object extension every snapshot carries. It names the
// whole pipeline, so an operator listing the bucket can tell what a byte stream
// is without opening it — and so the retention sweep can never match an object
// some other system put in the same prefix.
const SnapshotSuffix = ".tar.gz.age"

// Object is one stored snapshot.
type Object struct {
	Key       string
	CreatedAt time.Time
	Size      int64
}

func (s Store) client(ctx context.Context) (*s3.Client, error) {
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(s.region()))
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}
	return s3.NewFromConfig(cfg, func(o *s3.Options) {
		if s.Endpoint != "" {
			o.BaseEndpoint = aws.String(s.Endpoint)
		}
		// Path-style addressing: the bucket is a path segment, not a hostname
		// label. Every S3-compatible store bex targets (Hetzner, Wasabi, MinIO)
		// needs this, and bucket names with dots break virtual-host TLS anyway.
		o.UsePathStyle = true
	}), nil
}

func (s Store) region() string {
	if s.Region != "" {
		return s.Region
	}
	// Signing needs a region even where the store ignores it.
	return "us-east-1"
}

// Put streams body to key. The upload manager splits it into parts as it
// reads, so a 10 TB volume never has to be sized, buffered, or staged first —
// the length is not known until the last byte of the tar has been produced.
func (s Store) Put(ctx context.Context, key string, body io.Reader) error {
	client, err := s.client(ctx)
	if err != nil {
		return err
	}
	uploader := transfermanager.New(client)
	if _, err := uploader.UploadObject(ctx, &transfermanager.UploadObjectInput{
		Bucket: aws.String(s.Bucket),
		Key:    aws.String(s.join(key)),
		Body:   body,
	}); err != nil {
		return fmt.Errorf("upload %s: %w", key, err)
	}
	return nil
}

// Get opens key for reading. The caller closes it.
func (s Store) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	client, err := s.client(ctx)
	if err != nil {
		return nil, err
	}
	out, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.Bucket),
		Key:    aws.String(s.join(key)),
	})
	if err != nil {
		return nil, fmt.Errorf("get %s: %w", key, err)
	}
	return out.Body, nil
}

// List returns the snapshots under prefix, oldest first.
func (s Store) List(ctx context.Context, prefix string) ([]Object, error) {
	client, err := s.client(ctx)
	if err != nil {
		return nil, err
	}
	var out []Object
	pages := s3.NewListObjectsV2Paginator(client, &s3.ListObjectsV2Input{
		Bucket: aws.String(s.Bucket),
		Prefix: aws.String(s.join(prefix)),
	})
	for pages.HasMorePages() {
		page, err := pages.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("list %s: %w", prefix, err)
		}
		for _, item := range page.Contents {
			key := aws.ToString(item.Key)
			if !strings.HasSuffix(key, SnapshotSuffix) {
				continue
			}
			out = append(out, Object{
				Key:       strings.TrimPrefix(key, s.join("")),
				CreatedAt: aws.ToTime(item.LastModified),
				Size:      aws.ToInt64(item.Size),
			})
		}
	}
	// Sort by key, not by LastModified: the key carries the snapshot's own
	// timestamp, so it stays stable if an object is ever re-uploaded, and the
	// retention sweep must never be re-ordered by a touch.
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out, nil
}

// Delete removes one object. A missing object is not an error: retention and
// purge are both idempotent sweeps that may race a previous run.
func (s Store) Delete(ctx context.Context, key string) error {
	client, err := s.client(ctx)
	if err != nil {
		return err
	}
	if _, err := client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.Bucket),
		Key:    aws.String(s.join(key)),
	}); err != nil {
		return fmt.Errorf("delete %s: %w", key, err)
	}
	return nil
}

// Prune deletes the oldest snapshots under prefix until retain remain — the
// 7-day retention Render documents. It returns what it deleted.
func (s Store) Prune(ctx context.Context, prefix string, retain int) ([]string, error) {
	if retain < 1 {
		return nil, fmt.Errorf("retain must be at least 1, got %d", retain)
	}
	objects, err := s.List(ctx, prefix)
	if err != nil {
		return nil, err
	}
	if len(objects) <= retain {
		return nil, nil
	}
	var deleted []string
	for _, obj := range objects[:len(objects)-retain] {
		if err := s.Delete(ctx, obj.Key); err != nil {
			return deleted, err
		}
		deleted = append(deleted, obj.Key)
	}
	return deleted, nil
}

// PurgeAll removes every snapshot under prefix — what a deleted disk's
// snapshots get, so a detached volume stops costing storage and stops holding
// tenant data nobody can reach any more.
func (s Store) PurgeAll(ctx context.Context, prefix string) (int, error) {
	objects, err := s.List(ctx, prefix)
	if err != nil {
		return 0, err
	}
	for i, obj := range objects {
		if err := s.Delete(ctx, obj.Key); err != nil {
			return i, err
		}
	}
	return len(objects), nil
}

func (s Store) join(key string) string {
	prefix := strings.Trim(s.Prefix, "/")
	key = strings.TrimPrefix(key, "/")
	if prefix == "" {
		return key
	}
	if key == "" {
		return prefix + "/"
	}
	return prefix + "/" + key
}

// DiskPrefix is where one disk's snapshots live: a path segment per workspace
// and per disk, so a listing is naturally scoped and a purge cannot reach
// another tenant's objects by prefix collision.
func DiskPrefix(workspaceID, diskID string) string {
	return strings.Trim(workspaceID, "/") + "/" + strings.Trim(diskID, "/") + "/"
}

// SnapshotKey names a snapshot taken at t. RFC3339 in UTC sorts lexically in
// time order, which is what lets List/Prune work on names alone.
func SnapshotKey(prefix string, t time.Time) string {
	return prefix + t.UTC().Format("2006-01-02T15:04:05Z") + SnapshotSuffix
}
