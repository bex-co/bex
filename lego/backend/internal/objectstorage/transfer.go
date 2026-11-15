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
	"log"
	"sync"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// The transfer shadow meters (ADR112 D8): object_storage_ingress_bytes and
// object_storage_egress_bytes. They are recorded on the request path, so they
// must cost one map write and never fail the request: bytes are summed in
// memory per workspace, meter and UTC hour, and a loop flushes the sums to
// usage_hourly, where they are added to the hour's row (every replica adds its
// own). A process that dies loses at most one flush interval of a shadow meter,
// which is shown at $0.

// TransferFlushInterval is how often the in-memory sums reach usage_hourly.
const TransferFlushInterval = time.Minute

// maxPendingTransfers bounds the sums held while the database is unreachable:
// one entry per workspace, meter and hour. Past it new keys are dropped, which
// is the shadow meter undercounting, never the request failing.
const maxPendingTransfers = 100_000

// UsageSink is where the transfer meters flush to. *store.PGStore implements it.
type UsageSink interface {
	AddObjectStorageUsage(ctx context.Context, deltas []store.ObjectStorageUsageDelta) error
}

type transferKey struct {
	workspaceID, kind string
	window            time.Time
}

type transferMeter struct {
	sink UsageSink

	mu      sync.Mutex
	pending map[transferKey]int64
}

// add counts n bytes of one meter for a workspace at time at.
func (m *transferMeter) add(workspaceID, kind string, n int64, at time.Time) {
	if n <= 0 || workspaceID == "" {
		return
	}
	k := transferKey{workspaceID: workspaceID, kind: kind, window: at.UTC().Truncate(time.Hour)}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, seen := m.pending[k]; !seen && len(m.pending) >= maxPendingTransfers {
		return
	}
	m.pending[k] += n
}

// flush writes the sums to the sink. On failure it keeps them for the next try.
func (m *transferMeter) flush(ctx context.Context) error {
	m.mu.Lock()
	batch := m.pending
	m.pending = map[transferKey]int64{}
	m.mu.Unlock()
	if len(batch) == 0 {
		return nil
	}
	deltas := make([]store.ObjectStorageUsageDelta, 0, len(batch))
	for k, n := range batch {
		deltas = append(deltas, store.ObjectStorageUsageDelta{WorkspaceID: k.workspaceID, Kind: k.kind, Window: k.window, Quantity: n})
	}
	if err := m.sink.AddObjectStorageUsage(ctx, deltas); err != nil {
		m.mu.Lock()
		for k, n := range batch {
			m.pending[k] += n
		}
		m.mu.Unlock()
		return err
	}
	return nil
}

// SetUsageSink turns the transfer meters on: RecordIngress and RecordEgress sum
// into memory, and RunMeter flushes the sums to sink. Without it they are no-ops,
// which is how tests and a deployment with no control-plane store run. Call it
// before the service takes requests.
func (s *Service) SetUsageSink(sink UsageSink) {
	if sink == nil {
		s.meter = nil
		return
	}
	s.meter = &transferMeter{sink: sink, pending: map[transferKey]int64{}}
}

// RunMeter flushes the transfer meters every interval until ctx ends, then once
// more, so a clean shutdown loses nothing. It returns at once when no sink is set.
func (s *Service) RunMeter(ctx context.Context, interval time.Duration) {
	if s == nil || s.meter == nil {
		return
	}
	core.Poll(ctx, "object storage transfer meter", interval, s.meter.flush)
	final, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if err := s.meter.flush(final); err != nil {
		log.Printf("object storage transfer meter: final flush: %v", err)
	}
}
