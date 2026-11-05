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
	"context"
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

const agentModelTestModel = "deepseek/deepseek-v4.1-flash-1017"

func agentModelRow(kind string, quantity int64) store.HourlyRow {
	return store.HourlyRow{
		WorkspaceID: "tea-a", ServiceID: "ags-x", Kind: kind,
		Tier: agentModelTestModel, ResourceKind: store.ResourceKindAgentSession,
		WindowStart: time.Date(2026, 10, 27, 9, 0, 0, 0, time.UTC), Quantity: quantity,
	}
}

// ADR091 D3: token rows are the display breakdown only — accounted locally at
// zero, never a Stripe meter event, never a reject.
func TestAgentModelTokensAreNeverExported(t *testing.T) {
	for _, kind := range store.AgentModelUsageKinds {
		if _, _, skip := stripeMeterEvent(toEvent(agentModelRow(kind, 4321))); !skip {
			t.Fatalf("%s must map to the local zero-rate path", kind)
		}
	}
	c, stub := newStripeTest(t, func(_ string, _ string) (int, string) {
		return 200, `{"id":"cus_1","object":"customer"}`
	})
	c.storeCustomer("tea-a", "cus_1")
	event := toEvent(agentModelRow(store.UsageKindAgentModelOutputTokens, 4321))
	result := c.IngestBatch(context.Background(), []Event{event})
	if len(result.Accepted) != 1 || result.Accepted[0] != event.TransactionID || len(result.Failed) != 0 {
		t.Fatalf("IngestBatch = %+v, want the row accepted locally", result)
	}
	if got := stub.count("/billing/meter_events"); got != 0 {
		t.Fatalf("meter events posted = %d, want 0 for a token row", got)
	}
}

// The cost row ships as one event on the single model-agnostic meter, its
// value the price in micro-USD.
func TestAgentModelCostExportsToTheSingleMeter(t *testing.T) {
	row := agentModelRow(store.UsageKindAgentModelCostMicroUSD, 1300)
	name, value, skip := stripeMeterEvent(toEvent(row))
	if skip || name != "agent_model_cost_microusd" || value != "1300" {
		t.Fatalf("stripeMeterEvent = (%q,%q,%v), want agent_model_cost_microusd 1300", name, value, skip)
	}
	other := row
	other.Tier = "openai/gpt-x"
	if name, _, skip := stripeMeterEvent(toEvent(other)); skip || name != "agent_model_cost_microusd" {
		t.Fatalf("the cost meter must be model-agnostic, got (%q,%v)", name, skip)
	}

	c, stub := newStripeTest(t, func(_ string, _ string) (int, string) {
		return 200, `{"id":"cus_1","object":"customer"}`
	})
	c.storeCustomer("tea-a", "cus_1")
	result := c.IngestBatch(context.Background(), []Event{toEvent(row)})
	if len(result.Accepted) != 1 || len(result.Failed) != 0 {
		t.Fatalf("IngestBatch = %+v, want one accepted event", result)
	}
	if got := stub.count("/billing/meter_events"); got != 1 {
		t.Fatalf("meter events posted = %d, want 1", got)
	}
	stub.mu.Lock()
	joined := strings.Join(stub.bodies, "\n")
	stub.mu.Unlock()
	decoded, _ := url.QueryUnescape(joined)
	for _, want := range []string{"event_name=agent_model_cost_microusd", "payload[value]=1300"} {
		if !strings.Contains(decoded, want) {
			t.Fatalf("meter-event body missing %q: %s", want, joined)
		}
	}
}

// countingEmitterStore counts the full-pass-only store calls.
type countingEmitterStore struct {
	fakeEmitterStore
	quarantineCalls, statsCalls int
}

func (c *countingEmitterStore) QuarantineOldUsageAttempts(ctx context.Context, before, at time.Time) (int64, error) {
	c.quarantineCalls++
	return c.fakeEmitterStore.QuarantineOldUsageAttempts(ctx, before, at)
}

func (c *countingEmitterStore) BillingExportStats(ctx context.Context, floor time.Time, seal store.UsageSeal, now time.Time, requirePaymentMethod bool) (store.BillingExportStats, error) {
	c.statsCalls++
	return c.fakeEmitterStore.BillingExportStats(ctx, floor, seal, now, requirePaymentMethod)
}

// A held kind (its Stripe Price not provisioned) rides on the full pass's
// seal, so the store keeps it pending rather than the mapper dropping it as
// zero. A held fast kind is not fast: with every fast kind held the fast pass
// touches nothing.
func TestEmitterHeldFastKindsGetNoFastPass(t *testing.T) {
	now := time.Date(2026, 10, 27, 10, 6, 0, 0, time.UTC)
	st := &countingEmitterStore{}
	e := newEmitter(st, &fakeIngester{}, now)
	e.Metrics = NewMetrics(prometheus.NewRegistry())
	e.FastKinds = []string{store.UsageKindAgentModelCostMicroUSD}
	e.HoldKinds = []string{store.UsageKindAgentModelCostMicroUSD}
	e.emitOnce(context.Background(), true)
	if len(st.selCalls) != 0 || st.quarantineCalls != 0 || st.statsCalls != 0 {
		t.Fatalf("fast pass with every fast kind held made store calls: select=%d quarantine=%d stats=%d",
			len(st.selCalls), st.quarantineCalls, st.statsCalls)
	}
	e.emitOnce(context.Background(), false)
	if len(st.selCalls) != 1 {
		t.Fatalf("SelectUnemittedUsage calls = %d, want 1", len(st.selCalls))
	}
	seal := st.selCalls[0].seal
	if !slices.Equal(seal.HoldKinds, e.HoldKinds) || len(seal.FastKinds) != 0 || !seal.FastBefore.IsZero() {
		t.Fatalf("full pass seal = %+v, want the held kind held and no fast kinds", seal)
	}
}

// The fast pass leaves quarantine and the metrics snapshot to the full pass.
func TestFastPassSkipsQuarantineAndStats(t *testing.T) {
	now := time.Date(2026, 10, 27, 10, 6, 0, 0, time.UTC)
	st := &countingEmitterStore{}
	e := newEmitter(st, &fakeIngester{}, now)
	e.Metrics = NewMetrics(prometheus.NewRegistry())
	e.FastKinds = []string{store.UsageKindAgentModelCostMicroUSD}
	e.emitOnce(context.Background(), true)
	if len(st.selCalls) != 1 || st.quarantineCalls != 0 || st.statsCalls != 0 {
		t.Fatalf("fast pass: select=%d quarantine=%d stats=%d, want 1/0/0", len(st.selCalls), st.quarantineCalls, st.statsCalls)
	}
	e.emitOnce(context.Background(), false)
	if st.quarantineCalls != 1 || st.statsCalls != 1 {
		t.Fatalf("full pass: quarantine=%d stats=%d, want 1/1", st.quarantineCalls, st.statsCalls)
	}
}

// ADR091 D3 (amended): the cost meter skips the 48h seal. The fast pass
// selects only the fast kinds, sealed 65 minutes after their window starts,
// and ships a row a few minutes after its hour closes; a resend of the same
// row reuses the same deterministic identifier.
func TestFastPassExportsCostRowsWithoutTheSeal(t *testing.T) {
	now := time.Date(2026, 10, 27, 10, 6, 0, 0, time.UTC)
	row := agentModelRow(store.UsageKindAgentModelCostMicroUSD, 1300) // 09:00 window, closed 6 min ago
	st := &fakeEmitterStore{queue: [][]store.HourlyRow{{row}}}
	ing := &fakeIngester{}
	e := newEmitter(st, ing, now)
	e.FastKinds = []string{store.UsageKindAgentModelCostMicroUSD}
	e.emitOnce(context.Background(), true)

	if len(st.selCalls) != 1 {
		t.Fatalf("SelectUnemittedUsage calls = %d, want 1", len(st.selCalls))
	}
	seal := st.selCalls[0].seal
	if !seal.Before.IsZero() {
		t.Fatalf("fast pass seal.Before = %s, want zero (no slow-kind row qualifies)", seal.Before)
	}
	if !seal.FastBefore.Equal(now.Add(-65*time.Minute)) || !slices.Equal(seal.FastKinds, e.FastKinds) {
		t.Fatalf("fast pass seal = %+v, want fast kinds sealed before now−65m", seal)
	}
	if !seal.SealsAbove(e.floor(now)) || !seal.FastBefore.After(row.WindowStart) {
		t.Fatalf("a closed hour's cost row must be sealed on the fast pass: %+v", seal)
	}
	if st.stampedCount() != 1 || len(ing.batches) != 1 || ing.batches[0][0].EventType != store.UsageKindAgentModelCostMicroUSD {
		t.Fatalf("fast pass stamped=%d batches=%+v, want the cost row shipped", st.stampedCount(), ing.batches)
	}
	if st.attempts[0][0].EventName != "agent_model_cost_microusd" {
		t.Fatalf("attempt event name = %q", st.attempts[0][0].EventName)
	}

	// A resend (crash before the local stamp) carries the same identifier,
	// so Stripe's identifier dedupe drops the duplicate.
	st.queue = [][]store.HourlyRow{{row}}
	e.emitOnce(context.Background(), true)
	if len(ing.batches) != 2 || ing.batches[1][0].TransactionID != ing.batches[0][0].TransactionID {
		t.Fatalf("resend identifiers = %+v, want the first event's identifier reused", ing.batches)
	}

	// The full pass also carries the fast predicate, so a missed fast tick
	// is caught up hourly.
	st.selCalls = nil
	e.emitOnce(context.Background(), false)
	if full := st.selCalls[0].seal; !full.Before.Equal(now.Add(-48*time.Hour)) || !full.FastBefore.Equal(seal.FastBefore) {
		t.Fatalf("full pass seal = %+v, want the 48h horizon plus the fast horizon", full)
	}
}

// The early return honours the fast horizon: a floor newer than the 48h seal
// still exports fast kinds, and one newer than the fast seal exports nothing.
func TestFastPassFloorGuard(t *testing.T) {
	now := time.Date(2026, 10, 27, 10, 6, 0, 0, time.UTC)
	for name, tc := range map[string]struct {
		epoch      time.Time
		wantSelect bool
	}{
		"floor inside the 48h seal":  {now.Add(-3 * time.Hour), true},
		"floor inside the fast seal": {now.Add(-30 * time.Minute), false},
	} {
		st := &fakeEmitterStore{}
		e := newEmitter(st, &fakeIngester{}, now)
		e.Epoch = tc.epoch
		e.FastKinds = []string{store.UsageKindAgentModelCostMicroUSD}
		e.emitOnce(context.Background(), true)
		if got := len(st.selCalls) > 0; got != tc.wantSelect {
			t.Errorf("%s: selected = %v, want %v", name, got, tc.wantSelect)
		}
	}
	// Without fast kinds the fast pass never selects.
	st := &fakeEmitterStore{}
	newEmitter(st, &fakeIngester{}, now).emitOnce(context.Background(), true)
	if len(st.selCalls) != 0 {
		t.Fatalf("fast pass without FastKinds selected %d times", len(st.selCalls))
	}
}
