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
	"net/http"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
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
		{Kind: store.UsageKindEgressBytes, Quantity: 3 * bytesPerGiB, Recent: 3 * bytesPerGiB},
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
		{Kind: store.UsageKindEgressBytes, Quantity: 3 * bytesPerGiB, Recent: 3 * bytesPerGiB},
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
		{Kind: store.UsageKindEgressBytes, Quantity: 3 * bytesPerGiB, Recent: bytesPerGiB},
	})
	if err != nil {
		t.Fatalf("an old shortfall blocked the settle: %v", err)
	}
}
