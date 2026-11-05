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
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

// BillingTrust is a workspace's durable payment record (ADR091 D10.1): the
// counters its agent-model trust tier is a pure function of. A workspace with
// no row has the zero record (T0, no lock).
type BillingTrust struct {
	WorkspaceID       string
	LifetimePaidCents int64
	PaidCycleInvoices int
	// LockedUntil pins the workspace to T0 while now is before it. Zero means
	// no payment lock; LockedIndefinitely means FailedInvoiceID is unpaid.
	LockedUntil time.Time
	// FailedInvoiceID is the invoice whose failed payment set the open lock;
	// only that invoice's payment settles it. LastFailureAt is that failure.
	FailedInvoiceID string
	LastFailureAt   time.Time
	// DisputeID is an open (or lost) dispute holding the workspace at T0
	// independently of LockedUntil; only that dispute closing as won clears
	// it. Ops clears a lost dispute's hold by hand (see the 0194 migration).
	DisputeID string
}

// Lock is the instant until which the workspace is held at T0: the payment
// lock, or LockedIndefinitely while a dispute holds it.
func (t BillingTrust) Lock() time.Time {
	if t.DisputeID != "" {
		return LockedIndefinitely
	}
	return t.LockedUntil
}

// LockedIndefinitely is LockedUntil for a stored 'infinity': a failed invoice
// not paid since.
var LockedIndefinitely = time.Date(9999, 12, 31, 23, 59, 59, 0, time.UTC)

// TrustLockAfterSettlement is how long a workspace stays at T0 after the
// payment that settles a failure (ADR091 D10.1).
const TrustLockAfterSettlement = 30 * 24 * time.Hour

// BillingTrustPayment is one paid, non-zero bex invoice.
type BillingTrustPayment struct {
	InvoiceID   string
	WorkspaceID string
	AmountCents int64
	// Cycle is true for a subscription_cycle invoice (the paid-cycle count).
	Cycle  bool
	PaidAt time.Time
}

const billingTrustColumns = `lifetime_paid_cents, paid_cycle_invoices, locked_until, failed_invoice_id, last_failure_at, dispute_id`

// trustRow scans billingTrustColumns; every column is nullable so the same
// scan serves a LEFT JOIN that found no row (the zero record).
type trustRow struct {
	lifetime               *int64
	cycles                 *int
	locked, failed         pgtype.Timestamptz
	failedInvoice, dispute *string
}

func (r *trustRow) dest() []any {
	return []any{&r.lifetime, &r.cycles, &r.locked, &r.failedInvoice, &r.failed, &r.dispute}
}

func (r *trustRow) record(workspaceID string) BillingTrust {
	deref := func(v *string) string {
		if v == nil {
			return ""
		}
		return *v
	}
	t := BillingTrust{WorkspaceID: workspaceID, LockedUntil: trustTime(r.locked), LastFailureAt: trustTime(r.failed),
		FailedInvoiceID: deref(r.failedInvoice), DisputeID: deref(r.dispute)}
	if r.lifetime != nil {
		t.LifetimePaidCents = *r.lifetime
	}
	if r.cycles != nil {
		t.PaidCycleInvoices = *r.cycles
	}
	return t
}

func scanBillingTrust(row pgx.Row, workspaceID string) (BillingTrust, error) {
	var r trustRow
	if err := row.Scan(r.dest()...); err != nil {
		return BillingTrust{}, err
	}
	return r.record(workspaceID), nil
}

func trustTime(v pgtype.Timestamptz) time.Time {
	switch {
	case !v.Valid:
		return time.Time{}
	case v.InfinityModifier == pgtype.Infinity:
		return LockedIndefinitely
	case v.InfinityModifier == pgtype.NegativeInfinity:
		return time.Time{}
	}
	return v.Time.UTC()
}

// GetBillingTrust returns workspaceID's record; a workspace that never paid
// or failed has the zero record (no error).
func (s *PGStore) GetBillingTrust(ctx context.Context, workspaceID string) (BillingTrust, error) {
	t, err := scanBillingTrust(s.Pool.QueryRow(ctx,
		`SELECT `+billingTrustColumns+` FROM billing_trust WHERE workspace_id = $1`, workspaceID), workspaceID)
	if errors.Is(err, pgx.ErrNoRows) {
		return BillingTrust{WorkspaceID: workspaceID}, nil
	}
	return t, err
}

// trustUpdate runs one change to workspaceID's record in a transaction that
// creates (if needed) and row-locks it first, returning the record before
// and after. change returns the UPDATE's SET list and arguments ($1 is the
// workspace), or "" to leave the row unchanged.
func (s *PGStore) trustUpdate(ctx context.Context, workspaceID string, change func(tx pgx.Tx, before BillingTrust) (string, []any, error)) (before, after BillingTrust, err error) {
	err = pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `INSERT INTO billing_trust (workspace_id) VALUES ($1) ON CONFLICT DO NOTHING`, workspaceID); err != nil {
			return trustWriteError(workspaceID, err)
		}
		var err error
		if before, err = scanBillingTrust(tx.QueryRow(ctx,
			`SELECT `+billingTrustColumns+` FROM billing_trust WHERE workspace_id = $1 FOR UPDATE`, workspaceID), workspaceID); err != nil {
			return err
		}
		after = before
		set, args, err := change(tx, before)
		if err != nil || set == "" {
			return err
		}
		after, err = scanBillingTrust(tx.QueryRow(ctx,
			`UPDATE billing_trust SET `+set+`, updated_at = now() WHERE workspace_id = $1 RETURNING `+billingTrustColumns,
			append([]any{workspaceID}, args...)...), workspaceID)
		return err
	})
	if err != nil {
		return BillingTrust{}, BillingTrust{}, err
	}
	return before, after, nil
}

func trustWriteError(workspaceID string, err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23503" {
		return fmt.Errorf("billing trust for %s: %w", workspaceID, ErrWorkspaceGone)
	}
	return err
}

// RecordBillingTrustPayment counts one paid invoice exactly once (the
// billing_trust_invoices ledger is keyed by invoice id, so a redelivered
// webhook adds nothing). Paying the invoice whose failure set the open lock
// settles it: the workspace stays at T0 until paid_at + 30 days. A payment of
// any other invoice, or one older than the failure, never unlocks. counted is
// false for a duplicate.
func (s *PGStore) RecordBillingTrustPayment(ctx context.Context, p BillingTrustPayment) (before, after BillingTrust, counted bool, err error) {
	if p.InvoiceID == "" || p.WorkspaceID == "" || p.AmountCents <= 0 || p.PaidAt.IsZero() {
		return BillingTrust{}, BillingTrust{}, false, fmt.Errorf("billing trust payment needs an invoice, a workspace, a positive amount and a paid time")
	}
	paidAt := p.PaidAt.UTC()
	// An invoice paid after it was written off is no longer bad debt. Its own
	// statement, keyed by invoice alone, so it lands even when the workspace
	// is gone (the trust update below then fails its foreign key).
	if _, err := s.Pool.Exec(ctx, `DELETE FROM billing_agent_model_writeoffs WHERE invoice_id = $1`, p.InvoiceID); err != nil {
		return BillingTrust{}, BillingTrust{}, false, err
	}
	before, after, err = s.trustUpdate(ctx, p.WorkspaceID, func(tx pgx.Tx, _ BillingTrust) (string, []any, error) {
		tag, err := tx.Exec(ctx, `
			INSERT INTO billing_trust_invoices (invoice_id, workspace_id, amount_cents, cycle, paid_at)
			VALUES ($1, $2, $3, $4, $5) ON CONFLICT (invoice_id) DO NOTHING`,
			p.InvoiceID, p.WorkspaceID, p.AmountCents, p.Cycle, paidAt)
		if err != nil {
			return "", nil, trustWriteError(p.WorkspaceID, err)
		}
		if counted = tag.RowsAffected() == 1; !counted {
			return "", nil, nil
		}
		if _, err := tx.Exec(ctx, `
			UPDATE billing_trust_failed_invoices SET settled_at = COALESCE(settled_at, $3)
			WHERE workspace_id = $1 AND invoice_id = $2`, p.WorkspaceID, p.InvoiceID, paidAt); err != nil {
			return "", nil, err
		}
		cycle := 0
		if p.Cycle {
			cycle = 1
		}
		settles := `failed_invoice_id = $4 AND $5::timestamptz >= last_failure_at`
		return `lifetime_paid_cents = lifetime_paid_cents + $2,
			paid_cycle_invoices = paid_cycle_invoices + $3,
			locked_until = CASE WHEN ` + settles + ` THEN $6::timestamptz ELSE locked_until END,
			failed_invoice_id = CASE WHEN ` + settles + ` THEN NULL ELSE failed_invoice_id END`,
			[]any{p.AmountCents, cycle, p.InvoiceID, paidAt, paidAt.Add(TrustLockAfterSettlement)}, nil
	})
	if err != nil {
		return BillingTrust{}, BillingTrust{}, false, err
	}
	return before, after, counted, nil
}

// RecordBillingTrustFailure drops the workspace to T0 until invoiceID is paid
// (locked_until = 'infinity'; that invoice's payment turns it into paid_at +
// 30 days) and opens invoiceID for the mint's balance-due refusal. A failure
// of an invoice already recorded as paid or settled is a late redelivery and
// changes nothing. While another failure is open, the newer of the two names
// the invoice that settles the lock (an out-of-order older failure keeps the
// lock but not its invoice); every failed invoice stays open for the refusal
// until it is itself paid or voided. applied reports whether the lock was set.
func (s *PGStore) RecordBillingTrustFailure(ctx context.Context, workspaceID, invoiceID string, at time.Time) (before, after BillingTrust, applied bool, err error) {
	if workspaceID == "" || invoiceID == "" || at.IsZero() {
		return BillingTrust{}, BillingTrust{}, false, fmt.Errorf("billing trust failure needs a workspace, an invoice and a time")
	}
	before, after, err = s.trustUpdate(ctx, workspaceID, func(tx pgx.Tx, _ BillingTrust) (string, []any, error) {
		var settled bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM billing_trust_invoices WHERE invoice_id = $2)
			OR EXISTS (SELECT 1 FROM billing_trust_failed_invoices WHERE workspace_id = $1 AND invoice_id = $2 AND settled_at IS NOT NULL)`,
			workspaceID, invoiceID).Scan(&settled); err != nil || settled {
			return "", nil, err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO billing_trust_failed_invoices (workspace_id, invoice_id, failed_at) VALUES ($1, $2, $3)
			ON CONFLICT (workspace_id, invoice_id) DO NOTHING`, workspaceID, invoiceID, at.UTC()); err != nil {
			return "", nil, err
		}
		applied = true
		newest := `failed_invoice_id IS NULL OR $3::timestamptz >= last_failure_at`
		return `locked_until = 'infinity',
			failed_invoice_id = CASE WHEN ` + newest + ` THEN $2 ELSE failed_invoice_id END,
			last_failure_at = CASE WHEN ` + newest + ` THEN $3::timestamptz ELSE last_failure_at END`,
			[]any{invoiceID, at.UTC()}, nil
	})
	if err != nil {
		return BillingTrust{}, BillingTrust{}, false, err
	}
	return before, after, applied, nil
}

// SettleBillingTrustInvoice closes invoiceID for the mint's balance-due
// refusal without counting a payment: invoice.voided, or an invoice.paid with
// nothing paid (a credit). Recorded even before the invoice's failure arrives,
// so a late invoice.payment_failed cannot reopen it. The T0 lock is untouched:
// only a payment settles it.
func (s *PGStore) SettleBillingTrustInvoice(ctx context.Context, workspaceID, invoiceID string, at time.Time) error {
	if workspaceID == "" || invoiceID == "" || at.IsZero() {
		return fmt.Errorf("billing trust settlement needs a workspace, an invoice and a time")
	}
	_, err := s.Pool.Exec(ctx, `
		INSERT INTO billing_trust_failed_invoices (workspace_id, invoice_id, settled_at) VALUES ($1, $2, $3)
		ON CONFLICT (workspace_id, invoice_id) DO UPDATE
		SET settled_at = COALESCE(billing_trust_failed_invoices.settled_at, EXCLUDED.settled_at)`,
		workspaceID, invoiceID, at.UTC())
	return trustWriteError(workspaceID, err)
}

// RecordBillingTrustDispute holds the workspace at T0 for disputeID until it
// closes as won. A dispute already closed as won (its closure delivered
// first) changes nothing.
func (s *PGStore) RecordBillingTrustDispute(ctx context.Context, workspaceID, disputeID string) (before, after BillingTrust, err error) {
	if workspaceID == "" || disputeID == "" {
		return BillingTrust{}, BillingTrust{}, fmt.Errorf("billing trust dispute needs a workspace and a dispute")
	}
	return s.trustUpdate(ctx, workspaceID, func(pgx.Tx, BillingTrust) (string, []any, error) {
		return `dispute_id = CASE WHEN won_dispute_id IS DISTINCT FROM $2 THEN $2 ELSE dispute_id END`, []any{disputeID}, nil
	})
}

// RecordBillingTrustDisputeWon clears the dispute hold set by disputeID (and
// remembers the win, so a late charge.dispute.created cannot re-set it). A
// lost dispute never reaches here: its hold stays until ops clears it.
func (s *PGStore) RecordBillingTrustDisputeWon(ctx context.Context, workspaceID, disputeID string) (before, after BillingTrust, err error) {
	if workspaceID == "" || disputeID == "" {
		return BillingTrust{}, BillingTrust{}, fmt.Errorf("billing trust dispute needs a workspace and a dispute")
	}
	return s.trustUpdate(ctx, workspaceID, func(pgx.Tx, BillingTrust) (string, []any, error) {
		return `dispute_id = CASE WHEN dispute_id = $2 THEN NULL ELSE dispute_id END, won_dispute_id = $2`, []any{disputeID}, nil
	})
}

// BillingWorkspaceForCustomer resolves the workspace a same-mode Stripe
// Customer belongs to (billing_provider_mappings.customer_id is unique).
// found is false for a customer bex does not map.
func (s *PGStore) BillingWorkspaceForCustomer(ctx context.Context, customerID string, livemode bool) (string, bool, error) {
	var workspaceID string
	err := s.Pool.QueryRow(ctx, `
		SELECT workspace_id FROM billing_provider_mappings
		WHERE customer_id = $1 AND livemode = $2`, customerID, livemode).Scan(&workspaceID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return workspaceID, true, nil
}

// AgentModelThresholdState is a workspace's trust record with the Stripe
// agent-model threshold last applied for it (ADR091 D10.2).
type AgentModelThresholdState struct {
	Trust BillingTrust
	// Synced is false until a value was applied; SyncedMicroUSD is that
	// value (nil = none, T4).
	Synced         bool
	SyncedMicroUSD *int64
}

// AgentModelThresholdsDue lists up to limit workspaces whose threshold may be
// stale at now: never applied, trust record changed since the value applied,
// or payment lock expired after it was computed. A workspace in failure
// backoff is skipped until its retry time.
func (s *PGStore) AgentModelThresholdsDue(ctx context.Context, now time.Time, limit int) ([]string, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT workspace_id FROM billing_trust
		WHERE (threshold_next_attempt_at IS NULL OR threshold_next_attempt_at <= $1)
		  AND (threshold_synced_version IS DISTINCT FROM updated_at
		       OR (locked_until > threshold_synced_at AND locked_until <= $1))
		ORDER BY threshold_next_attempt_at NULLS FIRST, workspace_id
		LIMIT $2`, now.UTC(), limit)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[string])
}

// WithAgentModelThresholdSync serializes one workspace's threshold sync across
// replicas (a transaction-scoped advisory lock, held across apply's Stripe
// call like WithTenantAdvisoryLock). apply gets the state read under the lock
// and returns the value it brought Stripe to. On success that value is
// recorded against the trust version apply saw (a change committed meanwhile
// leaves the workspace due again); on failure the workspace backs off
// (1 minute doubling to 1 hour) and apply's error is returned. A workspace
// with no trust record runs apply on the zero record and records nothing.
func (s *PGStore) WithAgentModelThresholdSync(ctx context.Context, workspaceID string, now time.Time, apply func(AgentModelThresholdState) (*int64, error)) error {
	var applyErr error
	err := pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`,
			"agent-model-threshold:"+workspaceID); err != nil {
			return err
		}
		var (
			r       trustRow
			version pgtype.Timestamptz
			state   AgentModelThresholdState
		)
		err := tx.QueryRow(ctx, `
			SELECT `+billingTrustColumns+`, updated_at, threshold_synced_version IS NOT NULL, threshold_synced_microusd
			FROM billing_trust WHERE workspace_id = $1`, workspaceID,
		).Scan(append(r.dest(), &version, &state.Synced, &state.SyncedMicroUSD)...)
		found := err == nil
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		state.Trust = r.record(workspaceID)
		value, err := apply(state)
		if !found {
			applyErr = err
			return nil
		}
		if err != nil {
			applyErr = err
			_, err = tx.Exec(ctx, `
				UPDATE billing_trust SET threshold_attempts = threshold_attempts + 1,
				  threshold_next_attempt_at = $2::timestamptz + LEAST(interval '1 minute' * power(2, LEAST(threshold_attempts, 6)), interval '1 hour')
				WHERE workspace_id = $1`, workspaceID, now.UTC())
			return err
		}
		_, err = tx.Exec(ctx, `
			UPDATE billing_trust SET threshold_synced_version = $2, threshold_synced_microusd = $3, threshold_synced_at = $4,
			  threshold_attempts = 0, threshold_next_attempt_at = NULL
			WHERE workspace_id = $1`, workspaceID, version, value, now.UTC())
		return err
	})
	if err != nil {
		return err
	}
	return applyErr
}
