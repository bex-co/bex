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
	"encoding/json"
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
	RequestID   string
	WorkspaceID string
	SessionID   string
	Model       string
	At          time.Time
	// StartedAt is when the gateway sent the exchange upstream, already
	// clamped by the caller; zero ⇒ unknown, and the per-turn read falls back
	// to the record time.
	StartedAt        time.Time
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
			 upstream_cost_microusd, cost_source, price_microusd, started_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		ON CONFLICT (request_id) DO NOTHING`,
		u.RequestID, u.WorkspaceID, u.SessionID, u.Model, window,
		u.InputTokens, u.OutputTokens, u.CacheReadTokens, u.CacheWriteTokens,
		u.UpstreamCostMicroUSD, u.CostSource, u.PriceMicroUSD, nullTime(u.StartedAt))
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
// compacted rows (the partial usage_hourly_agent_session_idx).
var agentSessionMonthToDateSQL = agentSessionUsageSumSQL("workspace_id = $1 AND ", "$2", "$3")

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
	// BalanceDue reports an open failed invoice (paid or voided closes it) or
	// an unpaid agent-model write-off (ADR091 D10.3).
	BalanceDue bool
	// ChargedMicroUSD is what the month's paid threshold charges already
	// collected (ADR091 D10.2a): month-to-date minus it is the spend not yet
	// billed. ChargeSeq is the month's last charge (0 = none), whatever its
	// status; ChargePending reports a charge in flight (in any month).
	ChargedMicroUSD int64
	ChargeSeq       int
	ChargePending   bool
	// Chargeable reports that a threshold charge can be collected: the
	// workspace has a Stripe Subscription and is neither excluded from
	// billing nor comped.
	Chargeable bool
}

// AgentSpendInputs reads workspaceID's plan, trust record, cap override,
// month-to-date agent-model cost, BYO model key flag, balance due and
// threshold charges in one round trip (the mint's hot path).
// ErrNotFound when the workspace does not exist.
func (s *PGStore) AgentSpendInputs(ctx context.Context, workspaceID string, now time.Time) (AgentSpendInputs, error) {
	var in AgentSpendInputs
	var trust trustRow
	// The bt.* list is billingTrustColumns, in order.
	err := s.Pool.QueryRow(ctx, `
		SELECT t.plan, bt.lifetime_paid_cents, bt.paid_cycle_invoices, bt.locked_until, bt.failed_invoice_id,
		       bt.last_failure_at, bt.dispute_id, ae.agent_model_spend_cap_override_microusd,
		       `+agentSessionMonthToDateSQL+`, ae.model_key_set_at IS NOT NULL,
		       EXISTS (SELECT 1 FROM billing_trust_failed_invoices f
		               WHERE f.workspace_id = t.id AND f.failed_at IS NOT NULL AND f.settled_at IS NULL)
		       OR EXISTS (SELECT 1 FROM billing_agent_model_writeoffs w WHERE w.workspace_id = t.id),
		       ch.charged, ch.seq,
		       EXISTS (SELECT 1 FROM agent_model_threshold_charges p WHERE p.workspace_id = t.id AND p.status = 'pending'),
		       NOT t.billing_excluded AND NOT t.billing_comped
		       AND EXISTS (SELECT 1 FROM billing_provider_mappings m WHERE m.workspace_id = t.id AND m.subscription_id IS NOT NULL)
		FROM tenants t
		LEFT JOIN billing_trust bt ON bt.workspace_id = t.id
		LEFT JOIN agent_environments ae ON ae.workspace_id = t.id
		CROSS JOIN LATERAL (
			SELECT COALESCE(SUM(c.amount_microusd) FILTER (WHERE c.status = 'paid'), 0)::bigint AS charged,
			       COALESCE(MAX(c.seq), 0) AS seq
			FROM agent_model_threshold_charges c
			WHERE c.workspace_id = t.id AND c.month = ($2::timestamptz AT TIME ZONE 'UTC')::date
		) ch
		WHERE t.id = $1`, workspaceID, MonthStartUTC(now), []string{UsageKindAgentModelCostMicroUSD},
	).Scan(append(append([]any{&in.Plan}, trust.dest()...), &in.OverrideMicroUSD, &in.MonthToDateMicroUSD, &in.ModelKeySet, &in.BalanceDue,
		&in.ChargedMicroUSD, &in.ChargeSeq, &in.ChargePending, &in.Chargeable)...)
	if err != nil {
		return AgentSpendInputs{}, classify("workspace", err)
	}
	in.Trust = trust.record(workspaceID)
	return in, nil
}

// AgentTurnModelUsage is one session turn's platform-route model usage
// (ADR091 D6): the requests started while the turn was the session's latest,
// summed. Turn is 0 for requests started before the session's first turn.
type AgentTurnModelUsage struct {
	Turn             int
	Requests         int64
	InputTokens      int64
	OutputTokens     int64
	CacheReadTokens  int64
	CacheWriteTokens int64
	// PriceMicroUSD is the customer price, agent_model_cost_microusd's unit.
	PriceMicroUSD int64
	// Models are the model ids the turn's requests used, sorted.
	Models []string
	// FirstAt and LastAt bound the turn's requests by start time.
	FirstAt time.Time
	LastAt  time.Time
}

// AgentSessionModelCost is one session's platform-route model usage per turn
// (ADR091 D6) with the session fields the read needs to label it.
type AgentSessionModelCost struct {
	WorkspaceID string
	// AgentConfig is the session's stored agent config: its modelEndpoint
	// says which model route the session was created on.
	AgentConfig json.RawMessage
	// Turns lists the turns that made requests, oldest first.
	Turns []AgentTurnModelUsage
}

// AgentSessionModelUsage reads sessionID and sums its agent_model_usage_events
// (in the session's own workspace) per turn, in one statement; ErrNotFound
// when the session does not exist. Turns run one at a time, so a request
// belongs to the turn that was the latest when the request started
// (started_at, else recorded_at for rows that predate it): a streamed request
// reported after the next turn began still counts to the turn that sent it.
// Each turn's range is [its created_at, the next turn's created_at), built
// once and range-joined. Events are kept AgentModelEventRetentionMonths, so
// an older session reads fewer (or no) turns.
func (s *PGStore) AgentSessionModelUsage(ctx context.Context, sessionID string) (AgentSessionModelCost, error) {
	rows, err := s.Pool.Query(ctx, `
		WITH session AS (
			SELECT id, workspace_id, agent_config FROM agent_sessions WHERE id = $1
		), turns AS (
			SELECT turn, created_at AS from_at,
			       lead(created_at) OVER (ORDER BY turn) AS until_at
			FROM agent_session_turns
			WHERE session_id = $1
		), events AS (
			SELECT e.*, COALESCE(e.started_at, e.recorded_at) AS at
			FROM agent_model_usage_events e
			JOIN session s ON e.workspace_id = s.workspace_id AND e.session_id = s.id
		), per_turn AS (
			SELECT COALESCE(t.turn, 0) AS turn, COUNT(*) AS requests,
			       SUM(e.input_tokens)::bigint AS input_tokens, SUM(e.output_tokens)::bigint AS output_tokens,
			       SUM(e.cache_read_tokens)::bigint AS cache_read_tokens, SUM(e.cache_write_tokens)::bigint AS cache_write_tokens,
			       SUM(e.price_microusd)::bigint AS price_microusd,
			       array_agg(DISTINCT e.model ORDER BY e.model) AS models,
			       MIN(e.at) AS first_at, MAX(e.at) AS last_at
			FROM events e
			LEFT JOIN turns t
			  ON e.at >= t.from_at AND (t.until_at IS NULL OR e.at < t.until_at)
			GROUP BY 1
		)
		SELECT s.workspace_id, s.agent_config, p.turn, p.requests,
		       p.input_tokens, p.output_tokens, p.cache_read_tokens, p.cache_write_tokens,
		       p.price_microusd, p.models, p.first_at, p.last_at
		FROM session s
		LEFT JOIN per_turn p ON true
		ORDER BY p.turn`, sessionID)
	if err != nil {
		return AgentSessionModelCost{}, err
	}
	defer rows.Close()
	out := AgentSessionModelCost{Turns: []AgentTurnModelUsage{}}
	found := false
	for rows.Next() {
		var (
			turn                         *int
			requests, in, outTok, cr, cw *int64
			price                        *int64
			models                       []string
			firstAt, lastAt              *time.Time
		)
		if err := rows.Scan(&out.WorkspaceID, &out.AgentConfig, &turn, &requests, &in, &outTok, &cr, &cw,
			&price, &models, &firstAt, &lastAt); err != nil {
			return AgentSessionModelCost{}, err
		}
		found = true
		if turn == nil {
			continue // the session has no requests
		}
		out.Turns = append(out.Turns, AgentTurnModelUsage{
			Turn: *turn, Requests: *requests, InputTokens: *in, OutputTokens: *outTok,
			CacheReadTokens: *cr, CacheWriteTokens: *cw, PriceMicroUSD: *price, Models: models,
			FirstAt: *firstAt, LastAt: *lastAt,
		})
	}
	if err := rows.Err(); err != nil {
		return AgentSessionModelCost{}, err
	}
	if !found {
		return AgentSessionModelCost{}, ErrNotFound
	}
	return out, nil
}

// AgentSessionModelSpendMicroUSD is what sessionID has spent on the platform
// route: the sum of its recorded customer prices, in micro-USD (ADR094 D8's
// per-session ceiling reads it before each model request). A session without
// events reads 0. Events are kept AgentModelEventRetentionMonths, far past any
// session's life.
func (s *PGStore) AgentSessionModelSpendMicroUSD(ctx context.Context, workspaceID, sessionID string) (int64, error) {
	var spent int64
	err := s.Pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(price_microusd), 0)::bigint
		FROM agent_model_usage_events
		WHERE workspace_id = $1 AND session_id = $2`, workspaceID, sessionID).Scan(&spent)
	return spent, err
}
