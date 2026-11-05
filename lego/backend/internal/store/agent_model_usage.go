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

package store

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// Platform model route token meters (w10/m14 t003, ADR090 § Platform
// OpenRouter route). Each is a plain token count; the row's tier is the
// OpenRouter model id, so one model's tokens price separately from another's.
const (
	UsageKindAgentModelInputTokens      = "agent_model_input_tokens"
	UsageKindAgentModelOutputTokens     = "agent_model_output_tokens"
	UsageKindAgentModelCacheReadTokens  = "agent_model_cache_read_tokens"
	UsageKindAgentModelCacheWriteTokens = "agent_model_cache_write_tokens"
)

// UsageKindAgentModelCostMicroUSD is the one billed agent-model meter
// (ADR091 D1/D3): each platform-route request's customer price — upstream
// cost × pricing.yaml agentModel.markup — in micro-USD, rounded half-up per
// request. The row's tier is the model id (display breakdown only; the Stripe
// meter is model-agnostic).
const UsageKindAgentModelCostMicroUSD = "agent_model_cost_microusd"

// Cost sources of an agent model usage event (ADR091 D2).
const (
	// AgentModelCostReported is OpenRouter's own per-request usage.cost.
	AgentModelCostReported = "reported"
	// AgentModelCostComputed is a cost computed from the cached OpenRouter
	// /api/v1/models price table, overrides applied at request time.
	AgentModelCostComputed = "computed"
	// AgentModelCostUnpriced is neither: no reported cost and no price-table
	// entry for the model. The request is recorded at zero and logged.
	AgentModelCostUnpriced = "unpriced"
)

// AgentModelUsageKinds lists the four token meters in a stable order.
var AgentModelUsageKinds = []string{
	UsageKindAgentModelInputTokens, UsageKindAgentModelOutputTokens,
	UsageKindAgentModelCacheReadTokens, UsageKindAgentModelCacheWriteTokens,
}

// agentModelRecordedKinds is every usage_hourly kind one exchange writes: the
// token meters, then the cost meter, in RecordAgentModelUsage's quantity order.
var agentModelRecordedKinds = [...]string{
	UsageKindAgentModelInputTokens, UsageKindAgentModelOutputTokens,
	UsageKindAgentModelCacheReadTokens, UsageKindAgentModelCacheWriteTokens,
	UsageKindAgentModelCostMicroUSD,
}

// ResourceKindAgentSession is the usage resource kind of platform-route model
// tokens: service_id is the agent session (`ags-…`) that spent them.
const ResourceKindAgentSession = "agent_session"

// AgentModelUsage is one platform-route model exchange's token usage, as the
// gateway model proxy read it from the provider response.
type AgentModelUsage struct {
	// RequestID is the gateway-minted id of the exchange; it is the
	// idempotency key, so a retried report is recorded once.
	RequestID        string
	WorkspaceID      string
	SessionID        string
	Model            string
	At               time.Time
	InputTokens      int64
	OutputTokens     int64
	CacheReadTokens  int64
	CacheWriteTokens int64
	// UpstreamCostMicroUSD is the provider cost bex paid, half-up micro-USD.
	UpstreamCostMicroUSD int64
	// CostSource is AgentModelCostReported, AgentModelCostComputed or
	// AgentModelCostUnpriced.
	CostSource string
	// PriceMicroUSD is the customer price (cost × markup), half-up micro-USD;
	// a positive value is also metered as agent_model_cost_microusd.
	PriceMicroUSD int64
}

// RecordAgentModelUsage records one exchange and adds its tokens and price to
// the exchange's usage_hourly window, in one transaction. The ledger insert is
// the idempotency gate: recorded is false (and nothing is added) when the
// request id was already recorded.
func (s *PGStore) RecordAgentModelUsage(ctx context.Context, u AgentModelUsage) (recorded bool, err error) {
	window := u.At.UTC().Truncate(time.Hour)
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	tag, err := tx.Exec(ctx, `
		INSERT INTO agent_model_usage_events
			(request_id, workspace_id, session_id, model, window_start,
			 input_tokens, output_tokens, cache_read_tokens, cache_write_tokens,
			 upstream_cost_microusd, cost_source, price_microusd)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		ON CONFLICT (request_id) DO NOTHING`,
		u.RequestID, u.WorkspaceID, u.SessionID, u.Model, window,
		u.InputTokens, u.OutputTokens, u.CacheReadTokens, u.CacheWriteTokens,
		u.UpstreamCostMicroUSD, u.CostSource, u.PriceMicroUSD)
	if err != nil {
		return false, err
	}
	if tag.RowsAffected() == 0 {
		return false, nil
	}
	quantities := [len(agentModelRecordedKinds)]int64{u.InputTokens, u.OutputTokens, u.CacheReadTokens, u.CacheWriteTokens, u.PriceMicroUSD}
	for i, kind := range agentModelRecordedKinds {
		if quantities[i] <= 0 {
			continue
		}
		added, err := addAgentModelUsage(ctx, tx, u, kind, window, quantities[i])
		if err != nil {
			return false, err
		}
		if added {
			continue
		}
		// The record's hour was already exported (or an export attempt is in
		// flight, whose payload is fixed): adding to it would never be billed.
		// Land the quantity in the current hour instead, which is still open.
		late := time.Now().UTC().Truncate(time.Hour)
		if !late.After(window) {
			late = window.Add(time.Hour)
		}
		if added, err = addAgentModelUsage(ctx, tx, u, kind, late, quantities[i]); err != nil {
			return false, err
		}
		if !added {
			return false, fmt.Errorf("agent model usage %s: %s rows for %s and %s are both past export", u.RequestID, kind, window.Format(time.RFC3339), late.Format(time.RFC3339))
		}
	}
	return true, tx.Commit(ctx)
}

// addAgentModelUsage adds quantity to the exchange's kind row of window,
// creating it when absent. An existing row grows only while it is pending and
// unattempted; added is false when it is past that (exported, rejected, or an
// export attempt whose payload is already fixed).
func addAgentModelUsage(ctx context.Context, tx pgx.Tx, u AgentModelUsage, kind string, window time.Time, quantity int64) (added bool, err error) {
	tag, err := tx.Exec(ctx, `
		INSERT INTO usage_hourly (workspace_id, service_id, kind, tier, resource_kind, window_start, quantity)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (resource_kind, service_id, kind, tier, window_start)
		DO UPDATE SET quantity = usage_hourly.quantity + EXCLUDED.quantity
		WHERE usage_hourly.billing_export_state = 'pending'
		  AND usage_hourly.billing_export_attempted_at IS NULL`,
		u.WorkspaceID, u.SessionID, kind, u.Model, ResourceKindAgentSession, window, quantity)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

// agentSessionMonthToDateSQL sums workspace $1's agent_session meters of the
// kinds in $3 over the calendar month starting at $2, across hourly and
// compacted rows. resource_kind is a literal so the planner can use the
// partial usage_hourly_agent_session_idx.
const agentSessionMonthToDateSQL = `(SELECT COALESCE(SUM(quantity), 0)::bigint FROM (
		SELECT quantity FROM usage_hourly
		WHERE workspace_id = $1 AND resource_kind = '` + ResourceKindAgentSession + `' AND kind = ANY($3::text[])
		  AND window_start >= $2::timestamptz
		UNION ALL
		SELECT quantity FROM usage_monthly
		WHERE workspace_id = $1 AND resource_kind = '` + ResourceKindAgentSession + `' AND kind = ANY($3::text[])
		  AND month = ($2::timestamptz AT TIME ZONE 'UTC')::date
	) u)`

// AgentSpendInputs is everything one workspace's agent-model spend position
// (ADR091 D5, D10.1) is computed from.
type AgentSpendInputs struct {
	Plan  string
	Trust BillingTrust
	// OverrideMicroUSD is the admin's cap override; nil means the plan default.
	OverrideMicroUSD *int64
	// MonthToDateMicroUSD is agent_model_cost_microusd over the calendar month
	// (UTC) containing now.
	MonthToDateMicroUSD int64
	// ModelKeySet reports whether the workspace set its own model key, which
	// takes it off the platform route (AgentEnvironment.ModelKeySet).
	ModelKeySet bool
}

// AgentSpendInputs reads workspaceID's plan, trust record, cap override,
// month-to-date agent-model cost and BYO model key flag in one round trip (the
// mint's hot path).
// ErrNotFound when the workspace does not exist.
func (s *PGStore) AgentSpendInputs(ctx context.Context, workspaceID string, now time.Time) (AgentSpendInputs, error) {
	now = now.UTC()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	var in AgentSpendInputs
	var trust trustRow
	// The bt.* list is billingTrustColumns, in order.
	err := s.Pool.QueryRow(ctx, `
		SELECT t.plan, bt.lifetime_paid_cents, bt.paid_cycle_invoices, bt.locked_until, bt.failed_invoice_id,
		       bt.last_failure_at, bt.dispute_id, ae.agent_model_spend_cap_override_microusd,
		       `+agentSessionMonthToDateSQL+`, ae.model_key_set_at IS NOT NULL
		FROM tenants t
		LEFT JOIN billing_trust bt ON bt.workspace_id = t.id
		LEFT JOIN agent_environments ae ON ae.workspace_id = t.id
		WHERE t.id = $1`, workspaceID, monthStart, []string{UsageKindAgentModelCostMicroUSD},
	).Scan(append(append([]any{&in.Plan}, trust.dest()...), &in.OverrideMicroUSD, &in.MonthToDateMicroUSD, &in.ModelKeySet)...)
	if err != nil {
		return AgentSpendInputs{}, classify("workspace", err)
	}
	in.Trust = trust.record(workspaceID)
	return in, nil
}
