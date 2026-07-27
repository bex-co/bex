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
	"slices"
	"time"

	stripe "github.com/stripe/stripe-go/v86"

	"github.com/bex-co/bex/lego/backend/internal/pricing"
)

const stripeLookupKeyLimit = 10

// EnsureContract attaches every priced meter to one Stripe Subscription for a
// billable workspace. The emitter's store query excludes Mode A workspaces
// before this method is reached, so excluded tenants get neither Customer nor
// Subscription. List-before-create plus Stripe's idempotency key closes process
// restarts and concurrent reconciles.
func (c *StripeClient) EnsureContract(ctx context.Context, tenantID string) error {
	if err := c.EnsureCustomer(ctx, tenantID); err != nil {
		return err
	}
	customerID, ok := c.lookupCustomer(tenantID)
	if !ok {
		return fmt.Errorf("stripe: customer cache missing after ensure for workspace %s", tenantID)
	}
	if _, found, err := c.findSubscription(ctx, tenantID, customerID); err != nil {
		return err
	} else if found {
		return nil
	}

	priceIDs, err := c.resolvePriceIDs(ctx)
	if err != nil {
		return err
	}
	items := make([]*stripe.SubscriptionItemsParams, 0, len(priceIDs))
	for _, priceID := range priceIDs {
		items = append(items, &stripe.SubscriptionItemsParams{Price: stripe.String(priceID)})
	}
	params := &stripe.SubscriptionParams{
		Customer:         stripe.String(customerID),
		CollectionMethod: stripe.String(string(stripe.SubscriptionCollectionMethodChargeAutomatically)),
		Items:            items,
		Metadata: map[string]string{
			workspaceMetadataKey:    tenantID,
			subscriptionMetadataKey: "true",
		},
	}
	params.Context = ctx
	params.SetIdempotencyKey("bex-subscription-" + tenantID)
	if !c.billingEpoch.IsZero() && c.billingEpoch.Before(time.Now()) {
		params.BackdateStartDate = stripe.Int64(c.billingEpoch.Unix())
	}
	sub, err := c.sc.Subscriptions.New(params)
	if err != nil {
		return fmt.Errorf("stripe: create subscription for %s: %w", tenantID, err)
	}
	if sub.ID == "" {
		return fmt.Errorf("stripe: create subscription for %s returned an empty id", tenantID)
	}
	return nil
}

// CompCustomer applies Mode B: keep rated line items and invoice history, but
// apply the setup script's perpetual 100%-off coupon to the workspace's
// subscription. Updating with a deterministic idempotency key makes repeated
// operator invocations safe.
func (c *StripeClient) CompCustomer(ctx context.Context, tenantID string) error {
	if err := c.EnsureContract(ctx, tenantID); err != nil {
		return err
	}
	customerID, ok := c.lookupCustomer(tenantID)
	if !ok {
		return fmt.Errorf("stripe: customer cache missing after ensure for workspace %s", tenantID)
	}
	subscriptionID, found, err := c.findSubscription(ctx, tenantID, customerID)
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("stripe: subscription missing after ensure for workspace %s", tenantID)
	}
	params := &stripe.SubscriptionParams{
		Discounts: []*stripe.SubscriptionDiscountParams{{Coupon: stripe.String(c.compCouponID)}},
		// A comp changes collection policy, not the underlying service period;
		// never manufacture a proration invoice while applying it.
		ProrationBehavior: stripe.String("none"),
	}
	params.Context = ctx
	params.SetIdempotencyKey("bex-comp-" + tenantID)
	if _, err := c.sc.Subscriptions.Update(subscriptionID, params); err != nil {
		return fmt.Errorf("stripe: apply comp coupon %s to %s: %w", c.compCouponID, tenantID, err)
	}
	return nil
}

// findSubscription resolves the one live bex billing contract for a workspace
// without creating it. Multiple matches are rejected instead of silently
// choosing one and risking double-rated usage.
func (c *StripeClient) findSubscription(ctx context.Context, tenantID, customerID string) (string, bool, error) {
	subscription, err := c.findSubscriptionObject(ctx, tenantID, customerID)
	if err != nil {
		return "", false, err
	}
	if subscription == nil {
		return "", false, nil
	}
	return subscription.ID, true, nil
}

// resolvePriceIDs validates the full active Stripe price catalog once per
// process. Stripe accepts at most 10 lookup_keys per list request, so the 13
// current dimensions are fetched in bounded chunks. No subscription is created
// until every expected key has exactly one metered monthly USD price.
func (c *StripeClient) resolvePriceIDs(ctx context.Context) ([]string, error) {
	c.mu.Lock()
	if len(c.priceIDs) > 0 {
		out := slices.Clone(c.priceIDs)
		c.mu.Unlock()
		return out, nil
	}
	c.mu.Unlock()

	lookupKeys := pricing.Default.BillableMeterNames()
	found := make(map[string]string, len(lookupKeys))
	for start := 0; start < len(lookupKeys); start += stripeLookupKeyLimit {
		end := min(start+stripeLookupKeyLimit, len(lookupKeys))
		keys := make([]*string, 0, end-start)
		for _, key := range lookupKeys[start:end] {
			keys = append(keys, stripe.String(key))
		}
		params := &stripe.PriceListParams{
			ListParams: stripe.ListParams{Limit: stripe.Int64(100)},
			Active:     stripe.Bool(true),
			LookupKeys: keys,
		}
		params.Context = ctx
		if c.taxCode != "" {
			params.AddExpand("data.product")
		}
		iter := c.sc.Prices.List(params)
		for iter.Next() {
			price := iter.Price()
			if prior, exists := found[price.LookupKey]; exists && prior != price.ID {
				return nil, fmt.Errorf("stripe: duplicate active price lookup key %s (%s, %s)", price.LookupKey, prior, price.ID)
			}
			if price.Recurring == nil || price.Recurring.UsageType != stripe.PriceRecurringUsageTypeMetered || price.Recurring.Meter == "" || price.Currency != stripe.CurrencyUSD {
				return nil, fmt.Errorf("stripe: price %s (%s) is not a USD metered recurring price with a meter", price.ID, price.LookupKey)
			}
			if c.taxCode != "" {
				if string(price.TaxBehavior) != c.taxBehavior {
					return nil, fmt.Errorf("stripe: price %s (%s) tax_behavior=%q, want %q", price.ID, price.LookupKey, price.TaxBehavior, c.taxBehavior)
				}
				if price.Product == nil || price.Product.TaxCode == nil || price.Product.TaxCode.ID != c.taxCode {
					return nil, fmt.Errorf("stripe: price %s (%s) product is not expanded with tax_code %s", price.ID, price.LookupKey, c.taxCode)
				}
			}
			found[price.LookupKey] = price.ID
		}
		if err := iter.Err(); err != nil {
			return nil, fmt.Errorf("stripe: list prices: %w", err)
		}
	}

	priceIDs := make([]string, 0, len(lookupKeys))
	for _, key := range lookupKeys {
		id, ok := found[key]
		if !ok {
			return nil, fmt.Errorf("stripe: active price with lookup key %s is missing; rerun scripts/stripe-billing-setup.py", key)
		}
		priceIDs = append(priceIDs, id)
	}
	c.mu.Lock()
	if len(c.priceIDs) == 0 {
		c.priceIDs = slices.Clone(priceIDs)
	}
	out := slices.Clone(c.priceIDs)
	c.mu.Unlock()
	return out, nil
}
