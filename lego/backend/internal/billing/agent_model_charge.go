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
	"strconv"
	"time"

	stripe "github.com/stripe/stripe-go/v86"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// Agent-model threshold charges (ADR091 D10.2a). When a workspace's unbilled
// agent-model spend (month-to-date minus the month's paid charges) reaches its
// trust tier's charge threshold, the model-credential mint charges the card
// there and then and waits for the result: paid, the request is served and
// the workspace climbs a tier; declined, it is refused with
// BILLING_BALANCE_DUE; undecided, it is refused and the charge is retried.
//
// The charge is a STANDALONE invoice on the workspace's Customer, never on its
// Subscription: it moves neither the billing anchor nor proration, and
// hosting's dunning never reads it as a contract invoice. The hourly
// meter still exports all agent-model usage, so a paid charge is netted out of
// the next cycle invoice by a negative pending invoice item on the
// Subscription. That nets a payment already collected; it issues no
// value, so it is not ADR071's rejected credit system.

const (
	// agentModelChargeMetadataKey marks a threshold charge invoice (value
	// "true"); the invoice also carries workspaceMetadataKey.
	agentModelChargeMetadataKey = "bex_agent_model_charge"
	// agentModelChargeMonthMetadataKey and agentModelChargeSeqMetadataKey
	// name the ledger row (store.AgentModelChargeKey).
	agentModelChargeMonthMetadataKey = "bex_agent_model_charge_month"
	agentModelChargeSeqMetadataKey   = "bex_agent_model_charge_seq"
	// agentModelChargeInvoiceMetadataKey, on a net item, is the charge
	// invoice it nets.
	agentModelChargeInvoiceMetadataKey = "bex_agent_model_charge_invoice"
)

const (
	// minAgentModelChargeCents is Stripe's minimum USD charge.
	minAgentModelChargeCents = 50
	// agentModelChargeJoinWait bounds how long a request that finds another
	// request's charge in flight waits for it.
	agentModelChargeJoinWait = 15 * time.Second
	// agentModelChargeRunTimeout bounds one charge attempt, inside the
	// gateway's 30 s mint timeout.
	agentModelChargeRunTimeout = 20 * time.Second
	// agentModelChargeRetryAfter is how old a pending charge is before the
	// sweep retries it: past any attempt a mint still has running.
	agentModelChargeRetryAfter = 2 * time.Minute
	agentModelChargePoll       = 250 * time.Millisecond
	agentModelChargeSweepLimit = 100
)

// agentModelChargeCents bills micro-USD in cents, rounded up, at least
// Stripe's minimum charge.
func agentModelChargeCents(microUSD int64) int64 {
	return max((microUSD+microUSDPerCent-1)/microUSDPerCent, minAgentModelChargeCents)
}

// agentModelChargeInvoice reports whether invoice is a threshold charge
// invoice, and its workspace.
func agentModelChargeInvoice(invoice *stripe.Invoice) (workspaceID string, ok bool) {
	if invoice == nil || invoice.Metadata[agentModelChargeMetadataKey] != "true" {
		return "", false
	}
	workspaceID = invoice.Metadata[workspaceMetadataKey]
	return workspaceID, workspaceID != ""
}

// bexInvoiceWorkspace is the workspace of a bex invoice: a Subscription
// contract invoice, or a threshold charge invoice.
func bexInvoiceWorkspace(invoice *stripe.Invoice) (workspaceID string, charge, ok bool) {
	if ws, _, ok := invoiceContract(invoice); ok {
		return ws, false, true
	}
	ws, ok := agentModelChargeInvoice(invoice)
	return ws, ok, ok
}

func agentModelChargeName(key store.AgentModelChargeKey) string {
	return fmt.Sprintf("%s-%s-%d", key.WorkspaceID, store.MonthStartUTC(key.Month).Format("2006-01"), key.Seq)
}

// agentModelChargeKey is one Stripe idempotency key of the charge: the step,
// and for a step a retry may repeat with a different outcome, the attempt
// (Stripe replays a key's first answer, a 500 included, for 24 hours).
func agentModelChargeKey(charge store.AgentModelCharge, step string, perAttempt bool) string {
	key := "bex-threshold-" + agentModelChargeName(charge.AgentModelChargeKey) + "-" + step
	if perAttempt {
		key += "-a" + strconv.Itoa(charge.Attempts)
	}
	return key
}

func agentModelChargeMetadata(charge store.AgentModelCharge) map[string]string {
	return map[string]string{
		workspaceMetadataKey:             charge.WorkspaceID,
		agentModelChargeMetadataKey:      "true",
		agentModelChargeMonthMetadataKey: store.MonthStartUTC(charge.Month).Format("2006-01"),
		agentModelChargeSeqMetadataKey:   strconv.Itoa(charge.Seq),
	}
}

func agentModelChargeDescription(charge store.AgentModelCharge) string {
	return "Agent model usage, " + store.MonthStartUTC(charge.Month).Format("January 2006")
}

// AgentModelChargeResult is one decided charge attempt.
type AgentModelChargeResult struct {
	Paid             bool
	InvoiceID        string
	HostedInvoiceURL string
	// AmountPaidCents is what the card paid: the trust history counts it.
	AmountPaidCents int64
	// Failure is the Stripe decline code of a declined charge, or
	// invoice_void / invoice_uncollectible for an invoice closed unpaid.
	Failure string
}

// The failure a charge row records: how Stripe declined or closed its invoice.
const (
	chargeFailureDeclined      = "declined"
	chargeFailurePaymentFailed = "payment_failed"
	chargeFailureVoid          = "invoice_" + string(stripe.InvoiceStatusVoid)
	chargeFailureUncollectible = "invoice_" + string(stripe.InvoiceStatusUncollectible)
)

// closedUnpaid reports a charge whose invoice was voided or written off
// rather than declined: no payment failed, so no balance is due for it.
func closedUnpaid(failure string) bool {
	return failure == chargeFailureVoid || failure == chargeFailureUncollectible
}

// ChargeAgentModelUsage makes one attempt at charge: a standalone invoice on
// the workspace's Customer (recordInvoice persists its id before anything can
// charge it), the charge's amount on it, finalized and paid off-session. A
// charge that already has an invoice resumes it where the last attempt
// stopped. A declined or authentication-required payment is a decided result
// (Paid false); anything Stripe did not decide is an error.
func (c *StripeClient) ChargeAgentModelUsage(ctx context.Context, charge store.AgentModelCharge, recordInvoice func(context.Context, string) error) (AgentModelChargeResult, error) {
	invoice, err := c.agentModelChargeInvoice(ctx, charge, recordInvoice)
	if err != nil {
		c.metrics.Operation("agent_model_charge", "error")
		return AgentModelChargeResult{}, err
	}
	if invoice.Status == stripe.InvoiceStatusDraft {
		if invoice.Subtotal == 0 {
			if err := c.addAgentModelChargeItem(ctx, charge, invoice); err != nil {
				c.metrics.Operation("agent_model_charge", "error")
				return AgentModelChargeResult{InvoiceID: invoice.ID}, err
			}
		}
		finalized, err := c.finalizeDraftInvoice(ctx, invoice.ID, agentModelChargeKey(charge, "finalize", true), stripe.Bool(false))
		if err == nil && finalized == nil {
			err = fmt.Errorf("stripe: agent-model charge invoice %s is gone", invoice.ID)
		}
		if err != nil {
			c.metrics.Operation("agent_model_charge", "error")
			return AgentModelChargeResult{InvoiceID: invoice.ID}, err
		}
		invoice = finalized
	}
	result := AgentModelChargeResult{InvoiceID: invoice.ID, HostedInvoiceURL: invoice.HostedInvoiceURL}
	switch invoice.Status {
	case stripe.InvoiceStatusPaid:
		result.Paid, result.AmountPaidCents = true, invoice.AmountPaid
		c.metrics.Operation("agent_model_charge", "paid")
		return result, nil
	case stripe.InvoiceStatusVoid, stripe.InvoiceStatusUncollectible:
		result.Failure = "invoice_" + string(invoice.Status)
		c.metrics.Operation("agent_model_charge", "closed")
		return result, nil
	case stripe.InvoiceStatusOpen:
	default:
		c.metrics.Operation("agent_model_charge", "error")
		return result, fmt.Errorf("stripe: agent-model charge invoice %s is %q", invoice.ID, invoice.Status)
	}
	params := &stripe.InvoicePayParams{OffSession: stripe.Bool(true)}
	params.Context = ctx
	params.SetIdempotencyKey(agentModelChargeKey(charge, "pay", true))
	paid, err := c.sc.Invoices.Pay(invoice.ID, params)
	if err != nil {
		code, _ := safeStripeError(err)
		if chargeDeclined(err) || paymentNeedsCustomer(code) {
			result.Failure = code
			c.metrics.Operation("agent_model_charge", "declined")
			log.Printf("billing: agent-model charge invoice %s of %s declined (%s)", invoice.ID, charge.WorkspaceID, code)
			return result, nil
		}
		c.metrics.Operation("agent_model_charge", "error")
		return result, fmt.Errorf("stripe: pay agent-model charge invoice %s: %s", invoice.ID, code)
	}
	if paid.HostedInvoiceURL != "" {
		result.HostedInvoiceURL = paid.HostedInvoiceURL
	}
	if paid.Status != stripe.InvoiceStatusPaid {
		// Accepted but not settled (a processing debit): undecided.
		c.metrics.Operation("agent_model_charge", "processing")
		return result, fmt.Errorf("stripe: agent-model charge invoice %s is %q after payment", invoice.ID, paid.Status)
	}
	result.Paid, result.AmountPaidCents = true, paid.AmountPaid
	c.metrics.Operation("agent_model_charge", "paid")
	return result, nil
}

// paymentNeedsCustomer reports a payment Stripe refused because the customer
// must act (authenticate, or add a payment method), whatever error type
// carries it.
func paymentNeedsCustomer(code string) bool {
	switch code {
	case "authentication_required", "invoice_payment_intent_requires_action", "payment_intent_authentication_failure",
		"invoice_no_payment_method_types", "invoice_no_customer_payment_method", "payment_method_unactivated":
		return true
	}
	return false
}

// agentModelChargeInvoice is the charge's invoice: re-read when it has one,
// else a new draft (recorded before it is returned).
func (c *StripeClient) agentModelChargeInvoice(ctx context.Context, charge store.AgentModelCharge, recordInvoice func(context.Context, string) error) (*stripe.Invoice, error) {
	if charge.InvoiceID != "" {
		params := &stripe.InvoiceParams{}
		params.Context = ctx
		invoice, err := c.sc.Invoices.Get(charge.InvoiceID, params)
		if err != nil {
			code, _ := safeStripeError(err)
			return nil, fmt.Errorf("stripe: retrieve agent-model charge invoice %s: %s", charge.InvoiceID, code)
		}
		return invoice, nil
	}
	customerID, found, err := c.findCustomer(ctx, charge.WorkspaceID)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, fmt.Errorf("stripe: workspace %s has no Customer to charge", charge.WorkspaceID)
	}
	// Read for its tax setting only: the invoice is the Customer's, so the
	// Subscription's anchor, proration and dunning never see it.
	sub, err := c.liveSubscription(ctx, charge.WorkspaceID, customerID, false)
	if err != nil {
		return nil, err
	}
	params := &stripe.InvoiceParams{
		Customer:                    stripe.String(customerID),
		CollectionMethod:            stripe.String(string(stripe.InvoiceCollectionMethodChargeAutomatically)),
		AutoAdvance:                 stripe.Bool(false),
		PendingInvoiceItemsBehavior: stripe.String("exclude"),
		Description:                 stripe.String(agentModelChargeDescription(charge)),
		Metadata:                    agentModelChargeMetadata(charge),
	}
	if sub != nil && sub.AutomaticTax != nil && sub.AutomaticTax.Enabled {
		params.AutomaticTax = &stripe.InvoiceAutomaticTaxParams{Enabled: stripe.Bool(true)}
	}
	params.Context = ctx
	params.SetIdempotencyKey(agentModelChargeKey(charge, "invoice", true))
	invoice, err := c.sc.Invoices.New(params)
	if err != nil {
		code, _ := safeStripeError(err)
		return nil, fmt.Errorf("stripe: create agent-model charge invoice for %s: %s", charge.WorkspaceID, code)
	}
	if err := recordInvoice(ctx, invoice.ID); err != nil {
		return nil, fmt.Errorf("record agent-model charge invoice %s: %w", invoice.ID, err)
	}
	if invoice.Customer == nil {
		invoice.Customer = &stripe.Customer{ID: customerID}
	}
	return invoice, nil
}

// agentModelChargeTax applies the configured tax behavior and code to an
// item on an automatically taxed invoice, as the catalog Prices carry them.
func (c *StripeClient) agentModelChargeTax(params *stripe.InvoiceItemParams) {
	if c.taxBehavior != "" {
		params.TaxBehavior = stripe.String(c.taxBehavior)
	}
	if c.taxCode != "" {
		params.TaxCode = stripe.String(c.taxCode)
	}
}

// addAgentModelChargeItem puts the charge's amount on its draft invoice.
func (c *StripeClient) addAgentModelChargeItem(ctx context.Context, charge store.AgentModelCharge, invoice *stripe.Invoice) error {
	if invoice.Customer == nil || invoice.Customer.ID == "" {
		return fmt.Errorf("stripe: agent-model charge invoice %s has no customer", invoice.ID)
	}
	params := &stripe.InvoiceItemParams{
		Customer:    stripe.String(invoice.Customer.ID),
		Invoice:     stripe.String(invoice.ID),
		Amount:      stripe.Int64(charge.AmountCents),
		Currency:    stripe.String(string(stripe.CurrencyUSD)),
		Description: stripe.String(agentModelChargeDescription(charge)),
		Metadata:    agentModelChargeMetadata(charge),
	}
	if invoice.AutomaticTax != nil && invoice.AutomaticTax.Enabled {
		c.agentModelChargeTax(params)
	}
	params.Context = ctx
	params.SetIdempotencyKey(agentModelChargeKey(charge, "item", true))
	if _, err := c.sc.InvoiceItems.New(params); err != nil {
		code, _ := safeStripeError(err)
		return fmt.Errorf("stripe: add agent-model charge to invoice %s: %s", invoice.ID, code)
	}
	return nil
}

// NetAgentModelCharge adds the paid charge's amount, negated, as a pending
// invoice item on the workspace's live Subscription, so the next cycle invoice
// bills only agent-model usage not already charged. Its idempotency key is
// fixed per charge, so a retry never nets twice.
func (c *StripeClient) NetAgentModelCharge(ctx context.Context, charge store.AgentModelCharge) (string, error) {
	if charge.Status != store.AgentModelChargePaid || charge.InvoiceID == "" {
		return "", fmt.Errorf("agent-model charge %s is not paid", agentModelChargeName(charge.AgentModelChargeKey))
	}
	customerID, found, err := c.findCustomer(ctx, charge.WorkspaceID)
	if err != nil {
		return "", err
	}
	if !found {
		return "", fmt.Errorf("stripe: workspace %s has no Customer to net its agent-model charge", charge.WorkspaceID)
	}
	sub, err := c.liveSubscription(ctx, charge.WorkspaceID, customerID, false)
	if err != nil {
		return "", err
	}
	if sub == nil {
		// The next Subscription's first invoice nets it; the sweep retries.
		return "", fmt.Errorf("stripe: workspace %s has no live Subscription to net agent-model charge %s against", charge.WorkspaceID, charge.InvoiceID)
	}
	metadata := agentModelChargeMetadata(charge)
	metadata[agentModelChargeInvoiceMetadataKey] = charge.InvoiceID
	params := &stripe.InvoiceItemParams{
		Customer:     stripe.String(customerID),
		Subscription: stripe.String(sub.ID),
		Amount:       stripe.Int64(-charge.AmountCents),
		Currency:     stripe.String(string(stripe.CurrencyUSD)),
		Description:  stripe.String("Agent model usage already charged on " + charge.InvoiceID),
		Metadata:     metadata,
	}
	if sub.AutomaticTax != nil && sub.AutomaticTax.Enabled {
		c.agentModelChargeTax(params)
	}
	params.Context = ctx
	params.SetIdempotencyKey(agentModelChargeKey(charge, "net", false))
	item, err := c.sc.InvoiceItems.New(params)
	if err != nil {
		c.metrics.Operation("agent_model_charge_net", "error")
		code, _ := safeStripeError(err)
		return "", fmt.Errorf("stripe: net agent-model charge %s on %s: %s", charge.InvoiceID, sub.ID, code)
	}
	c.metrics.Operation("agent_model_charge_net", "success")
	return item.ID, nil
}

// AgentModelChargeStore is AgentModelCharger's store surface; *store.PGStore
// satisfies it.
type AgentModelChargeStore interface {
	BeginAgentModelCharge(ctx context.Context, workspaceID string, now time.Time, seenSeq int, amountMicroUSD, amountCents int64) (store.AgentModelCharge, bool, error)
	GetAgentModelCharge(ctx context.Context, key store.AgentModelChargeKey) (store.AgentModelCharge, error)
	SetAgentModelChargeInvoice(ctx context.Context, key store.AgentModelChargeKey, invoiceID string) error
	CountAgentModelChargeAttempt(ctx context.Context, key store.AgentModelChargeKey) error
	SettleAgentModelCharge(ctx context.Context, key store.AgentModelChargeKey, settlement store.AgentModelChargeSettlement) (store.AgentModelCharge, error)
	SetAgentModelChargeNetItem(ctx context.Context, key store.AgentModelChargeKey, itemID string) error
	AgentModelChargesToRetry(ctx context.Context, pendingOlderThan time.Time, limit int) ([]store.AgentModelCharge, error)
	RunExclusive(ctx context.Context, name string, fn func(context.Context) error) (bool, error)
}

// AgentModelChargeProvider is the Stripe side; *StripeClient satisfies it.
type AgentModelChargeProvider interface {
	ChargeAgentModelUsage(ctx context.Context, charge store.AgentModelCharge, recordInvoice func(context.Context, string) error) (AgentModelChargeResult, error)
	NetAgentModelCharge(ctx context.Context, charge store.AgentModelCharge) (string, error)
}

// ChargeOutcome is how a threshold charge ended for the request waiting on it.
type ChargeOutcome int

const (
	// ChargePending: Stripe did not decide in time; the charge stays pending
	// and the sweep retries it.
	ChargePending ChargeOutcome = iota
	// ChargePaid: the card paid.
	ChargePaid
	// ChargeDeclined: the card was declined or needs authentication; the
	// workspace has a balance due until the invoice is paid.
	ChargeDeclined
)

// ChargeResult is a threshold charge's outcome and its invoice.
type ChargeResult struct {
	Outcome          ChargeOutcome
	InvoiceID        string
	HostedInvoiceURL string
}

// AgentModelCharger makes threshold charges (ChargeThreshold, from the mint)
// and finishes the ones a mint could not (Sweep: pending charges retried,
// paid ones netted), one replica at a time.
type AgentModelCharger struct {
	Store  AgentModelChargeStore
	Stripe AgentModelChargeProvider
	// Trust records a decided charge in the trust history at once (the
	// webhook for the same invoice is then a no-op) and announces the tier
	// change. Nil leaves that to the webhook.
	Trust    *Trust
	Interval time.Duration // sweep period; zero is one minute
	Clock    func() time.Time

	// joinWait overrides agentModelChargeJoinWait in tests.
	joinWait time.Duration
}

func (c *AgentModelCharger) now() time.Time {
	if c.Clock != nil {
		return c.Clock().UTC()
	}
	return time.Now().UTC()
}

func (c *AgentModelCharger) wait() time.Duration {
	if c.joinWait > 0 {
		return c.joinWait
	}
	return agentModelChargeJoinWait
}

// ChargeThreshold charges workspaceID's unbilled agent-model spend, as its
// spend read at at saw it (seenSeq: the month's last charge in that read),
// and waits for the result. Exactly one request starts a crossing's charge;
// any other joins it and waits up to agentModelChargeJoinWait. The started
// charge runs to its own timeout even if the request gives up.
func (c *AgentModelCharger) ChargeThreshold(ctx context.Context, workspaceID string, at time.Time, seenSeq int, unbilledMicroUSD int64) (ChargeResult, error) {
	if c == nil || c.Store == nil || c.Stripe == nil {
		return ChargeResult{}, fmt.Errorf("agent-model charger unavailable")
	}
	if unbilledMicroUSD <= 0 {
		return ChargeResult{}, fmt.Errorf("agent-model charge of %s has nothing to charge", workspaceID)
	}
	charge, started, err := c.Store.BeginAgentModelCharge(ctx, workspaceID, at, seenSeq, unbilledMicroUSD, agentModelChargeCents(unbilledMicroUSD))
	if err != nil {
		return ChargeResult{}, fmt.Errorf("begin agent-model charge of %s: %w", workspaceID, err)
	}
	if !started {
		return c.await(ctx, charge)
	}
	runCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), agentModelChargeRunTimeout)
	defer cancel()
	return c.run(runCtx, charge), nil
}

// await waits for a charge another request started. One Stripe already could
// not decide is left to the sweep: the request does not wait on it.
func (c *AgentModelCharger) await(ctx context.Context, charge store.AgentModelCharge) (ChargeResult, error) {
	deadline := time.Now().Add(c.wait())
	for {
		if result, settled := settledResult(charge); settled {
			return result, nil
		}
		if charge.Attempts > 0 || c.now().Sub(charge.CreatedAt) > c.wait() || !time.Now().Before(deadline) {
			return ChargeResult{Outcome: ChargePending, InvoiceID: charge.InvoiceID}, nil
		}
		select {
		case <-ctx.Done():
			return ChargeResult{Outcome: ChargePending, InvoiceID: charge.InvoiceID}, nil
		case <-time.After(agentModelChargePoll):
		}
		next, err := c.Store.GetAgentModelCharge(ctx, charge.AgentModelChargeKey)
		if err != nil {
			return ChargeResult{}, fmt.Errorf("read agent-model charge %s: %w", agentModelChargeName(charge.AgentModelChargeKey), err)
		}
		charge = next
	}
}

// settledResult is a settled charge's outcome; settled is false while it is
// pending.
func settledResult(charge store.AgentModelCharge) (ChargeResult, bool) {
	result := ChargeResult{InvoiceID: charge.InvoiceID, HostedInvoiceURL: charge.HostedInvoiceURL}
	switch charge.Status {
	case store.AgentModelChargePaid:
		result.Outcome = ChargePaid
	case store.AgentModelChargeFailed:
		result.Outcome = ChargeDeclined
		if closedUnpaid(charge.Failure) {
			result.Outcome = ChargePending
		}
	default:
		return ChargeResult{}, false
	}
	return result, true
}

// run makes one attempt at a pending charge and settles what Stripe decided.
func (c *AgentModelCharger) run(ctx context.Context, charge store.AgentModelCharge) ChargeResult {
	key := charge.AgentModelChargeKey
	res, err := c.Stripe.ChargeAgentModelUsage(ctx, charge, func(ctx context.Context, invoiceID string) error {
		return c.Store.SetAgentModelChargeInvoice(ctx, key, invoiceID)
	})
	if err != nil {
		log.Printf("billing: agent-model charge %s stays pending (the sweep retries it): %v", agentModelChargeName(key), err)
		if err := c.Store.CountAgentModelChargeAttempt(ctx, key); err != nil {
			log.Printf("billing: count agent-model charge %s attempt: %v", agentModelChargeName(key), err)
		}
		return ChargeResult{Outcome: ChargePending, InvoiceID: res.InvoiceID}
	}
	now := c.now()
	settlement := store.AgentModelChargeSettlement{Status: store.AgentModelChargePaid, HostedInvoiceURL: res.HostedInvoiceURL, At: now}
	if !res.Paid {
		settlement.Status, settlement.Failure = store.AgentModelChargeFailed, res.Failure
		if settlement.Failure == "" {
			settlement.Failure = chargeFailureDeclined
		}
	}
	settled, err := c.Store.SettleAgentModelCharge(ctx, key, settlement)
	if err != nil {
		// Stripe decided; the sweep re-reads the invoice and records it.
		log.Printf("billing: settle agent-model charge %s (%s): %v", agentModelChargeName(key), settlement.Status, err)
		return ChargeResult{Outcome: ChargePending, InvoiceID: res.InvoiceID}
	}
	result := ChargeResult{Outcome: ChargePaid, InvoiceID: res.InvoiceID, HostedInvoiceURL: res.HostedInvoiceURL}
	switch {
	case res.Paid:
		c.recordPayment(ctx, charge.WorkspaceID, res.InvoiceID, res.AmountPaidCents, now)
		if err := c.net(ctx, settled); err != nil {
			log.Printf("billing: %v (the sweep retries it)", err)
		}
	case closedUnpaid(res.Failure):
		// Voided or written off rather than declined: its own webhook
		// settles the balance; the next crossing charges again.
		result.Outcome = ChargePending
	default:
		result.Outcome = ChargeDeclined
		c.recordFailure(ctx, charge.WorkspaceID, res.InvoiceID, now)
	}
	return result
}

// recordPayment counts a paid charge in the trust history now, so the
// request that made it sees the higher tier; invoice.paid for the same
// invoice then adds nothing. A charge the card did not pay (a customer
// balance covered it) is no payment history.
func (c *AgentModelCharger) recordPayment(ctx context.Context, workspaceID, invoiceID string, cents int64, at time.Time) {
	if c.Trust == nil || c.Trust.Store == nil || cents <= 0 {
		return
	}
	before, after, _, err := c.Trust.Store.RecordBillingTrustPayment(ctx, store.BillingTrustPayment{
		InvoiceID: invoiceID, WorkspaceID: workspaceID, AmountCents: cents, PaidAt: at,
	})
	if err = ignoreWorkspaceGone(err); err != nil {
		log.Printf("billing: record agent-model charge %s of %s as paid (its invoice.paid webhook does): %v", invoiceID, workspaceID, err)
		return
	}
	c.Trust.notifyTierChange(ctx, workspaceID, before, after)
}

// recordFailure opens the declined charge's invoice for the mint's
// BILLING_BALANCE_DUE refusal and holds the workspace at T0 until it is paid.
func (c *AgentModelCharger) recordFailure(ctx context.Context, workspaceID, invoiceID string, at time.Time) {
	if c.Trust == nil || c.Trust.Store == nil || invoiceID == "" {
		return
	}
	before, after, _, err := c.Trust.Store.RecordBillingTrustFailure(ctx, workspaceID, invoiceID, at)
	if err = ignoreWorkspaceGone(err); err != nil {
		log.Printf("billing: record agent-model charge %s of %s as failed (its invoice.payment_failed webhook does): %v", invoiceID, workspaceID, err)
		return
	}
	c.Trust.notifyTierChange(ctx, workspaceID, before, after)
}

// net nets a paid charge out of the next cycle invoice and records the item.
func (c *AgentModelCharger) net(ctx context.Context, charge store.AgentModelCharge) error {
	itemID, err := c.Stripe.NetAgentModelCharge(ctx, charge)
	if err != nil {
		return fmt.Errorf("net agent-model charge %s: %w", agentModelChargeName(charge.AgentModelChargeKey), err)
	}
	if err := c.Store.SetAgentModelChargeNetItem(ctx, charge.AgentModelChargeKey, itemID); err != nil {
		return fmt.Errorf("record net item %s of agent-model charge %s: %w", itemID, agentModelChargeName(charge.AgentModelChargeKey), err)
	}
	return nil
}

// Sweep finishes what a mint could not: it retries each pending charge older
// than agentModelChargeRetryAfter (paying the invoice it already has,
// idempotently, or creating it) and nets each paid charge not netted yet.
func (c *AgentModelCharger) Sweep(ctx context.Context) error {
	charges, err := c.Store.AgentModelChargesToRetry(ctx, c.now().Add(-agentModelChargeRetryAfter), agentModelChargeSweepLimit)
	if err != nil {
		return fmt.Errorf("agent-model charges to retry: %w", err)
	}
	var errs []error
	for _, charge := range charges {
		if err := ctx.Err(); err != nil {
			return errors.Join(append(errs, err)...)
		}
		switch charge.Status {
		case store.AgentModelChargePending:
			// run logs a charge that stays pending; the next sweep retries it.
			runCtx, cancel := context.WithTimeout(ctx, agentModelChargeRunTimeout)
			c.run(runCtx, charge)
			cancel()
		case store.AgentModelChargePaid:
			if err := c.net(ctx, charge); err != nil {
				errs = append(errs, err)
			}
		}
	}
	return errors.Join(errs...)
}

// RunOnce is one sweep, on the one replica holding its lock.
func (c *AgentModelCharger) RunOnce(ctx context.Context) error {
	_, err := c.Store.RunExclusive(ctx, "billing:agent-model-charges", c.Sweep)
	return err
}

// Run sweeps every Interval until ctx ends.
func (c *AgentModelCharger) Run(ctx context.Context) {
	interval := c.Interval
	if interval <= 0 {
		interval = time.Minute
	}
	core.Poll(ctx, "agent-model charge sweep", interval, c.RunOnce)
}
