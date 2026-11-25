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
	"cmp"
	"context"
	"errors"
	"fmt"
	"log"
	"slices"
	"strings"
	"time"

	stripe "github.com/stripe/stripe-go/v86"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

// Paying an overdue invoice the moment a card is bound (w6/m626). Render
// suspends an unpaid workspace "until all outstanding amounts are paid in
// full" (render.com/terms), and its Pay Now pays an invoice on Stripe at once.
// Without this, a card bound while dunning holds a workspace (grace,
// enforcing, enforced) only became the default: the invoice waited for
// Stripe's next scheduled retry, days away under Smart Retries, and so did
// every service enforcement had suspended.
//
// So when a card binds, through bex's Checkout or (once the endpoint sends
// customer.updated, w2/070) the Billing Portal, bex pays each unpaid cycle
// invoice of the live Subscription with it, off-session, oldest first. Each
// invoice gets one attempt per card (store.ClaimOverdueInvoicePayment): a
// declined card is never charged again for the same invoice, and the attempts
// stop at the first decline. When every one is paid, the recovery is recorded
// at once, through the same transition the invoice.paid webhook drives, so
// recovery starts without waiting for the webhook or the reconcile poll. A
// decline leaves the lifecycle as it is and records its reason; the member
// still has Pay now, Stripe's hosted page, which can authenticate a card the
// off-session attempt could not.
//
// The attempt is best-effort: the binding has already succeeded, and a
// failure here is logged and counted, never returned, so it cannot make
// Stripe redeliver the event that bound the card.

// OverdueStore is the durable side of paying at binding; *store.PGStore
// satisfies it.
type OverdueStore interface {
	GetBillingLifecycle(ctx context.Context, workspaceID string) (store.BillingLifecycle, error)
	ClaimOverdueInvoicePayment(ctx context.Context, workspaceID, invoiceID, cardKey string, at time.Time) (int, error)
	RecordOverdueInvoicePayment(ctx context.Context, workspaceID, invoiceID, cardKey, outcome, reason string) error
	RecordStripeBillingEvent(ctx context.Context, e store.StripeBillingEvent, grace time.Duration) (store.BillingLifecycle, bool, bool, error)
}

// OverduePayments turns on paying at binding. The composition root sets it
// only when dunning runs; GracePeriod is dunning's, which the recorded
// recovery goes through the lifecycle store with.
type OverduePayments struct {
	Store       OverdueStore
	GracePeriod time.Duration
}

// OverdueInvoice is the invoice a workspace in dunning owes: the oldest unpaid
// cycle invoice of its live Subscription. HostedInvoiceURL is Stripe's hosted
// page for it, the lifecycle alert's Pay now, omitted when Stripe gives none.
type OverdueInvoice struct {
	ID               string `json:"id"`
	HostedInvoiceURL string `json:"hostedInvoiceUrl,omitempty"`
}

// inDunning reports the lifecycle states in which an invoice is overdue:
// grace, and enforcement in progress or in force. Recovering has been paid.
func inDunning(status string) bool {
	return status == store.BillingGrace || status == store.BillingEnforcing || status == store.BillingEnforced
}

// oldestFirst orders invoices by creation, then id.
func oldestFirst(invoices []*stripe.Invoice) {
	slices.SortFunc(invoices, func(a, b *stripe.Invoice) int {
		return cmp.Or(cmp.Compare(a.Created, b.Created), strings.Compare(a.ID, b.ID))
	})
}

// OverdueInvoice returns the workspace's oldest unpaid cycle invoice, or nil
// when it has none (or no live contract). Service.Status asks only while the
// lifecycle is in dunning, so a healthy workspace costs no Stripe read.
func (c *StripeClient) OverdueInvoice(ctx context.Context, workspaceID string) (*OverdueInvoice, error) {
	customerID, found, err := c.findCustomer(ctx, workspaceID)
	if err != nil || !found {
		return nil, err
	}
	sub, err := c.liveSubscription(ctx, workspaceID, customerID, false)
	if err != nil || sub == nil {
		return nil, err
	}
	invoices, err := c.unpaidContractInvoices(ctx, SettleContract{WorkspaceID: workspaceID, CustomerID: customerID, SubscriptionID: sub.ID})
	if err != nil || len(invoices) == 0 {
		return nil, err
	}
	oldestFirst(invoices)
	return &OverdueInvoice{ID: invoices[0].ID, HostedInvoiceURL: invoices[0].HostedInvoiceURL}, nil
}

// cardKey names the card an attempt is made with: its fingerprint, which a
// card entered again in a new Checkout keeps, or the PaymentMethod id for a
// method that has none.
func cardKey(pm *stripe.PaymentMethod) string {
	if pm.Card != nil && pm.Card.Fingerprint != "" {
		return "card:" + pm.Card.Fingerprint
	}
	return "pm:" + pm.ID
}

// payOverdueInvoices pays the workspace's unpaid cycle invoices with the card
// just bound, when dunning holds the workspace (see the file comment).
func (c *StripeClient) payOverdueInvoices(ctx context.Context, workspaceID, customerID, subscriptionID string, pm *stripe.PaymentMethod) {
	if c.overdue == nil {
		return
	}
	state, err := c.overdue.Store.GetBillingLifecycle(ctx, workspaceID)
	if errors.Is(err, store.ErrNotFound) {
		return
	}
	if err != nil {
		c.metrics.Operation("overdue_payment", "error")
		log.Printf("billing: read lifecycle of %s before paying overdue invoices: %v", workspaceID, err)
		return
	}
	if !inDunning(state.Status) {
		return
	}
	contract := SettleContract{WorkspaceID: workspaceID, CustomerID: customerID, SubscriptionID: subscriptionID}
	invoices, err := c.unpaidContractInvoices(ctx, contract)
	if err != nil {
		c.metrics.Operation("overdue_payment", "error")
		log.Printf("billing: list overdue invoices of %s: %v", workspaceID, err)
		return
	}
	if len(invoices) == 0 {
		return
	}
	oldestFirst(invoices)
	key := cardKey(pm)
	paid := 0
	for _, invoice := range invoices {
		outcome := c.payOverdueInvoice(ctx, workspaceID, invoice, pm.ID, key)
		if outcome == store.OverduePaymentPaid {
			paid++
			continue
		}
		// A declined card is not offered the next invoice; anything else
		// (another attempt in flight, a Stripe error) leaves the rest to the
		// next binding, Stripe's schedule or Pay now.
		break
	}
	if paid == len(invoices) {
		c.recordOverdueRecovery(ctx, contract, invoices[len(invoices)-1].ID)
	}
}

// payOverdueInvoice makes the one claimed attempt at invoice with the card,
// and returns its recorded outcome, or "" when the card already had its
// attempt at this invoice.
func (c *StripeClient) payOverdueInvoice(ctx context.Context, workspaceID string, invoice *stripe.Invoice, paymentMethodID, key string) string {
	o := c.overdue
	attempt, err := o.Store.ClaimOverdueInvoicePayment(ctx, workspaceID, invoice.ID, key, time.Now().UTC())
	if err != nil {
		c.metrics.Operation("overdue_payment", "error")
		log.Printf("billing: claim payment of overdue invoice %s of %s: %v", invoice.ID, workspaceID, err)
		return store.OverduePaymentError
	}
	if attempt == 0 {
		c.metrics.Operation("overdue_payment", "already_attempted")
		return ""
	}
	params := &stripe.InvoicePayParams{PaymentMethod: stripe.String(paymentMethodID), OffSession: stripe.Bool(true)}
	params.Context = ctx
	params.SetIdempotencyKey(fmt.Sprintf("bex-overdue-pay-%s-%s-%d", invoice.ID, paymentMethodID, attempt))
	result, err := c.sc.Invoices.Pay(invoice.ID, params)
	outcome, reason := overduePaymentOutcome(result, err)
	c.metrics.Operation("overdue_payment", outcome)
	log.Printf("billing: overdue invoice %s of %s paid with the newly bound card: %s %s", invoice.ID, workspaceID, outcome, reason)
	if err := o.Store.RecordOverdueInvoicePayment(ctx, workspaceID, invoice.ID, key, outcome, reason); err != nil {
		log.Printf("billing: record payment of overdue invoice %s of %s: %v", invoice.ID, workspaceID, err)
	}
	return outcome
}

// overduePaymentOutcome classifies Invoices.Pay's answer: paid; declined, with
// the issuer's decline code or Stripe's error code, when the card or its
// holder must act; or error, for anything Stripe did not decide about the
// card (a processing payment, an invoice Stripe would not pay, an outage).
func overduePaymentOutcome(invoice *stripe.Invoice, err error) (outcome, reason string) {
	if err == nil {
		if invoice != nil && (invoice.Status == stripe.InvoiceStatusPaid || invoice.AmountRemaining <= 0) {
			return store.OverduePaymentPaid, ""
		}
		status := "unknown"
		if invoice != nil {
			status = string(invoice.Status)
		}
		return store.OverduePaymentError, "invoice_" + status
	}
	code, _ := safeStripeError(err)
	if chargeDeclined(err) || paymentNeedsCustomer(code) {
		var se *stripe.Error
		if errors.As(err, &se) && se.DeclineCode != "" {
			return store.OverduePaymentDeclined, string(se.DeclineCode)
		}
		return store.OverduePaymentDeclined, code
	}
	return store.OverduePaymentError, code
}

// recordOverdueRecovery records the paid invoices as the invoice.paid
// webhook would, so the lifecycle moves (to recovering after enforcement, to
// healthy from grace) without waiting for it. The webhook and the reconcile
// poll that follow read the same success and change nothing.
func (c *StripeClient) recordOverdueRecovery(ctx context.Context, contract SettleContract, newestInvoiceID string) {
	now := time.Now().UTC()
	event := store.StripeBillingEvent{
		EventID: "bex-overdue-paid:" + newestInvoiceID, EventType: "billing.invoice.paid_at_binding",
		WorkspaceID: contract.WorkspaceID, CustomerID: contract.CustomerID, SubscriptionID: contract.SubscriptionID,
		ObjectID: newestInvoiceID, Livemode: c.ExpectedLivemode(), ProviderCreatedAt: now, ReceivedAt: now,
		Outcome: store.BillingOutcomeSuccess, Reason: "payment_succeeded",
	}
	if _, _, _, err := c.overdue.Store.RecordStripeBillingEvent(ctx, event, c.overdue.GracePeriod); err != nil {
		c.metrics.Operation("overdue_recovery", "error")
		log.Printf("billing: record the recovery of %s after paying its overdue invoices: %v (the invoice.paid webhook or the reconcile poll records it)", contract.WorkspaceID, err)
		return
	}
	c.metrics.Operation("overdue_recovery", "success")
}
