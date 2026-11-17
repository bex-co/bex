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
	"strings"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/pricing"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

const cronMinimumSubscriptions = `{"object":"list","url":"/v1/subscriptions","has_more":false,"data":[{"id":"sub_1","object":"subscription","status":"active","metadata":{"bex_workspace":"tea-a","bex_billing_contract":"true"}}]}`

func cronMinimumEvent(microUSD int64) Event {
	return toEvent(store.HourlyRow{
		WorkspaceID: "tea-a", ServiceID: "crn-a", Kind: store.UsageKindCronMinimumMicroUSD,
		ResourceKind: store.ResourceKindService, WindowStart: time.Date(2026, 10, 31, 23, 0, 0, 0, time.UTC),
		Quantity: microUSD,
	})
}

// w9/m448: a cron minimum row is billed as one pending invoice item on the
// workspace's live Subscription, never as a meter event, named for the cron
// job and month and keyed by the row's identifier so a retry adds nothing.
func TestStripeIngestBatchBillsCronMinimumAsInvoiceItem(t *testing.T) {
	c, stub := newStripeTest(t, func(_ string, path string) (int, string) {
		switch {
		case strings.HasPrefix(path, "/v1/subscriptions"):
			return 200, cronMinimumSubscriptions
		case path == "/v1/invoiceitems":
			return 200, `{"id":"ii_1","object":"invoiceitem"}`
		}
		return 404, `{"error":{"type":"invalid_request_error","message":"unexpected"}}`
	})
	c.storeCustomer("tea-a", "cus_1")
	event := cronMinimumEvent(299995)
	result := c.IngestBatch(context.Background(), []Event{event})
	if len(result.Failed) != 0 || len(result.Accepted) != 1 || result.Accepted[0] != event.TransactionID {
		t.Fatalf("IngestBatch = %+v, want the cron minimum accepted", result)
	}
	if n := stub.count("/billing/meter_events"); n != 0 {
		t.Fatalf("a cron minimum sent %d meter events, want none", n)
	}
	items := stub.requests("/v1/invoiceitems")
	if len(items) != 1 {
		t.Fatalf("invoice items = %d, want 1", len(items))
	}
	form, err := url.ParseQuery(items[0].body)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"customer": "cus_1", "subscription": "sub_1", "amount": "30", "currency": "usd",
		"description":                "Cron job minimum: crn-a, October 2026",
		"metadata[bex_workspace]":    "tea-a",
		"metadata[bex_cron_minimum]": event.TransactionID,
	}
	for key, value := range want {
		if got := form.Get(key); got != value {
			t.Errorf("invoice item %s = %q, want %q", key, got, value)
		}
	}
	if key := items[0].header.Get("Idempotency-Key"); key != "bex-cron-minimum-"+event.TransactionID {
		t.Errorf("Idempotency-Key = %q, want the row's identifier", key)
	}
}

// Stripe's answer decides the row: a 4xx is a durable reject, a 5xx or a
// missing Subscription is retried, and a top-up under half a cent owes
// nothing and calls Stripe not at all.
func TestStripeCronMinimumOutcomes(t *testing.T) {
	for _, tc := range []struct {
		name               string
		value              int64
		subs               string
		status             int
		accepted           bool
		permanent, invoked bool
	}{
		{"declined by Stripe", 700000, cronMinimumSubscriptions, 400, false, true, true},
		{"Stripe unavailable", 700000, cronMinimumSubscriptions, 500, false, false, true},
		{"no live Subscription", 700000, `{"object":"list","url":"/v1/subscriptions","has_more":false,"data":[]}`, 200, false, false, false},
		{"under half a cent", 4999, cronMinimumSubscriptions, 200, true, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, stub := newStripeTest(t, func(_ string, path string) (int, string) {
				if strings.HasPrefix(path, "/v1/subscriptions") {
					return 200, tc.subs
				}
				if tc.status != 200 {
					return tc.status, `{"error":{"type":"invalid_request_error","message":"no"}}`
				}
				return 200, `{"id":"ii_1","object":"invoiceitem"}`
			})
			c.storeCustomer("tea-a", "cus_1")
			result := c.IngestBatch(context.Background(), []Event{cronMinimumEvent(tc.value)})
			if got := len(result.Accepted) == 1; got != tc.accepted {
				t.Fatalf("accepted = %v, want %v (%+v)", got, tc.accepted, result)
			}
			if !tc.accepted && (len(result.Failed) != 1 || result.Failed[0].Permanent != tc.permanent) {
				t.Fatalf("failure = %+v, want permanent=%v", result.Failed, tc.permanent)
			}
			if got := len(stub.requests("/v1/invoiceitems")) > 0; got != tc.invoked {
				t.Fatalf("invoice item posted = %v, want %v", got, tc.invoked)
			}
		})
	}
}

// The cron minimum is no catalog dimension: the Subscription stays within
// Stripe's 20 items, settle awaits no meter for it, and the emitter records
// the invoice item it becomes rather than a zero-rate local event.
func TestCronMinimumIsNotAMeter(t *testing.T) {
	for _, name := range pricing.Default.BillableMeterNames() {
		if strings.Contains(name, "cron") {
			t.Fatalf("catalog dimension %s: the cron minimum must not take a Subscription item", name)
		}
	}
	if _, _, skip := stripeMeterEvent(cronMinimumEvent(700000)); !skip {
		t.Fatal("stripeMeterEvent maps the cron minimum to a meter")
	}
	if got := cronMinimumCents(5000); got != 1 {
		t.Fatalf("cronMinimumCents(5000) = %d, want 1 (half a cent rounds up)", got)
	}
}
