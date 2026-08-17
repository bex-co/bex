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
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
)

const (
	BillingHealthy    = "healthy"
	BillingGrace      = "grace"
	BillingEnforcing  = "enforcing"
	BillingEnforced   = "enforced"
	BillingRecovering = "recovering"
	BillingExcluded   = "excluded"
	BillingComped     = "comped"

	BillingOutcomeFailure = "failure"
	BillingOutcomeSuccess = "success"
)

// BillingProviderMapping is the durable relationship that lets a signed
// Stripe event be resolved without a network call on the webhook request path.
// Provider ids never enter tenant-editable state or public API results.
type BillingProviderMapping struct {
	WorkspaceID    string
	CustomerID     string
	SubscriptionID string
	Livemode       bool
	UpdatedAt      time.Time
}

// BillingLifecycle is the provider-neutral durable dunning state.
type BillingLifecycle struct {
	WorkspaceID        string     `json:"workspaceId"`
	Status             string     `json:"status"`
	Reason             string     `json:"reason,omitempty"`
	GraceDeadline      *time.Time `json:"-"`
	SourceEventID      string     `json:"-"`
	SourceEventAt      *time.Time `json:"-"`
	SourceEventOutcome string     `json:"-"`
	InvoiceID          string     `json:"-"`
	SubscriptionID     string     `json:"-"`
	TransitionVersion  int64      `json:"-"`
	RetryAt            *time.Time `json:"-"`
	ClaimedUntil       *time.Time `json:"-"`
	AttemptCount       int        `json:"-"`
	LastError          string     `json:"-"`
	EnforcedAt         *time.Time `json:"-"`
	RecoveredAt        *time.Time `json:"-"`
	RecoveryTarget     string     `json:"-"`
	CreatedAt          time.Time  `json:"-"`
	UpdatedAt          time.Time  `json:"updatedAt"`
}

// StripeBillingEvent is the normalized, non-sensitive subset retained from a
// signature-verified event. Outcome is failure or success; the full body,
// payment method, card, and customer billing details are never persisted.
type StripeBillingEvent struct {
	EventID           string
	EventType         string
	WorkspaceID       string
	CustomerID        string
	SubscriptionID    string
	ObjectID          string
	Livemode          bool
	ProviderCreatedAt time.Time
	ReceivedAt        time.Time
	Outcome           string
	Reason            string
}

type BillingEnforcement struct {
	WorkspaceID  string
	ResourceKind string
	ResourceName string
	MarkerID     string
	EnforcedAt   time.Time
	RecoveredAt  *time.Time
}

type BillingNotification struct {
	WorkspaceID       string
	TransitionVersion int64
	Status            string
	Reason            string
	GraceDeadline     *time.Time
	Livemode          bool
	AttemptCount      int
}

const billingLifecycleColumns = `workspace_id, status, reason, grace_deadline,
	source_event_id, source_event_at, source_event_outcome, invoice_id, subscription_id,
	transition_version, retry_at, claimed_until, attempt_count, last_error,
	enforced_at, recovered_at, recovery_target, created_at, updated_at`

type rowScanner interface{ Scan(...any) error }

func scanBillingLifecycle(row rowScanner) (BillingLifecycle, error) {
	var b BillingLifecycle
	err := row.Scan(&b.WorkspaceID, &b.Status, &b.Reason, &b.GraceDeadline,
		&b.SourceEventID, &b.SourceEventAt, &b.SourceEventOutcome, &b.InvoiceID, &b.SubscriptionID,
		&b.TransitionVersion, &b.RetryAt, &b.ClaimedUntil, &b.AttemptCount,
		&b.LastError, &b.EnforcedAt, &b.RecoveredAt, &b.RecoveryTarget, &b.CreatedAt, &b.UpdatedAt)
	return b, err
}

func (s *PGStore) UpsertBillingProviderMapping(ctx context.Context, m BillingProviderMapping) error {
	if m.WorkspaceID == "" || m.CustomerID == "" {
		return fmt.Errorf("billing provider mapping requires workspace and customer")
	}
	// A livemode flip (test→live cutover or a rollback) invalidates the
	// mode-bound halves of the row: the payment-method marker was proven
	// against the other mode's Stripe objects yet alone gates paid intent and
	// usage export locally, and the cached subscription id names an object the
	// new mode cannot see. Both reset on flip so a fresh same-mode checkout /
	// EnsureContract must re-establish them; customer_id self-heals by
	// overwrite (w4/m81 t001).
	_, err := s.Pool.Exec(ctx, `
		INSERT INTO billing_provider_mappings (workspace_id, customer_id, subscription_id, livemode)
		VALUES ($1, $2, NULLIF($3, ''), $4)
		ON CONFLICT (workspace_id) DO UPDATE
		SET customer_id = EXCLUDED.customer_id,
		    subscription_id = CASE
		        WHEN billing_provider_mappings.livemode <> EXCLUDED.livemode THEN EXCLUDED.subscription_id
		        ELSE COALESCE(EXCLUDED.subscription_id, billing_provider_mappings.subscription_id)
		    END,
		    payment_method_bound_at = CASE
		        WHEN billing_provider_mappings.livemode <> EXCLUDED.livemode THEN NULL
		        ELSE billing_provider_mappings.payment_method_bound_at
		    END,
		    livemode = EXCLUDED.livemode,
		    updated_at = now()`, m.WorkspaceID, m.CustomerID, m.SubscriptionID, m.Livemode)
	return err
}

func (s *PGStore) ListBillingProviderMappings(ctx context.Context, livemode bool, limit int) ([]BillingProviderMapping, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	// A mapping from the other Stripe mode references objects the current
	// client's key cannot retrieve (a test→live cutover leaves test-mode rows
	// behind until each workspace re-onboards), so polling it can only 404.
	rows, err := s.Pool.Query(ctx, `
		SELECT workspace_id, customer_id, COALESCE(subscription_id, ''), livemode, updated_at
		FROM billing_provider_mappings
		WHERE subscription_id IS NOT NULL AND livemode=$2
		ORDER BY updated_at, workspace_id LIMIT $1`, limit, livemode)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []BillingProviderMapping
	for rows.Next() {
		var m BillingProviderMapping
		if err := rows.Scan(&m.WorkspaceID, &m.CustomerID, &m.SubscriptionID, &m.Livemode, &m.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// TouchBillingProviderMapping rotates a mapping to the back of the bounded
// reconciliation queue after every attempt. That prevents the oldest 500
// workspaces (including a permanently failing one) from starving the rest.
func (s *PGStore) TouchBillingProviderMapping(ctx context.Context, workspaceID string, at time.Time) error {
	_, err := s.Pool.Exec(ctx, `UPDATE billing_provider_mappings SET updated_at=$2 WHERE workspace_id=$1`, workspaceID, at)
	return err
}

// ensureBillingLifecycleSQL seeds the default lifecycle row from the tenant's
// exception flags; workspace_id is the table's sole unique constraint (its
// PK), so the bare conflict target is equivalent to naming it.
const ensureBillingLifecycleSQL = `
	INSERT INTO billing_lifecycles (workspace_id, status)
	SELECT id, CASE WHEN billing_excluded THEN 'excluded'
	                    WHEN billing_comped THEN 'comped' ELSE 'healthy' END
	FROM tenants WHERE id = $1
	ON CONFLICT (workspace_id) DO NOTHING`

// EnsureBillingLifecycle materializes the default/readable state when a
// Customer/Subscription is first observed. Exclusion and comp flags win.
func (s *PGStore) EnsureBillingLifecycle(ctx context.Context, workspaceID string) (BillingLifecycle, error) {
	if _, err := s.Pool.Exec(ctx, ensureBillingLifecycleSQL, workspaceID); err != nil {
		return BillingLifecycle{}, err
	}
	return s.GetBillingLifecycle(ctx, workspaceID)
}

// normalizeBillingOverride applies the shared operator-override input
// contract: actor defaults to the control plane, reason is a required,
// bounded operator explanation.
func normalizeBillingOverride(actor, reason string) (string, string, error) {
	if actor == "" {
		actor = "control-plane"
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return "", "", fmt.Errorf("billing override reason is required")
	}
	if len(reason) > 200 {
		return "", "", fmt.Errorf("billing override reason is too long")
	}
	return actor, reason, nil
}

// insertBillingNotification enqueues one status-change notification row; the
// conflict guard keeps a transition version's notification single-shot.
func insertBillingNotification(ctx context.Context, tx pgx.Tx, workspaceID string, version int64, status, reason string, deadline *time.Time, livemode bool, at time.Time) error {
	_, err := tx.Exec(ctx, `INSERT INTO billing_notifications
		(workspace_id, transition_version, status, reason, grace_deadline, livemode, next_attempt_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7) ON CONFLICT DO NOTHING`,
		workspaceID, version, status, reason, deadline, livemode, at)
	return err
}

// insertBillingOverrideAudit records one operator billing override in the
// audit log; excludedTo is non-nil only for the exclusion toggle.
func insertBillingOverrideAudit(ctx context.Context, tx pgx.Tx, workspaceID, actor, verb string, at time.Time, excludedTo *bool, reason, statusFrom, statusTo string) error {
	_, err := tx.Exec(ctx, `INSERT INTO audit_events
		(id,workspace_id,caller,caller_method,verb,resource,outcome,at,billing_excluded_to,billing_override_reason,billing_status_from,billing_status_to)
		VALUES($1,$2,$3,'control-plane',$4,$5,'allowed',$6,$7,$8,$9,$10)`,
		ids.New(ids.Audit), workspaceID, actor, verb, core.WorkspaceObject(workspaceID), at,
		excludedTo, reason, statusFrom, statusTo)
	return err
}

func (s *PGStore) GetBillingLifecycle(ctx context.Context, workspaceID string) (BillingLifecycle, error) {
	b, err := scanBillingLifecycle(s.Pool.QueryRow(ctx,
		`SELECT `+billingLifecycleColumns+` FROM billing_lifecycles WHERE workspace_id = $1`, workspaceID))
	if err != nil {
		return BillingLifecycle{}, classify("billing lifecycle", err)
	}
	return b, nil
}

// lockBillingLifecycle reads one lifecycle row FOR UPDATE, pinning it for the
// rest of the transaction. The raw scan error is returned; callers that can
// observe a missing row classify it themselves.
func lockBillingLifecycle(ctx context.Context, tx pgx.Tx, workspaceID string) (BillingLifecycle, error) {
	return scanBillingLifecycle(tx.QueryRow(ctx,
		`SELECT `+billingLifecycleColumns+` FROM billing_lifecycles WHERE workspace_id=$1 FOR UPDATE`, workspaceID))
}

// normalized validates the event's required fields (and the grace window that
// accompanies it) and canonicalizes both timestamps to UTC, defaulting
// ReceivedAt to now.
func (e StripeBillingEvent) normalized(grace time.Duration) (StripeBillingEvent, error) {
	if e.EventID == "" || e.WorkspaceID == "" || e.ProviderCreatedAt.IsZero() {
		return StripeBillingEvent{}, fmt.Errorf("incomplete Stripe billing event")
	}
	if e.Outcome != BillingOutcomeFailure && e.Outcome != BillingOutcomeSuccess {
		return StripeBillingEvent{}, fmt.Errorf("unknown billing outcome %q", e.Outcome)
	}
	if grace <= 0 {
		return StripeBillingEvent{}, fmt.Errorf("grace must be positive")
	}
	if e.CustomerID == "" || e.SubscriptionID == "" {
		return StripeBillingEvent{}, fmt.Errorf("Stripe billing event missing customer/subscription")
	}
	if e.ReceivedAt.IsZero() {
		e.ReceivedAt = time.Now().UTC()
	}
	e.ProviderCreatedAt = e.ProviderCreatedAt.UTC()
	e.ReceivedAt = e.ReceivedAt.UTC()
	return e, nil
}

// verifyBillingProviderMapping binds or verifies the event's workspace mapping.
// The immutable Subscription metadata names the workspace. Bind or verify
// the mapping in the same transaction before accepting its event: the
// upsert touches exactly one row iff the mapping was created or already
// matches (the DO UPDATE's WHERE); zero rows means a conflicting mapping.
func verifyBillingProviderMapping(ctx context.Context, tx pgx.Tx, e StripeBillingEvent) error {
	mapping, err := tx.Exec(ctx, `
		INSERT INTO billing_provider_mappings (workspace_id, customer_id, subscription_id, livemode)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (workspace_id) DO UPDATE
		SET customer_id = EXCLUDED.customer_id,
		    subscription_id = EXCLUDED.subscription_id,
		    livemode = EXCLUDED.livemode,
		    updated_at = now()
		WHERE billing_provider_mappings.customer_id = EXCLUDED.customer_id
		  AND billing_provider_mappings.subscription_id = EXCLUDED.subscription_id
		  AND billing_provider_mappings.livemode = EXCLUDED.livemode`, e.WorkspaceID, e.CustomerID, e.SubscriptionID, e.Livemode)
	if err != nil {
		return fmt.Errorf("verify billing mapping: %w", err)
	}
	if mapping.RowsAffected() != 1 {
		return fmt.Errorf("provider mapping mismatch")
	}
	return nil
}

// markStripeBillingEventApplied stamps the event row applied — the shared tail
// of both the stale-event guard and the applied-transition path.
func markStripeBillingEventApplied(ctx context.Context, tx pgx.Tx, e StripeBillingEvent) error {
	_, err := tx.Exec(ctx, `UPDATE stripe_billing_events SET applied_at=$2 WHERE event_id=$1`, e.EventID, e.ReceivedAt)
	return err
}

// stripeBillingEventGuards seeds and locks the lifecycle row, then applies the
// no-transition guards. done reports that the locked state should be returned
// unchanged: a replayed already-applied event, an excluded/comped tenant, or a
// provider timestamp not newer than the applied source event (which is still
// stamped applied).
func stripeBillingEventGuards(ctx context.Context, tx pgx.Tx, e StripeBillingEvent, inserted bool) (BillingLifecycle, bool, error) {
	if _, err := tx.Exec(ctx, ensureBillingLifecycleSQL, e.WorkspaceID); err != nil {
		return BillingLifecycle{}, false, err
	}
	state, err := lockBillingLifecycle(ctx, tx, e.WorkspaceID)
	if err != nil {
		return BillingLifecycle{}, false, classify("billing lifecycle", err)
	}
	// A freshly inserted event row has applied_at NULL by construction, so
	// the duplicate-application read only matters for a replayed event.
	if !inserted {
		var appliedAt *time.Time
		if err := tx.QueryRow(ctx, `SELECT applied_at FROM stripe_billing_events WHERE event_id=$1`, e.EventID).Scan(&appliedAt); err != nil {
			return BillingLifecycle{}, false, err
		}
		if appliedAt != nil {
			return state, true, nil
		}
	}
	var excluded, comped bool
	if err := tx.QueryRow(ctx, `SELECT billing_excluded,billing_comped FROM tenants WHERE id=$1`, e.WorkspaceID).Scan(&excluded, &comped); err != nil {
		return BillingLifecycle{}, false, err
	}
	if excluded || comped || state.Status == BillingExcluded || state.Status == BillingComped {
		return state, true, nil
	}
	if state.SourceEventAt != nil && !e.ProviderCreatedAt.After(*state.SourceEventAt) {
		if err := markStripeBillingEventApplied(ctx, tx, e); err != nil {
			return BillingLifecycle{}, false, err
		}
		return state, true, nil
	}
	return state, false, nil
}

// applyStripeBillingTransition computes and persists the event's transition,
// enqueues the notification when the status edge warrants one, and stamps the
// event row applied. changed reports whether status, reason, or deadline moved.
func applyStripeBillingTransition(ctx context.Context, tx pgx.Tx, state BillingLifecycle, e StripeBillingEvent, grace time.Duration) (BillingLifecycle, bool, error) {
	previous := state.Status
	next, reason, deadline := transitionBillingState(state, e, grace)
	changed := next != state.Status || reason != state.Reason || !sameTime(deadline, state.GraceDeadline)
	version := state.TransitionVersion
	attemptCount := state.AttemptCount
	if changed {
		version++
		attemptCount = 0
	}
	state, err := scanBillingLifecycle(tx.QueryRow(ctx, `
		UPDATE billing_lifecycles SET
		status=$2, reason=$3, grace_deadline=$4, source_event_id=$5,
		source_event_at=$6, source_event_outcome=$7, invoice_id=$8, subscription_id=$9,
		transition_version=$10, attempt_count=$11, retry_at=NULL, claimed_until=NULL,
		last_error='', recovery_target=CASE WHEN $2='recovering' THEN 'healthy' ELSE recovery_target END, updated_at=$12
		WHERE workspace_id=$1 RETURNING `+billingLifecycleColumns,
		e.WorkspaceID, next, reason, deadline, e.EventID, e.ProviderCreatedAt, e.Outcome,
		e.ObjectID, e.SubscriptionID, version, attemptCount, e.ReceivedAt))
	if err != nil {
		return BillingLifecycle{}, false, err
	}
	if changed && notifyBillingStatus(previous, next) {
		if err := insertBillingNotification(ctx, tx, e.WorkspaceID, version, next, reason, deadline, e.Livemode, e.ReceivedAt); err != nil {
			return BillingLifecycle{}, false, err
		}
	}
	if err := markStripeBillingEventApplied(ctx, tx, e); err != nil {
		return BillingLifecycle{}, false, err
	}
	return state, changed, nil
}

// RecordStripeBillingEvent inserts the provider event and applies its normalized
// transition in one transaction. Duplicate ids and stale provider timestamps
// are retained/recognized without repeating a state change or notification.
func (s *PGStore) RecordStripeBillingEvent(ctx context.Context, e StripeBillingEvent, grace time.Duration) (BillingLifecycle, bool, bool, error) {
	e, err := e.normalized(grace)
	if err != nil {
		return BillingLifecycle{}, false, false, err
	}
	var state BillingLifecycle
	var inserted, changed bool
	err = s.withTx(ctx, func(tx pgx.Tx) error {
		if err := verifyBillingProviderMapping(ctx, tx, e); err != nil {
			return err
		}
		cmd, err := tx.Exec(ctx, `
		INSERT INTO stripe_billing_events
		(event_id, event_type, workspace_id, customer_id, subscription_id, object_id, livemode, provider_created_at, received_at, outcome, reason)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		ON CONFLICT (event_id) DO NOTHING`, e.EventID, e.EventType, e.WorkspaceID,
			e.CustomerID, e.SubscriptionID, e.ObjectID, e.Livemode, e.ProviderCreatedAt, e.ReceivedAt, e.Outcome, e.Reason)
		if err != nil {
			return err
		}
		inserted = cmd.RowsAffected() == 1
		var done bool
		state, done, err = stripeBillingEventGuards(ctx, tx, e, inserted)
		if err != nil || done {
			return err
		}
		state, changed, err = applyStripeBillingTransition(ctx, tx, state, e, grace)
		return err
	})
	if err != nil {
		return BillingLifecycle{}, false, false, err
	}
	return state, inserted, changed, nil
}

func transitionBillingState(state BillingLifecycle, e StripeBillingEvent, grace time.Duration) (string, string, *time.Time) {
	if e.Outcome == BillingOutcomeSuccess {
		switch state.Status {
		case BillingEnforcing, BillingEnforced, BillingRecovering:
			return BillingRecovering, "payment_recovered", nil
		default:
			return BillingHealthy, "", nil
		}
	}
	// A new failure during recovery cancels recovery and preserves enforcement.
	if state.Status == BillingRecovering || state.Status == BillingEnforcing {
		return BillingEnforcing, nonempty(e.Reason, "payment_failed"), state.GraceDeadline
	}
	if state.Status == BillingEnforced {
		return BillingEnforced, nonempty(e.Reason, "payment_failed"), state.GraceDeadline
	}
	if state.Status == BillingGrace && state.GraceDeadline != nil {
		return BillingGrace, state.Reason, state.GraceDeadline
	}
	deadline := e.ProviderCreatedAt.Add(grace).UTC()
	return BillingGrace, nonempty(e.Reason, "payment_failed"), &deadline
}

func sameTime(a, b *time.Time) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return a.Equal(*b)
}

func nonempty(v, fallback string) string {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	return v
}

func notifyBillingStatus(previous, next string) bool {
	if previous == next {
		return false
	}
	switch next {
	case BillingGrace, BillingEnforced, BillingExcluded, BillingComped:
		return true
	case BillingHealthy:
		return previous != BillingHealthy
	default:
		return false
	}
}

// ClaimDueBillingLifecycle leases one enforcement/recovery row. FOR UPDATE
// SKIP LOCKED makes the claim safe across bex-api replicas.
func (s *PGStore) ClaimDueBillingLifecycle(ctx context.Context, now time.Time, lease time.Duration) (BillingLifecycle, bool, error) {
	if lease <= 0 {
		lease = time.Minute
	}
	var b BillingLifecycle
	var claimed bool
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		var workspaceID string
		err := tx.QueryRow(ctx, `
		SELECT workspace_id FROM billing_lifecycles
		WHERE (
		    (status='grace' AND grace_deadline <= $1) OR
		    (status IN ('enforcing','recovering') AND COALESCE(retry_at, '-infinity') <= $1)
		) AND (claimed_until IS NULL OR claimed_until < $1)
		ORDER BY COALESCE(retry_at, grace_deadline, updated_at), workspace_id
		FOR UPDATE SKIP LOCKED LIMIT 1`, now).Scan(&workspaceID)
		if err == pgx.ErrNoRows {
			return nil
		}
		if err != nil {
			return err
		}
		b, err = scanBillingLifecycle(tx.QueryRow(ctx, `
		UPDATE billing_lifecycles SET
		status=CASE WHEN status='grace' THEN 'enforcing' ELSE status END,
		transition_version=CASE WHEN status='grace' THEN transition_version+1 ELSE transition_version END,
		claimed_until=$2, attempt_count=attempt_count+1, updated_at=$1
		WHERE workspace_id=$3 RETURNING `+billingLifecycleColumns, now, now.Add(lease), workspaceID))
		if err != nil {
			return err
		}
		claimed = true
		return nil
	})
	if err != nil {
		return BillingLifecycle{}, false, err
	}
	return b, claimed, nil
}

func (s *PGStore) CompleteBillingLifecycleWork(ctx context.Context, workspaceID string, expectedVersion int64, status string, now time.Time) (BillingLifecycle, error) {
	if status != BillingEnforced && status != BillingHealthy && status != BillingExcluded && status != BillingComped {
		return BillingLifecycle{}, fmt.Errorf("invalid completed billing status %q", status)
	}
	var b BillingLifecycle
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		current, err := lockBillingLifecycle(ctx, tx, workspaceID)
		if err != nil {
			return classify("billing lifecycle", err)
		}
		if current.TransitionVersion != expectedVersion ||
			(status == BillingEnforced && current.Status != BillingEnforcing) ||
			(status != BillingEnforced && current.Status != BillingRecovering) {
			b = current
			return nil
		}
		if current.Status == status {
			b = current
			return nil
		}
		version := current.TransitionVersion + 1
		reason := current.Reason
		var deadline *time.Time = current.GraceDeadline
		if status == BillingHealthy {
			reason, deadline = "", nil
		}
		b, err = scanBillingLifecycle(tx.QueryRow(ctx, `
		UPDATE billing_lifecycles SET status=$2, reason=$3, grace_deadline=$4,
		transition_version=$5, attempt_count=0, retry_at=NULL, claimed_until=NULL, last_error='',
		enforced_at=CASE WHEN $2='enforced' THEN $6 ELSE enforced_at END,
		recovered_at=CASE WHEN $2 IN ('healthy','excluded','comped') THEN $6 ELSE recovered_at END,
		recovery_target='healthy',
		updated_at=$6 WHERE workspace_id=$1 RETURNING `+billingLifecycleColumns,
			workspaceID, status, reason, deadline, version, now))
		if err != nil {
			return err
		}
		var livemode bool
		if err := tx.QueryRow(ctx, `SELECT livemode FROM billing_provider_mappings WHERE workspace_id=$1`, workspaceID).Scan(&livemode); err != nil {
			return err
		}
		if notifyBillingStatus(current.Status, status) {
			if err := insertBillingNotification(ctx, tx, workspaceID, version, status, reason, deadline, livemode, now); err != nil {
				return err
			}
		}
		verb := core.AuditVerbBillingEnforced
		if status != BillingEnforced {
			verb = core.AuditVerbBillingRecovered
		}
		_, err = tx.Exec(ctx, `INSERT INTO audit_events
		(id,workspace_id,caller,caller_method,verb,resource,outcome,at)
		VALUES($1,$2,'billing-worker','internal',$3,$4,'allowed',$5)`,
			ids.New(ids.Audit), workspaceID, verb, core.WorkspaceObject(workspaceID), now)
		return err
	})
	if err != nil {
		return BillingLifecycle{}, err
	}
	return b, nil
}

func (s *PGStore) FailBillingLifecycleWork(ctx context.Context, workspaceID string, expectedVersion int64, message string, now, retryAt time.Time) error {
	if len(message) > 300 {
		message = message[:300]
	}
	_, err := s.Pool.Exec(ctx, `UPDATE billing_lifecycles
		SET claimed_until=NULL, retry_at=$2, last_error=$3, updated_at=$1
		WHERE workspace_id=$4 AND transition_version=$5 AND status IN ('enforcing','recovering')`, now, retryAt, message, workspaceID, expectedVersion)
	return err
}

func (s *PGStore) EnsureBillingEnforcement(ctx context.Context, e BillingEnforcement) (BillingEnforcement, error) {
	if e.MarkerID == "" {
		return BillingEnforcement{}, fmt.Errorf("billing enforcement marker is required")
	}
	err := s.Pool.QueryRow(ctx, `
		INSERT INTO billing_enforcements (workspace_id,resource_kind,resource_name,marker_id)
		VALUES ($1,$2,$3,$4)
		ON CONFLICT (workspace_id,resource_kind,resource_name) DO UPDATE
		SET marker_id=CASE WHEN billing_enforcements.recovered_at IS NULL THEN billing_enforcements.marker_id ELSE EXCLUDED.marker_id END,
		    enforced_at=CASE WHEN billing_enforcements.recovered_at IS NULL THEN billing_enforcements.enforced_at ELSE now() END,
		    recovered_at=NULL
		RETURNING workspace_id,resource_kind,resource_name,marker_id,enforced_at,recovered_at`,
		e.WorkspaceID, e.ResourceKind, e.ResourceName, e.MarkerID).
		Scan(&e.WorkspaceID, &e.ResourceKind, &e.ResourceName, &e.MarkerID, &e.EnforcedAt, &e.RecoveredAt)
	return e, err
}

func (s *PGStore) ListActiveBillingEnforcements(ctx context.Context, workspaceID string) ([]BillingEnforcement, error) {
	rows, err := s.Pool.Query(ctx, `SELECT workspace_id,resource_kind,resource_name,marker_id,enforced_at,recovered_at
		FROM billing_enforcements WHERE workspace_id=$1 AND recovered_at IS NULL
		ORDER BY resource_kind,resource_name`, workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []BillingEnforcement
	for rows.Next() {
		var e BillingEnforcement
		if err := rows.Scan(&e.WorkspaceID, &e.ResourceKind, &e.ResourceName, &e.MarkerID, &e.EnforcedAt, &e.RecoveredAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (s *PGStore) MarkBillingEnforcementRecovered(ctx context.Context, workspaceID, kind, name string, at time.Time) error {
	_, err := s.Pool.Exec(ctx, `UPDATE billing_enforcements SET recovered_at=$4
		WHERE workspace_id=$1 AND resource_kind=$2 AND resource_name=$3 AND recovered_at IS NULL`, workspaceID, kind, name, at)
	return err
}

func (s *PGStore) ClaimBillingNotifications(ctx context.Context, now time.Time, lease time.Duration, limit int) ([]BillingNotification, error) {
	if lease <= 0 {
		lease = time.Minute
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	rows, err := s.Pool.Query(ctx, `WITH due AS (
		SELECT workspace_id,transition_version FROM billing_notifications
		WHERE delivered_at IS NULL AND next_attempt_at <= $1
		  AND (claimed_until IS NULL OR claimed_until < $1)
		ORDER BY next_attempt_at,workspace_id LIMIT $2 FOR UPDATE SKIP LOCKED
	) UPDATE billing_notifications n SET claimed_until=$3, attempt_count=n.attempt_count+1
	  FROM due WHERE n.workspace_id=due.workspace_id AND n.transition_version=due.transition_version
	  RETURNING n.workspace_id,n.transition_version,n.status,n.reason,n.grace_deadline,n.livemode,n.attempt_count`,
		now, limit, now.Add(lease))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []BillingNotification
	for rows.Next() {
		var n BillingNotification
		if err := rows.Scan(&n.WorkspaceID, &n.TransitionVersion, &n.Status, &n.Reason, &n.GraceDeadline, &n.Livemode, &n.AttemptCount); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

func (s *PGStore) ListBillingOwnerSubjects(ctx context.Context, workspaceID string) ([]string, error) {
	rows, err := s.Pool.Query(ctx, `SELECT subject FROM tenant_members
		WHERE tenant_id=$1 AND role='admin' ORDER BY created_at,subject`, workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var subject string
		if err := rows.Scan(&subject); err != nil {
			return nil, err
		}
		out = append(out, subject)
	}
	return out, rows.Err()
}

func (s *PGStore) CompleteBillingNotification(ctx context.Context, workspaceID string, version int64, at time.Time) error {
	_, err := s.Pool.Exec(ctx, `UPDATE billing_notifications SET delivered_at=$3,claimed_until=NULL,last_error=''
		WHERE workspace_id=$1 AND transition_version=$2`, workspaceID, version, at)
	return err
}

func (s *PGStore) FailBillingNotification(ctx context.Context, workspaceID string, version int64, message string, next time.Time) error {
	if len(message) > 300 {
		message = message[:300]
	}
	_, err := s.Pool.Exec(ctx, `UPDATE billing_notifications SET claimed_until=NULL,last_error=$3,next_attempt_at=$4
		WHERE workspace_id=$1 AND transition_version=$2`, workspaceID, version, message, next)
	return err
}

func (s *PGStore) PurgeStripeBillingEvents(ctx context.Context, before time.Time) (int64, error) {
	cmd, err := s.Pool.Exec(ctx, `DELETE FROM stripe_billing_events WHERE received_at < $1`, before)
	if err != nil {
		return 0, err
	}
	return cmd.RowsAffected(), nil
}

// CheckBillingMutationAllowed is the feature-service gate for billable creates,
// upgrades, deploys, and tenant resume attempts. Grace remains usable; the
// gate begins only once enforcement is due/active.
func (s *PGStore) CheckBillingMutationAllowed(ctx context.Context, workspaceID string) error {
	var status string
	err := s.Pool.QueryRow(ctx, `SELECT status FROM billing_lifecycles WHERE workspace_id=$1`, workspaceID).Scan(&status)
	if err == pgx.ErrNoRows {
		return nil
	}
	if err != nil {
		return err
	}
	switch status {
	case BillingEnforcing, BillingEnforced, BillingRecovering:
		return fmt.Errorf("%w: workspace billing state is %s", core.ErrBillingEnforced, status)
	default:
		return nil
	}
}

// billingExceptionOutcome is billingExceptionTransition's computed transition.
type billingExceptionOutcome struct {
	next         string // lifecycle status to persist
	target       string // recovery_target accompanying it
	reason       string // operator-prefixed reason ("" for healthy)
	version      int64  // transition_version to persist
	stateChanged bool   // whether status or reason moved
}

// billingExceptionTransition computes the lifecycle transition one exception
// toggle produces: the toggle is folded into the tenant's pre-toggle flags,
// active enforcement (or an enforcement-shaped status) routes the change
// through the ordinary recovery worker, and the version advances only on a
// real change. Pure — every input is read before it runs.
func billingExceptionTransition(state BillingLifecycle, exception string, enabled bool, excluded, comped, active bool, reason string) billingExceptionOutcome {
	if exception == BillingExcluded {
		excluded = enabled
	} else {
		comped = enabled
	}
	desired := BillingHealthy
	if excluded {
		desired = BillingExcluded
	} else if comped {
		desired = BillingComped
	}
	next, target := desired, BillingHealthy
	if active || state.Status == BillingEnforced || state.Status == BillingEnforcing || state.Status == BillingRecovering {
		next, target = BillingRecovering, desired
	}
	nextReason := "operator_" + exception
	if enabled {
		nextReason += ": " + reason
	} else {
		nextReason += "_removed: " + reason
	}
	if next == BillingHealthy {
		nextReason = ""
	}
	version := state.TransitionVersion
	stateChanged := next != state.Status || nextReason != state.Reason
	if stateChanged {
		version++
	}
	return billingExceptionOutcome{next: next, target: target, reason: nextReason, version: version, stateChanged: stateChanged}
}

// applyBillingExceptionTransition seeds and locks the lifecycle row, computes
// the pure exception transition from the pre-toggle flags plus any still-active
// enforcement, and persists it. previousStatus is the pre-update status the
// audit row records.
func applyBillingExceptionTransition(ctx context.Context, tx pgx.Tx, workspaceID, exception string, enabled bool, excluded, comped bool, reason string, at time.Time) (BillingLifecycle, billingExceptionOutcome, string, error) {
	// Deliberately the plain 'healthy' seed, not ensureBillingLifecycleSQL: the
	// tenant flag was already toggled in this transaction, so the flag-aware
	// seed would stamp a fresh row 'excluded'/'comped' and change the audit
	// row's previousStatus for excluded/comped tenants.
	if _, err := tx.Exec(ctx, `INSERT INTO billing_lifecycles(workspace_id,status) VALUES($1,'healthy') ON CONFLICT DO NOTHING`, workspaceID); err != nil {
		return BillingLifecycle{}, billingExceptionOutcome{}, "", err
	}
	state, err := lockBillingLifecycle(ctx, tx, workspaceID)
	if err != nil {
		return BillingLifecycle{}, billingExceptionOutcome{}, "", err
	}
	previousStatus := state.Status
	var active bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM billing_enforcements WHERE workspace_id=$1 AND recovered_at IS NULL)`, workspaceID).Scan(&active); err != nil {
		return BillingLifecycle{}, billingExceptionOutcome{}, "", err
	}
	out := billingExceptionTransition(state, exception, enabled, excluded, comped, active, reason)
	state, err = scanBillingLifecycle(tx.QueryRow(ctx, `UPDATE billing_lifecycles SET status=$2,reason=$3,grace_deadline=NULL,recovery_target=$4,transition_version=$5,attempt_count=0,retry_at=NULL,claimed_until=NULL,last_error='',updated_at=$6 WHERE workspace_id=$1 RETURNING `+billingLifecycleColumns, workspaceID, out.next, out.reason, out.target, out.version, at))
	if err != nil {
		return BillingLifecycle{}, billingExceptionOutcome{}, "", err
	}
	return state, out, previousStatus, nil
}

// SetBillingException atomically applies/removes the structural exclusion or
// rated-but-free comp flag and moves any billing-owned suspension through the
// ordinary recovery worker. reason is a bounded operator explanation, never a
// payment detail or arbitrary payload.
func (s *PGStore) SetBillingException(ctx context.Context, workspaceID, exception string, enabled bool, actor, reason string, at time.Time) (bool, BillingLifecycle, error) {
	if exception != BillingExcluded && exception != BillingComped {
		return false, BillingLifecycle{}, fmt.Errorf("invalid billing exception %q", exception)
	}
	actor, reason, err := normalizeBillingOverride(actor, reason)
	if err != nil {
		return false, BillingLifecycle{}, err
	}
	column := "billing_excluded"
	verb := core.AuditVerbBillingExclusionChanged
	if exception == BillingComped {
		column, verb = "billing_comped", core.AuditVerbBillingCompChanged
	}
	var changed bool
	var state BillingLifecycle
	err = s.withTx(ctx, func(tx pgx.Tx) error {
		// One locked read of both flags; the row stays locked for the whole
		// transaction and the UPDATE below is its only writer, so the post-update
		// pair is derived locally instead of re-read.
		var excluded, comped bool
		if err := tx.QueryRow(ctx, `SELECT billing_excluded,billing_comped FROM tenants WHERE id=$1 FOR UPDATE`, workspaceID).Scan(&excluded, &comped); err != nil {
			return classify("tenant", err)
		}
		current := excluded
		if exception == BillingComped {
			current = comped
		}
		changed = current != enabled
		if !changed {
			var err error
			state, err = scanBillingLifecycle(tx.QueryRow(ctx, `SELECT `+billingLifecycleColumns+` FROM billing_lifecycles WHERE workspace_id=$1`, workspaceID))
			if err == pgx.ErrNoRows {
				state, err = BillingLifecycle{WorkspaceID: workspaceID, Status: BillingHealthy}, nil
			}
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE tenants SET `+column+`=$2,updated_at=now() WHERE id=$1`, workspaceID, enabled); err != nil {
			return err
		}
		st, out, previousStatus, err := applyBillingExceptionTransition(ctx, tx, workspaceID, exception, enabled, excluded, comped, reason, at)
		if err != nil {
			return err
		}
		state = st
		var livemode bool
		_ = tx.QueryRow(ctx, `SELECT livemode FROM billing_provider_mappings WHERE workspace_id=$1`, workspaceID).Scan(&livemode)
		if out.next != BillingRecovering && out.stateChanged {
			if err := insertBillingNotification(ctx, tx, workspaceID, out.version, out.next, out.reason, nil, livemode, at); err != nil {
				return err
			}
		}
		var excludedTo *bool
		if exception == BillingExcluded {
			excludedTo = &enabled
		}
		return insertBillingOverrideAudit(ctx, tx, workspaceID, actor, verb, at, excludedTo, reason, previousStatus, state.Status)
	})
	if err != nil {
		return false, BillingLifecycle{}, err
	}
	return changed, state, nil
}

func (s *PGStore) ExtendBillingGrace(ctx context.Context, workspaceID string, extension time.Duration, actor, reason string, at time.Time) (BillingLifecycle, error) {
	if extension <= 0 {
		return BillingLifecycle{}, fmt.Errorf("grace extension must be positive")
	}
	actor, reason, err := normalizeBillingOverride(actor, reason)
	if err != nil {
		return BillingLifecycle{}, err
	}
	var state BillingLifecycle
	err = s.withTx(ctx, func(tx pgx.Tx) error {
		var err error
		state, err = scanBillingLifecycle(tx.QueryRow(ctx, `UPDATE billing_lifecycles SET grace_deadline=GREATEST(grace_deadline,$2)+($3 * interval '1 second'),reason=$4,transition_version=transition_version+1,updated_at=$2 WHERE workspace_id=$1 AND status='grace' RETURNING `+billingLifecycleColumns, workspaceID, at, int64(extension/time.Second), "operator_grace_extension: "+reason))
		if err != nil {
			return classify("billing grace", err)
		}
		return insertBillingOverrideAudit(ctx, tx, workspaceID, actor, core.AuditVerbBillingGraceExtended, at, nil, reason, BillingGrace, BillingGrace)
	})
	if err != nil {
		return BillingLifecycle{}, err
	}
	return state, nil
}

func (s *PGStore) ForceBillingRecovery(ctx context.Context, workspaceID, actor, reason string, at time.Time) (BillingLifecycle, error) {
	actor, reason, err := normalizeBillingOverride(actor, reason)
	if err != nil {
		return BillingLifecycle{}, err
	}
	var state BillingLifecycle
	err = s.withTx(ctx, func(tx pgx.Tx) error {
		var previous string
		if err := tx.QueryRow(ctx, `SELECT status FROM billing_lifecycles WHERE workspace_id=$1 FOR UPDATE`, workspaceID).Scan(&previous); err != nil {
			return classify("billing lifecycle", err)
		}
		var err error
		state, err = scanBillingLifecycle(tx.QueryRow(ctx, `UPDATE billing_lifecycles SET status='recovering',reason=$2,recovery_target='healthy',grace_deadline=NULL,transition_version=transition_version+1,attempt_count=0,retry_at=NULL,claimed_until=NULL,last_error='',updated_at=$3 WHERE workspace_id=$1 AND status IN ('grace','enforcing','enforced','recovering') RETURNING `+billingLifecycleColumns, workspaceID, "operator_recovery: "+reason, at))
		if err != nil {
			return classify("billing lifecycle", err)
		}
		return insertBillingOverrideAudit(ctx, tx, workspaceID, actor, core.AuditVerbBillingRecoveryForced, at, nil, reason, previous, BillingRecovering)
	})
	if err != nil {
		return BillingLifecycle{}, err
	}
	return state, nil
}
