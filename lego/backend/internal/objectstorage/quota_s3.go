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
	"fmt"
	"hash/fnv"
	"log"
	"sync"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

// The S3 surface's quota accounting (ADR112 D8). bex sees the length of an S3 put first
// and its bytes after, so the gate is the last observation of the region plus what bex
// has itself written since (a write-through counter shared by every replica), plus what
// this replica has in flight, plus the unfinished multipart bytes, plus the new write.
// Deletes and overwrites free quota only at the next observation (up to 72 hours for a
// large region): a `sync --delete` that frees space and refills it inside that window is
// refused. Counting them incrementally, which bex can now do for what passes through it,
// is ADR112 D10.

// WriteCounter is the write-through counter's store. *store.PGStore implements it; a
// repository without it leaves the gate on the last observation alone.
type WriteCounter interface {
	AddObjectStorageWrite(ctx context.Context, workspaceID, region string, bytes, objects int64) error
	ObjectStorageWriteTotals(ctx context.Context, workspaceID, region string) (bytes, objects int64, err error)
}

// EgressReader reads a workspace's month-to-date egress meter.
type EgressReader interface {
	ObjectStorageWorkspaceEgressSince(ctx context.Context, workspaceID string, since time.Time) (int64, error)
}

// MultipartBytes reports the bytes of a workspace's unfinished multipart uploads in a
// region, which an object listing does not see (ADR112 D3).
type MultipartBytes interface {
	OpenMultipartBytes(ctx context.Context, workspaceID, region string) (bytes int64, err error)
}

// inflight is the declared length and count of this replica's puts and parts in flight.
type inflight struct {
	mu      sync.Mutex
	bytes   map[string]int64
	objects map[string]int64
}

func newInflight() *inflight {
	return &inflight{bytes: map[string]int64{}, objects: map[string]int64{}}
}

func inflightKey(workspaceID string, region Region) string { return workspaceID + "|" + string(region) }

func (f *inflight) add(key string, bytes, objects int64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.bytes[key] += bytes
	f.objects[key] += objects
	if f.bytes[key] == 0 && f.objects[key] == 0 {
		delete(f.bytes, key)
		delete(f.objects, key)
	}
}

func (f *inflight) get(key string) (bytes, objects int64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.bytes[key], f.objects[key]
}

// gateShards is how many locks serialize the check-and-reserve of GateAndReserve: one
// workspace's gate must not interleave with itself, but it makes database reads, so
// every workspace on a replica must not queue behind one lock.
const gateShards = 64

func gateShard(workspaceID string) int {
	h := fnv.New32a()
	_, _ = h.Write([]byte(workspaceID))
	return int(h.Sum32() % gateShards)
}

// GateAndReserve is Gate for a stream bex is about to take: it refuses a write that
// would pass a cap and, when it does not, holds the write's declared length against the
// workspace's region until release is called, so a second put on this replica sees it.
// One at a time: the check and the reservation must not interleave.
func (s *Service) GateAndReserve(ctx context.Context, workspaceID string, region Region, size int64) (release func(), err error) {
	mu := &s.gateMu[gateShard(workspaceID)]
	mu.Lock()
	defer mu.Unlock()
	if err := s.Gate(ctx, workspaceID, region, size); err != nil {
		return nil, err
	}
	key := inflightKey(workspaceID, region)
	s.inflight.add(key, size, 1)
	var once sync.Once
	return func() { once.Do(func() { s.inflight.add(key, -size, -1) }) }, nil
}

// RecordWrite counts a completed write into the shared write-through counter. It never
// fails the request: the object is stored, and the next observation corrects a miss.
func (s *Service) RecordWrite(ctx context.Context, workspaceID string, region Region, bytes int64) {
	wc, ok := s.repo.(WriteCounter)
	if !ok {
		return
	}
	if err := wc.AddObjectStorageWrite(context.WithoutCancel(ctx), workspaceID, string(region), bytes, 1); err != nil {
		log.Printf("object storage: count a write of %d bytes into %s/%s: %v", bytes, workspaceID, region, err)
	}
}

// pendingUsage is what the gate adds to the observation.
func (s *Service) pendingUsage(ctx context.Context, workspaceID string, region Region) (bytes, objects int64, err error) {
	if wc, ok := s.repo.(WriteCounter); ok {
		if bytes, objects, err = wc.ObjectStorageWriteTotals(ctx, workspaceID, string(region)); err != nil {
			return 0, 0, fmt.Errorf("read the write-through counter: %w", err)
		}
	}
	heldBytes, heldObjects := s.inflight.get(inflightKey(workspaceID, region))
	bytes, objects = bytes+heldBytes, objects+heldObjects
	if mp, ok := s.repo.(MultipartBytes); ok {
		open, err := mp.OpenMultipartBytes(ctx, workspaceID, string(region))
		if err != nil {
			return 0, 0, fmt.Errorf("read the unfinished multipart bytes: %w", err)
		}
		bytes += open
	}
	return bytes, objects, nil
}

// QuotaEgress names the monthly S3 egress budget in a QuotaError.
const QuotaEgress QuotaResource = "egress"

// CheckEgressBudget refuses an S3 read once the workspace's month-to-date egress passes
// the larger of the bytes it stores and S3EgressBudgetFloor (ADR112 D3): a rate does not
// bound a month, and an authenticated download streams from the same shared Wasabi
// account, with its allowance, as a public read does. The meter flushes once a minute and
// the read is cached for half of it.
func (s *Service) CheckEgressBudget(ctx context.Context, workspaceID string) error {
	er, ok := s.repo.(EgressReader)
	if !ok {
		return nil
	}
	now := s.nowFunc()
	egress, err := s.egressCache.Load(ctx, workspaceID, EgressBudgetCacheTTL, func(ctx context.Context) (int64, error) {
		return er.ObjectStorageWorkspaceEgressSince(ctx, workspaceID, store.MonthStartUTC(now))
	})
	if err != nil {
		return fmt.Errorf("read the egress meter: %w", err)
	}
	if egress < S3EgressBudgetFloor {
		return nil
	}
	var stored int64
	for _, r := range Regions {
		b, _, err := s.repo.ObjectStorageUsageTotals(ctx, workspaceID, string(r))
		if err != nil {
			return fmt.Errorf("read object storage usage: %w", err)
		}
		stored += b
	}
	if limit := max(stored, S3EgressBudgetFloor); egress >= limit {
		return &QuotaError{Resource: QuotaEgress, Limit: limit, Resets: store.MonthStartUTC(now).AddDate(0, 1, 0)}
	}
	return nil
}

// EgressBudgetCacheTTL is how long a workspace's month-to-date egress is trusted.
const EgressBudgetCacheTTL = 30 * time.Second

// InflightForTest is the declared bytes and objects this replica holds in flight in a
// region, for tests of the gate.
func (s *Service) InflightForTest(workspaceID string, region Region) (bytes, objects int64) {
	return s.inflight.get(inflightKey(workspaceID, region))
}
