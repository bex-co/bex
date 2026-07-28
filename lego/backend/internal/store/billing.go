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
)

// SelectUnemittedUsage returns up to limit sealed usage_hourly rows that have
// not yet shipped to Stripe — the billing outbox read
// (docs/ADR040-billing-metronome.md §4). A row qualifies when its window is
// final (window_start < sealBefore, i.e. past the rewrite horizon), not below
// the billing floor (window_start >= floor), still un-emitted, and its
// workspace is not billing_excluded. Oldest first, so batches ship in order and
// the loop makes forward progress. The JOIN filters excluded tenants at the
// source: an excluded workspace's rows are never even considered for export.
func (s *PGStore) SelectUnemittedUsage(ctx context.Context, floor, sealBefore time.Time, limit int) ([]HourlyRow, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT u.workspace_id, u.service_id, u.kind, u.tier, u.resource_kind, u.window_start, u.quantity
		FROM usage_hourly u
		JOIN tenants t ON t.id = u.workspace_id
		WHERE u.billing_export_state = 'pending'
		  AND u.window_start >= $1
		  AND u.window_start <  $2
		  AND t.billing_excluded = false
		ORDER BY u.window_start
		LIMIT $3`,
		floor, sealBefore, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []HourlyRow
	for rows.Next() {
		var r HourlyRow
		if err := rows.Scan(&r.WorkspaceID, &r.ServiceID, &r.Kind, &r.Tier, &r.ResourceKind, &r.WindowStart, &r.Quantity); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// UsageExportAttempt binds the durable outbox row to the deterministic Stripe
// identifier and meter name selected by the billing mapper.
type UsageExportAttempt struct {
	Row           HourlyRow
	TransactionID string
	EventName     string
}

// UsageExportReject is the non-secret, bounded context retained for a
// permanent provider rejection.
type UsageExportReject struct {
	Attempt UsageExportAttempt
	Code    string
	Message string
}

// BillingExportStats is the low-cardinality snapshot exported to Prometheus.
type BillingExportStats struct {
	PendingRows      int64
	OldestPendingAge time.Duration
	RejectedRows     int64
	AmbiguousRows    int64
}

// MarkUsageAttempted durably records the deterministic identifier before any
// provider call. The first-attempt timestamp never moves: once the identifier
// window expires, the row is quarantined instead of being replayed blindly.
func (s *PGStore) MarkUsageAttempted(ctx context.Context, attempts []UsageExportAttempt, at time.Time) error {
	if len(attempts) == 0 {
		return nil
	}
	resourceKinds, serviceIDs, kinds, tiers, windows := usageExportKeys(attempts)
	transactionIDs := make([]string, len(attempts))
	eventNames := make([]string, len(attempts))
	for i, attempt := range attempts {
		transactionIDs[i] = attempt.TransactionID
		eventNames[i] = attempt.EventName
	}
	tag, err := s.Pool.Exec(ctx, `
		UPDATE usage_hourly u
		SET billing_export_attempted_at = COALESCE(u.billing_export_attempted_at, $1),
		    billing_export_transaction_id = k.transaction_id,
		    billing_export_event_name = k.event_name
		FROM unnest($2::text[], $3::text[], $4::text[], $5::text[], $6::timestamptz[], $7::text[], $8::text[])
			AS k(resource_kind, service_id, kind, tier, window_start, transaction_id, event_name)
		WHERE u.resource_kind = k.resource_kind
		  AND u.service_id = k.service_id
		  AND u.kind = k.kind
		  AND u.tier = k.tier
		  AND u.window_start = k.window_start
		  AND u.billing_export_state = 'pending'`,
		at.UTC(), resourceKinds, serviceIDs, kinds, tiers, windows, transactionIDs, eventNames)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != int64(len(attempts)) {
		return fmt.Errorf("persisted %d of %d billing export attempts", tag.RowsAffected(), len(attempts))
	}
	return nil
}

// RecordUsageExportResult atomically stamps accepted rows and moves permanent
// rejects into the durable issue table. Transient failures are intentionally
// absent: they remain pending with their immutable first-attempt timestamp.
func (s *PGStore) RecordUsageExportResult(ctx context.Context, accepted []UsageExportAttempt, rejected []UsageExportReject, at time.Time) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if len(accepted) > 0 {
		resourceKinds, serviceIDs, kinds, tiers, windows := usageExportKeys(accepted)
		transactionIDs := make([]string, len(accepted))
		for i, attempt := range accepted {
			transactionIDs[i] = attempt.TransactionID
		}
		if _, err = tx.Exec(ctx, `
			UPDATE usage_hourly u
			SET emitted_at = $1, billing_export_state = 'emitted',
			    billing_export_error_code = '', billing_export_error = ''
			FROM unnest($2::text[], $3::text[], $4::text[], $5::text[], $6::timestamptz[], $7::text[])
				AS k(resource_kind, service_id, kind, tier, window_start, transaction_id)
			WHERE u.resource_kind = k.resource_kind AND u.service_id = k.service_id
			  AND u.kind = k.kind AND u.tier = k.tier AND u.window_start = k.window_start
			  AND u.billing_export_transaction_id = k.transaction_id
			  AND u.billing_export_state = 'pending'`,
			at.UTC(), resourceKinds, serviceIDs, kinds, tiers, windows, transactionIDs); err != nil {
			return err
		}
	}
	for _, reject := range rejected {
		a := reject.Attempt
		code := boundedBillingDiagnostic(reject.Code, 80)
		message := boundedBillingDiagnostic(reject.Message, 240)
		tag, updateErr := tx.Exec(ctx, `
			UPDATE usage_hourly SET billing_export_state='rejected',
			    billing_export_error_code=$6, billing_export_error=$7
			WHERE resource_kind=$1 AND service_id=$2 AND kind=$3 AND tier=$4 AND window_start=$5
			  AND billing_export_transaction_id=$8 AND billing_export_state='pending'`,
			NormalizeResourceKind(a.Row.ResourceKind), a.Row.ServiceID, a.Row.Kind, a.Row.Tier,
			a.Row.WindowStart.UTC(), code, message, a.TransactionID)
		if updateErr != nil {
			return updateErr
		}
		if tag.RowsAffected() == 0 {
			// A sibling replica may already have stamped the same deterministic
			// event accepted. Never create a reject issue for a finalized row.
			continue
		}
		if _, err = tx.Exec(ctx, `
			INSERT INTO billing_export_issues
			(transaction_id, workspace_id, resource_kind, service_id, kind, tier, window_start,
			 event_name, issue_kind, error_code, error_message, first_seen_at, last_seen_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,'permanent_reject',$9,$10,$11,$11)
			ON CONFLICT (transaction_id) DO UPDATE SET
			  issue_kind='permanent_reject', error_code=EXCLUDED.error_code,
			  error_message=EXCLUDED.error_message, last_seen_at=EXCLUDED.last_seen_at,
			  resolved_at=NULL, resolution='', actor='', reason=''`,
			a.TransactionID, a.Row.WorkspaceID, NormalizeResourceKind(a.Row.ResourceKind),
			a.Row.ServiceID, a.Row.Kind, a.Row.Tier, a.Row.WindowStart.UTC(), a.EventName,
			code, message, at.UTC()); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// QuarantineOldUsageAttempts stops automatic replay after Stripe's rolling
// identifier de-duplication window. These rows require explicit reconciliation
// and an audited operator decision.
func (s *PGStore) QuarantineOldUsageAttempts(ctx context.Context, before, at time.Time) (int64, error) {
	tag, err := s.Pool.Exec(ctx, `
		WITH moved AS (
			UPDATE usage_hourly
			SET billing_export_state='ambiguous',
			    billing_export_error_code='dedupe_window_elapsed',
			    billing_export_error='provider outcome requires reconciliation before repair'
			WHERE billing_export_state='pending'
			  AND billing_export_attempted_at IS NOT NULL
			  AND billing_export_attempted_at < $1
			  AND billing_export_transaction_id IS NOT NULL
			RETURNING workspace_id, resource_kind, service_id, kind, tier, window_start,
			          billing_export_transaction_id, billing_export_event_name,
			          billing_export_attempted_at
		)
		INSERT INTO billing_export_issues
		(transaction_id, workspace_id, resource_kind, service_id, kind, tier, window_start,
		 event_name, issue_kind, error_code, error_message, first_seen_at, last_seen_at)
		SELECT billing_export_transaction_id, workspace_id, resource_kind, service_id, kind, tier,
		       window_start, COALESCE(billing_export_event_name,''), 'stamp_ambiguity',
		       'dedupe_window_elapsed', 'provider outcome requires reconciliation before repair',
		       billing_export_attempted_at, $2
		FROM moved WHERE billing_export_transaction_id IS NOT NULL
		ON CONFLICT (transaction_id) DO UPDATE SET
		  issue_kind='stamp_ambiguity', last_seen_at=EXCLUDED.last_seen_at,
		  resolved_at=NULL, resolution='', actor='', reason=''`, before.UTC(), at.UTC())
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

func (s *PGStore) BillingExportStats(ctx context.Context, floor, sealBefore, now time.Time) (BillingExportStats, error) {
	var stats BillingExportStats
	var ageSeconds float64
	err := s.Pool.QueryRow(ctx, `
		SELECT count(*) FILTER (WHERE u.billing_export_state='pending' AND u.window_start >= $2 AND u.window_start < $3 AND NOT t.billing_excluded),
		       COALESCE(EXTRACT(EPOCH FROM ($1 - (min(u.window_start) FILTER (
		           WHERE u.billing_export_state='pending' AND u.window_start >= $2 AND u.window_start < $3 AND NOT t.billing_excluded)))), 0),
		       (SELECT count(*) FROM billing_export_issues WHERE resolved_at IS NULL AND issue_kind='permanent_reject'),
		       (SELECT count(*) FROM billing_export_issues WHERE resolved_at IS NULL AND issue_kind='stamp_ambiguity')
		FROM usage_hourly u JOIN tenants t ON t.id=u.workspace_id`,
		now.UTC(), floor.UTC(), sealBefore.UTC()).Scan(&stats.PendingRows, &ageSeconds, &stats.RejectedRows, &stats.AmbiguousRows)
	stats.OldestPendingAge = time.Duration(ageSeconds * float64(time.Second))
	return stats, err
}

func usageExportKeys(attempts []UsageExportAttempt) ([]string, []string, []string, []string, []time.Time) {
	resourceKinds := make([]string, len(attempts))
	serviceIDs := make([]string, len(attempts))
	kinds := make([]string, len(attempts))
	tiers := make([]string, len(attempts))
	windows := make([]time.Time, len(attempts))
	for i, attempt := range attempts {
		resourceKinds[i] = NormalizeResourceKind(attempt.Row.ResourceKind)
		serviceIDs[i] = attempt.Row.ServiceID
		kinds[i] = attempt.Row.Kind
		tiers[i] = attempt.Row.Tier
		windows[i] = attempt.Row.WindowStart.UTC()
	}
	return resourceKinds, serviceIDs, kinds, tiers, windows
}

func boundedBillingDiagnostic(value string, limit int) string {
	if len(value) > limit {
		return value[:limit]
	}
	return value
}

// MarkUsageEmitted stamps emitted_at on exactly the given rows — the outbox
// write after a successful ingest. Keyed by the full primary key
// (resource_kind, service_id, kind, tier, window_start), it only touches rows
// still un-emitted (emitted_at IS NULL), so a re-run after a crash between
// ingest and stamp is a safe no-op. A single unnest-driven statement keeps the
// whole batch atomic.
func (s *PGStore) MarkUsageEmitted(ctx context.Context, rows []HourlyRow, at time.Time) error {
	if len(rows) == 0 {
		return nil
	}
	resourceKinds := make([]string, len(rows))
	serviceIDs := make([]string, len(rows))
	kinds := make([]string, len(rows))
	tiers := make([]string, len(rows))
	windows := make([]time.Time, len(rows))
	for i, r := range rows {
		resourceKinds[i] = NormalizeResourceKind(r.ResourceKind)
		serviceIDs[i] = r.ServiceID
		kinds[i] = r.Kind
		tiers[i] = r.Tier
		windows[i] = r.WindowStart
	}
	_, err := s.Pool.Exec(ctx, `
		UPDATE usage_hourly u SET emitted_at = $1, billing_export_state = 'emitted'
		FROM unnest($2::text[], $3::text[], $4::text[], $5::text[], $6::timestamptz[])
			AS k(resource_kind, service_id, kind, tier, window_start)
		WHERE u.resource_kind = k.resource_kind
		  AND u.service_id = k.service_id
		  AND u.kind = k.kind
		  AND u.tier = k.tier
		  AND u.window_start = k.window_start
		  AND u.emitted_at IS NULL`,
		at, resourceKinds, serviceIDs, kinds, tiers, windows)
	return err
}

// SetTenantBillingExcluded flips a workspace's billing-exclusion flag
// (docs/ADR040-billing-metronome.md §7, Mode A) and, when the value actually
// changes, records an audit_events row (verb billing.SetExclusion) attributing
// it to actor. This flag decides whether money is owed, so its only caller is
// the admin-only control-plane internal API — never a tenant. Returns whether
// the value changed (a no-op toggle writes no audit row); ErrNotFound when the
// workspace does not exist.
func (s *PGStore) SetTenantBillingExcluded(ctx context.Context, tenantID string, excluded bool, actor string, at time.Time) (bool, error) {
	changed, _, err := s.SetBillingException(ctx, tenantID, BillingExcluded, excluded, actor, "structural exclusion", at)
	return changed, err
}
