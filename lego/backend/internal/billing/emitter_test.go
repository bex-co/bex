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

package billing

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"log"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

// --- fakes -------------------------------------------------------------------

type selCall struct {
	floor, sealBefore    time.Time
	limit                int
	requirePaymentMethod bool
}

type fakeEmitterStore struct {
	queue           [][]store.HourlyRow // successive SelectUnemittedUsage returns
	selCalls        []selCall
	stamped         [][]store.HourlyRow
	attempts        [][]store.UsageExportAttempt
	rejected        []store.UsageExportReject
	stampErr        error
	quarantineCount int64
	quarantineErr   error
}

func (f *fakeEmitterStore) SelectUnemittedUsage(_ context.Context, floor, sealBefore time.Time, limit int, requirePaymentMethod bool) ([]store.HourlyRow, error) {
	f.selCalls = append(f.selCalls, selCall{floor, sealBefore, limit, requirePaymentMethod})
	if len(f.queue) == 0 {
		return nil, nil
	}
	rows := f.queue[0]
	f.queue = f.queue[1:]
	return rows, nil
}

func (f *fakeEmitterStore) MarkUsageAttempted(_ context.Context, attempts []store.UsageExportAttempt, _ time.Time) error {
	f.attempts = append(f.attempts, attempts)
	return nil
}

func (f *fakeEmitterStore) RecordUsageExportResult(_ context.Context, accepted []store.UsageExportAttempt, rejected []store.UsageExportReject, _ time.Time) error {
	if f.stampErr != nil {
		return f.stampErr
	}
	rows := make([]store.HourlyRow, len(accepted))
	for i, attempt := range accepted {
		rows[i] = attempt.Row
	}
	f.stamped = append(f.stamped, rows)
	f.rejected = append(f.rejected, rejected...)
	return nil
}

func (f *fakeEmitterStore) QuarantineOldUsageAttempts(_ context.Context, _, _ time.Time) (int64, error) {
	return f.quarantineCount, f.quarantineErr
}

func (f *fakeEmitterStore) BillingExportStats(_ context.Context, _, _, _ time.Time, _ bool) (store.BillingExportStats, error) {
	return store.BillingExportStats{}, nil
}

func (f *fakeEmitterStore) stampedCount() int {
	n := 0
	for _, b := range f.stamped {
		n += len(b)
	}
	return n
}

type fakeIngester struct {
	ensured    []string
	contracted []string
	ensureErr  map[string]error
	batches    [][]Event
	ingestErr  error
	result     *IngestResult
}

func (f *fakeIngester) EnsureCustomer(_ context.Context, id string) error {
	f.ensured = append(f.ensured, id)
	return f.ensureErr[id]
}

func (f *fakeIngester) EnsureContract(_ context.Context, id string) error {
	f.contracted = append(f.contracted, id)
	return nil
}

func (f *fakeIngester) IngestBatch(_ context.Context, events []Event) IngestResult {
	f.batches = append(f.batches, events)
	if f.result != nil {
		return *f.result
	}
	if f.ingestErr != nil {
		failed := make([]IngestFailure, len(events))
		for i, event := range events {
			failed[i] = IngestFailure{TransactionID: event.TransactionID, Code: "transient", Message: f.ingestErr.Error()}
		}
		return IngestResult{Failed: failed}
	}
	accepted := make([]string, len(events))
	for i, event := range events {
		accepted[i] = event.TransactionID
	}
	return IngestResult{Accepted: accepted}
}

func TestEmitOnceAccountsAcceptedAndPermanentRejectSeparately(t *testing.T) {
	now := time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC)
	w := now.Add(-72 * time.Hour)
	acceptedRow := row("tea-a", "srv-1", "instance_seconds", "starter", "service", 3600, w)
	rejectedRow := row("tea-a", "srv-1", "build_seconds", "", "service", 30, w)
	acceptedID := toEvent(acceptedRow).TransactionID
	rejectedID := toEvent(rejectedRow).TransactionID
	st := &fakeEmitterStore{queue: [][]store.HourlyRow{{acceptedRow, rejectedRow}}}
	ing := &fakeIngester{result: &IngestResult{
		Accepted: []string{acceptedID},
		Failed:   []IngestFailure{{TransactionID: rejectedID, Code: "invalid_value", Message: "bad value", Permanent: true}},
	}}
	newEmitter(st, ing, now).emitOnce(context.Background())

	if st.stampedCount() != 1 || len(st.rejected) != 1 {
		t.Fatalf("accepted=%d rejected=%d, want 1/1", st.stampedCount(), len(st.rejected))
	}
	if st.rejected[0].Attempt.TransactionID != rejectedID || st.rejected[0].Code != "invalid_value" {
		t.Fatalf("rejected result = %+v", st.rejected[0])
	}
}

func (f *fakeIngester) allEvents() []Event {
	var out []Event
	for _, b := range f.batches {
		out = append(out, b...)
	}
	return out
}

func row(ws, svc, kind, tier, rk string, qty int64, window time.Time) store.HourlyRow {
	return store.HourlyRow{
		WorkspaceID: ws, ServiceID: svc, Kind: kind, Tier: tier,
		ResourceKind: rk, Quantity: qty, WindowStart: window,
	}
}

func newEmitter(st EmitterStore, ing Ingester, now time.Time) *Emitter {
	e := NewEmitter(st, ing)
	e.now = func() time.Time { return now }
	return e
}

// --- tests -------------------------------------------------------------------

func TestTransactionIDDeterministicAndMatchesSpec(t *testing.T) {
	w := time.Date(2026, 7, 19, 1, 0, 0, 0, time.UTC)
	got := transactionID("service", "srv-xyz", "instance_seconds", "starter", w)
	if got != transactionID("service", "srv-xyz", "instance_seconds", "starter", w) {
		t.Fatal("transactionID is not deterministic")
	}
	// The id is sha256 over the exact "|"-joined string in ADR040 §3.
	want := sha256.Sum256([]byte("service|srv-xyz|instance_seconds|starter|2026-07-19T01:00:00Z"))
	if got != hex.EncodeToString(want[:]) {
		t.Fatalf("transactionID = %s, want %s", got, hex.EncodeToString(want[:]))
	}
	// A different dimension yields a different id.
	if got == transactionID("service", "srv-xyz", "egress_bytes", "starter", w) {
		t.Fatal("transactionID collides across event kinds")
	}
}

func TestToEventMapping(t *testing.T) {
	w := time.Date(2026, 7, 19, 1, 0, 0, 0, time.UTC)
	e := toEvent(row("tea-abc", "srv-xyz", "instance_seconds", "starter", "service", 3600, w))
	if e.CustomerID != "tea-abc" {
		t.Errorf("CustomerID = %q, want tea-abc", e.CustomerID)
	}
	if e.EventType != "instance_seconds" {
		t.Errorf("EventType = %q, want instance_seconds", e.EventType)
	}
	if !e.Timestamp.Equal(w) {
		t.Errorf("Timestamp = %s, want %s", e.Timestamp, w)
	}
	want := map[string]string{"tier": "starter", "resource_kind": "service", "service_id": "srv-xyz", "value": "3600"}
	for k, v := range want {
		if e.Properties[k] != v {
			t.Errorf("Properties[%q] = %q, want %q", k, e.Properties[k], v)
		}
	}
}

func TestEmitOnceHappyPathStampsAfterIngest(t *testing.T) {
	now := time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC)
	w := now.Add(-72 * time.Hour) // sealed (older than 48h) and above the 34-day floor
	st := &fakeEmitterStore{queue: [][]store.HourlyRow{{
		row("tea-a", "srv-1", "instance_seconds", "starter", "service", 3600, w),
		row("tea-a", "srv-1", "egress_bytes", "", "service", 2048, w),
	}}}
	ing := &fakeIngester{}
	newEmitter(st, ing, now).emitOnce(context.Background())

	if got := len(ing.allEvents()); got != 2 {
		t.Fatalf("ingested events = %d, want 2", got)
	}
	if got := st.stampedCount(); got != 2 {
		t.Fatalf("stamped rows = %d, want 2 (stamp after successful ingest)", got)
	}
	if len(ing.ensured) != 1 || ing.ensured[0] != "tea-a" {
		t.Fatalf("ensured = %v, want exactly [tea-a]", ing.ensured)
	}
}

func TestEmitOnceFloorAndSealHorizon(t *testing.T) {
	now := time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC)
	st := &fakeEmitterStore{}
	ing := &fakeIngester{}
	e := newEmitter(st, ing, now) // Epoch unset ⇒ floor = now − 34d
	e.emitOnce(context.Background())

	if len(st.selCalls) != 1 {
		t.Fatalf("SelectUnemittedUsage calls = %d, want 1", len(st.selCalls))
	}
	c := st.selCalls[0]
	if wantFloor := now.Add(-BackfillHorizon); !c.floor.Equal(wantFloor) {
		t.Errorf("floor = %s, want %s (now − 34d, epoch unset)", c.floor, wantFloor)
	}
	if wantSeal := now.Add(-DefaultSealHours); !c.sealBefore.Equal(wantSeal) {
		t.Errorf("sealBefore = %s, want %s (now − 48h)", c.sealBefore, wantSeal)
	}
	if c.requirePaymentMethod {
		t.Error("default emitter unexpectedly enabled payment-method withholding")
	}
}

func TestEmitOnceThreadsPaymentMethodGateToDurableOutboxRead(t *testing.T) {
	now := time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC)
	st := &fakeEmitterStore{}
	e := newEmitter(st, &fakeIngester{}, now)
	e.RequirePaymentMethod = true
	e.emitOnce(context.Background())
	if len(st.selCalls) != 1 || !st.selCalls[0].requirePaymentMethod {
		t.Fatalf("SelectUnemittedUsage calls = %+v, want gated read", st.selCalls)
	}
}

func TestEmitOnceWithheldWorkspaceMakesNoStripeCallsThenShipsAfterBinding(t *testing.T) {
	now := time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC)
	sealed := row("tea-cardless", "srv-1", "instance_seconds", "starter", "service", 3600, now.Add(-72*time.Hour))
	// The store's real-Postgres test proves the first empty selection is the
	// cardless marker filter and the second selection follows marker binding.
	// Here the emitter boundary proves that an empty gated selection cannot
	// reach any Customer, Subscription, or meter-event provider call.
	st := &fakeEmitterStore{queue: [][]store.HourlyRow{nil, {sealed}}}
	ing := &fakeIngester{}
	e := newEmitter(st, ing, now)
	e.RequirePaymentMethod = true

	e.emitOnce(context.Background())
	if len(ing.ensured) != 0 || len(ing.contracted) != 0 || len(ing.allEvents()) != 0 || st.stampedCount() != 0 {
		t.Fatalf("withheld pass reached Stripe: customers=%v contracts=%v events=%v stamped=%d", ing.ensured, ing.contracted, ing.allEvents(), st.stampedCount())
	}

	e.emitOnce(context.Background())
	if len(ing.ensured) != 1 || len(ing.contracted) != 1 || len(ing.allEvents()) != 1 || st.stampedCount() != 1 {
		t.Fatalf("post-bind pass did not ship once: customers=%v contracts=%v events=%v stamped=%d", ing.ensured, ing.contracted, ing.allEvents(), st.stampedCount())
	}
}

func TestEmitOnceEpochOverridesBackfillFloor(t *testing.T) {
	now := time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC)
	epoch := now.Add(-10 * 24 * time.Hour) // 10 days ago, more recent than the 34d backfill
	st := &fakeEmitterStore{}
	e := newEmitter(st, &fakeIngester{}, now)
	e.Epoch = epoch
	e.emitOnce(context.Background())

	if !st.selCalls[0].floor.Equal(epoch) {
		t.Fatalf("floor = %s, want the epoch %s (max(epoch, now−34d))", st.selCalls[0].floor, epoch)
	}
}

func TestEmitOnceSkipsWhenFloorAtOrAfterSealHorizon(t *testing.T) {
	now := time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC)
	st := &fakeEmitterStore{}
	e := newEmitter(st, &fakeIngester{}, now)
	e.Epoch = now // floor = now, which is after the seal horizon (now − 48h)
	e.emitOnce(context.Background())
	if len(st.selCalls) != 0 {
		t.Fatalf("SelectUnemittedUsage called %d times, want 0 (nothing sealed above the floor)", len(st.selCalls))
	}
}

func TestEmitOnceGapWarningLoggedOnce(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)

	now := time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC)
	e := newEmitter(&fakeEmitterStore{}, &fakeIngester{}, now)
	e.emitOnce(context.Background())
	e.emitOnce(context.Background())
	if n := strings.Count(buf.String(), "Stripe export active"); n != 1 {
		t.Fatalf("gap warning logged %d times, want exactly 1", n)
	}
}

func TestEmitOnceLeavesRowsUnstampedOnIngestFailure(t *testing.T) {
	now := time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC)
	w := now.Add(-72 * time.Hour)
	st := &fakeEmitterStore{queue: [][]store.HourlyRow{{
		row("tea-a", "srv-1", "instance_seconds", "starter", "service", 3600, w),
	}}}
	ing := &fakeIngester{ingestErr: errors.New("stripe down")}
	newEmitter(st, ing, now).emitOnce(context.Background())

	if st.stampedCount() != 0 {
		t.Fatalf("stamped %d rows, want 0 (ingest failed → nothing marked emitted)", st.stampedCount())
	}
}

func TestEmitOnceSkipsWorkspaceWhoseCustomerFails(t *testing.T) {
	now := time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC)
	w := now.Add(-72 * time.Hour)
	st := &fakeEmitterStore{queue: [][]store.HourlyRow{{
		row("tea-bad", "srv-1", "instance_seconds", "starter", "service", 1, w),
		row("tea-ok", "srv-2", "instance_seconds", "starter", "service", 2, w),
	}}}
	ing := &fakeIngester{ensureErr: map[string]error{"tea-bad": errors.New("provision failed")}}
	newEmitter(st, ing, now).emitOnce(context.Background())

	events := ing.allEvents()
	if len(events) != 1 || events[0].CustomerID != "tea-ok" {
		t.Fatalf("ingested events = %v, want only tea-ok's row", events)
	}
	if st.stampedCount() != 1 {
		t.Fatalf("stamped %d rows, want 1 (only the workspace whose customer succeeded)", st.stampedCount())
	}
}

func TestEmitOnceDrainsMultipleBatches(t *testing.T) {
	now := time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC)
	w := now.Add(-72 * time.Hour)
	full := []store.HourlyRow{
		row("tea-a", "srv-1", "instance_seconds", "starter", "service", 1, w),
		row("tea-a", "srv-2", "instance_seconds", "starter", "service", 2, w),
	}
	short := []store.HourlyRow{
		row("tea-a", "srv-3", "instance_seconds", "starter", "service", 3, w),
	}
	st := &fakeEmitterStore{queue: [][]store.HourlyRow{full, short}}
	e := newEmitter(st, &fakeIngester{}, now)
	e.BatchLimit = 2 // full batch == limit triggers another drain pass
	e.emitOnce(context.Background())

	if len(st.selCalls) != 2 {
		t.Fatalf("SelectUnemittedUsage calls = %d, want 2 (drain until a short batch)", len(st.selCalls))
	}
	if st.stampedCount() != 3 {
		t.Fatalf("stamped %d rows, want 3 (both batches)", st.stampedCount())
	}
}
