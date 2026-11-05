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

	"github.com/jackc/pgx/v5"
)

// PaymentEligibility is the local paid-intent/export snapshot for one
// workspace. Excluded and comped are explicit operator exemptions; Bound is
// stamped only by the verified checkout.session.completed path (ADR046).
type PaymentEligibility struct {
	Bound    bool
	Excluded bool
	Comped   bool
	// Rejection is why the last card offered was refused at binding (ADR091
	// D10.5–D10.6), "" when none is pending. It never affects AllowsPaidIntent:
	// a workspace that already holds a card keeps it.
	Rejection PaymentMethodRejection
}

func (p PaymentEligibility) AllowsPaidIntent() bool {
	return p.Bound || p.Excluded || p.Comped
}

// SetPaymentMethodBound stamps the marker monotonically. Replayed Stripe
// webhooks retain the first successful bind timestamp, so the operation is
// idempotent and never moves the enforcement snapshot backwards or forwards.
func (s *PGStore) SetPaymentMethodBound(ctx context.Context, workspaceID string, at time.Time) error {
	tag, err := s.Pool.Exec(ctx, `
		UPDATE billing_provider_mappings
		SET payment_method_bound_at = COALESCE(payment_method_bound_at, $2),
		    payment_method_rejection = NULL,
		    updated_at = now()
		WHERE workspace_id = $1`, workspaceID, at.UTC())
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// MarkCheckoutStarted records that a hosted Checkout session was created for the
// workspace. This is intent, NOT payment state. Monotonic like the bind marker
// — the first attempt is the one worth keeping — and silent for an absent
// mapping: a workspace without a Customer opens Checkout without one (Stripe
// creates it on submit), so only workspaces that already own a Customer, and
// rows minted back when Checkout pre-created it, carry the stamp.
func (s *PGStore) MarkCheckoutStarted(ctx context.Context, workspaceID string, at time.Time) error {
	_, err := s.Pool.Exec(ctx, `
		UPDATE billing_provider_mappings
		SET checkout_started_at = COALESCE(checkout_started_at, $2),
		    updated_at = now()
		WHERE workspace_id = $1`, workspaceID, at.UTC())
	return err
}

// PaymentMethodBound is the narrow marker accessor. An unknown or unstamped
// workspace returns false rather than leaking whether a tenant exists.
func (s *PGStore) PaymentMethodBound(ctx context.Context, workspaceID string) (bool, error) {
	var bound bool
	err := s.Pool.QueryRow(ctx, `
		SELECT payment_method_bound_at IS NOT NULL
		FROM billing_provider_mappings
		WHERE workspace_id = $1`, workspaceID).Scan(&bound)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	return bound, err
}

// PaymentEligibility reads the marker and both billing exemptions in one
// local SELECT. The tenant row is authoritative, so a workspace that has not
// entered Checkout yet still returns an unstamped (not missing) snapshot.
func (s *PGStore) PaymentEligibility(ctx context.Context, workspaceID string) (PaymentEligibility, error) {
	var eligibility PaymentEligibility
	var rejection *string
	err := s.Pool.QueryRow(ctx, `
		SELECT m.payment_method_bound_at IS NOT NULL,
		       t.billing_excluded, t.billing_comped, m.payment_method_rejection
		FROM tenants t
		LEFT JOIN billing_provider_mappings m ON m.workspace_id = t.id
		WHERE t.id = $1`, workspaceID).Scan(&eligibility.Bound, &eligibility.Excluded, &eligibility.Comped, &rejection)
	if errors.Is(err, pgx.ErrNoRows) {
		return PaymentEligibility{}, ErrNotFound
	}
	if rejection != nil {
		eligibility.Rejection = PaymentMethodRejection(*rejection)
	}
	return eligibility, err
}

// PaymentMethodRejection is why a payment method was refused at binding
// (ADR091 D10.5–D10.6). It is stored on the workspace's provider mapping until
// a method binds, and is a column CHECK-constrained to these values.
type PaymentMethodRejection string

const (
	// PaymentMethodRejectedPrepaid is a prepaid card (card.funding=prepaid).
	PaymentMethodRejectedPrepaid PaymentMethodRejection = "prepaid_card"
	// PaymentMethodRejectedUnpaidBalance is a card whose fingerprint matches an
	// unpaid agent-model write-off.
	PaymentMethodRejectedUnpaidBalance PaymentMethodRejection = "unpaid_balance"
)

// RecordPaymentMethodRejection stores why the last payment method offered for
// the workspace was refused at binding. The bind marker is untouched: a
// workspace that already has a card keeps it. A workspace with no mapping row
// has nothing to annotate. SetPaymentMethodBound clears it, so a later
// successful binding never shows a stale refusal.
func (s *PGStore) RecordPaymentMethodRejection(ctx context.Context, workspaceID string, reason PaymentMethodRejection) error {
	_, err := s.Pool.Exec(ctx, `
		UPDATE billing_provider_mappings
		SET payment_method_rejection = $2, updated_at = now()
		WHERE workspace_id = $1`, workspaceID, string(reason))
	return err
}

// UsageSeal is the outbox's finality horizon. A row is sealed when
// window_start < Before (the rollup rewrite horizon, ADR040 §3) or, for a
// kind in FastKinds, when window_start < FastBefore. A fast kind's hourly row
// is written only while its hour is open and never rewritten by the rollup
// (agent_model_cost_microusd, ADR091 D3), so it is final once the hour has
// closed and needs no 48h wait.
type UsageSeal struct {
	Before     time.Time
	FastKinds  []string
	FastBefore time.Time
	// HoldKinds are kinds never selected for export: their rows stay pending
	// until the kind is released (ADR091 D3: the agent model cost meter waits
	// for its Stripe catalog Price), so nothing is dropped while held.
	HoldKinds []string
}

// SealsAbove reports whether any window at or above floor can be sealed, so
// the emitter can skip a pass that cannot export anything.
func (u UsageSeal) SealsAbove(floor time.Time) bool {
	return floor.Before(u.Before) || (len(u.FastKinds) > 0 && floor.Before(u.FastBefore))
}

// sealedPendingUsage is the outbox's "sealed, pending, above the floor, not
// held" predicate over usage_hourly u, shared by the outbox read and its
// metrics so the two never disagree. Parameters: $1 floor, $2 UsageSeal.Before,
// $5 FastKinds, $6 FastBefore, $7 HoldKinds. The GREATEST upper bound is
// implied by the OR below it; it is stated so the window_start index range
// is bounded on both ends.
const sealedPendingUsage = `u.billing_export_state = 'pending'
		  AND u.window_start >= $1
		  AND u.window_start < GREATEST($2::timestamptz, $6::timestamptz)
		  AND (u.window_start < $2 OR (u.kind = ANY($5) AND u.window_start < $6))
		  AND NOT (u.kind = ANY($7))`

// SelectUnemittedUsage returns up to limit sealed usage_hourly rows that have
// not yet shipped to Stripe — the billing outbox read
// (docs/ADR040-billing-metronome.md §4). A row qualifies when its window is
// final (sealed per UsageSeal, i.e. past the rewrite horizon), not below
// the billing floor (window_start >= floor), still un-emitted, and its
// workspace is not billing_excluded. Oldest first, so batches ship in order and
// the loop makes forward progress. The JOIN filters excluded tenants at the
// source: an excluded workspace's rows are never even considered for export.
func (s *PGStore) SelectUnemittedUsage(ctx context.Context, floor time.Time, seal UsageSeal, limit int, requirePaymentMethod bool) ([]HourlyRow, error) {
	return s.selectPendingUsage(ctx, "", floor, seal, limit, requirePaymentMethod)
}

// SelectWorkspaceUnemittedUsage is SelectUnemittedUsage for one workspace:
// the settle-before-delete export (ADR091 D10.4), which passes a seal at the
// present instead of the rollup horizon. Floor, held kinds, exclusion and the
// payment-method withholding apply exactly as in the outbox read.
// usage_hourly_workspace_pending_idx (migration 0198) serves it.
func (s *PGStore) SelectWorkspaceUnemittedUsage(ctx context.Context, workspaceID string, floor time.Time, seal UsageSeal, limit int, requirePaymentMethod bool) ([]HourlyRow, error) {
	if workspaceID == "" {
		return nil, fmt.Errorf("%w: workspace id required", ErrInvalid)
	}
	return s.selectPendingUsage(ctx, workspaceID, floor, seal, limit, requirePaymentMethod)
}

// selectPendingUsage is the one outbox read; workspaceID "" reads every
// workspace.
func (s *PGStore) selectPendingUsage(ctx context.Context, workspaceID string, floor time.Time, seal UsageSeal, limit int, requirePaymentMethod bool) ([]HourlyRow, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT u.workspace_id, u.service_id, u.kind, u.tier, u.resource_kind, u.window_start, u.quantity
		FROM usage_hourly u
		JOIN tenants t ON t.id = u.workspace_id
		LEFT JOIN billing_provider_mappings m ON m.workspace_id = u.workspace_id
		WHERE `+sealedPendingUsage+`
		  AND ($8 = '' OR u.workspace_id = $8)
		  AND t.billing_excluded = false
		  AND ($4 = false OR t.billing_comped OR m.payment_method_bound_at IS NOT NULL)
		ORDER BY u.window_start
		LIMIT $3`,
		floor, seal.Before, limit, requirePaymentMethod, nonNil(seal.FastKinds), seal.FastBefore, nonNil(seal.HoldKinds), workspaceID)
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

// ExportedUsageTotal is one workspace's exported quantity of one meter
// dimension (kind, tier, resource kind) over a window: Quantity counts every
// row Stripe accepted, Recent only those accepted at or after a cutoff.
type ExportedUsageTotal struct {
	Kind         string
	Tier         string
	ResourceKind string
	Quantity     int64
	Recent       int64
}

// ExportedUsageTotals sums the workspace's usage rows stamped emitted whose
// window starts in [from, to), per meter dimension. Settle before delete
// (ADR091 D10.4) compares it with Stripe's meter event summaries before it
// invoices. Served by the (workspace_id, window_start) index.
func (s *PGStore) ExportedUsageTotals(ctx context.Context, workspaceID string, from, to, recentSince time.Time) ([]ExportedUsageTotal, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT kind, tier, resource_kind, sum(quantity)::bigint,
		       COALESCE(sum(quantity) FILTER (WHERE emitted_at >= $4), 0)::bigint
		FROM usage_hourly
		WHERE workspace_id = $1 AND window_start >= $2 AND window_start < $3
		  AND billing_export_state = 'emitted'
		GROUP BY kind, tier, resource_kind`,
		workspaceID, from.UTC(), to.UTC(), recentSince.UTC())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ExportedUsageTotal
	for rows.Next() {
		var t ExportedUsageTotal
		if err := rows.Scan(&t.Kind, &t.Tier, &t.ResourceKind, &t.Quantity, &t.Recent); err != nil {
			return nil, err
		}
		out = append(out, t)
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
	PendingRows        int64
	OldestPendingAge   time.Duration
	RejectedRows       int64
	AmbiguousRows      int64
	WithheldWorkspaces int64
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

func (s *PGStore) BillingExportStats(ctx context.Context, floor time.Time, seal UsageSeal, now time.Time, requirePaymentMethod bool) (BillingExportStats, error) {
	var stats BillingExportStats
	var ageSeconds float64
	err := s.Pool.QueryRow(ctx, `
		SELECT count(*) FILTER (WHERE `+sealedPendingUsage+` AND NOT t.billing_excluded),
		       COALESCE(EXTRACT(EPOCH FROM ($3 - (min(u.window_start) FILTER (
		           WHERE `+sealedPendingUsage+` AND NOT t.billing_excluded)))), 0),
		       (SELECT count(*) FROM billing_export_issues WHERE resolved_at IS NULL AND issue_kind='permanent_reject'),
		       (SELECT count(*) FROM billing_export_issues WHERE resolved_at IS NULL AND issue_kind='stamp_ambiguity'),
		       count(DISTINCT u.workspace_id) FILTER (
		           WHERE $4 AND `+sealedPendingUsage+`
		             AND NOT t.billing_excluded AND NOT t.billing_comped
		             AND m.payment_method_bound_at IS NULL)
		FROM usage_hourly u
		JOIN tenants t ON t.id=u.workspace_id
		LEFT JOIN billing_provider_mappings m ON m.workspace_id=u.workspace_id`,
		floor.UTC(), seal.Before.UTC(), now.UTC(), requirePaymentMethod, nonNil(seal.FastKinds), seal.FastBefore.UTC(), nonNil(seal.HoldKinds)).Scan(
		&stats.PendingRows, &ageSeconds, &stats.RejectedRows, &stats.AmbiguousRows, &stats.WithheldWorkspaces)
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
