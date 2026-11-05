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
// 30 days). A failure of an invoice already recorded as paid is a late
// redelivery and changes nothing. While another failure is open, the newer
// of the two names the invoice that settles the lock (an out-of-order older
// failure keeps the lock but not its invoice). applied reports whether the
// lock was set.
func (s *PGStore) RecordBillingTrustFailure(ctx context.Context, workspaceID, invoiceID string, at time.Time) (before, after BillingTrust, applied bool, err error) {
	if workspaceID == "" || invoiceID == "" || at.IsZero() {
		return BillingTrust{}, BillingTrust{}, false, fmt.Errorf("billing trust failure needs a workspace, an invoice and a time")
	}
	before, after, err = s.trustUpdate(ctx, workspaceID, func(tx pgx.Tx, _ BillingTrust) (string, []any, error) {
		var paid bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM billing_trust_invoices WHERE invoice_id = $1)`,
			invoiceID).Scan(&paid); err != nil || paid {
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
