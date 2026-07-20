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

	metronome "github.com/Metronome-Industries/metronome-go/v3"
	"github.com/Metronome-Industries/metronome-go/v3/option"
)

// readOpts disables SDK retries on billing reads: they sit on the usage query's
// hot path, so a degraded Metronome must fail fast and let the caller fall back
// to estimate-only rather than stall the response behind a retry backoff. The
// dashboard's 60s poll is the natural retry.
var readOpts = option.WithMaxRetries(0)

// Billing is the real, Metronome-computed cost surfaced beside the advisory
// pricing.yaml estimate (ADR040 Phase 2). It is attached to a usage summary
// only when the workspace has a Metronome contract that actually rates its
// usage; a workspace with no contract (billing_excluded, comped Mode A, or
// billing simply off) has no Billing and its clients show estimate-only.
type Billing struct {
	// CurrentCost is the in-progress current-period spend Metronome has computed
	// so far — the real counterpart to estimatedCost for the open month. Nil
	// when the customer has no rated usage yet.
	CurrentCost *Amount `json:"currentCost,omitempty"`
	// Invoices are the customer's finalized (non-draft) invoices, newest first.
	// Always non-nil for JSON stability (empty slice, not null).
	Invoices []Invoice `json:"invoices"`
}

// Amount is a normalized monetary value: a major-unit string (e.g. "12.34")
// plus a currency label, over a period. Metronome amounts can arrive in cents
// (credit type "USD (cents)"); normalizeAmount folds those to major units.
type Amount struct {
	AmountUSD   string `json:"amountUsd"`
	Currency    string `json:"currency"`
	PeriodStart string `json:"periodStart"` // RFC3339
	PeriodEnd   string `json:"periodEnd"`   // RFC3339
}

// Invoice is one normalized finalized Metronome invoice.
type Invoice struct {
	ID          string `json:"id"`
	Status      string `json:"status"`
	AmountUSD   string `json:"amountUsd"`
	Currency    string `json:"currency"`
	PeriodStart string `json:"periodStart"` // RFC3339
	PeriodEnd   string `json:"periodEnd"`   // RFC3339
}

// BillingFor reads a customer's real billing from Metronome: the current-period
// (in-progress) cost over [periodStart, periodEnd) plus its finalized invoices.
// It returns (nil, nil) when the customer has no rated billing (no contract →
// no cost, no invoices) so callers cleanly fall back to estimate-only. A
// Metronome error is returned as-is; callers log it and fall back rather than
// 500 (ADR040 §Consequences — outages degrade gracefully).
func (c *Client) BillingFor(ctx context.Context, customerID string, periodStart, periodEnd time.Time) (*Billing, error) {
	current, err := c.currentCost(ctx, customerID, periodStart, periodEnd)
	if err != nil {
		return nil, err
	}
	invoices, err := c.finalizedInvoices(ctx, customerID)
	if err != nil {
		return nil, err
	}
	if current == nil && len(invoices) == 0 {
		return nil, nil // no contract / no rated usage → estimate-only
	}
	return &Billing{CurrentCost: current, Invoices: invoices}, nil
}

// currentCost sums the customer's per-window costs over [start, end) into a
// single current-period amount. Returns nil when Metronome reports no cost
// (no contract or genuinely $0 usage) — indistinguishable and both ⇒ nothing
// real to show.
func (c *Client) currentCost(ctx context.Context, customerID string, start, end time.Time) (*Amount, error) {
	page, err := c.mc.V1.Customers.ListCosts(ctx, metronome.V1CustomerListCostsParams{
		CustomerID:   customerID,
		StartingOn:   start,
		EndingBefore: end,
	}, readOpts)
	if err != nil {
		return nil, fmt.Errorf("metronome: list costs %s: %w", customerID, err)
	}
	// Sum each window's cost per credit type, then pick the USD-ish one.
	byCurrency := map[string]float64{}
	for _, window := range page.Data {
		for _, ct := range window.CreditTypes {
			byCurrency[ct.Name] += ct.Cost
		}
	}
	name, total, ok := pickPrimaryCurrency(byCurrency)
	if !ok || total == 0 {
		return nil, nil
	}
	amt := normalizeAmount(total, name)
	amt.PeriodStart = start.UTC().Format(time.RFC3339)
	amt.PeriodEnd = end.UTC().Format(time.RFC3339)
	return &amt, nil
}

// finalizedInvoices lists the customer's non-draft invoices, normalized and
// newest-first. Draft invoices are omitted — only finalized amounts are real.
func (c *Client) finalizedInvoices(ctx context.Context, customerID string) ([]Invoice, error) {
	page, err := c.mc.V1.Customers.Invoices.List(ctx, metronome.V1CustomerInvoiceListParams{
		CustomerID: customerID,
	}, readOpts)
	if err != nil {
		return nil, fmt.Errorf("metronome: list invoices %s: %w", customerID, err)
	}
	out := make([]Invoice, 0, len(page.Data))
	for _, inv := range page.Data {
		if strings.EqualFold(inv.Status, "DRAFT") {
			continue
		}
		amt := normalizeAmount(inv.Total, inv.CreditType.Name)
		out = append(out, Invoice{
			ID:          inv.ID,
			Status:      inv.Status,
			AmountUSD:   amt.AmountUSD,
			Currency:    amt.Currency,
			PeriodStart: rfc3339OrEmpty(inv.StartTimestamp),
			PeriodEnd:   rfc3339OrEmpty(inv.EndTimestamp),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].PeriodStart > out[j].PeriodStart })
	return out, nil
}

// normalizeAmount folds a Metronome amount into a major-unit string. Metronome
// credit types named "… (cents)" carry integer cents, so divide by 100 and
// relabel as USD; any other credit type is passed through with its own name.
func normalizeAmount(total float64, creditTypeName string) Amount {
	if strings.Contains(strings.ToLower(creditTypeName), "cent") {
		return Amount{AmountUSD: fmt.Sprintf("%.2f", total/100), Currency: "USD"}
	}
	cur := creditTypeName
	if cur == "" {
		cur = "USD"
	}
	return Amount{AmountUSD: fmt.Sprintf("%.2f", total), Currency: cur}
}

// pickPrimaryCurrency chooses which credit type is the billable one: prefer a
// USD-named type, else the single one present. Deterministic on ties.
func pickPrimaryCurrency(byCurrency map[string]float64) (name string, total float64, ok bool) {
	if len(byCurrency) == 0 {
		return "", 0, false
	}
	names := make([]string, 0, len(byCurrency))
	for n := range byCurrency {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		if strings.Contains(strings.ToLower(n), "usd") {
			return n, byCurrency[n], true
		}
	}
	return names[0], byCurrency[names[0]], true
}

func rfc3339OrEmpty(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}
