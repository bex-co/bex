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
	"errors"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"time"

	stripe "github.com/stripe/stripe-go/v86"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// Settle before delete (ADR091 D10.4, ADR086). Before a workspace is torn
// down, its unbilled usage is exported to Stripe past the seal horizon, and
// the workspace's balance is charged synchronously:
//
//  1. Contract. No Customer, a Customer already retired (bex_deleted_at), or
//     no live Subscription means an earlier attempt already got past the
//     Stripe purger, or nothing was ever billable: the workspace is treated
//     as settled and nothing is exported or charged.
//  2. Export. Every pending row the settle seal admits is sent now.
//  3. Charge. An open or uncollectible invoice of the contract (a retry after
//     a declined settle) is paid as it is, without invoicing again. Otherwise
//     bex waits until Stripe's meter event summaries cover everything bex
//     has exported for the period (BILLING_SETTLING, retryable, if they do
//     not within the settle wait; see below), previews the upcoming invoice, and only when it
//     is above zero resets the billing cycle anchor to now with
//     proration_behavior=always_invoice and payment_behavior=
//     default_incomplete: Stripe invoices the shortened period's metered
//     usage without attempting payment, and bex then pays it exactly once,
//     off-session.
//
// A decline refuses the deletion with BILLING_BALANCE_DUE and the invoice's
// hosted payment link. Nothing has been torn down and the Subscription is
// live, so the workspace stays intact, and the open invoice keeps the
// failed-invoice refusal of model spend (D10.3) in force through its
// invoice.payment_failed webhook. Cancelling with invoice_now instead of the
// anchor reset would bill the same usage, but only after the Subscription is
// gone, which is the wrong order when the charge can fail. The workspace
// purger still cancels the Subscription with invoice_now afterwards, which
// bills whatever reached Stripe in between.
//
// Waiting for the meter summaries. A Stripe test-mode probe (2026-11-12)
// measured 55 s and 62 s before the summaries reflected exported usage, longer
// than any request can wait inside bex-api's 30 s HTTP write timeout. So an
// API request waits at most DefaultSettleWait (20 s, and never past its own
// deadline less the charge reserve) and answers the retryable
// BILLING_SETTLING; the dashboard keeps retrying the deletion for a few
// minutes. A caller not bound by a request (the account-deletion worker's
// teardown) passes a longer wait with core.WithSettleWait.
//
// The settle's own past_due. With payment_behavior=default_incomplete Stripe
// leaves the Subscription past_due from the anchor reset until bex pays the
// new invoice (billing_reason subscription_update, attempt_count 0), and
// reports it through customer.subscription.updated and the reconciler's
// snapshot. That is not a payment failure, so before the reset the settle
// records a short-lived mark (store.MarkBillingSettling, settleMarkTTL ahead),
// and the lifecycle store ignores a subscription_past_due observed before it.
// A store mark, not an inspection of the latest invoice, because the webhook
// path sees only the Subscription and has no Stripe client to look the invoice
// up, and the mark is written before Stripe can emit the event. Only past_due
// is excused: a declined settle charge, like any failed invoice, still arrives
// as invoice.payment_failed and opens the grace period, and a settle that
// never pays shows past_due again once the mark expires.

// SettleUsageStore is the per-workspace outbox read plus the exported totals
// the meter summaries are checked against; *store.PGStore satisfies it.
type SettleUsageStore interface {
	SelectWorkspaceUnemittedUsage(ctx context.Context, workspaceID string, floor time.Time, seal store.UsageSeal, limit int, requirePaymentMethod bool) ([]store.HourlyRow, error)
	ExportedUsageTotals(ctx context.Context, workspaceID string, from, to, recentSince time.Time) ([]store.ExportedUsageTotal, error)
}

// SettleContract is the live billing contract a settle charges, and the
// minute-aligned window [From, To) of its current period that Stripe's meter
// summaries are read over. From is zero when the period is unknown.
type SettleContract struct {
	WorkspaceID    string
	CustomerID     string
	SubscriptionID string
	From, To       time.Time
}

// SettleInvoicer is the Stripe side of a settle; *StripeClient satisfies it.
type SettleInvoicer interface {
	SettleContract(ctx context.Context, workspaceID string) (SettleContract, bool, error)
	AwaitMeteredUsage(ctx context.Context, contract SettleContract, totals []store.ExportedUsageTotal) error
	ChargeBalance(ctx context.Context, contract SettleContract, awaitUsage func(context.Context) error) error
	VoidAgentModelChargesBilledBy(ctx context.Context, workspaceID, customerID string, until time.Time) error
}

// Settler runs the settle step. It satisfies the workspaces feature's
// WorkspaceSettler seam structurally. Built only when Stripe is enabled, so
// with billing off the deletion path is unchanged.
type Settler struct {
	Emitter  *Emitter
	Usage    SettleUsageStore
	Invoices SettleInvoicer
}

// settleRecentExport is how far back an export can still be aggregating at
// Stripe. A meter summary short by more than what was exported within it is
// not lag (the event was dropped, or billed to an earlier Customer), so it
// never blocks a deletion.
const settleRecentExport = time.Hour

// SettleWorkspace settles the workspace's balance (see the file comment). A
// declined charge returns core's BILLING_BALANCE_DUE refusal and lagging meter
// aggregation BILLING_SETTLING; any other failure is an error that leaves the
// deletion retryable. Idempotent: a retry after the invoice was paid exports
// nothing new, and with nothing due it invoices nothing.
func (s *Settler) SettleWorkspace(ctx context.Context, workspaceID string) error {
	if s == nil || s.Invoices == nil || core.SettledBalance(ctx) {
		return nil
	}
	contract, live, err := s.Invoices.SettleContract(ctx, workspaceID)
	if err != nil {
		return fmt.Errorf("settle workspace %s: %w", workspaceID, err)
	}
	if !live {
		return nil
	}
	if s.Emitter != nil && s.Usage != nil {
		if err := s.Emitter.ExportWorkspace(ctx, s.Usage, workspaceID); err != nil {
			return fmt.Errorf("settle workspace %s: %w", workspaceID, err)
		}
	}
	settledAt := time.Now()
	if err := s.Invoices.ChargeBalance(ctx, contract, func(ctx context.Context) error {
		if s.Usage == nil || contract.From.IsZero() {
			return nil
		}
		totals, err := s.Usage.ExportedUsageTotals(ctx, workspaceID, contract.From, contract.To, contract.To.Add(-settleRecentExport))
		if err != nil {
			return fmt.Errorf("settle workspace %s: read exported usage: %w", workspaceID, err)
		}
		return s.Invoices.AwaitMeteredUsage(ctx, contract, totals)
	}); err != nil {
		return err
	}
	// The settle invoice billed every agent-model hour, a declined threshold
	// charge's too: void that charge's open invoice before the workspace (and
	// the webhook's way back to it) is gone. A failure leaves the deletion
	// retryable; the retry finds nothing more to charge.
	if err := s.Invoices.VoidAgentModelChargesBilledBy(ctx, workspaceID, contract.CustomerID, settledAt); err != nil {
		return fmt.Errorf("settle workspace %s: %w", workspaceID, err)
	}
	return nil
}

// settleSeal is the settle export's finality horizon at now: every hour that
// has closed, plus, for the fast kinds, the open hour too. A fast kind's
// pending row grows only until its first export attempt and then spills into
// a later hour (store.RecordAgentModelUsage), so exporting it early loses
// nothing if the deletion is refused. A rollup-written kind is only written
// for closed hours; the open hour of an additive kind (sandbox compute) is
// left alone, because a refused deletion would keep accruing into a row
// already exported.
func (e *Emitter) settleSeal(now time.Time) store.UsageSeal {
	seal := store.UsageSeal{Before: now.Truncate(time.Hour), FastKinds: e.activeFastKinds(), HoldKinds: e.HoldKinds}
	if len(seal.FastKinds) > 0 {
		seal.FastBefore = now.Add(time.Second)
	}
	return seal
}

// ExportWorkspace ships every pending usage row of one workspace that
// settleSeal admits, bypassing the seal horizon but keeping the floor, the
// held kinds and the payment-method withholding. The caller has resolved the
// workspace's Customer and live Subscription, so nothing is provisioned here.
// It returns an error unless every selected row was accepted or permanently
// rejected, so a deletion never proceeds past usage still on its way.
func (e *Emitter) ExportWorkspace(ctx context.Context, usage SettleUsageStore, workspaceID string) error {
	clock := e.now
	if clock == nil {
		clock = time.Now
	}
	now := clock().UTC()
	seal, floor := e.settleSeal(now), e.floor(now)
	limit := e.BatchLimit
	if limit <= 0 {
		limit = DefaultBatchLimit
	}
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		rows, err := usage.SelectWorkspaceUnemittedUsage(ctx, workspaceID, floor, seal, limit, e.RequirePaymentMethod)
		if err != nil {
			return fmt.Errorf("select pending usage: %w", err)
		}
		if len(rows) == 0 {
			return nil
		}
		if done := e.emitRows(ctx, rows, now, map[string]bool{workspaceID: true}); done < len(rows) {
			e.Metrics.Operation("settle_export", "error")
			return fmt.Errorf("exported %d of %d pending usage rows; retry the deletion", done, len(rows))
		}
		e.Metrics.Operation("settle_export", "success")
		if len(rows) < limit {
			return nil
		}
	}
}

// SettleContract resolves the workspace's live contract without creating
// anything. live is false when there is nothing to settle: no Customer, a
// Customer retired by an earlier attempt's Stripe purger (bex_deleted_at), or
// no live Subscription.
func (c *StripeClient) SettleContract(ctx context.Context, workspaceID string) (SettleContract, bool, error) {
	customerID, found, err := c.findCustomer(ctx, workspaceID)
	if err != nil || !found {
		return SettleContract{}, false, err
	}
	sub, err := c.liveSubscription(ctx, workspaceID, customerID, false)
	if err != nil || sub == nil {
		return SettleContract{}, false, err
	}
	customer, gone, err := c.liveCustomer(ctx, customerID)
	if err != nil {
		return SettleContract{}, false, err
	}
	if gone || customer.Metadata[deletedAtMetadataKey] != "" {
		return SettleContract{}, false, nil
	}
	contract := SettleContract{WorkspaceID: workspaceID, CustomerID: customerID, SubscriptionID: sub.ID,
		To: time.Now().UTC().Truncate(time.Minute).Add(time.Minute)}
	if start, _ := subscriptionPeriod(sub); start > 0 {
		contract.From = time.Unix(start, 0).UTC().Truncate(time.Minute)
	}
	return contract, true, nil
}

// meterTarget is what bex exported to one Stripe meter over the window, and
// how much of it was exported within settleRecentExport.
type meterTarget struct {
	eventName    string
	sent, recent float64
}

// meterTargets converts bex's exported totals to Stripe meter units exactly
// as the export does (stripeMeterEvent) and groups them by meter id.
func (c *StripeClient) meterTargets(ctx context.Context, totals []store.ExportedUsageTotal) (map[string]*meterTarget, error) {
	if len(totals) == 0 {
		return nil, nil
	}
	if _, err := c.resolvePriceIDs(ctx); err != nil {
		return nil, err
	}
	targets := map[string]*meterTarget{}
	for _, total := range totals {
		row := store.HourlyRow{Kind: total.Kind, Tier: total.Tier, ResourceKind: total.ResourceKind}
		row.Quantity = total.Quantity
		name, sent, skip := stripeMeterEvent(toEvent(row))
		if skip {
			continue
		}
		row.Quantity = total.Recent
		_, recent, _ := stripeMeterEvent(toEvent(row))
		c.mu.Lock()
		meter := c.priceMeters[c.priceByKey[name]]
		c.mu.Unlock()
		if meter == "" {
			log.Printf("billing: settle: no catalog meter for event %s; its aggregation is not awaited", name)
			continue
		}
		target := targets[meter]
		if target == nil {
			target = &meterTarget{eventName: name}
			targets[meter] = target
		}
		target.sent += decimalValue(sent)
		target.recent += decimalValue(recent)
	}
	return targets, nil
}

func decimalValue(s string) float64 {
	r, ok := new(big.Rat).SetString(s)
	if !ok {
		return 0
	}
	f, _ := r.Float64()
	return f
}

// DefaultSettleWait bounds how long a deletion waits for Stripe to aggregate
// the usage it exported. It fits an API request inside bex-api's 30 s HTTP
// write timeout; Stripe's summaries can take a minute (file comment), and the
// caller retries BILLING_SETTLING.
const DefaultSettleWait = 20 * time.Second

// settleMarkTTL is how long a settle's mark excuses its Subscription's
// past_due: the reset and the one charge take seconds, and a webhook for it
// can arrive minutes later.
const settleMarkTTL = 10 * time.Minute

// settleChargeReserve is the time kept, before a caller's deadline, for the
// preview, the anchor reset and the charge that follow the wait.
const settleChargeReserve = 5 * time.Second

// AwaitMeteredUsage polls each meter's event summary for the Customer over the
// contract window, with backoff, until it covers what bex exported there. A
// meter still short after the settle wait (DefaultSettleWait, or
// core.WithSettleWait's; never past ctx's deadline less settleChargeReserve)
// refuses with the retryable BILLING_SETTLING; the Subscription is not touched. A shortfall larger than
// the recent exports cannot be aggregation lag, so it is logged and counted
// (settle_meter_mismatch) instead of blocking the deletion forever.
func (c *StripeClient) AwaitMeteredUsage(ctx context.Context, contract SettleContract, totals []store.ExportedUsageTotal) error {
	targets, err := c.meterTargets(ctx, totals)
	if err != nil {
		return fmt.Errorf("stripe: resolve meters for settle of %s: %w", contract.WorkspaceID, err)
	}
	wait := c.settleWait
	if wait <= 0 {
		wait = DefaultSettleWait
	}
	if longer, ok := core.SettleWait(ctx); ok {
		wait = longer
	}
	deadline := time.Now().Add(wait)
	// An API request's own deadline (GraphQL execution, HTTP write) is
	// usually nearer than the wait: refuse with BILLING_SETTLING while there
	// is still time to say so, and to preview, invoice and charge.
	if until, ok := ctx.Deadline(); ok && until.Add(-settleChargeReserve).Before(deadline) {
		deadline = until.Add(-settleChargeReserve)
	}
	delay := min(500*time.Millisecond, wait/20)
	for {
		for meter, target := range targets {
			got, err := c.meterSummary(ctx, meter, contract)
			if err != nil {
				return err
			}
			gap := target.sent - got
			if gap <= 1e-6+1e-9*target.sent {
				delete(targets, meter)
				continue
			}
			if gap > target.recent+1e-6+1e-9*target.sent {
				c.metrics.Operation("settle_meter_mismatch", target.eventName)
				log.Printf("billing: settle of %s: Stripe meter %s summary is short by more than the last hour's exports; not awaited", contract.WorkspaceID, target.eventName)
				delete(targets, meter)
			}
		}
		if len(targets) == 0 {
			c.metrics.Operation("settle_meter_summary", "caught_up")
			return nil
		}
		if time.Now().Add(delay).After(deadline) {
			c.metrics.Operation("settle_meter_summary", "settling")
			log.Printf("billing: settle of %s: %d Stripe meter(s) still aggregating after %s; refusing as BILLING_SETTLING", contract.WorkspaceID, len(targets), wait)
			return core.NewBillingSettlingError(contract.WorkspaceID)
		}
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
		delay = min(delay*2, 8*time.Second)
	}
}

// meterSummary is the Customer's aggregated value of one meter over the
// contract window: one summary, since no grouping window is asked for.
func (c *StripeClient) meterSummary(ctx context.Context, meterID string, contract SettleContract) (float64, error) {
	params := &stripe.BillingMeterEventSummaryListParams{
		ID:        stripe.String(meterID),
		Customer:  stripe.String(contract.CustomerID),
		StartTime: stripe.Int64(contract.From.Unix()),
		EndTime:   stripe.Int64(contract.To.Unix()),
	}
	params.Context = ctx
	params.Limit = stripe.Int64(100)
	iter := c.sc.BillingMeterEventSummaries.List(params)
	var total float64
	for iter.Next() {
		total += iter.BillingMeterEventSummary().AggregatedValue
	}
	if err := iter.Err(); err != nil {
		code, _ := safeStripeError(err)
		return 0, fmt.Errorf("stripe: read meter summary for settle of %s: %s", contract.WorkspaceID, code)
	}
	return total, nil
}

// settleInvoiceCap bounds the unpaid invoices one settle reads per status.
const settleInvoiceCap = 300

// ChargeBalance charges the contract's balance: an existing unpaid invoice as
// it is, else (after awaitUsage) the shortened period's usage, invoiced only
// when the preview shows something due. Each invoice is paid once.
func (c *StripeClient) ChargeBalance(ctx context.Context, contract SettleContract, awaitUsage func(context.Context) error) error {
	unpaid, err := c.unpaidContractInvoices(ctx, contract)
	if err != nil {
		return err
	}
	if len(unpaid) == 0 {
		if awaitUsage != nil {
			if err := awaitUsage(ctx); err != nil {
				return err
			}
		}
		invoice, err := c.invoiceUsageNow(ctx, contract)
		if err != nil {
			return err
		}
		if invoice != nil {
			unpaid = append(unpaid, invoice)
		}
	}
	return c.chargeInvoices(ctx, contract.WorkspaceID, unpaid)
}

// unpaidInvoiceStatuses are the invoice states that still owe money: open
// (including a failed charge) and uncollectible (written off, still owed).
var unpaidInvoiceStatuses = []stripe.InvoiceStatus{stripe.InvoiceStatusOpen, stripe.InvoiceStatusUncollectible}

// unpaidContractInvoices lists the Subscription's invoices that still owe.
func (c *StripeClient) unpaidContractInvoices(ctx context.Context, contract SettleContract) ([]*stripe.Invoice, error) {
	var out []*stripe.Invoice
	for _, status := range unpaidInvoiceStatuses {
		params := &stripe.InvoiceListParams{Subscription: stripe.String(contract.SubscriptionID), Status: stripe.String(string(status))}
		params.Context = ctx
		params.Limit = stripe.Int64(100)
		iter := c.sc.Invoices.List(params)
		for seen := 0; seen < settleInvoiceCap && iter.Next(); seen++ {
			invoice := iter.Invoice()
			if ws, _, ok := invoiceContract(invoice); ok && ws == contract.WorkspaceID && invoice.AmountRemaining > 0 {
				out = append(out, invoice)
			}
		}
		if err := iter.Err(); err != nil {
			code, _ := safeStripeError(err)
			return nil, fmt.Errorf("stripe: list %s invoices of %s: %s", status, contract.WorkspaceID, code)
		}
	}
	return out, nil
}

// invoiceUsageNow invoices the current period's usage now, when the preview
// shows anything due: the billing cycle anchor is reset with
// proration_behavior=always_invoice and payment_behavior=default_incomplete,
// so Stripe creates the invoice without attempting payment. A draft is
// finalized (auto_advance untouched, so Stripe does not charge it on its own).
// It returns the invoice to charge, or nil when nothing is due.
func (c *StripeClient) invoiceUsageNow(ctx context.Context, contract SettleContract) (*stripe.Invoice, error) {
	preview := &stripe.InvoiceCreatePreviewParams{Subscription: stripe.String(contract.SubscriptionID)}
	preview.Context = ctx
	upcoming, err := c.sc.Invoices.CreatePreview(preview)
	if err != nil {
		c.metrics.Operation("settle_invoice", "error")
		code, _ := safeStripeError(err)
		return nil, fmt.Errorf("stripe: preview settle invoice of %s: %s", contract.WorkspaceID, code)
	}
	if upcoming.AmountDue <= 0 {
		c.metrics.Operation("settle_invoice", "nothing_due")
		return nil, nil
	}
	// Mark first: Stripe reports the past_due this reset causes as soon as
	// the update returns (see the file comment).
	if c.state != nil {
		if err := c.state.MarkBillingSettling(ctx, contract.WorkspaceID, time.Now().Add(settleMarkTTL)); err != nil {
			return nil, fmt.Errorf("mark settle of %s: %w", contract.WorkspaceID, err)
		}
	}
	params := &stripe.SubscriptionParams{
		BillingCycleAnchorNow: stripe.Bool(true),
		ProrationBehavior:     stripe.String("always_invoice"),
		PaymentBehavior:       stripe.String("default_incomplete"),
	}
	params.Context = ctx
	params.AddExpand("latest_invoice")
	sub, err := c.sc.Subscriptions.Update(contract.SubscriptionID, params)
	if err != nil {
		c.metrics.Operation("settle_invoice", "error")
		code, _ := safeStripeError(err)
		return nil, fmt.Errorf("stripe: invoice usage of %s now: %s", contract.WorkspaceID, code)
	}
	c.metrics.Operation("settle_invoice", "success")
	invoice := sub.LatestInvoice
	if invoice == nil || invoice.ID == "" {
		return nil, nil
	}
	if invoice.Status == stripe.InvoiceStatusDraft {
		if invoice, err = c.finalizeDraftInvoice(ctx, invoice.ID, "bex-finalize-settle-"+invoice.ID, nil); err != nil || invoice == nil {
			return nil, err
		}
	}
	if invoice.Status != stripe.InvoiceStatusOpen || invoice.AmountRemaining <= 0 {
		return nil, nil
	}
	return invoice, nil
}

// finalizeDraftInvoice finalizes a draft invoice under idempotencyKey and
// returns it; autoAdvance nil leaves the invoice's setting alone. An invoice
// already finalized (by Stripe, or by a redelivery after a lost response) is
// returned as it is; a missing one is nil.
func (c *StripeClient) finalizeDraftInvoice(ctx context.Context, invoiceID, idempotencyKey string, autoAdvance *bool) (*stripe.Invoice, error) {
	params := &stripe.InvoiceFinalizeInvoiceParams{AutoAdvance: autoAdvance}
	params.Context = ctx
	params.SetIdempotencyKey(idempotencyKey)
	invoice, err := c.sc.Invoices.FinalizeInvoice(invoiceID, params)
	if err == nil {
		return invoice, nil
	}
	if resourceMissing(err) {
		return nil, nil
	}
	getParams := &stripe.InvoiceParams{}
	getParams.Context = ctx
	if current, gerr := c.sc.Invoices.Get(invoiceID, getParams); gerr == nil && current.Status != stripe.InvoiceStatusDraft {
		return current, nil
	}
	code, _ := safeStripeError(err)
	return nil, fmt.Errorf("stripe: finalize invoice %s: %s", invoiceID, code)
}

// chargeInvoices pays each invoice once with the default payment method,
// off-session. Declines are collected into one BILLING_BALANCE_DUE refusal
// naming the total still due and the first declined invoice's hosted page.
func (c *StripeClient) chargeInvoices(ctx context.Context, workspaceID string, invoices []*stripe.Invoice) error {
	var due *core.BalanceDue
	for _, invoice := range invoices {
		unpaid, err := c.payInvoice(ctx, invoice)
		if err != nil {
			return fmt.Errorf("stripe: pay invoice %s of %s: %w", invoice.ID, workspaceID, err)
		}
		if unpaid == nil {
			continue
		}
		if due == nil {
			due = &core.BalanceDue{WorkspaceID: workspaceID, InvoiceID: unpaid.ID, HostedInvoiceURL: unpaid.HostedInvoiceURL, Currency: string(unpaid.Currency)}
		}
		due.AmountDueCents += unpaid.AmountRemaining
	}
	if due != nil {
		c.metrics.Operation("settle_charge", "declined")
		log.Printf("billing: settle before delete of %s refused: invoice %s is unpaid", workspaceID, due.InvoiceID)
		return core.NewBillingBalanceDueError(*due)
	}
	c.metrics.Operation("settle_charge", "success")
	return nil
}

// payInvoice charges one invoice off-session, once. It returns nil when the
// invoice is paid, the invoice still owing when the charge was declined (the
// one it was given, whose amount and hosted link are current), and an error
// for anything else (a transient failure, which keeps the deletion retryable
// without blaming the card).
func (c *StripeClient) payInvoice(ctx context.Context, invoice *stripe.Invoice) (*stripe.Invoice, error) {
	params := &stripe.InvoicePayParams{OffSession: stripe.Bool(true)}
	params.Context = ctx
	paid, err := c.sc.Invoices.Pay(invoice.ID, params)
	if err == nil {
		if paid.Status == stripe.InvoiceStatusPaid || paid.AmountRemaining <= 0 {
			return nil, nil
		}
		return paid, nil
	}
	if !chargeDeclined(err) {
		code, _ := safeStripeError(err)
		return nil, errors.New(code)
	}
	return invoice, nil
}

// chargeDeclined reports a payment the customer must act on: a card error, or
// Stripe's 402 for an invoice it could not collect (no usable payment method,
// authentication required).
func chargeDeclined(err error) bool {
	var se *stripe.Error
	if !errors.As(err, &se) {
		return false
	}
	return se.Type == stripe.ErrorTypeCard || se.HTTPStatusCode == http.StatusPaymentRequired
}
