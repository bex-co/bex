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

package pricing

import (
	"fmt"
	"math"
	"slices"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

// Agent-model spend caps and trust tiers (ADR091 D5, D10.1). The plan default
// cap, the admin override bound and the trust ladder are all sheet data; the
// tier itself is a pure function of a workspace's payment counters, its lock
// and this table, so every caller (mint, admission, settings view, the Stripe
// threshold) reads one answer. Caps are held in micro-USD, the unit of
// agent_model_cost_microusd, so the mint compares integers.

// Tier is one rung of the D10.1 trust ladder.
type Tier struct {
	// Name is the display id ("T0".."T4"); Level is its index (T0 = 0).
	Name  string `json:"name"`
	Level int    `json:"level"`
	// MinLifetimePaidCents and MinPaidCycleInvoices are both required to
	// reach the tier (successful, non-zero bex invoices only).
	MinLifetimePaidCents int64 `json:"minLifetimePaidCents"`
	MinPaidCycleInvoices int   `json:"minPaidCycleInvoices"`
	// ChargeThresholdUSD is the agent-model charge threshold; nil means none
	// (month-end invoice only, T4).
	ChargeThresholdUSD *float64 `json:"chargeThresholdUsd"`
	// CapCeilingMicroUSD is the most a workspace spends at this tier while its
	// threshold charge is unpaid: the threshold plus the tier's in-flight
	// allowance (ADR091 D10.2a). Paying the charge moves the workspace up a
	// tier. nil means the plan (or override) cap applies unchanged.
	CapCeilingMicroUSD *int64 `json:"capCeilingMicroUsd"`
	// MaxConcurrentSessions caps live platform-model sessions; 0 means the
	// tier adds no limit.
	MaxConcurrentSessions int `json:"maxConcurrentSessions"`
}

// EffectiveCapMicroUSD is min(base, the tier's cap ceiling): base is the
// plan default or the admin override.
func (t Tier) EffectiveCapMicroUSD(base int64) int64 {
	if t.CapCeilingMicroUSD != nil && *t.CapCeilingMicroUSD < base {
		return *t.CapCeilingMicroUSD
	}
	return base
}

type tierFile struct {
	Tier                  string   `json:"tier"`
	MinLifetimePaidUSD    float64  `json:"minLifetimePaidUsd"`
	MinPaidCycleInvoices  int      `json:"minPaidCycleInvoices"`
	ChargeThresholdUSD    *float64 `json:"chargeThresholdUsd"`
	InFlightAllowanceUSD  *float64 `json:"inFlightAllowanceUsd"`
	MaxConcurrentSessions *int     `json:"maxConcurrentSessions"`
}

// USDToMicro converts USD to micro-USD (agent_model_cost_microusd's unit),
// rounded half away from zero.
func USDToMicro(usd float64) int64 { return int64(math.Round(usd * microUSDPerUSD)) }

// MicroToUSD converts micro-USD to USD.
func MicroToUSD(micro int64) float64 { return float64(micro) / microUSDPerUSD }

// USDToCents converts USD to cents, rounded half away from zero.
func USDToCents(usd float64) int64 { return int64(math.Round(usd * centsPerUSD)) }

const centsPerUSD = 100

func positiveFinite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) && v > 0 }

func (s *Sheet) parseSpendControls(f sheetFile) error {
	am := f.AgentModel
	if !positiveFinite(am.MaxCapMultiplier) || am.MaxCapMultiplier < 1 {
		return fmt.Errorf("agentModel.maxCapMultiplier %v must be at least 1", am.MaxCapMultiplier)
	}
	s.agentModelMaxCapMultiplier = am.MaxCapMultiplier
	s.agentModelSpendCap = make(map[string]int64, len(am.MonthlySpendCapUSD))
	for _, e := range am.MonthlySpendCapUSD {
		if !slices.Contains(store.WorkspacePlans, e.Plan) {
			return fmt.Errorf("agentModel.monthlySpendCapUsd plan %q is not a workspace plan", e.Plan)
		}
		if _, dup := s.agentModelSpendCap[e.Plan]; dup {
			return fmt.Errorf("agentModel.monthlySpendCapUsd lists plan %s twice", e.Plan)
		}
		if !positiveFinite(e.USD) {
			return fmt.Errorf("agentModel.monthlySpendCapUsd plan %s needs a positive usd", e.Plan)
		}
		s.agentModelSpendCap[e.Plan] = USDToMicro(e.USD)
	}
	for _, plan := range []string{store.PlanHobby, store.PlanPro, store.PlanScale} {
		if _, ok := s.agentModelSpendCap[plan]; !ok {
			return fmt.Errorf("agentModel.monthlySpendCapUsd is missing plan %s", plan)
		}
	}
	if len(am.TrustTiers) == 0 {
		return fmt.Errorf("agentModel.trustTiers is required")
	}
	s.trustTiers = make([]Tier, 0, len(am.TrustTiers))
	for i, e := range am.TrustTiers {
		if e.Tier == "" || slices.ContainsFunc(s.trustTiers, func(t Tier) bool { return t.Name == e.Tier }) {
			return fmt.Errorf("agentModel.trustTiers[%d] needs a unique tier name", i)
		}
		if math.IsNaN(e.MinLifetimePaidUSD) || e.MinLifetimePaidUSD < 0 || e.MinPaidCycleInvoices < 0 {
			return fmt.Errorf("agentModel.trustTiers %s has a negative reach condition", e.Tier)
		}
		t := Tier{
			Name: e.Tier, Level: i,
			MinLifetimePaidCents: USDToCents(e.MinLifetimePaidUSD), MinPaidCycleInvoices: e.MinPaidCycleInvoices,
			ChargeThresholdUSD: e.ChargeThresholdUSD,
		}
		if e.InFlightAllowanceUSD != nil {
			a := *e.InFlightAllowanceUSD
			if math.IsNaN(a) || math.IsInf(a, 0) || a < 0 || e.ChargeThresholdUSD == nil {
				return fmt.Errorf("agentModel.trustTiers %s inFlightAllowanceUsd must be zero or positive, on a tier with a charge threshold", e.Tier)
			}
			ceiling := USDToMicro(*e.ChargeThresholdUSD + a)
			t.CapCeilingMicroUSD = &ceiling
		}
		if i == 0 && (t.MinLifetimePaidCents != 0 || t.MinPaidCycleInvoices != 0) {
			return fmt.Errorf("agentModel.trustTiers %s (the first tier) must need nothing", e.Tier)
		}
		if i > 0 {
			prev := s.trustTiers[i-1]
			if t.MinLifetimePaidCents < prev.MinLifetimePaidCents || t.MinPaidCycleInvoices < prev.MinPaidCycleInvoices ||
				(t.MinLifetimePaidCents == prev.MinLifetimePaidCents && t.MinPaidCycleInvoices == prev.MinPaidCycleInvoices) {
				return fmt.Errorf("agentModel.trustTiers %s must need more than %s", e.Tier, prev.Name)
			}
		}
		if e.ChargeThresholdUSD != nil && !positiveFinite(*e.ChargeThresholdUSD) {
			return fmt.Errorf("agentModel.trustTiers %s chargeThresholdUsd must be positive or null", e.Tier)
		}
		if e.MaxConcurrentSessions != nil {
			if *e.MaxConcurrentSessions <= 0 {
				return fmt.Errorf("agentModel.trustTiers %s maxConcurrentSessions must be positive or null", e.Tier)
			}
			t.MaxConcurrentSessions = *e.MaxConcurrentSessions
		}
		s.trustTiers = append(s.trustTiers, t)
	}
	return nil
}

// AgentModelPlanSpendCapMicroUSD is plan's default monthly agent-model spend
// cap (ADR091 D5). Enterprise and unknown plans take the highest listed cap,
// the same "unknown is the top tier, never a silent downgrade" rule as
// store.LimitsFor.
func (s *Sheet) AgentModelPlanSpendCapMicroUSD(plan string) int64 {
	if s == nil {
		return 0
	}
	if micro, ok := s.agentModelSpendCap[plan]; ok {
		return micro
	}
	var top int64
	for _, micro := range s.agentModelSpendCap {
		top = max(top, micro)
	}
	return top
}

// AgentModelMaxCapMicroUSD is the highest cap an admin may set for plan:
// maxCapMultiplier × the plan default.
func (s *Sheet) AgentModelMaxCapMicroUSD(plan string) int64 {
	if s == nil {
		return 0
	}
	return int64(math.Round(float64(s.AgentModelPlanSpendCapMicroUSD(plan)) * s.agentModelMaxCapMultiplier))
}

// AgentModelMaxCapMultiplier is the override bound as a multiple of the plan
// default (the "10x" in the settings copy).
func (s *Sheet) AgentModelMaxCapMultiplier() float64 {
	if s == nil {
		return 0
	}
	return s.agentModelMaxCapMultiplier
}

// TrustTiers is the ladder, T0 first (a copy).
func (s *Sheet) TrustTiers() []Tier {
	if s == nil {
		return nil
	}
	return slices.Clone(s.trustTiers)
}

// TrustTier is the tier a workspace's counters reach: the highest rung whose
// lifetime-paid AND paid-cycle-invoice minimums both hold. A lock (a failed
// charge or dispute, held until 30 days after settlement) pins it to the
// first rung while now is before lockedUntil.
func (s *Sheet) TrustTier(lifetimePaidCents int64, paidCycleInvoices int, lockedUntil, now time.Time) Tier {
	if s == nil || len(s.trustTiers) == 0 {
		return Tier{}
	}
	if now.Before(lockedUntil) {
		return s.trustTiers[0]
	}
	tier := s.trustTiers[0]
	for _, t := range s.trustTiers[1:] {
		if lifetimePaidCents >= t.MinLifetimePaidCents && paidCycleInvoices >= t.MinPaidCycleInvoices {
			tier = t
		}
	}
	return tier
}

// TrustTier is Default.TrustTier: the production ladder.
func TrustTier(lifetimePaidCents int64, paidCycleInvoices int, lockedUntil, now time.Time) Tier {
	return Default.TrustTier(lifetimePaidCents, paidCycleInvoices, lockedUntil, now)
}
