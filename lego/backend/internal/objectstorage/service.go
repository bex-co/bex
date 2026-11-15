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
)

// Repository is the part of the store the service reads and writes.
// *store.PGStore implements it.
type Repository interface {
	CreateObjectBucket(ctx context.Context, in store.ObjectBucket, userCap int) (store.ObjectBucket, error)
	EnsureSystemObjectBucket(ctx context.Context, workspaceID, name, region string) (store.ObjectBucket, error)
	GetObjectBucket(ctx context.Context, workspaceID, name string) (store.ObjectBucket, error)
	ListObjectBucketSummaries(ctx context.Context, workspaceID string) ([]store.ObjectBucket, error)
	DeleteObjectBucket(ctx context.Context, workspaceID, id string) error
	ObjectStorageUsageTotals(ctx context.Context, workspaceID, region string) (bytes, objects int64, err error)
	ListObjectStorageUsage(ctx context.Context, workspaceID string) ([]store.ObjectStorageUsage, error)
	SetObjectBucketCORS(ctx context.Context, workspaceID, id string, rules json.RawMessage) error
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

// DeleteBucket deletes an empty bucket. The row is the only record mapping the
// objects' key prefix, so deleting a non-empty bucket would orphan them: the
// check lists one key under the bucket's prefix first. ADR112 is silent on
// deleting a system bucket, so it follows the same rule.
func (s *Service) DeleteBucket(ctx context.Context, workspaceID, name string) error {
	b, err := s.GetBucket(ctx, workspaceID, name)
	if err != nil {
		return err
	}
	st, err := s.Store(Region(b.Region))
	if err != nil {
		return err
	}
	page, err := st.List(ctx, Prefix(workspaceID, b.ID), "", 1)
	if err != nil {
		return fmt.Errorf("check bucket %s is empty: %w", name, err)
	}
	if len(page.Objects) > 0 {
		return ErrBucketNotEmpty
	}
	if err := s.repo.DeleteObjectBucket(ctx, workspaceID, b.ID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return ErrBucketNotFound
		}
		return err
	}
	return nil
}
