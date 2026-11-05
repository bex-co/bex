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
	"errors"
	"fmt"
	"time"
)

// MonthStartUTC is the first instant of t's calendar month in UTC.
func MonthStartUTC(t time.Time) time.Time {
	t = t.UTC()
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
}

// agentSessionUsageSumSQL sums agent_session usage of the kinds in the
// text[] placeholder kinds over the UTC calendar month starting at the
// timestamptz placeholder start: its hourly rows plus its compacted monthly
// row. where narrows both halves (e.g. "workspace_id = $1 AND "). resource_kind
// is a literal so the planner can use the partial usage_hourly indexes. The
// month's end is computed on a timestamp without time zone, so the session
// TimeZone cannot shift it.
func agentSessionUsageSumSQL(where, start, kinds string) string {
	return `(SELECT COALESCE(SUM(quantity), 0)::bigint FROM (
		SELECT quantity FROM usage_hourly
		WHERE ` + where + `resource_kind = '` + ResourceKindAgentSession + `' AND kind = ANY(` + kinds + `::text[])
		  AND window_start >= ` + start + `::timestamptz
		  AND window_start < ((` + start + `::timestamptz AT TIME ZONE 'UTC') + interval '1 month') AT TIME ZONE 'UTC'
		UNION ALL
		SELECT quantity FROM usage_monthly
		WHERE ` + where + `resource_kind = '` + ResourceKindAgentSession + `' AND kind = ANY(` + kinds + `::text[])
		  AND month = (` + start + `::timestamptz AT TIME ZONE 'UTC')::date
	) u)`
}

// ErrInvalidAgentModelWriteOff refuses a write-off missing a field or with an
// amount outside (0, AgentModelNetMicroUSD].
var ErrInvalidAgentModelWriteOff = errors.New("invalid agent-model write-off")

// AgentModelWriteOff is the agent-model share of one Stripe invoice marked
// uncollectible (ADR091 D10): bad debt, in agent_model_cost_microusd's unit.
type AgentModelWriteOff struct {
	InvoiceID   string
	WorkspaceID string
	CustomerID  string
	// UsageMonth is any instant in the calendar month (UTC) the invoice's
	// agent-model usage was billed for.
	UsageMonth time.Time
	// AgentModelNetMicroUSD is the invoice's agent-model lines after
	// discounts; AmountMicroUSD the written-off share of it.
	AgentModelNetMicroUSD int64
	AmountMicroUSD        int64
	WrittenOffAt          time.Time
	// CardFingerprint is the Stripe fingerprint of the card whose charge of
	// the invoice failed, else the Customer's default card ("" ⇒ none): binding a card with the same fingerprint
	// is refused while the debt is unpaid (D10.6). Never logged.
	CardFingerprint string
}

// RecordAgentModelWriteOff records w once per invoice (a redelivered
// invoice.marked_uncollectible adds nothing). recorded is false for a
// duplicate, or for an invoice already recorded as paid.
func (s *PGStore) RecordAgentModelWriteOff(ctx context.Context, w AgentModelWriteOff) (recorded bool, err error) {
	if w.InvoiceID == "" || w.WorkspaceID == "" || w.CustomerID == "" || w.UsageMonth.IsZero() || w.WrittenOffAt.IsZero() ||
		w.AmountMicroUSD <= 0 || w.AmountMicroUSD > w.AgentModelNetMicroUSD {
		return false, fmt.Errorf("%w: invoice %q", ErrInvalidAgentModelWriteOff, w.InvoiceID)
	}
	tag, err := s.Pool.Exec(ctx, `
		INSERT INTO billing_agent_model_writeoffs
		  (invoice_id, workspace_id, customer_id, usage_month, agent_model_net_microusd, amount_microusd, written_off_at, card_fingerprint)
		SELECT $1, $2, $3, $4, $5, $6, $7, NULLIF($8, '')
		WHERE NOT EXISTS (SELECT 1 FROM billing_trust_invoices WHERE invoice_id = $1)
		ON CONFLICT (invoice_id) DO NOTHING`,
		w.InvoiceID, w.WorkspaceID, w.CustomerID, MonthStartUTC(w.UsageMonth), w.AgentModelNetMicroUSD, w.AmountMicroUSD, w.WrittenOffAt.UTC(), w.CardFingerprint)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

// UnpaidWriteOffForCardFingerprint reports whether any unpaid agent-model
// write-off, in any workspace including a deleted one, was recorded against a
// card with this Stripe fingerprint (ADR091 D10.6). A paid invoice deletes its
// write-off, so every row is unpaid. An empty fingerprint never matches.
func (s *PGStore) UnpaidWriteOffForCardFingerprint(ctx context.Context, fingerprint string) (bool, error) {
	if fingerprint == "" {
		return false, nil
	}
	var found bool
	err := s.Pool.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM billing_agent_model_writeoffs WHERE card_fingerprint = $1)`,
		fingerprint).Scan(&found)
	return found, err
}

// AgentModelWriteOffRetention is how long a write-off is kept as a debt
// record, including after its workspace is deleted (ADR086 amendment
// 2026-11-05).
const AgentModelWriteOffRetention = 7 * 365 * 24 * time.Hour

// PurgeAgentModelWriteOffs deletes write-offs recorded before before.
func (s *PGStore) PurgeAgentModelWriteOffs(ctx context.Context, before time.Time) (int64, error) {
	tag, err := s.Pool.Exec(ctx, `DELETE FROM billing_agent_model_writeoffs WHERE written_off_at < $1`, before.UTC())
	return tag.RowsAffected(), err
}

// RefreshAgentModelRevenue raises month's (any instant in it) agent-model
// revenue snapshot to the current sum over every workspace. It never lowers
// it: a deleted workspace's usage rows cascade away, but its revenue stays
// counted.
func (s *PGStore) RefreshAgentModelRevenue(ctx context.Context, month time.Time) error {
	_, err := s.Pool.Exec(ctx, `
		INSERT INTO billing_agent_model_revenue_months AS m (month, revenue_microusd, refreshed_at)
		SELECT ($1::timestamptz AT TIME ZONE 'UTC')::date, `+agentSessionUsageSumSQL("", "$1", "$2")+`, now()
		ON CONFLICT (month) DO UPDATE
		SET revenue_microusd = GREATEST(m.revenue_microusd, EXCLUDED.revenue_microusd), refreshed_at = EXCLUDED.refreshed_at`,
		MonthStartUTC(month), []string{UsageKindAgentModelCostMicroUSD})
	return err
}

// AgentModelBadDebt is one calendar month's agent-model revenue (metered
// customer price, agent_model_cost_microusd, as last snapshotted) and the
// agent-model amount written off on invoices billing that month's usage.
// ADR091 D10 moves Hobby to prepaid credits when WrittenOffMicroUSD exceeds 2%
// of RevenueMicroUSD.
type AgentModelBadDebt struct {
	RevenueMicroUSD    int64
	WrittenOffMicroUSD int64
}

// AgentModelBadDebtForMonth reads month's (any instant in it, UTC) revenue
// snapshot and write-offs across every workspace.
func (s *PGStore) AgentModelBadDebtForMonth(ctx context.Context, month time.Time) (AgentModelBadDebt, error) {
	var out AgentModelBadDebt
	err := s.Pool.QueryRow(ctx, `
		SELECT
		  COALESCE((SELECT revenue_microusd FROM billing_agent_model_revenue_months WHERE month = $1), 0),
		  (SELECT COALESCE(SUM(amount_microusd), 0)::bigint FROM billing_agent_model_writeoffs WHERE usage_month = $1)`,
		MonthStartUTC(month)).Scan(&out.RevenueMicroUSD, &out.WrittenOffMicroUSD)
	return out, err
}
