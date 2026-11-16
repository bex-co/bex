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
	"net/http"
	"testing"
	"time"

	stripe "github.com/stripe/stripe-go/v86"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

// w21/098, end to end against Postgres: the abandoned-checkout reclaimer
// cancels a cardless workspace's Subscription, and the customer.subscription.
// deleted event Stripe sends for it must not open grace or queue a notice. The
// same event for a card-bound workspace, and a cancellation Stripe ends after a
// failed payment, still do.
func TestReclaimerCancellationOfACardlessWorkspaceOpensNoGrace(t *testing.T) {
	pg, pool := writeOffTestStore(t)
	ctx := context.Background()
	h := &Lifecycle{Store: pg, GracePeriod: 7 * 24 * time.Hour}
	deleted := func(workspaceID, customerID, subscriptionID, reason string) *stripe.Event {
		return lifecycleEvent("evt_098_"+workspaceID, stripe.EventTypeCustomerSubscriptionDeleted,
			canceledSubscription(workspaceID, customerID, subscriptionID, reason))
	}
	tenantWithMapping := func(prefix, customerID, subscriptionID string, bound bool) store.Tenant {
		t.Helper()
		tenant := writeOffTestTenant(t, pg, prefix)
		if err := pg.UpsertBillingProviderMapping(ctx, store.BillingProviderMapping{WorkspaceID: tenant.ID, CustomerID: customerID, SubscriptionID: subscriptionID}); err != nil {
			t.Fatal(err)
		}
		if bound {
			if err := pg.SetPaymentMethodBound(ctx, tenant.ID, time.Now().Add(-time.Hour)); err != nil {
				t.Fatal(err)
			}
		}
		return tenant
	}
	notices := func(workspaceID string) (n int) {
		t.Helper()
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM billing_notifications WHERE workspace_id=$1`, workspaceID).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}

	// The reclaimer's own pass: the stub answers its Subscription cancel, and
	// that answer is the object the deleted event carries.
	cardless := tenantWithMapping("reclaim-cardless", "cus_r", "sub_r", false)
	fixture := reclaimFixture{created: time.Now().Add(-72 * time.Hour), owner: cardless.ID}
	c, stub := newStripeTest(t, fixture.route)
	reclaimed, err := c.ReclaimUnboundCustomer(ctx, cardless.ID, "cus_r", time.Now().Add(-DefaultReclaimHorizon))
	if err != nil || !reclaimed || stub.count("/v1/subscriptions/sub_r") != 1 {
		t.Fatalf("reclaim = %v err=%v calls=%v, want the cardless Customer's Subscription cancelled", reclaimed, err, stub.hits)
	}
	_, cancelBody := fixture.route(http.MethodDelete, "/v1/subscriptions/sub_r")
	if err := h.HandleStripeEvent(ctx, lifecycleEvent("evt_098_reclaim", stripe.EventTypeCustomerSubscriptionDeleted, cancelBody)); err != nil {
		t.Fatal(err)
	}
	if state, err := pg.GetBillingLifecycle(ctx, cardless.ID); err != nil || state.Status != store.BillingHealthy || state.Reason != "" || state.GraceDeadline != nil || notices(cardless.ID) != 0 {
		t.Fatalf("reclaimed cardless workspace = %+v notices=%d err=%v, want healthy with no notice", state, notices(cardless.ID), err)
	}

	// A card-bound workspace whose Subscription is cancelled (here through the
	// API, whatever the reason) is still in grace and still told.
	bound := tenantWithMapping("reclaim-bound", "cus_098_bound", "sub_098_bound", true)
	if err := h.HandleStripeEvent(ctx, deleted(bound.ID, "cus_098_bound", "sub_098_bound", "cancellation_requested")); err != nil {
		t.Fatal(err)
	}
	if state, err := pg.GetBillingLifecycle(ctx, bound.ID); err != nil || state.Status != store.BillingGrace || state.Reason != store.BillingReasonSubscriptionCanceled || notices(bound.ID) != 1 {
		t.Fatalf("card-bound cancellation = %+v notices=%d err=%v, want grace with one notice", state, notices(bound.ID), err)
	}

	// Stripe's own cancellation after failed collection counts for a workspace
	// whose marker is unset too (a mapping from before the marker existed).
	dunned := tenantWithMapping("reclaim-dunned", "cus_098_dunned", "sub_098_dunned", false)
	if err := h.HandleStripeEvent(ctx, deleted(dunned.ID, "cus_098_dunned", "sub_098_dunned", "payment_failed")); err != nil {
		t.Fatal(err)
	}
	if state, err := pg.GetBillingLifecycle(ctx, dunned.ID); err != nil || state.Status != store.BillingGrace || notices(dunned.ID) != 1 {
		t.Fatalf("Stripe's cancellation after failed collection = %+v notices=%d err=%v, want grace with one notice", state, notices(dunned.ID), err)
	}
}

// The reconciler's poll of a canceled Subscription reads the same fact: only a
// cancellation Stripe attributes to a failed payment or a dispute is marked.
func TestBillingSnapshotMarksOnlyCollectionCancellations(t *testing.T) {
	for _, tc := range []struct {
		reason string
		want   bool
	}{
		{"cancellation_requested", false},
		{"payment_failed", true},
		{"payment_disputed", true},
		{"canceled_by_retention_policy", false},
		{"", false},
	} {
		c, _ := newStripeTest(t, func(method, path string) (int, string) {
			if method == http.MethodGet && path == "/v1/subscriptions/sub_s" {
				return 200, canceledSubscription("tea-s", "cus_s", "sub_s", tc.reason)
			}
			return 500, `{"error":{"type":"api_error","message":"unexpected route"}}`
		})
		event, err := c.BillingSnapshot(context.Background(), store.BillingProviderMapping{WorkspaceID: "tea-s", CustomerID: "cus_s", SubscriptionID: "sub_s"}, time.Now())
		if err != nil {
			t.Fatalf("reason %q: %v", tc.reason, err)
		}
		if event.Outcome != store.BillingOutcomeFailure || event.Reason != store.BillingReasonSubscriptionCanceled || event.CollectionFailed != tc.want {
			t.Fatalf("reason %q: snapshot = %+v, want a subscription_canceled failure with CollectionFailed=%v", tc.reason, event, tc.want)
		}
	}
}
