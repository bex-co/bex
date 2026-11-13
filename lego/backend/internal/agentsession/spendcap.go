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
	"log"
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
// the workspace's own monthly cap, the new-workspace (trust tier) ceiling, the
// hold after a failed or disputed payment, or a threshold charge the payment
// provider has not decided yet (ADR091 D10.2a).
const (
	ModelSpendCapReachedReason    = "this workspace reached its monthly agent model spend cap (" + ModelSpendCapReachedCode + "); a workspace admin can raise the cap in Settings → Agents, or wait for it to reset on the 1st (UTC), then start a new session"
	modelSpendNewWorkspaceReason  = "this workspace's first agent model charge is not paid yet (" + ModelSpendCapReachedCode + "); a new workspace is charged when its spend reaches the charge threshold, and its limit rises once that charge is paid — check the card in Settings → Billing, then start a new session"
	modelSpendLockedReason        = "this workspace's agent model spend is limited after a failed or disputed payment (" + ModelSpendCapReachedCode + "); settle the outstanding payment in Settings → Billing, then start a new session"
	modelSpendChargePendingReason = "this workspace's agent model charge has not gone through yet (" + ModelSpendCapReachedCode + "); bex charges the card when spend reaches the charge threshold and retries a charge the payment provider could not complete — start a new session in a few minutes, or check the card in Settings → Billing"
)

// ModelSpendScopeSession is the scope of a refusal at the session's own
// maxSpendUSD ceiling (ADR094 D8), as the gateway's error body names it.
// The workspace's monthly cap carries no scope.
const ModelSpendScopeSession = "session"

// ModelSessionSpendCapReachedReason is the terminal failureReason of a session
// stopped at its own maxSpendUSD ceiling; it carries the same stable code as
// the monthly cap, with the session scope named in the text.
const ModelSessionSpendCapReachedReason = "this session reached its maxSpendUSD ceiling (" + ModelSpendCapReachedCode + ", scope session); start a new session or send a follow-up with a higher ceiling"

// ModelFailureCauseSessionSpendCapReached is ModelMintRequest.Cause on a
// report that a mint was refused at the session's ceiling.
const ModelFailureCauseSessionSpendCapReached = "session_spend_cap_reached"

// RefusalModelSessionSpendCapReached is RefusalHeader's value for
// ErrModelSessionSpendCapReached.
const RefusalModelSessionSpendCapReached = "model-session-spend-cap-reached"

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

// ErrModelSessionSpendCapReached refuses a platform-route mint once the
// session's recorded model price reaches its own maxSpendUSD ceiling. It is
// an ErrModelSpendCapReached (same code, same 429) with the session scope.
var ErrModelSessionSpendCapReached = fmt.Errorf("session ceiling: %w", ErrModelSpendCapReached)

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

// SessionSpendStore reads what one session has spent on the platform route;
// *store.PGStore satisfies it.
type SessionSpendStore interface {
	AgentSessionModelSpendMicroUSD(ctx context.Context, workspaceID, sessionID string) (int64, error)
}

// SessionTreeSpendStore reads the spend headroom of a session's tree (ADR102
// D3): what the tightest maxSpendUSD ceiling on the session and its ancestors
// leaves after the spend of the session and everything below the ceiling's
// owner. A SessionSpendStore that also implements it gets the tree check;
// *store.PGStore does.
type SessionTreeSpendStore interface {
	AgentSessionCeilingHeadroom(ctx context.Context, workspaceID, sessionID string) (headroom int64, bounded bool, err error)
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
	// OwnCap is the workspace's own monthly cap: the override clamped to
	// MaxCap, or the plan default. EffectiveCap = min(OwnCap, the tier's cap
	// ceiling); TierLimited is true when the ceiling is what binds.
	OwnCap       int64
	EffectiveCap int64
	TierLimited  bool
	MonthToDate  int64
	// WindowSpend is the spend in the charge window: the month's hours since
	// a subscription invoice last billed the meter
	// (store.AgentSpendInputs.UnbilledFrom).
	WindowSpend int64
	// ModelKeySet: the workspace set its own model key, so the platform route
	// (and this cap) does not apply to it.
	ModelKeySet bool
	// ThresholdMicroUSD is the tier's agent-model charge threshold (nil =
	// none, T4): bex charges the card in the request whose unbilled spend
	// reaches it (ADR091 D10.2a). ChargedMicroUSD is what the month's paid
	// charges collected, ChargeSeq the month's last charge, ChargePending a
	// charge in flight; Chargeable says a charge can be collected at all
	// (store.AgentSpendInputs).
	ThresholdMicroUSD *int64
	ChargedMicroUSD   int64
	ChargeSeq         int
	ChargePending     bool
	Chargeable        bool
}

// Reached reports whether month-to-date spend is at or over the workspace's
// own cap, or the unbilled spend at or over the tier's ceiling where one
// binds. The ceiling bounds what no invoice or charge has billed: spend a
// subscription invoice already billed this month is not held against it.
func (s SpendStatus) Reached() bool { return s.ownCapReached() || s.tierCeilingReached() }

func (s SpendStatus) ownCapReached() bool { return s.MonthToDate >= s.OwnCap }

func (s SpendStatus) tierCeilingReached() bool {
	return s.TierLimited && s.UnbilledMicroUSD() >= s.EffectiveCap
}

// UnbilledMicroUSD is the charge window's spend that the paid threshold
// charges made in it have not collected yet.
func (s SpendStatus) UnbilledMicroUSD() int64 { return max(s.WindowSpend-s.ChargedMicroUSD, 0) }

// ChargeDue reports that the unbilled spend reached the tier's charge
// threshold, so the mint charges the card before serving the request. A
// locked workspace (whose tier a payment would not lift), one with a balance
// due (refused anyway), one on its own model key, and one bex cannot charge
// (excluded, comped, or no Subscription) are never charged here: the cap
// alone stops them.
func (s SpendStatus) ChargeDue() bool {
	return !s.Locked && !s.BalanceDue && !s.ModelKeySet && s.Chargeable &&
		s.ThresholdMicroUSD != nil && s.UnbilledMicroUSD() >= *s.ThresholdMicroUSD
}

// SpendCapReachedReason is the readable failureReason for a session refused
// at s's cap: the hold after a failed payment, a threshold charge not made
// yet, the new-workspace ceiling, or the workspace's own monthly cap.
func (s SpendStatus) SpendCapReachedReason() string {
	switch {
	case s.Locked:
		return modelSpendLockedReason
	case s.ChargePending:
		return modelSpendChargePendingReason
	case s.TierLimited && !s.ownCapReached():
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
		Plan:            in.Plan,
		Tier:            sheet.TrustTier(in.Trust.LifetimePaidCents, in.Trust.PaidCycleInvoices, lock, now),
		Locked:          now.Before(lock),
		BalanceDue:      in.BalanceDue,
		PlanCap:         sheet.AgentModelPlanSpendCapMicroUSD(in.Plan),
		MaxCap:          sheet.AgentModelMaxCapMicroUSD(in.Plan),
		MonthToDate:     in.MonthToDateMicroUSD,
		WindowSpend:     in.WindowSpendMicroUSD,
		ModelKeySet:     in.ModelKeySet,
		ChargedMicroUSD: in.ChargedMicroUSD,
		ChargeSeq:       in.ChargeSeq,
		ChargePending:   in.ChargePending,
		Chargeable:      in.Chargeable,
	}
	if threshold := status.Tier.ChargeThresholdUSD; threshold != nil {
		micro := pricing.USDToMicro(*threshold)
		status.ThresholdMicroUSD = &micro
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
	s.OwnCap = base
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

// ThresholdCharger charges a workspace's unbilled agent-model spend once it
// reaches the tier's charge threshold (ADR091 D10.2a), and waits for the
// result, so the mint either serves the request without interruption or
// refuses it. bex-api adapts the billing charger to it (this package stays
// free of Stripe).
type ThresholdCharger interface {
	ChargeThreshold(ctx context.Context, charge ThresholdCharge) (ThresholdChargeOutcome, error)
}

// ThresholdCharge is one crossing, as the spend read it at At saw it.
type ThresholdCharge struct {
	WorkspaceID string
	At          time.Time
	// SeenSeq is the month's last charge in that read: a charge numbered past
	// it already covers this crossing, so the charger joins it.
	SeenSeq          int
	UnbilledMicroUSD int64
}

// ThresholdChargeOutcome is how a threshold charge ended.
type ThresholdChargeOutcome int

const (
	// ThresholdChargePending: the payment provider could not decide the
	// charge in time; it stays pending and is retried, and this request is
	// refused.
	ThresholdChargePending ThresholdChargeOutcome = iota
	// ThresholdChargePaid: the card paid; the workspace is one tier higher.
	ThresholdChargePaid
	// ThresholdChargeDeclined: the card was declined or needs
	// authentication; the workspace has a balance due.
	ThresholdChargeDeclined
	// ThresholdChargeNotDue: a subscription invoice billed the meter since
	// the spend read, so nothing was charged; the spend is read again.
	ThresholdChargeNotDue
)

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
	return m.readPlatformStatus(ctx, workspaceID, now)
}

// readPlatformStatus reads workspaceID's status at now past the cache and
// caches it.
func (m *ModelMinter) readPlatformStatus(ctx context.Context, workspaceID string, now time.Time) (SpendStatus, error) {
	status, err := ReadSpendStatus(ctx, m.Spend, m.Sheet, workspaceID, now)
	if err != nil {
		return SpendStatus{}, err
	}
	m.spend.put(workspaceID, status, now)
	return status, nil
}

// chargeThreshold makes the threshold charge status says is due and returns
// the status the mint continues with: re-read after a paid charge (the
// workspace is a tier higher, so the T0 ceiling no longer binds). A declined
// charge refuses with ErrBillingBalanceDue; one the payment provider could
// not decide, with ErrModelSpendCapReached. The cached view is confirmed on
// a fresh read first: another request or replica may have settled the
// charge since it was cached.
//
// A charge the charger finds not due (a subscription invoice billed the
// meter since the read) is re-read: the narrower window may still be due, and
// is charged then.
func (m *ModelMinter) chargeThreshold(ctx context.Context, workspaceID string) (SpendStatus, error) {
	for range 2 {
		now := m.now()
		status, err := m.readPlatformStatus(ctx, workspaceID, now)
		if err != nil {
			return SpendStatus{}, err
		}
		if status.BalanceDue {
			return SpendStatus{}, ErrBillingBalanceDue
		}
		if !status.ChargeDue() {
			return status, nil
		}
		outcome, err := m.Charges.ChargeThreshold(ctx, ThresholdCharge{
			WorkspaceID: workspaceID, At: now, SeenSeq: status.ChargeSeq, UnbilledMicroUSD: status.UnbilledMicroUSD(),
		})
		if err != nil {
			log.Printf("agent model threshold charge for %s not made: %v", workspaceID, err)
			return SpendStatus{}, ErrModelSpendCapReached
		}
		switch outcome {
		case ThresholdChargePaid:
			return m.readPlatformStatus(ctx, workspaceID, m.now())
		case ThresholdChargeNotDue:
			continue
		case ThresholdChargeDeclined:
			// The next mint re-reads, and sees the balance due.
			m.ForgetSpend(workspaceID)
			return SpendStatus{}, ErrBillingBalanceDue
		default:
			m.ForgetSpend(workspaceID)
			return SpendStatus{}, ErrModelSpendCapReached
		}
	}
	// The window moved twice in one request: serve on a fresh read, and let
	// the next mint charge whatever is still due.
	return m.readPlatformStatus(ctx, workspaceID, m.now())
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

// ReportSessionSpendCapReached is ReportSpendCapReached for a mint refused at
// the session's own ceiling: the session fails with
// ModelSessionSpendCapReachedReason.
func (c *ModelClient) ReportSessionSpendCapReached(ctx context.Context, req ModelMintRequest) error {
	req.Cause = ModelFailureCauseSessionSpendCapReached
	return c.ReportAuthFailure(ctx, req)
}
