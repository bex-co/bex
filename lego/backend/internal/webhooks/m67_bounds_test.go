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

package webhooks

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// TestEndpointLimitIsRefusedWithACode is the w1/m67 F2 surface half: creation past
// the workspace cap must be a typed, human-readable refusal on every adapter, not
// a raw store error. (The transactional enforcement itself lives in the store and
// is exercised against real Postgres.)
func TestEndpointLimitIsRefusedWithACode(t *testing.T) {
	err := mapCreateErr(store.ErrWebhookEndpointLimit)

	if !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("quota refusal must map to a client error, got %v", err)
	}
	var coded *core.CodedError
	if !errors.As(err, &coded) {
		t.Fatalf("quota refusal must be machine-readable, got %T", err)
	}
	if coded.Code != EndpointLimitCode {
		t.Errorf("code = %q, want %q", coded.Code, EndpointLimitCode)
	}
	if coded.Params["limit"] != store.MaxWebhookEndpointsPerWorkspace {
		t.Errorf("params.limit = %v, want %d", coded.Params["limit"], store.MaxWebhookEndpointsPerWorkspace)
	}
	// Every other store error keeps the shared mapping.
	if got := mapCreateErr(store.ErrNotFound); !errors.Is(got, core.ErrNotFound) {
		t.Errorf("unrelated store error was reshaped: %v", got)
	}
	if mapCreateErr(nil) != nil {
		t.Error("nil must stay nil")
	}
}

// terminalDelivery is a delivered row created at `at`; pendingDelivery is one
// still awaiting delivery. The sweep must only ever see the former.
func seedDelivery(f *fakeWorkerStore, id, endpoint string, at time.Time, terminal bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	d := &store.WebhookDelivery{ID: id, EndpointID: endpoint, CreatedAt: at, NextAttemptAt: at}
	if terminal {
		delivered := at
		d.DeliveredAt = &delivered
	}
	f.queue[id] = d
	f.queueOrder = append(f.queueOrder, id)
}

// TestRetentionSweepPurgesOnlyEligibleTerminalRows is the w1/m67 F3 regression.
// webhook_deliveries is both the durable queue and the product's history view,
// and nothing ever reclaimed a delivered/exhausted row — ordinary tenant activity
// grew shared table, index, and backup storage without bound.
func TestRetentionSweepPurgesOnlyEligibleTerminalRows(t *testing.T) {
	now := time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC)
	st := newFakeWorkerStore()
	w := &Worker{Store: st, Clock: func() time.Time { return now }, RetentionDays: 30, RetentionKeepPerEndpoint: 2}

	old := now.AddDate(0, 0, -60)
	recent := now.AddDate(0, 0, -1)
	seedDelivery(st, "old-terminal", "wh-1", old, true)
	seedDelivery(st, "old-pending", "wh-1", old, false)   // still retryable: never eligible
	seedDelivery(st, "recent-1", "wh-1", recent, true)    // within age + count
	seedDelivery(st, "recent-2", "wh-1", recent, true)    // within age + count
	seedDelivery(st, "recent-3", "wh-1", recent, true)    // beyond keep=2 for this endpoint
	seedDelivery(st, "other-endpoint", "wh-2", recent, true)

	if err := w.sweepRetention(context.Background()); err != nil {
		t.Fatalf("sweepRetention: %v", err)
	}

	if len(st.sweeps) != 1 {
		t.Fatalf("sweeps = %d, want 1", len(st.sweeps))
	}
	if got, want := st.sweeps[0].before, now.AddDate(0, 0, -30); !got.Equal(want) {
		t.Errorf("age cutoff = %s, want %s", got, want)
	}
	if st.sweeps[0].keepPerEndpoint != 2 {
		t.Errorf("keepPerEndpoint = %d, want 2", st.sweeps[0].keepPerEndpoint)
	}
	if st.sweeps[0].limit <= 0 {
		t.Error("a sweep must be bounded per pass, not unbounded")
	}

	st.mu.Lock()
	defer st.mu.Unlock()
	if _, ok := st.queue["old-pending"]; !ok {
		t.Error("a delivery still awaiting delivery must never be purged")
	}
	if _, ok := st.queue["old-terminal"]; ok {
		t.Error("a terminal delivery past the age window should be purged")
	}
	if _, ok := st.queue["other-endpoint"]; !ok {
		t.Error("another endpoint's in-policy history must be untouched")
	}
	kept := 0
	for _, id := range []string{"recent-1", "recent-2", "recent-3"} {
		if _, ok := st.queue[id]; ok {
			kept++
		}
	}
	if kept != 2 {
		t.Errorf("endpoint kept %d recent rows, want the configured 2", kept)
	}
}

// The sweep rides the dispatch tick, so it must throttle itself rather than run
// on every poll.
func TestRetentionSweepIsThrottled(t *testing.T) {
	now := time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC)
	st := newFakeWorkerStore()
	w := &Worker{Store: st, Clock: func() time.Time { return now }}

	for range 5 {
		if err := w.sweepRetention(context.Background()); err != nil {
			t.Fatalf("sweepRetention: %v", err)
		}
	}
	if len(st.sweeps) != 1 {
		t.Fatalf("sweeps = %d within one interval, want 1", len(st.sweeps))
	}

	now = now.Add(sweepInterval + time.Minute)
	if err := w.sweepRetention(context.Background()); err != nil {
		t.Fatalf("sweepRetention after the interval: %v", err)
	}
	if len(st.sweeps) != 2 {
		t.Errorf("sweeps = %d after the interval elapsed, want 2", len(st.sweeps))
	}
}

// Retention must not depend on a workspace still having an enabled endpoint —
// deleting the last one previously left its whole history behind forever.
func TestRetentionRunsEvenWithNoEnabledEndpoints(t *testing.T) {
	now := time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC)
	st := newFakeWorkerStore()
	w := &Worker{Store: st, Clock: func() time.Time { return now }}

	if err := w.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if len(st.sweeps) != 1 {
		t.Errorf("sweeps = %d with no enabled endpoints, want 1", len(st.sweeps))
	}
}

// Defaults must be sane when nothing is configured.
func TestRetentionDefaults(t *testing.T) {
	now := time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC)
	st := newFakeWorkerStore()
	w := &Worker{Store: st, Clock: func() time.Time { return now }}

	if err := w.sweepRetention(context.Background()); err != nil {
		t.Fatalf("sweepRetention: %v", err)
	}
	if got, want := st.sweeps[0].before, now.AddDate(0, 0, -DefaultRetentionDays); !got.Equal(want) {
		t.Errorf("default age cutoff = %s, want %s", got, want)
	}
	if st.sweeps[0].keepPerEndpoint != defaultRetentionKeepPerEndpoint {
		t.Errorf("default keepPerEndpoint = %d, want %d", st.sweeps[0].keepPerEndpoint, defaultRetentionKeepPerEndpoint)
	}
}
