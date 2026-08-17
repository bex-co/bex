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
	// Credits is the remaining promotional/purchased billing-credit balance
	// (w5/m70): read-only display of Stripe credit grants, absent when the
	// balance is zero or the read degrades. Granting stays operator-side
	// (ADR049); credit never bypasses the ADR046 payment gate.
	Credits *Credits `json:"credits,omitempty"`
}

// Credits is the workspace's available Stripe billing-credit balance plus the
// per-grant remainders that fund it. Values come from Stripe's
// credit_balance_summary/credit_grants APIs — never derived from the invoice
// preview, whose total does not reliably reflect credit under flexible
// billing mode.
type Credits struct {
	AvailableUSD string        `json:"availableUsd"`
	Currency     string        `json:"currency"`
	Grants       []CreditGrant `json:"grants"`
}

// CreditGrant is one active grant's remaining balance. ExpiresAt is empty for
// a grant that never expires.
type CreditGrant struct {
	Name         string `json:"name,omitempty"`
	RemainingUSD string `json:"remainingUsd"`
	ExpiresAt    string `json:"expiresAt,omitempty"`
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
func (c *StripeClient) BillingFor(ctx context.Context, tenantID string, periodStart, periodEnd time.Time) (billing *Billing, err error) {
	defer func() {
		if err != nil {
			c.metrics.Operation("invoice_read", "error")
		} else {
			c.metrics.Operation("invoice_read", "success")
		}
	}()
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
	// Credits degrade independently: a failed credit read (missing key
	// permission, API error) omits the block but never takes the invoice
	// preview or readiness down with it.
	credits, creditErr := c.creditsFor(ctx, customerID)
	if creditErr != nil {
		c.metrics.Operation("credit_read", "error")
		log.Printf("billing: credit read for customer %s: %v (credits omitted)", customerID, creditErr)
		credits = nil
	} else {
		c.metrics.Operation("credit_read", "success")
	}
	return &Billing{CurrentCost: current, Invoices: invoices, Credits: credits}, nil
}

// maxCreditGrants bounds the per-grant remainder reads: each active grant
// costs one extra balance-summary call, and a workspace holds a handful of
// grants at most. Grants beyond the cap are still counted in AvailableUSD
// (which comes from the single aggregate call) — only their per-grant rows
// are dropped.
const maxCreditGrants = 10

// creditsFor reads the customer's available metered-price credit balance and
// the active grants funding it. (nil, nil) means no credit to show — the
// caller omits the block. bex's catalog is USD-only; a non-USD credit is an
// error, matching the invoice-amount contract.
func (c *StripeClient) creditsFor(ctx context.Context, customerID string) (*Credits, error) {
	sum, err := c.sc.BillingCreditBalanceSummary.Get(&stripe.BillingCreditBalanceSummaryParams{
		Params:   stripe.Params{Context: ctx},
		Customer: stripe.String(customerID),
		Filter: &stripe.BillingCreditBalanceSummaryFilterParams{
			Type: stripe.String("applicability_scope"),
			ApplicabilityScope: &stripe.BillingCreditBalanceSummaryFilterApplicabilityScopeParams{
				PriceType: stripe.String("metered"),
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("stripe: credit balance summary for customer %s: %w", customerID, err)
	}
	total, err := sumAvailableCreditBalances(sum.Balances)
	if err != nil {
		return nil, err
	}
	if total <= 0 {
		return nil, nil
	}
	available, err := normalizedStripeAmount(total, stripe.CurrencyUSD)
	if err != nil {
		return nil, err
	}
	grants, err := c.activeGrantRemainders(ctx, customerID)
	if err != nil {
		return nil, err
	}
	return &Credits{AvailableUSD: available.AmountUSD, Currency: available.Currency, Grants: grants}, nil
}

// sumAvailableCreditBalances totals a balance summary's available amounts.
// bex's catalog is USD-only, so a non-USD balance is an error rather than a
// silently mislabeled sum — the same contract normalizedStripeAmount enforces
// for invoice totals.
func sumAvailableCreditBalances(balances []*stripe.BillingCreditBalanceSummaryBalance) (int64, error) {
	var total int64
	for _, b := range balances {
		if b == nil || b.AvailableBalance == nil || b.AvailableBalance.Monetary == nil {
			continue
		}
		if b.AvailableBalance.Monetary.Currency != stripe.CurrencyUSD {
			return 0, fmt.Errorf("unsupported credit currency %q (bex catalog is USD)", b.AvailableBalance.Monetary.Currency)
		}
		total += b.AvailableBalance.Monetary.Value
	}
	return total, nil
}

// activeGrantRemainders lists the customer's credit grants and reads each
// active one's remaining balance (a per-grant balance-summary call). Voided,
// expired, and not-yet-effective grants are skipped, as are grants with
// nothing left.
func (c *StripeClient) activeGrantRemainders(ctx context.Context, customerID string) ([]CreditGrant, error) {
	lp := &stripe.BillingCreditGrantListParams{Customer: stripe.String(customerID)}
	lp.Context = ctx
	lp.Limit = stripe.Int64(25)
	iter := c.sc.BillingCreditGrants.List(lp)
	now := time.Now().Unix()
	out := make([]CreditGrant, 0)
	// examined counts every active grant whose balance we read — not just the
	// ones with a positive remainder — so a pile of depleted grants can't turn
	// the cap into an unbounded per-grant call loop.
	examined := 0
	for iter.Next() {
		g := iter.BillingCreditGrant()
		if g.VoidedAt > 0 || (g.ExpiresAt > 0 && g.ExpiresAt <= now) || g.EffectiveAt > now {
			continue
		}
		if examined >= maxCreditGrants {
			break
		}
		examined++
		sum, err := c.sc.BillingCreditBalanceSummary.Get(&stripe.BillingCreditBalanceSummaryParams{
			Params:   stripe.Params{Context: ctx},
			Customer: stripe.String(customerID),
			Filter: &stripe.BillingCreditBalanceSummaryFilterParams{
				Type:        stripe.String("credit_grant"),
				CreditGrant: stripe.String(g.ID),
			},
		})
		if err != nil {
			return nil, fmt.Errorf("stripe: credit grant %s balance: %w", g.ID, err)
		}
		remaining, err := sumAvailableCreditBalances(sum.Balances)
		if err != nil {
			return nil, fmt.Errorf("stripe: credit grant %s: %w", g.ID, err)
		}
		if remaining <= 0 {
			continue
		}
		amount, err := normalizedStripeAmount(remaining, stripe.CurrencyUSD)
		if err != nil {
			return nil, fmt.Errorf("stripe: credit grant %s: %w", g.ID, err)
		}
		out = append(out, CreditGrant{
			Name:         g.Name,
			RemainingUSD: amount.AmountUSD,
			ExpiresAt:    unixOrFallback(g.ExpiresAt, time.Time{}),
		})
	}
	if err := iter.Err(); err != nil {
		return nil, fmt.Errorf("stripe: list credit grants for customer %s: %w", customerID, err)
	}
	// Earliest-expiring first so the UI's "expires soonest" note is index 0;
	// never-expiring grants sort last.
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i].ExpiresAt, out[j].ExpiresAt
		if (a == "") != (b == "") {
			return a != ""
		}
		return a < b
	})
	return out, nil
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
