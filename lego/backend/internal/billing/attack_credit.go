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
	"github.com/bex-co/bex/lego/backend/internal/pricing"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// Hours Stripe already has come back as a credit (w4/m709 t004, ADR023 §
// Bandwidth served to a DDoS attack is not billed). The emitter sends an hour
// 48 hours after it ends, so a window declared within two days of the attack
// keeps every hour it covers off the bill by itself. A window declared later
// covers hours Stripe already rated, and Stripe can cancel a meter event only
// within 24 hours of receiving it. Those hours are credited instead, as a
// negative invoice item on the workspace's live Subscription, so the credit
// lands on the next invoice whether the hours' own invoice is still open or
// already finalized.

// egressAttackCreditMetadataKey tags each credit's invoice item with its
// workspace-month and seq.
const egressAttackCreditMetadataKey = "bex_egress_attack_credit"

// DefaultAttackCreditInterval is how often AttackCredits runs: hourly, as the
// emitter does.
const DefaultAttackCreditInterval = time.Hour

// attackCreditBatch bounds the unsent credits one pass sends.
const attackCreditBatch = 100

// AttackCreditStore is the persistence AttackCredits needs; *store.PGStore
// satisfies it.
type AttackCreditStore interface {
	EgressAttackCreditMonths(ctx context.Context) ([]store.EgressAttackCreditMonth, error)
	EgressAttackLedger(ctx context.Context, workspaceID string, month time.Time) (store.EgressAttackLedger, error)
	RecordEgressAttackCredit(ctx context.Context, credit store.EgressAttackCredit) (bool, error)
	UnsentEgressAttackCredits(ctx context.Context, limit int) ([]store.EgressAttackCredit, error)
	MarkEgressAttackCreditSent(ctx context.Context, credit store.EgressAttackCredit, invoiceItemID string, at time.Time) error
}

// AttackCreditor puts a credit on a workspace's next invoice and returns the
// invoice item's id; *StripeClient satisfies it.
type AttackCreditor interface {
	CreditEgressAttack(ctx context.Context, credit store.EgressAttackCredit) (string, error)
}

// AttackCredits works out, for every workspace-month a live window covers
// exported hours of, what those hours billed that they would not have with
// the window in place, records the part not yet credited, and sends each
// recorded credit to Stripe. Built only when Stripe is enabled.
type AttackCredits struct {
	Store  AttackCreditStore
	Stripe AttackCreditor
	// Plans reads each workspace's included bandwidth, the allowance the
	// emitter bills above.
	Plans    pricing.PlanReader
	Interval time.Duration

	now func() time.Time
}

// Run credits until ctx is cancelled.
func (a *AttackCredits) Run(ctx context.Context) {
	interval := a.Interval
	if interval <= 0 {
		interval = DefaultAttackCreditInterval
	}
	core.Poll(ctx, "egress attack credits", interval, a.RunOnce)
}

func (a *AttackCredits) clock() time.Time {
	if a.now != nil {
		return a.now()
	}
	return time.Now()
}

// RunOnce records every credit now owed, then sends every recorded credit
// Stripe has not accepted. A failure on one workspace-month or one credit is
// logged, joined into the result and retried on the next pass.
func (a *AttackCredits) RunOnce(ctx context.Context) error {
	months, err := a.Store.EgressAttackCreditMonths(ctx)
	if err != nil {
		return fmt.Errorf("egress attack credit months: %w", err)
	}
	var errs []error
	for _, m := range months {
		if err := a.record(ctx, m); err != nil {
			errs = append(errs, fmt.Errorf("%s %s: %w", m.WorkspaceID, m.Month.Format("2006-01"), err))
		}
	}
	credits, err := a.Store.UnsentEgressAttackCredits(ctx, attackCreditBatch)
	if err != nil {
		return errors.Join(append(errs, fmt.Errorf("unsent egress attack credits: %w", err))...)
	}
	for _, credit := range credits {
		itemID, err := a.Stripe.CreditEgressAttack(ctx, credit)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if err := a.Store.MarkEgressAttackCreditSent(ctx, credit, itemID, a.clock()); err != nil {
			errs = append(errs, fmt.Errorf("mark egress attack credit %s %s/%d sent: %w", credit.WorkspaceID, credit.Month.Format("2006-01"), credit.Seq, err))
			continue
		}
		log.Printf("billing: credited %s %d cents for %d bytes of %s attack bandwidth (credit %d, invoice item %s)",
			credit.WorkspaceID, credit.AmountCents, credit.Bytes, credit.Month.Format("2006-01"), credit.Seq, itemID)
	}
	return errors.Join(errs...)
}

// record works out m's credit and records what is not yet credited.
func (a *AttackCredits) record(ctx context.Context, m store.EgressAttackCreditMonth) error {
	included, err := pricing.IncludedForWorkspace(ctx, a.Plans, m.WorkspaceID)
	if err != nil {
		return err
	}
	ledger, err := a.Store.EgressAttackLedger(ctx, m.WorkspaceID, m.Month)
	if err != nil {
		return err
	}
	if ledger.Compacted {
		log.Printf("billing: %s's %s egress is compacted; its attack hours can no longer be credited", m.WorkspaceID, m.Month.Format("2006-01"))
		return nil
	}
	credit, ok := owedAttackCredit(ledger, included[store.UsageKindEgressBytes])
	if !ok {
		return nil
	}
	credit.WorkspaceID, credit.Month = m.WorkspaceID, m.Month
	_, err = a.Store.RecordEgressAttackCredit(ctx, credit)
	return err
}

// owedAttackCredit is the credit the ledger still owes, if any: the bytes and
// cents attackCreditBytes finds above those already credited. The cents are
// worked out on the month's whole credit and the earlier credits' cents
// subtracted, so rounding never drifts across several credits.
func owedAttackCredit(ledger store.EgressAttackLedger, allowance int64) (store.EgressAttackCredit, bool) {
	due := attackCreditBytes(ledger.Rows, allowance)
	bytes := due - ledger.CreditedBytes
	cents := pricing.Default.BandwidthCents(due) - ledger.CreditedCents
	if bytes <= 0 || cents <= 0 {
		return store.EgressAttackCredit{}, false
	}
	return store.EgressAttackCredit{Seq: ledger.NextSeq, Bytes: bytes, AmountCents: cents}, true
}

// attackCreditBytes is what a month's exported egress hours billed beyond what
// they owe with every live window in place: the export's own arithmetic
// (owedAbove against the allowance, in UsageBefore's order) run again with
// the windows' hours owing nothing and using none of the allowance. It covers
// the hours a window holds and the later hours an attack's bytes pushed above
// the allowance. Never negative: a window withdrawn after its hours shipped
// as 0 takes nothing back.
func attackCreditBytes(rows []store.EgressMonthRow, allowance int64) int64 {
	var billed, owed, before int64
	for _, r := range rows {
		if r.AttackHour {
			if r.Emitted {
				billed += r.Billed
			}
			continue
		}
		o := r.Quantity
		if allowance > 0 {
			o = owedAbove(before, r.Quantity, allowance)
		}
		before += r.Quantity
		if r.Emitted {
			billed += r.Billed
			owed += o
		}
	}
	return max(billed-owed, 0)
}

// CreditEgressAttack adds credit, negated, as a pending invoice item on the
// workspace's live Subscription, so its next invoice carries it. Its
// idempotency key is fixed per credit, so a retry never credits twice.
func (c *StripeClient) CreditEgressAttack(ctx context.Context, credit store.EgressAttackCredit) (string, error) {
	customerID, found, err := c.findCustomer(ctx, credit.WorkspaceID)
	if err != nil {
		return "", err
	}
	if !found {
		return "", fmt.Errorf("stripe: workspace %s has no Customer to credit its attack bandwidth", credit.WorkspaceID)
	}
	sub, err := c.liveSubscription(ctx, credit.WorkspaceID, customerID, false)
	if err != nil {
		return "", err
	}
	if sub == nil {
		return "", fmt.Errorf("stripe: workspace %s has no live Subscription to credit its attack bandwidth on", credit.WorkspaceID)
	}
	month := credit.Month.Format("2006-01")
	params := &stripe.InvoiceItemParams{
		Customer:     stripe.String(customerID),
		Subscription: stripe.String(sub.ID),
		Amount:       stripe.Int64(-credit.AmountCents),
		Currency:     stripe.String(string(stripe.CurrencyUSD)),
		Description:  stripe.String(attackCreditDescription(credit)),
		Metadata: map[string]string{
			workspaceMetadataKey:          credit.WorkspaceID,
			egressAttackCreditMetadataKey: month + "/" + strconv.Itoa(credit.Seq),
		},
	}
	if sub.AutomaticTax != nil && sub.AutomaticTax.Enabled {
		c.invoiceItemTax(params)
	}
	params.Context = ctx
	params.SetIdempotencyKey("bex-egress-attack-credit-" + credit.WorkspaceID + "-" + month + "-" + strconv.Itoa(credit.Seq))
	item, err := c.sc.InvoiceItems.New(params)
	if err != nil {
		c.metrics.Operation("egress_attack_credit", "error")
		code, _ := safeStripeError(err)
		return "", fmt.Errorf("stripe: credit %s's %s attack bandwidth on %s: %s", credit.WorkspaceID, month, sub.ID, code)
	}
	c.metrics.Operation("egress_attack_credit", "success")
	return item.ID, nil
}

// attackCreditDescription is the invoice line a credit shows the customer.
func attackCreditDescription(credit store.EgressAttackCredit) string {
	return "Outbound bandwidth from a DDoS attack, not billed (" + credit.Month.Format("January 2006") + ")"
}
