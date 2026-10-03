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
	"fmt"
	"log"
	"time"

	stripe "github.com/stripe/stripe-go/v86"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

// DefaultReclaimHorizon is how long an unbound Customer must have existed —
// and how long since its workspace last opened Checkout — before the
// reclaimer may delete it. It is far above Checkout's 35-minute session
// lifetime, so nobody can still be standing on the payment page.
const DefaultReclaimHorizon = 24 * time.Hour

const (
	reclaimBatch              = 10
	reclaimInterval           = 15 * time.Minute
	metricMappingReclaim      = "abandoned_checkout_reclaim"
	metricExpiredSessionSweep = "expired_session_reclaim"
)

// CheckoutReclaimStore is the persistence seam of the abandoned-checkout
// reclaimer. *store.PGStore satisfies it.
type CheckoutReclaimStore interface {
	ClaimUnboundBillingCustomers(ctx context.Context, livemode bool, horizon time.Duration, limit int) ([]store.BillingProviderMapping, error)
	DeleteUnboundBillingProviderMapping(ctx context.Context, workspaceID, customerID string) error
}

// CheckoutReclaimer deletes Stripe Customers that never bound a payment
// method. Two sources feed it:
//
//   - mappings minted while opening Checkout still created the Customer and a
//     cardless Subscription up front, which nothing reclaimed;
//   - Customers Stripe created for an expired customer_creation=always session
//     whose submit failed (a declined card), which no completion ever adopted.
//
// It runs only while the payment gate is on: with the gate off the usage
// emitter legitimately provisions cardless Customers and would re-mint them.
// Comped workspaces are never claimed.
type CheckoutReclaimer struct {
	Store    CheckoutReclaimStore
	Provider *StripeClient
	Metrics  *Metrics
	Interval time.Duration
	Horizon  time.Duration
	now      func() time.Time
}

func (r *CheckoutReclaimer) Run(ctx context.Context) {
	if r.Store == nil || r.Provider == nil {
		return
	}
	interval := r.Interval
	if interval <= 0 {
		interval = reclaimInterval
	}
	r.runOnce(ctx)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.runOnce(ctx)
		}
	}
}

func (r *CheckoutReclaimer) horizon() time.Duration {
	if r.Horizon > 0 {
		return r.Horizon
	}
	return DefaultReclaimHorizon
}

func (r *CheckoutReclaimer) clock() time.Time {
	if r.now != nil {
		return r.now()
	}
	return time.Now()
}

func (r *CheckoutReclaimer) runOnce(ctx context.Context) {
	mappings, err := r.Store.ClaimUnboundBillingCustomers(ctx, r.Provider.ExpectedLivemode(), r.horizon(), reclaimBatch)
	if err != nil {
		log.Printf("billing: abandoned-checkout reclaim claim failed: %v", err)
		r.Metrics.Operation(metricMappingReclaim, "error")
		return
	}
	for _, m := range mappings {
		reclaimed, err := r.Provider.ReclaimUnboundCustomer(ctx, m.WorkspaceID, m.CustomerID, r.clock().Add(-r.horizon()))
		if err != nil {
			log.Printf("billing: reclaim unbound Customer %s for %s: %v", m.CustomerID, m.WorkspaceID, err)
			r.Metrics.Operation(metricMappingReclaim, "error")
			continue
		}
		if !reclaimed {
			r.Metrics.Operation(metricMappingReclaim, "kept")
			continue
		}
		if err := r.Store.DeleteUnboundBillingProviderMapping(ctx, m.WorkspaceID, m.CustomerID); err != nil {
			log.Printf("billing: drop reclaimed mapping for %s: %v", m.WorkspaceID, err)
			r.Metrics.Operation(metricMappingReclaim, "error")
			continue
		}
		r.Metrics.Operation(metricMappingReclaim, "success")
	}
	// Sessions live 35 minutes, so twice the horizon comfortably covers every
	// session that could have expired since the last successful pass.
	deleted, err := r.Provider.ReclaimExpiredSessionCustomers(ctx, r.clock().Add(-2*r.horizon()))
	if err != nil {
		log.Printf("billing: reclaim expired-session Customers: %v", err)
		r.Metrics.Operation(metricExpiredSessionSweep, "error")
	}
	for range deleted {
		r.Metrics.Operation(metricExpiredSessionSweep, "success")
	}
}

// ReclaimUnboundCustomer deletes a workspace's Customer — after cancelling its
// bex Subscriptions — when Stripe confirms nothing worth keeping is attached:
// it is this workspace's, older than createdBefore, has no default or attached
// payment method, no invoice that ever carried money, no open Checkout
// session, and no non-zero charges accrued in the current period. It reports
// false (kept) otherwise. A Customer already gone counts as reclaimed so the
// stale mapping is dropped. A session opened on the Customer between the
// open-session probe and the delete would be stranded; the caller's 24h
// checkout_started_at filter makes that window practically unreachable.
func (c *StripeClient) ReclaimUnboundCustomer(ctx context.Context, workspaceID, customerID string, createdBefore time.Time) (bool, error) {
	customer, gone, err := c.liveCustomer(ctx, customerID)
	if err != nil {
		return false, err
	}
	if gone {
		c.forgetCustomer(workspaceID)
		return true, nil
	}
	if customer.Metadata[workspaceMetadataKey] != workspaceID || customer.Metadata[deletedAtMetadataKey] != "" ||
		customer.Created >= createdBefore.Unix() || customerDefaultPaymentMethod(customer) != "" {
		return false, nil
	}
	if keep, err := c.customerHasPaymentMethod(ctx, customerID); err != nil || keep {
		return false, err
	}
	if keep, err := c.customerHasMoneyInvoice(ctx, customerID); err != nil || keep {
		return false, err
	}
	if keep, err := c.customerHasOpenCheckout(ctx, customerID); err != nil || keep {
		return false, err
	}
	if keep, err := c.customerHasAccruedCharges(ctx, workspaceID, customerID); err != nil || keep {
		return false, err
	}
	if err := c.cancelWorkspaceSubscriptions(ctx, workspaceID, customerID); err != nil {
		return false, err
	}
	if err := c.deleteCustomer(ctx, customerID); err != nil {
		return false, err
	}
	c.forgetCustomer(workspaceID)
	return true, nil
}

// ReclaimExpiredSessionCustomers deletes Customers that an expired
// customer_creation=always session created (Stripe does so on a submit
// attempt, even a declined one) and that no completion adopted. Only sessions
// created after createdAfter are scanned; each candidate is re-read, and one
// carrying any workspace tag is left alone.
func (c *StripeClient) ReclaimExpiredSessionCustomers(ctx context.Context, createdAfter time.Time) ([]string, error) {
	params := &stripe.CheckoutSessionListParams{
		Status:       stripe.String(string(stripe.CheckoutSessionStatusExpired)),
		CreatedRange: &stripe.RangeQueryParams{GreaterThanOrEqual: createdAfter.Unix()},
	}
	params.Context = ctx
	params.Limit = stripe.Int64(100)
	// Expanded so each candidate costs no extra read; already-deleted
	// Customers come back with deleted=true and are skipped for free.
	params.AddExpand("data.customer")
	iter := c.sc.CheckoutSessions.List(params)
	var deleted []string
	for iter.Next() {
		session := iter.CheckoutSession()
		workspaceID := session.Metadata[workspaceMetadataKey]
		if workspaceID == "" || session.Customer == nil || session.Customer.ID == "" ||
			session.CustomerCreation != stripe.CheckoutSessionCustomerCreationAlways || !c.expectedLivemode(session.Livemode) {
			continue
		}
		customer := session.Customer
		if customer.Deleted || customer.Metadata[workspaceMetadataKey] != "" {
			continue // already gone, or adopted by a completion
		}
		if err := c.deleteCustomer(ctx, customer.ID); err != nil {
			return deleted, err
		}
		deleted = append(deleted, customer.ID)
	}
	if err := iter.Err(); err != nil {
		return deleted, fmt.Errorf("stripe: list expired Checkout Sessions: %w", err)
	}
	return deleted, nil
}

func (c *StripeClient) customerHasPaymentMethod(ctx context.Context, customerID string) (bool, error) {
	params := &stripe.PaymentMethodListParams{Customer: stripe.String(customerID)}
	params.Context = ctx
	params.Limit = stripe.Int64(1)
	iter := c.sc.PaymentMethods.List(params)
	has := iter.Next()
	if err := iter.Err(); err != nil {
		return false, fmt.Errorf("stripe: list payment methods for %s: %w", customerID, err)
	}
	return has, nil
}

func (c *StripeClient) customerHasMoneyInvoice(ctx context.Context, customerID string) (bool, error) {
	params := &stripe.InvoiceListParams{Customer: stripe.String(customerID)}
	params.Context = ctx
	params.Limit = stripe.Int64(100)
	iter := c.sc.Invoices.List(params)
	for iter.Next() {
		inv := iter.Invoice()
		if inv.Total != 0 || inv.AmountPaid != 0 || inv.AmountDue != 0 {
			return true, nil
		}
	}
	if err := iter.Err(); err != nil {
		return false, fmt.Errorf("stripe: list invoices for %s: %w", customerID, err)
	}
	return false, nil
}

func (c *StripeClient) customerHasOpenCheckout(ctx context.Context, customerID string) (bool, error) {
	params := &stripe.CheckoutSessionListParams{
		Customer: stripe.String(customerID),
		Status:   stripe.String(string(stripe.CheckoutSessionStatusOpen)),
	}
	params.Context = ctx
	params.Limit = stripe.Int64(1)
	iter := c.sc.CheckoutSessions.List(params)
	has := iter.Next()
	if err := iter.Err(); err != nil {
		return false, fmt.Errorf("stripe: list open Checkout Sessions for %s: %w", customerID, err)
	}
	return has, nil
}

// customerHasAccruedCharges previews the upcoming invoice of every live bex
// Subscription: usage exported before the payment gate was on would otherwise
// vanish with the Customer.
func (c *StripeClient) customerHasAccruedCharges(ctx context.Context, workspaceID, customerID string) (bool, error) {
	params := &stripe.SubscriptionListParams{Customer: stripe.String(customerID), Status: stripe.String("all")}
	params.Context = ctx
	params.Limit = stripe.Int64(100)
	iter := c.sc.Subscriptions.List(params)
	for iter.Next() {
		sub := iter.Subscription()
		if sub.Metadata[workspaceMetadataKey] != workspaceID || sub.Status == stripe.SubscriptionStatusCanceled || sub.Status == stripe.SubscriptionStatusIncompleteExpired {
			continue
		}
		preview := &stripe.InvoiceCreatePreviewParams{Customer: stripe.String(customerID), Subscription: stripe.String(sub.ID)}
		preview.Context = ctx
		inv, err := c.sc.Invoices.CreatePreview(preview)
		if err != nil {
			return false, fmt.Errorf("stripe: preview invoice for %s: %w", sub.ID, err)
		}
		if inv.Total != 0 || inv.AmountDue != 0 {
			return true, nil
		}
	}
	if err := iter.Err(); err != nil {
		return false, fmt.Errorf("stripe: list Subscriptions for %s: %w", customerID, err)
	}
	return false, nil
}
