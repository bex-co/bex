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
	"slices"
	"time"

	stripe "github.com/stripe/stripe-go/v86"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/pricing"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// Threshold billing (ADR091 D10.2). The Subscription's agent-model item
// carries billing_thresholds.usage_gte equal to the workspace's trust-tier
// charge threshold in meter units (micro-USD); Stripe then invoices and
// charges that item's usage as soon as it crosses the threshold, and the
// cycle invoice bills only the remainder (Stripe test-mode probe 2026-11-05). The
// threshold is item-level so compute never triggers it. A held agent-model
// meter (no catalog Price yet) has no item, so no threshold is ever set.

// agentModelMeter is the resolved agent-model catalog Price and its meter,
// both "" while the meter is held or the catalog is unresolved.
func (c *StripeClient) agentModelMeter() (priceID, meter string) {
	if slices.Contains(c.heldMeters, store.UsageKindAgentModelCostMicroUSD) {
		return "", ""
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	priceID = c.priceByKey[store.UsageKindAgentModelCostMicroUSD]
	return priceID, c.priceMeters[priceID]
}

// agentModelItem picks the Subscription item rating the agent-model meter:
// the one on the catalog Price, else one on a retired Price of the same
// meter. More than one such item (each would carry its own threshold) is
// reported.
func (c *StripeClient) agentModelItem(workspaceID, subscriptionID string, items []*stripe.SubscriptionItem) *stripe.SubscriptionItem {
	priceID, meter := c.agentModelMeter()
	if meter == "" {
		return nil
	}
	var picked *stripe.SubscriptionItem
	matches := 0
	for _, item := range items {
		if item.Price == nil || item.Price.Recurring == nil || item.Price.Recurring.Meter != meter {
			continue
		}
		matches++
		if picked == nil || item.Price.ID == priceID {
			picked = item
		}
	}
	if matches > 1 {
		c.metrics.Operation("agent_model_item", "duplicate")
		log.Printf("billing: subscription %s (%s) has %d items rating the agent-model meter; the threshold is kept on %s only",
			subscriptionID, workspaceID, matches, picked.ID)
	}
	return picked
}

// wantAgentModelThreshold is the workspace's threshold in micro-USD, nil for
// none (T4, or no threshold source wired).
func (c *StripeClient) wantAgentModelThreshold(ctx context.Context, workspaceID string) (*int64, error) {
	if c.agentModelThreshold == nil {
		return nil, nil
	}
	want, err := c.agentModelThreshold(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("agent-model threshold for %s: %w", workspaceID, err)
	}
	return want, nil
}

// thresholdParams is a new agent-model item's billing_thresholds, nil for
// none. A failed tier read is reported and the item is created without one:
// the next contract sync (EnsureContract) applies it, and nothing blocks the
// Subscription or usage export.
func (c *StripeClient) thresholdParams(ctx context.Context, workspaceID string) *stripe.SubscriptionItemBillingThresholdsParams {
	want, err := c.wantAgentModelThreshold(ctx, workspaceID)
	if err != nil {
		c.metrics.Operation("agent_model_threshold", "error")
		log.Printf("billing: creating the agent-model item of %s without a threshold (the next sync sets it): %v", workspaceID, err)
		return nil
	}
	if want == nil {
		return nil
	}
	return &stripe.SubscriptionItemBillingThresholdsParams{UsageGTE: stripe.Int64(*want)}
}

// contractItems is a new Subscription's item list: every catalog Price, the
// agent-model one carrying the workspace's charge threshold.
func (c *StripeClient) contractItems(ctx context.Context, workspaceID string, priceIDs []string) []*stripe.SubscriptionItemsParams {
	agentPrice, _ := c.agentModelMeter()
	items := make([]*stripe.SubscriptionItemsParams, 0, len(priceIDs))
	for _, priceID := range priceIDs {
		item := &stripe.SubscriptionItemsParams{Price: stripe.String(priceID)}
		if priceID == agentPrice {
			item.BillingThresholds = c.thresholdParams(ctx, workspaceID)
		}
		items = append(items, item)
	}
	return items
}

// applyAgentModelThreshold updates item's usage threshold to want (nil
// removes it), only when Stripe's current value differs.
func (c *StripeClient) applyAgentModelThreshold(ctx context.Context, workspaceID string, item *stripe.SubscriptionItem, want *int64) error {
	var current, target int64
	if item.BillingThresholds != nil {
		current = item.BillingThresholds.UsageGTE
	}
	if want != nil {
		target = *want
	}
	if current == target {
		return nil
	}
	params := &stripe.SubscriptionItemParams{ProrationBehavior: stripe.String("none")}
	if want == nil {
		params.AddUnsetField(stripe.SubscriptionItemParamsUnsetFieldBillingThresholds)
	} else {
		params.BillingThresholds = &stripe.SubscriptionItemBillingThresholdsParams{UsageGTE: stripe.Int64(target)}
	}
	params.Context = ctx
	// No idempotency key: the update is idempotent by value, and a fixed key
	// would replay a stale response when a tier goes $10 → $25 → $10 within
	// Stripe's 24h key window.
	if _, err := c.sc.SubscriptionItems.Update(item.ID, params); err != nil {
		c.metrics.Operation("agent_model_threshold", "error")
		code, _ := safeStripeError(err)
		return fmt.Errorf("stripe: set agent-model threshold on %s for %s: %s", item.ID, workspaceID, code)
	}
	c.metrics.Operation("agent_model_threshold", "success")
	log.Printf("billing: workspace %s agent-model threshold %d → %d micro-USD (0 = none)", workspaceID, current, target)
	return nil
}

// SyncAgentModelThreshold brings the workspace's agent-model item threshold
// to want (nil = none). A workspace with no Customer, no live Subscription,
// or no agent-model item (the meter is held, or the item is added by the
// next EnsureContract with its threshold) is a no-op, as is an item already
// at want.
func (c *StripeClient) SyncAgentModelThreshold(ctx context.Context, workspaceID string, want *int64) error {
	if _, err := c.resolvePriceIDs(ctx); err != nil {
		return err
	}
	if _, meter := c.agentModelMeter(); meter == "" {
		return nil // held: no item, never a threshold
	}
	customerID, found, err := c.findCustomer(ctx, workspaceID)
	if err != nil || !found {
		return err
	}
	sub, err := c.liveSubscription(ctx, workspaceID, customerID, false)
	if err != nil || sub == nil {
		return err
	}
	items, err := c.subscriptionItems(ctx, workspaceID, sub)
	if err != nil {
		return err
	}
	item := c.agentModelItem(workspaceID, sub.ID, items)
	if item == nil {
		// Its threshold cannot reach Stripe until EnsureContract adds the
		// item, and until then the workspace is never charged mid-cycle.
		log.Printf("billing: subscription %s of workspace %s has no agent-model item; its threshold waits for the next contract sync", sub.ID, workspaceID)
		return nil
	}
	return c.applyAgentModelThreshold(ctx, workspaceID, item, want)
}

// subscriptionItems is every item of sub: the embedded first page when it is
// complete, else the full list.
func (c *StripeClient) subscriptionItems(ctx context.Context, workspaceID string, sub *stripe.Subscription) ([]*stripe.SubscriptionItem, error) {
	if sub.Items != nil && !sub.Items.HasMore {
		return sub.Items.Data, nil
	}
	params := &stripe.SubscriptionItemListParams{
		ListParams:   stripe.ListParams{Limit: stripe.Int64(100)},
		Subscription: stripe.String(sub.ID),
	}
	params.Context = ctx
	iter := c.sc.SubscriptionItems.List(params)
	var items []*stripe.SubscriptionItem
	for iter.Next() {
		items = append(items, iter.SubscriptionItem())
	}
	if err := iter.Err(); err != nil {
		return nil, fmt.Errorf("stripe: list items of subscription %s for %s: %w", sub.ID, workspaceID, err)
	}
	return items, nil
}

// AgentModelInvoice is the agent-model share of one invoice.
type AgentModelInvoice struct {
	// NetCents is the agent-model lines' amount after their discounts.
	NetCents int64
	// UsageMonth is the start of the first agent-model line's period (zero
	// when the invoice has none).
	UsageMonth time.Time
}

// AgentModelInvoiceLines sums the invoice's lines that rate the agent-model
// meter, net of their discounts. Lines are re-read from Stripe: a webhook
// payload carries only the first page of a many-item invoice.
func (c *StripeClient) AgentModelInvoiceLines(ctx context.Context, invoiceID string) (AgentModelInvoice, error) {
	if _, err := c.resolvePriceIDs(ctx); err != nil {
		return AgentModelInvoice{}, err
	}
	agentPrice, agentMeter := c.agentModelMeter()
	if agentMeter == "" {
		return AgentModelInvoice{}, nil
	}
	params := &stripe.InvoiceListLinesParams{Invoice: stripe.String(invoiceID)}
	params.Context = ctx
	params.Limit = stripe.Int64(100)
	params.AddExpand("data.pricing.price_details.price")
	iter := c.sc.Invoices.ListLines(params)
	var out AgentModelInvoice
	for iter.Next() {
		line := iter.InvoiceLineItem()
		if line.Pricing == nil || line.Pricing.PriceDetails == nil || line.Pricing.PriceDetails.Price == nil {
			continue
		}
		price := line.Pricing.PriceDetails.Price
		if price.ID != agentPrice && (price.Recurring == nil || price.Recurring.Meter != agentMeter) {
			continue
		}
		out.NetCents += line.Amount
		for _, d := range line.DiscountAmounts {
			if d != nil {
				out.NetCents -= d.Amount
			}
		}
		if out.UsageMonth.IsZero() && line.Period != nil && line.Period.Start > 0 {
			out.UsageMonth = time.Unix(line.Period.Start, 0).UTC()
		}
	}
	if err := iter.Err(); err != nil {
		code, _ := safeStripeError(err)
		return AgentModelInvoice{}, fmt.Errorf("stripe: list lines of invoice %s: %s", invoiceID, code)
	}
	return out, nil
}

// FinalizeThresholdInvoice finalizes a draft threshold invoice with
// auto_advance, so Stripe collects it now instead of after its one-hour draft
// window (probe 2026-11-05). An invoice no longer in draft is a no-op.
func (c *StripeClient) FinalizeThresholdInvoice(ctx context.Context, invoiceID string) error {
	if _, err := c.finalizeDraftInvoice(ctx, invoiceID, "bex-finalize-threshold-"+invoiceID, stripe.Bool(true)); err != nil {
		c.metrics.Operation("threshold_invoice_finalize", "error")
		return fmt.Errorf("stripe: finalize threshold invoice %s: %w", invoiceID, err)
	}
	c.metrics.Operation("threshold_invoice_finalize", "success")
	return nil
}

// ThresholdStore is AgentModelThresholds' store surface; *store.PGStore
// satisfies it.
type ThresholdStore interface {
	GetBillingTrust(ctx context.Context, workspaceID string) (store.BillingTrust, error)
	AgentModelThresholdsDue(ctx context.Context, now time.Time, limit int) ([]string, error)
	WithAgentModelThresholdSync(ctx context.Context, workspaceID string, now time.Time, apply func(store.AgentModelThresholdState) (*int64, error)) error
	RunExclusive(ctx context.Context, name string, fn func(context.Context) error) (bool, error)
	RefreshAgentModelRevenue(ctx context.Context, month time.Time) error
	AgentModelBadDebtForMonth(ctx context.Context, month time.Time) (store.AgentModelBadDebt, error)
}

// ThresholdSetter is the Stripe side; *StripeClient satisfies it.
type ThresholdSetter interface {
	SyncAgentModelThreshold(ctx context.Context, workspaceID string, want *int64) error
}

// AgentModelThresholds keeps every workspace's Stripe agent-model threshold
// equal to its trust tier's charge threshold. A webhook tier change syncs at
// once (OnTierChanged); a periodic sweep (Run) syncs every workspace whose
// trust record changed, or whose payment lock expired, since the value last
// applied (store.AgentModelThresholdsDue), which covers the tier change no
// webhook announces and retries a failed sync with per-workspace backoff. One
// replica sweeps at a time. Each pass also snapshots the month's agent-model
// revenue and exports the bad-debt gauges (the D10 prepaid trigger).
type AgentModelThresholds struct {
	Store    ThresholdStore
	Stripe   ThresholdSetter
	Sheet    *pricing.Sheet // nil ⇒ pricing.Default
	Interval time.Duration
	Metrics  *Metrics
	Clock    func() time.Time

	// closedMonth is the last closed month whose revenue this process
	// snapshotted after it closed.
	closedMonth time.Time
}

// thresholdSweepLimit bounds one pass; the rest are due next pass.
const thresholdSweepLimit = 500

func (a *AgentModelThresholds) now() time.Time {
	if a.Clock != nil {
		return a.Clock().UTC()
	}
	return time.Now().UTC()
}

func (a *AgentModelThresholds) sheet() *pricing.Sheet {
	if a.Sheet != nil {
		return a.Sheet
	}
	return pricing.Default
}

// TierThresholdMicroUSD is tier's agent-model charge threshold in micro-USD
// (the meter's unit); nil when the tier has none (T4).
func TierThresholdMicroUSD(tier pricing.Tier) *int64 {
	if tier.ChargeThresholdUSD == nil {
		return nil
	}
	micro := pricing.USDToMicro(*tier.ChargeThresholdUSD)
	return &micro
}

func (a *AgentModelThresholds) threshold(trust store.BillingTrust, now time.Time) *int64 {
	return TierThresholdMicroUSD(a.sheet().TrustTier(trust.LifetimePaidCents, trust.PaidCycleInvoices, trust.Lock(), now))
}

// Want is workspaceID's threshold now: StripeConfig.AgentModelThreshold.
func (a *AgentModelThresholds) Want(ctx context.Context, workspaceID string) (*int64, error) {
	trust, err := a.Store.GetBillingTrust(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	return a.threshold(trust, a.now()), nil
}

// sync brings workspaceID's Stripe threshold to its tier at now and records
// the value applied. Stripe is not called when the recorded value already
// equals the tier's.
func (a *AgentModelThresholds) sync(ctx context.Context, workspaceID string, now time.Time) error {
	return a.Store.WithAgentModelThresholdSync(ctx, workspaceID, now, func(state store.AgentModelThresholdState) (*int64, error) {
		want := a.threshold(state.Trust, now)
		if state.Synced && sameThreshold(state.SyncedMicroUSD, want) {
			return want, nil
		}
		return want, a.Stripe.SyncAgentModelThreshold(ctx, workspaceID, want)
	})
}

func sameThreshold(a, b *int64) bool {
	return (a == nil) == (b == nil) && (a == nil || *a == *b)
}

// OnTierChanged is Trust.OnTierChanged: the threshold follows the new tier at
// once. A failure is logged; the sweep retries it after its backoff.
func (a *AgentModelThresholds) OnTierChanged(ctx context.Context, workspaceID string, tier pricing.Tier) {
	log.Printf("billing trust: workspace %s is now agent-model trust tier %s", workspaceID, tier.Name)
	if err := a.sync(ctx, workspaceID, a.now()); err != nil {
		log.Printf("billing: agent-model threshold sync for %s failed (the sweep retries): %v", workspaceID, err)
	}
}

// Run sweeps every Interval until ctx ends.
func (a *AgentModelThresholds) Run(ctx context.Context) {
	interval := a.Interval
	if interval <= 0 {
		interval = 5 * time.Minute
	}
	core.Poll(ctx, "agent-model threshold sweep", interval, a.RunOnce)
}

// RunOnce is one pass: the sweep on the one replica holding its lock, then
// every replica's bad-debt gauges (read from the snapshot, so cheap).
func (a *AgentModelThresholds) RunOnce(ctx context.Context) error {
	now := a.now()
	_, err := a.Store.RunExclusive(ctx, "billing:agent-model-threshold-sweep", func(ctx context.Context) error {
		return a.sweep(ctx, now)
	})
	a.exportBadDebt(ctx, now)
	return err
}

// sweep snapshots revenue, then syncs every due workspace; one workspace's
// failure (backed off in the store) never stops the others.
func (a *AgentModelThresholds) sweep(ctx context.Context, now time.Time) error {
	var errs []error
	if err := a.refreshRevenue(ctx, now); err != nil {
		errs = append(errs, err)
	}
	due, err := a.Store.AgentModelThresholdsDue(ctx, now, thresholdSweepLimit)
	if err != nil {
		return errors.Join(append(errs, err)...)
	}
	for _, ws := range due {
		if err := a.sync(ctx, ws, now); err != nil {
			errs = append(errs, fmt.Errorf("workspace %s: %w", ws, err))
		}
	}
	return errors.Join(errs...)
}

// refreshRevenue snapshots the current month's agent-model revenue every pass,
// and the previous (closed) month's until an hour into the new month, after
// which its last usage has landed and this process stops recomputing it.
func (a *AgentModelThresholds) refreshRevenue(ctx context.Context, now time.Time) error {
	thisMonth := store.MonthStartUTC(now)
	if err := a.Store.RefreshAgentModelRevenue(ctx, thisMonth); err != nil {
		return fmt.Errorf("agent-model revenue for %s: %w", thisMonth.Format("2006-01"), err)
	}
	closed := thisMonth.AddDate(0, -1, 0)
	if a.closedMonth.Equal(closed) {
		return nil
	}
	if err := a.Store.RefreshAgentModelRevenue(ctx, closed); err != nil {
		return fmt.Errorf("agent-model revenue for %s: %w", closed.Format("2006-01"), err)
	}
	if now.Sub(thisMonth) >= time.Hour {
		a.closedMonth = closed
	}
	return nil
}

// exportBadDebt sets the current and previous month's agent-model revenue and
// write-off gauges. Best-effort: a read failure is logged.
func (a *AgentModelThresholds) exportBadDebt(ctx context.Context, now time.Time) {
	if a.Metrics == nil {
		return
	}
	thisMonth := store.MonthStartUTC(now)
	for _, m := range []struct {
		label string
		month time.Time
	}{{"current", thisMonth}, {"previous", thisMonth.AddDate(0, -1, 0)}} {
		debt, err := a.Store.AgentModelBadDebtForMonth(ctx, m.month)
		if err != nil {
			log.Printf("billing: agent-model bad debt for the %s month: %v", m.label, err)
			continue
		}
		a.Metrics.SetAgentModelBadDebt(m.label, debt)
	}
}
