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
	"time"

	stripe "github.com/stripe/stripe-go/v86"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

var errNotBexBillingContract = errors.New("not a bex billing contract")

// LifecycleStore is the durable intake seam. *store.PGStore satisfies it;
// tests use a small fake so event normalization stays independent of Postgres.
type LifecycleStore interface {
	RecordStripeBillingEvent(context.Context, store.StripeBillingEvent, time.Duration) (store.BillingLifecycle, bool, bool, error)
}

// Lifecycle accepts only already signature-verified Stripe Events, reduces
// them to a non-sensitive normalized transition, and commits before the
// webhook is acknowledged.
type Lifecycle struct {
	Store            LifecycleStore
	GracePeriod      time.Duration
	ExpectedLivemode bool
	Clock            func() time.Time
}

func (l *Lifecycle) now() time.Time {
	if l.Clock != nil {
		return l.Clock().UTC()
	}
	return time.Now().UTC()
}

func (l *Lifecycle) HandleStripeEvent(ctx context.Context, event *stripe.Event) error {
	if l == nil || l.Store == nil {
		return fmt.Errorf("billing lifecycle store unavailable")
	}
	if event == nil || event.ID == "" || event.Data == nil {
		return fmt.Errorf("invalid Stripe lifecycle event")
	}
	if event.Livemode != l.ExpectedLivemode {
		return fmt.Errorf("Stripe event mode mismatch")
	}
	if l.GracePeriod <= 0 {
		return fmt.Errorf("billing grace period is not configured")
	}
	normalized, err := normalizeStripeLifecycleEvent(event, l.now())
	if err != nil {
		if errors.Is(err, errNotBexBillingContract) {
			return nil
		}
		return err
	}
	_, _, _, err = l.Store.RecordStripeBillingEvent(ctx, normalized, l.GracePeriod)
	return err
}

func normalizeStripeLifecycleEvent(event *stripe.Event, receivedAt time.Time) (store.StripeBillingEvent, error) {
	e := store.StripeBillingEvent{
		EventID: event.ID, EventType: string(event.Type), Livemode: event.Livemode,
		ProviderCreatedAt: time.Unix(event.Created, 0).UTC(), ReceivedAt: receivedAt.UTC(),
	}
	if event.Created <= 0 {
		return e, fmt.Errorf("Stripe event %s has no provider timestamp", event.ID)
	}
	switch event.Type {
	case stripe.EventTypeInvoicePaymentFailed, stripe.EventTypeInvoicePaymentActionRequired,
		stripe.EventTypeInvoicePaymentSucceeded, stripe.EventTypeInvoicePaid:
		var invoice stripe.Invoice
		if err := json.Unmarshal(event.Data.Raw, &invoice); err != nil {
			return e, fmt.Errorf("invalid invoice event: %w", err)
		}
		workspaceID, subscriptionID, ok := invoiceContract(&invoice)
		if !ok {
			return e, fmt.Errorf("invoice %s: %w", invoice.ID, errNotBexBillingContract)
		}
		if invoice.Customer == nil || invoice.Customer.ID == "" {
			return e, fmt.Errorf("invoice %s has no customer", invoice.ID)
		}
		if invoice.Livemode != event.Livemode {
			return e, fmt.Errorf("invoice %s mode mismatch", invoice.ID)
		}
		e.WorkspaceID, e.CustomerID, e.SubscriptionID, e.ObjectID = workspaceID, invoice.Customer.ID, subscriptionID, invoice.ID
		switch event.Type {
		case stripe.EventTypeInvoicePaymentSucceeded, stripe.EventTypeInvoicePaid:
			e.Outcome, e.Reason = store.BillingOutcomeSuccess, "payment_succeeded"
		case stripe.EventTypeInvoicePaymentActionRequired:
			e.Outcome, e.Reason = store.BillingOutcomeFailure, "payment_action_required"
		default:
			e.Outcome, e.Reason = store.BillingOutcomeFailure, "payment_failed"
		}
	case stripe.EventTypeCustomerSubscriptionCreated, stripe.EventTypeCustomerSubscriptionUpdated,
		stripe.EventTypeCustomerSubscriptionDeleted, stripe.EventTypeCustomerSubscriptionPaused,
		stripe.EventTypeCustomerSubscriptionResumed:
		var sub stripe.Subscription
		if err := json.Unmarshal(event.Data.Raw, &sub); err != nil {
			return e, fmt.Errorf("invalid Subscription event: %w", err)
		}
		if sub.Metadata[subscriptionMetadataKey] != "true" || sub.Metadata[workspaceMetadataKey] == "" {
			return e, fmt.Errorf("Subscription %s: %w", sub.ID, errNotBexBillingContract)
		}
		if sub.Customer == nil || sub.Customer.ID == "" {
			return e, fmt.Errorf("Subscription %s has no customer", sub.ID)
		}
		if sub.Livemode != event.Livemode {
			return e, fmt.Errorf("Subscription %s mode mismatch", sub.ID)
		}
		e.WorkspaceID, e.CustomerID, e.SubscriptionID, e.ObjectID = sub.Metadata[workspaceMetadataKey], sub.Customer.ID, sub.ID, sub.ID
		if event.Type == stripe.EventTypeCustomerSubscriptionDeleted {
			e.Outcome, e.Reason = store.BillingOutcomeFailure, "subscription_canceled"
		} else {
			e.Outcome, e.Reason = subscriptionOutcome(sub.Status)
		}
	default:
		return e, fmt.Errorf("unsupported Stripe lifecycle event %s", event.Type)
	}
	return e, nil
}

func invoiceContract(invoice *stripe.Invoice) (workspaceID, subscriptionID string, ok bool) {
	if invoice == nil || invoice.Parent == nil || invoice.Parent.SubscriptionDetails == nil {
		return "", "", false
	}
	details := invoice.Parent.SubscriptionDetails
	if details.Metadata[subscriptionMetadataKey] != "true" {
		return "", "", false
	}
	workspaceID = details.Metadata[workspaceMetadataKey]
	if details.Subscription != nil {
		subscriptionID = details.Subscription.ID
	}
	return workspaceID, subscriptionID, workspaceID != "" && subscriptionID != ""
}

func subscriptionOutcome(status stripe.SubscriptionStatus) (string, string) {
	switch status {
	case stripe.SubscriptionStatusActive, stripe.SubscriptionStatusTrialing:
		return store.BillingOutcomeSuccess, "payment_succeeded"
	case stripe.SubscriptionStatusPastDue:
		return store.BillingOutcomeFailure, "subscription_past_due"
	case stripe.SubscriptionStatusUnpaid:
		return store.BillingOutcomeFailure, "subscription_unpaid"
	case stripe.SubscriptionStatusPaused:
		return store.BillingOutcomeFailure, "subscription_paused"
	case stripe.SubscriptionStatusCanceled:
		return store.BillingOutcomeFailure, "subscription_canceled"
	case stripe.SubscriptionStatusIncompleteExpired:
		return store.BillingOutcomeFailure, "subscription_incomplete_expired"
	default:
		return store.BillingOutcomeFailure, "subscription_" + string(status)
	}
}

func isLifecycleEvent(eventType stripe.EventType) bool {
	switch eventType {
	case stripe.EventTypeInvoicePaymentFailed, stripe.EventTypeInvoicePaymentActionRequired,
		stripe.EventTypeInvoicePaymentSucceeded, stripe.EventTypeInvoicePaid,
		stripe.EventTypeCustomerSubscriptionCreated, stripe.EventTypeCustomerSubscriptionUpdated,
		stripe.EventTypeCustomerSubscriptionDeleted, stripe.EventTypeCustomerSubscriptionPaused,
		stripe.EventTypeCustomerSubscriptionResumed:
		return true
	default:
		return false
	}
}
