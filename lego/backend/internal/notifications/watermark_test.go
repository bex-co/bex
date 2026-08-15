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

package notifications

import (
	"context"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

// The push worker tails the same composed feed as internal/webhooks, and since
// the pager itself was hoisted to store.TailFeed the two no longer run separate
// copies of the algorithm — internal/store/feedtail_test.go owns its rules.
//
// What these tests own now is this worker's WIRING of that seam, which the seam
// test structurally cannot reach: the pager takes its page size, park interval
// and tenant set as fields of a struct literal, so a knob can be dropped and
// still compile. Asserting the rules end to end through RunOnce is what pins
// them — deleting these as "duplicates" of the seam test loses that. (Verified:
// mis-wiring the page size fails here and passes there.)
//
// Every failure below is silent in production — events skipped, or redelivered
// forever, with no error anywhere.

// countingPushStore observes how the pager drives the store.
type countingPushStore struct {
	*fakePushWorkerStore
	reads  int
	writes []pushWMWrite
}

type pushWMWrite struct {
	items int
	at    time.Time
	key   string
}

func (c *countingPushStore) ListWebhookEvents(ctx context.Context, afterAt time.Time, afterKey string, until time.Time, verbs, tenants []string, limit int) ([]store.WebhookEventRow, error) {
	c.reads++
	return c.fakePushWorkerStore.ListWebhookEvents(ctx, afterAt, afterKey, until, verbs, tenants, limit)
}

func (c *countingPushStore) EnqueuePushNotifications(ctx context.Context, items []store.PushNotificationBatchItem, at time.Time, key string) error {
	c.writes = append(c.writes, pushWMWrite{items: len(items), at: at, key: key})
	return c.fakePushWorkerStore.EnqueuePushNotifications(ctx, items, at, key)
}

func countingPushWorker(now time.Time) (*PushWorker, *countingPushStore) {
	st := &countingPushStore{fakePushWorkerStore: newFakePushWorkerStore()}
	st.serviceIDs["tea-one\x00api"] = "srv-c185th5c2rvvnhbfiltg"
	st.destinations = []store.ActivePushSubscription{{
		TenantID: "tea-one", Subject: "alice", Role: "admin", DeviceID: "alice-ios",
		Provider: "expo", Platform: "ios", Token: "token-alice-ios",
		CreatedAt: now.Add(-24 * time.Hour),
	}}
	return &PushWorker{Store: st, Clock: func() time.Time { return now }}, st
}

func pushDeployRow(key string, at time.Time) store.WebhookEventRow {
	return store.WebhookEventRow{
		CursorAt: at, Key: key, At: at, TenantID: "tea-one", ServiceName: "api",
		Source: store.EventSourceDeploy, Phase: store.EventPhaseEnded,
		Status: store.DeployUpdateFailed,
	}
}

// A quiet window must not write the cursor forward on every tick: that is one
// Postgres write per poll interval forever on an idle platform.
func TestPushQuietWindowParksOnlyOncePastTheParkInterval(t *testing.T) {
	now := time.Date(2026, 8, 3, 12, 0, 10, 0, time.UTC)
	until := now.Add(-pushDispatchLag)

	t.Run("inside the park interval writes nothing", func(t *testing.T) {
		w, st := countingPushWorker(now)
		st.watermarkAt = until.Add(-pushParkInterval / 2)
		if err := w.dispatch(context.Background(), st.destinations); err != nil {
			t.Fatalf("dispatch: %v", err)
		}
		if len(st.writes) != 0 {
			t.Errorf("wrote the cursor %d times inside the park interval, want 0: %+v", len(st.writes), st.writes)
		}
	})

	t.Run("past the park interval writes once, with no notifications", func(t *testing.T) {
		w, st := countingPushWorker(now)
		st.watermarkAt = until.Add(-2 * pushParkInterval)
		if err := w.dispatch(context.Background(), st.destinations); err != nil {
			t.Fatalf("dispatch: %v", err)
		}
		if len(st.writes) != 1 {
			t.Fatalf("cursor writes = %d, want exactly 1: %+v", len(st.writes), st.writes)
		}
		if got := st.writes[0]; got.items != 0 || !got.at.Equal(until) || got.key != "" {
			t.Errorf("park write = %+v, want an empty batch parked at %v with no key", got, until)
		}
	})
}

// The notifications and the cursor advance in ONE call: splitting them either
// drops events (advance first, crash) or redelivers them (insert first, crash).
func TestPushBatchAndWatermarkAdvanceInOneStoreCall(t *testing.T) {
	now := time.Date(2026, 8, 3, 12, 0, 10, 0, time.UTC)
	eventAt := now.Add(-time.Minute)
	w, st := countingPushWorker(now)
	st.watermarkAt = eventAt.Add(-time.Hour)
	st.events = []store.WebhookEventRow{
		pushDeployRow("dep-one:ended", eventAt),
		pushDeployRow("dep-two:ended", eventAt.Add(time.Second)),
	}

	if err := w.dispatch(context.Background(), st.destinations); err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if len(st.writes) != 1 {
		t.Fatalf("store calls = %d, want exactly 1 carrying both the batch and the cursor: %+v", len(st.writes), st.writes)
	}
	got := st.writes[0]
	if got.items == 0 {
		t.Error("enqueued nothing; the page projected no notifications")
	}
	if got.key != "dep-two:ended" || !got.at.Equal(eventAt.Add(time.Second)) {
		t.Errorf("cursor advanced to (%v,%q), want the last row of the page", got.at, got.key)
	}
}

// A full page is followed by another read; a short page ends the pass. Always
// stopping drains a backlog one page per tick forever.
func TestPushFullPageLoopsAndShortPageEndsThePass(t *testing.T) {
	now := time.Date(2026, 8, 3, 12, 0, 10, 0, time.UTC)
	eventAt := now.Add(-time.Hour)

	t.Run("short page reads once", func(t *testing.T) {
		w, st := countingPushWorker(now)
		st.watermarkAt = eventAt.Add(-time.Hour)
		st.events = []store.WebhookEventRow{pushDeployRow("dep-one:ended", eventAt)}
		if err := w.dispatch(context.Background(), st.destinations); err != nil {
			t.Fatalf("dispatch: %v", err)
		}
		if st.reads != 1 {
			t.Errorf("reads = %d, want 1 for a short page", st.reads)
		}
	})

	t.Run("full page reads again", func(t *testing.T) {
		w, st := countingPushWorker(now)
		st.watermarkAt = eventAt.Add(-time.Hour)
		for i := range pushDispatchBatch {
			st.events = append(st.events, pushDeployRow(
				"dep-"+string(rune('a'+i%26))+string(rune('a'+i/26))+":ended",
				eventAt.Add(time.Duration(i)*time.Millisecond),
			))
		}
		if err := w.dispatch(context.Background(), st.destinations); err != nil {
			t.Fatalf("dispatch: %v", err)
		}
		if st.reads != 2 {
			t.Errorf("reads = %d, want 2: a full page must be followed by another read", st.reads)
		}
	})
}

// A device never receives events from before it was registered — the push-side
// equivalent of the webhook endpoint's CreatedAt guard. Without it, registering
// a phone replays whatever backlog the shared cursor happens to be sitting on.
//
// A second, older device is present on purpose: it keeps the events inside the
// page so the NEW device's guard is what rejects them, not an upstream filter.
func TestPushNoEventIsDeliveredFromBeforeItsDeviceWasRegistered(t *testing.T) {
	now := time.Date(2026, 8, 3, 12, 0, 10, 0, time.UTC)
	old := now.Add(-24 * time.Hour)
	w, st := countingPushWorker(now)
	st.watermarkAt = old.Add(-time.Hour)
	st.destinations = []store.ActivePushSubscription{
		{
			TenantID: "tea-one", Subject: "alice", Role: "admin", DeviceID: "alice-old",
			Provider: "expo", Platform: "ios", Token: "token-old", CreatedAt: old.Add(-time.Hour),
		},
		{
			TenantID: "tea-one", Subject: "alice", Role: "admin", DeviceID: "alice-new",
			Provider: "expo", Platform: "android", Token: "token-new", CreatedAt: now.Add(-time.Hour),
		},
	}
	st.events = []store.WebhookEventRow{
		pushDeployRow("dep-old:ended", old),
		pushDeployRow("dep-new:ended", now.Add(-30*time.Minute)),
	}

	if err := w.dispatch(context.Background(), st.destinations); err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	perDevice := map[string]int{}
	for _, d := range st.deliveries {
		perDevice[d.deviceID]++
	}
	if perDevice["alice-old"] != 2 {
		t.Errorf("the pre-existing device got %d deliveries, want both events", perDevice["alice-old"])
	}
	if perDevice["alice-new"] != 1 {
		t.Errorf("the device registered an hour ago got %d deliveries, want only the event that postdates it", perDevice["alice-new"])
	}
}
