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
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// dispatch is a watermark-tailing pager: it reads a page of the composed feed
// after the durable cursor, enqueues what it projects, and advances the cursor —
// the insert and the advance in ONE store call, so a crash re-reads rather than
// drops. The pager itself now lives in store.TailFeed, shared with the
// notifications push worker, and internal/store/feedtail_test.go owns its rules.
//
// What this file owns is how THIS worker wires that seam, which the seam test
// structurally cannot reach: the pager takes its page size, park interval, verb
// filter and tenant set as fields of a struct literal, so a knob can be dropped
// and still compile. Asserting the rules end to end through dispatch is what
// pins them — deleting these as "duplicates" of the seam test loses that.
// (Verified: mis-wiring the page size fails here and passes there.)
//
// Every failure below is silent in production — events skipped, or redelivered
// forever, with no error anywhere.

// countingStore observes how the pager drives the store: how many pages it read
// and every watermark write it made.
type countingStore struct {
	*fakeWorkerStore
	reads  int
	writes []wmWrite
}

type wmWrite struct {
	items int
	at    time.Time
	key   string
}

func (c *countingStore) ListWebhookEvents(ctx context.Context, afterAt time.Time, afterKey string, until time.Time, verbs, tenants []string, limit int) ([]store.WebhookEventRow, error) {
	c.reads++
	return c.fakeWorkerStore.ListWebhookEvents(ctx, afterAt, afterKey, until, verbs, tenants, limit)
}

func (c *countingStore) EnqueueWebhookDeliveries(ctx context.Context, d []store.WebhookDelivery, at time.Time, key string) error {
	c.writes = append(c.writes, wmWrite{items: len(d), at: at, key: key})
	return c.fakeWorkerStore.EnqueueWebhookDeliveries(ctx, d, at, key)
}

func countingWorker(now time.Time) (*Worker, *countingStore) {
	st := &countingStore{fakeWorkerStore: newFakeWorkerStore()}
	st.endpoints = []store.WebhookEndpoint{
		endpoint("whk-1", "tea-a", "https://a.example/hook", "whsec_x", TypeDeployStarted),
	}
	return &Worker{Store: st, Clock: func() time.Time { return now }}, st
}

func deployRow(key string, at time.Time) store.WebhookEventRow {
	return store.WebhookEventRow{
		Key: key, At: at, TenantID: "tea-a", ServiceID: "acme-api", ServiceName: "api",
		Source: store.EventSourceDeploy, Phase: store.EventPhaseStarted, DeployID: "dep",
	}
}

// A quiet window must not write the watermark forward on every tick — that is a
// Postgres write every poll interval, forever, on a platform where nothing is
// happening. It parks only once the cursor lags by parkInterval.
func TestQuietWindowParksTheWatermarkOnlyOncePastTheParkInterval(t *testing.T) {
	now := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
	until := now.Add(-dispatchLag)

	t.Run("inside the park interval writes nothing", func(t *testing.T) {
		w, st := countingWorker(now)
		st.wmSeeded, st.wmAt = true, until.Add(-parkInterval/2)
		if err := w.dispatch(context.Background(), st.endpoints); err != nil {
			t.Fatalf("dispatch: %v", err)
		}
		if len(st.writes) != 0 {
			t.Errorf("wrote the watermark %d times inside the park interval, want 0: %+v", len(st.writes), st.writes)
		}
	})

	t.Run("past the park interval writes once, with no deliveries", func(t *testing.T) {
		w, st := countingWorker(now)
		st.wmSeeded, st.wmAt = true, until.Add(-2*parkInterval)
		if err := w.dispatch(context.Background(), st.endpoints); err != nil {
			t.Fatalf("dispatch: %v", err)
		}
		if len(st.writes) != 1 {
			t.Fatalf("watermark writes = %d, want exactly 1: %+v", len(st.writes), st.writes)
		}
		if got := st.writes[0]; got.items != 0 || !got.at.Equal(until) || got.key != "" {
			t.Errorf("park write = %+v, want an empty batch parked at %v with no key", got, until)
		}
		if !st.wmAt.Equal(until) || st.wmKey != "" {
			t.Errorf("in-memory cursor = (%v,%q), want (%v,\"\") — a restart must not re-read the parked window", st.wmAt, st.wmKey, until)
		}
	})
}

// The deliveries and the cursor advance in ONE call. Splitting them would either
// drop events (advance first, crash) or redeliver them (insert first, crash) —
// neither of which surfaces as an error.
func TestBatchAndWatermarkAdvanceInOneStoreCall(t *testing.T) {
	now := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
	eventAt := now.Add(-time.Minute)
	w, st := countingWorker(now)
	st.wmSeeded, st.wmAt = true, eventAt.Add(-time.Hour)
	st.events = []store.WebhookEventRow{
		deployRow("dep-1:started", eventAt),
		deployRow("dep-2:started", eventAt.Add(time.Second)),
	}

	if err := w.dispatch(context.Background(), st.endpoints); err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if len(st.writes) != 1 {
		t.Fatalf("store calls = %d, want exactly 1 carrying both the batch and the cursor: %+v", len(st.writes), st.writes)
	}
	got := st.writes[0]
	if got.items != 2 {
		t.Errorf("enqueued %d deliveries, want 2", got.items)
	}
	if got.key != "dep-2:started" || !got.at.Equal(eventAt.Add(time.Second)) {
		t.Errorf("cursor advanced to (%v,%q), want the last row of the page", got.at, got.key)
	}
}

// A full page means there may be more behind it, so the pager reads again
// immediately instead of waiting a whole tick; a short page ends the pass. Get
// this wrong in the "always stop" direction and a backlog drains one page per
// tick forever; in the "always loop" direction the tick never returns.
func TestFullPageLoopsAndShortPageEndsThePass(t *testing.T) {
	now := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
	eventAt := now.Add(-time.Hour)

	t.Run("short page reads once", func(t *testing.T) {
		w, st := countingWorker(now)
		st.wmSeeded, st.wmAt = true, eventAt.Add(-time.Hour)
		st.events = []store.WebhookEventRow{deployRow("dep-1:started", eventAt)}
		if err := w.dispatch(context.Background(), st.endpoints); err != nil {
			t.Fatalf("dispatch: %v", err)
		}
		if st.reads != 1 {
			t.Errorf("reads = %d, want 1 for a short page", st.reads)
		}
	})

	t.Run("full page reads again", func(t *testing.T) {
		w, st := countingWorker(now)
		st.wmSeeded, st.wmAt = true, eventAt.Add(-time.Hour)
		for i := range dispatchBatch {
			st.events = append(st.events, deployRow("dep-"+string(rune('a'+i%26))+string(rune('a'+i/26))+":started", eventAt.Add(time.Duration(i)*time.Millisecond)))
		}
		if err := w.dispatch(context.Background(), st.endpoints); err != nil {
			t.Fatalf("dispatch: %v", err)
		}
		// One read fills the page, the second comes back empty and ends the pass.
		if st.reads != 2 {
			t.Errorf("reads = %d, want 2: a full page must be followed by another read", st.reads)
		}
	})
}

// The audit arm of the feed is filtered by verb IN THE QUERY (`e.verb = ANY(...)`),
// so the dispatcher has to push its vocabulary down as the pass's Verbs. An empty
// or missing Verbs matches no verb at all and drops the audit arm from the union
// entirely — every audit-sourced webhook type (plan changed, scaled, restarted,
// Postgres created, ...) would silently stop being delivered, while deploy- and
// fact-sourced ones kept working.
//
// Nothing caught that before: the only audit-source test called project()
// directly, so no test ever drove an audit row through the pager.
func TestAuditSourcedEventsReachTheirEndpoint(t *testing.T) {
	now := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
	eventAt := now.Add(-time.Minute)
	w, st := countingWorker(now)
	st.wmSeeded, st.wmAt = true, eventAt.Add(-time.Hour)
	st.endpoints = []store.WebhookEndpoint{
		endpoint("whk-1", "tea-a", "https://a.example/hook", "whsec_x", TypePlanChanged),
	}
	st.events = []store.WebhookEventRow{{
		Key: "aud-plan:", At: eventAt, TenantID: "tea-a",
		ServiceID: "acme-api", ServiceName: "api",
		Source: store.EventSourceAudit, Verb: core.AuditVerbSetPlan,
	}}

	if err := w.dispatch(context.Background(), st.endpoints); err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if len(st.queue) != 1 {
		t.Fatalf("audit-sourced event produced %d deliveries, want 1 — the verb filter must reach the query", len(st.queue))
	}
	for _, d := range st.queue {
		if d.EventType != TypePlanChanged {
			t.Errorf("delivery type = %q, want %q", d.EventType, TypePlanChanged)
		}
	}
}

// An endpoint never receives events from before it existed, however far back the
// durable cursor was when it was created. Two mechanisms enforce that, and this
// test is built to isolate the second: the read floor starts at the OLDEST
// enabled endpoint (so an endpoint-less stretch is skipped, not paged through),
// and a per-endpoint guard drops any row older than that endpoint.
//
// A single new endpoint would NOT exercise the guard — the floor alone would
// keep the old event out of the page, and the test would pass with the guard
// deleted. So there is an OLD endpoint too, which drags the floor back and puts
// the old event squarely in the page the new endpoint's guard must reject.
func TestNoEventIsDeliveredFromBeforeItsEndpointExisted(t *testing.T) {
	now := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
	old := now.Add(-24 * time.Hour)
	w, st := countingWorker(now)
	st.wmSeeded, st.wmAt = true, old.Add(-time.Hour)

	oldEndpoint := endpoint("whk-old", "tea-a", "https://a.example/old", "whsec_x", TypeDeployStarted)
	oldEndpoint.CreatedAt = old.Add(-time.Hour)
	newEndpoint := endpoint("whk-new", "tea-a", "https://a.example/new", "whsec_y", TypeDeployStarted)
	newEndpoint.CreatedAt = now.Add(-time.Hour)
	st.endpoints = []store.WebhookEndpoint{oldEndpoint, newEndpoint}
	st.events = []store.WebhookEventRow{
		deployRow("dep-old:started", old),
		deployRow("dep-new:started", now.Add(-30*time.Minute)),
	}

	if err := w.dispatch(context.Background(), st.endpoints); err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	got := map[string][]string{}
	for _, d := range st.queue {
		got[d.EndpointID] = append(got[d.EndpointID], d.EventID)
	}
	if len(got["whk-old"]) != 2 {
		t.Errorf("the pre-existing endpoint got %d deliveries, want both events", len(got["whk-old"]))
	}
	if len(got["whk-new"]) != 1 {
		t.Errorf("the endpoint created an hour ago got %d deliveries, want only the event that postdates it", len(got["whk-new"]))
	}
}
