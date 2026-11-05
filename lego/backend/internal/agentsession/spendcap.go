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

package agentsession

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"sync"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/pricing"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// Monthly agent-model spend cap (ADR091 D5) and trust tier (D10.1). The mint
// compares the workspace's month-to-date agent_model_cost_microusd (customer
// price) with its effective cap, min(admin override or plan default, the
// tier's cap ceiling), before releasing the platform key. BYO-key exchanges
// are the tenant's own spend and are never capped.

// ModelSpendCapReachedCode is the stable code of a spend-cap refusal: the
// session's failureReasonCode and the gateway's error body both carry it.
const ModelSpendCapReachedCode = "AGENT_MODEL_SPEND_CAP_REACHED"

// The terminal failureReason of a session refused at the spend cap, by cause:
// the workspace's own monthly cap, the new-workspace (trust tier) ceiling, or
// the hold after a failed or disputed payment.
const (
	ModelSpendCapReachedReason   = "this workspace reached its monthly agent model spend cap (" + ModelSpendCapReachedCode + "); a workspace admin can raise the cap in Settings → Agents, or wait for it to reset on the 1st (UTC), then start a new session"
	modelSpendNewWorkspaceReason = "this workspace reached the agent model spend limit for new workspaces (" + ModelSpendCapReachedCode + "); the limit rises after the workspace's first successful payment, or resets on the 1st (UTC)"
	modelSpendLockedReason       = "this workspace's agent model spend is limited after a failed or disputed payment (" + ModelSpendCapReachedCode + "); settle the outstanding payment in Settings → Billing, then start a new session"
)

// ModelFailureCauseSpendCapReached is ModelMintRequest.Cause on a report that a
// mint was refused at the spend cap. Empty Cause is the original auth-failure
// report.
const ModelFailureCauseSpendCapReached = "spend_cap_reached"

// RefusalModelSpendCapReached is RefusalHeader's value for
// ErrModelSpendCapReached.
const RefusalModelSpendCapReached = "model-spend-cap-reached"

// ErrModelSpendCapReached refuses a platform-route mint once the workspace's
// month-to-date agent-model spend reaches its effective cap. It deliberately
// does NOT wrap ErrForbidden: it is a standing state, never the create-time
// race the gateway retries, and it travels as HTTP 429.
var ErrModelSpendCapReached = errors.New("agent model spend cap reached")

// BillingBalanceDueCode is the stable code of the refusal while the workspace
// has an unpaid failed invoice (ADR091 D10.3): model spend is cash out, so it
// gets no ADR040 §7 grace period.
const BillingBalanceDueCode = "BILLING_BALANCE_DUE"

// BillingBalanceDueReason is the failureReason of a session refused with
// BillingBalanceDueCode.
const BillingBalanceDueReason = "a payment for this workspace failed; update the payment method or pay the outstanding invoice, then start a new session"

// ModelFailureCauseBillingBalanceDue is ModelMintRequest.Cause on a report
// that a mint was refused for an unpaid failed invoice.
const ModelFailureCauseBillingBalanceDue = "billing_balance_due"

// RefusalBillingBalanceDue is RefusalHeader's value for ErrBillingBalanceDue.
const RefusalBillingBalanceDue = "billing-balance-due"

// ErrBillingBalanceDue refuses a platform-route mint while the workspace has
// an unpaid failed invoice. Like ErrModelSpendCapReached it is a standing
// state, travels as HTTP 429, and never wraps ErrForbidden.
var ErrBillingBalanceDue = errors.New("billing balance due")

// SpendStore is the store surface of the spend status; *store.PGStore
// satisfies it.
type SpendStore interface {
	AgentSpendInputs(ctx context.Context, workspaceID string, now time.Time) (store.AgentSpendInputs, error)
}

// SpendStatus is one workspace's agent-model spend position. Every amount is
// micro-USD.
type SpendStatus struct {
	Plan string
	Tier pricing.Tier
	// Locked is true while a failed or disputed payment holds the workspace
	// at the first tier.
	Locked bool
	// BalanceDue is true while a failed invoice is open or a written-off one
	// unpaid (store.AgentSpendInputs.BalanceDue): the mint refuses the
	// platform route with BillingBalanceDueCode until each is paid or voided.
	BalanceDue bool
	// PlanCap is the plan default; MaxCap the highest override an admin may
	// set; Override the admin's override as stored (nil when unset).
	PlanCap  int64
	MaxCap   int64
	Override *int64
	// EffectiveCap = min(the override clamped to MaxCap, or the plan default;
	// the tier's cap ceiling). TierLimited is true when the ceiling is what
	// binds.
	EffectiveCap int64
	TierLimited  bool
	MonthToDate  int64
	// ModelKeySet: the workspace set its own model key, so the platform route
	// (and this cap) does not apply to it.
	ModelKeySet bool
}

// Reached reports whether month-to-date spend is at or over the cap.
func (s SpendStatus) Reached() bool { return s.MonthToDate >= s.EffectiveCap }

// SpendCapReachedReason is the readable failureReason for a session refused
// at s's cap: the hold after a failed payment, the new-workspace ceiling, or
// the workspace's own monthly cap.
func (s SpendStatus) SpendCapReachedReason() string {
	switch {
	case s.Locked:
		return modelSpendLockedReason
	case s.TierLimited:
		return modelSpendNewWorkspaceReason
	}
	return ModelSpendCapReachedReason
}

// SpendStatusOf resolves the spend position in from sheet (nil ⇒
// pricing.Default) at now. Pure: the store read is AgentSpendInputs.
func SpendStatusOf(sheet *pricing.Sheet, in store.AgentSpendInputs, now time.Time) SpendStatus {
	if sheet == nil {
		sheet = pricing.Default
	}
	lock := in.Trust.Lock()
	status := SpendStatus{
		Plan:        in.Plan,
		Tier:        sheet.TrustTier(in.Trust.LifetimePaidCents, in.Trust.PaidCycleInvoices, lock, now),
		Locked:      now.Before(lock),
		BalanceDue:  in.BalanceDue,
		PlanCap:     sheet.AgentModelPlanSpendCapMicroUSD(in.Plan),
		MaxCap:      sheet.AgentModelMaxCapMicroUSD(in.Plan),
		MonthToDate: in.MonthToDateMicroUSD,
		ModelKeySet: in.ModelKeySet,
	}
	return status.WithOverride(in.OverrideMicroUSD)
}

// WithOverride is s with the admin override replaced and the effective cap
// re-derived, so a caller that just wrote the override need not re-read.
func (s SpendStatus) WithOverride(override *int64) SpendStatus {
	s.Override = override
	base := s.PlanCap
	if override != nil {
		// A plan downgrade can leave an override above the new plan's bound.
		base = min(*override, s.MaxCap)
	}
	s.EffectiveCap = s.Tier.EffectiveCapMicroUSD(base)
	s.TierLimited = s.EffectiveCap < base
	return s
}

// ReadSpendStatus reads workspaceID's spend position at now (one query).
func ReadSpendStatus(ctx context.Context, st SpendStore, sheet *pricing.Sheet, workspaceID string, now time.Time) (SpendStatus, error) {
	in, err := st.AgentSpendInputs(ctx, workspaceID, now)
	if err != nil {
		return SpendStatus{}, fmt.Errorf("agent model spend: %w", err)
	}
	return SpendStatusOf(sheet, in, now), nil
}

// spendStatusTTL bounds how stale the mint's view of a workspace's spend may
// be. The mint runs once per model exchange, so the cache keeps the hot path
// off the database at the cost of up to this much overshoot past the cap. A
// newly failed invoice (BalanceDue) likewise reaches a replica's mint within
// this TTL of the invoice.payment_failed webhook committing, and a payment
// lifts the refusal within the same bound.
const spendStatusTTL = 20 * time.Second

// spendCache is the mint's per-workspace SpendStatus cache. The settings
// view and admission never read it.
type spendCache struct {
	mu      sync.Mutex
	entries map[string]cachedSpend
}

type cachedSpend struct {
	status SpendStatus
	at     time.Time
}

// maxSpendCacheEntries bounds the cache; past it, expired entries are swept.
const maxSpendCacheEntries = 4096

func (c *spendCache) get(workspaceID string, now time.Time) (SpendStatus, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[workspaceID]
	if !ok || now.Sub(e.at) >= spendStatusTTL || now.Before(e.at) {
		return SpendStatus{}, false
	}
	return e.status, true
}

func (c *spendCache) put(workspaceID string, status SpendStatus, now time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.entries == nil {
		c.entries = map[string]cachedSpend{}
	}
	if len(c.entries) >= maxSpendCacheEntries {
		maps.DeleteFunc(c.entries, func(_ string, e cachedSpend) bool { return now.Sub(e.at) >= spendStatusTTL })
	}
	c.entries[workspaceID] = cachedSpend{status: status, at: now}
}

// errSpendStoreUnwired fails a platform-route mint closed when the minter has
// no spend store: without it the mint can neither enforce the cap nor tell
// whether the workspace set its own key.
var errSpendStoreUnwired = errors.New("agent model spend store unavailable")

// platformStatus is the mint's one store read for a workspace the platform
// route covers, through the per-workspace cache: the spend position (ADR091
// D5) and whether the workspace set its own model key (D7). A workspace admin
// writing or clearing the key on this replica drops the entry (ForgetSpend);
// other replicas see the change within spendStatusTTL. Unwired (nil Spend)
// fails closed.
func (m *ModelMinter) platformStatus(ctx context.Context, workspaceID string) (SpendStatus, error) {
	if m.Spend == nil {
		return SpendStatus{}, errSpendStoreUnwired
	}
	now := m.now()
	if status, ok := m.spend.get(workspaceID, now); ok {
		return status, nil
	}
	status, err := ReadSpendStatus(ctx, m.Spend, m.Sheet, workspaceID, now)
	if err != nil {
		return SpendStatus{}, err
	}
	m.spend.put(workspaceID, status, now)
	return status, nil
}

// ForgetSpend drops workspaceID's cached mint status, so this replica's next
// mint re-reads it: wired to the agent environment's model key writes.
func (m *ModelMinter) ForgetSpend(workspaceID string) {
	m.spend.mu.Lock()
	defer m.spend.mu.Unlock()
	delete(m.spend.entries, workspaceID)
}

// ReportSpendCapReached tells bex-api to fail the session whose platform-route
// mint was refused at the spend cap, with a SpendCapReachedReason. It is the
// auth-failure verb with a cause, so the same live-sandbox binding applies.
func (c *ModelClient) ReportSpendCapReached(ctx context.Context, req ModelMintRequest) error {
	req.Cause = ModelFailureCauseSpendCapReached
	return c.ReportAuthFailure(ctx, req)
}

// ReportBillingBalanceDue is ReportSpendCapReached for a mint refused with
// ErrBillingBalanceDue: the session fails with BillingBalanceDueReason.
func (c *ModelClient) ReportBillingBalanceDue(ctx context.Context, req ModelMintRequest) error {
	req.Cause = ModelFailureCauseBillingBalanceDue
	return c.ReportAuthFailure(ctx, req)
}
