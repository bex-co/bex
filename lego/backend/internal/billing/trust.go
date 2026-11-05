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
	"log"
	"slices"
	"time"

	stripe "github.com/stripe/stripe-go/v86"

	"github.com/bex-co/bex/lego/backend/internal/pricing"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// Trust feeds the agent-model trust tiers (ADR091 D10.1) from Stripe: a paid,
// non-zero bex invoice adds to the workspace's lifetime paid (and, for a
// subscription_cycle invoice, its paid cycle invoices); a failed invoice
// payment holds it at T0 until that invoice is paid, plus 30 days; a dispute
// holds it at T0 until the dispute closes as won. Every write is idempotent
// (paid invoices are keyed by invoice id), so Stripe's redelivery never
// double counts. Amounts, cards and payment details are never logged. The
// webhook has already rejected an event of the wrong mode.
type Trust struct {
	Store   TrustStore
	Charges DisputedChargeResolver
	// OnTierChanged is told after a committed tier change (the Stripe
	// agent-model threshold follows the tier); nil ⇒ nothing.
	OnTierChanged TierChangedFunc
	Clock         func() time.Time
}

// TierChangedFunc is told a workspace's trust tier changed through a
// webhook. A lock expiring changes the tier without an event; a consumer
// that must track it reads pricing.TrustTier on its own schedule.
type TierChangedFunc func(ctx context.Context, workspaceID string, tier pricing.Tier)

// TrustStore is the durable trust record; *store.PGStore satisfies it.
type TrustStore interface {
	RecordBillingTrustPayment(context.Context, store.BillingTrustPayment) (store.BillingTrust, store.BillingTrust, bool, error)
	RecordBillingTrustFailure(ctx context.Context, workspaceID, invoiceID string, at time.Time) (store.BillingTrust, store.BillingTrust, bool, error)
	RecordBillingTrustDispute(ctx context.Context, workspaceID, disputeID string) (store.BillingTrust, store.BillingTrust, error)
	RecordBillingTrustDisputeWon(ctx context.Context, workspaceID, disputeID string) (store.BillingTrust, store.BillingTrust, error)
	BillingWorkspaceForCustomer(ctx context.Context, customerID string, livemode bool) (string, bool, error)
}

// DisputedChargeResolver re-reads a disputed Charge's Customer from Stripe
// (a Dispute names only its Charge); *StripeClient satisfies it.
type DisputedChargeResolver interface {
	DisputedChargeCustomer(ctx context.Context, chargeID string) (string, error)
}

// trustEventTypes are the events Trust reduces. invoice.paid is the payment
// signal (invoice.payment_succeeded for the same invoice would be a
// duplicate the ledger absorbs anyway).
var trustEventTypes = []stripe.EventType{
	stripe.EventTypeInvoicePaid, stripe.EventTypeInvoicePaymentFailed,
	stripe.EventTypeChargeDisputeCreated, stripe.EventTypeChargeDisputeClosed,
}

func isTrustEvent(eventType stripe.EventType) bool {
	return slices.Contains(trustEventTypes, eventType)
}

func (t *Trust) now() time.Time {
	if t.Clock != nil {
		return t.Clock().UTC()
	}
	return time.Now().UTC()
}

// HandleStripeEvent reduces one signature-verified event. Events that are not
// bex's, or whose workspace is gone, are acknowledged; a store failure is
// returned so Stripe retries.
func (t *Trust) HandleStripeEvent(ctx context.Context, event *stripe.Event) error {
	if t == nil || t.Store == nil {
		return fmt.Errorf("billing trust store unavailable")
	}
	if event == nil || event.ID == "" || event.Data == nil || event.Created <= 0 {
		return fmt.Errorf("invalid Stripe trust event")
	}
	at := time.Unix(event.Created, 0).UTC()
	var (
		workspaceID   string
		before, after store.BillingTrust
		err           error
	)
	switch event.Type {
	case stripe.EventTypeInvoicePaid, stripe.EventTypeInvoicePaymentFailed:
		var invoice stripe.Invoice
		if err := json.Unmarshal(event.Data.Raw, &invoice); err != nil {
			return fmt.Errorf("invalid invoice event: %w", err)
		}
		ws, _, ok := invoiceContract(&invoice)
		if !ok || invoice.ID == "" {
			return nil
		}
		workspaceID = ws
		if event.Type == stripe.EventTypeInvoicePaymentFailed {
			before, after, _, err = t.Store.RecordBillingTrustFailure(ctx, ws, invoice.ID, at)
			break
		}
		if invoice.AmountPaid <= 0 {
			return nil // credits and $0 invoices are not payment history
		}
		paidAt := at
		if invoice.StatusTransitions != nil && invoice.StatusTransitions.PaidAt > 0 {
			paidAt = time.Unix(invoice.StatusTransitions.PaidAt, 0).UTC()
		}
		before, after, _, err = t.Store.RecordBillingTrustPayment(ctx, store.BillingTrustPayment{
			InvoiceID: invoice.ID, WorkspaceID: ws, AmountCents: invoice.AmountPaid,
			Cycle: invoice.BillingReason == stripe.InvoiceBillingReasonSubscriptionCycle, PaidAt: paidAt,
		})
	case stripe.EventTypeChargeDisputeCreated, stripe.EventTypeChargeDisputeClosed:
		var dispute stripe.Dispute
		if err := json.Unmarshal(event.Data.Raw, &dispute); err != nil {
			return fmt.Errorf("invalid dispute event: %w", err)
		}
		if dispute.ID == "" {
			return nil
		}
		if event.Type == stripe.EventTypeChargeDisputeClosed && dispute.Status != stripe.DisputeStatusWon {
			// A lost (or otherwise closed) dispute keeps the hold: ops lifts it
			// once the account is settled (migration 0193).
			return nil
		}
		ws, found, derr := t.disputedWorkspace(ctx, event, &dispute)
		if derr != nil || !found {
			return derr
		}
		workspaceID = ws
		if event.Type == stripe.EventTypeChargeDisputeCreated {
			before, after, err = t.Store.RecordBillingTrustDispute(ctx, ws, dispute.ID)
		} else {
			before, after, err = t.Store.RecordBillingTrustDisputeWon(ctx, ws, dispute.ID)
		}
	default:
		return nil
	}
	if errors.Is(err, store.ErrWorkspaceGone) {
		return nil
	}
	if err != nil {
		return err
	}
	t.notifyTierChange(ctx, workspaceID, before, after)
	return nil
}

// disputedWorkspace resolves a dispute's workspace through its Charge's
// Customer, re-read from Stripe (the event payload is never trusted for
// ownership) and mapped by billing_provider_mappings.
func (t *Trust) disputedWorkspace(ctx context.Context, event *stripe.Event, dispute *stripe.Dispute) (string, bool, error) {
	if dispute.Charge == nil || dispute.Charge.ID == "" {
		return "", false, nil
	}
	if t.Charges == nil {
		return "", false, fmt.Errorf("Stripe charge lookup unavailable")
	}
	customerID, err := t.Charges.DisputedChargeCustomer(ctx, dispute.Charge.ID)
	if err != nil {
		return "", false, err
	}
	if customerID == "" {
		return "", false, nil
	}
	workspaceID, found, err := t.Store.BillingWorkspaceForCustomer(ctx, customerID, event.Livemode)
	if err != nil {
		return "", false, err
	}
	if !found {
		log.Printf("billing trust: dispute event=%s names a customer bex does not map; acknowledged", event.ID)
	}
	return workspaceID, found, nil
}

func (t *Trust) notifyTierChange(ctx context.Context, workspaceID string, before, after store.BillingTrust) {
	if t.OnTierChanged == nil {
		return
	}
	now := t.now()
	was := pricing.TrustTier(before.LifetimePaidCents, before.PaidCycleInvoices, before.Lock(), now)
	is := pricing.TrustTier(after.LifetimePaidCents, after.PaidCycleInvoices, after.Lock(), now)
	if was.Level != is.Level {
		t.OnTierChanged(ctx, workspaceID, is)
	}
}

// DisputedChargeCustomer re-reads a Charge and returns its Customer id ("" for
// a customer-less charge). Only the id leaves this function.
func (c *StripeClient) DisputedChargeCustomer(ctx context.Context, chargeID string) (string, error) {
	params := &stripe.ChargeParams{}
	params.Context = ctx
	charge, err := c.sc.Charges.Get(chargeID, params)
	if resourceMissing(err) {
		return "", nil
	}
	if err != nil {
		code, _ := safeStripeError(err)
		return "", fmt.Errorf("stripe: retrieve disputed Charge: %s", code)
	}
	if charge.Customer == nil {
		return "", nil
	}
	return charge.Customer.ID, nil
}
