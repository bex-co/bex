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

package objectstorage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// Errors the adapters map to each contract's status and error codes.
var (
	// ErrNotConfigured answers every call while no credential is installed.
	ErrNotConfigured  = errors.New("object storage is not configured")
	ErrBucketNotFound = errors.New("bucket not found")
	ErrBucketExists   = errors.New("bucket name is taken")
	ErrBucketNotEmpty = errors.New("bucket is not empty")
	ErrTooManyBuckets = errors.New("bucket limit reached")
	// ErrSystemBucket refuses deleting a render-<region> bucket.
	ErrSystemBucket = errors.New("render-<region> is the Render CLI's object namespace and cannot be deleted")
	// ErrBucketDeleting refuses a put into a bucket a delete is checking.
	ErrBucketDeleting = errors.New("the bucket is being deleted")
)

// Repository is the part of the store the service reads and writes.
// *store.PGStore implements it.
type Repository interface {
	CreateObjectBucket(ctx context.Context, in store.ObjectBucket, userCap int) (store.ObjectBucket, error)
	EnsureSystemObjectBucket(ctx context.Context, workspaceID, name, region string) (store.ObjectBucket, error)
	GetObjectBucket(ctx context.Context, workspaceID, name string) (store.ObjectBucket, error)
	ListObjectBucketSummaries(ctx context.Context, workspaceID string) ([]store.ObjectBucket, error)
	DeleteObjectBucketIfEmpty(ctx context.Context, workspaceID, id string, empty func(context.Context) (bool, error)) (deleted bool, err error)
	CheckObjectBucketWritable(ctx context.Context, workspaceID, id string) error
	ConfirmObjectBucketLive(ctx context.Context, workspaceID, id string) error
	ObjectStorageUsageTotals(ctx context.Context, workspaceID, region string) (bytes, objects int64, err error)
	ListObjectStorageUsage(ctx context.Context, workspaceID string) ([]store.ObjectStorageUsage, error)
	SetObjectBucketCORS(ctx context.Context, workspaceID, id string, rules json.RawMessage) error
	SetObjectBucketPublic(ctx context.Context, workspaceID, id string, public bool) error
	GetObjectBucketByID(ctx context.Context, id string) (store.ObjectBucket, error)
	SetObjectBucketLifecycle(ctx context.Context, workspaceID, id string, rules json.RawMessage) error
}

// Service is the object store's core (ADR006: one core, thin adapters). The
// Render and Cloudflare routes call it; it holds no HTTP.
type Service struct {
	repo    Repository
	stores  map[Region]Store
	streams *StreamLimiter
	mints   *core.KeyedRateLimiter[string]
	// streamDeadline bounds one streamed put or get against Wasabi.
	streamDeadline time.Duration
	// meter sums the transfer shadow meters until RunMeter flushes them; nil
	// (the default) records nothing. SetUsageSink installs it.
	meter *transferMeter
	// readPace and writePace pace the S3 surface's downloads and uploads per workspace
	// (pacing.go); gateMu, inflight and egressCache are its quota accounting (quota_s3.go).
	readPace, writePace *pacer
	gateMu              [gateShards]sync.Mutex
	inflight            *inflight
	egressCache         *core.TTLCache[int64]
	// nowFunc is the clock the egress budget reads; tests replace it.
	nowFunc func() time.Time
}

// NewService returns the service. A nil or empty stores map leaves it disabled:
// every method answers ErrNotConfigured, which is how the code ships before any
// bucket exists.
func NewService(repo Repository, stores map[Region]Store) *Service {
	return &Service{
		repo: repo, stores: stores,
		streams: NewStreamLimiter(MaxConcurrentStreamsPerWorkspace, MaxConcurrentStreamsPerReplica),
		mints:   core.NewKeyedRateLimiter[string](MintPerSecond*60, MintBurst, time.Minute, time.Minute),
		// A Wasabi that stays connected and stops answering must not hold a
		// stream slot.
		streamDeadline: StreamDeadline,
		readPace:       newPacer(S3ReadBytesPerSecond), writePace: newPacer(S3WriteBytesPerSecond),
		inflight: newInflight(), egressCache: core.NewTTLCache[int64](), nowFunc: time.Now,
	}
}

// Enabled reports whether a credential is installed.
func (s *Service) Enabled() bool { return s != nil && len(s.stores) > 0 }

// Store returns a region's Wasabi store.
func (s *Service) Store(r Region) (Store, error) {
	if !s.Enabled() {
		return nil, ErrNotConfigured
	}
	st, ok := s.stores[r]
	if !ok {
		return nil, &RegionNotOfferedError{Value: string(r)}
	}
	return st, nil
}

// CreateBucket creates a user bucket in region. hint is the Cloudflare location
// hint it was asked for, kept so `bucket info` can report it.
func (s *Service) CreateBucket(ctx context.Context, workspaceID, name string, region Region, hint string) (store.ObjectBucket, error) {
	if !s.Enabled() {
		return store.ObjectBucket{}, ErrNotConfigured
	}
	if err := ValidateBucketName(name); err != nil {
		return store.ObjectBucket{}, err
	}
	if _, err := s.Store(region); err != nil {
		return store.ObjectBucket{}, err
	}
	b, err := s.repo.CreateObjectBucket(ctx, store.ObjectBucket{
		WorkspaceID: workspaceID, Name: name, Region: string(region), LocationHint: hint,
	}, MaxUserBuckets)
	switch {
	case errors.Is(err, store.ErrObjectBucketExists):
		return store.ObjectBucket{}, ErrBucketExists
	case errors.Is(err, store.ErrObjectBucketQuota):
		return store.ObjectBucket{}, ErrTooManyBuckets
	case err != nil:
		return store.ObjectBucket{}, err
	}
	return b, nil
}

// SystemBucket returns the workspace's render-<region> bucket, the namespace of
// Render's /v1/objects routes, creating it on first use.
func (s *Service) SystemBucket(ctx context.Context, workspaceID string, region Region) (store.ObjectBucket, error) {
	if _, err := s.Store(region); err != nil {
		return store.ObjectBucket{}, err
	}
	b, err := s.repo.EnsureSystemObjectBucket(ctx, workspaceID, SystemBucketName(region), string(region))
	if err != nil {
		return store.ObjectBucket{}, fmt.Errorf("system bucket %s: %w", SystemBucketName(region), err)
	}
	return b, nil
}

// GetBucket returns the workspace's live bucket called name.
func (s *Service) GetBucket(ctx context.Context, workspaceID, name string) (store.ObjectBucket, error) {
	if !s.Enabled() {
		return store.ObjectBucket{}, ErrNotConfigured
	}
	b, err := s.repo.GetObjectBucket(ctx, workspaceID, name)
	if errors.Is(err, store.ErrNotFound) {
		return store.ObjectBucket{}, ErrBucketNotFound
	}
	return b, err
}

// ListBuckets returns the workspace's live buckets, system buckets included,
// without their CORS and lifecycle rules.
func (s *Service) ListBuckets(ctx context.Context, workspaceID string) ([]store.ObjectBucket, error) {
	if !s.Enabled() {
		return nil, ErrNotConfigured
	}
	return s.repo.ListObjectBucketSummaries(ctx, workspaceID)
}

// DeleteBucket deletes an empty user bucket. The row is the only record
// mapping the objects' key prefix, so deleting a non-empty bucket would orphan
// them: the check lists one key under the bucket's prefix first, and the store
// runs it so that no streamed put can land between the check and the delete
// (store.PGStore.DeleteObjectBucketIfEmpty).
//
// A system bucket is never deleted (ErrSystemBucket). It is the Render CLI's
// whole namespace, which Render gives no way to delete, and a deleted one comes
// back under a new id on the next Render call while a URL MintPut signed
// before the delete still uploads under the old one, where nothing lists,
// reads or deletes the object (w4/678).
func (s *Service) DeleteBucket(ctx context.Context, workspaceID, name string) error {
	b, err := s.GetBucket(ctx, workspaceID, name)
	if err != nil {
		return err
	}
	if b.System {
		return ErrSystemBucket
	}
	st, err := s.Store(Region(b.Region))
	if err != nil {
		return err
	}
	deleted, err := s.repo.DeleteObjectBucketIfEmpty(ctx, workspaceID, b.ID, func(ctx context.Context) (bool, error) {
		ctx, cancel := context.WithTimeout(ctx, deleteCheckTimeout)
		defer cancel()
		page, err := st.List(ctx, Prefix(workspaceID, b.ID), "", 1)
		if err != nil {
			return false, fmt.Errorf("check bucket %s is empty: %w", name, err)
		}
		if len(page.Objects) > 0 {
			return false, nil
		}
		// An unfinished upload keeps a bucket from being deleted, as on S3: its parts are
		// in no listing, and the bucket's prefix would hold them with no row to map.
		if mp, ok := s.repo.(MultipartRepository); ok {
			open, err := mp.ObjectBucketHasMultipartUploads(ctx, workspaceID, b.ID)
			if err != nil {
				return false, fmt.Errorf("check bucket %s has no unfinished upload: %w", name, err)
			}
			return !open, nil
		}
		return true, nil
	})
	switch {
	case errors.Is(err, store.ErrNotFound):
		return ErrBucketNotFound
	case err != nil:
		return err
	case !deleted:
		return ErrBucketNotEmpty
	}
	return nil
}
