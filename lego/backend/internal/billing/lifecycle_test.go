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
	"encoding/json"
	"strings"
	"testing"
	"time"

	stripe "github.com/stripe/stripe-go/v86"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

type lifecycleCapture struct {
	events []store.StripeBillingEvent
	grace  time.Duration
}

func (c *lifecycleCapture) RecordStripeBillingEvent(_ context.Context, e store.StripeBillingEvent, grace time.Duration) (store.BillingLifecycle, bool, bool, error) {
	c.events = append(c.events, e)
	c.grace = grace
	return store.BillingLifecycle{WorkspaceID: e.WorkspaceID}, true, true, nil
}

func lifecycleEvent(id string, eventType stripe.EventType, object string) *stripe.Event {
	return &stripe.Event{
		ID: id, Type: eventType, Created: 1785196800, Livemode: false,
		Data: &stripe.EventData{Raw: json.RawMessage(object)},
	}
}

func TestLifecycleNormalizesTrustedInvoiceAndSubscriptionEvents(t *testing.T) {
	capture := &lifecycleCapture{}
	h := &Lifecycle{Store: capture, GracePeriod: 7 * 24 * time.Hour, Clock: func() time.Time {
		return time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC)
	}}
	invoice := `{"id":"in_1","object":"invoice","livemode":false,"customer":"cus_1","parent":{"type":"subscription_details","subscription_details":{"subscription":"sub_1","metadata":{"bex_workspace":"tea-a","bex_billing_contract":"true"}}}}`
	subscription := `{"id":"sub_1","object":"subscription","livemode":false,"customer":"cus_1","status":"active","metadata":{"bex_workspace":"tea-a","bex_billing_contract":"true"}}`

	for _, tc := range []struct {
		type_   stripe.EventType
		object  string
		outcome string
		reason  string
	}{
		{stripe.EventTypeInvoicePaymentFailed, invoice, store.BillingOutcomeFailure, "payment_failed"},
		{stripe.EventTypeInvoicePaymentActionRequired, invoice, store.BillingOutcomeFailure, "payment_action_required"},
		{stripe.EventTypeInvoicePaid, invoice, store.BillingOutcomeSuccess, "payment_succeeded"},
		{stripe.EventTypeCustomerSubscriptionUpdated, subscription, store.BillingOutcomeSuccess, "payment_succeeded"},
		{stripe.EventTypeCustomerSubscriptionDeleted, subscription, store.BillingOutcomeFailure, "subscription_canceled"},
	} {
		if err := h.HandleStripeEvent(context.Background(), lifecycleEvent("evt_"+string(tc.type_), tc.type_, tc.object)); err != nil {
			t.Fatalf("%s: %v", tc.type_, err)
		}
		got := capture.events[len(capture.events)-1]
		if got.WorkspaceID != "tea-a" || got.CustomerID != "cus_1" || got.SubscriptionID != "sub_1" || got.Outcome != tc.outcome || got.Reason != tc.reason || got.Livemode {
			t.Fatalf("%s normalized = %+v", tc.type_, got)
		}
	}
	if capture.grace != 7*24*time.Hour {
		t.Fatalf("grace = %s", capture.grace)
	}
}

func TestLifecycleIgnoresForeignContractsAndRejectsModeMismatch(t *testing.T) {
	capture := &lifecycleCapture{}
	h := &Lifecycle{Store: capture, GracePeriod: time.Hour}
	foreign := lifecycleEvent("evt_foreign", stripe.EventTypeCustomerSubscriptionUpdated,
		`{"id":"sub_other","object":"subscription","livemode":false,"customer":"cus_other","status":"past_due","metadata":{}}`)
	if err := h.HandleStripeEvent(context.Background(), foreign); err != nil {
		t.Fatalf("foreign contract should be ignored: %v", err)
	}
	if len(capture.events) != 0 {
		t.Fatalf("foreign contract persisted: %+v", capture.events)
	}

	live := lifecycleEvent("evt_live", stripe.EventTypeCustomerSubscriptionUpdated,
		`{"id":"sub_live","object":"subscription","livemode":true,"customer":"cus_live","status":"past_due","metadata":{"bex_workspace":"tea-a","bex_billing_contract":"true"}}`)
	live.Livemode = true
	if err := h.HandleStripeEvent(context.Background(), live); err == nil || !strings.Contains(err.Error(), "mode mismatch") {
		t.Fatalf("live event error = %v", err)
	}
}
