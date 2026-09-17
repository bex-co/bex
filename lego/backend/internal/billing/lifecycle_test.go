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
	"errors"
	"fmt"
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

// The w7/m52 test-only fence is lifted (w4/m81 t002): a live-mode Lifecycle is
// a supported configuration that records live events and rejects test-mode
// ones — the exact wiring main.go now threads from the runtime key's mode.
func TestLifecycleLiveModeRecordsLiveAndRejectsTestEvents(t *testing.T) {
	capture := &lifecycleCapture{}
	h := &Lifecycle{Store: capture, GracePeriod: time.Hour, ExpectedLivemode: true}

	live := lifecycleEvent("evt_live_ok", stripe.EventTypeCustomerSubscriptionUpdated,
		`{"id":"sub_live","object":"subscription","livemode":true,"customer":"cus_live","status":"past_due","metadata":{"bex_workspace":"tea-a","bex_billing_contract":"true"}}`)
	live.Livemode = true
	if err := h.HandleStripeEvent(context.Background(), live); err != nil {
		t.Fatalf("live lifecycle rejected a live event: %v", err)
	}
	if len(capture.events) != 1 || !capture.events[0].Livemode {
		t.Fatalf("live event not recorded as live: %+v", capture.events)
	}

	test := lifecycleEvent("evt_test_leak", stripe.EventTypeCustomerSubscriptionUpdated,
		`{"id":"sub_test","object":"subscription","livemode":false,"customer":"cus_test","status":"past_due","metadata":{"bex_workspace":"tea-a","bex_billing_contract":"true"}}`)
	if err := h.HandleStripeEvent(context.Background(), test); err == nil || !strings.Contains(err.Error(), "mode mismatch") {
		t.Fatalf("test event against live lifecycle = %v, want mode mismatch", err)
	}
	if len(capture.events) != 1 {
		t.Fatalf("test event persisted against live lifecycle: %+v", capture.events)
	}
}

// failingLifecycleStore returns a fixed error so the handler's disposition of
// store failures can be asserted directly.
type failingLifecycleStore struct{ err error }

func (f *failingLifecycleStore) RecordStripeBillingEvent(_ context.Context, e store.StripeBillingEvent, _ time.Duration) (store.BillingLifecycle, bool, bool, error) {
	return store.BillingLifecycle{WorkspaceID: e.WorkspaceID}, false, false, f.err
}

// A workspace's own deletion emits its last subscription event, so the event
// outlives the tenant row it references. Acknowledge it: returning an error here
// makes the webhook answer 503 and Stripe retry the same doomed write for three
// days, and persistent failures get endpoints disabled — which would silently
// drop checkout.session.completed, the sole writer of payment_method_bound_at.
func TestLifecycleAcknowledgesEventsForDeletedWorkspaces(t *testing.T) {
	h := &Lifecycle{
		Store:       &failingLifecycleStore{err: fmt.Errorf("billing provider mapping for tea-a: %w", store.ErrWorkspaceGone)},
		GracePeriod: 7 * 24 * time.Hour,
	}
	subscription := `{"id":"sub_1","object":"subscription","livemode":false,"customer":"cus_1","status":"canceled","metadata":{"bex_workspace":"tea-a","bex_billing_contract":"true"}}`

	err := h.HandleStripeEvent(context.Background(), lifecycleEvent("evt_gone", stripe.EventTypeCustomerSubscriptionDeleted, subscription))

	if err != nil {
		t.Fatalf("deleted-workspace event = %v, want nil so the webhook answers 204", err)
	}
}

// Only the gone-workspace case is benign. Every other store failure must still
// surface so Stripe retries rather than dropping a real billing transition.
func TestLifecyclePropagatesOtherStoreFailures(t *testing.T) {
	h := &Lifecycle{
		Store:       &failingLifecycleStore{err: errors.New("connection refused")},
		GracePeriod: 7 * 24 * time.Hour,
	}
	subscription := `{"id":"sub_1","object":"subscription","livemode":false,"customer":"cus_1","status":"canceled","metadata":{"bex_workspace":"tea-a","bex_billing_contract":"true"}}`

	err := h.HandleStripeEvent(context.Background(), lifecycleEvent("evt_down", stripe.EventTypeCustomerSubscriptionDeleted, subscription))

	if err == nil {
		t.Fatal("store outage = nil, want an error so Stripe retries")
	}
}
