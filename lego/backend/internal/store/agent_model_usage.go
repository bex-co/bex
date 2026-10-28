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
	"time"
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

// AgentModelUsageKinds lists the four token meters in a stable order.
var AgentModelUsageKinds = []string{
	UsageKindAgentModelInputTokens, UsageKindAgentModelOutputTokens,
	UsageKindAgentModelCacheReadTokens, UsageKindAgentModelCacheWriteTokens,
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
}

// RecordAgentModelUsage records one exchange and adds its tokens to the
// exchange's usage_hourly window, in one transaction. The ledger insert is
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
			 input_tokens, output_tokens, cache_read_tokens, cache_write_tokens)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (request_id) DO NOTHING`,
		u.RequestID, u.WorkspaceID, u.SessionID, u.Model, window,
		u.InputTokens, u.OutputTokens, u.CacheReadTokens, u.CacheWriteTokens)
	if err != nil {
		return false, err
	}
	if tag.RowsAffected() == 0 {
		return false, nil
	}
	quantities := []int64{u.InputTokens, u.OutputTokens, u.CacheReadTokens, u.CacheWriteTokens}
	for i, kind := range AgentModelUsageKinds {
		if quantities[i] <= 0 {
			continue
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO usage_hourly (workspace_id, service_id, kind, tier, resource_kind, window_start, quantity)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
			ON CONFLICT (resource_kind, service_id, kind, tier, window_start)
			DO UPDATE SET quantity = usage_hourly.quantity + EXCLUDED.quantity`,
			u.WorkspaceID, u.SessionID, kind, u.Model, ResourceKindAgentSession, window, quantities[i]); err != nil {
			return false, err
		}
	}
	return true, tx.Commit(ctx)
}

// AgentModelTokensMonthToDate is workspaceID's total platform-route tokens
// (all four token meters, every model) in the calendar month containing now:
// the input to the plan quota check (w10/m14 t005).
func (s *PGStore) AgentModelTokensMonthToDate(ctx context.Context, workspaceID string, now time.Time) (int64, error) {
	now = now.UTC()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	var total int64
	err := s.Pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(quantity), 0)::bigint FROM (
			SELECT quantity FROM usage_hourly
			WHERE workspace_id = $1 AND resource_kind = $3 AND kind = ANY($4)
			  AND window_start >= $2
			UNION ALL
			SELECT quantity FROM usage_monthly
			WHERE workspace_id = $1 AND resource_kind = $3 AND kind = ANY($4)
			  AND month = ($2::timestamptz AT TIME ZONE 'UTC')::date
		) u`, workspaceID, monthStart, ResourceKindAgentSession, AgentModelUsageKinds).Scan(&total)
	return total, err
}
