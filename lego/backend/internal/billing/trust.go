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
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"slices"
	"time"

	stripe "github.com/stripe/stripe-go/v86"

	"github.com/bex-co/bex/lego/backend/internal/pricing"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// Trust feeds the agent-model trust tiers (ADR091 D10.1) from Stripe: a paid,
// non-zero bex invoice adds to the workspace's lifetime paid (and, for a
// subscription_cycle invoice, its paid cycle invoices); a failed invoice
// payment holds it at T0 until that invoice is paid, plus 30 days; a dispute
// holds it at T0 until the dispute closes as won. Every write is idempotent
// (paid invoices are keyed by invoice id), so Stripe's redelivery never
// double counts. Amounts, cards and payment details are never logged. The
// webhook has already rejected an event of the wrong mode.
type Trust struct {
	Store   TrustStore
	Charges DisputedChargeResolver
	// OnTierChanged is told after a committed tier change (the Stripe
	// agent-model threshold follows the tier); nil ⇒ nothing.
	OnTierChanged TierChangedFunc
	// WriteOffs and Invoices record an invoice marked uncollectible as
	// agent-model bad debt (ADR091 D10); either nil ⇒ the event is ignored.
	// Invoices also finalizes threshold invoices at once (nil ⇒ Stripe does
	// an hour later).
	WriteOffs WriteOffStore
	Invoices  TrustInvoices
	// ModelCharges settles the agent-model threshold charge ledger row of a
	// charge invoice (ADR091 D10.2a) that Stripe decided after the mint gave
	// up, or that the customer paid later on its hosted page; nil ⇒ the
	// trust record alone is kept.
	ModelCharges AgentModelChargeLedger
	Clock        func() time.Time
}

// AgentModelChargeLedger settles a threshold charge by its invoice;
// *store.PGStore satisfies it.
type AgentModelChargeLedger interface {
	SettleAgentModelChargeInvoice(ctx context.Context, invoiceID string, settlement store.AgentModelChargeSettlement) (store.AgentModelCharge, bool, error)
	RecordAgentModelBilledUntil(ctx context.Context, workspaceID string, until time.Time) (bool, error)
}

// WriteOffStore records agent-model bad debt; *store.PGStore satisfies it.
type WriteOffStore interface {
	RecordAgentModelWriteOff(ctx context.Context, w store.AgentModelWriteOff) (bool, error)
}

// TrustInvoices re-reads an invoice's agent-model lines and finalizes draft
// threshold invoices; *StripeClient satisfies it.
type TrustInvoices interface {
	AgentModelInvoiceLines(ctx context.Context, invoiceID string) (AgentModelInvoice, error)
	FinalizeThresholdInvoice(ctx context.Context, invoiceID string) error
	// WrittenOffCardFingerprint is the fingerprint of the card whose charge
	// of the invoice failed, else the Customer's default card ("" ⇒ none),
	// kept on the write-off for D10.6.
	WrittenOffCardFingerprint(ctx context.Context, invoiceID, customerID string) (string, error)
	// VoidAgentModelChargesBilledBy voids the workspace's open (declined)
	// threshold charge invoices that a subscription invoice billing the
	// meter through until bills again.
	VoidAgentModelChargesBilledBy(ctx context.Context, workspaceID, customerID string, until time.Time) error
}

// TierChangedFunc is told a workspace's trust tier changed through a
// webhook. A lock expiring changes the tier without an event; a consumer
// that must track it reads pricing.TrustTier on its own schedule.
type TierChangedFunc func(ctx context.Context, workspaceID string, tier pricing.Tier)

// TrustStore is the durable trust record; *store.PGStore satisfies it.
type TrustStore interface {
	RecordBillingTrustPayment(context.Context, store.BillingTrustPayment) (store.BillingTrust, store.BillingTrust, bool, error)
	RecordBillingTrustFailure(ctx context.Context, workspaceID, invoiceID string, at time.Time) (store.BillingTrust, store.BillingTrust, bool, error)
	SettleBillingTrustInvoice(ctx context.Context, workspaceID, invoiceID string, at time.Time) error
	RecordBillingTrustDispute(ctx context.Context, workspaceID, disputeID string) (store.BillingTrust, store.BillingTrust, error)
	RecordBillingTrustDisputeWon(ctx context.Context, workspaceID, disputeID string) (store.BillingTrust, store.BillingTrust, error)
	BillingWorkspaceForCustomer(ctx context.Context, customerID string, livemode bool) (string, bool, error)
}

// DisputedChargeResolver re-reads a disputed Charge's Customer from Stripe
// (a Dispute names only its Charge); *StripeClient satisfies it.
type DisputedChargeResolver interface {
	DisputedChargeCustomer(ctx context.Context, chargeID string) (string, error)
}

// trustEventTypes are the events Trust reduces. invoice.paid is the payment
// signal (invoice.payment_succeeded for the same invoice would be a
// duplicate the ledger absorbs anyway).
var trustEventTypes = []stripe.EventType{
	stripe.EventTypeInvoicePaid, stripe.EventTypeInvoicePaymentFailed,
	stripe.EventTypeChargeDisputeCreated, stripe.EventTypeChargeDisputeClosed,
	// Not tier inputs (ADR091 D10.2–D10.3): a voided invoice closes its
	// balance-due refusal; a written-off one records agent-model bad debt; a
	// draft threshold invoice is finalized at once.
	stripe.EventTypeInvoiceVoided, stripe.EventTypeInvoiceMarkedUncollectible, stripe.EventTypeInvoiceCreated,
}

func isTrustEvent(eventType stripe.EventType) bool {
	return slices.Contains(trustEventTypes, eventType)
}

func (t *Trust) now() time.Time {
	if t.Clock != nil {
		return t.Clock().UTC()
	}
	return time.Now().UTC()
}

// HandleStripeEvent reduces one signature-verified event. Events that are not
// bex's, or whose workspace is gone, are acknowledged; a store failure is
// returned so Stripe retries. A bex invoice is a Subscription contract
// invoice or a standalone agent-model threshold charge invoice (named by its
// own metadata); both feed the trust record the same way.
func (t *Trust) HandleStripeEvent(ctx context.Context, event *stripe.Event) error {
	if t == nil || t.Store == nil {
		return fmt.Errorf("billing trust store unavailable")
	}
	if event == nil || event.ID == "" || event.Data == nil || event.Created <= 0 {
		return fmt.Errorf("invalid Stripe trust event")
	}
	at := time.Unix(event.Created, 0).UTC()
	var (
		workspaceID   string
		before, after store.BillingTrust
		err           error
	)
	switch event.Type {
	case stripe.EventTypeInvoicePaid, stripe.EventTypeInvoicePaymentFailed, stripe.EventTypeInvoiceVoided,
		stripe.EventTypeInvoiceMarkedUncollectible, stripe.EventTypeInvoiceCreated:
		var invoice stripe.Invoice
		if err := json.Unmarshal(event.Data.Raw, &invoice); err != nil {
			return fmt.Errorf("invalid invoice event: %w", err)
		}
		ws, charge, ok := bexInvoiceWorkspace(&invoice)
		if !ok || invoice.ID == "" {
			return nil
		}
		workspaceID = ws
		if err := t.recordMeterBilled(ctx, event.Type, ws, &invoice); err != nil {
			return err
		}
		switch event.Type {
		case stripe.EventTypeInvoiceCreated:
			return t.finalizeThresholdInvoice(ctx, &invoice)
		case stripe.EventTypeInvoiceMarkedUncollectible:
			// A charge invoice rates no meter line, so it records no
			// write-off: its unpaid usage is still on the meter.
			return t.recordWriteOff(ctx, ws, &invoice, at)
		case stripe.EventTypeInvoiceVoided:
			if err := ignoreWorkspaceGone(t.Store.SettleBillingTrustInvoice(ctx, ws, invoice.ID, at)); err != nil || !charge {
				return err
			}
			return t.settleModelCharge(ctx, &invoice, store.AgentModelChargeFailed, chargeFailureVoid, at)
		case stripe.EventTypeInvoicePaymentFailed:
			before, after, _, err = t.Store.RecordBillingTrustFailure(ctx, ws, invoice.ID, at)
		default:
			payment := trustPayment(&invoice, ws, at)
			if payment.AmountCents <= 0 {
				// Credits and $0 invoices are not payment history, but they
				// do close the invoice for the balance-due refusal.
				err = ignoreWorkspaceGone(t.Store.SettleBillingTrustInvoice(ctx, ws, invoice.ID, payment.PaidAt))
				if err != nil || !charge {
					return err
				}
				return t.settleModelCharge(ctx, &invoice, store.AgentModelChargePaid, "", payment.PaidAt)
			}
			before, after, _, err = t.Store.RecordBillingTrustPayment(ctx, payment)
		}
		if err == nil && charge {
			status, failure := store.AgentModelChargePaid, ""
			if event.Type == stripe.EventTypeInvoicePaymentFailed {
				status, failure = store.AgentModelChargeFailed, chargeFailurePaymentFailed
			}
			if err = t.settleModelCharge(ctx, &invoice, status, failure, at); err != nil {
				return err
			}
		}
	case stripe.EventTypeChargeDisputeCreated, stripe.EventTypeChargeDisputeClosed:
		var dispute stripe.Dispute
		if err := json.Unmarshal(event.Data.Raw, &dispute); err != nil {
			return fmt.Errorf("invalid dispute event: %w", err)
		}
		if dispute.ID == "" {
			return nil
		}
		if event.Type == stripe.EventTypeChargeDisputeClosed && dispute.Status != stripe.DisputeStatusWon {
			// A lost (or otherwise closed) dispute keeps the hold: ops lifts it
			// once the account is settled (migration 0193).
			return nil
		}
		ws, found, derr := t.disputedWorkspace(ctx, event, &dispute)
		if derr != nil || !found {
			return derr
		}
		workspaceID = ws
		if event.Type == stripe.EventTypeChargeDisputeCreated {
			before, after, err = t.Store.RecordBillingTrustDispute(ctx, ws, dispute.ID)
		} else {
			before, after, err = t.Store.RecordBillingTrustDisputeWon(ctx, ws, dispute.ID)
		}
	default:
		return nil
	}
	if err != nil {
		return ignoreWorkspaceGone(err)
	}
	t.notifyTierChange(ctx, workspaceID, before, after)
	return nil
}

// settleModelCharge settles the threshold charge whose invoice this is: a
// failure Stripe decided after the mint stopped waiting, or a payment made
// later on the invoice's hosted page (the sweep then nets it). An invoice the
// ledger does not know is acknowledged.
func (t *Trust) settleModelCharge(ctx context.Context, invoice *stripe.Invoice, status, failure string, at time.Time) error {
	if t.ModelCharges == nil {
		return nil
	}
	_, found, err := t.ModelCharges.SettleAgentModelChargeInvoice(ctx, invoice.ID, store.AgentModelChargeSettlement{
		Status: status, HostedInvoiceURL: invoice.HostedInvoiceURL, Failure: failure, At: at,
	})
	if err == nil && !found {
		log.Printf("billing trust: agent-model charge invoice %s is not in the charge ledger; acknowledged", invoice.ID)
	}
	return err
}

// trustPayment is invoice as trust history counts it, with paid_at from the
// invoice's own transition, else fallback. Amount paid is gross: a later
// refund stays counted, as the invoice.paid webhook counts it.
func trustPayment(invoice *stripe.Invoice, workspaceID string, fallback time.Time) store.BillingTrustPayment {
	paidAt := fallback
	if invoice.StatusTransitions != nil && invoice.StatusTransitions.PaidAt > 0 {
		paidAt = time.Unix(invoice.StatusTransitions.PaidAt, 0).UTC()
	}
	return store.BillingTrustPayment{
		InvoiceID: invoice.ID, WorkspaceID: workspaceID, AmountCents: invoice.AmountPaid,
		Cycle: invoice.BillingReason == stripe.InvoiceBillingReasonSubscriptionCycle, PaidAt: paidAt,
	}
}

// paidInvoicePayment is a listed paid invoice as trust history, ok only for a
// bex invoice (a contract or a threshold charge) with a positive amount paid.
func paidInvoicePayment(invoice *stripe.Invoice) (store.BillingTrustPayment, bool) {
	workspaceID, _, ok := bexInvoiceWorkspace(invoice)
	if !ok || invoice.AmountPaid <= 0 {
		return store.BillingTrustPayment{}, false
	}
	return trustPayment(invoice, workspaceID, time.Unix(invoice.Created, 0).UTC()), true
}

// PaidInvoiceHistory lists a Stripe customer's paid bex invoices.
type PaidInvoiceHistory interface {
	PaidInvoices(ctx context.Context, customerID string) ([]store.BillingTrustPayment, error)
}

// Backfill counts the paid invoices of the mapped workspaces that the
// invoice.paid webhook never recorded; the invoice ledger keeps a rerun from
// counting anything twice. An invoice whose contract names another workspace
// is left alone. It returns how many invoices it counted.
func (t *Trust) Backfill(ctx context.Context, mappings []store.BillingProviderMapping, history PaidInvoiceHistory) (int, error) {
	if t == nil || t.Store == nil || history == nil {
		return 0, fmt.Errorf("billing trust backfill unavailable")
	}
	counted := 0
	var errs []error
	for _, m := range mappings {
		if err := ctx.Err(); err != nil {
			return counted, errors.Join(append(errs, err)...)
		}
		if m.CustomerID == "" {
			continue
		}
		payments, err := history.PaidInvoices(ctx, m.CustomerID)
		if err != nil {
			errs = append(errs, fmt.Errorf("workspace %s: %w", m.WorkspaceID, err))
			continue
		}
		for _, p := range payments {
			if p.WorkspaceID != m.WorkspaceID || p.AmountCents <= 0 {
				continue
			}
			before, after, ok, err := t.Store.RecordBillingTrustPayment(ctx, p)
			if err = ignoreWorkspaceGone(err); err != nil {
				errs = append(errs, fmt.Errorf("workspace %s invoice %s: %w", p.WorkspaceID, p.InvoiceID, err))
				continue
			}
			if ok {
				counted++
				t.notifyTierChange(ctx, p.WorkspaceID, before, after)
			}
		}
	}
	return counted, errors.Join(errs...)
}

// ignoreWorkspaceGone acknowledges an event whose workspace was deleted.
func ignoreWorkspaceGone(err error) error {
	if errors.Is(err, store.ErrWorkspaceGone) {
		return nil
	}
	return err
}

// recordMeterBilled acts on a subscription invoice that billed the meter,
// once it exists (created) and again once it is paid, in case the created
// event is not delivered: it moves the workspace's charge window past it
// (the charger also reads it from Stripe before each charge), and voids the
// declined charge invoices whose usage it now bills, so that usage is never
// owed twice.
func (t *Trust) recordMeterBilled(ctx context.Context, eventType stripe.EventType, workspaceID string, invoice *stripe.Invoice) error {
	if eventType != stripe.EventTypeInvoiceCreated && eventType != stripe.EventTypeInvoicePaid {
		return nil
	}
	until := meterBilledUntil(invoice)
	if until.IsZero() {
		return nil
	}
	if t.ModelCharges != nil {
		if _, err := t.ModelCharges.RecordAgentModelBilledUntil(ctx, workspaceID, until); err != nil {
			return fmt.Errorf("record where %s was last billed: %w", workspaceID, err)
		}
	}
	if t.Invoices != nil && invoice.Customer != nil {
		if err := t.Invoices.VoidAgentModelChargesBilledBy(ctx, workspaceID, invoice.Customer.ID, until); err != nil {
			return fmt.Errorf("void the agent-model charges %s bills again: %w", invoice.ID, err)
		}
	}
	return nil
}

// finalizeThresholdInvoice finalizes a draft subscription_threshold invoice so
// Stripe charges it now rather than after its one-hour draft window.
func (t *Trust) finalizeThresholdInvoice(ctx context.Context, invoice *stripe.Invoice) error {
	if t.Invoices == nil || invoice.BillingReason != stripe.InvoiceBillingReasonSubscriptionThreshold ||
		invoice.Status != stripe.InvoiceStatusDraft {
		return nil
	}
	return t.Invoices.FinalizeThresholdInvoice(ctx, invoice.ID)
}

// recordWriteOff records the agent-model share of an invoice marked
// uncollectible: its agent-model lines net of discounts (re-read from
// Stripe), prorated by the part of the invoice still unpaid
// (amount_remaining / total). An invoice with no agent-model spend records
// nothing.
func (t *Trust) recordWriteOff(ctx context.Context, workspaceID string, invoice *stripe.Invoice, at time.Time) error {
	if t.WriteOffs == nil || t.Invoices == nil || invoice.Customer == nil || invoice.Customer.ID == "" || invoice.Total <= 0 {
		return nil
	}
	lines, err := t.Invoices.AgentModelInvoiceLines(ctx, invoice.ID)
	if err != nil {
		return err
	}
	if lines.NetCents <= 0 {
		return nil
	}
	remaining := min(max(invoice.AmountRemaining, 0), invoice.Total)
	writtenOff := lines.NetCents * remaining / invoice.Total
	if writtenOff <= 0 {
		return nil
	}
	month := lines.UsageMonth
	if month.IsZero() {
		month = time.Unix(invoice.PeriodStart, 0).UTC()
	}
	// The card that owes the debt (ADR091 D10.6): binding it again, in any
	// workspace, is refused while the write-off stands.
	fingerprint, err := t.Invoices.WrittenOffCardFingerprint(ctx, invoice.ID, invoice.Customer.ID)
	if err != nil {
		return err
	}
	recorded, err := t.WriteOffs.RecordAgentModelWriteOff(ctx, store.AgentModelWriteOff{
		InvoiceID: invoice.ID, WorkspaceID: workspaceID, CustomerID: invoice.Customer.ID, UsageMonth: month,
		AgentModelNetMicroUSD: lines.NetCents * microUSDPerCent, AmountMicroUSD: writtenOff * microUSDPerCent, WrittenOffAt: at,
		CardFingerprint: fingerprint,
	})
	if errors.Is(err, store.ErrInvalidAgentModelWriteOff) {
		log.Printf("billing: invoice %s of workspace %s: %v; acknowledged", invoice.ID, workspaceID, err)
		return nil
	}
	if err == nil && recorded {
		log.Printf("billing: invoice %s of workspace %s written off; agent-model bad debt recorded", invoice.ID, workspaceID)
	}
	return err
}

// microUSDPerCent converts an invoice line's cents to the meter's micro-USD.
const microUSDPerCent = 10_000

// disputedWorkspace resolves a dispute's workspace through its Charge's
// Customer, re-read from Stripe (the event payload is never trusted for
// ownership) and mapped by billing_provider_mappings.
func (t *Trust) disputedWorkspace(ctx context.Context, event *stripe.Event, dispute *stripe.Dispute) (string, bool, error) {
	if dispute.Charge == nil || dispute.Charge.ID == "" {
		return "", false, nil
	}
	if t.Charges == nil {
		return "", false, fmt.Errorf("Stripe charge lookup unavailable")
	}
	customerID, err := t.Charges.DisputedChargeCustomer(ctx, dispute.Charge.ID)
	if err != nil {
		return "", false, err
	}
	if customerID == "" {
		return "", false, nil
	}
	workspaceID, found, err := t.Store.BillingWorkspaceForCustomer(ctx, customerID, event.Livemode)
	if err != nil {
		return "", false, err
	}
	if !found {
		log.Printf("billing trust: dispute event=%s names a customer bex does not map; acknowledged", event.ID)
	}
	return workspaceID, found, nil
}

func (t *Trust) notifyTierChange(ctx context.Context, workspaceID string, before, after store.BillingTrust) {
	if t.OnTierChanged == nil {
		return
	}
	now := t.now()
	was := pricing.TrustTier(before.LifetimePaidCents, before.PaidCycleInvoices, before.Lock(), now)
	is := pricing.TrustTier(after.LifetimePaidCents, after.PaidCycleInvoices, after.Lock(), now)
	if was.Level != is.Level {
		t.OnTierChanged(ctx, workspaceID, is)
	}
}

// DisputedChargeCustomer re-reads a Charge and returns its Customer id ("" for
// a customer-less charge). Only the id leaves this function.
func (c *StripeClient) DisputedChargeCustomer(ctx context.Context, chargeID string) (string, error) {
	params := &stripe.ChargeParams{}
	params.Context = ctx
	charge, err := c.sc.Charges.Get(chargeID, params)
	if resourceMissing(err) {
		return "", nil
	}
	if err != nil {
		code, _ := safeStripeError(err)
		return "", fmt.Errorf("stripe: retrieve disputed Charge: %s", code)
	}
	if charge.Customer == nil {
		return "", nil
	}
	return charge.Customer.ID, nil
}
