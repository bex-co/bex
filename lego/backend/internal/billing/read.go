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

	"github.com/bex-co/bex/lego/backend/internal/store"
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
	// (ADR071); credit never bypasses the ADR046 payment gate.
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

// Amount is a normalized USD major-unit view of one Stripe invoice period.
//
// The three money figures are deliberately separate. Under ADR071 credit
// grants, Stripe's invoice total is already net of credit consumption, so a
// single field cannot say both what the period cost and what is owed — a
// workspace whose usage a grant fully covered read "$0.00 month to date"
// beside a charge tree summing to $74 (w6/m98):
//
//   - AmountUSD is the gross rated charge for the period: Stripe's invoice
//     subtotal, before invoice-level discounts (ADR040 Mode B comps), before
//     billing credit, and before tax. This is what the charge tree adds up to.
//   - CreditsAppliedUSD is the billing-credit consumption Stripe applied to
//     this invoice. Zero for a workspace with no grant.
//   - AmountDueUSD is what Stripe actually collects, after discounts, credit,
//     and tax.
//
// On the current-period preview these are provisional: Stripe applies credit
// only at finalization, and the credit shown on a preview or draft invoice can
// change before then.
type Amount struct {
	AmountUSD         string `json:"amountUsd"`
	CreditsAppliedUSD string `json:"creditsAppliedUsd"`
	AmountDueUSD      string `json:"amountDueUsd"`
	Currency          string `json:"currency"`
	PeriodStart       string `json:"periodStart"`
	PeriodEnd         string `json:"periodEnd"`
}

// Invoice is one normalized non-draft Stripe invoice. Its money fields carry
// the same gross/credit/due split as Amount, for the same reason.
type Invoice struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	// Kind is InvoiceKindCycle for the subscription's own invoices and
	// InvoiceKindAgentModelCharge for a charge bex made when agent-model spend
	// reached its threshold, which the next cycle invoice deducts.
	Kind              string `json:"kind"`
	AmountUSD         string `json:"amountUsd"`
	CreditsAppliedUSD string `json:"creditsAppliedUsd"`
	AmountDueUSD      string `json:"amountDueUsd"`
	Currency          string `json:"currency"`
	PeriodStart       string `json:"periodStart"`
	PeriodEnd         string `json:"periodEnd"`
}

// The kinds of invoice a workspace's billing history lists.
const (
	InvoiceKindCycle            = "cycle"
	InvoiceKindAgentModelCharge = "agent_model_charge"
)

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
	sub, err := c.liveSubscription(ctx, tenantID, customerID, false)
	if err != nil {
		return nil, err
	}
	if sub == nil {
		return nil, nil
	}
	subscriptionID := sub.ID
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
	readAt := time.Now().UTC()
	_, subPeriodEnd := subscriptionPeriod(sub)
	read, creditErr := c.creditsFor(ctx, customerID, subPeriodEnd)
	if creditErr != nil {
		c.metrics.Operation("credit_read", "error")
		log.Printf("billing: credit read for customer %s: %v (credits omitted)", customerID, creditErr)
	} else {
		c.metrics.Operation("credit_read", "success")
		c.recordAgentModelCredit(ctx, tenantID, read, readAt)
	}
	return &Billing{CurrentCost: current, Invoices: invoices, Credits: read.credits}, nil
}

// AgentModelCreditRecorder records the billing credit agent-model spend may
// draw on (*store.PGStore); a BillingStateStore that implements it gets the
// credit a Billing read saw.
type AgentModelCreditRecorder interface {
	RecordAgentModelCredits(ctx context.Context, workspaceID string, microUSD int64, at time.Time) error
}

// recordAgentModelCredit records, from a Billing read taken at readAt, the
// credit the charger would read before a charge (ADR091 D10.2a): the balance
// less what grants expiring by the period end hold. The mint and the agent
// environment then count it before the first crossing (w6/m400). A read that
// stopped at maxCreditGrants may have missed an expiring grant, so it records
// nothing; a failure leaves the charger to read the credit at the crossing.
func (c *StripeClient) recordAgentModelCredit(ctx context.Context, workspaceID string, read creditRead, readAt time.Time) {
	recorder, ok := c.state.(AgentModelCreditRecorder)
	if !ok || !read.complete {
		return
	}
	micro := max(read.cents-read.expiringCents, 0) * microUSDPerCent
	if err := recorder.RecordAgentModelCredits(ctx, workspaceID, micro, readAt); err != nil {
		log.Printf("billing: record the agent-model credit of %s from a Billing read: %v", workspaceID, err)
	}
}

// creditRead is one read of a customer's metered billing credit: the Billing
// page's block (nil when there is none), the available balance and what the
// metered grants expiring by the period end hold, in cents, and whether every
// active grant was read.
type creditRead struct {
	credits       *Credits
	cents         int64
	expiringCents int64
	complete      bool
}

// maxCreditGrants bounds the per-grant remainder reads: each active grant
// costs one extra balance-summary call, and a workspace holds a handful of
// grants at most. Grants beyond the cap are still counted in AvailableUSD
// (which comes from the single aggregate call) — only their per-grant rows
// are dropped.
const maxCreditGrants = 10

// creditsFor reads the customer's available metered-price credit balance and
// the active grants funding it, noting what those expiring by periodEnd hold.
// A nil block means no credit to show — the caller omits it. bex's catalog is
// USD-only; a non-USD credit is an error, matching the invoice-amount contract.
func (c *StripeClient) creditsFor(ctx context.Context, customerID string, periodEnd int64) (creditRead, error) {
	total, err := c.meteredCreditCents(ctx, customerID)
	if err != nil {
		return creditRead{}, err
	}
	if total <= 0 {
		return creditRead{complete: true}, nil
	}
	available, err := normalizedStripeUSD(total, stripe.CurrencyUSD)
	if err != nil {
		return creditRead{}, err
	}
	grants, expiring, complete, err := c.activeGrantRemainders(ctx, customerID, periodEnd)
	if err != nil {
		return creditRead{}, err
	}
	return creditRead{credits: &Credits{AvailableUSD: available, Currency: "USD", Grants: grants},
		cents: total, expiringCents: expiring, complete: complete}, nil
}

// meteredCreditCents is the customer's available billing-credit balance on
// metered prices (ADR071's grant scope), in cents. Credits a draft invoice
// already holds are not available.
func (c *StripeClient) meteredCreditCents(ctx context.Context, customerID string) (int64, error) {
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
		return 0, fmt.Errorf("stripe: credit balance summary for customer %s: %w", customerID, err)
	}
	return sumAvailableCreditBalances(sum.Balances)
}

// grantCreditCents is one credit grant's available balance, in cents.
func (c *StripeClient) grantCreditCents(ctx context.Context, customerID, grantID string) (int64, error) {
	sum, err := c.sc.BillingCreditBalanceSummary.Get(&stripe.BillingCreditBalanceSummaryParams{
		Params:   stripe.Params{Context: ctx},
		Customer: stripe.String(customerID),
		Filter: &stripe.BillingCreditBalanceSummaryFilterParams{
			Type:        stripe.String("credit_grant"),
			CreditGrant: stripe.String(grantID),
		},
	})
	if err != nil {
		return 0, fmt.Errorf("stripe: credit grant %s balance: %w", grantID, err)
	}
	remaining, err := sumAvailableCreditBalances(sum.Balances)
	if err != nil {
		return 0, fmt.Errorf("stripe: credit grant %s: %w", grantID, err)
	}
	return remaining, nil
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

// creditGrantActive reports a grant that is in effect at now: not voided,
// not expired, and already effective.
func creditGrantActive(g *stripe.BillingCreditGrant, now int64) bool {
	return g.VoidedAt == 0 && (g.ExpiresAt == 0 || g.ExpiresAt > now) && g.EffectiveAt <= now
}

// activeGrantRemainders lists the customer's credit grants and reads each
// active one's remaining balance (a per-grant balance-summary call). Voided,
// expired, and not-yet-effective grants are skipped, as are grants with
// nothing left.
func (c *StripeClient) activeGrantRemainders(ctx context.Context, customerID string, periodEnd int64) (out []CreditGrant, expiringCents int64, complete bool, err error) {
	lp := &stripe.BillingCreditGrantListParams{Customer: stripe.String(customerID)}
	lp.Context = ctx
	lp.Limit = stripe.Int64(25)
	iter := c.sc.BillingCreditGrants.List(lp)
	now := time.Now().Unix()
	out = make([]CreditGrant, 0)
	complete = true
	// examined counts every active grant whose balance we read — not just the
	// ones with a positive remainder — so a pile of depleted grants can't turn
	// the cap into an unbounded per-grant call loop.
	examined := 0
	for iter.Next() {
		g := iter.BillingCreditGrant()
		if !creditGrantActive(g, now) {
			continue
		}
		if examined >= maxCreditGrants {
			complete = false
			break
		}
		examined++
		remaining, err := c.grantCreditCents(ctx, customerID, g.ID)
		if err != nil {
			return nil, 0, false, err
		}
		if remaining <= 0 {
			continue
		}
		if expiresByPeriodEnd(g, periodEnd) {
			expiringCents += remaining
		}
		amount, err := normalizedStripeUSD(remaining, stripe.CurrencyUSD)
		if err != nil {
			return nil, 0, false, fmt.Errorf("stripe: credit grant %s: %w", g.ID, err)
		}
		out = append(out, CreditGrant{
			Name:         g.Name,
			RemainingUSD: amount,
			ExpiresAt:    unixOrFallback(g.ExpiresAt, time.Time{}),
		})
	}
	if err := iter.Err(); err != nil {
		return nil, 0, false, fmt.Errorf("stripe: list credit grants for customer %s: %w", customerID, err)
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
	return out, expiringCents, complete, nil
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
	gross, creditsApplied, due, err := invoiceFigures(inv)
	if err != nil {
		return nil, fmt.Errorf("stripe: preview invoice %s: %w", inv.ID, err)
	}
	return &Amount{
		AmountUSD:         gross,
		CreditsAppliedUSD: creditsApplied,
		AmountDueUSD:      due,
		Currency:          "USD",
		PeriodStart:       unixOrFallback(inv.PeriodStart, fallbackStart),
		PeriodEnd:         unixOrFallback(inv.PeriodEnd, fallbackEnd),
	}, nil
}

// PaidInvoices lists customerID's paid bex invoices with a positive amount
// paid, the history Trust.Backfill counts.
func (c *StripeClient) PaidInvoices(ctx context.Context, customerID string) ([]store.BillingTrustPayment, error) {
	params := &stripe.InvoiceListParams{
		ListParams: stripe.ListParams{Limit: stripe.Int64(100)},
		Customer:   stripe.String(customerID),
		Status:     stripe.String(string(stripe.InvoiceStatusPaid)),
	}
	params.Context = ctx
	iter := c.sc.Invoices.List(params)
	var out []store.BillingTrustPayment
	for iter.Next() {
		if p, ok := paidInvoicePayment(iter.Invoice()); ok {
			out = append(out, p)
		}
	}
	if err := iter.Err(); err != nil {
		return nil, fmt.Errorf("stripe: list paid invoices for customer %s: %w", customerID, err)
	}
	return out, nil
}

func (c *StripeClient) finalizedInvoices(ctx context.Context, customerID, subscriptionID string) ([]Invoice, error) {
	cycle, err := c.listInvoices(ctx, customerID, subscriptionID, false)
	if err != nil {
		return nil, err
	}
	// A threshold charge is a standalone invoice on the customer (it never
	// names the subscription, so it moves neither the billing anchor nor
	// proration); it belongs in the history beside the cycle invoices that
	// deduct it.
	charges, err := c.listInvoices(ctx, customerID, "", true)
	if err != nil {
		return nil, err
	}
	out := append(cycle, charges...)
	sort.SliceStable(out, func(i, j int) bool { return out[i].PeriodStart > out[j].PeriodStart })
	return out, nil
}

// listInvoices is customerID's finalized invoices (on subscriptionID when it
// is set): its agent-model charge invoices when charges, else every other one.
func (c *StripeClient) listInvoices(ctx context.Context, customerID, subscriptionID string, charges bool) ([]Invoice, error) {
	kind := InvoiceKindCycle
	if charges {
		kind = InvoiceKindAgentModelCharge
	}
	params := &stripe.InvoiceListParams{
		ListParams: stripe.ListParams{Limit: stripe.Int64(100)},
		Customer:   stripe.String(customerID),
	}
	if subscriptionID != "" {
		params.Subscription = stripe.String(subscriptionID)
	}
	params.Context = ctx
	iter := c.sc.Invoices.List(params)
	out := make([]Invoice, 0)
	for iter.Next() {
		inv := iter.Invoice()
		_, isCharge := agentModelChargeInvoice(inv)
		if inv.Status == stripe.InvoiceStatusDraft || isCharge != charges {
			continue
		}
		gross, creditsApplied, due, err := invoiceFigures(inv)
		if err != nil {
			return nil, fmt.Errorf("stripe: invoice %s: %w", inv.ID, err)
		}
		out = append(out, Invoice{
			ID:                inv.ID,
			Status:            strings.ToUpper(string(inv.Status)),
			Kind:              kind,
			AmountUSD:         gross,
			CreditsAppliedUSD: creditsApplied,
			AmountDueUSD:      due,
			Currency:          "USD",
			PeriodStart:       unixOrFallback(inv.PeriodStart, time.Time{}),
			PeriodEnd:         unixOrFallback(inv.PeriodEnd, time.Time{}),
		})
	}
	if err := iter.Err(); err != nil {
		return nil, fmt.Errorf("stripe: list invoices for customer %s: %w", customerID, err)
	}
	return out, nil
}

// invoiceFigures splits one Stripe invoice into the three figures the billing
// page needs, all in major-unit USD strings.
//
// The gross charge is the invoice subtotal, not its total: Stripe applies
// billing credit after discounts and before tax, so it never reaches the
// subtotal, while the total is already net of it. Reading the total as "the
// current cost" is what made a fully-credited period render as $0.00 (w6/m98).
// The credit that did the netting is reported beside it rather than folded in,
// and the due figure is Stripe's own amount_due — no arithmetic of ours sits
// between Stripe's rating and what the workspace is told it owes.
func invoiceFigures(inv *stripe.Invoice) (gross, creditsApplied, due string, err error) {
	if inv.Currency != stripe.CurrencyUSD {
		return "", "", "", fmt.Errorf("unsupported invoice currency %q (bex catalog is USD)", inv.Currency)
	}
	return usdString(inv.Subtotal),
		usdString(appliedCreditGrants(inv.TotalPretaxCreditAmounts)),
		usdString(inv.AmountDue),
		nil
}

// appliedCreditGrants totals the billing-credit consumption on an invoice.
// Stripe's pretax credit list also carries discount entries (ADR040 Mode B
// comps land there); only credit-balance-transaction entries are grant
// consumption, and folding a comp discount in would misreport it as credit.
func appliedCreditGrants(amounts []*stripe.InvoiceTotalPretaxCreditAmount) int64 {
	var total int64
	for _, a := range amounts {
		if a == nil || a.Type != stripe.InvoiceTotalPretaxCreditAmountTypeCreditBalanceTransaction {
			continue
		}
		total += a.Amount
	}
	return total
}

// normalizedStripeUSD renders Stripe minor units as a major-unit string. bex's
// catalog is USD-only, so any other currency is an error rather than a
// silently mislabeled number.
func normalizedStripeUSD(minor int64, currency stripe.Currency) (string, error) {
	if currency != stripe.CurrencyUSD {
		return "", fmt.Errorf("unsupported invoice currency %q (bex catalog is USD)", currency)
	}
	return usdString(minor), nil
}

// usdString renders minor units as major units. A sub-dollar negative keeps
// its sign, which integer division alone would drop.
func usdString(minor int64) string {
	prefix := ""
	if minor < 0 && minor > -100 {
		prefix = "-"
	}
	return fmt.Sprintf("%s%d.%02d", prefix, minor/100, abs(minor%100))
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
