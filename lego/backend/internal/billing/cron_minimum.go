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
	"strconv"

	stripe "github.com/stripe/stripe-go/v86"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

// The cron job minimum (w9/m448, docs/ADR030 § Cron job minimum) reaches
// Stripe as an invoice item, not a meter event: every catalog Price is one
// item on the workspace's Subscription, Stripe caps a Subscription at 20
// items, and the catalog has 20 (TestCatalogFitsOneSubscription). A
// cron_minimum_microusd row becomes one pending invoice item on the live
// Subscription, so the next invoice carries a "Cron job minimum" line for that
// cron job and month. The row's deterministic identifier is the item's
// idempotency key, so a retry inside Stripe's 24-hour key window creates
// nothing more, the same bound the meter events' identifiers have.

// cronMinimumEventName is the export attempt's event name for a cron minimum
// row: the invoice item it becomes, not a meter.
const cronMinimumEventName = "invoice_item.cron_minimum"

// cronMinimumMetadataKey marks a cron minimum invoice item with the export
// identifier of the row it bills.
const cronMinimumMetadataKey = "bex_cron_minimum"

// cronMinimumCents bills micro-USD in cents, rounded half up.
func cronMinimumCents(microUSD int64) int64 {
	return (microUSD + microUSDPerCent/2) / microUSDPerCent
}

// resourceNameReader reads the retained names billing lines show
// (store.ResourceDisplayNames); *store.PGStore has it.
type resourceNameReader interface {
	ResourceDisplayNames(ctx context.Context, tenantID string, refs []store.ResourceDisplayName) (map[string]string, error)
}

// cronMinimumDescription is the invoice line: the cron job, by its retained
// name where one is recorded (a deleted cron job keeps it), and the calendar
// month whose minimum it bills.
func (c *StripeClient) cronMinimumDescription(ctx context.Context, e Event) string {
	service := e.Properties["service_id"]
	if names, ok := c.state.(resourceNameReader); ok {
		ref := store.ResourceDisplayName{Kind: store.ResourceKindService, ID: service}
		if got, err := names.ResourceDisplayNames(ctx, e.CustomerID, []store.ResourceDisplayName{ref}); err == nil {
			if name := got[store.ResourceDisplayNameKey(ref.Kind, ref.ID)]; name != "" {
				service = name
			}
		}
	}
	return fmt.Sprintf("Cron job minimum: %s, %s", service, store.MonthStartUTC(e.Timestamp).Format("January 2006"))
}

// addCronMinimumItem bills one cron minimum row as a pending invoice item on
// the workspace's live Subscription; nil when Stripe accepted it or nothing is
// owed. The emitter has already provisioned the Customer and Subscription.
func (c *StripeClient) addCronMinimumItem(ctx context.Context, e Event) *IngestFailure {
	fail := func(code, message string, permanent bool) *IngestFailure {
		return &IngestFailure{TransactionID: e.TransactionID, Code: code, Message: message, Permanent: permanent}
	}
	microUSD, err := strconv.ParseInt(e.Properties["value"], 10, 64)
	if err != nil {
		return fail("invalid_quantity", "cron minimum quantity is not an integer", true)
	}
	cents := cronMinimumCents(microUSD)
	if cents <= 0 {
		return nil
	}
	customerID, ok := c.lookupCustomer(e.CustomerID)
	if !ok {
		return fail("customer_not_ready", "Stripe customer mapping unavailable", false)
	}
	sub, err := c.liveSubscription(ctx, e.CustomerID, customerID, false)
	if err != nil || sub == nil {
		return fail("subscription_not_ready", "Stripe Subscription unavailable", false)
	}
	params := &stripe.InvoiceItemParams{
		Customer:     stripe.String(customerID),
		Subscription: stripe.String(sub.ID),
		Amount:       stripe.Int64(cents),
		Currency:     stripe.String(string(stripe.CurrencyUSD)),
		Description:  stripe.String(c.cronMinimumDescription(ctx, e)),
		Metadata: map[string]string{
			workspaceMetadataKey:   e.CustomerID,
			cronMinimumMetadataKey: e.TransactionID,
		},
	}
	if sub.AutomaticTax != nil && sub.AutomaticTax.Enabled {
		c.invoiceItemTax(params)
	}
	params.Context = ctx
	params.SetIdempotencyKey("bex-cron-minimum-" + e.TransactionID)
	if _, err := c.sc.InvoiceItems.New(params); err != nil {
		code, message := safeStripeError(err)
		if permanentStripeError(err) {
			c.metrics.Operation("cron_minimum_item", "rejected")
			return fail(code, message, true)
		}
		c.metrics.Operation("cron_minimum_item", "retryable")
		return fail(code, "transient Stripe invoice item failure", false)
	}
	c.metrics.Operation("cron_minimum_item", "success")
	return nil
}
