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
	"sort"
	"strings"
	"time"

	stripe "github.com/stripe/stripe-go/v86"
)

// Billing is Stripe's real rated cost surfaced beside pricing.yaml's advisory
// estimate. It is present only after the workspace has a bex Subscription.
type Billing struct {
	CurrentCost *Amount   `json:"currentCost,omitempty"`
	Invoices    []Invoice `json:"invoices"`
}

// Amount is a normalized USD major-unit value over Stripe's invoice period.
type Amount struct {
	AmountUSD   string `json:"amountUsd"`
	Currency    string `json:"currency"`
	PeriodStart string `json:"periodStart"`
	PeriodEnd   string `json:"periodEnd"`
}

// Invoice is one normalized non-draft Stripe invoice.
type Invoice struct {
	ID          string `json:"id"`
	Status      string `json:"status"`
	AmountUSD   string `json:"amountUsd"`
	Currency    string `json:"currency"`
	PeriodStart string `json:"periodStart"`
	PeriodEnd   string `json:"periodEnd"`
}

// BillingFor resolves (without creating) the workspace's Customer and bex
// Subscription, then reads Stripe's current invoice preview plus finalized
// history. No Customer/subscription means estimate-only. API failures are
// returned so the usage service can log and gracefully fall back.
func (c *StripeClient) BillingFor(ctx context.Context, tenantID string, periodStart, periodEnd time.Time) (*Billing, error) {
	customerID, found, err := c.findCustomer(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, nil
	}
	subscriptionID, found, err := c.findSubscription(ctx, tenantID, customerID)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, nil
	}
	current, err := c.currentInvoice(ctx, customerID, subscriptionID, periodStart, periodEnd)
	if err != nil {
		return nil, err
	}
	invoices, err := c.finalizedInvoices(ctx, customerID, subscriptionID)
	if err != nil {
		return nil, err
	}
	return &Billing{CurrentCost: current, Invoices: invoices}, nil
}

func (c *StripeClient) currentInvoice(ctx context.Context, customerID, subscriptionID string, fallbackStart, fallbackEnd time.Time) (*Amount, error) {
	params := &stripe.InvoiceCreatePreviewParams{
		Customer:     stripe.String(customerID),
		Subscription: stripe.String(subscriptionID),
	}
	params.Context = ctx
	inv, err := c.sc.Invoices.CreatePreview(params)
	if err != nil {
		return nil, fmt.Errorf("stripe: preview invoice for customer %s: %w", customerID, err)
	}
	amount, err := normalizedStripeAmount(inv.Total, inv.Currency)
	if err != nil {
		return nil, fmt.Errorf("stripe: preview invoice %s: %w", inv.ID, err)
	}
	amount.PeriodStart = unixOrFallback(inv.PeriodStart, fallbackStart)
	amount.PeriodEnd = unixOrFallback(inv.PeriodEnd, fallbackEnd)
	return &amount, nil
}

func (c *StripeClient) finalizedInvoices(ctx context.Context, customerID, subscriptionID string) ([]Invoice, error) {
	params := &stripe.InvoiceListParams{
		ListParams:   stripe.ListParams{Limit: stripe.Int64(100)},
		Customer:     stripe.String(customerID),
		Subscription: stripe.String(subscriptionID),
	}
	params.Context = ctx
	iter := c.sc.Invoices.List(params)
	out := make([]Invoice, 0)
	for iter.Next() {
		inv := iter.Invoice()
		if inv.Status == stripe.InvoiceStatusDraft {
			continue
		}
		amount, err := normalizedStripeAmount(inv.Total, inv.Currency)
		if err != nil {
			return nil, fmt.Errorf("stripe: invoice %s: %w", inv.ID, err)
		}
		out = append(out, Invoice{
			ID:          inv.ID,
			Status:      strings.ToUpper(string(inv.Status)),
			AmountUSD:   amount.AmountUSD,
			Currency:    amount.Currency,
			PeriodStart: unixOrFallback(inv.PeriodStart, time.Time{}),
			PeriodEnd:   unixOrFallback(inv.PeriodEnd, time.Time{}),
		})
	}
	if err := iter.Err(); err != nil {
		return nil, fmt.Errorf("stripe: list invoices for customer %s: %w", customerID, err)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].PeriodStart > out[j].PeriodStart })
	return out, nil
}

func normalizedStripeAmount(total int64, currency stripe.Currency) (Amount, error) {
	if currency != stripe.CurrencyUSD {
		return Amount{}, fmt.Errorf("unsupported invoice currency %q (bex catalog is USD)", currency)
	}
	prefix := ""
	if total < 0 && total > -100 {
		prefix = "-"
	}
	return Amount{AmountUSD: fmt.Sprintf("%s%d.%02d", prefix, total/100, abs(total%100)), Currency: "USD"}, nil
}

func unixOrFallback(unix int64, fallback time.Time) string {
	if unix > 0 {
		return time.Unix(unix, 0).UTC().Format(time.RFC3339)
	}
	if fallback.IsZero() {
		return ""
	}
	return fallback.UTC().Format(time.RFC3339)
}

func abs(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}
