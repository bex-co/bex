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

package agentsession

import (
	"context"
	"math"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// openRouterModels mirrors the live /api/v1/models shape for the pinned
// model (2026-11-05): decimal-string USD-per-token prices plus weekday peak
// overrides, and one model whose evening window wraps midnight.
const openRouterModels = `{"data":[
 {"id":"deepseek/deepseek-v4.1-flash-1017","canonical_slug":"deepseek/deepseek-v4.1-flash-1017",
  "pricing":{"prompt":"0.00000015","completion":"0.0000006","input_cache_read":"0.000000003",
   "overrides":[
    {"utc_days":["monday","tuesday","wednesday","thursday","friday"],"utc_start":100,"utc_end":400,
     "prompt":"0.0000003","completion":"0.0000012","input_cache_read":"0.000000006"},
    {"utc_days":["monday","tuesday","wednesday","thursday","friday"],"utc_start":600,"utc_end":1000,
     "prompt":"0.0000003","completion":"0.0000012","input_cache_read":"0.000000006"}]}},
 {"id":"vendor/night-owl","pricing":{"prompt":"0.000001","completion":"0.000002","input_cache_write":"0.0000015",
   "overrides":[{"utc_days":["saturday"],"utc_start":2200,"utc_end":0,"prompt":"0.0000005"},
                {"utc_days":["sunday"],"utc_start":2300,"utc_end":200,"completion":"0.000001"}]}},
 {"id":"openrouter/auto","pricing":{"prompt":"-1","completion":"-1"}}
]}`

func priceServer(t *testing.T, body *atomic.Value, fail *atomic.Bool, hits *atomic.Int32) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		if fail.Load() {
			http.Error(w, "down", http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte(body.Load().(string)))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func near(a, b float64) bool { return math.Abs(a-b) < 1e-12 }

// refreshIfDue runs, synchronously, the refresh Cost would start in the
// background at the table's current time.
func refreshIfDue(table *OpenRouterPriceTable) {
	if table.due(table.snapshot.Load(), table.now()) {
		table.refresh(context.Background())
	}
}

// Cost never waits on the network: with no table yet it answers unpriced at
// once and fetches in the background, one fetch at a time.
func TestOpenRouterPriceTableCostNeverBlocks(t *testing.T) {
	release := make(chan struct{})
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		<-release
		_, _ = w.Write([]byte(openRouterModels))
	}))
	t.Cleanup(srv.Close)
	t.Cleanup(func() {
		select {
		case <-release:
		default:
			close(release)
		}
	})
	table := &OpenRouterPriceTable{URL: srv.URL, HTTP: srv.Client()}
	table.Start(context.Background())
	done := make(chan bool, 1)
	go func() {
		_, ok := table.Cost(DefaultDevModel, time.Now(), 1, 0, 0, 0)
		table.Cost(DefaultDevModel, time.Now(), 1, 0, 0, 0)
		done <- ok
	}()
	select {
	case ok := <-done:
		if ok {
			t.Fatal("Cost priced from a table that was never fetched")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Cost blocked on the in-flight fetch")
	}
	close(release)
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, ok := table.Cost(DefaultDevModel, time.Now(), 1, 0, 0, 0); ok {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the background fetch never landed")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if hits.Load() != 1 {
		t.Fatalf("fetches = %d, want 1 (one refresh at a time, then fresh)", hits.Load())
	}
}

func TestOpenRouterPriceTableAppliesOverrides(t *testing.T) {
	var body atomic.Value
	body.Store(openRouterModels)
	var fail atomic.Bool
	var hits atomic.Int32
	srv := priceServer(t, &body, &fail, &hits)
	now := time.Date(2026, 11, 3, 2, 30, 0, 0, time.UTC) // Tuesday 02:30 UTC
	table := &OpenRouterPriceTable{URL: srv.URL, HTTP: srv.Client(), Now: func() time.Time { return now }}
	refreshIfDue(table)
	const model = "deepseek/deepseek-v4.1-flash-1017"

	// 1M input + 1M output + 1M cache read + 1M cache write tokens.
	const m = 1_000_000
	base, ok := table.Cost(model, time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC), m, m, m, m)
	// Cache write is unlisted, so it is priced at prompt: 0.15+0.60+0.003+0.15.
	if !ok || !near(base, 0.903) {
		t.Fatalf("Tuesday 12:00 (off-peak) cost = %v ok=%v, want 0.903", base, ok)
	}
	peak, _ := table.Cost(model, now, m, m, m, m)
	if !near(peak, 2*base) {
		t.Fatalf("Tuesday 02:30 (peak) cost = %v, want doubled %v", peak, 2*base)
	}
	if end, _ := table.Cost(model, time.Date(2026, 11, 3, 4, 0, 0, 0, time.UTC), m, 0, 0, 0); !near(end, 0.15) {
		t.Fatalf("04:00 is past the exclusive window end: cost = %v, want 0.15", end)
	}
	if weekend, _ := table.Cost(model, time.Date(2026, 11, 7, 2, 30, 0, 0, time.UTC), m, 0, 0, 0); !near(weekend, 0.15) {
		t.Fatalf("Saturday 02:30 is not a weekday peak: cost = %v, want 0.15", weekend)
	}

	// utc_end 0 runs the window to midnight; start > end wraps past it.
	sat := time.Date(2026, 11, 7, 23, 15, 0, 0, time.UTC)
	if c, _ := table.Cost("vendor/night-owl", sat, m, m, 0, m); !near(c, 0.5+2+1.5) {
		t.Fatalf("Saturday 23:15 cost = %v, want override prompt 0.5 + base completion 2 + listed cache write 1.5", c)
	}
	if c, _ := table.Cost("vendor/night-owl", sat.Add(-2*time.Hour), m, 0, 0, 0); !near(c, 1) {
		t.Fatalf("Saturday 21:15 is before the window: cost = %v, want base 1", c)
	}
	if c, _ := table.Cost("vendor/night-owl", time.Date(2026, 11, 8, 1, 0, 0, 0, time.UTC), 0, m, 0, 0); !near(c, 1) {
		t.Fatalf("Sunday 01:00 (wrapped window) completion = %v, want 1", c)
	}
	if c, _ := table.Cost("vendor/night-owl", time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC), 0, 0, 0, m); !near(c, 1.5) {
		t.Fatalf("listed input_cache_write = %v, want 1.5", c)
	}

	if _, ok := table.Cost("openrouter/auto", now, 1, 1, 0, 0); ok {
		t.Fatal("a variable-price router must be unpriced")
	}
	if _, ok := table.Cost("vendor/unknown", now, 1, 1, 0, 0); ok {
		t.Fatal("an unlisted model must be unpriced")
	}
	if hits.Load() != 1 {
		t.Fatalf("fetches = %d, want 1 within the refresh hour", hits.Load())
	}
}

func TestOpenRouterPriceTableKeepsLastGoodCopy(t *testing.T) {
	var body atomic.Value
	body.Store(openRouterModels)
	var fail atomic.Bool
	var hits atomic.Int32
	srv := priceServer(t, &body, &fail, &hits)
	now := time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)
	table := &OpenRouterPriceTable{URL: srv.URL, HTTP: srv.Client(), Now: func() time.Time { return now }}
	refreshIfDue(table)
	const model = "deepseek/deepseek-v4.1-flash-1017"
	if c, ok := table.Cost(model, now, 1_000_000, 0, 0, 0); !ok || !near(c, 0.15) {
		t.Fatalf("first cost = %v ok=%v", c, ok)
	}

	// An hour later the refresh fails: the last good copy keeps pricing.
	fail.Store(true)
	now = now.Add(61 * time.Minute)
	refreshIfDue(table)
	if c, ok := table.Cost(model, now, 1_000_000, 0, 0, 0); !ok || !near(c, 0.15) {
		t.Fatalf("cost after a failed refresh = %v ok=%v, want the last good 0.15", c, ok)
	}
	// A garbage body is a failed refresh too, not an empty table.
	fail.Store(false)
	body.Store(`{"data":[]}`)
	now = now.Add(2 * time.Minute)
	refreshIfDue(table)
	if _, ok := table.Cost(model, now, 1, 0, 0, 0); !ok {
		t.Fatal("an empty model list must not replace the last good copy")
	}
	// Retries are spaced: two more calls within a minute fetch nothing.
	before := hits.Load()
	refreshIfDue(table)
	now = now.Add(30 * time.Second)
	refreshIfDue(table)
	if hits.Load() != before {
		t.Fatalf("fetches during the retry spacing = %d, want 0", hits.Load()-before)
	}
	// A good refresh replaces the table.
	body.Store(`{"data":[{"id":"` + model + `","pricing":{"prompt":"0.000001","completion":"0.000001"}}]}`)
	now = now.Add(2 * time.Minute)
	refreshIfDue(table)
	if c, _ := table.Cost(model, now, 1_000_000, 0, 0, 0); !near(c, 1) {
		t.Fatalf("cost after a good refresh = %v, want 1", c)
	}
}

// The recorder prices each exchange (ADR091 D1/D2): reported cost first,
// else the table at the request's start, else unpriced at zero.
func TestModelUsageRecorderPricesByCostSource(t *testing.T) {
	ledger := &usageLedger{session: store.AgentSession{ID: "ags-one", WorkspaceID: "tea-a"}, recorded: map[string]store.AgentModelUsage{}}
	at := time.Date(2026, 11, 3, 2, 35, 0, 0, time.UTC) // Tuesday, inside the 01:00–04:00 peak
	var body atomic.Value
	body.Store(openRouterModels)
	var fail atomic.Bool
	var hits atomic.Int32
	srv := priceServer(t, &body, &fail, &hits)
	table := &OpenRouterPriceTable{URL: srv.URL, HTTP: srv.Client(), Now: func() time.Time { return at }}
	refreshIfDue(table)
	rec := &ModelUsageRecorder{Store: ledger, Now: func() time.Time { return at }, Prices: table, Markup: 1.3}
	record := func(rep ModelUsageReport) store.AgentModelUsage {
		t.Helper()
		rep.RequestID, rep.SessionID, rep.Workspace = id.New(id.AgentModelRequest), "ags-one", "tea-a"
		if _, err := rec.Record(context.Background(), rep); err != nil {
			t.Fatalf("record %+v: %v", rep, err)
		}
		return ledger.recorded[rep.RequestID]
	}

	cost := 0.001
	reported := record(ModelUsageReport{Model: DefaultDevModel, InputTokens: 10, UpstreamCostUSD: &cost})
	if reported.CostSource != store.AgentModelCostReported || reported.UpstreamCostMicroUSD != 1000 || reported.PriceMicroUSD != 1300 {
		t.Fatalf("reported = %+v, want reported 1000 → 1300", reported)
	}

	// No reported cost: 1M input at the peak $0.30/M = $0.30 → × 1.3.
	computed := record(ModelUsageReport{Model: DefaultDevModel, InputTokens: 1_000_000, StartedAt: at.Add(-time.Minute)})
	if computed.CostSource != store.AgentModelCostComputed || computed.UpstreamCostMicroUSD != 300_000 || computed.PriceMicroUSD != 390_000 {
		t.Fatalf("computed = %+v, want computed 300000 → 390000 (peak override)", computed)
	}
	// The start time picks the price: 01:00 tokens started at 00:59 are off-peak.
	offPeak := record(ModelUsageReport{Model: DefaultDevModel, InputTokens: 1_000_000, StartedAt: time.Date(2026, 11, 3, 0, 59, 0, 0, time.UTC)})
	if offPeak.UpstreamCostMicroUSD != 150_000 {
		t.Fatalf("off-peak start = %+v, want 150000", offPeak)
	}
	// The window is always the record time, never the start time.
	if !offPeak.At.Equal(at) {
		t.Fatalf("recorded At = %s, want the record time %s", offPeak.At, at)
	}

	unpriced := record(ModelUsageReport{Model: "vendor/unknown", InputTokens: 5})
	if unpriced.CostSource != store.AgentModelCostUnpriced || unpriced.PriceMicroUSD != 0 || unpriced.UpstreamCostMicroUSD != 0 {
		t.Fatalf("unpriced = %+v", unpriced)
	}

	// Half-up rounding per request: $0.0000005 → 1 µUSD; × 1.3 = 0.65 → 1.
	half := 0.0000005
	if r := record(ModelUsageReport{Model: DefaultDevModel, InputTokens: 1, UpstreamCostUSD: &half}); r.UpstreamCostMicroUSD != 1 || r.PriceMicroUSD != 1 {
		t.Fatalf("half-up = %+v", r)
	}

	// An out-of-range reported cost is treated as missing: the exchange is
	// priced from the table and its tokens are still recorded.
	for name, bad := range map[string]float64{"negative": -0.01, "too large": 100.01, "NaN": math.NaN(), "Inf": math.Inf(1)} {
		c := bad
		r := record(ModelUsageReport{Model: DefaultDevModel, InputTokens: 1_000_000, StartedAt: at.Add(-time.Minute), UpstreamCostUSD: &c})
		if r.CostSource != store.AgentModelCostComputed || r.UpstreamCostMicroUSD != 300_000 || r.InputTokens != 1_000_000 {
			t.Errorf("%s cost: recorded %+v, want the computed 300000 and the tokens kept", name, r)
		}
	}
}
