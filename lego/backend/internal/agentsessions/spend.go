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

package agentsessions

import (
	"context"
	"fmt"
	"log"
	"math"
	"strconv"
	"strings"

	"github.com/bex-co/bex/lego/backend/internal/agentsession"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/pricing"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// SpendStore is the store surface of the agent-model spend controls
// (ADR091 D5, D10.1); *store.PGStore satisfies it.
type SpendStore = agentsession.SpendStore

// TrustLimitCode refuses a session that would exceed the workspace trust
// tier's concurrent platform-model sessions (ADR091 D10.1; T0: 2).
const TrustLimitCode = "AGENT_SESSION_TRUST_LIMIT"

func (s *Service) sheet() *pricing.Sheet {
	if s.Sheet != nil {
		return s.Sheet
	}
	return pricing.Default
}

// trustAdmission is the trust tier's concurrent-session limit for a session
// of agent in workspaceID: zero when none applies (no spend store, a BYO-key
// session, a workspace off the platform route, or a tier without a limit).
// Only sessions whose credential is the platform route's key count: the
// route's agent profile in a workspace the route covers that has not set its
// own model key (byoKey). A store error fails closed.
type trustAdmission struct {
	limit  int
	status agentsession.SpendStatus
	count  store.AgentSessionCount
}

func (s *Service) trustAdmissionFor(ctx context.Context, workspaceID, agent string, byoKey bool) (trustAdmission, error) {
	if !s.platformCounted(workspaceID, agent, byoKey) {
		return trustAdmission{}, nil
	}
	status, err := agentsession.ReadSpendStatus(ctx, s.Spend, s.sheet(), workspaceID, s.Now().UTC())
	if err != nil {
		log.Printf("agent-session trust limit: read spend status failed (workspace=%s): %v", workspaceID, err)
		return trustAdmission{}, core.NewAgentSessionDependencyUnavailableError()
	}
	return trustAdmission{
		limit: status.Tier.MaxConcurrentSessions, status: status,
		count: store.AgentSessionCount{Agent: strings.ToLower(strings.TrimSpace(agent))},
	}, nil
}

// refusal is the 409 for a session beyond the limit, worded by cause: a new
// workspace before its first payment, or one held back by a failed or
// disputed payment.
func (a trustAdmission) refusal(live int) error {
	why := "until its first successful payment"
	if a.status.Locked {
		why = "while a payment is failed or disputed; settle the outstanding payment in Settings → Billing to lift the limit"
	}
	return core.NewConflictError(TrustLimitCode,
		fmt.Sprintf("this workspace can run %d agent sessions on the platform model at once %s (trust tier %s); wait for a running session to finish or stop one, then try again", a.limit, why, a.status.Tier.Name),
		map[string]any{"live": live, "limit": a.limit, "trustTier": a.status.Tier.Name, "locked": a.status.Locked})
}

// MetaRouteCapacityCode refuses a platform-only (muse) session while the
// platform-wide limit of live sessions on the shared Meta key is reached.
const MetaRouteCapacityCode = "AGENT_SESSION_AGENT_AT_CAPACITY"

// DefaultMetaRouteMaxSessions keeps the shared Meta key's live sessions
// within muse-spark-1.3-contributor's 150 requests per minute per key, at the
// ~5–20 requests per minute one busy session makes (ADR108).
const DefaultMetaRouteMaxSessions = 8

// enforceMetaRouteCapacity refuses starting, steering or resuming a session
// of a platform-only agent (excludeID: the session being reactivated) when
// the platform-wide live-session limit is reached. A best-effort pre-check:
// a burst past it meets Meta's own 429, which fails the turn readably.
func (s *Service) enforceMetaRouteCapacity(ctx context.Context, agent, excludeID string) error {
	if !agentsession.OnMetaRoute(agent) {
		return nil
	}
	profile, _ := agentsession.LookupAgentProfile(agent)
	limit := s.MetaRouteMaxSessions
	if limit <= 0 {
		limit = DefaultMetaRouteMaxSessions
	}
	n, err := s.Store.CountActiveAgentSessions(ctx, "", store.AgentSessionCount{
		AllWorkspaces: true, Agent: profile.ID, ExcludeID: excludeID,
	})
	if err != nil {
		return mapStoreError("", err)
	}
	if n >= limit {
		return core.NewConflictError(MetaRouteCapacityCode,
			fmt.Sprintf("the %s agent is at its platform capacity of %d running sessions; try again when one finishes", profile.ID, limit),
			map[string]any{"agent": profile.ID, "live": n, "limit": limit})
	}
	return nil
}

// enforceTrustConcurrency refuses a steer or resume of sessionID that would
// put the workspace over its trust tier's concurrent platform-model
// sessions. The session itself never counts. Unlike create (which takes the
// count and insert atomically), this is a best-effort pre-check.
func (s *Service) enforceTrustConcurrency(ctx context.Context, workspaceID, sessionID, agent string, byoKey bool) error {
	admission, err := s.trustAdmissionFor(ctx, workspaceID, agent, byoKey)
	if err != nil || admission.limit == 0 {
		return err
	}
	admission.count.ExcludeID = sessionID
	n, err := s.Store.CountActiveAgentSessions(ctx, workspaceID, admission.count)
	if err != nil {
		return mapStoreError("", err)
	}
	if n >= admission.limit {
		return admission.refusal(n)
	}
	return nil
}

// validateSpendCap applies the spend-cap part of an environment update
// (ADR091 D5): 0 < cap ≤ maxCapMultiplier × the plan default, or null to
// return to the plan default (resetAgentModelMonthlySpendCap does the same).
// The caller already holds can_manage and has read status.
func (s *Service) validateSpendCap(status agentsession.SpendStatus, input EnvironmentInput, patch *store.AgentEnvironmentPatch) error {
	if input.ResetAgentModelMonthlySpendCap {
		if input.AgentModelMonthlySpendCapUSD != nil {
			return core.NewBadRequestError(SpendCapInvalidCode,
				"set agentModelMonthlySpendCapUsd or resetAgentModelMonthlySpendCap, not both", map[string]any{"field": spendCapField})
		}
		patch.ClearSpendCapOverride = true
		return nil
	}
	if !input.AgentModelMonthlySpendCapSet && input.AgentModelMonthlySpendCapUSD == nil {
		return nil
	}
	if input.AgentModelMonthlySpendCapUSD == nil {
		patch.ClearSpendCapOverride = true
		return nil
	}
	usd := *input.AgentModelMonthlySpendCapUSD
	micro := pricing.USDToMicro(usd)
	if math.IsNaN(usd) || math.IsInf(usd, 0) || usd <= 0 || micro <= 0 || micro > status.MaxCap {
		maxUSD := pricing.MicroToUSD(status.MaxCap)
		return core.NewBadRequestError(SpendCapInvalidCode,
			fmt.Sprintf("agentModelMonthlySpendCapUsd must be more than $0 and at most $%s (%sx the plan default)",
				strconv.FormatFloat(maxUSD, 'f', -1, 64), strconv.FormatFloat(s.sheet().AgentModelMaxCapMultiplier(), 'f', -1, 64)),
			map[string]any{"field": spendCapField, "maxUsd": maxUSD, "planCapUsd": pricing.MicroToUSD(status.PlanCap)})
	}
	patch.SpendCapOverrideMicroUSD = &micro
	return nil
}

// SpendCapInvalidCode refuses a spend-cap override outside (0, 10× plan].
const SpendCapInvalidCode = "AGENT_ENVIRONMENT_SPEND_CAP_INVALID"

// readSpend reads the workspace's spend position for the environment view
// (never the mint's cache). A read failure fails the caller closed.
func (s *Service) readSpend(ctx context.Context, workspaceID string) (agentsession.SpendStatus, error) {
	status, err := agentsession.ReadSpendStatus(ctx, s.Spend, s.sheet(), workspaceID, s.Now().UTC())
	if err != nil {
		log.Printf("agent environment: read spend status failed (workspace=%s): %v", workspaceID, err)
		return agentsession.SpendStatus{}, core.NewAgentSessionDependencyUnavailableError()
	}
	return status, nil
}

// withSpend fills the environment view's spend-control fields from status.
func withSpend(view Environment, status agentsession.SpendStatus) Environment {
	usd := func(micro int64) *float64 { v := pricing.MicroToUSD(micro); return &v }
	view.AgentModelMonthlySpendCapUSD = usd(status.EffectiveCap)
	view.AgentModelPlanSpendCapUSD = usd(status.PlanCap)
	view.AgentModelMaxSpendCapUSD = usd(status.MaxCap)
	if status.Override != nil {
		view.AgentModelSpendCapOverrideUSD = usd(*status.Override)
	}
	view.AgentModelSpendMonthToDateUSD = usd(status.MonthToDate)
	tier := status.Tier.Name
	view.TrustTier = &tier
	locked, tierLimited, balanceDue := status.Locked, status.TierLimited, status.BalanceDue
	reached, charges := status.RefusesAtCap(), status.ChargesAtThreshold()
	view.TrustTierLocked = &locked
	view.AgentModelSpendCapTierLimited = &tierLimited
	view.BillingBalanceDue = &balanceDue
	view.AgentModelSpendCapReached = &reached
	view.AgentModelChargesAtThreshold = &charges
	if t := status.Tier.ChargeThresholdUSD; t != nil {
		threshold := *t
		view.NextChargeThresholdUSD = &threshold
	}
	return view
}

// platformCounted reports whether agent's sessions in workspaceID count
// toward the trust tier: a bex-owned route serves them (byoKey: the
// workspace set its own key, which takes the claude profile off its route)
// and the spend store is wired.
func (s *Service) platformCounted(workspaceID, agent string, byoKey bool) bool {
	return s.Spend != nil && agentsession.PlatformRouteMeters(workspaceID, agent, byoKey)
}
