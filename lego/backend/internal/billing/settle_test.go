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
	"errors"
	"fmt"
	"math"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/pricing"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// settleMeterClient is a client whose catalog is already resolved, with the
// egress meter's summary answering summary GiB.
func settleMeterClient(t *testing.T, summary *float64) (*StripeClient, *stripeStub) {
	t.Helper()
	c, stub := newStripeTest(t, func(method, path string) (int, string) {
		if method == http.MethodGet && path == "/v1/billing/meters/mtr_egress/event_summaries" {
			return 200, fmt.Sprintf(`{"object":"list","data":[{"id":"mtrusg_1","object":"billing.meter_event_summary","aggregated_value":%g}],"has_more":false,"url":"/x"}`, *summary)
		}
		return 500, `{"error":{"type":"api_error","message":"unexpected route"}}`
	})
	c.priceIDs = []string{"price_egress"}
	c.priceByKey = map[string]string{"egress_gib": "price_egress"}
	c.priceMeters = map[string]string{"price_egress": "mtr_egress"}
	return c, stub
}

var settleContractFixture = SettleContract{WorkspaceID: "tea-a", CustomerID: "cus_1", SubscriptionID: "sub_1",
	From: time.Unix(1_700_000_040, 0), To: time.Unix(1_700_003_640, 0)}

// The settle waits on Stripe's summary in meter units, converted exactly as
// the export converts them: 3 GiB of egress bytes is 3 on the egress_gib
// meter. Caught up returns at once; zero-rated kinds are not awaited.
func TestAwaitMeteredUsageConvertsToMeterUnits(t *testing.T) {
	summary := 3.0
	c, stub := settleMeterClient(t, &summary)
	totals := []store.ExportedUsageTotal{
		{Kind: store.UsageKindEgressBytes, Quantity: 3 * pricing.BytesPerGiB, Recent: 3 * pricing.BytesPerGiB},
		{Kind: store.UsageKindInstanceSeconds, Tier: "free", ResourceKind: store.ResourceKindService, Quantity: 3600},
	}
	if err := c.AwaitMeteredUsage(context.Background(), settleContractFixture, totals); err != nil {
		t.Fatal(err)
	}
	if n := stub.count("/v1/billing/meters/mtr_egress/event_summaries"); n != 1 {
		t.Fatalf("summary reads = %d, want 1", n)
	}
}

// Lagging summaries refuse with the retryable BILLING_SETTLING, within the
// caller's own deadline rather than the full wait.
func TestAwaitMeteredUsageRefusesWithinCallerDeadline(t *testing.T) {
	summary := 1.0
	c, _ := settleMeterClient(t, &summary)
	c.settleWait = time.Minute
	ctx, cancel := context.WithTimeout(context.Background(), settleChargeReserve+time.Second)
	defer cancel()
	start := time.Now()
	err := c.AwaitMeteredUsage(ctx, settleContractFixture, []store.ExportedUsageTotal{
		{Kind: store.UsageKindEgressBytes, Quantity: 3 * pricing.BytesPerGiB, Recent: 3 * pricing.BytesPerGiB},
	})
	var coded *core.CodedError
	if !errors.As(err, &coded) || coded.Code != core.CodeBillingSettling {
		t.Fatalf("lagging summary = %v, want BILLING_SETTLING", err)
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("waited %s past the caller's deadline budget", elapsed)
	}
}

// A shortfall larger than the last hour's exports cannot be aggregation lag
// (the event was dropped, or billed to an earlier Customer): it does not
// block the deletion.
func TestAwaitMeteredUsageIgnoresOldShortfall(t *testing.T) {
	summary := 1.0
	c, _ := settleMeterClient(t, &summary)
	c.settleWait = time.Minute
	err := c.AwaitMeteredUsage(context.Background(), settleContractFixture, []store.ExportedUsageTotal{
		{Kind: store.UsageKindEgressBytes, Quantity: 3 * pricing.BytesPerGiB, Recent: pricing.BytesPerGiB},
	})
	if err != nil {
		t.Fatalf("an old shortfall blocked the settle: %v", err)
	}
}

// A caller bound by no request (core.WithSettleWait) outwaits Stripe's
// aggregation lag that would refuse an API request's settle.
func TestAwaitMeteredUsageHonorsALongerCallerWait(t *testing.T) {
	// The summary catches up 200ms after caughtUpAt is armed.
	var caughtUpAt atomic.Int64
	caughtUpAt.Store(time.Now().Add(time.Hour).UnixNano())
	c, _ := newStripeTest(t, func(method, path string) (int, string) {
		value := 1.0
		if time.Now().UnixNano() >= caughtUpAt.Load() {
			value = 3
		}
		return 200, fmt.Sprintf(`{"object":"list","data":[{"id":"mtrusg_1","object":"billing.meter_event_summary","aggregated_value":%g}],"has_more":false,"url":"/x"}`, value)
	})
	c.priceIDs = []string{"price_egress"}
	c.priceByKey = map[string]string{"egress_gib": "price_egress"}
	c.priceMeters = map[string]string{"price_egress": "mtr_egress"}
	c.settleWait = 10 * time.Millisecond
	totals := []store.ExportedUsageTotal{
		{Kind: store.UsageKindEgressBytes, Quantity: 3 * pricing.BytesPerGiB, Recent: 3 * pricing.BytesPerGiB},
	}
	var coded *core.CodedError
	if err := c.AwaitMeteredUsage(context.Background(), settleContractFixture, totals); !errors.As(err, &coded) || coded.Code != core.CodeBillingSettling {
		t.Fatalf("short wait = %v, want BILLING_SETTLING", err)
	}
	caughtUpAt.Store(time.Now().Add(200 * time.Millisecond).UnixNano())
	if err := c.AwaitMeteredUsage(core.WithSettleWait(context.Background(), 30*time.Second), settleContractFixture, totals); err != nil {
		t.Fatalf("longer caller wait = %v, want caught up", err)
	}
	if DefaultSettleWait+settleChargeReserve > 25*time.Second {
		t.Fatalf("DefaultSettleWait %v plus the charge reserve no longer fits a request under the 30 s write timeout", DefaultSettleWait)
	}
}

// w22/m314: sandbox CPU and memory billed on use were exported on the weighted
// sandbox meter, so the settle expects them there, in its units, beside the
// weighted usage itself. (ExportedUsageTotals leaves out what was recorded
// before the switch, which was never sent.)
func TestSettleExpectsSandboxUsageOnTheWeightedMeter(t *testing.T) {
	summary := 0.0
	c, _ := settleMeterClient(t, &summary)
	c.priceIDs = []string{"price_sandbox"}
	c.priceByKey = map[string]string{store.UsageKindSandboxComputeSeconds: "price_sandbox"}
	c.priceMeters = map[string]string{"price_sandbox": "mtr_sandbox"}
	targets, err := c.meterTargets(context.Background(), []store.ExportedUsageTotal{
		{Kind: store.UsageKindSandboxComputeSeconds, ResourceKind: store.ResourceKindSandbox, Quantity: 1000, Recent: 1000},
		{Kind: store.UsageKindSandboxCPUSeconds, Tier: "1c-2g", ResourceKind: store.ResourceKindSandbox, Quantity: 600},
		{Kind: store.UsageKindSandboxMemoryGBSeconds, Tier: "1c-2g", ResourceKind: store.ResourceKindSandbox, Quantity: 7200, Recent: 7200},
	})
	if err != nil {
		t.Fatal(err)
	}
	got := targets["mtr_sandbox"]
	const cpu, memory = 563128.491620111725, 1069944.134078212383
	if len(targets) != 1 || got == nil || math.Abs(got.sent-(1000+cpu+memory)) > 1e-6 || math.Abs(got.recent-(1000+memory)) > 1e-6 {
		t.Fatalf("targets = %+v (%+v), want one sandbox meter expecting %.6f sent and %.6f recent", targets, got, 1000+cpu+memory, 1000+memory)
	}
}
